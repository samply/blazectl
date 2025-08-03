// Copyright 2019 - 2025 The Samply Community
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"strings"
	"text/template"
)

// ErrorResponse represents an error returned from the FHIR server.
type ErrorResponse struct {
	StatusCode       int
	OperationOutcome *fm.OperationOutcome
	OtherError       string
}

// String returns the ErrorResponse in a default formatted way.
func (errRes *ErrorResponse) String() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("StatusCode  : %d\n", errRes.StatusCode))
	if errRes.OperationOutcome != nil {
		builder.WriteString(FmtOperationOutcomes([]*fm.OperationOutcome{errRes.OperationOutcome}))
	}
	if len(errRes.OtherError) > 0 {
		builder.WriteString(fmt.Sprintf("Error       : %s\n", IndentExceptFirstLine(14, errRes.OtherError)))
	}
	return builder.String()
}

var outcomeTemplate, _ = template.New("outcomes").
	Funcs(template.FuncMap{"join": strings.Join}).
	Parse(`{{ define "issue" -}}
Severity    : {{ .Severity.Display }}
Code        : {{ .Code.Definition }}
{{ with .Details -}}
{{ with .Text -}}
Details     : {{ . }}
{{ end -}}
{{ range .Coding -}}
{{ with .Code -}}
Details     : {{ . }}
{{ end -}}
{{ end -}}
{{ end -}}
{{ with .Diagnostics -}}
Diagnostics : {{ . }}
{{ end -}}
{{ with .Expression -}}
Expression  : {{ join . ", " }}
{{ end -}}
{{ end -}}

{{ define "outcome" -}}
{{ range $index, $issue := .Issue -}}
{{ if $index }}---
{{ end -}}
{{ template "issue" $issue -}} 
{{ end -}}
{{ end -}}

{{ range $index, $outcome := . -}}
{{ if $index }}---
{{ end -}}
{{ template "outcome" $outcome -}} 
{{ end -}}
`)

func FmtOperationOutcomes(outcome []*fm.OperationOutcome) string {
	builder := strings.Builder{}

	err := outcomeTemplate.Execute(&builder, outcome)
	if err != nil {
		return err.Error()
	}

	return builder.String()
}

func Indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + IndentExceptFirstLine(spaces, v)
}

func IndentExceptFirstLine(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return strings.ReplaceAll(v, "\n", "\n"+pad)
}
