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
		maxConcurrency := 64

		parameters := createDiskPerfParameters(&database, &fileSize, &phaseDuration, &maxConcurrency)

		assert.Equal(t, "database", parameters.Parameter[0].Name)
		assert.Equal(t, "index", *parameters.Parameter[0].ValueCode)

		assert.Equal(t, "file-size", parameters.Parameter[1].Name)
		assert.Equal(t, json.Number("8"), *parameters.Parameter[1].ValueDecimal)

		assert.Equal(t, "phase-duration", parameters.Parameter[2].Name)
		assert.Equal(t, json.Number("60.5"), *parameters.Parameter[2].ValueDecimal)

		assert.Equal(t, "max-concurrency", parameters.Parameter[3].Name)
		assert.Equal(t, 64, *parameters.Parameter[3].ValuePositiveInt)
		assert.Nil(t, parameters.Parameter[3].ValueUnsignedInt)
	})

	t.Run("with database only", func(t *testing.T) {
		database := "transaction"

		parameters := createDiskPerfParameters(&database, nil, nil, nil)

		assert.Len(t, parameters.Parameter, 1)
		assert.Equal(t, "database", parameters.Parameter[0].Name)
		assert.Equal(t, "transaction", *parameters.Parameter[0].ValueCode)
	})
}

func TestDiskPerfFlags(t *testing.T) {
	t.Run("max-concurrency defaults to 32", func(t *testing.T) {
		flag := diskPerfCmd.Flags().Lookup("max-concurrency")

		if assert.NotNil(t, flag) {
			assert.Equal(t, "32", flag.DefValue)
		}
	})

	t.Run("concurrency is gone", func(t *testing.T) {
		assert.Nil(t, diskPerfCmd.Flags().Lookup("concurrency"))
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

func TestDiskPerfMaxConcurrencyValidation(t *testing.T) {
	for _, value := range []string{"0", "-1", "1025"} {
		t.Run("rejects "+value, func(t *testing.T) {
			diskPerfCmd.SilenceUsage = false
			defer resetDiskPerfMaxConcurrencyFlag()

			out, err := executeDiskPerf("index", "--server", "http://localhost:1/fhir", "--max-concurrency", value)

			if assert.Error(t, err) {
				assert.Equal(t, "invalid max-concurrency `"+value+"`. Must be between 1 and 1024", err.Error())
			}
			assert.Contains(t, out, "Usage:")
		})
	}

	for _, value := range []string{"1", "1024"} {
		t.Run("accepts "+value, func(t *testing.T) {
			diskPerfCmd.SilenceUsage = false
			defer resetDiskPerfMaxConcurrencyFlag()

			out, err := executeDiskPerf("index", "--server", "http://localhost:1/fhir", "--max-concurrency", value)

			// fails later because no server is running
			assert.Error(t, err)
			assert.NotContains(t, out, "Usage:")
		})
	}
}

// resetDiskPerfMaxConcurrencyFlag resets the max-concurrency flag to its
// default so that it doesn't leak into other tests.
func resetDiskPerfMaxConcurrencyFlag() {
	flag := diskPerfCmd.Flags().Lookup("max-concurrency")
	_ = flag.Value.Set(flag.DefValue)
	flag.Changed = false
}

func TestFmtQuantity(t *testing.T) {
	t.Run("By/s without value", func(t *testing.T) {
		code := "By/s"
		quantity := fm.Quantity{Code: &code}

		assert.NotPanics(t, func() {
			assert.Equal(t, " B/s", fmtQuantity(quantity))
		})
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

func positiveIntParameter(name string, value int) fm.ParametersParameter {
	return fm.ParametersParameter{Name: name, ValuePositiveInt: &value}
}

func randReadParameter(concurrency int, iops, throughput, p50, p95, p99, max string) fm.ParametersParameter {
	return fm.ParametersParameter{
		Name: "rand-read",
		Part: []fm.ParametersParameter{
			positiveIntParameter("concurrency", concurrency),
			quantityParameter("iops", iops, "/s"),
			quantityParameter("throughput", throughput, "By/s"),
			quantityParameter("latency-p50", p50, "us"),
			quantityParameter("latency-p95", p95, "us"),
			quantityParameter("latency-p99", p99, "us"),
			quantityParameter("latency-max", max, "us"),
		},
	}
}

func TestRenderDiskPerfReport(t *testing.T) {
	t.Run("full report", func(t *testing.T) {
		parameters := fm.Parameters{
			Parameter: []fm.ParametersParameter{
				quantityParameter("seq-write-throughput", "868220928", "By/s"),
				randReadParameter(1, "5000", "83886080", "190", "250", "300", "900"),
				randReadParameter(32, "85000", "348127232", "210", "350", "500", "1200"),
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
Fsync Rate             520/s
Fsync Latency (p50)    1100 µs
Fsync Latency (p95)    1500 µs
Fsync Latency (p99)    1900 µs
Direct I/O             yes
Score                  87.5
Rating                 good
Processing Duration    65.2 s

Random Reads
Concurrency     IOPS    Throughput  Latency (p50)  Latency (p95)  Latency (p99)  Latency (max)
          1   5000/s   80.00 MiB/s         190 µs         250 µs         300 µs         900 µs
         32  85000/s  332.00 MiB/s         210 µs         350 µs         500 µs        1200 µs
`, renderDiskPerfReport(parameters))
	})

	t.Run("direct-io false", func(t *testing.T) {
		parameters := fm.Parameters{
			Parameter: []fm.ParametersParameter{booleanParameter("direct-io", false)},
		}

		assert.Equal(t, "Direct I/O  no\n", renderDiskPerfReport(parameters))
	})

	t.Run("unknown parameters are ignored", func(t *testing.T) {
		parameters := fm.Parameters{
			Parameter: []fm.ParametersParameter{
				decimalParameter("score", "42"),
				quantityParameter("write-iops", "1000", "/s"),
			},
		}

		assert.Equal(t, "Score  42\n", renderDiskPerfReport(parameters))
	})

	t.Run("unknown rand-read parts are ignored", func(t *testing.T) {
		randRead := randReadParameter(1, "5000", "83886080", "190", "250", "300", "900")
		randRead.Part = append(randRead.Part, quantityParameter("latency-p999", "700", "us"))
		parameters := fm.Parameters{Parameter: []fm.ParametersParameter{randRead}}

		assert.Equal(t, `
Random Reads
Concurrency    IOPS   Throughput  Latency (p50)  Latency (p95)  Latency (p99)  Latency (max)
          1  5000/s  80.00 MiB/s         190 µs         250 µs         300 µs         900 µs
`, renderDiskPerfReport(parameters))
	})
}
