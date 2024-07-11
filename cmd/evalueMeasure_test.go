package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/samply/blazectl/data"
	"github.com/samply/blazectl/fhir"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCreateMeasureResource(t *testing.T) {
	measureUrl, err := RandomUrl()
	if err != nil {
		t.Fatalf("error while generating random URL: %v", err)
	}

	libraryUrl, err := RandomUrl()
	if err != nil {
		t.Fatalf("error while generating random URL: %v", err)
	}

	t.Run("empty Measure", func(t *testing.T) {
		m := data.Measure{}

		_, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err == nil {
			t.Fatal("expected error")
		}

		assert.Equal(t, "missing group", err.Error())
	})

	t.Run("with one empty group", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{},
			},
		}

		_, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err == nil {
			t.Fatal("expected error")
		}

		assert.Equal(t, "error in group[0]: missing population", err.Error())
	})

	t.Run("with one group and one empty population", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{
					Population: []data.Population{
						{},
					},
				},
			},
		}

		_, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err == nil {
			t.Fatal("expected error")
		}

		assert.Equal(t, "error in group[0]: population[0]: missing expression name", err.Error())
	})

	t.Run("with one group and one population", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{
					Population: []data.Population{
						{
							Expression: "InInitialPopulation",
						},
					},
				},
			},
		}

		resource, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err != nil {
			t.Fatalf("error while generating the measure resource: %v", err)
		}

		assert.Equal(t, measureUrl, *resource.Url)
		assert.Equal(t, fm.PublicationStatusActive, resource.Status)
		assert.Equal(t, "http://hl7.org/fhir/resource-types", *resource.SubjectCodeableConcept.Coding[0].System)
		assert.Equal(t, "Patient", *resource.SubjectCodeableConcept.Coding[0].Code)
		assert.Equal(t, 1, len(resource.Library))
		assert.Equal(t, libraryUrl, resource.Library[0])
		assert.Equal(t, 1, len(resource.Scoring.Coding))
		assert.Equal(t, "http://terminology.hl7.org/CodeSystem/measure-scoring", *resource.Scoring.Coding[0].System)
		assert.Equal(t, "cohort", *resource.Scoring.Coding[0].Code)
		assert.Equal(t, 1, len(resource.Group))
		assert.Equal(t, 1, len(resource.Group[0].Population))
		assert.Equal(t, 1, len(resource.Group[0].Population[0].Code.Coding))
		assert.Equal(t, "http://terminology.hl7.org/CodeSystem/measure-population", *resource.Group[0].Population[0].Code.Coding[0].System)
		assert.Equal(t, "initial-population", *resource.Group[0].Population[0].Code.Coding[0].Code)
		assert.Equal(t, "text/cql-identifier", resource.Group[0].Population[0].Criteria.Language)
		assert.Equal(t, "InInitialPopulation", *resource.Group[0].Population[0].Criteria.Expression)
	})

	t.Run("with one group and one population and one empty stratifier", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{
					Population: []data.Population{
						{
							Expression: "InInitialPopulation",
						},
					},
					Stratifier: []data.Stratifier{
						{},
					},
				},
			},
		}

		_, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err == nil {
			t.Fatal("expected error")
		}

		assert.Equal(t, "error in group[0]: stratifier[0]: missing code", err.Error())
	})

	t.Run("with one group and one population and one stratifier with missing expression", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{
					Population: []data.Population{
						{
							Expression: "InInitialPopulation",
						},
					},
					Stratifier: []data.Stratifier{
						{
							Code: "foo",
						},
					},
				},
			},
		}

		_, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err == nil {
			t.Fatal("expected error")
		}

		assert.Equal(t, "error in group[0]: stratifier[0]: missing expression name", err.Error())
	})

	t.Run("with one group and one population and one stratifier", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{
					Population: []data.Population{
						{
							Expression: "InInitialPopulation",
						},
					},
					Stratifier: []data.Stratifier{
						{
							Code:       "foo",
							Expression: "Foo",
						},
					},
				},
			},
		}

		resource, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err != nil {
			t.Fatalf("error while generating the measure resource: %v", err)
		}

		assert.Equal(t, 1, len(resource.Group))
		assert.Equal(t, 1, len(resource.Group[0].Stratifier))
		assert.Equal(t, "foo", *resource.Group[0].Stratifier[0].Code.Text)
	})

	t.Run("with one Condition group", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{
					Type: "Condition",
					Population: []data.Population{
						{
							Expression: "InInitialPopulation",
						},
					},
				},
			},
		}

		resource, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err != nil {
			t.Fatalf("error while generating the measure resource: %v", err)
		}

		assert.Equal(t, 1, len(resource.Group))
		assert.Equal(t, 1, len(resource.Group[0].Extension))
		assert.Equal(t, "http://hl7.org/fhir/us/cqfmeasures/StructureDefinition/cqfm-populationBasis", resource.Group[0].Extension[0].Url)
		assert.Equal(t, "Condition", *resource.Group[0].Extension[0].ValueCode)
	})
}

func TestCreateLibraryResource(t *testing.T) {
	libraryUrl, err := RandomUrl()
	if err != nil {
		t.Fatalf("error while generating random URL: %v", err)
	}

	t.Run("empty Measure", func(t *testing.T) {
		m := data.Measure{}

		_, err := CreateLibraryResource(m, libraryUrl)
		if err == nil {
			t.Fatal("expected error")
		}

		assert.Equal(t, "error while reading the measure file: missing CQL library filename", err.Error())
	})

	t.Run("empty Library filename", func(t *testing.T) {
		m := data.Measure{
			Library: "",
		}

		_, err := CreateLibraryResource(m, libraryUrl)
		if err == nil {
			t.Fatal("expected error")
		}

		assert.Equal(t, "error while reading the measure file: missing CQL library filename", err.Error())
	})

	t.Run("empty Library filename", func(t *testing.T) {
		m := data.Measure{
			Library: "foo",
		}

		_, err := CreateLibraryResource(m, libraryUrl)
		if err == nil {
			t.Fatal("expected error")
		}

		assert.Equal(t, "error while reading the CQL library file: open foo: no such file or directory", err.Error())
	})

	t.Run("success", func(t *testing.T) {
		m := data.Measure{
			Library: "all.cql",
		}

		resource, err := CreateLibraryResource(m, libraryUrl)
		if err != nil {
			t.Fatalf("error while generating the library resource: %v", err)
		}

		assert.Equal(t, libraryUrl, *resource.Url)
		assert.Equal(t, fm.PublicationStatusActive, resource.Status)
		assert.Equal(t, 1, len(resource.Type.Coding))
		assert.Equal(t, "http://terminology.hl7.org/CodeSystem/library-type", *resource.Type.Coding[0].System)
		assert.Equal(t, "logic-library", *resource.Type.Coding[0].Code)
		assert.Equal(t, 1, len(resource.Content))
		assert.Equal(t, "text/cql", *resource.Content[0].ContentType)
		assert.Equal(t, "bGlicmFyeSAiYWxsIgp1c2luZyBGSElSIHZlcnNpb24gJzQuMC4wJwoKZGVmaW5lIEluSW5pdGlhbFBvcHVsYXRpb246CiAgdHJ1ZQo=", *resource.Content[0].Data)
	})
}

func TestEvaluateMeasure(t *testing.T) {

	t.Run("Request to FHIR server fails", func(t *testing.T) {
		baseURL, _ := url.ParseRequestURI("http://localhost")
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasure(client, "foo")

		assert.Error(t, err)
	})

	t.Run("Successful return empty body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simply do not respond with anything
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		measureReport, _ := evaluateMeasure(client, "foo")

		assert.Equal(t, 0, len(measureReport))
	})

	t.Run("missing parameter error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := fm.OperationOutcome{
				Issue: []fm.OperationOutcomeIssue{{
					Severity: fm.IssueSeverityError,
					Code:     fm.IssueTypeValue,
				}},
			}

			w.Header().Set("Content-Type", "application/fhir+json")
			w.WriteHeader(http.StatusBadRequest)
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasure(client, "foo")

		assert.Contains(t, err.Error(), "An element or header value is invalid.")
	})

	t.Run("timeout error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := fm.OperationOutcome{
				Issue: []fm.OperationOutcomeIssue{{
					Severity: fm.IssueSeverityError,
					Code:     fm.IssueTypeTimeout,
				}},
			}

			w.Header().Set("Content-Type", "application/fhir+json")
			w.WriteHeader(http.StatusServiceUnavailable)
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasure(client, "foo")

		assert.True(t, isRetryable(errors.Unwrap(err)))
	})

	t.Run("timeout error response with successful retry", func(t *testing.T) {
		numResp := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if numResp == 0 {
				response := fm.OperationOutcome{
					Issue: []fm.OperationOutcomeIssue{{
						Severity: fm.IssueSeverityError,
						Code:     fm.IssueTypeTimeout,
					}},
				}

				w.Header().Set("Content-Type", "application/fhir+json")
				w.WriteHeader(http.StatusServiceUnavailable)
				encoder := json.NewEncoder(w)
				if err := encoder.Encode(response); err != nil {
					t.Error(err)
				}
				numResp++
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		measureReport, err := evaluateMeasureWithRetry(client, "foo")

		assert.Equal(t, 0, len(measureReport))
		assert.Nil(t, err)
	})

	t.Run("timeout error response with unsuccessful retry", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := fm.OperationOutcome{
				Issue: []fm.OperationOutcomeIssue{{
					Severity: fm.IssueSeverityError,
					Code:     fm.IssueTypeTimeout,
				}},
			}

			w.Header().Set("Content-Type", "application/fhir+json")
			w.WriteHeader(http.StatusServiceUnavailable)
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasureWithRetry(client, "foo")

		assert.Contains(t, err.Error(), "An internal timeout has occurred.")
	})

	t.Run("async response with empty response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/Measure/$evaluate-measure":
				w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
				w.WriteHeader(http.StatusAccepted)
			case "/async-poll":
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasure(client, "foo")

		assert.Contains(t, err.Error(), "error while reading the async response Bundle: unexpected end of JSON input")
	})

	t.Run("async response with non JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/Measure/$evaluate-measure":
				w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
				w.WriteHeader(http.StatusAccepted)
			case "/async-poll":
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte{'{'})
				if err != nil {
					t.Error(err)
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasure(client, "foo")

		assert.Contains(t, err.Error(), "error while reading the async response Bundle: unexpected end of JSON input")
	})

	t.Run("async error response with non JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/Measure/$evaluate-measure":
				w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
				w.WriteHeader(http.StatusAccepted)
			case "/async-poll":
				w.WriteHeader(http.StatusServiceUnavailable)
				_, err := w.Write([]byte("unavailable"))
				if err != nil {
					t.Error(err)
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasure(client, "foo")

		assert.Contains(t, err.Error(), "Error while evaluating the measure with canonical URL foo:\n\nunavailable")
	})

	t.Run("async response with missing bundle entry", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/Measure/$evaluate-measure":
				w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
				w.WriteHeader(http.StatusAccepted)
			case "/async-poll":
				response := fm.Bundle{}

				w.WriteHeader(http.StatusOK)
				encoder := json.NewEncoder(w)
				if err := encoder.Encode(response); err != nil {
					t.Error(err)
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasure(client, "foo")

		assert.Contains(t, err.Error(), "expected one entry in async response Bundle but was 0 entries")
	})

	t.Run("successful async response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/Measure/$evaluate-measure":
				w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
				w.WriteHeader(http.StatusAccepted)
			case "/async-poll":
				response := fm.Bundle{
					Entry: []fm.BundleEntry{{
						Resource: []byte{},
					}},
				}

				w.WriteHeader(http.StatusOK)
				encoder := json.NewEncoder(w)
				if err := encoder.Encode(response); err != nil {
					t.Error(err)
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}

		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		measureReport, err := evaluateMeasure(client, "foo")

		assert.Equal(t, 0, len(measureReport))
		assert.Nil(t, err)
	})
}
