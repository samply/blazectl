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
// $evaluate-measure operation. The given CQL parameters, if any, are added as a
// nested Parameters resource under the `parameters` input parameter.
func evaluateMeasureParameters(measureUrl string, parameters []fm.ParametersParameter) (fm.Parameters, error) {
	body := fm.Parameters{
		Parameter: []fm.ParametersParameter{
			{Name: "measure", ValueString: new(measureUrl)},
			{Name: "periodStart", ValueDate: new("1900")},
			{Name: "periodEnd", ValueDate: new("2200")},
		},
	}
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
	return &fm.Library{
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
	}, nil
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

// newEvaluateMeasureRequest builds the $evaluate-measure request. If CQL
// parameters were given, they can only be transmitted via POST with a
// Parameters body. Otherwise, the operation is invoked via GET.
func newEvaluateMeasureRequest(client *fhir.Client, measureUrl string) (*http.Request, error) {
	if len(measureParameters) > 0 {
		body, err := evaluateMeasureParameters(measureUrl, measureParameters)
		if err != nil {
			return nil, err
		}
		return client.NewPostTypeOperationRequest("Measure", "evaluate-measure", !forceSync, body)
	}
	return client.NewTypeOperationRequest("Measure", "evaluate-measure", !forceSync,
		url.Values{
			"measure":     []string{measureUrl},
			"periodStart": []string{"1900"},
			"periodEnd":   []string{"2200"},
		})
}

func evaluateMeasure(client *fhir.Client, measureUrl string) ([]byte, error) {
	req, err := newEvaluateMeasureRequest(client, measureUrl)
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
			return nil, fmt.Errorf("Error while evaluating the measure with canonical URL %s:\n\n%w",
				measureUrl, err)
		}
		return measureReportBytes, nil
	default:
		return handleErrorResponse(resp)
	}
}

func evaluateMeasureWithRetry(client *fhir.Client, measureUrl string) ([]byte, error) {
	var lastErr error
	for wait := 100 * time.Millisecond; wait < 5*time.Second; wait *= 2 {
		measureReport, err := evaluateMeasure(client, measureUrl)
		lastErr = err
		if !isRetryable(errors.Unwrap(err)) {
			return measureReport, err
		}
		fmt.Fprintf(os.Stderr, "Retry evaluating the measure...\n")
		<-time.After(wait)
	}
	return nil, lastErr
}

var evaluateMeasureCmd = &cobra.Command{
	Use:   "evaluate-measure [measure-file]",
	Short: "Evaluates a Measure",
	Long: `Given a measure in YAML form, creates the required FHIR resources, 
evaluates that measure and returns the measure report.

Examples:
  blazectl evaluate-measure --server "http://localhost:8080/fhir" stratifier-condition-code.yml

  blazectl evaluate-measure --server "http://localhost:8080/fhir" \
    --parameter Gender=male --parameter MinAge:integer=18 gender-age.yml

See: https://github.com/samply/blaze/blob/main/docs/cql-queries/blazectl.md`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a measure-file argument")
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
		m, err := readMeasureFile(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

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

		measureUrl, err := RandomUrl()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		libraryUrl, err := RandomUrl()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		measure, err := CreateMeasureResource(*m, measureUrl, libraryUrl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error while reading the measure file: %v\n", err)
			os.Exit(1)
		}

		library, err := CreateLibraryResource(*m, libraryUrl)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		measureBytes, err := json.Marshal(measure)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		libraryBytes, err := json.Marshal(library)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		bundle := fm.Bundle{
			Type: fm.BundleTypeTransaction,
			Entry: []fm.BundleEntry{
				createBundleEntry("Library", libraryBytes),
				createBundleEntry("Measure", measureBytes),
			},
		}

		bundleBytes, err := json.Marshal(bundle)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		err = createClient()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		req, err := client.NewTransactionRequest(bytes.NewReader(bundleBytes))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			_, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		} else {
			_, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return fmt.Errorf("can't create the Measure and/or Library Resource")
		}

		fmt.Fprintf(os.Stderr, "Evaluate measure with canonical URL %s on %s ...\n\n", measureUrl, server)

		measureReport, err := evaluateMeasureWithRetry(client, measureUrl)
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
	evaluateMeasureCmd.Flags().StringArrayVarP(&rawMeasureParameters, "parameter", "p", nil,
		"set the value of a CQL parameter, in the form name=value or name:type=value "+
			"(supported types: "+supportedParameterTypes+"; the type defaults to string; "+
			"repeatable; repeated names form a list)")

	_ = evaluateMeasureCmd.MarkFlagRequired("server")
}
