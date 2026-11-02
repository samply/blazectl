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
	"testing"
)

// executeCommand executes the command with the given name and args and returns
// the combined output alongside the error.
func executeCommand(name string, args ...string) (string, error) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(append([]string{name}, args...))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	err := rootCmd.Execute()
	return out.String(), err
}

func TestCreateClient(t *testing.T) {
	t.Run("FailsWithInvalidUrl", func(t *testing.T) {
		server = "invalid-url"
		if err := createClient(); err == nil {
			t.Fatal("Expected the command to fail if an invalid URL is provided as a server information.")
		}
	})

	t.Run("SucceedsWithValidUrl", func(t *testing.T) {
		server = "localhost:9200"
		if err := createClient(); err != nil {
			t.Fatal("Expected the command to succeed if a valid URL is provided as a server information.")
		}
	})
}
