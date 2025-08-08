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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"time"

	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
)

// networkStats describes network statistics that arise when downloading resources from
// a FHIR server.

var downloadHistoryCmd = &cobra.Command{
	Use:   "download-history [resource-type [resource-id]]",
	Short: "Download history in NDJSON format",
	Long: `Downloads history using FHIR search, extracts the history from
the returned bundles and outputs one resource per line in NDJSON format.

If the optional resource-type is given, the corresponding type-level
history will pulled. Otherwise, the system-level search will be used and
all resources of the whole system will be downloaded. 

Resources will be either streamed to STDOUT, delimited by newline, or
stored in a file if the --output-file flag is given.

Examples:
  blazectl download-history --server http://localhost:8080/fhir Patient DFRE25Q627JVEWOS > patient-history.ndjson
  blazectl download-history --server http://localhost:8080/fhir Patient > patients-history.ndjson
  blazectl download-history --server http://localhost:8080/fhir > system-history.ndjson`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return resourceTypes, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := createClient()
		if err != nil {
			return err
		}
		var stats commandStats
		startTime := time.Now()

		var file *os.File
		if outputFile == "" {
			file = os.Stdout
		} else {
			file = createOutputFileOrDie(outputFile)
		}
		sink := bufio.NewWriter(file)
		defer file.Close()
		defer file.Sync()
		defer sink.Flush()

		bundleChannel := make(chan downloadBundle, 2)

		var resourceType string
		var resourceId string
		if len(args) > 0 {
			resourceType = args[0]
		} else {
			resourceType = ""
			resourceId = ""
		}
		if len(args) > 1 {
			resourceId = args[1]
		}

		go downloadHistory(client, resourceType, resourceId, bundleChannel)

		for bundle := range bundleChannel {
			stats.totalPages++

			if bundle.err != nil || bundle.errResponse != nil {
				fmt.Printf("Failed to download resources: %v\n", bundle.err)

				stats.error = bundle.errResponse
				stats.totalDuration = time.Since(startTime)
				fmt.Println(stats.String())
				os.Exit(1)
			} else {
				stats.requestDurations = append(stats.requestDurations, bundle.stats.requestDuration)
				stats.processingDurations = append(stats.processingDurations, bundle.stats.processingDuration)
				stats.totalBytesIn += bundle.stats.totalBytesIn

				resources, inlineOutcomes, err := writeResources(bundle.responseBody, sink)
				stats.resourcesPerPage = append(stats.resourcesPerPage, resources)
				stats.inlineOperationOutcomes = append(stats.inlineOperationOutcomes, inlineOutcomes...)

				if err != nil {
					fmt.Printf("Failed to write downloaded resources received from request to URL %s: %v\n", bundle.associatedRequestURL.String(), err)
					os.Exit(2)
				}
			}
		}

		stats.totalDuration = time.Since(startTime)
		fmt.Fprint(os.Stderr, stats.String())
		return nil
	},
}

// downloadResources tries to download all resources of a given resource type from a FHIR server using
// the given client. Resources that are downloaded can optionally be limited by a given FHIR search query.
// The download respects pagination, i.e. it follows pagination links until there is no other next link.
//
// Downloaded resources as well as errors are sent to a given result channel.
// As soon as an error occurs, it is written to the channel and the channel and closed thereafter.
func downloadHistory(client *fhir.Client, resourceType string, resourceId string, resChannel chan<- downloadBundle) {
	defer close(resChannel)

	var requestStart time.Time
	var processingStart time.Time
	var request *http.Request
	var nextPageURL *url.URL
	var err error

	for ok := true; ok; ok = nextPageURL != nil {
		var stats networkStats

		if request == nil {
			if resourceType != "" {
				if resourceId != "" {
					request, err = client.NewHistoryResourceRequest(resourceType, resourceId)
				} else {
					request, err = client.NewHistoryTypeRequest(resourceType)
				}
			} else {
				request, err = client.NewHistorySystemRequest()
			}
		} else {
			request, err = client.NewPaginatedRequest(nextPageURL)
		}
		if err != nil {
			resChannel <- downloadBundleError("could not create FHIR server request: %v\n", err)
			return
		}

		trace := &httptrace.ClientTrace{
			GotConn: func(_ httptrace.GotConnInfo) {
				requestStart = time.Now()
			},
			WroteRequest: func(_ httptrace.WroteRequestInfo) {
				processingStart = time.Now()
			},
			GotFirstResponseByte: func() {
				stats.processingDuration = time.Since(processingStart).Seconds()
			},
		}
		request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))

		response, err := client.Do(request)
		if err != nil {
			resChannel <- downloadBundleError("could not request the FHIR server with URL %s: %v\n", request.URL, err)
			return
		}

		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			resChannel <- downloadBundleError("could not read FHIR server response after request to URL %s: %v\n", request.URL, err)
			return
		}
		if err := response.Body.Close(); err != nil {
			resChannel <- downloadBundleError("could not close the response body: %v\n", err)
			return
		}
		stats.requestDuration = time.Since(requestStart).Seconds()
		stats.totalBytesIn += int64(len(responseBody))

		if response.StatusCode != http.StatusOK {
			outcome, err := fm.UnmarshalOperationOutcome(responseBody)
			if err != nil {
				bundle := downloadBundleError("request to FHIR server with URL %s had a non-ok response status (%d) but the expected operation outcome could not be parsed: %v", request.URL, response.StatusCode, err)
				bundle.stats = &stats
				resChannel <- bundle
				return
			}

			bundle := downloadBundleError("request to FHIR server with URL %s had a non-ok response status (%d)", request.URL, response.StatusCode)
			bundle.errResponse = &util.ErrorResponse{
				StatusCode:       response.StatusCode,
				OperationOutcome: &outcome,
			}
			bundle.stats = &stats
			resChannel <- bundle
			return
		}

		if linkHeader := response.Header.Get("Link"); linkHeader != "" {
			nextLink, err := getNextLink(linkHeader)
			if err != nil {
				resChannel <- downloadBundleError("could not parse the self link from the Link header after request to URL %s: %v", request.URL, err)
				return
			}

			resChannel <- downloadBundle{
				associatedRequestURL: *request.URL,
				responseBody:         responseBody,
				stats:                &stats,
			}

			nextPageURL = nextLink
		} else {
			var bundle linkBundle
			if err := json.Unmarshal(responseBody, &bundle); err != nil {
				resChannel <- downloadBundleError("could not parse FHIR server response after request to URL %s: %v\n", request.URL, err)
				return
			}
			resChannel <- downloadBundle{
				associatedRequestURL: *request.URL,
				responseBody:         responseBody,
				stats:                &stats,
			}

			nextPageURL, err = getNextPageURL(bundle.Link)
			if err != nil {
				resChannel <- downloadBundleError("could not parse the next page link within the FHIR server response after request to URL %s: %v\n", request.URL, err)
				return
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(downloadHistoryCmd)

	downloadHistoryCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")
	downloadHistoryCmd.Flags().StringVarP(&outputFile, "output-file", "o", "", "write to file instead of stdout")

	_ = downloadHistoryCmd.MarkFlagRequired("server")
	_ = downloadHistoryCmd.MarkFlagFilename("output-file", "ndjson")
}
