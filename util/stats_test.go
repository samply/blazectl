// Copyright 2019 - 2021 The Samply Community
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
	"github.com/stretchr/testify/assert"
	"math"
	"strings"
	"testing"
	"time"
)

func TestCalculateDurationStatistics_emptyDurationSet(t *testing.T) {
	statistics := CalculateDurationStatistics([]float64{})
	assert.Equal(t, time.Duration(0), statistics.Mean)
	assert.Equal(t, time.Duration(0), statistics.Max)
	assert.Equal(t, time.Duration(0), statistics.Q50)
	assert.Equal(t, time.Duration(0), statistics.Q95)
	assert.Equal(t, time.Duration(0), statistics.Q99)
}

func TestCalculateDurationStatistics(t *testing.T) {
	statistics := CalculateDurationStatistics([]float64{1.0})
	assert.Equal(t, 1.0*time.Second, statistics.Mean)
	assert.Equal(t, 1.0*time.Second, statistics.Max)
	assert.Equal(t, 1.0*time.Second, statistics.Q50)
	assert.Equal(t, 1.0*time.Second, statistics.Q95)
	assert.Equal(t, 1.0*time.Second, statistics.Q99)
}

func TestFmtBytesHumanReadable(t *testing.T) {
	byteUnitMappings := map[float32]string{
		1:                               "B",
		float32(10 * math.Pow(1024, 1)): "KiB",
		float32(10 * math.Pow(1024, 2)): "MiB",
		float32(10 * math.Pow(1024, 3)): "GiB",
		float32(10 * math.Pow(1024, 4)): "TiB",
		float32(10 * math.Pow(1024, 5)): "PiB",
		float32(10 * math.Pow(1024, 6)): "PiB",
	}

	for bytes, unit := range byteUnitMappings {
		t.Run(unit, func(t *testing.T) {
			humanReadableResult := FmtBytesHumanReadable(bytes)
			assert.True(t, strings.HasSuffix(humanReadableResult, unit))
		})
	}
}

func TestFmtDurationHumanReadable(t *testing.T) {
	durationFormatMappings := map[string]string{
		"0s512ms":   "512ms",
		"1012ms":    "1.012s",
		"1005ms":    "1.005s",
		"1000ms":    "1s",
		"2800ms":    "2.8s",
		"60000ms":   "1m0s",
		"62000ms":   "1m2s",
		"620000ms":  "10m20s",
		"3600000ms": "1h0m0s",
	}

	for duration, format := range durationFormatMappings {
		t.Run(format, func(t *testing.T) {
			d, _ := time.ParseDuration(duration)

			humanReadableResult := FmtDurationHumanReadable(d)
			assert.Equal(t, format, humanReadableResult)
		})
	}
}
