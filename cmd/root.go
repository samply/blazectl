// Copyright 2019 - 2023 The Samply Community
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
var disableTlsSecurity bool
var caCert string
var basicAuthUser string
var basicAuthPassword string
var bearerToken string
var noProgress bool

var client *fhir.Client

func createClient() error {
	fhirServerBaseUrl, err := url.ParseRequestURI(server)
	if err != nil {
		return fmt.Errorf("could not parse server's base URL: %v", err)
	}

	if disableTlsSecurity {
		client = fhir.NewClientInsecure(*fhirServerBaseUrl, clientAuth())
	} else if caCert != "" {
		client, err = fhir.NewClientCa(*fhirServerBaseUrl, clientAuth(), caCert)
		if err != nil {
			return err
		}
	} else {
		client = fhir.NewClient(*fhirServerBaseUrl, clientAuth())
	}
	return nil
}

func clientAuth() fhir.Auth {
	if basicAuthUser != "" && basicAuthPassword != "" {
		return fhir.BasicAuth{User: basicAuthUser, Password: basicAuthPassword}
	} else if bearerToken != "" {
		return fhir.TokenAuth{Token: bearerToken}
	} else {
		return nil
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "blazectl",
	Short: "Control your FHIR® Server from the Command Line",
	Long: `blazectl is a command line tool to control your FHIR® server.

Currently you can upload transaction bundles from a directory, download
and count resources and evaluate measures.`,
	Version: "0.15.0",
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
	rootCmd.PersistentFlags().BoolVarP(&disableTlsSecurity, "insecure", "k", false, "allow insecure server connections when using SSL")
	rootCmd.PersistentFlags().StringVar(&caCert, "certificate-authority", "", "path to a cert file for the certificate authority")
	rootCmd.PersistentFlags().StringVar(&basicAuthUser, "user", "", "user information for basic authentication")
	rootCmd.PersistentFlags().StringVar(&basicAuthPassword, "password", "", "password information for basic authentication")
	rootCmd.PersistentFlags().StringVar(&bearerToken, "token", "", "bearer token for authentication")
	rootCmd.PersistentFlags().BoolVarP(&noProgress, "no-progress", "", false, "don't show progress bar")
}
