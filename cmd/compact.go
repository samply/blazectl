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
	"github.com/samply/blazectl/fhir"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"slices"
	"strings"
)

var databases = []string{"index", "transaction", "resource"}
var indexColumnFamilies = []string{
	"search-param-value-index",
	"resource-value-index",
	"compartment-search-param-value-index",
	"compartment-resource-type-index",
	"active-search-params",
	"tx-success-index",
	"tx-error-index",
	"t-by-instant-index",
	"resource-as-of-index",
	"type-as-of-index",
	"system-as-of-index",
	"patient-last-change-index",
	"type-stats-index",
	"system-stats-index",
	"cql-bloom-filter",
	"cql-bloom-filter-by-t",
}
var otherColumnFamilies = []string{"default"}

var compactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Compact a Database Column Family",
	Long:  "Initiates compaction of a column family of a RocksDB database.",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return databases, cobra.ShellCompDirectiveNoFileComp
		case 1:
			switch args[0] {
			case "index":
				return indexColumnFamilies, cobra.ShellCompDirectiveNoFileComp
			default:
				return otherColumnFamilies, cobra.ShellCompDirectiveNoFileComp
			}
		default:
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("requires exactly 2 arguments: database and column-family")
		}
		switch args[0] {
		case "index":
			if !slices.Contains(indexColumnFamilies, args[1]) {
				return fmt.Errorf("invalid column family. Must be one of: %s", strings.Join(indexColumnFamilies, ", "))
			}
		default:
			if args[1] != "default" {
				return fmt.Errorf("invalid column family. Must be: default")
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := createClient()
		if err != nil {
			return err
		}

		req, err := client.NewPostSystemOperationRequest("compact", true, createParameters(args[0], args[1]))
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 202 {
			contentLocation := resp.Header.Get("Content-Location")
			if err := fhir.DiscardAndClose(resp.Body); err != nil {
				return err
			}
			interruptChan := make(chan os.Signal, 1)
			signal.Notify(interruptChan, os.Interrupt)
			_, err := client.PollAsyncStatus(contentLocation, interruptChan)
			if err != nil {
				return err
			}
			fmt.Printf("Successfully compacted column family `%s` in database `%s`.\n", args[1], args[0])
		} else {
			fmt.Println("Error while compacting.")
		}

		return nil
	},
}

func createParameters(database string, columnFamily string) fm.Parameters {
	return fm.Parameters{
		Parameter: []fm.ParametersParameter{
			{
				Name:      "database",
				ValueCode: &database,
			},
			{
				Name:      "column-family",
				ValueCode: &columnFamily,
			},
		},
	}
}

func init() {
	rootCmd.AddCommand(compactCmd)

	compactCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")

	_ = compactCmd.MarkFlagRequired("server")
}
