package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/samply/blazectl/data"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"net/url"
	"os"
)

func CreateMeasureResource(m data.Measure, measureUrl string, libraryUrl string) fm.Measure {
	measure := fm.Measure{
		Url:    &measureUrl,
		Status: fm.PublicationStatusActive,
		Scoring: &fm.CodeableConcept{
			Coding: []fm.Coding{
				createCoding("http://terminology.hl7.org/CodeSystem/measure-scoring", "cohort"),
			},
		},
		Library: []string{libraryUrl},
		Group:   make([]fm.MeasureGroup, 0, len(m.Group)),
	}
	for _, group := range m.Group {
		measure.Group = append(measure.Group, createMeasureGroup(group))
	}
	return measure
}

func createMeasureGroup(g data.Group) fm.MeasureGroup {
	group := fm.MeasureGroup{
		Population: make([]fm.MeasureGroupPopulation, 0, len(g.Population)),
		Stratifier: make([]fm.MeasureGroupStratifier, 0, len(g.Stratifier)),
	}
	for _, population := range g.Population {
		group.Population = append(group.Population, createMeasureGroupPopulation(population))
	}
	for _, stratifier := range g.Stratifier {
		group.Stratifier = append(group.Stratifier, createMeasureGroupStratifier(stratifier))
	}
	return group
}

func createMeasureGroupPopulation(population data.Population) fm.MeasureGroupPopulation {
	return fm.MeasureGroupPopulation{
		Code: &fm.CodeableConcept{
			Coding: []fm.Coding{
				createCoding("http://terminology.hl7.org/CodeSystem/measure-population", "initial-population"),
			},
		},
		Criteria: fm.Expression{
			Language:   "text/cql-identifier",
			Expression: &population.Expression,
		},
	}
}

func createMeasureGroupStratifier(stratifier data.Stratifier) fm.MeasureGroupStratifier {
	return fm.MeasureGroupStratifier{
		Code: &fm.CodeableConcept{
			Text: &stratifier.Code,
		},
		Criteria: &fm.Expression{
			Language:   "text/cql-identifier",
			Expression: &stratifier.Expression,
		},
	}
}

func createCoding(system string, code string) fm.Coding {
	return fm.Coding{System: &system, Code: &code}
}

func CreateLibraryResource(m data.Measure, libraryUrl string) (*fm.Library, error) {
	libraryFile, err := os.ReadFile(m.Library)
	if err != nil {
		return nil, err
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

func randomUrl() (string, error) {
	myUuid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	return "urn:uuid:" + myUuid.String(), nil
}

var evaluateMeasureCmd = &cobra.Command{
	Use:   "evaluate-measure [measure-file]",
	Short: "Evaluates a Measure",
	Long: `Given a measure in YAML form, creates the required FHIR resources, 
evaluates that measure and returns the measure report.

See: https://github.com/samply/blaze/blob/master/docs/cql-queries/blazectl.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		measure, err := readMeasureFile(args[0])
		if err != nil {
			return err
		}

		measureUrl, err := randomUrl()
		if err != nil {
			return err
		}

		libraryUrl, err := randomUrl()
		if err != nil {
			return err
		}

		measureBytes, err := json.Marshal(CreateMeasureResource(*measure, measureUrl, libraryUrl))
		if err != nil {
			return err
		}

		library, err := CreateLibraryResource(*measure, libraryUrl)
		if err != nil {
			return err
		}

		libraryBytes, err := json.Marshal(library)
		if err != nil {
			return err
		}

		bundle := fm.Bundle{
			Type: fm.BundleTypeTransaction,
			Entry: []fm.BundleEntry{
				createBundleEntry("Library", libraryBytes),
				createBundleEntry("Measure", measureBytes),
			},
		}

		bundleBytes, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return err
		}

		err = createClient()
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
			_, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				return err
			}
		} else {
			_, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			return fmt.Errorf("can't create the Measure and/or Library Resource")
		}

		fmt.Fprintf(os.Stderr, "Evaluate measure with canonical URL %s on %s ...\n\n", measureUrl, server)

		measureReport, err := evaluateMeasure(measureUrl)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		measureReportBytes, err := json.MarshalIndent(measureReport, "", "  ")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Println(string(measureReportBytes))

		return nil
	},
}

func evaluateMeasure(measureUrl string) (*fm.MeasureReport, error) {
	req, err := client.NewTypeOperationRequest("Measure", "evaluate-measure",
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
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		measureReport := fm.MeasureReport{}
		err = json.Unmarshal(body, &measureReport)
		if err != nil {
			return nil, err
		}

		return &measureReport, nil
	} else {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		operationOutcome := fm.OperationOutcome{}

		err = json.Unmarshal(body, &operationOutcome)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("Error while evaluating the measure with canonical URL %s:\n\n%s",
			measureUrl, util.FmtOperationOutcomes([]*fm.OperationOutcome{&operationOutcome}))
	}
}

func init() {
	rootCmd.AddCommand(evaluateMeasureCmd)

	evaluateMeasureCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")

	_ = evaluateMeasureCmd.MarkFlagRequired("server")
}
