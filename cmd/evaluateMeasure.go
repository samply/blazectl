package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/samply/blazectl/data"
	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"
)

var forceSync bool

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

func createMeasureGroupStratifier(stratifier data.Stratifier) (*fm.MeasureGroupStratifier, error) {
	if stratifier.Code == "" {
		return nil, fmt.Errorf("missing code")
	}
	if stratifier.Expression == "" {
		return nil, fmt.Errorf("missing expression name")
	}
	return &fm.MeasureGroupStratifier{
		Code: &fm.CodeableConcept{
			Text: &stratifier.Code,
		},
		Criteria: &fm.Expression{
			Language:   "text/cql-identifier",
			Expression: &stratifier.Expression,
		},
	}, nil
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

func evaluateMeasure(client *fhir.Client, measureUrl string) ([]byte, error) {
	req, err := client.NewTypeOperationRequest("Measure", "evaluate-measure", !forceSync,
		url.Values{
			"measure":     []string{measureUrl},
			"periodStart": []string{"1900"},
			"periodEnd":   []string{"2200"},
		})
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		measureReportBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error while reading the MeasureReport: %v", err)
		}

		return measureReportBytes, nil
	} else if resp.StatusCode == 202 {
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
	} else {
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
			fmt.Println(err)
			os.Exit(1)
		}

		measureUrl, err := RandomUrl()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		libraryUrl, err := RandomUrl()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		measure, err := CreateMeasureResource(*m, measureUrl, libraryUrl)
		if err != nil {
			fmt.Printf("error while reading the measure file: %v\n", err)
			os.Exit(1)
		}

		library, err := CreateLibraryResource(*m, libraryUrl)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		measureBytes, err := json.Marshal(measure)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		libraryBytes, err := json.Marshal(library)
		if err != nil {
			fmt.Println(err)
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
			fmt.Println(err)
			os.Exit(1)
		}

		err = createClient()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		req, err := client.NewTransactionRequest(bytes.NewReader(bundleBytes))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			_, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else {
			_, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			return fmt.Errorf("can't create the Measure and/or Library Resource")
		}

		fmt.Fprintf(os.Stderr, "Evaluate measure with canonical URL %s on %s ...\n\n", measureUrl, server)

		measureReport, err := evaluateMeasureWithRetry(client, measureUrl)
		if err != nil {
			fmt.Println(err)
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

	_ = evaluateMeasureCmd.MarkFlagRequired("server")
}
