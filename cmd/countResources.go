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
	"encoding/json"
	"fmt"
	"github.com/life-research/blazectl/fhir"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
)

func fetchResourceTypes(client *fhir.Client) ([]string, error) {
	req, err := client.NewCapabilitiesRequest()
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var capabilityStatement fhir.CapabilityStatement
	if err := json.Unmarshal(body, &capabilityStatement); err != nil {
		return nil, err
	}
	resourceTypes := make([]string, 0, 100)
	for _, rest := range capabilityStatement.Rest {
		if rest.Mode == "server" {
			for _, resource := range rest.Resource {
				resourceTypes = append(resourceTypes, resource.Type)
			}
		}
	}
	return resourceTypes, nil
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
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var bundle fhir.Bundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return 0, err
	}
	return bundle.Total, nil
}

// countResourcesCmd represents the countResources command
var countResourcesCmd = &cobra.Command{
	Use:   "count-resources",
	Short: "Counts all resources by type",
	Long: `Uses the capability statement to detect all resource types supported
on a server and issues an empty search for each resource type with 
_summary=count to count all resources by type.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := &fhir.Client{Base: server}
		resourceTypes, err := fetchResourceTypes(client)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		for _, resourceType := range resourceTypes {
			total, err := fetchResourceTotal(client, resourceType)
			if err != nil {
				fmt.Printf("%-33s : %s\n", resourceType, err)
			} else if total != 0 {
				fmt.Printf("%-33s : %d\n", resourceType, total)
			}
		}
		client.CloseIdleConnections()
	},
}

func init() {
	rootCmd.AddCommand(countResourcesCmd)
}
