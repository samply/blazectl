package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/samply/blazectl/data"
	"github.com/samply/blazectl/fhir"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
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

	t.Run("with one group with code and one population", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{
					Code: "observation",
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
		assert.Equal(t, "observation", *resource.Group[0].Code.Text)
	})

	t.Run("with one group with description and one population", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{
				{
					Description: "all the observations",
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
		assert.Equal(t, "all the observations", *resource.Group[0].Description)
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

	t.Run("with one group and one population and one stratifier with description", func(t *testing.T) {
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
							Code:        "foo",
							Description: "the foo stratifier",
							Expression:  "Foo",
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
		assert.Equal(t, "the foo stratifier", *resource.Group[0].Stratifier[0].Description)
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

func TestBuildMeasureParameter(t *testing.T) {
	t.Run("unsupported type", func(t *testing.T) {
		_, err := buildMeasureParameter("Foo", "quantity", "1")
		assert.Error(t, err)
	})

	t.Run("string", func(t *testing.T) {
		p, err := buildMeasureParameter("Foo", "string", "bar")
		assert.Nil(t, err)
		assert.Equal(t, "Foo", p.Name)
		assert.Equal(t, "bar", *p.ValueString)
	})

	t.Run("code", func(t *testing.T) {
		p, err := buildMeasureParameter("Foo", "code", "active")
		assert.Nil(t, err)
		assert.Equal(t, "active", *p.ValueCode)
	})

	t.Run("date", func(t *testing.T) {
		p, err := buildMeasureParameter("Foo", "date", "2020-01-01")
		assert.Nil(t, err)
		assert.Equal(t, "2020-01-01", *p.ValueDate)
	})

	t.Run("dateTime", func(t *testing.T) {
		p, err := buildMeasureParameter("Foo", "dateTime", "2020-01-01T12:00:00Z")
		assert.Nil(t, err)
		assert.Equal(t, "2020-01-01T12:00:00Z", *p.ValueDateTime)
	})

	t.Run("boolean", func(t *testing.T) {
		p, err := buildMeasureParameter("Foo", "boolean", "true")
		assert.Nil(t, err)
		assert.Equal(t, true, *p.ValueBoolean)
	})

	t.Run("invalid boolean", func(t *testing.T) {
		_, err := buildMeasureParameter("Foo", "boolean", "yes")
		assert.Error(t, err)
	})

	t.Run("integer", func(t *testing.T) {
		p, err := buildMeasureParameter("Foo", "integer", "42")
		assert.Nil(t, err)
		assert.Equal(t, 42, *p.ValueInteger)
	})

	t.Run("invalid integer", func(t *testing.T) {
		_, err := buildMeasureParameter("Foo", "integer", "1.5")
		assert.Error(t, err)
	})

	t.Run("decimal", func(t *testing.T) {
		p, err := buildMeasureParameter("Foo", "decimal", "1.5")
		assert.Nil(t, err)
		assert.Equal(t, json.Number("1.5"), *p.ValueDecimal)
	})

	t.Run("invalid decimal", func(t *testing.T) {
		_, err := buildMeasureParameter("Foo", "decimal", "abc")
		assert.Error(t, err)
	})
}

func TestParseParameterOverrides(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		overrides, err := parseParameterOverrides(nil)
		assert.Nil(t, err)
		assert.Empty(t, overrides)
	})

	t.Run("missing equals sign", func(t *testing.T) {
		_, err := parseParameterOverrides([]string{"foo"})
		assert.Error(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		_, err := parseParameterOverrides([]string{"=bar"})
		assert.Error(t, err)
	})

	t.Run("keeps an equals sign in the value", func(t *testing.T) {
		overrides, err := parseParameterOverrides([]string{"Foo=a=b"})
		assert.Nil(t, err)
		assert.Equal(t, []string{"a=b"}, overrides["Foo"])
	})

	t.Run("repeated name becomes a list", func(t *testing.T) {
		overrides, err := parseParameterOverrides([]string{"Foo=1", "Foo=2"})
		assert.Nil(t, err)
		assert.Equal(t, []string{"1", "2"}, overrides["Foo"])
	})
}

func TestBuildMeasureParameters(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		parameters, err := buildMeasureParameters(nil, nil)
		assert.Nil(t, err)
		assert.Empty(t, parameters)
	})

	t.Run("missing parameter name", func(t *testing.T) {
		declared := []data.Parameter{{Type: "integer", Value: data.ParameterValue{Values: []string{"1"}}}}
		_, err := buildMeasureParameters(declared, nil)
		assert.Error(t, err)
	})

	t.Run("uses the declared value", func(t *testing.T) {
		declared := []data.Parameter{{Name: "MinAge", Type: "integer", Value: data.ParameterValue{Values: []string{"18"}}}}
		parameters, err := buildMeasureParameters(declared, nil)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(parameters))
		assert.Equal(t, "MinAge", parameters[0].Name)
		assert.Equal(t, 18, *parameters[0].ValueInteger)
	})

	t.Run("a declared sequence becomes a list", func(t *testing.T) {
		declared := []data.Parameter{{Name: "Codes", Type: "code", Value: data.ParameterValue{Values: []string{"a", "b"}}}}
		parameters, err := buildMeasureParameters(declared, nil)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(parameters))
		assert.Equal(t, "a", *parameters[0].ValueCode)
		assert.Equal(t, "b", *parameters[1].ValueCode)
	})

	t.Run("a parameter without a value contributes nothing", func(t *testing.T) {
		declared := []data.Parameter{{Name: "MinAge", Type: "integer"}}
		parameters, err := buildMeasureParameters(declared, nil)
		assert.Nil(t, err)
		assert.Empty(t, parameters)
	})

	t.Run("a command line override replaces the declared value", func(t *testing.T) {
		declared := []data.Parameter{{Name: "MinAge", Type: "integer", Value: data.ParameterValue{Values: []string{"18"}}}}
		parameters, err := buildMeasureParameters(declared, map[string][]string{"MinAge": {"21"}})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(parameters))
		assert.Equal(t, 21, *parameters[0].ValueInteger)
	})

	t.Run("an override uses the declared type", func(t *testing.T) {
		declared := []data.Parameter{{Name: "MinAge", Type: "integer"}}
		_, err := buildMeasureParameters(declared, map[string][]string{"MinAge": {"abc"}})
		assert.Error(t, err)
	})

	t.Run("overriding an undeclared parameter is an error", func(t *testing.T) {
		declared := []data.Parameter{{Name: "MinAge", Type: "integer"}}
		_, err := buildMeasureParameters(declared, map[string][]string{"MaxAge": {"99"}})
		assert.Error(t, err)
	})
}

func TestEvaluateMeasureParameters(t *testing.T) {
	t.Run("without CQL parameters", func(t *testing.T) {
		params, err := evaluateMeasureParameters("measure-url", nil)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(params.Parameter))
		assert.Equal(t, "measure", params.Parameter[0].Name)
		assert.Equal(t, "measure-url", *params.Parameter[0].ValueString)
		assert.Equal(t, "periodStart", params.Parameter[1].Name)
		assert.Equal(t, "1900", *params.Parameter[1].ValueDate)
		assert.Equal(t, "periodEnd", params.Parameter[2].Name)
		assert.Equal(t, "2200", *params.Parameter[2].ValueDate)
	})

	t.Run("with CQL parameters as a nested Parameters resource", func(t *testing.T) {
		params, err := evaluateMeasureParameters("measure-url",
			[]fm.ParametersParameter{{Name: "Foo", ValueString: new("bar")}})
		assert.Nil(t, err)
		assert.Equal(t, 4, len(params.Parameter))
		assert.Equal(t, "parameters", params.Parameter[3].Name)

		nested, err := fm.UnmarshalParameters(params.Parameter[3].Resource)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(nested.Parameter))
		assert.Equal(t, "Foo", nested.Parameter[0].Name)
		assert.Equal(t, "bar", *nested.Parameter[0].ValueString)
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

	t.Run("Created (201) response returns the report", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
			w.WriteHeader(http.StatusCreated)
			if _, err := w.Write([]byte(`{"resourceType":"MeasureReport"}`)); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		measureReport, err := evaluateMeasure(client, "foo")

		assert.Nil(t, err)
		assert.Equal(t, `{"resourceType":"MeasureReport"}`, string(measureReport))
	})

	t.Run("async response with 201 Created status returns the report", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/Measure/$evaluate-measure":
				w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
				w.WriteHeader(http.StatusAccepted)
			case "/async-poll":
				response := fm.Bundle{
					Type: fm.BundleTypeBatchResponse,
					Entry: []fm.BundleEntry{{
						Response: &fm.BundleEntryResponse{
							Status: "201 Created",
						},
						Resource: []byte(`{"resourceType":"MeasureReport"}`),
					}},
				}

				w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
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

		assert.Nil(t, err)
		assert.Equal(t, `{"resourceType":"MeasureReport"}`, string(measureReport))
	})

	t.Run("async response with only a location fetches the report", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/Measure/$evaluate-measure":
				w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
				w.WriteHeader(http.StatusAccepted)
			case "/async-poll":
				response := fm.Bundle{
					Type: fm.BundleTypeBatchResponse,
					Entry: []fm.BundleEntry{{
						Response: &fm.BundleEntryResponse{
							Status:   "201",
							Location: new(fmt.Sprintf("http://%s/MeasureReport/123", r.Host)),
						},
					}},
				}

				w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Error(err)
				}
			case "/MeasureReport/123":
				w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(`{"resourceType":"MeasureReport","id":"123"}`)); err != nil {
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

		assert.Nil(t, err)
		assert.Equal(t, `{"resourceType":"MeasureReport","id":"123"}`, string(measureReport))
	})

	t.Run("with CQL parameters the request is posted", func(t *testing.T) {
		var capturedMethod string
		var capturedBody []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			capturedBody, _ = io.ReadAll(r.Body)
			w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		measureParameters = []fm.ParametersParameter{{Name: "Foo", ValueString: new("bar")}}
		defer func() { measureParameters = nil }()

		_, err := evaluateMeasure(client, "measure-url")
		assert.Nil(t, err)
		assert.Equal(t, "POST", capturedMethod)

		params, err := fm.UnmarshalParameters(capturedBody)
		assert.Nil(t, err)
		assert.Equal(t, 4, len(params.Parameter))
		assert.Equal(t, "parameters", params.Parameter[3].Name)
	})

	t.Run("missing parameter error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := fm.OperationOutcome{
				Issue: []fm.OperationOutcomeIssue{{
					Severity: fm.IssueSeverityError,
					Code:     fm.IssueTypeValue,
				}},
			}

			w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
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

			w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
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

				w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
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

			w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
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

	t.Run("async response with non FHIR response", func(t *testing.T) {
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

		assert.Equal(t, "Error while evaluating the measure with canonical URL foo:\n\nnon FHIR response", err.Error())
	})

	for _, numPolls := range []int{0, 1, 2, 5} {
		t.Run(fmt.Sprintf("successful async response after %d retries", numPolls), func(t *testing.T) {
			poll := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/Measure/$evaluate-measure":
					w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
					w.WriteHeader(http.StatusAccepted)
				case "/async-poll":
					if poll < numPolls {
						w.WriteHeader(http.StatusAccepted)
					} else {
						response := fm.Bundle{
							Type: fm.BundleTypeBatchResponse,
							Entry: []fm.BundleEntry{{
								Response: &fm.BundleEntryResponse{
									Status: "200 OK",
								},
								Resource: []byte{},
							}},
						}

						w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
						w.WriteHeader(http.StatusOK)
						encoder := json.NewEncoder(w)
						if err := encoder.Encode(response); err != nil {
							t.Error(err)
						}
					}
					poll++
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
}
