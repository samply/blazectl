// Copyright © 2019 Alexander Kiel <alexander.kiel@life.uni-leipzig.de>
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
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"sort"
	"strings"
)

func fetchResourceTypesWithSearchTypeInteraction(client *fhir.Client) ([]string, error) {
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
		resourceTypes := make([]string, 0, 100)
		for _, rest := range capabilityStatement.Rest {
			if rest.Mode == "server" {
				for _, resource := range rest.Resource {
					if resource.DoesSupportsInteraction("search-type") {
						resourceTypes = append(resourceTypes, resource.Type)
					}
				}
			}
		}
		return resourceTypes, nil
	}
	return nil, fmt.Errorf("Non-OK status while fetching the capability statement: %s", resp.Status)
}

func fetchResourcesTotal(client *fhir.Client, resourceTypes []string) (map[string]int, error) {
	entries := make([]fhir.BundleEntry, 0, 100)
	for _, resourceType := range resourceTypes {
		entries = append(entries, fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Method: "GET",
				URL:    resourceType + "?_summary=count",
			},
		})
	}
	bundle := fhir.Bundle{
		Type:  "batch",
		Entry: entries,
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	req, err := client.NewBatchRequest(bytes.NewReader(payload))
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
		counts := make(map[string]int)
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
			searchset, err := fhir.UnmarshalBundle(entry.Resource)
			if err != nil {
				return nil, err
			}
			if searchset.Total != nil {
				counts[resourceTypes[i]] = *searchset.Total
			}
		}
		return counts, nil
	}
	return nil, fmt.Errorf("non-OK status while performing a batch interaction: %s", resp.Status)
}

func max(counts map[string]int) (maxResourceTypeLen int, total int) {
	for resourceType, count := range counts {
		if len(resourceType) > maxResourceTypeLen {
			maxResourceTypeLen = len(resourceType)
		}
		total += count
	}
	return maxResourceTypeLen, total
}

// countResourcesCmd represents the countResources command
var countResourcesCmd = &cobra.Command{
	Use:   "count-resources",
	Short: "Counts all resources by type",
	Long: `Uses the capability statement to detect all resource types supported
on a server and issues an empty search for each resource type with 
_summary=count to count all resources by type.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Count all resources on %s ...\n\n", server)

		client := &fhir.Client{Base: server}
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

		resourceTypes = make([]string, 0, len(counts))
		for resourceType := range counts {
			resourceTypes = append(resourceTypes, resourceType)
		}
		sort.Strings(resourceTypes)
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
	},
}

func init() {
	rootCmd.AddCommand(countResourcesCmd)
}
