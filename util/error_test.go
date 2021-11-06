package util

import (
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestString_empty(t *testing.T) {
	errorResponse := &ErrorResponse{
		StatusCode: 400,
		Error:      &fm.OperationOutcome{},
	}
	assert.Equal(t, "StatusCode  : 400\n", errorResponse.String())
}

func TestString_withOneIssue(t *testing.T) {
	errorResponse := &ErrorResponse{
		StatusCode: 400,
		Error: &fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{{}},
		},
	}
	assert.Equal(t, `StatusCode  : 400
Severity    : Fatal
Code        : Content invalid against the specification or a profile.
`, errorResponse.String())
}

var text = "text-133546"

func TestString_withOneIssueAndDetailsWithText(t *testing.T) {
	errorResponse := &ErrorResponse{
		StatusCode: 400,
		Error: &fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{
				{Details: &fm.CodeableConcept{Text: &text}},
			},
		},
	}
	assert.Equal(t, `StatusCode  : 400
Severity    : Fatal
Code        : Content invalid against the specification or a profile.
Details     : text-133546
`, errorResponse.String())
}

var code = "code-130834"

func TestString_withOneIssueAndDetailsWithCode(t *testing.T) {
	errorResponse := &ErrorResponse{
		StatusCode: 400,
		Error: &fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{
				{Details: &fm.CodeableConcept{Coding: []fm.Coding{{Code: &code}}}},
			},
		},
	}
	assert.Equal(t, `StatusCode  : 400
Severity    : Fatal
Code        : Content invalid against the specification or a profile.
Details     : code-130834
`, errorResponse.String())
}

var diagnostics = "diagnostics-131023"

func TestString_withOneIssueAndDiagnostics(t *testing.T) {
	errorResponse := &ErrorResponse{
		StatusCode: 400,
		Error: &fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{{Diagnostics: &diagnostics}},
		},
	}
	assert.Equal(t, `StatusCode  : 400
Severity    : Fatal
Code        : Content invalid against the specification or a profile.
Diagnostics : diagnostics-131023
`, errorResponse.String())
}

func TestString_withOneIssueAndOneExpression(t *testing.T) {
	errorResponse := &ErrorResponse{
		StatusCode: 400,
		Error: &fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{{Expression: []string{"expression-131256"}}},
		},
	}
	assert.Equal(t, `StatusCode  : 400
Severity    : Fatal
Code        : Content invalid against the specification or a profile.
Expression  : expression-131256
`, errorResponse.String())
}

func TestString_withOneIssueAndTwoExpressions(t *testing.T) {
	errorResponse := &ErrorResponse{
		StatusCode: 400,
		Error: &fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{
				{Expression: []string{"expression-131256", "expression-131345"}},
			},
		},
	}
	assert.Equal(t, `StatusCode  : 400
Severity    : Fatal
Code        : Content invalid against the specification or a profile.
Expression  : expression-131256, expression-131345
`, errorResponse.String())
}

func TestString_withTwoIssues(t *testing.T) {
	errorResponse := &ErrorResponse{
		StatusCode: 400,
		Error: &fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{{}, {}},
		},
	}
	assert.Equal(t, `StatusCode  : 400
Severity    : Fatal
Code        : Content invalid against the specification or a profile.
---
Severity    : Fatal
Code        : Content invalid against the specification or a profile.
`, errorResponse.String())
}
