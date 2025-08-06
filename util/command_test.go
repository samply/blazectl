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
	"testing"
	"time"

	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
)

func TestCommandStats_String(t *testing.T) {
	t.Run("Empty CommandStats", func(t *testing.T) {
		cs := &CommandStats{}
		result := cs.String()

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Pages")
		assert.Contains(t, result, "Resources")
		assert.Contains(t, result, "Duration")
		assert.Contains(t, result, "Bytes In")
	})

	t.Run("CommandStats with basic data", func(t *testing.T) {
		cs := &CommandStats{
			TotalPages:       3,
			ResourcesPerPage: []int{10, 15, 12},
			TotalDuration:    5 * time.Second,
			TotalBytesIn:     2048,
		}
		result := cs.String()

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "3")  // TotalPages
		assert.Contains(t, result, "37") // Total resources (10+15+12)
	})

	t.Run("CommandStats with durations", func(t *testing.T) {
		cs := &CommandStats{
			RequestDurations:    []float64{100, 150, 200, 250, 300},
			ProcessingDurations: []float64{50, 75, 100, 125, 150},
		}
		result := cs.String()

		assert.Contains(t, result, "Requ. Latencies")
		assert.Contains(t, result, "Proc. Latencies")
	})

	t.Run("CommandStats with warnings", func(t *testing.T) {
		textMsg := "Warning message"
		cs := &CommandStats{
			InlineOperationOutcomes: []*fm.OperationOutcome{
				{
					Issue: []fm.OperationOutcomeIssue{
						{
							Details: &fm.CodeableConcept{
								Text: &textMsg,
							},
						},
					},
				},
			},
		}
		result := cs.String()

		assert.Contains(t, result, "Server Warnings")
	})

	t.Run("CommandStats with error", func(t *testing.T) {
		cs := &CommandStats{
			Error: &ErrorResponse{
				StatusCode: 500,
				OtherError: "Internal Server Error",
			},
		}
		result := cs.String()

		assert.Contains(t, result, "Server Error")
	})

	t.Run("CommandStats returns valid string", func(t *testing.T) {
		cs := &CommandStats{
			TotalPages:          5,
			ResourcesPerPage:    []int{20, 25, 30, 15, 10},
			RequestDurations:    []float64{100, 150, 200},
			ProcessingDurations: []float64{50, 75, 100},
			TotalBytesIn:        4096,
			TotalDuration:       10 * time.Second,
		}
		result := cs.String()

		// Just verify it's a non-empty string without specific format checks
		assert.NotEmpty(t, result)
		assert.IsType(t, "", result)
	})
}
