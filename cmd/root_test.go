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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samply/blazectl/util"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// executeCommandSplit executes the command with the given name and args and
// returns its stdout and stderr output alongside the error. Resets the global
// flags and the SilenceUsage of the command afterwards.
func executeCommandSplit(name string, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(append([]string{name}, args...))
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		resetFlags(rootCmd, "silent", "show-error", "no-progress")
		if cmd, _, err := rootCmd.Find([]string{name}); err == nil {
			cmd.SilenceUsage = false
		}
	}()

	err := rootCmd.Execute()
	return stdout.String(), stderr.String(), err
}

// executeCommand executes the command with the given name and args and returns
// the combined output alongside the error.
func executeCommand(name string, args ...string) (string, error) {
	stdout, stderr, err := executeCommandSplit(name, args...)
	return stdout + stderr, err
}

// resetFlags resets the flags with the given names of cmd to their defaults.
func resetFlags(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			flag = cmd.PersistentFlags().Lookup(name)
		}
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	}
}

func TestSilentMode(t *testing.T) {
	missingDir := filepath.Join(os.TempDir(), "blazectl-missing-dir")

	t.Run("shows usage and error without --silent", func(t *testing.T) {
		stdout, stderr, err := executeCommandSplit("upload", "--server", "http://localhost:1/fhir", missingDir)

		assert.Error(t, err)
		assert.Contains(t, stdout+stderr, "Usage:")
		assert.Contains(t, stderr, "Error: directory `"+missingDir+"` doesn't exist")
	})

	for _, flag := range []string{"-s", "--silent"} {
		t.Run(flag+" suppresses usage and error", func(t *testing.T) {
			stdout, stderr, err := executeCommandSplit("upload", flag, "--server", "http://localhost:1/fhir", missingDir)

			assert.Error(t, err)
			assert.Empty(t, stdout)
			assert.Empty(t, stderr)
		})
	}

	for _, flags := range [][]string{{"-sS"}, {"--silent", "--show-error"}} {
		t.Run(strings.Join(flags, " ")+" shows error but no usage", func(t *testing.T) {
			stdout, stderr, err := executeCommandSplit("upload", append(flags, "--server", "http://localhost:1/fhir", missingDir)...)

			assert.Error(t, err)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: directory `"+missingDir+"` doesn't exist\n", stderr)
		})
	}

	t.Run("--show-error alone shows usage and error", func(t *testing.T) {
		stdout, stderr, err := executeCommandSplit("upload", "-S", "--server", "http://localhost:1/fhir", missingDir)

		assert.Error(t, err)
		assert.Contains(t, stdout+stderr, "Usage:")
		assert.Contains(t, stderr, "Error: directory")
	})
}

func TestMetaOut(t *testing.T) {
	cmd := &cobra.Command{}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	defer resetFlags(rootCmd, "silent", "show-error")

	for _, test := range []struct {
		flags []string
		meta  string
		err   string
	}{
		{nil, "meta\n", "error\n"},
		{[]string{"--show-error"}, "meta\n", "error\n"},
		{[]string{"--silent"}, "", ""},
		{[]string{"--silent", "--show-error"}, "", "error\n"},
	} {
		t.Run(strings.Join(test.flags, " "), func(t *testing.T) {
			resetFlags(rootCmd, "silent", "show-error")
			assert.NoError(t, rootCmd.PersistentFlags().Parse(test.flags))

			stderr.Reset()
			fmt.Fprintln(metaOut(cmd), "meta")
			assert.Equal(t, test.meta, stderr.String())

			stderr.Reset()
			fmt.Fprintln(errOut(cmd), "error")
			assert.Equal(t, test.err, stderr.String())
		})
	}
}

func TestExitCode(t *testing.T) {
	t.Run("is 1 for plain errors", func(t *testing.T) {
		assert.Equal(t, 1, exitCode(errors.New("foo")))
	})

	t.Run("is the code of an ExitCodeError", func(t *testing.T) {
		err := &util.ExitCodeError{Code: 3, Err: errors.New("foo")}

		assert.Equal(t, 3, exitCode(fmt.Errorf("wrapped: %w", err)))
	})
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
