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
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/samply/blazectl/fhir"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"sort"
	"strings"
)

func fetchResourceTypesWithSearchTypeInteraction(client *fhir.Client) ([]fm.ResourceType, error) {
	req, err := client.NewCapabilitiesRequest()
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		capabilityStatement, err := fhir.ReadCapabilityStatement(resp.Body)
		if err != nil {
			return nil, err
		}
		return extractResourceTypesWithSearchTypeInteraction(capabilityStatement), nil
	}
	return nil, fmt.Errorf("Non-OK status while fetching the capability statement: %s", resp.Status)
}

func extractResourceTypesWithSearchTypeInteraction(capabilityStatement fm.CapabilityStatement) []fm.ResourceType {
	resourceTypes := make([]fm.ResourceType, 0, 100)
	for _, rest := range capabilityStatement.Rest {
		if rest.Mode == fm.RestfulCapabilityModeServer {
			for _, resource := range rest.Resource {
				if fhir.DoesSupportsInteraction(resource, fm.TypeRestfulInteractionSearchType) {
					resourceTypes = append(resourceTypes, resource.Type)
				}
			}
		}
	}
	return resourceTypes
}

func fetchResourcesTotal(client *fhir.Client, resourceTypes []fm.ResourceType) (map[fm.ResourceType]int, error) {
	bundle := buildCountBundle(resourceTypes)
	payload, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}

	req, err := client.NewTransactionRequest(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		batchResponse, err := fhir.ReadBundle(resp.Body)
		if err != nil {
			return nil, err
		}
		if len(batchResponse.Entry) != len(resourceTypes) {
			return nil, fmt.Errorf("expect %d bundle entries but got %d",
				len(resourceTypes), len(batchResponse.Entry))
		}
		return extractTotalCounts(batchResponse, resourceTypes)
	}
	return nil, fmt.Errorf("non-OK status while performing a batch interaction: %s", resp.Status)
}

func buildCountBundle(resourceTypes []fm.ResourceType) fm.Bundle {
	entries := make([]fm.BundleEntry, 0, 100)
	for _, resourceType := range resourceTypes {
		entries = append(entries, fm.BundleEntry{
			Request: &fm.BundleEntryRequest{
				Method: fm.HTTPVerbGET,
				Url:    resourceType.Code() + "?_summary=count",
			},
		})
	}
	return fm.Bundle{
		Type:  fm.BundleTypeBatch,
		Entry: entries,
	}
}

func extractTotalCounts(batchResponse fm.Bundle, resourceTypes []fm.ResourceType) (map[fm.ResourceType]int, error) {
	counts := make(map[fm.ResourceType]int)
	for i, entry := range batchResponse.Entry {
		if entry.Response == nil {
			return nil, fmt.Errorf("missing response in entry with index %d", i)
		}
		if !strings.HasPrefix(entry.Response.Status, "200") {
			return nil, fmt.Errorf("unexpected response status code %s in entry with index %d",
				entry.Response.Status, i)
		}
		if entry.Resource == nil {
			return nil, fmt.Errorf("missing resource in entry with index %d", i)
		}
		searchsetBundle, err := fm.UnmarshalBundle(entry.Resource)
		if err != nil {
			return nil, err
		}
		if searchsetBundle.Total != nil {
			counts[resourceTypes[i]] = *searchsetBundle.Total
		}
	}
	return counts, nil
}

// countResourcesCmd represents the countResources command
var countResourcesCmd = &cobra.Command{
	Use:   "count-resources",
	Short: "Counts all resources by type",
	Long: `Uses the capability statement to detect all resource types supported
on a server and issues an empty search for each resource type with 
_summary=count to count all resources by type.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := createClient()
		if err != nil {
			return err
		}
		fmt.Printf("Count all resources on %s ...\n\n", server)

		resourceTypes, err := fetchResourceTypesWithSearchTypeInteraction(client)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		counts, err := fetchResourcesTotal(client, resourceTypes)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		client.CloseIdleConnections()

		resourceTypeCodes := make([]string, 0, len(counts))
		for resourceType := range counts {
			resourceTypeCodes = append(resourceTypeCodes, resourceType.Code())
		}
		sort.Strings(resourceTypeCodes)
		maxResourceTypeLen, total := max(counts)
		maxCount := len(fmt.Sprintf("%d", total))
		format := "%-" + fmt.Sprintf("%d", maxResourceTypeLen) + "s : %" + fmt.Sprintf("%d", maxCount) + "d\n"
		for _, resourceType := range resourceTypes {
			if counts[resourceType] != 0 {
				fmt.Printf(format, resourceType, counts[resourceType])
			}
		}
		bar := ""
		for i := 0; i < maxResourceTypeLen+maxCount+3; i++ {
			bar += "-"
		}
		fmt.Println(bar)
		fmt.Printf(format, "total", total)
		return nil
	},
}

func max(counts map[fm.ResourceType]int) (maxResourceTypeLen int, total int) {
	for resourceType, count := range counts {
		if len(resourceType.Code()) > maxResourceTypeLen {
			maxResourceTypeLen = len(resourceType.Code())
		}
		total += count
	}
	return maxResourceTypeLen, total
}

func init() {
	rootCmd.AddCommand(countResourcesCmd)

	countResourcesCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")

	_ = countResourcesCmd.MarkFlagRequired("server")
}
