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
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	"github.com/spf13/cobra"
)

var outputFile string
var fhirSearchQuery string
var usePost bool

var downloadCmd = &cobra.Command{
	Use:   "download [resource-type]",
	Short: "Download resources in NDJSON format",
	Long: `Downloads resources using FHIR search, extracts the resources from
the returned bundles and outputs one resource per line in NDJSON format.

If the optional resource-type is given, the corresponding type-level
search will be used. Otherwise, the system-level search will be used and
all resources of the whole system will be downloaded. 

The --query flag will take an optional FHIR search query that will be used
to constrain the resources to download.

With the flag --use-post you can ensure that the FHIR search query specified
with --query is send as POST request in the body.

Resources will be either streamed to STDOUT, delimited by newline, or
stored in a file if the --output-file flag is given.

Examples:
  blazectl download --server http://localhost:8080/fhir Patient > all-patients.ndjson
  blazectl download --server http://localhost:8080/fhir Patient -q "gender=female" -o female-patients.ndjson
  blazectl download --server http://localhost:8080/fhir > all-resources.ndjson`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return fhir.ResourceTypes, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := createClient(); err != nil {
			return err
		}
		var stats util.CommandStats
		startTime := time.Now()

		var file *os.File
		if outputFile == "" {
			file = os.Stdout
		} else {
			file = util.CreateOutputFileOrDie(outputFile)
		}
		sink := newOutputSink(file)
		defer file.Close()
		defer file.Sync()

		bundleChannel := make(chan fhir.DownloadBundle, 2)

		var resourceType string
		if len(args) > 0 {
			resourceType = args[0]
		}

		go downloadResources(client, resourceType, fhirSearchQuery, usePost, bundleChannel)

		err := processBundles(bundleChannel, &stats, sink)

		stats.TotalDuration = time.Since(startTime)
		fmt.Fprint(os.Stderr, stats.String())
		if err != nil {
			// All inputs are validated here, so printing the usage would only
			// distract from the error.
			cmd.SilenceUsage = true
			return err
		}
		return nil
	},
}

// outputSinkSize is the buffer size of the output sink. A big buffer reduces
// the number of write syscalls, which dominate the CPU time of downloads.
// Bigger buffers barely reduce the CPU time further, and 64 KiB matches the
// maximum pipe buffer size.
const outputSinkSize = 64 << 10

// newOutputSink returns a buffered writer of outputSinkSize wrapping w.
func newOutputSink(w io.Writer) *bufio.Writer {
	return bufio.NewWriterSize(w, outputSinkSize)
}

// processBundles writes the resources of all bundles received from
// bundleChannel to sink and flushes it. Stops at the first error, still
// flushing the resources written so far. As sink returns its first write
// error again on flush, a flush error already contained in err is omitted.
func processBundles(bundleChannel <-chan fhir.DownloadBundle, stats *util.CommandStats, sink *bufio.Writer) error {
	for bundle := range bundleChannel {
		if err := processBundle(bundle, stats, sink); err != nil {
			if flushErr := sink.Flush(); flushErr != nil && !errors.Is(err, flushErr) {
				return errors.Join(err, flushErr)
			}
			return err
		}
	}
	return sink.Flush()
}

func processBundle(bundle fhir.DownloadBundle, stats *util.CommandStats, sink *bufio.Writer) error {
	stats.TotalPages++

	if bundle.Err != nil || bundle.ErrResponse != nil {
		stats.Error = bundle.ErrResponse
		return fmt.Errorf("failed to download resources: %v", bundle.Err)
	}

	stats.RequestDurations = append(stats.RequestDurations, bundle.Stats.RequestDuration)
	stats.ProcessingDurations = append(stats.ProcessingDurations, bundle.Stats.ProcessingDuration)
	stats.TotalBytesIn += bundle.Stats.TotalBytesIn

	resources, inlineOutcomes, err := fhir.WriteResources(bundle.ResponseBody, sink)
	stats.ResourcesPerPage = append(stats.ResourcesPerPage, resources)
	stats.InlineOperationOutcomes = append(stats.InlineOperationOutcomes, inlineOutcomes...)

	if err != nil {
		return fmt.Errorf("failed to write downloaded resources received from request to URL %s: %w", bundle.AssociatedRequestURL.String(), err)
	}
	return nil
}

// downloadResources tries to download all resources of a given resource type from a FHIR server using
// the given client. Resources that are downloaded can optionally be limited by a given FHIR search query.
// The download respects pagination, i.e. it follows pagination links until there is no other next link.
//
// Downloaded resources as well as errors are sent to a given result channel.
// As soon as an error occurs, it is written to the channel and the channel and closed thereafter.
func downloadResources(client *fhir.Client, resourceType string, fhirSearchQuery string, usePost bool,
	resChannel chan<- fhir.DownloadBundle) {
	defer close(resChannel)

	var query url.Values
	var err error
	if strings.HasPrefix(fhirSearchQuery, "@") {
		query, err = util.ReadQueryFromFile(fhirSearchQuery)
	} else {
		query, err = url.ParseQuery(fhirSearchQuery)
	}
	if err != nil {
		resChannel <- fhir.DownloadBundleError("could not parse the FHIR search query: %v\n", err)
		return
	}

	var request *http.Request
	if usePost {
		request, err = client.NewPostSearchTypeRequest(resourceType, query)
	} else {
		if resourceType == "" {
			request, err = client.NewSearchSystemRequest(query)
		} else {
			request, err = client.NewSearchTypeRequest(resourceType, query)
		}
	}
	if err != nil {
		resChannel <- fhir.DownloadBundleError("could not create FHIR server request: %v\n", err)
		return
	}

	client.ExpandPages(request, resChannel)
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")
	downloadCmd.Flags().StringVarP(&outputFile, "output-file", "o", "", "write to file instead of stdout")
	downloadCmd.Flags().StringVarP(&fhirSearchQuery, "query", "q", "", "FHIR search query. @filename syntax is supported")
	downloadCmd.Flags().BoolVarP(&usePost, "use-post", "p", false, "use POST to execute the search")

	_ = downloadCmd.MarkFlagRequired("server")
	_ = downloadCmd.MarkFlagFilename("output-file", "ndjson")
}
