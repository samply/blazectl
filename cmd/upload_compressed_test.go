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
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// newRecordingUploadTestServer returns a test server that accepts all
// transactions and a function returning the bodies received so far, trimmed of
// the trailing newline delimiter.
func newRecordingUploadTestServer(t *testing.T) (*httptest.Server, func() []string) {
	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mu.Lock()
		bodies = append(bodies, strings.TrimSuffix(string(body), "\n"))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"resourceType":"Bundle","type":"transaction-response"}`))
	}))
	t.Cleanup(server.Close)
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}

// testBundleLines returns n distinct transaction bundles, each on one line.
func testBundleLines(prefix string, n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf(`{"resourceType":"Bundle","type":"transaction","id":"%s-%d"}`, prefix, i)
	}
	return lines
}

// ndjsonContent joins lines with newlines and adds a trailing newline and an
// empty line.
func ndjsonContent(lines []string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n\n")
}

func writeGzipFile(t *testing.T, path string, content []byte) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(content)
	assert.NoError(t, err)
	assert.NoError(t, w.Close())
	assert.NoError(t, os.WriteFile(path, buf.Bytes(), 0644))
}

// copyTestdata copies the file testdata/src to dst.
func copyTestdata(t *testing.T, src string, dst string) {
	content, err := os.ReadFile(filepath.Join("testdata", src))
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(dst, content, 0644))
}

func TestUploadBundleProducerCompressed(t *testing.T) {
	t.Run("emits the content of each non-empty line of a compressed ndjson file in one pass", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a.ndjson.gz")
		writeGzipFile(t, path, []byte("one\n\ntwo\nthree"))

		bundles := drainBundles(newUploadBundleProducer().createUploadBundles(processableFiles{
			multiBundleFiles: []string{path},
		}))[path]

		if assert.Len(t, bundles, 4) {
			for i, expected := range []struct {
				data   string
				number int
			}{{"one", 1}, {"two", 3}, {"three", 4}} {
				assert.NoError(t, bundles[i].err)
				assert.Equal(t, expected.data, string(bundles[i].data))
				assert.Equal(t, expected.number, bundles[i].id.bundleNumber)
			}
			assert.True(t, bundles[3].endOfFile)
			assert.Equal(t, 3, bundles[3].bundleCount)
		}
	})

	t.Run("emits an error bundle after the complete lines of a truncated compressed file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a.ndjson.gz")
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		_, err := w.Write([]byte(strings.Repeat("x", 100) + "\n" + strings.Repeat("y", 100000)))
		assert.NoError(t, err)
		assert.NoError(t, w.Close())
		assert.NoError(t, os.WriteFile(path, buf.Bytes()[:buf.Len()-20], 0644))

		bundles := drainBundles(newUploadBundleProducer().createUploadBundles(processableFiles{
			multiBundleFiles: []string{path},
		}))[path]

		if assert.Len(t, bundles, 3) {
			assert.Equal(t, strings.Repeat("x", 100), string(bundles[0].data))
			assert.Error(t, bundles[1].err)
			assert.Nil(t, bundles[1].data)
			assert.Equal(t, 2, bundles[1].id.bundleNumber)
			assert.True(t, bundles[2].endOfFile)
			assert.Equal(t, 2, bundles[2].bundleCount)
		}
	})
}

func TestUploadCompressedNdjson(t *testing.T) {
	for _, args := range [][]string{{"-c", "1"}, {}} {
		t.Run(fmt.Sprintf("uploads a mixed directory with args %v", args), func(t *testing.T) {
			defer resetFlags(uploadCmd, "concurrency")
			server, received := newRecordingUploadTestServer(t)
			dir := t.TempDir()
			gzLines := testBundleLines("gz", 3)
			plainLines := testBundleLines("plain", 2)
			single := testBundleLines("single", 1)
			writeGzipFile(t, filepath.Join(dir, "a.ndjson.gz"), ndjsonContent(gzLines))
			assert.NoError(t, os.WriteFile(filepath.Join(dir, "b.ndjson"), ndjsonContent(plainLines), 0644))
			writeGzipFile(t, filepath.Join(dir, "c.json.gz"), []byte(single[0]))

			_, stderr, err := executeCommandSplit("upload", append([]string{"--server", server.URL, "--no-progress", dir}, args...)...)

			assert.NoError(t, err)
			assert.Contains(t, stderr, "Uploading 3 files (1 JSON files and 2 NDJSON files) ...")
			assert.Contains(t, stderr, "Uploads          [total, concurrency]     6, ")
			assert.ElementsMatch(t, slices.Concat(gzLines, plainLines, single), received())
		})
	}

	t.Run("uploads the same bundles as the uncompressed file if the file spans several read buffers and lacks a trailing newline", func(t *testing.T) {
		lines := testBundleLines("big", 300)
		content := []byte(strings.Join(lines, "\n"))
		upload := func(write func(dir string)) (string, []string) {
			server, received := newRecordingUploadTestServer(t)
			dir := t.TempDir()
			write(dir)
			_, stderr, err := executeCommandSplit("upload", "--server", server.URL, "--no-progress", dir)
			assert.NoError(t, err)
			return stderr, received()
		}

		plainStderr, plainBodies := upload(func(dir string) {
			assert.NoError(t, os.WriteFile(filepath.Join(dir, "a.ndjson"), content, 0644))
		})
		gzStderr, gzBodies := upload(func(dir string) {
			writeGzipFile(t, filepath.Join(dir, "a.ndjson.gz"), content)
		})

		assert.Contains(t, plainStderr, "Uploads          [total, concurrency]     300, ")
		assert.Contains(t, gzStderr, "Uploads          [total, concurrency]     300, ")
		assert.ElementsMatch(t, lines, plainBodies)
		assert.ElementsMatch(t, lines, gzBodies)
	})

	t.Run("uploads two compressed ndjson files", func(t *testing.T) {
		server, received := newRecordingUploadTestServer(t)
		dir := t.TempDir()
		linesA := testBundleLines("a", 4)
		linesB := testBundleLines("b", 3)
		writeGzipFile(t, filepath.Join(dir, "a.ndjson.gz"), ndjsonContent(linesA))
		writeGzipFile(t, filepath.Join(dir, "b.ndjson.gz"), ndjsonContent(linesB))

		_, _, err := executeCommandSplit("upload", "--server", server.URL, "--no-progress", dir)

		assert.NoError(t, err)
		assert.ElementsMatch(t, append(linesA, linesB...), received())
	})

	for _, name := range []string{"bad.ndjson.gz", "bad.ndjson.bz2"} {
		t.Run("fails with the file name on the corrupt file "+name, func(t *testing.T) {
			server, received := newRecordingUploadTestServer(t)
			dir := t.TempDir()
			assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("not compressed"), 0644))

			_, stderr, err := executeCommandSplit("upload", "--server", server.URL, "--no-progress", dir)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to upload")
			assert.Contains(t, stderr, name)
			assert.Empty(t, received())
		})
	}

	t.Run("uploads all bundles of a bzip2 compressed ndjson file", func(t *testing.T) {
		server, received := newRecordingUploadTestServer(t)
		dir := t.TempDir()
		copyTestdata(t, "bundles.ndjson.bz2", filepath.Join(dir, "bundles.ndjson.bz2"))

		_, stderr, err := executeCommandSplit("upload", "--server", server.URL, "--no-progress", dir)

		assert.NoError(t, err)
		assert.Contains(t, stderr, "Uploads          [total, concurrency]     3, ")
		assert.ElementsMatch(t, testBundleLines("bz2", 3), received())
	})

	t.Run("uploads all bundles of a gzip compressed ndjson file", func(t *testing.T) {
		server, received := newRecordingUploadTestServer(t)
		dir := t.TempDir()
		lines := testBundleLines("gz", 3)
		writeGzipFile(t, filepath.Join(dir, "bundles.ndjson.gz"), ndjsonContent(lines))

		_, stderr, err := executeCommandSplit("upload", "--server", server.URL, "--no-progress", dir)

		assert.NoError(t, err)
		assert.Contains(t, stderr, "Uploads          [total, concurrency]     3, ")
		assert.ElementsMatch(t, lines, received())
	})
}
