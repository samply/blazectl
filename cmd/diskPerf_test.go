// Copyright 2019 - 2026 The Samply Community
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
	"bytes"
	"encoding/json"
	"testing"

	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
)

func TestCreateDiskPerfParameters(t *testing.T) {
	t.Run("without inputs", func(t *testing.T) {
		parameters := createDiskPerfParameters(nil, nil, nil, nil)

		assert.Empty(t, parameters.Parameter)
	})

	t.Run("with all inputs", func(t *testing.T) {
		database := "index"
		fileSize := 8.0
		phaseDuration := 60.5
		concurrency := 16

		parameters := createDiskPerfParameters(&database, &fileSize, &phaseDuration, &concurrency)

		assert.Equal(t, "database", parameters.Parameter[0].Name)
		assert.Equal(t, "index", *parameters.Parameter[0].ValueCode)

		assert.Equal(t, "file-size", parameters.Parameter[1].Name)
		assert.Equal(t, json.Number("8"), *parameters.Parameter[1].ValueDecimal)

		assert.Equal(t, "phase-duration", parameters.Parameter[2].Name)
		assert.Equal(t, json.Number("60.5"), *parameters.Parameter[2].ValueDecimal)

		assert.Equal(t, "concurrency", parameters.Parameter[3].Name)
		assert.Equal(t, 16, *parameters.Parameter[3].ValueUnsignedInt)
	})

	t.Run("with database only", func(t *testing.T) {
		database := "transaction"

		parameters := createDiskPerfParameters(&database, nil, nil, nil)

		assert.Len(t, parameters.Parameter, 1)
		assert.Equal(t, "database", parameters.Parameter[0].Name)
		assert.Equal(t, "transaction", *parameters.Parameter[0].ValueCode)
	})
}

func TestIsBlazeServer(t *testing.T) {
	t.Run("without software", func(t *testing.T) {
		assert.False(t, isBlazeServer(fm.CapabilityStatement{}))
	})

	t.Run("other software", func(t *testing.T) {
		capabilityStatement := fm.CapabilityStatement{
			Software: &fm.CapabilityStatementSoftware{Name: "HAPI FHIR Server"},
		}

		assert.False(t, isBlazeServer(capabilityStatement))
	})

	t.Run("Blaze", func(t *testing.T) {
		capabilityStatement := fm.CapabilityStatement{
			Software: &fm.CapabilityStatementSoftware{Name: "Blaze"},
		}

		assert.True(t, isBlazeServer(capabilityStatement))
	})
}

func capabilityStatementWithVersion(version string) fm.CapabilityStatement {
	return fm.CapabilityStatement{
		Software: &fm.CapabilityStatementSoftware{Name: "Blaze", Version: &version},
	}
}

func TestIsBlazeVersionOlderThan(t *testing.T) {
	t.Run("without software", func(t *testing.T) {
		assert.False(t, isBlazeVersionOlderThan(fm.CapabilityStatement{}, "1.11.0"))
	})

	t.Run("without version", func(t *testing.T) {
		capabilityStatement := fm.CapabilityStatement{
			Software: &fm.CapabilityStatementSoftware{Name: "Blaze"},
		}

		assert.False(t, isBlazeVersionOlderThan(capabilityStatement, "1.11.0"))
	})

	t.Run("older version", func(t *testing.T) {
		assert.True(t, isBlazeVersionOlderThan(capabilityStatementWithVersion("1.10.1"), "1.11.0"))
	})

	t.Run("same version", func(t *testing.T) {
		assert.False(t, isBlazeVersionOlderThan(capabilityStatementWithVersion("1.11.0"), "1.11.0"))
	})

	t.Run("newer version", func(t *testing.T) {
		assert.False(t, isBlazeVersionOlderThan(capabilityStatementWithVersion("1.12.3"), "1.11.0"))
	})

	t.Run("invalid version", func(t *testing.T) {
		assert.False(t, isBlazeVersionOlderThan(capabilityStatementWithVersion("unknown"), "1.11.0"))
	})
}

// executeDiskPerf executes the disk-perf command with the given args and
// returns the combined output alongside the error.
func executeDiskPerf(args ...string) (string, error) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(append([]string{"disk-perf"}, args...))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	err := rootCmd.Execute()
	return out.String(), err
}

func TestDiskPerfUsageOutput(t *testing.T) {
	t.Run("shown on invalid database argument", func(t *testing.T) {
		diskPerfCmd.SilenceUsage = false

		out, err := executeDiskPerf("foo", "--server", "http://localhost:1/fhir")

		assert.Error(t, err)
		assert.Contains(t, out, "Usage:")
	})

	t.Run("not shown on server errors", func(t *testing.T) {
		diskPerfCmd.SilenceUsage = false

		out, err := executeDiskPerf("index", "--server", "http://localhost:1/fhir")

		assert.Error(t, err)
		assert.NotContains(t, out, "Usage:")
	})
}

func quantityParameter(name string, value string, code string) fm.ParametersParameter {
	number := json.Number(value)
	system := "http://unitsofmeasure.org"
	return fm.ParametersParameter{
		Name:          name,
		ValueQuantity: &fm.Quantity{Value: &number, System: &system, Code: &code},
	}
}

func decimalParameter(name string, value string) fm.ParametersParameter {
	number := json.Number(value)
	return fm.ParametersParameter{Name: name, ValueDecimal: &number}
}

func codeParameter(name string, value string) fm.ParametersParameter {
	return fm.ParametersParameter{Name: name, ValueCode: &value}
}

func booleanParameter(name string, value bool) fm.ParametersParameter {
	return fm.ParametersParameter{Name: name, ValueBoolean: &value}
}

func TestRenderDiskPerfReport(t *testing.T) {
	t.Run("full report", func(t *testing.T) {
		parameters := fm.Parameters{
			Parameter: []fm.ParametersParameter{
				quantityParameter("seq-write-throughput", "868220928", "By/s"),
				quantityParameter("read-iops", "85000", "/s"),
				quantityParameter("read-throughput", "348127232", "By/s"),
				quantityParameter("read-latency-p50", "210", "us"),
				quantityParameter("read-latency-p95", "350", "us"),
				quantityParameter("read-latency-p99", "500", "us"),
				quantityParameter("read-latency-max", "1200", "us"),
				quantityParameter("fsync-rate", "520", "/s"),
				quantityParameter("fsync-latency-p50", "1100", "us"),
				quantityParameter("fsync-latency-p95", "1500", "us"),
				quantityParameter("fsync-latency-p99", "1900", "us"),
				booleanParameter("direct-io", true),
				decimalParameter("score", "87.5"),
				codeParameter("rating", "good"),
				quantityParameter("processing-duration", "65.2", "s"),
			},
		}

		assert.Equal(t, `Seq. Write Throughput  828.00 MiB/s
Read IOPS              85000/s
Read Throughput        332.00 MiB/s
Read Latency (p50)     210 µs
Read Latency (p95)     350 µs
Read Latency (p99)     500 µs
Read Latency (max)     1200 µs
Fsync Rate             520/s
Fsync Latency (p50)    1100 µs
Fsync Latency (p95)    1500 µs
Fsync Latency (p99)    1900 µs
Direct I/O             yes
Score                  87.5
Rating                 good
Processing Duration    65.2 s
`, renderDiskPerfReport(parameters))
	})

	t.Run("direct-io false", func(t *testing.T) {
		parameters := fm.Parameters{
			Parameter: []fm.ParametersParameter{booleanParameter("direct-io", false)},
		}

		assert.Equal(t, "Direct I/O  no\n", renderDiskPerfReport(parameters))
	})

	t.Run("unknown parameters are rendered with their name", func(t *testing.T) {
		parameters := fm.Parameters{
			Parameter: []fm.ParametersParameter{
				decimalParameter("score", "42"),
				quantityParameter("write-iops", "1000", "/s"),
			},
		}

		assert.Equal(t, `Score       42
write-iops  1000/s
`, renderDiskPerfReport(parameters))
	})
}
