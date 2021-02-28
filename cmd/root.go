// Copyright 2019 The Samply Development Community
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
	"github.com/spf13/cobra"
	"net/url"
	"os"
)

var server string
var basicAuthUser string
var basicAuthPassword string

var client *fhir.Client

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "blazectl",
	Short: "Control your FHIR® Server from the Command Line",
	Long: `blazectl is a command line tool to control your FHIR® server.

Currently you can upload transaction bundles from a directory and count resources.`,
	Version: "0.5.0",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		fhirServerBaseUrl, err := url.ParseRequestURI(server)
		if err != nil {
			return fmt.Errorf("could not parse server's base URL: %v", err)
		}

		clientAuth := fhir.ClientAuth{BasicAuthUser: basicAuthUser, BasicAuthPassword: basicAuthPassword}
		client = fhir.NewClient(*fhirServerBaseUrl, clientAuth)
		return nil
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&server, "server", "", "the base URL of the server to use")
	rootCmd.PersistentFlags().StringVar(&basicAuthUser, "user", "", "user information for basic authentication")
	rootCmd.PersistentFlags().StringVar(&basicAuthPassword, "password", "", "password information for basic authentication")

	_ = rootCmd.MarkPersistentFlagRequired("server")
}
