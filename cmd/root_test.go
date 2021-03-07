// Copyright 2019 - 2021 The Samply Community
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
	"github.com/spf13/cobra"
	"io/ioutil"
	"testing"
)

func TestRootCmd_InvalidServerAddress(t *testing.T) {
	rootCmd.SetOut(ioutil.Discard)
	rootCmd.SetArgs([]string{"--server", "invalid-url"})
	rootCmd.Run = func(cmd *cobra.Command, args []string) {}
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("Expected the command to fail if an invalid URL is provided as a server information.")
	}
}

func TestRootCmd_ValidServerAddress(t *testing.T) {
	rootCmd.SetOut(ioutil.Discard)
	rootCmd.SetArgs([]string{"--server", "localhost:9200"})
	rootCmd.Run = func(cmd *cobra.Command, args []string) {}
	if err := rootCmd.Execute(); err != nil {
		t.Fatal("Expected the command to succeed if a valid URL is provided as a server information.")
	}
}
