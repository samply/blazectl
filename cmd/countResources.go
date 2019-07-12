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
	"fmt"
	"github.com/life-research/blazectl/fhir"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"sort"
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

func fetchResourceTotal(client *fhir.Client, resourceType string) (int, error) {
	req, err := client.NewSearchTypeRequest(resourceType)
	if err != nil {
		return 0, err
	}
	req.URL.RawQuery = "_summary=count"
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		bundle, err := fhir.ReadBundle(resp.Body)
		if err != nil {
			return 0, err
		}
		return bundle.Total, nil
	}
	return 0, fmt.Errorf("Non-OK status while performing a summary count search on resource type `%s`: %s",
		resourceType, resp.Status)
}

type result struct {
	resourceType string
	count        int
	err          error
}

func max(counts map[string]int) (maxResourceTypeLen int, maxCount int) {
	for resourceType, count := range counts {
		if len(resourceType) > maxResourceTypeLen {
			maxResourceTypeLen = len(resourceType)
		}
		if count > maxCount {
			maxCount = count
		}
	}
	return maxResourceTypeLen, len(fmt.Sprintf("%d", maxCount))
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

		counts := make(map[string]int)
		errors := make(map[string]error)
		finished := make(chan bool)
		resultCh := make(chan result)
		go func() {
			for result := range resultCh {
				if result.err != nil {
					errors[result.resourceType] = result.err
				} else if result.count != 0 {
					counts[result.resourceType] = result.count
				}
			}
			finished <- true
		}()

		sem := make(chan bool, concurrency)
		for _, resourceType := range resourceTypes {
			sem <- true
			go func(resourceType string) {
				defer func() { <-sem }()
				total, err := fetchResourceTotal(client, resourceType)
				resultCh <- result{
					resourceType: resourceType,
					count:        total,
					err:          err,
				}
			}(resourceType)
		}

		// Wait for all uploads to finish
		for i := 0; i < cap(sem); i++ {
			sem <- true
		}
		close(resultCh)
		client.CloseIdleConnections()

		<-finished

		resourceTypes = make([]string, 0, len(counts))
		for resourceType := range counts {
			resourceTypes = append(resourceTypes, resourceType)
		}
		sort.Strings(resourceTypes)
		maxResourceTypeLen, maxCount := max(counts)
		for _, resourceType := range resourceTypes {
			fmt.Printf("%-"+fmt.Sprintf("%d", maxResourceTypeLen)+"s : %"+fmt.Sprintf("%d", maxCount)+"d\n",
				resourceType, counts[resourceType])
		}

		resourceTypes = make([]string, 0, len(errors))
		for resourceType := range errors {
			resourceTypes = append(resourceTypes, resourceType)
		}
		sort.Strings(resourceTypes)
		if len(resourceTypes) > 0 {
			fmt.Println("\nErrors:")
			for _, resourceType := range resourceTypes {
				fmt.Printf("%-33s %s\n", resourceType, errors[resourceType])
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(countResourcesCmd)

	countResourcesCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 2, "number of parallel searches")
}
