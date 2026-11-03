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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFindProcessableFiles(t *testing.T) {

	for _, fileExt := range []string{"json", "json.gz", "json.bz2"} {

		t.Run("dir with one "+fileExt+" file", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "bundles")
			if err != nil {
				t.Fatal("can't create a temp dir")
			}
			defer os.Remove(dir)
			bundlePath := filepath.Join(dir, "bundle."+fileExt)
			err = os.WriteFile(bundlePath, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath)
			files, err := findProcessableFiles(dir)
			if err != nil {
				t.Fatalf("error file filtering processable files %v", err)
			}
			assert.Equal(t, bundlePath, files.singleBundleFiles[0])
		})

		t.Run("dir with two "+fileExt+" files", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "bundles")
			if err != nil {
				t.Fatal("can't create a temp dir")
			}
			defer os.Remove(dir)
			bundlePath1 := filepath.Join(dir, "bundle1."+fileExt)
			err = os.WriteFile(bundlePath1, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath1)
			bundlePath2 := filepath.Join(dir, "bundle2."+fileExt)
			err = os.WriteFile(bundlePath2, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath2)
			files, err := findProcessableFiles(dir)
			if err != nil {
				t.Fatalf("error file filtering processable files %v", err)
			}
			assert.Equal(t, bundlePath1, files.singleBundleFiles[0])
			assert.Equal(t, bundlePath2, files.singleBundleFiles[1])
		})

		t.Run("dir with one "+fileExt+" file and one in a sub dir", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "bundles")
			if err != nil {
				t.Fatal("can't create a temp dir")
			}
			defer os.Remove(dir)
			bundlePath1 := filepath.Join(dir, "bundle1."+fileExt)
			err = os.WriteFile(bundlePath1, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath1)
			subDir, err := os.MkdirTemp(dir, "bundles")
			if err != nil {
				t.Fatal("can't create a temp dir")
			}
			fmt.Println("subDir:", subDir)
			defer os.Remove(subDir)
			bundlePath2 := filepath.Join(subDir, "bundle2."+fileExt)
			err = os.WriteFile(bundlePath2, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath2)
			files, err := findProcessableFiles(dir)
			if err != nil {
				t.Fatalf("error file filtering processable files %v", err)
			}
			assert.Equal(t, bundlePath1, files.singleBundleFiles[0])
			assert.Equal(t, bundlePath2, files.singleBundleFiles[1])
		})
	}

	t.Run("dir with one ndjson file", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "bundles")
		if err != nil {
			t.Fatal("can't create a temp dir")
		}
		defer os.Remove(dir)
		bundlePath := filepath.Join(dir, "bundle.ndjson")
		err = os.WriteFile(bundlePath, []byte("{}"), 0644)
		if err != nil {
			t.Fatal("can't create a temp ndjson file")
		}
		defer os.Remove(bundlePath)
		files, err := findProcessableFiles(dir)
		if err != nil {
			t.Fatalf("error file filtering processable files %v", err)
		}
		assert.Equal(t, bundlePath, files.multiBundleFiles[0])
	})

	t.Run("dir with two ndjson files", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "bundles")
		if err != nil {
			t.Fatal("can't create a temp dir")
		}
		defer os.Remove(dir)
		bundlePath1 := filepath.Join(dir, "bundle1.ndjson")
		err = os.WriteFile(bundlePath1, []byte("{}"), 0644)
		if err != nil {
			t.Fatal("can't create a temp ndjson file")
		}
		defer os.Remove(bundlePath1)
		bundlePath2 := filepath.Join(dir, "bundle2.ndjson")
		err = os.WriteFile(bundlePath2, []byte("{}"), 0644)
		if err != nil {
			t.Fatal("can't create a temp ndjson file")
		}
		defer os.Remove(bundlePath2)
		files, err := findProcessableFiles(dir)
		if err != nil {
			t.Fatalf("error file filtering processable files %v", err)
		}
		assert.Equal(t, bundlePath1, files.multiBundleFiles[0])
		assert.Equal(t, bundlePath2, files.multiBundleFiles[1])
	})
}

// newBundleDir returns a temporary directory containing one transaction bundle.
func newBundleDir(t *testing.T) string {
	dir := t.TempDir()
	bundle := []byte(`{"resourceType":"Bundle","type":"transaction"}`)
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), bundle, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// newUploadTestServer returns a test server that accepts all transactions.
func newUploadTestServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"transaction-response"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestUploadMetaOutput(t *testing.T) {
	t.Run("prints messages and stats to stderr", func(t *testing.T) {
		server := newUploadTestServer(t)

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, "--no-progress", newBundleDir(t))

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Starting Upload to "+server.URL+" ...")
		assert.Contains(t, stderr, "Found 1 bundles in total")
		assert.Contains(t, stderr, "Uploads          [total, concurrency]     1, 2")
		assert.Contains(t, stderr, "Status Codes     [code:count]             200:1")
	})

	t.Run("--silent suppresses messages, stats and the progress bar", func(t *testing.T) {
		server := newUploadTestServer(t)

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, "-s", newBundleDir(t))

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
	})

	t.Run("prints a message to stderr if there are no bundles", func(t *testing.T) {
		server := newUploadTestServer(t)

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, t.TempDir())

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Found no bundles to upload.")
	})

	t.Run("fails with non-OK responses printed to stderr", func(t *testing.T) {
		server := newErrorTestServer(t)

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, "--no-progress", newBundleDir(t))

		assert.EqualError(t, err, "failed to upload 1 of 1 bundles")
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Non-OK Responses:")
		assert.Contains(t, stderr, "StatusCode  : 404")
		assert.Contains(t, stderr, "Error: failed to upload 1 of 1 bundles")
		assert.NotContains(t, stderr, "Usage:")
	})

	t.Run("--silent suppresses non-OK responses", func(t *testing.T) {
		server := newErrorTestServer(t)

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, "-s", newBundleDir(t))

		assert.Error(t, err)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
	})

	t.Run("-sS shows non-OK responses but no stats", func(t *testing.T) {
		server := newErrorTestServer(t)

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, "-sS", newBundleDir(t))

		assert.Error(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Non-OK Responses:")
		assert.Contains(t, stderr, "Error: failed to upload 1 of 1 bundles")
		assert.NotContains(t, stderr, "Uploads")
		assert.NotContains(t, stderr, "Starting Upload")
	})
}

func TestCreateProgress(t *testing.T) {
	t.Run("creates a progress bar by default", func(t *testing.T) {
		progress := createProgress(1, io.Discard)
		progress.increment(time.Millisecond)
		progress.wait()

		assert.IsType(t, realProgress{}, progress)
	})

	for _, flag := range []string{"no-progress", "silent"} {
		t.Run("creates no progress bar with --"+flag, func(t *testing.T) {
			defer resetFlags(rootCmd, flag)
			assert.NoError(t, rootCmd.PersistentFlags().Set(flag, "true"))

			assert.IsType(t, noopProgress{}, createProgress(1, io.Discard))
		})
	}
}
