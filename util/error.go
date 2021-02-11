package util

import (
	"fmt"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"strings"
)

// ErrorResponse represents an error returned from the FHIR server.
type ErrorResponse struct {
	StatusCode int
	Error      *fm.OperationOutcome
}

// String returns the ErrorResponse in a default formatted way.
func (errRes *ErrorResponse) String(indentationSteps int) string {
	builder := strings.Builder{}
	builder.WriteString(indentString(indentationSteps, fmt.Sprintf("StatusCode	: %d\n", errRes.StatusCode)))
	builder.WriteString(FmtOperationOutcome(indentationSteps, []*fm.OperationOutcome{errRes.Error}))
	return builder.String()
}

func FmtOperationOutcome(indentationSteps int, outcome []*fm.OperationOutcome) string {
	builder := strings.Builder{}

	for i, o := range outcome {
		if i != 0 {
			builder.WriteString("---")
		}

		for j, issue := range o.Issue {
			if j != 0 {
				builder.WriteString("---")
			}

			builder.WriteString(indentString(indentationSteps, fmt.Sprintf("Severity	: %s\n", issue.Severity.Display())))
			builder.WriteString(indentString(indentationSteps, fmt.Sprintf("Code		: %s\n", issue.Code.Definition())))
			if details := issue.Details; details != nil {
				if text := details.Text; text != nil {
					builder.WriteString(indentString(indentationSteps, fmt.Sprintf("Details	: %s\n", *text)))
				} else if codings := details.Coding; len(codings) > 0 {
					if code := codings[0].Code; code != nil {
						builder.WriteString(indentString(indentationSteps, fmt.Sprintf("Details	: %s\n", *code)))
					}
				}
			}
			if diagnostics := issue.Diagnostics; diagnostics != nil {
				builder.WriteString(indentString(indentationSteps, fmt.Sprintf("Diagnostics	: %s\n", *diagnostics)))
			}
			if expressions := issue.Expression; len(expressions) > 0 {
				builder.WriteString(indentString(indentationSteps, fmt.Sprintf("Expression	: %s\n", strings.Join(expressions, ", "))))
			}
		}
	}

	return builder.String()
}

// indentString takes a source string and indents it with a many whitespace as indentation steps are specified.
// Returns the indented string.
//
// Panics if the given indentation steps are negative or if the result of (len(source) * indentationSteps) overflows.
func indentString(indentationSteps int, source string) string {
	indentation := strings.Repeat(" ", indentationSteps)
	return indentation + source
}
