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
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	"github.com/spf13/cobra"
)

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

Examples:
  blazectl download-history --server http://localhost:8080/fhir Patient DFRE25Q627JVEWOS > patient-history.ndjson
  blazectl download-history --server http://localhost:8080/fhir Patient > patients-history.ndjson
  blazectl download-history --server http://localhost:8080/fhir > system-history.ndjson`,
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
		sink := bufio.NewWriter(file)
		defer file.Close()
		defer file.Sync()
		defer sink.Flush()

		bundleChannel := make(chan fhir.DownloadBundle, 2)

		var resourceType string
		var resourceId string
		if len(args) > 0 {
			resourceType = args[0]
		}
		if len(args) > 1 {
			resourceId = args[1]
		}

		go downloadHistory(client, resourceType, resourceId, bundleChannel)

		for bundle := range bundleChannel {
			processBundle(bundle, &stats, startTime, sink)
		}

		stats.TotalDuration = time.Since(startTime)
		fmt.Fprint(os.Stderr, stats.String())
		return nil
	},
}

// downloadHistory downloads the history of resources from a FHIR server.
// The history can be at system-level (if resourceType is empty), type-level (if only resourceType
// is provided), or instance-level (if both resourceType and resourceId are provided).
// The download respects pagination, i.e., it follows pagination links until there is no other next link.
//
// Downloaded bundles as well as errors are sent to the given result channel.
// As soon as an error occurs, it is written to the channel and the channel is closed thereafter.
func downloadHistory(client *fhir.Client, resourceType string, resourceId string, resChannel chan<- fhir.DownloadBundle) {
	defer close(resChannel)

	var request *http.Request
	var err error

	if resourceType != "" {
		if resourceId != "" {
			request, err = client.NewHistoryInstanceRequest(resourceType, resourceId)
		} else {
			request, err = client.NewHistoryTypeRequest(resourceType)
		}
	} else {
		request, err = client.NewHistorySystemRequest()
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

	_ = downloadHistoryCmd.MarkFlagRequired("server")
	_ = downloadHistoryCmd.MarkFlagFilename("output-file", "ndjson")
}
