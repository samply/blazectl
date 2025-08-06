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
	"sort"
	"strings"
	"time"

	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
)

type CommandStats struct {
	TotalPages                            int
	ResourcesPerPage                      []int
	RequestDurations, ProcessingDurations []float64
	TotalBytesIn                          int64
	TotalDuration                         time.Duration
	InlineOperationOutcomes               []*fm.OperationOutcome
	Error                                 *ErrorResponse
}

func (cs *CommandStats) String() string {

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("Pages		[total]			%d\n", cs.TotalPages))

	var resourcesTotal int
	for _, res := range cs.ResourcesPerPage {
		resourcesTotal += res
	}
	builder.WriteString(fmt.Sprintf("Resources 	[total]			%d\n", resourcesTotal))

	if len(cs.ResourcesPerPage) > 0 {
		sort.Ints(cs.ResourcesPerPage)
		var totalResources int
		for _, v := range cs.ResourcesPerPage {
			totalResources += v
		}

		builder.WriteString(fmt.Sprintf("Resources/Page	[min, mean, max]	%d, %d, %d\n", cs.ResourcesPerPage[0], totalResources/len(cs.ResourcesPerPage), cs.ResourcesPerPage[len(cs.ResourcesPerPage)-1]))
	}

	builder.WriteString(fmt.Sprintf("Duration	[total]			%s\n", FmtDurationHumanReadable(cs.TotalDuration)))

	if len(cs.RequestDurations) > 0 {
		p := CalculateDurationStatistics(cs.RequestDurations)
		builder.WriteString(fmt.Sprintf("Requ. Latencies	[mean, 50, 95, 99, max]	%s, %s, %s, %s, %s\n", p.Mean, p.Q50, p.Q95, p.Q99, p.Max))
	}

	if len(cs.ProcessingDurations) > 0 {
		p := CalculateDurationStatistics(cs.ProcessingDurations)
		builder.WriteString(fmt.Sprintf("Proc. Latencies	[mean, 50, 95, 99, max]	%s, %s, %s, %s, %s\n", p.Mean, p.Q50, p.Q95, p.Q99, p.Max))
	}

	totalRequests := len(cs.RequestDurations)
	builder.WriteString(fmt.Sprintf("Bytes In	[total, mean]		%s, %s\n", FmtBytesHumanReadable(float32(cs.TotalBytesIn)), FmtBytesHumanReadable(float32(cs.TotalBytesIn)/float32(totalRequests))))

	if len(cs.InlineOperationOutcomes) > 0 {
		builder.WriteString("\nServer Warnings & Information:\n")
		builder.WriteString(Indent(2, FmtOperationOutcomes(cs.InlineOperationOutcomes)))
	}

	if cs.Error != nil {
		builder.WriteString("\nServer Error:\n")
		builder.WriteString(Indent(2, cs.Error.String()))
	}

	return builder.String()
}
