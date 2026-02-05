package cmd

import (
	"bytes"
	"testing"

	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
)

func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func TestRenderReport(t *testing.T) {
	report := fm.MeasureReport{
		Group: []fm.MeasureReportGroup{
			{
				Code: &fm.CodeableConcept{
					Text: stringPtr("Main Group"),
				},
				Population: []fm.MeasureReportGroupPopulation{
					{
						Count: intPtr(100),
					},
				},
				Stratifier: []fm.MeasureReportGroupStratifier{
					{
						Code: []fm.CodeableConcept{
							{
								Text: stringPtr("Gender"),
							},
						},
						Stratum: []fm.MeasureReportGroupStratifierStratum{
							{
								Value: &fm.CodeableConcept{
									Text: stringPtr("male"),
								},
								Population: []fm.MeasureReportGroupStratifierStratumPopulation{
									{
										Count: intPtr(45),
									},
								},
							},
							{
								Value: &fm.CodeableConcept{
									Coding: []fm.Coding{
										createCoding("http://hl7.org/fhir/administrative-gender", "female"),
									},
								},
								Population: []fm.MeasureReportGroupStratifierStratumPopulation{
									{
										Count: intPtr(55),
									},
								},
							},
						},
					},
				},
			},
			{
				// No code, should show "Group"
				Population: []fm.MeasureReportGroupPopulation{
					{
						Count: intPtr(10),
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := renderReport(&buf, report)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	assert.Contains(t, output, "MeasureReport")
	assert.Contains(t, output, "Main Group")
	assert.Contains(t, output, "Gender")
	assert.Contains(t, output, "male")
	assert.Contains(t, output, "female")
	assert.Contains(t, output, "45")
	assert.Contains(t, output, "55")
	assert.Contains(t, output, "45.00 %")
	assert.Contains(t, output, "55.00 %")
	assert.Contains(t, output, "2. Group")
}

func TestRenderReport_MultipleCodings(t *testing.T) {
	report := fm.MeasureReport{
		Group: []fm.MeasureReportGroup{
			{
				Population: []fm.MeasureReportGroupPopulation{
					{
						Count: intPtr(100),
					},
				},
				Stratifier: []fm.MeasureReportGroupStratifier{
					{
						Code: []fm.CodeableConcept{
							{
								Text: stringPtr("Combo"),
							},
						},
						Stratum: []fm.MeasureReportGroupStratifierStratum{
							{
								Value: &fm.CodeableConcept{
									Coding: []fm.Coding{
										createCoding("sys1", "code1"),
										createCoding("sys2", "code2"),
									},
									Text: stringPtr("Combo Text"),
								},
								Population: []fm.MeasureReportGroupStratifierStratumPopulation{
									{
										Count: intPtr(10),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := renderReport(&buf, report)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	assert.Contains(t, output, "sys1")
	assert.Contains(t, output, "sys2")
	assert.Contains(t, output, "code1")
	assert.Contains(t, output, "code2")
}

func TestRenderReport_Coding_NullText(t *testing.T) {
	report := fm.MeasureReport{
		Group: []fm.MeasureReportGroup{
			{
				Population: []fm.MeasureReportGroupPopulation{
					{
						Count: intPtr(100),
					},
				},
				Stratifier: []fm.MeasureReportGroupStratifier{
					{
						Code: []fm.CodeableConcept{
							{
								Text: stringPtr("Combo"),
							},
						},
						Stratum: []fm.MeasureReportGroupStratifierStratum{
							{
								Value: &fm.CodeableConcept{
									Coding: []fm.Coding{
										createCoding("sys1", "code1"),
									},
									Text: stringPtr("null"),
								},
								Population: []fm.MeasureReportGroupStratifierStratumPopulation{
									{
										Count: intPtr(42),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := renderReport(&buf, report)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	assert.Contains(t, output, "sys1")
	assert.Contains(t, output, "code1")
	assert.Contains(t, output, "42")
}

func TestRenderReport_Nothing(t *testing.T) {
	report := fm.MeasureReport{
		Group: []fm.MeasureReportGroup{
			{
				Population: []fm.MeasureReportGroupPopulation{
					{
						Count: intPtr(100),
					},
				},
				Stratifier: []fm.MeasureReportGroupStratifier{
					{
						Code: []fm.CodeableConcept{
							{
								Text: stringPtr("Empty Stratifier"),
							},
						},
						Stratum: []fm.MeasureReportGroupStratifierStratum{
							{
								Value: &fm.CodeableConcept{
									Text: stringPtr("null"),
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := renderReport(&buf, report)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	assert.Contains(t, output, "nothing")
}

func TestRenderReport_Empty(t *testing.T) {
	report := fm.MeasureReport{}

	var buf bytes.Buffer
	err := renderReport(&buf, report)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	assert.Contains(t, output, "MeasureReport")
}
