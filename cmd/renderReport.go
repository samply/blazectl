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

package cmd

import (
	_ "embed"
	"encoding/json"
	"io"
	"os"

	"html/template"

	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
)

//go:embed report-template.gohtml
var reportTemplate string

func renderReport(wr io.Writer, report fm.MeasureReport) error {
	funcMap := template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
		"ratio": func(n int, d int) float32 {
			return float32(n*100) / float32(d)
		},
		"isNullString": func(s *string) bool {
			return s == nil || *s == "null"
		},
	}

	tmpl := template.Must(template.New("report").Funcs(funcMap).Parse(reportTemplate))

	return tmpl.Execute(wr, report)
}

var renderReportCmd = &cobra.Command{
	Use:   "render-report",
	Short: "Renders a MeasureReport",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		var report fm.MeasureReport
		if err := json.Unmarshal(data, &report); err != nil {
			return err
		}

		if err := renderReport(os.Stdout, report); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(renderReportCmd)
}
