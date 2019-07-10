/*
Copyright © 2019 Alexander Kiel <alexander.kiel@life.uni-leipzig.de>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cmd contains all commands of blazectl
package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/life-research/blazectl/fhir"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os"
)

func fetchResourceTypes(client *http.Client, baseUri string) ([]string, error) {
	req, err := http.NewRequest("GET", baseUri+"/metadata", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var m fhir.CapabilityStatement
	if err := json.Unmarshal(body, &m); err != nil {
		fmt.Println(body)
		return nil, err
	}
	resourceTypes := make([]string, 0, 100)
	for _, rest := range m.Rest {
		if rest.Mode == "server" {
			for _, resource := range rest.Resource {
				resourceTypes = append(resourceTypes, resource.Type)
			}
		}
	}
	return resourceTypes, nil
}

func fetchResourceTotal(client *http.Client, baseUri string, resourceType string) (int, error) {
	req, err := http.NewRequest("GET", baseUri+"/"+resourceType+"?_summary=count", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var m fhir.Bundle
	if err := json.Unmarshal(body, &m); err != nil {
		return 0, err
	}
	return m.Total, nil
}

// countResourcesCmd represents the countResources command
var countResourcesCmd = &cobra.Command{
	Use:   "count-resources",
	Short: "Counts all resources by type",
	Long: `Uses the capability statement to detect all resource types supported
on a server and issues an empty search for each resource type with 
_summary=count to count all resources by type.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := &http.Client{}
		resourceTypes, err := fetchResourceTypes(client, server)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		for _, resourceType := range resourceTypes {
			total, err := fetchResourceTotal(client, server, resourceType)
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
