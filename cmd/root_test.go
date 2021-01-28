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
