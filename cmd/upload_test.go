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
	"sync"
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
		assert.Contains(t, stderr, "Uploading 1 files (1 JSON files and 0 NDJSON files) ...")
		assert.NotContains(t, stderr, "Inspecting files")
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

	t.Run("prints a message to stderr if there are no files", func(t *testing.T) {
		server := newUploadTestServer(t)

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, t.TempDir())

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Found no files to upload.")
	})

	t.Run("prints a message to stderr if the only file is an empty NDJSON file", func(t *testing.T) {
		server := newUploadTestServer(t)
		dir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "empty.ndjson"), nil, 0644))

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, "--no-progress", dir)

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Uploading 1 files (0 JSON files and 1 NDJSON files) ...")
		assert.Contains(t, stderr, "Found no bundles to upload.")
	})

	t.Run("uploads a multi-bundle NDJSON file with the progress bar", func(t *testing.T) {
		server := newUploadTestServer(t)
		dir := t.TempDir()
		bundles := `{"resourceType":"Bundle","type":"transaction"}` + "\n" +
			`{"resourceType":"Bundle","type":"transaction"}` + "\n"
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "bundles.ndjson"), []byte(bundles), 0644))

		stdout, stderr, err := executeCommandSplit("upload", "--server", server.URL, dir)

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Uploading 1 files (0 JSON files and 1 NDJSON files) ...")
		assert.Contains(t, stderr, "Uploads          [total, concurrency]     2, 2")
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

type recordingProgress struct {
	durations []time.Duration
}

func (r *recordingProgress) increment(d time.Duration) { r.durations = append(r.durations, d) }
func (r *recordingProgress) wait()                     {}

func TestFileProgress(t *testing.T) {
	t.Run("bundles then marker increments once with the sum when the marker arrives", func(t *testing.T) {
		rec := &recordingProgress{}
		fp := newFileProgress(rec)

		fp.bundleDone("a.ndjson", time.Second)
		fp.bundleDone("a.ndjson", 2*time.Second)
		assert.Empty(t, rec.durations)

		fp.fileEnd("a.ndjson", 2)
		assert.Equal(t, []time.Duration{3 * time.Second}, rec.durations)
	})

	t.Run("marker first increments on the last bundle with the sum", func(t *testing.T) {
		rec := &recordingProgress{}
		fp := newFileProgress(rec)

		fp.fileEnd("a.ndjson", 2)
		fp.bundleDone("a.ndjson", time.Second)
		assert.Empty(t, rec.durations)

		fp.bundleDone("a.ndjson", 2*time.Second)
		assert.Equal(t, []time.Duration{3 * time.Second}, rec.durations)
	})

	t.Run("marker with count 0 increments immediately with duration 0", func(t *testing.T) {
		rec := &recordingProgress{}
		fp := newFileProgress(rec)

		fp.fileEnd("empty.ndjson", 0)

		assert.Equal(t, []time.Duration{0}, rec.durations)
	})

	t.Run("interleaved files complete independently", func(t *testing.T) {
		rec := &recordingProgress{}
		fp := newFileProgress(rec)

		fp.bundleDone("a", time.Second)
		fp.fileEnd("b", 2)
		fp.bundleDone("b", 10*time.Second)
		fp.fileEnd("a", 2)
		assert.Empty(t, rec.durations)

		fp.bundleDone("b", 10*time.Second)
		assert.Equal(t, []time.Duration{20 * time.Second}, rec.durations)

		fp.bundleDone("a", time.Second)
		assert.Equal(t, []time.Duration{20 * time.Second, 2 * time.Second}, rec.durations)
	})
}

// drainBundles collects all bundles from ch until it is closed and groups them
// by filename, preserving the order in which they arrived.
func drainBundles(ch <-chan bundle) map[string][]bundle {
	byFile := make(map[string][]bundle)
	for b := range ch {
		byFile[b.id.filename] = append(byFile[b.id.filename], b)
	}
	return byFile
}

func TestUploadBundleProducer(t *testing.T) {
	t.Run("emits one marker per file after all of its bundles with the number of bundles", func(t *testing.T) {
		dir := t.TempDir()
		threeBundles := filepath.Join(dir, "three.ndjson")
		emptyChunks := filepath.Join(dir, "empty.ndjson")
		single := filepath.Join(dir, "single.json")
		for path, content := range map[string]string{
			threeBundles: "{\"a\":1}\n{\"a\":2}\n{\"a\":3}\n",
			emptyChunks:  "\n\n\n",
			single:       "{}",
		} {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}

		byFile := drainBundles(newUploadBundleProducer().createUploadBundles(processableFiles{
			singleBundleFiles: []string{single},
			multiBundleFiles:  []string{threeBundles, emptyChunks},
		}))

		for path, expectedCount := range map[string]int{threeBundles: 3, emptyChunks: 0, single: 1} {
			bundles := byFile[path]
			if assert.Len(t, bundles, expectedCount+1, path) {
				for _, b := range bundles[:expectedCount] {
					assert.False(t, b.endOfFile, path)
				}
				marker := bundles[expectedCount]
				assert.True(t, marker.endOfFile, path)
				assert.Equal(t, expectedCount, marker.bundleCount, path)
			}
		}
	})

	t.Run("emits an error bundle and a marker with count 1 for a file that cannot be opened", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")

		byFile := drainBundles(newUploadBundleProducer().createUploadBundles(processableFiles{
			singleBundleFiles: []string{missing + ".json"},
			multiBundleFiles:  []string{missing + ".ndjson"},
		}))

		for _, path := range []string{missing + ".json", missing + ".ndjson"} {
			bundles := byFile[path]
			if assert.Len(t, bundles, 2, path) {
				assert.Error(t, bundles[0].err, path)
				assert.False(t, bundles[0].endOfFile, path)
				assert.True(t, bundles[1].endOfFile, path)
				assert.Equal(t, 1, bundles[1].bundleCount, path)
			}
		}
	})
}

func TestUploadBundleConsumer(t *testing.T) {
	t.Run("forwards a marker as a result without uploading", func(t *testing.T) {
		resultCh := make(chan bundleUploadResult, 1)
		bundleCh := make(chan bundle, 1)
		bundleCh <- bundle{id: bundleIdentifier{filename: "a.ndjson"}, endOfFile: true, bundleCount: 3}
		close(bundleCh)

		var wg sync.WaitGroup
		newUploadBundleConsumer(nil, resultCh).uploadBundles(bundleCh, 2, &wg)
		wg.Wait()
		close(resultCh)

		results := drainResults(resultCh)
		if assert.Len(t, results, 1) {
			assert.True(t, results[0].endOfFile)
			assert.Equal(t, "a.ndjson", results[0].id.filename)
			assert.Equal(t, 3, results[0].bundleCount)
			assert.NoError(t, results[0].err)
		}
	})
}

func TestAggregateUploadResultsFileProgress(t *testing.T) {
	t.Run("markers drive the progress but are not counted as processed bundles", func(t *testing.T) {
		rec := &recordingProgress{}
		resultCh := make(chan bundleUploadResult, 6)
		aggCh := make(chan aggregatedUploadResults, 1)
		ok := func(filename string) bundleUploadResult {
			return bundleUploadResult{id: bundleIdentifier{filename: filename}, uploadInfo: uploadInfo{statusCode: http.StatusOK}, duration: time.Second}
		}
		marker := func(filename string, count int) bundleUploadResult {
			return bundleUploadResult{id: bundleIdentifier{filename: filename}, endOfFile: true, bundleCount: count}
		}
		for _, r := range []bundleUploadResult{
			ok("a"), ok("b"), marker("b", 1), ok("a"), marker("a", 2), marker("empty", 0),
		} {
			resultCh <- r
		}
		close(resultCh)

		aggregateUploadResults(resultCh, aggCh, newFileProgress(rec))

		assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 0}, rec.durations)
		agg := <-aggCh
		assert.Equal(t, 3, agg.totalProcessedBundles)
		assert.Len(t, agg.processingDurations, 3)
		assert.Len(t, agg.requestDurations, 3)
	})
}

func drainResults(ch <-chan bundleUploadResult) []bundleUploadResult {
	var results []bundleUploadResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}
