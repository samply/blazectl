package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/google/uuid"
	"github.com/samply/blazectl/data"
	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
)

var forceSync bool
var rawMeasureParameters []string
var measureParameters []fm.ParametersParameter
var existingMeasureUrl string
var existingMeasureId string

// measureRef identifies the Measure to evaluate: either by canonical URL via
// the type-level $evaluate-measure operation or by resource ID via the
// instance-level one. Exactly one of the two fields is set.
type measureRef struct {
	url string
	id  string
}

// description names the referenced Measure in user-facing messages.
func (ref measureRef) description() string {
	if ref.id != "" {
		return fmt.Sprintf("ID %s", ref.id)
	}
	return fmt.Sprintf("canonical URL %s", ref.url)
}

// validateMeasureSource ensures that exactly one way to identify the measure is
// given: the measure-file argument, the --measure-url flag or the --measure-id
// flag.
func validateMeasureSource(args []string, measureUrl string, measureId string) error {
	numSources := 0
	if len(args) > 0 {
		numSources++
	}
	if measureUrl != "" {
		numSources++
	}
	if measureId != "" {
		numSources++
	}
	switch numSources {
	case 0:
		return errors.New("requires a measure-file argument or one of the --measure-url or --measure-id flags")
	case 1:
		return nil
	default:
		return errors.New("the measure-file argument and the --measure-url and --measure-id flags are mutually exclusive")
	}
}

// supportedParameterTypes lists the FHIR primitive types accepted for a CQL
// parameter, in the order they are documented to the user.
const supportedParameterTypes = "string, code, date, dateTime, boolean, integer, decimal"

// buildMeasureParameter builds a single FHIR ParametersParameter from a name,
// its declared type and a string value. Supported types are the common
// primitives: string, code, date, dateTime, boolean, integer and decimal.
func buildMeasureParameter(name string, parameterType string, value string) (fm.ParametersParameter, error) {
	parameter := fm.ParametersParameter{Name: name}
	switch parameterType {
	case "string":
		parameter.ValueString = new(value)
	case "code":
		parameter.ValueCode = new(value)
	case "date":
		parameter.ValueDate = new(value)
	case "dateTime":
		parameter.ValueDateTime = new(value)
	case "boolean":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fm.ParametersParameter{}, fmt.Errorf("invalid boolean value `%s` for parameter `%s`", value, name)
		}
		parameter.ValueBoolean = new(b)
	case "integer":
		i, err := strconv.Atoi(value)
		if err != nil {
			return fm.ParametersParameter{}, fmt.Errorf("invalid integer value `%s` for parameter `%s`", value, name)
		}
		parameter.ValueInteger = new(i)
	case "decimal":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fm.ParametersParameter{}, fmt.Errorf("invalid decimal value `%s` for parameter `%s`", value, name)
		}
		parameter.ValueDecimal = new(json.Number(value))
	default:
		return fm.ParametersParameter{}, fmt.Errorf("unsupported type `%s` for parameter `%s` (supported types: %s)", parameterType, name, supportedParameterTypes)
	}
	return parameter, nil
}

// defaultParameterType is the type assumed for a CQL parameter whose type is
// given neither on the command line nor in the measure file.
const defaultParameterType = "string"

// parsedParameter is a CQL parameter given on the command line via the
// `--parameter` flag. Its Type is empty when it wasn't specified, in which case
// the type declared in the measure file or, absent that, the string default
// applies. Repeating a name supplies multiple values, which map to a CQL list.
type parsedParameter struct {
	Name   string
	Type   string
	Values []string
}

// parseParameterOverrides parses the repeated `--parameter name[:type]=value`
// flags. The optional type sits between the name and the `=`; when omitted it is
// left empty so that a declared type or the string default can be applied later.
// Splitting on the first `=` keeps any `=` and `:` in the value intact. Repeating
// a name appends to its list; a type, if given more than once for the same name,
// must be consistent.
func parseParameterOverrides(rawOverrides []string) ([]parsedParameter, error) {
	var overrides []parsedParameter
	indexByName := make(map[string]int)
	for _, rawOverride := range rawOverrides {
		nameType, value, ok := strings.Cut(rawOverride, "=")
		if !ok {
			return nil, fmt.Errorf("invalid parameter `%s`: expected format name=value or name:type=value", rawOverride)
		}
		name, parameterType, _ := strings.Cut(nameType, ":")
		if name == "" {
			return nil, fmt.Errorf("invalid parameter `%s`: expected format name=value or name:type=value", rawOverride)
		}
		if i, ok := indexByName[name]; ok {
			if parameterType != "" {
				if overrides[i].Type != "" && overrides[i].Type != parameterType {
					return nil, fmt.Errorf("conflicting types `%s` and `%s` for parameter `%s`", overrides[i].Type, parameterType, name)
				}
				overrides[i].Type = parameterType
			}
			overrides[i].Values = append(overrides[i].Values, value)
			continue
		}
		indexByName[name] = len(overrides)
		overrides = append(overrides, parsedParameter{Name: name, Type: parameterType, Values: []string{value}})
	}
	return overrides, nil
}

// buildTypedParameters builds the FHIR parameters for a single named CQL
// parameter and its value(s). An empty type defaults to string. Multiple values
// are mapped to a CQL list by repeating the parameter.
func buildTypedParameters(name string, parameterType string, values []string) ([]fm.ParametersParameter, error) {
	if parameterType == "" {
		parameterType = defaultParameterType
	}
	var parameters []fm.ParametersParameter
	for _, value := range values {
		parameter, err := buildMeasureParameter(name, parameterType, value)
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, parameter)
	}
	return parameters, nil
}

// buildMeasureParameters builds the FHIR parameters from the parameters given on
// the command line. The type of a parameter is taken from the command line if
// given there, otherwise it defaults to string. A parameter with multiple values
// (a repeated name) is mapped to a CQL list by repeating it.
func buildMeasureParameters(overrides []parsedParameter) ([]fm.ParametersParameter, error) {
	var parameters []fm.ParametersParameter
	for _, override := range overrides {
		built, err := buildTypedParameters(override.Name, override.Type, override.Values)
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, built...)
	}
	return parameters, nil
}

// evaluateMeasureParameters builds the FHIR Parameters body for the
// $evaluate-measure operation. The `measure` input parameter is only needed for
// the type-level operation, because the instance-level one identifies the
// Measure via the URL path. The given CQL parameters, if any, are added as a
// nested Parameters resource under the `parameters` input parameter.
func evaluateMeasureParameters(ref measureRef, parameters []fm.ParametersParameter) (fm.Parameters, error) {
	var body fm.Parameters
	if ref.url != "" {
		body.Parameter = append(body.Parameter, fm.ParametersParameter{Name: "measure", ValueString: new(ref.url)})
	}
	body.Parameter = append(body.Parameter,
		fm.ParametersParameter{Name: "periodStart", ValueDate: new("1900")},
		fm.ParametersParameter{Name: "periodEnd", ValueDate: new("2200")},
	)
	if len(parameters) > 0 {
		resource, err := json.Marshal(fm.Parameters{Parameter: parameters})
		if err != nil {
			return fm.Parameters{}, err
		}
		body.Parameter = append(body.Parameter, fm.ParametersParameter{
			Name:     "parameters",
			Resource: resource,
		})
	}
	return body, nil
}

func CreateMeasureResource(m data.Measure, measureUrl string, libraryUrl string) (*fm.Measure, error) {
	if len(m.Group) == 0 {
		return nil, fmt.Errorf("missing group")
	}
	measure := fm.Measure{
		Url:    &measureUrl,
		Status: fm.PublicationStatusActive,
		SubjectCodeableConcept: &fm.CodeableConcept{
			Coding: []fm.Coding{
				createCoding("http://hl7.org/fhir/resource-types", "Patient"),
			},
		},
		Library: []string{libraryUrl},
		Scoring: &fm.CodeableConcept{
			Coding: []fm.Coding{
				createCoding("http://terminology.hl7.org/CodeSystem/measure-scoring", "cohort"),
			},
		},
		Group: make([]fm.MeasureGroup, 0, len(m.Group)),
	}
	if m.Version != "" {
		measure.Version = &m.Version
	}
	for i, group := range m.Group {
		g, err := createMeasureGroup(group)
		if err != nil {
			return nil, fmt.Errorf("error in group[%d]: %v", i, err)
		}
		measure.Group = append(measure.Group, *g)
	}
	return &measure, nil
}

func createMeasureGroup(g data.Group) (*fm.MeasureGroup, error) {
	if len(g.Population) == 0 {
		return nil, fmt.Errorf("missing population")
	}
	group := fm.MeasureGroup{
		Population: make([]fm.MeasureGroupPopulation, 0, len(g.Population)),
		Stratifier: make([]fm.MeasureGroupStratifier, 0, len(g.Stratifier)),
	}
	if g.Type != "Patient" {
		group.Extension = []fm.Extension{
			{
				Url:       "http://hl7.org/fhir/us/cqfmeasures/StructureDefinition/cqfm-populationBasis",
				ValueCode: &g.Type,
			},
		}
	}
	if g.Code != "" {
		group.Code = &fm.CodeableConcept{Text: &g.Code}
	}
	if g.Description != "" {
		group.Description = &g.Description
	}
	for i, population := range g.Population {
		p, err := createMeasureGroupPopulation(population)
		if err != nil {
			return nil, fmt.Errorf("population[%d]: %v", i, err)
		}
		group.Population = append(group.Population, *p)
	}
	for i, stratifier := range g.Stratifier {
		s, err := createMeasureGroupStratifier(stratifier)
		if err != nil {
			return nil, fmt.Errorf("stratifier[%d]: %v", i, err)
		}
		group.Stratifier = append(group.Stratifier, *s)
	}
	return &group, nil
}

func createMeasureGroupPopulation(population data.Population) (*fm.MeasureGroupPopulation, error) {
	if population.Expression == "" {
		return nil, fmt.Errorf("missing expression name")
	}
	return &fm.MeasureGroupPopulation{
		Code: &fm.CodeableConcept{
			Coding: []fm.Coding{
				createCoding("http://terminology.hl7.org/CodeSystem/measure-population", "initial-population"),
			},
		},
		Criteria: fm.Expression{
			Language:   "text/cql-identifier",
			Expression: &population.Expression,
		},
	}, nil
}

func createMeasureGroupStratifier(s data.Stratifier) (*fm.MeasureGroupStratifier, error) {
	if s.Code == "" {
		return nil, fmt.Errorf("missing code")
	}
	if s.Expression == "" {
		return nil, fmt.Errorf("missing expression name")
	}
	stratifier := fm.MeasureGroupStratifier{
		Code: &fm.CodeableConcept{
			Text: &s.Code,
		},
		Criteria: &fm.Expression{
			Language:   "text/cql-identifier",
			Expression: &s.Expression,
		},
	}
	if s.Description != "" {
		stratifier.Description = &s.Description
	}
	return &stratifier, nil
}

func createCoding(system string, code string) fm.Coding {
	return fm.Coding{System: &system, Code: &code}
}

func CreateLibraryResource(m data.Measure, libraryUrl string) (*fm.Library, error) {
	if m.Library == "" {
		return nil, fmt.Errorf("error while reading the measure file: missing CQL library filename")
	}
	libraryFile, err := os.ReadFile(m.Library)
	if err != nil {
		return nil, fmt.Errorf("error while reading the CQL library file: %v", err)
	}
	library := fm.Library{
		Url:    &libraryUrl,
		Status: fm.PublicationStatusActive,
		Type: fm.CodeableConcept{
			Coding: []fm.Coding{
				createCoding("http://terminology.hl7.org/CodeSystem/library-type", "logic-library"),
			},
		},
		Content: []fm.Attachment{
			createAttachment("text/cql", base64.StdEncoding.EncodeToString(libraryFile)),
		},
	}
	name, version := parseCqlLibraryIdentifier(libraryFile)
	if name != "" {
		library.Name = &name
	}
	if version != "" {
		library.Version = &version
	}
	return &library, nil
}

// cqlLibraryDefinition matches the library definition of a CQL library,
// capturing the name, either quoted or unquoted, and the optional version. It
// skips any comments that precede it.
var cqlLibraryDefinition = regexp.MustCompile(
	`^(?:\s+|//[^\n]*|/\*(?s:.*?)\*/)*library\s+(?:"((?:[^"\\]|\\.)*)"|([A-Za-z_][A-Za-z0-9_.]*))(?:\s+version\s+'((?:[^'\\]|\\.)*)')?`)

// parseCqlLibraryIdentifier returns the name and version of the given CQL
// library. Both are empty if the library has no library definition, and the
// version is empty if the library definition has none.
func parseCqlLibraryIdentifier(cql []byte) (name string, version string) {
	match := cqlLibraryDefinition.FindSubmatch(cql)
	if match == nil {
		return "", ""
	}
	return string(match[1]) + string(match[2]), string(match[3])
}

func createAttachment(contentType string, data string) fm.Attachment {
	return fm.Attachment{
		ContentType: &contentType,
		Data:        &data,
	}
}

func createBundleEntry(url string, resource []byte) fm.BundleEntry {
	return fm.BundleEntry{
		Resource: resource,
		Request: &fm.BundleEntryRequest{
			Method: fm.HTTPVerbPOST,
			Url:    url,
		},
	}
}

func readMeasureFile(filename string) (*data.Measure, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	measure := data.Measure{}

	err = yaml.Unmarshal(file, &measure)
	if err != nil {
		return nil, err
	}
	return &measure, nil
}

func RandomUrl() (string, error) {
	myUuid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return "urn:uuid:" + myUuid.String(), nil
}

func isTransient(issue fm.OperationOutcomeIssue) bool {
	switch issue.Code {
	case fm.IssueTypeTransient,
		fm.IssueTypeLockError,
		fm.IssueTypeNoStore,
		fm.IssueTypeTimeout,
		fm.IssueTypeIncomplete,
		fm.IssueTypeThrottled:
		return true
	default:
		return false
	}
}

type operationOutcomeError struct {
	outcome *fm.OperationOutcome
}

func (err *operationOutcomeError) Error() string {
	return util.FmtOperationOutcomes([]*fm.OperationOutcome{err.outcome})
}

type retryableError interface {
	retryable() bool
}

func (err *operationOutcomeError) retryable() bool {
	for _, issue := range err.outcome.Issue {
		if isTransient(issue) {
			return true
		}
	}
	return false
}

func isRetryable(err error) bool {
	if re, ok := err.(retryableError); ok {
		return re.retryable()
	}
	return false
}

func handleErrorResponse(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if fhir.IsFhirResponse(resp) {
		operationOutcome := fm.OperationOutcome{}

		err = json.Unmarshal(body, &operationOutcome)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("%w", &operationOutcomeError{outcome: &operationOutcome})
	} else {
		return nil, fmt.Errorf("%s", body)
	}
}

// newEvaluateMeasureRequest builds the $evaluate-measure request. A measure
// referenced by ID is evaluated via the instance-level operation, one
// referenced by canonical URL via the type-level operation. If CQL parameters
// were given, they can only be transmitted via POST with a Parameters body.
// Otherwise, the operation is invoked via GET.
func newEvaluateMeasureRequest(client *fhir.Client, ref measureRef) (*http.Request, error) {
	if len(measureParameters) > 0 {
		body, err := evaluateMeasureParameters(ref, measureParameters)
		if err != nil {
			return nil, err
		}
		if ref.id != "" {
			return client.NewPostInstanceOperationRequest("Measure", ref.id, "evaluate-measure", !forceSync, body)
		}
		return client.NewPostTypeOperationRequest("Measure", "evaluate-measure", !forceSync, body)
	}
	query := url.Values{
		"periodStart": []string{"1900"},
		"periodEnd":   []string{"2200"},
	}
	if ref.id != "" {
		return client.NewInstanceOperationRequest("Measure", ref.id, "evaluate-measure", !forceSync, query)
	}
	query.Set("measure", ref.url)
	return client.NewTypeOperationRequest("Measure", "evaluate-measure", !forceSync, query)
}

func evaluateMeasure(client *fhir.Client, ref measureRef) ([]byte, error) {
	req, err := newEvaluateMeasureRequest(client, ref)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	// Blaze returns 200 for the GET form and can return 201 (Created) for the POST form of
	// $evaluate-measure, because the latter may persist the MeasureReport.
	case http.StatusOK, http.StatusCreated:
		measureReportBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error while reading the MeasureReport: %v", err)
		}

		return measureReportBytes, nil
	case http.StatusAccepted:
		contentLocation := resp.Header.Get("Content-Location")
		if err := fhir.DiscardAndClose(resp.Body); err != nil {
			return nil, err
		}
		interruptChan := make(chan os.Signal, 1)
		signal.Notify(interruptChan, os.Interrupt)
		measureReportBytes, err := client.PollAsyncStatus(contentLocation, interruptChan)
		if err != nil {
			return nil, fmt.Errorf("Error while evaluating the measure with %s:\n\n%w",
				ref.description(), err)
		}
		return measureReportBytes, nil
	default:
		return handleErrorResponse(resp)
	}
}

func evaluateMeasureWithRetry(client *fhir.Client, ref measureRef) ([]byte, error) {
	var lastErr error
	for wait := 100 * time.Millisecond; wait < 5*time.Second; wait *= 2 {
		measureReport, err := evaluateMeasure(client, ref)
		lastErr = err
		if !isRetryable(errors.Unwrap(err)) {
			return measureReport, err
		}
		fmt.Fprintf(os.Stderr, "Retry evaluating the measure...\n")
		<-time.After(wait)
	}
	return nil, lastErr
}

// searchResources searches for resources of the given type matching the query
// and returns the matches of the first page.
func searchResources(client *fhir.Client, resourceType string, query url.Values) ([]json.RawMessage, error) {
	req, err := client.NewSearchTypeRequest(resourceType, query)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, err := handleErrorResponse(resp)
		return nil, fmt.Errorf("error while searching for %s resources: %w", resourceType, err)
	}

	bundle, err := fhir.ReadBundle(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading the %s search result: %w", resourceType, err)
	}

	resources := make([]json.RawMessage, 0, len(bundle.Entry))
	for _, entry := range bundle.Entry {
		if entry.Search != nil && entry.Search.Mode != nil && *entry.Search.Mode != fm.SearchEntryModeMatch {
			continue
		}
		resources = append(resources, entry.Resource)
	}
	return resources, nil
}

// findExistingResource searches for resources of the given type matching the
// query and returns the one for which matchesExactly returns true, or nil if
// there is none. Returns an error if there is more than one. The description
// names the searched resource in error messages.
func findExistingResource[T any](client *fhir.Client, resourceType string, query url.Values, description string,
	unmarshal func([]byte) (T, error), matchesExactly func(T) bool) (*T, error) {
	resources, err := searchResources(client, resourceType, query)
	if err != nil {
		return nil, err
	}

	var found []T
	for _, resource := range resources {
		r, err := unmarshal(resource)
		if err != nil {
			return nil, fmt.Errorf("error while reading a %s resource: %w", resourceType, err)
		}
		if matchesExactly(r) {
			found = append(found, r)
		}
	}

	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return &found[0], nil
	default:
		return nil, fmt.Errorf("found %d %s resources with %s on the server, expected at most one",
			len(found), resourceType, description)
	}
}

// canonical returns the canonical reference to the resource with the given URL
// and the version, if not empty.
func canonical(url string, version string) string {
	if version == "" {
		return url
	}
	return url + "|" + version
}

func equalStringPtr(a *string, b *string) bool {
	return a != nil && b != nil && *a == *b
}

// cqlContent returns the decoded CQL content of the library, or nil if it has
// none.
func cqlContent(library fm.Library) ([]byte, error) {
	for _, attachment := range library.Content {
		if equalStringPtr(attachment.ContentType, new("text/cql")) && attachment.Data != nil {
			return base64.StdEncoding.DecodeString(*attachment.Data)
		}
	}
	return nil, nil
}

// findExistingLibrary returns the Library on the server with the name and
// version of the given library, or nil if there is none or the given library
// lacks a name or a version. Returns an error if the CQL content of the
// existing Library differs from the content of the given one.
func findExistingLibrary(client *fhir.Client, library fm.Library) (*fm.Library, error) {
	if library.Name == nil || library.Version == nil {
		return nil, nil
	}

	description := fmt.Sprintf("name `%s` and version `%s`", *library.Name, *library.Version)
	query := url.Values{"name": {*library.Name}, "version": {*library.Version}}
	existing, err := findExistingResource(client, "Library", query, description, fm.UnmarshalLibrary,
		func(l fm.Library) bool {
			// the name search parameter also matches names starting with the given name
			return equalStringPtr(l.Name, library.Name) && equalStringPtr(l.Version, library.Version)
		})
	if err != nil || existing == nil {
		return nil, err
	}

	existingContent, err := cqlContent(*existing)
	if err != nil {
		return nil, fmt.Errorf("error while reading the content of the Library with %s: %w", description, err)
	}
	content, err := cqlContent(library)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(existingContent, content) {
		return nil, fmt.Errorf("the Library with %s already exists on the server but its content differs from the CQL library file", description)
	}
	if existing.Url == nil {
		return nil, fmt.Errorf("the Library with %s exists on the server but has no canonical URL", description)
	}
	return existing, nil
}

// withoutServerManagedElements returns the JSON representation of the Measure
// without the elements the server manages.
func withoutServerManagedElements(measure fm.Measure) ([]byte, error) {
	measure.Id = nil
	measure.Meta = nil
	measure.Text = nil
	return json.Marshal(measure)
}

// findExistingMeasure returns the Measure on the server with the canonical URL
// and, if given, version of the given measure, or nil if there is none.
// Returns an error if the existing Measure differs from the given one.
func findExistingMeasure(client *fhir.Client, measure fm.Measure) (*fm.Measure, error) {
	description := fmt.Sprintf("canonical URL `%s`", *measure.Url)
	query := url.Values{"url": {*measure.Url}}
	if measure.Version != nil {
		description += fmt.Sprintf(" and version `%s`", *measure.Version)
		query.Set("version", *measure.Version)
	}
	existing, err := findExistingResource(client, "Measure", query, description, fm.UnmarshalMeasure,
		func(m fm.Measure) bool {
			return equalStringPtr(m.Url, measure.Url) && (measure.Version == nil || equalStringPtr(m.Version, measure.Version))
		})
	if err != nil || existing == nil {
		return nil, err
	}

	existingContent, err := withoutServerManagedElements(*existing)
	if err != nil {
		return nil, err
	}
	content, err := withoutServerManagedElements(measure)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(existingContent, content) {
		return nil, fmt.Errorf("the Measure with %s already exists on the server but its content differs from the measure file", description)
	}
	return existing, nil
}

// createResources creates the resources of the given bundle entries in one
// transaction. Does nothing if there are no entries.
func createResources(client *fhir.Client, entries []fm.BundleEntry) error {
	if len(entries) == 0 {
		return nil
	}

	bundle := fm.Bundle{
		Type:  fm.BundleTypeTransaction,
		Entry: entries,
	}

	bundleBytes, err := json.Marshal(bundle)
	if err != nil {
		return err
	}

	req, err := client.NewTransactionRequest(bytes.NewReader(bundleBytes))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			return err
		}
	} else {
		if _, err := io.ReadAll(resp.Body); err != nil {
			return err
		}
		return fmt.Errorf("can't create the Measure and/or Library Resource")
	}

	return nil
}

// createMeasureResources reads the measure file and returns a reference to the
// Measure to evaluate. A Library with the name and version of the CQL library
// and a Measure with the canonical URL (and version) of the measure file are
// reused if they already exist on the server with the same content. Otherwise,
// they are created, using random canonical URLs if none are given.
func createMeasureResources(client *fhir.Client, measureFile string) (measureRef, error) {
	m, err := readMeasureFile(measureFile)
	if err != nil {
		return measureRef{}, err
	}

	measureUrl := m.Url
	if measureUrl == "" {
		measureUrl, err = RandomUrl()
		if err != nil {
			return measureRef{}, err
		}
	}

	libraryUrl, err := RandomUrl()
	if err != nil {
		return measureRef{}, err
	}

	library, err := CreateLibraryResource(*m, libraryUrl)
	if err != nil {
		return measureRef{}, err
	}

	existingLibrary, err := findExistingLibrary(client, *library)
	if err != nil {
		return measureRef{}, err
	}
	if existingLibrary != nil {
		libraryUrl = *existingLibrary.Url
	}
	var libraryVersion string
	if library.Version != nil {
		libraryVersion = *library.Version
	}

	measure, err := CreateMeasureResource(*m, measureUrl, canonical(libraryUrl, libraryVersion))
	if err != nil {
		return measureRef{}, fmt.Errorf("error while reading the measure file: %v", err)
	}

	var entries []fm.BundleEntry

	if existingLibrary == nil {
		libraryBytes, err := json.Marshal(library)
		if err != nil {
			return measureRef{}, err
		}
		entries = append(entries, createBundleEntry("Library", libraryBytes))
	}

	var existingMeasure *fm.Measure
	if m.Url != "" {
		existingMeasure, err = findExistingMeasure(client, *measure)
		if err != nil {
			return measureRef{}, err
		}
	}
	if existingMeasure == nil {
		measureBytes, err := json.Marshal(measure)
		if err != nil {
			return measureRef{}, err
		}
		entries = append(entries, createBundleEntry("Measure", measureBytes))
	}

	if err := createResources(client, entries); err != nil {
		return measureRef{}, err
	}

	return measureRef{url: canonical(measureUrl, m.Version)}, nil
}

var evaluateMeasureCmd = &cobra.Command{
	Use:   "evaluate-measure [measure-file]",
	Short: "Evaluates a Measure",
	Long: `Given a measure in YAML form, creates the required FHIR resources,
evaluates that measure and returns the measure report.

If the CQL library declares a name and a version, an existing Library with that
name and version is reused if its content matches the CQL library file. If the
measure file declares a url and optionally a version, an existing Measure with
that canonical URL and version is reused if its content matches the measure
file. An existing resource with different content results in an error.

Instead of a measure file, an existing Measure on the server can be referenced
by its canonical URL with --measure-url or by its resource ID with
--measure-id. In that case no resources are created.

Examples:
  blazectl evaluate-measure --server "http://localhost:8080/fhir" stratifier-condition-code.yml

  blazectl evaluate-measure --server "http://localhost:8080/fhir" \
    --parameter Gender=male --parameter MinAge:integer=18 gender-age.yml

  blazectl evaluate-measure --server "http://localhost:8080/fhir" \
    --measure-url "https://example.com/fhir/Measure/example"

  blazectl evaluate-measure --server "http://localhost:8080/fhir" \
    --measure-id DACG22F3LKPU7ZZ5

See: https://github.com/samply/blaze/blob/main/docs/cql-queries/blazectl.md`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := validateMeasureSource(args, existingMeasureUrl, existingMeasureId); err != nil {
			return err
		}
		if len(args) == 0 {
			return nil
		}
		if info, err := os.Stat(args[0]); os.IsNotExist(err) {
			return fmt.Errorf("measure file `%s` doesn't exist", args[0])
		} else if info.IsDir() {
			return fmt.Errorf("`%s` is a directory", args[0])
		} else {
			return nil
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		overrides, err := parseParameterOverrides(rawMeasureParameters)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		measureParameters, err = buildMeasureParameters(overrides)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		err = createClient()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		ref := measureRef{url: existingMeasureUrl, id: existingMeasureId}
		if len(args) > 0 {
			ref, err = createMeasureResources(client, args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}

		fmt.Fprintf(os.Stderr, "Evaluate measure with %s on %s ...\n\n", ref.description(), server)

		measureReport, err := evaluateMeasureWithRetry(client, ref)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Println(string(measureReport))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(evaluateMeasureCmd)

	evaluateMeasureCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")
	evaluateMeasureCmd.Flags().BoolVarP(&forceSync, "force-sync", "", false, "force synchronous responses")
	evaluateMeasureCmd.Flags().StringVar(&existingMeasureUrl, "measure-url", "",
		"the canonical URL of an existing Measure to evaluate instead of a measure file")
	evaluateMeasureCmd.Flags().StringVar(&existingMeasureId, "measure-id", "",
		"the resource ID of an existing Measure to evaluate instead of a measure file")
	evaluateMeasureCmd.Flags().StringArrayVarP(&rawMeasureParameters, "parameter", "p", nil,
		"set the value of a CQL parameter, in the form name=value or name:type=value "+
			"(supported types: "+supportedParameterTypes+"; the type defaults to string; "+
			"repeatable; repeated names form a list)")

	_ = evaluateMeasureCmd.MarkFlagRequired("server")
}
