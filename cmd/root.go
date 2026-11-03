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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
)

var server string
var disableTlsSecurity bool
var caCert string
var basicAuthUser string
var basicAuthPassword string
var bearerToken string
var noProgress bool
var silent bool
var showError bool

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

func fetchCapabilityStatement(client *fhir.Client) (fm.CapabilityStatement, error) {
	req, err := client.NewCapabilitiesRequest()
	if err != nil {
		return fm.CapabilityStatement{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fm.CapabilityStatement{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fm.CapabilityStatement{}, fmt.Errorf("Non-OK status while fetching the capability statement: %s", resp.Status)
	}
	return fhir.ReadCapabilityStatement(resp.Body)
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

// metaOut returns the writer for the meta output of cmd, like progress messages
// and stats. Meta output goes to STDERR, so that STDOUT only contains the
// results of cmd. It's discarded in silent mode.
func metaOut(cmd *cobra.Command) io.Writer {
	if silent {
		return io.Discard
	}
	return cmd.ErrOrStderr()
}

// errOut returns the writer for error messages of cmd. It's discarded in silent
// mode, unless --show-error is given.
func errOut(cmd *cobra.Command) io.Writer {
	if silent && !showError {
		return io.Discard
	}
	return cmd.ErrOrStderr()
}

// applySilentMode configures the root command not to print usage in silent
// mode and not to print errors, unless --show-error is given.
func applySilentMode() {
	rootCmd.SilenceUsage = silent
	rootCmd.SilenceErrors = silent && !showError
}

// exitCode returns the exit code for err, which is the code of an
// util.ExitCodeError or 1 otherwise.
func exitCode(err error) int {
	var exitCodeErr *util.ExitCodeError
	if errors.As(err, &exitCodeErr) {
		return exitCodeErr.Code
	}
	return 1
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "blazectl",
	Short: "Control your FHIR® Server from the Command Line",
	Long: `blazectl is a command line tool to control your FHIR® server.

All commands print their results to STDOUT and meta output, like progress,
stats and error messages, to STDERR. Like in curl, the --silent flag
suppresses the meta output, while the exit code still signals failures. Use
--show-error together with --silent to still show error messages.`,
	Version: "1.6.0",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitCode(err))
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&disableTlsSecurity, "insecure", "k", false, "allow insecure server connections when using SSL")
	rootCmd.PersistentFlags().StringVar(&caCert, "certificate-authority", "", "path to a cert file for the certificate authority")
	rootCmd.PersistentFlags().StringVar(&basicAuthUser, "user", "", "user information for basic authentication")
	rootCmd.PersistentFlags().StringVar(&basicAuthPassword, "password", "", "password information for basic authentication")
	rootCmd.PersistentFlags().StringVar(&bearerToken, "token", "", "bearer token for authentication")
	rootCmd.PersistentFlags().BoolVarP(&noProgress, "no-progress", "", false, "don't show progress bar")
	rootCmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "don't show meta output like progress, stats and error messages")
	rootCmd.PersistentFlags().BoolVarP(&showError, "show-error", "S", false, "show error messages even with --silent")

	cobra.OnInitialize(applySilentMode)
}
