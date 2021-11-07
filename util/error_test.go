package util

import (
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"testing"
)

var text = "text-133546"
var code = "code-130834"
var diagnostics = "diagnostics-131023"

func TestString(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		errorResponse := &ErrorResponse{
			StatusCode: 400,
			Error:      &fm.OperationOutcome{},
		}
		assert.Equal(t, "StatusCode  : 400\n", errorResponse.String())
	})

	t.Run("WithOneIssue", func(t *testing.T) {
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
	})

	t.Run("WithOneIssueAndDetailsWithText", func(t *testing.T) {
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
	})

	t.Run("WithOneIssueAndDetailsWithCode", func(t *testing.T) {
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
	})

	t.Run("WithOneIssueAndDiagnostics", func(t *testing.T) {
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
	})

	t.Run("WithOneIssueAndOneExpression", func(t *testing.T) {
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
	})

	t.Run("WithOneIssueAndTwoExpressions", func(t *testing.T) {
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
	})

	t.Run("WithTwoIssues", func(t *testing.T) {
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
	})
}
