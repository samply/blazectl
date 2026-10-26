package cmd

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

	t.Run("without version", func(t *testing.T) {
		m := data.Measure{
			Group: []data.Group{{Population: []data.Population{{Expression: "InInitialPopulation"}}}},
		}

		resource, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err != nil {
			t.Fatalf("error while generating the measure resource: %v", err)
		}

		assert.Nil(t, resource.Version)
	})

	t.Run("with version", func(t *testing.T) {
		m := data.Measure{
			Version: "1.0.0",
			Group:   []data.Group{{Population: []data.Population{{Expression: "InInitialPopulation"}}}},
		}

		resource, err := CreateMeasureResource(m, measureUrl, libraryUrl)
		if err != nil {
			t.Fatalf("error while generating the measure resource: %v", err)
		}

		assert.Equal(t, "1.0.0", *resource.Version)
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
		assert.Equal(t, "all", *resource.Name)
		assert.Nil(t, resource.Version)
	})

	t.Run("with name and version", func(t *testing.T) {
		m := data.Measure{
			Library: writeFile(t, "library.cql", "library \"foo\" version '1.0.0'\n"),
		}

		resource, err := CreateLibraryResource(m, libraryUrl)
		if err != nil {
			t.Fatalf("error while generating the library resource: %v", err)
		}

		assert.Equal(t, "foo", *resource.Name)
		assert.Equal(t, "1.0.0", *resource.Version)
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

func TestEvaluateMeasureParameterFlagUsage(t *testing.T) {
	flag := evaluateMeasureCmd.Flags().Lookup("parameter")
	assert.NotNil(t, flag)
	assert.Contains(t, flag.Usage, supportedParameterTypes)
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

	t.Run("missing name with type", func(t *testing.T) {
		_, err := parseParameterOverrides([]string{":integer=1"})
		assert.Error(t, err)
	})

	t.Run("keeps an equals sign in the value", func(t *testing.T) {
		overrides, err := parseParameterOverrides([]string{"Foo=a=b"})
		assert.Nil(t, err)
		assert.Equal(t, []parsedParameter{{Name: "Foo", Values: []string{"a=b"}}}, overrides)
	})

	t.Run("repeated name becomes a list", func(t *testing.T) {
		overrides, err := parseParameterOverrides([]string{"Foo=1", "Foo=2"})
		assert.Nil(t, err)
		assert.Equal(t, []parsedParameter{{Name: "Foo", Values: []string{"1", "2"}}}, overrides)
	})

	t.Run("parses an explicit type", func(t *testing.T) {
		overrides, err := parseParameterOverrides([]string{"MinAge:integer=18"})
		assert.Nil(t, err)
		assert.Equal(t, []parsedParameter{{Name: "MinAge", Type: "integer", Values: []string{"18"}}}, overrides)
	})

	t.Run("leaves the type empty when omitted", func(t *testing.T) {
		overrides, err := parseParameterOverrides([]string{"Gender=male"})
		assert.Nil(t, err)
		assert.Equal(t, "", overrides[0].Type)
	})

	t.Run("keeps a colon in the value", func(t *testing.T) {
		overrides, err := parseParameterOverrides([]string{"Start:dateTime=2020-01-01T12:00:00Z"})
		assert.Nil(t, err)
		assert.Equal(t, []parsedParameter{{Name: "Start", Type: "dateTime", Values: []string{"2020-01-01T12:00:00Z"}}}, overrides)
	})

	t.Run("repeated name keeps the type given once", func(t *testing.T) {
		overrides, err := parseParameterOverrides([]string{"Foo:integer=1", "Foo=2"})
		assert.Nil(t, err)
		assert.Equal(t, []parsedParameter{{Name: "Foo", Type: "integer", Values: []string{"1", "2"}}}, overrides)
	})

	t.Run("conflicting types are an error", func(t *testing.T) {
		_, err := parseParameterOverrides([]string{"Foo:integer=1", "Foo:string=2"})
		assert.Error(t, err)
	})
}

func TestBuildMeasureParameters(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		parameters, err := buildMeasureParameters(nil)
		assert.Nil(t, err)
		assert.Empty(t, parameters)
	})

	t.Run("a parameter without a type defaults to string", func(t *testing.T) {
		parameters, err := buildMeasureParameters([]parsedParameter{{Name: "Gender", Values: []string{"male"}}})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(parameters))
		assert.Equal(t, "Gender", parameters[0].Name)
		assert.Equal(t, "male", *parameters[0].ValueString)
	})

	t.Run("a parameter uses its explicit type", func(t *testing.T) {
		parameters, err := buildMeasureParameters([]parsedParameter{{Name: "MinAge", Type: "integer", Values: []string{"18"}}})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(parameters))
		assert.Equal(t, 18, *parameters[0].ValueInteger)
	})

	t.Run("multiple values become a list", func(t *testing.T) {
		parameters, err := buildMeasureParameters([]parsedParameter{{Name: "Codes", Type: "code", Values: []string{"a", "b"}}})
		assert.Nil(t, err)
		assert.Equal(t, 2, len(parameters))
		assert.Equal(t, "a", *parameters[0].ValueCode)
		assert.Equal(t, "b", *parameters[1].ValueCode)
	})

	t.Run("an invalid value for the type is an error", func(t *testing.T) {
		_, err := buildMeasureParameters([]parsedParameter{{Name: "MinAge", Type: "integer", Values: []string{"abc"}}})
		assert.Error(t, err)
	})

	t.Run("parameters keep their order", func(t *testing.T) {
		parameters, err := buildMeasureParameters([]parsedParameter{
			{Name: "MinAge", Type: "integer", Values: []string{"18"}},
			{Name: "Gender", Values: []string{"male"}},
		})
		assert.Nil(t, err)
		assert.Equal(t, 2, len(parameters))
		assert.Equal(t, "MinAge", parameters[0].Name)
		assert.Equal(t, "Gender", parameters[1].Name)
	})
}

func TestEvaluateMeasureParameters(t *testing.T) {
	t.Run("without CQL parameters", func(t *testing.T) {
		params, err := evaluateMeasureParameters(measureRef{url: "measure-url"}, nil)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(params.Parameter))
		assert.Equal(t, "measure", params.Parameter[0].Name)
		assert.Equal(t, "measure-url", *params.Parameter[0].ValueString)
		assert.Equal(t, "periodStart", params.Parameter[1].Name)
		assert.Equal(t, "1900", *params.Parameter[1].ValueDate)
		assert.Equal(t, "periodEnd", params.Parameter[2].Name)
		assert.Equal(t, "2200", *params.Parameter[2].ValueDate)
	})

	t.Run("a measure referenced by ID omits the measure parameter", func(t *testing.T) {
		params, err := evaluateMeasureParameters(measureRef{id: "some-id"}, nil)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(params.Parameter))
		assert.Equal(t, "periodStart", params.Parameter[0].Name)
		assert.Equal(t, "1900", *params.Parameter[0].ValueDate)
		assert.Equal(t, "periodEnd", params.Parameter[1].Name)
		assert.Equal(t, "2200", *params.Parameter[1].ValueDate)
	})

	t.Run("with CQL parameters as a nested Parameters resource", func(t *testing.T) {
		params, err := evaluateMeasureParameters(measureRef{url: "measure-url"},
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

func TestMeasureRefDescription(t *testing.T) {
	t.Run("by canonical URL", func(t *testing.T) {
		assert.Equal(t, "canonical URL measure-url", measureRef{url: "measure-url"}.description())
	})

	t.Run("by ID", func(t *testing.T) {
		assert.Equal(t, "ID some-id", measureRef{id: "some-id"}.description())
	})
}

func TestValidateMeasureSource(t *testing.T) {
	t.Run("no measure given", func(t *testing.T) {
		err := validateMeasureSource(nil, "", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires a measure-file argument or one of the --measure-url or --measure-id flags")
	})

	t.Run("measure file only", func(t *testing.T) {
		assert.Nil(t, validateMeasureSource([]string{"measure.yml"}, "", ""))
	})

	t.Run("measure URL only", func(t *testing.T) {
		assert.Nil(t, validateMeasureSource(nil, "measure-url", ""))
	})

	t.Run("measure ID only", func(t *testing.T) {
		assert.Nil(t, validateMeasureSource(nil, "", "some-id"))
	})

	t.Run("measure file and URL", func(t *testing.T) {
		err := validateMeasureSource([]string{"measure.yml"}, "measure-url", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("measure file and ID", func(t *testing.T) {
		err := validateMeasureSource([]string{"measure.yml"}, "", "some-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("measure URL and ID", func(t *testing.T) {
		err := validateMeasureSource(nil, "measure-url", "some-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})
}

func TestEvaluateMeasure(t *testing.T) {

	t.Run("Request to FHIR server fails", func(t *testing.T) {
		baseURL, _ := url.ParseRequestURI("http://localhost")
		client := fhir.NewClient(*baseURL, nil)

		_, err := evaluateMeasure(client, measureRef{url: "foo"})

		assert.Error(t, err)
	})

	t.Run("Successful return empty body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simply do not respond with anything
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		measureReport, _ := evaluateMeasure(client, measureRef{url: "foo"})

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

		measureReport, err := evaluateMeasure(client, measureRef{url: "foo"})

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

		measureReport, err := evaluateMeasure(client, measureRef{url: "foo"})

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

		measureReport, err := evaluateMeasure(client, measureRef{url: "foo"})

		assert.Nil(t, err)
		assert.Equal(t, `{"resourceType":"MeasureReport","id":"123"}`, string(measureReport))
	})

	t.Run("a measure referenced by ID uses the instance-level operation", func(t *testing.T) {
		var capturedPath string
		var capturedQuery url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			capturedQuery = r.URL.Query()
			w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"resourceType":"MeasureReport"}`)); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		measureReport, err := evaluateMeasure(client, measureRef{id: "some-id"})

		assert.Nil(t, err)
		assert.Equal(t, `{"resourceType":"MeasureReport"}`, string(measureReport))
		assert.Equal(t, "/Measure/some-id/$evaluate-measure", capturedPath)
		assert.Empty(t, capturedQuery.Get("measure"))
		assert.Equal(t, "1900", capturedQuery.Get("periodStart"))
		assert.Equal(t, "2200", capturedQuery.Get("periodEnd"))
	})

	t.Run("with CQL parameters a measure referenced by ID is posted without a measure parameter", func(t *testing.T) {
		var capturedMethod string
		var capturedPath string
		var capturedBody []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			capturedPath = r.URL.Path
			capturedBody, _ = io.ReadAll(r.Body)
			w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		measureParameters = []fm.ParametersParameter{{Name: "Foo", ValueString: new("bar")}}
		defer func() { measureParameters = nil }()

		_, err := evaluateMeasure(client, measureRef{id: "some-id"})
		assert.Nil(t, err)
		assert.Equal(t, "POST", capturedMethod)
		assert.Equal(t, "/Measure/some-id/$evaluate-measure", capturedPath)

		params, err := fm.UnmarshalParameters(capturedBody)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(params.Parameter))
		assert.Equal(t, "periodStart", params.Parameter[0].Name)
		assert.Equal(t, "periodEnd", params.Parameter[1].Name)
		assert.Equal(t, "parameters", params.Parameter[2].Name)
	})

	t.Run("async error response with a measure referenced by ID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/Measure/some-id/$evaluate-measure":
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

		_, err := evaluateMeasure(client, measureRef{id: "some-id"})

		assert.Equal(t, "Error while evaluating the measure with ID some-id:\n\nnon FHIR response", err.Error())
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

		_, err := evaluateMeasure(client, measureRef{url: "measure-url"})
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

		_, err := evaluateMeasure(client, measureRef{url: "foo"})

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

		_, err := evaluateMeasure(client, measureRef{url: "foo"})

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

		measureReport, err := evaluateMeasureWithRetry(client, measureRef{url: "foo"})

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

		_, err := evaluateMeasureWithRetry(client, measureRef{url: "foo"})

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

		_, err := evaluateMeasure(client, measureRef{url: "foo"})

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

			measureReport, err := evaluateMeasure(client, measureRef{url: "foo"})

			assert.Equal(t, 0, len(measureReport))
			assert.Nil(t, err)
		})
	}
}

func TestParseCqlLibraryIdentifier(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		name, version := parseCqlLibraryIdentifier([]byte(""))
		assert.Equal(t, "", name)
		assert.Equal(t, "", version)
	})

	t.Run("quoted name without version", func(t *testing.T) {
		name, version := parseCqlLibraryIdentifier([]byte("library \"all\"\nusing FHIR version '4.0.0'\n"))
		assert.Equal(t, "all", name)
		assert.Equal(t, "", version)
	})

	t.Run("quoted name with version", func(t *testing.T) {
		name, version := parseCqlLibraryIdentifier([]byte("library \"inpatient-stress\" version '1.0.0'\nusing FHIR version '4.0.0'\n"))
		assert.Equal(t, "inpatient-stress", name)
		assert.Equal(t, "1.0.0", version)
	})

	t.Run("unquoted name with version", func(t *testing.T) {
		name, version := parseCqlLibraryIdentifier([]byte("library Retrieve version '2'\n"))
		assert.Equal(t, "Retrieve", name)
		assert.Equal(t, "2", version)
	})

	t.Run("preceded by comments", func(t *testing.T) {
		name, version := parseCqlLibraryIdentifier([]byte("// a comment\n/* another\ncomment */\nlibrary Retrieve version '2'\n"))
		assert.Equal(t, "Retrieve", name)
		assert.Equal(t, "2", version)
	})

	t.Run("using version isn't a library version", func(t *testing.T) {
		name, version := parseCqlLibraryIdentifier([]byte("library Retrieve\nusing FHIR version '4.0.0'\n"))
		assert.Equal(t, "Retrieve", name)
		assert.Equal(t, "", version)
	})
}

func TestReadMeasureFile(t *testing.T) {
	t.Run("with url and version", func(t *testing.T) {
		filename := writeFile(t, "measure.yml", "url: https://example.com/Measure/foo\nversion: 1.0.0\nlibrary: foo.cql\n")

		m, err := readMeasureFile(filename)
		if err != nil {
			t.Fatalf("error while reading the measure file: %v", err)
		}

		assert.Equal(t, "https://example.com/Measure/foo", m.Url)
		assert.Equal(t, "1.0.0", m.Version)
		assert.Equal(t, "foo.cql", m.Library)
	})
}

// writeFile writes the content to a file with the given name in a temporary
// directory and returns the path of the file.
func writeFile(t *testing.T, name string, content string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		t.Fatalf("error while writing the file %s: %v", filename, err)
	}
	return filename
}

// writeMeasureFile writes a CQL library file with the given content and a
// measure file referencing it with the given additional YAML properties and
// returns the path of the measure file.
func writeMeasureFile(t *testing.T, cql string, properties string) string {
	t.Helper()
	libraryFilename := writeFile(t, "library.cql", cql)
	return writeFile(t, "measure.yml", properties+"library: "+libraryFilename+"\n"+
		"group:\n- type: Patient\n  population:\n  - expression: InInitialPopulation\n")
}

// fakeMeasureServer is a FHIR server answering Library and Measure searches
// with the configured resources and recording the queries and the transaction
// bundles posted to it.
type fakeMeasureServer struct {
	libraries      []any
	measures       []any
	libraryQueries []url.Values
	measureQueries []url.Values
	transactions   []fm.Bundle
}

func (s *fakeMeasureServer) start(t *testing.T) *fhir.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/Library":
			s.libraryQueries = append(s.libraryQueries, r.URL.Query())
			writeSearchSet(t, w, s.libraries)
		case r.Method == http.MethodGet && r.URL.Path == "/Measure":
			s.measureQueries = append(s.measureQueries, r.URL.Query())
			writeSearchSet(t, w, s.measures)
		case r.Method == http.MethodPost && r.URL.Path == "/":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			bundle, err := fm.UnmarshalBundle(body)
			if err != nil {
				t.Error(err)
			}
			s.transactions = append(s.transactions, bundle)
			if _, err := w.Write([]byte(`{"resourceType":"Bundle","type":"transaction-response"}`)); err != nil {
				t.Error(err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	baseURL, _ := url.ParseRequestURI(server.URL)
	return fhir.NewClient(*baseURL, nil)
}

func writeSearchSet(t *testing.T, w http.ResponseWriter, resources []any) {
	bundle := fm.Bundle{Type: fm.BundleTypeSearchset, Total: new(len(resources))}
	for _, resource := range resources {
		resourceBytes, err := json.Marshal(resource)
		if err != nil {
			t.Error(err)
		}
		bundle.Entry = append(bundle.Entry, fm.BundleEntry{Resource: resourceBytes})
	}
	if err := json.NewEncoder(w).Encode(bundle); err != nil {
		t.Error(err)
	}
}

// transactionResources returns the resources of the single recorded
// transaction bundle, keyed by resource type.
func transactionResources(t *testing.T, s *fakeMeasureServer) (*fm.Library, *fm.Measure) {
	t.Helper()
	if !assert.Len(t, s.transactions, 1) {
		t.FailNow()
	}
	var library *fm.Library
	var measure *fm.Measure
	for _, entry := range s.transactions[0].Entry {
		assert.Equal(t, fm.HTTPVerbPOST, entry.Request.Method)
		switch entry.Request.Url {
		case "Library":
			l, err := fm.UnmarshalLibrary(entry.Resource)
			if err != nil {
				t.Fatal(err)
			}
			library = &l
		case "Measure":
			m, err := fm.UnmarshalMeasure(entry.Resource)
			if err != nil {
				t.Fatal(err)
			}
			measure = &m
		}
	}
	return library, measure
}

// existingLibrary returns a Library resource as it would be stored on the
// server, with the given name, version, canonical URL and CQL content.
func existingLibrary(name string, version string, libraryUrl string, cql string) fm.Library {
	return fm.Library{
		Id:      new("0"),
		Meta:    &fm.Meta{VersionId: new("1")},
		Url:     new(libraryUrl),
		Name:    new(name),
		Version: new(version),
		Status:  fm.PublicationStatusActive,
		Content: []fm.Attachment{createAttachment("text/cql", base64.StdEncoding.EncodeToString([]byte(cql)))},
	}
}

// existingMeasure returns the Measure resource generated from the measure file
// as it would be stored on the server.
func existingMeasure(t *testing.T, measureFile string, measureUrl string, libraryUrl string) fm.Measure {
	t.Helper()
	m, err := readMeasureFile(measureFile)
	if err != nil {
		t.Fatal(err)
	}
	measure, err := CreateMeasureResource(*m, measureUrl, libraryUrl)
	if err != nil {
		t.Fatal(err)
	}
	measure.Id = new("0")
	measure.Meta = &fm.Meta{VersionId: new("1"), LastUpdated: new("2026-10-26T00:00:00Z")}
	return *measure
}

func TestCreateMeasureResources(t *testing.T) {
	const versionedCql = "library \"foo\" version '1.0.0'\n"

	t.Run("a library without version is created with a random URL", func(t *testing.T) {
		s := &fakeMeasureServer{}
		client := s.start(t)
		measureFile := writeMeasureFile(t, "library \"foo\"\n", "")

		ref, err := createMeasureResources(client, measureFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Empty(t, s.libraryQueries)
		assert.Empty(t, s.measureQueries)
		library, measure := transactionResources(t, s)
		assert.NotNil(t, library)
		assert.NotNil(t, measure)
		assert.True(t, strings.HasPrefix(*library.Url, "urn:uuid:"))
		assert.True(t, strings.HasPrefix(*measure.Url, "urn:uuid:"))
		assert.Equal(t, []string{*library.Url}, measure.Library)
		assert.Equal(t, measureRef{url: *measure.Url}, ref)
	})

	t.Run("a versioned library not existing on the server is created", func(t *testing.T) {
		s := &fakeMeasureServer{}
		client := s.start(t)
		measureFile := writeMeasureFile(t, versionedCql, "")

		_, err := createMeasureResources(client, measureFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if assert.Len(t, s.libraryQueries, 1) {
			assert.Equal(t, "foo", s.libraryQueries[0].Get("name"))
			assert.Equal(t, "1.0.0", s.libraryQueries[0].Get("version"))
		}
		library, measure := transactionResources(t, s)
		assert.Equal(t, "foo", *library.Name)
		assert.Equal(t, "1.0.0", *library.Version)
		assert.True(t, strings.HasPrefix(*library.Url, "urn:uuid:"))
		assert.Equal(t, []string{*library.Url + "|1.0.0"}, measure.Library)
	})

	t.Run("a versioned library existing on the server with the same content is reused", func(t *testing.T) {
		s := &fakeMeasureServer{
			libraries: []any{existingLibrary("foo", "1.0.0", "urn:uuid:existing-library", versionedCql)},
		}
		client := s.start(t)
		measureFile := writeMeasureFile(t, versionedCql, "")

		_, err := createMeasureResources(client, measureFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		library, measure := transactionResources(t, s)
		assert.Nil(t, library)
		assert.Equal(t, []string{"urn:uuid:existing-library|1.0.0"}, measure.Library)
	})

	t.Run("a library with a name only matching by prefix isn't reused", func(t *testing.T) {
		s := &fakeMeasureServer{
			libraries: []any{existingLibrary("foo-bar", "1.0.0", "urn:uuid:existing-library", versionedCql)},
		}
		client := s.start(t)
		measureFile := writeMeasureFile(t, versionedCql, "")

		_, err := createMeasureResources(client, measureFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		library, _ := transactionResources(t, s)
		assert.NotNil(t, library)
	})

	t.Run("a versioned library existing on the server with different content is an error", func(t *testing.T) {
		s := &fakeMeasureServer{
			libraries: []any{existingLibrary("foo", "1.0.0", "urn:uuid:existing-library", versionedCql+"define Foo: true\n")},
		}
		client := s.start(t)
		measureFile := writeMeasureFile(t, versionedCql, "")

		_, err := createMeasureResources(client, measureFile)

		assert.EqualError(t, err, "the Library with name `foo` and version `1.0.0` already exists on the server but its content differs from the CQL library file")
		assert.Empty(t, s.transactions)
	})

	t.Run("multiple versioned libraries existing on the server is an error", func(t *testing.T) {
		s := &fakeMeasureServer{
			libraries: []any{
				existingLibrary("foo", "1.0.0", "urn:uuid:existing-library-1", versionedCql),
				existingLibrary("foo", "1.0.0", "urn:uuid:existing-library-2", versionedCql),
			},
		}
		client := s.start(t)
		measureFile := writeMeasureFile(t, versionedCql, "")

		_, err := createMeasureResources(client, measureFile)

		assert.EqualError(t, err, "found 2 Library resources with name `foo` and version `1.0.0` on the server, expected at most one")
		assert.Empty(t, s.transactions)
	})

	t.Run("a measure with URL not existing on the server is created", func(t *testing.T) {
		s := &fakeMeasureServer{}
		client := s.start(t)
		measureFile := writeMeasureFile(t, versionedCql, "url: https://example.com/Measure/foo\nversion: 2.0.0\n")

		ref, err := createMeasureResources(client, measureFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if assert.Len(t, s.measureQueries, 1) {
			assert.Equal(t, "https://example.com/Measure/foo", s.measureQueries[0].Get("url"))
			assert.Equal(t, "2.0.0", s.measureQueries[0].Get("version"))
		}
		_, measure := transactionResources(t, s)
		assert.Equal(t, "https://example.com/Measure/foo", *measure.Url)
		assert.Equal(t, "2.0.0", *measure.Version)
		assert.Equal(t, measureRef{url: "https://example.com/Measure/foo|2.0.0"}, ref)
	})

	t.Run("a measure with URL but without version is searched by URL only", func(t *testing.T) {
		s := &fakeMeasureServer{}
		client := s.start(t)
		measureFile := writeMeasureFile(t, versionedCql, "url: https://example.com/Measure/foo\n")

		ref, err := createMeasureResources(client, measureFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if assert.Len(t, s.measureQueries, 1) {
			assert.Equal(t, "https://example.com/Measure/foo", s.measureQueries[0].Get("url"))
			assert.False(t, s.measureQueries[0].Has("version"))
		}
		assert.Equal(t, measureRef{url: "https://example.com/Measure/foo"}, ref)
	})

	t.Run("a measure and library existing on the server with the same content are reused", func(t *testing.T) {
		measureFile := writeMeasureFile(t, versionedCql, "url: https://example.com/Measure/foo\nversion: 2.0.0\n")
		s := &fakeMeasureServer{
			libraries: []any{existingLibrary("foo", "1.0.0", "urn:uuid:existing-library", versionedCql)},
			measures:  []any{existingMeasure(t, measureFile, "https://example.com/Measure/foo", "urn:uuid:existing-library|1.0.0")},
		}
		client := s.start(t)

		ref, err := createMeasureResources(client, measureFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Empty(t, s.transactions)
		assert.Equal(t, measureRef{url: "https://example.com/Measure/foo|2.0.0"}, ref)
	})

	t.Run("a measure existing on the server with different content is an error", func(t *testing.T) {
		measureFile := writeMeasureFile(t, versionedCql, "url: https://example.com/Measure/foo\nversion: 2.0.0\n")
		s := &fakeMeasureServer{
			libraries: []any{existingLibrary("foo", "1.0.0", "urn:uuid:existing-library", versionedCql)},
			measures:  []any{existingMeasure(t, measureFile, "https://example.com/Measure/foo", "urn:uuid:other-library")},
		}
		client := s.start(t)

		_, err := createMeasureResources(client, measureFile)

		assert.EqualError(t, err, "the Measure with canonical URL `https://example.com/Measure/foo` and version `2.0.0` already exists on the server but its content differs from the measure file")
		assert.Empty(t, s.transactions)
	})

	t.Run("multiple measures existing on the server is an error", func(t *testing.T) {
		measureFile := writeMeasureFile(t, versionedCql, "url: https://example.com/Measure/foo\n")
		measure := existingMeasure(t, measureFile, "https://example.com/Measure/foo", "urn:uuid:existing-library")
		s := &fakeMeasureServer{
			libraries: []any{existingLibrary("foo", "1.0.0", "urn:uuid:existing-library", versionedCql)},
			measures:  []any{measure, measure},
		}
		client := s.start(t)

		_, err := createMeasureResources(client, measureFile)

		assert.EqualError(t, err, "found 2 Measure resources with canonical URL `https://example.com/Measure/foo` on the server, expected at most one")
		assert.Empty(t, s.transactions)
	})
}
