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
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	"github.com/spf13/cobra"
)

var historySince string

// fhirInstantPattern is the regex of the FHIR instant data type.
var fhirInstantPattern = regexp.MustCompile(`^([0-9]([0-9]([0-9][1-9]|[1-9]0)|[1-9]00)|[1-9]000)-(0[1-9]|1[0-2])-(0[1-9]|[1-2][0-9]|3[0-1])T([01][0-9]|2[0-3]):[0-5][0-9]:([0-5][0-9]|60)(\.[0-9]+)?(Z|(\+|-)((0[0-9]|1[0-3]):[0-5][0-9]|14:00))$`)

// isFhirInstant returns true if value is a FHIR instant denoting an existing date.
//
// The pattern is needed because time.Parse is too lenient. It accepts values like
// 2026-01-01T0:00:00Z, which servers may silently ignore instead of rejecting.
// The parsing is needed because the pattern accepts dates like 2026-02-30.
func isFhirInstant(value string) bool {
	if !fhirInstantPattern.MatchString(value) {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

var downloadHistoryCmd = &cobra.Command{
	Use:   "download-history [resource-type [resource-id]]",
	Short: "Download history in NDJSON format",
	Long: `Downloads history, extracts the resources from
the returned bundles and outputs one resource per line in NDJSON format.

If the optional resource-type and resource-id are given, the corresponding 
resource-level history will be downloaded.

If only the optional resource-type is given, the corresponding type-level
history will be downloaded.

If resource-type and -id are omitted, the system-level search will be used 
and all resources of the whole system will be downloaded. 

Resources will be either streamed to STDOUT, delimited by newline, or
stored in a file if the --output-file flag is given.

If the --since flag is given, only resource versions created at or after the
given instant will be downloaded.

The --count flag sets the number of resources per page using the FHIR parameter
_count.

Examples:
  blazectl download-history --server http://localhost:8080/fhir Patient DFRE25Q627JVEWOS > patient-history.ndjson
  blazectl download-history --server http://localhost:8080/fhir Patient > patients-history.ndjson
  blazectl download-history --server http://localhost:8080/fhir > system-history.ndjson
  blazectl download-history --server http://localhost:8080/fhir --since 2026-01-01T00:00:00Z > recent-history.ndjson`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return fhir.ResourceTypes, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("since") && !isFhirInstant(historySince) {
			return fmt.Errorf("invalid since `%s`. Must be a date-time with timezone like 2026-01-01T00:00:00Z", historySince)
		}

		if err := validateCount(cmd); err != nil {
			return err
		}

		if err := createClient(); err != nil {
			return err
		}
		var stats util.CommandStats
		startTime := time.Now()

		file, err := openOutputFile(cmd, outputFile)
		if err != nil {
			return err
		}
		sink := newOutputSink(file)
		defer file.Close()
		defer file.Sync()

		// unbuffered, so that at most one page is requested ahead
		bundleChannel := make(chan fhir.DownloadBundle)

		var resourceType string
		var resourceId string
		if len(args) > 0 {
			resourceType = args[0]
		}
		if len(args) > 1 {
			resourceId = args[1]
		}

		go downloadHistory(client, resourceType, resourceId, historySince, count, bundleChannel)

		err = processBundles(bundleChannel, &stats, sink)

		stats.TotalDuration = time.Since(startTime)
		fmt.Fprint(metaOut(cmd), stats.String())
		if err != nil {
			// All inputs are validated here, so printing the usage would only
			// distract from the error.
			cmd.SilenceUsage = true
			return err
		}
		return nil
	},
}

// downloadHistory downloads the history of resources from a FHIR server.
// The history can be at system-level (if resourceType is empty), type-level (if only resourceType
// is provided), or instance-level (if both resourceType and resourceId are provided).
// If since is not empty, only resource versions created at or after that instant are downloaded.
// If count is positive, it's used as the number of resources per page.
// The download respects pagination, i.e., it follows pagination links until there is no other next link.
//
// Downloaded bundles as well as errors are sent to the given result channel.
// As soon as an error occurs, it is written to the channel and the channel is closed thereafter.
func downloadHistory(client *fhir.Client, resourceType string, resourceId string, since string, count int, resChannel chan<- fhir.DownloadBundle) {
	defer close(resChannel)

	var request *http.Request
	var err error

	query := url.Values{}
	if since != "" {
		query.Set("_since", since)
	}
	if count > 0 {
		query.Set("_count", strconv.Itoa(count))
	}

	if resourceType != "" {
		if resourceId != "" {
			request, err = client.NewHistoryInstanceRequest(resourceType, resourceId, query)
		} else {
			request, err = client.NewHistoryTypeRequest(resourceType, query)
		}
	} else {
		request, err = client.NewHistorySystemRequest(query)
	}
	if err != nil {
		resChannel <- fhir.DownloadBundleError("could not create FHIR server request: %v\n", err)
		return
	}

	client.ExpandPages(request, resChannel)
}

func init() {
	rootCmd.AddCommand(downloadHistoryCmd)

	downloadHistoryCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")
	downloadHistoryCmd.Flags().StringVarP(&outputFile, "output-file", "o", "", "write to file instead of stdout")
	downloadHistoryCmd.Flags().StringVar(&historySince, "since", "", "only download resource versions created at or after this instant, e.g. 2026-01-01T00:00:00Z")
	downloadHistoryCmd.Flags().IntVar(&count, "count", 0, "the number of resources per page, sent as _count")

	_ = downloadHistoryCmd.MarkFlagRequired("server")
	_ = downloadHistoryCmd.MarkFlagFilename("output-file", "ndjson")
}
