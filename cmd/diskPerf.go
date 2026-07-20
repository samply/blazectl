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
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"

	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

// diskPerfMinBlazeVersion is the first Blaze version supporting the $disk-perf
// operation.
const diskPerfMinBlazeVersion = "1.11.0"

var diskPerfFileSize float64
var diskPerfPhaseDuration float64
var diskPerfConcurrency int
var diskPerfOutputFormat string

// diskPerfOutputLabels maps the output parameter names of the $disk-perf
// operation to human readable labels in rendering order.
var diskPerfOutputLabels = []struct{ name, label string }{
	{"seq-write-throughput", "Seq. Write Throughput"},
	{"read-iops", "Read IOPS"},
	{"read-throughput", "Read Throughput"},
	{"read-latency-p50", "Read Latency (p50)"},
	{"read-latency-p95", "Read Latency (p95)"},
	{"read-latency-p99", "Read Latency (p99)"},
	{"read-latency-max", "Read Latency (max)"},
	{"fsync-rate", "Fsync Rate"},
	{"fsync-latency-p50", "Fsync Latency (p50)"},
	{"fsync-latency-p95", "Fsync Latency (p95)"},
	{"fsync-latency-p99", "Fsync Latency (p99)"},
	{"direct-io", "Direct I/O"},
	{"score", "Score"},
	{"rating", "Rating"},
	{"processing-duration", "Processing Duration"},
}

var diskPerfCmd = &cobra.Command{
	Use:   "disk-perf [database]",
	Short: "Measure Disk Performance",
	Long: `Runs the $disk-perf operation that measures the performance of the disk
underlying a database directory volume.

The database can be one of index (default), transaction or resource. The
benchmark parameters can be tuned with the --file-size, --phase-duration and
--concurrency flags. Parameters not given on the command line are left to
their server-side defaults.

The results are printed in human readable form by default or as the FHIR
Parameters resource returned by the server if -o json is given.`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return databases, cobra.ShellCompDirectiveNoFileComp
		}
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("accepts at most 1 argument: database")
		}
		if len(args) == 1 && !slices.Contains(databases, args[0]) {
			return fmt.Errorf("invalid database. Must be one of: %s", strings.Join(databases, ", "))
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if diskPerfOutputFormat != "" && diskPerfOutputFormat != "json" {
			return fmt.Errorf("invalid output format `%s`. Must be: json", diskPerfOutputFormat)
		}

		err := createClient()
		if err != nil {
			return err
		}

		// All inputs are validated here, so errors from now on are server
		// related and printing the usage would only distract from them.
		cmd.SilenceUsage = true

		capabilityStatement, err := fetchCapabilityStatement(client)
		if err != nil {
			return err
		}
		if !isBlazeServer(capabilityStatement) {
			return fmt.Errorf("the server at %s isn't a Blaze server. The $disk-perf operation is only supported by Blaze", server)
		}
		if !fhir.DoesSupportSystemOperation(capabilityStatement, "disk-perf") {
			if isBlazeVersionOlderThan(capabilityStatement, diskPerfMinBlazeVersion) {
				return fmt.Errorf("the Blaze server at %s doesn't support the $disk-perf operation because its version %s is too old. Please update to Blaze version %s or later",
					server, *capabilityStatement.Software.Version, diskPerfMinBlazeVersion)
			}
			return fmt.Errorf("the Blaze server at %s doesn't support the $disk-perf operation. Please ensure that it is started with the environment variable ENABLE_ADMIN_API set to true", server)
		}

		var database *string
		if len(args) == 1 {
			database = &args[0]
		}
		var fileSize, phaseDuration *float64
		if cmd.Flags().Changed("file-size") {
			fileSize = &diskPerfFileSize
		}
		if cmd.Flags().Changed("phase-duration") {
			phaseDuration = &diskPerfPhaseDuration
		}
		var concurrency *int
		if cmd.Flags().Changed("concurrency") {
			concurrency = &diskPerfConcurrency
		}

		fmt.Fprintln(os.Stderr, "Start disk performance measurement...")
		req, err := client.NewPostSystemOperationRequest("disk-perf", true,
			createDiskPerfParameters(database, fileSize, phaseDuration, concurrency))
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 202 {
			return fmt.Errorf("expected status 202 (Accepted) while starting the disk performance measurement but was: %d", resp.StatusCode)
		}

		contentLocation := resp.Header.Get("Content-Location")
		if err := fhir.DiscardAndClose(resp.Body); err != nil {
			return err
		}
		interruptChan := make(chan os.Signal, 1)
		signal.Notify(interruptChan, os.Interrupt)
		resource, err := client.PollAsyncStatus(contentLocation, interruptChan)
		if err != nil {
			return err
		}

		if diskPerfOutputFormat == "json" {
			var buf bytes.Buffer
			if err := json.Indent(&buf, resource, "", "  "); err != nil {
				return err
			}
			fmt.Println(buf.String())
			return nil
		}

		parameters, err := fm.UnmarshalParameters(resource)
		if err != nil {
			return fmt.Errorf("error while reading the disk performance measurement result: %w", err)
		}
		fmt.Print(renderDiskPerfReport(parameters))
		return nil
	},
}

// isBlazeServer returns true if the capability statement identifies the server
// software as Blaze.
func isBlazeServer(capabilityStatement fm.CapabilityStatement) bool {
	return capabilityStatement.Software != nil && capabilityStatement.Software.Name == "Blaze"
}

// isBlazeVersionOlderThan returns true if the capability statement contains a
// valid software version that is older than the given version. Missing or
// invalid versions return false because they allow no conclusion.
func isBlazeVersionOlderThan(capabilityStatement fm.CapabilityStatement, version string) bool {
	if capabilityStatement.Software == nil || capabilityStatement.Software.Version == nil {
		return false
	}
	serverVersion := "v" + *capabilityStatement.Software.Version
	return semver.IsValid(serverVersion) && semver.Compare(serverVersion, "v"+version) < 0
}

// createDiskPerfParameters creates the input Parameters resource of the
// $disk-perf operation. Nil inputs are omitted so that the server-side
// defaults apply.
func createDiskPerfParameters(database *string, fileSize *float64, phaseDuration *float64,
	concurrency *int) fm.Parameters {
	var parameters []fm.ParametersParameter
	if database != nil {
		parameters = append(parameters, fm.ParametersParameter{Name: "database", ValueCode: database})
	}
	if fileSize != nil {
		parameters = append(parameters, fm.ParametersParameter{Name: "file-size", ValueDecimal: decimal(*fileSize)})
	}
	if phaseDuration != nil {
		parameters = append(parameters, fm.ParametersParameter{Name: "phase-duration", ValueDecimal: decimal(*phaseDuration)})
	}
	if concurrency != nil {
		parameters = append(parameters, fm.ParametersParameter{Name: "concurrency", ValueUnsignedInt: concurrency})
	}
	return fm.Parameters{Parameter: parameters}
}

func decimal(value float64) *json.Number {
	number := json.Number(strconv.FormatFloat(value, 'f', -1, 64))
	return &number
}

// renderDiskPerfReport renders the output Parameters resource of the
// $disk-perf operation in human readable form. Known output parameters are
// rendered in a fixed order with human readable labels, unknown ones after
// them with their name as label.
func renderDiskPerfReport(parameters fm.Parameters) string {
	type line struct{ label, value string }
	var lines []line

	remaining := slices.Clone(parameters.Parameter)
	takeByName := func(name string) *fm.ParametersParameter {
		for i, parameter := range remaining {
			if parameter.Name == name {
				remaining = slices.Delete(remaining, i, i+1)
				return &parameter
			}
		}
		return nil
	}

	for _, output := range diskPerfOutputLabels {
		if parameter := takeByName(output.name); parameter != nil {
			lines = append(lines, line{output.label, fmtParameterValue(*parameter)})
		}
	}
	for _, parameter := range remaining {
		lines = append(lines, line{parameter.Name, fmtParameterValue(parameter)})
	}

	var labelWidth int
	for _, l := range lines {
		if len(l.label) > labelWidth {
			labelWidth = len(l.label)
		}
	}

	builder := strings.Builder{}
	for _, l := range lines {
		fmt.Fprintf(&builder, "%-*s  %s\n", labelWidth, l.label, l.value)
	}
	return builder.String()
}

func fmtParameterValue(parameter fm.ParametersParameter) string {
	switch {
	case parameter.ValueQuantity != nil:
		return fmtQuantity(*parameter.ValueQuantity)
	case parameter.ValueBoolean != nil:
		if *parameter.ValueBoolean {
			return "yes"
		}
		return "no"
	case parameter.ValueDecimal != nil:
		return parameter.ValueDecimal.String()
	case parameter.ValueCode != nil:
		return *parameter.ValueCode
	case parameter.ValueString != nil:
		return *parameter.ValueString
	default:
		return "<unknown>"
	}
}

func fmtQuantity(quantity fm.Quantity) string {
	var value string
	if quantity.Value != nil {
		value = quantity.Value.String()
	}
	var unit string
	if quantity.Code != nil {
		unit = *quantity.Code
	} else if quantity.Unit != nil {
		unit = *quantity.Unit
	}
	switch unit {
	case "By/s":
		if bytesPerSecond, err := quantity.Value.Float64(); err == nil {
			return util.FmtBytesHumanReadable(float32(bytesPerSecond)) + "/s"
		}
		return value + " B/s"
	case "us":
		return value + " µs"
	case "":
		return value
	default:
		if strings.HasPrefix(unit, "/") {
			return value + unit
		}
		return value + " " + unit
	}
}

func init() {
	rootCmd.AddCommand(diskPerfCmd)

	diskPerfCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")
	diskPerfCmd.Flags().Float64Var(&diskPerfFileSize, "file-size", 4, "the size of the test file in GiB")
	diskPerfCmd.Flags().Float64Var(&diskPerfPhaseDuration, "phase-duration", 30, "the duration of the rand-read and the fsync phase in seconds")
	diskPerfCmd.Flags().IntVar(&diskPerfConcurrency, "concurrency", 8, "the number of concurrent reader threads in the rand-read phase")
	diskPerfCmd.Flags().StringVarP(&diskPerfOutputFormat, "output", "o", "", "output format. One of: json")

	_ = diskPerfCmd.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"json"}, cobra.ShellCompDirectiveNoFileComp
		})

	_ = diskPerfCmd.MarkFlagRequired("server")
}
