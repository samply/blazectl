package cmd

import (
	"github.com/samply/blazectl/data"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
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
