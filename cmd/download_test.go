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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
)

func TestDownloadResources(t *testing.T) {

	t.Run("RequestToFHIRServerFails", func(t *testing.T) {
		baseURL, _ := url.ParseRequestURI("http://localhost")
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadResources(client, "foo", url.Values{}, false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.NotNil(t, bundle.Err)
		}
		assert.Equal(t, 1, bundles)
	})

	t.Run("ErrorReadingResponseBody", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simply do not respond with anything
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadResources(client, "foo", url.Values{}, false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.Err)
			_, _, err := bundle.WriteResources(io.Discard)
			assert.NotNil(t, err)
		}
		assert.Equal(t, 1, bundles)
	})

	t.Run("InvalidFHIRBundleResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("{}"))
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadResources(client, "foo", url.Values{}, false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.Err)
			_, _, err := bundle.WriteResources(io.Discard)
			assert.NoError(t, err)
		}
		assert.Equal(t, 1, bundles)
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := fm.OperationOutcome{
				Issue: []fm.OperationOutcomeIssue{{
					Severity: fm.IssueSeverityError,
					Code:     fm.IssueTypeNotFound,
				}},
			}

			w.WriteHeader(http.StatusNotFound)
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadResources(client, "foo", url.Values{}, false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.NotNil(t, bundle.Err)
			assert.NotNil(t, bundle.ErrResponse)
			assert.NotNil(t, bundle.Stats)
		}
		assert.Equal(t, 1, bundles)
	})

	t.Run("ResponseWithOperationOutcomeEntry", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			total := 1
			searchModeA := fm.SearchEntryModeMatch
			searchModeB := fm.SearchEntryModeOutcome

			outcome := fm.OperationOutcome{
				Issue: []fm.OperationOutcomeIssue{{
					Severity: fm.IssueSeverityWarning,
					Code:     fm.IssueTypeTooLong,
				}},
			}

			outcomeBuf := bytes.NewBufferString("")
			outcomeEncoder := json.NewEncoder(outcomeBuf)
			_ = outcomeEncoder.Encode(outcome)

			patient := fm.Patient{}

			patientBuf := bytes.NewBufferString("")
			patientEncoder := json.NewEncoder(patientBuf)
			_ = patientEncoder.Encode(patient)

			response := fm.Bundle{
				Type:  fm.BundleTypeSearchset,
				Total: &total,
				Entry: []fm.BundleEntry{{
					Resource: patientBuf.Bytes(),
					Search: &fm.BundleEntrySearch{
						Mode: &searchModeA,
					},
				},
					{
						Resource: outcomeBuf.Bytes(),
						Search: &fm.BundleEntrySearch{
							Mode: &searchModeB,
						},
					}},
			}

			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadResources(client, "foo", url.Values{}, false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.Err)
			assert.Nil(t, bundle.ErrResponse)
			_, _, err := bundle.WriteResources(io.Discard)
			assert.NoError(t, err)
			assert.NotNil(t, bundle.Stats)
		}
		assert.Equal(t, 1, bundles)
	})

	t.Run("SinglePageResponse", func(t *testing.T) {
		var requestCounter int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCounter++
			total := 1
			searchMode := fm.SearchEntryModeMatch
			response := fm.Bundle{
				Type:  fm.BundleTypeSearchset,
				Total: &total,
				Entry: []fm.BundleEntry{{
					Resource: []byte("{\"foo\": \"bar\"}"),
					Search: &fm.BundleEntrySearch{
						Mode: &searchMode,
					},
				}},
			}

			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadResources(client, "foo", url.Values{}, false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.Err)
			assert.Nil(t, bundle.ErrResponse)
			_, _, err := bundle.WriteResources(io.Discard)
			assert.NoError(t, err)
			assert.NotNil(t, bundle.Stats)
		}
		assert.Equal(t, 1, bundles)
		assert.Equal(t, 1, requestCounter)
	})

	t.Run("MultiPageResponse without link Header", func(t *testing.T) {
		listen, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			t.Errorf("could not create listener for test server: %v\n", err)
		}

		testServerURL := fmt.Sprintf("http://%s", listen.Addr())

		var requestCounter int
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			total := 2
			searchMode := fm.SearchEntryModeMatch
			var response fm.Bundle

			if requestCounter == 0 {
				response = fm.Bundle{
					Type:  fm.BundleTypeSearchset,
					Total: &total,
					Entry: []fm.BundleEntry{{
						Resource: []byte("{\"foo\": \"bar\"}"),
						Search: &fm.BundleEntrySearch{
							Mode: &searchMode,
						},
					}},
					Link: []fm.BundleLink{
						{
							Relation: "self",
							Url:      "something",
						},
						{
							Relation: "next",
							Url:      fmt.Sprintf("%s/something-else", testServerURL),
						},
					},
				}
			} else {
				response = fm.Bundle{
					Type:  fm.BundleTypeSearchset,
					Total: &total,
					Entry: []fm.BundleEntry{{
						Resource: []byte("{\"foobar\": \"baz\"}"),
						Search: &fm.BundleEntrySearch{
							Mode: &searchMode,
						},
					}},
					Link: []fm.BundleLink{{
						Relation: "self",
						Url:      "something-else",
					}},
				}
			}

			requestCounter++
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()
		_ = server.Listener.Close()
		server.Listener = listen
		server.Start()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadResources(client, "foo", url.Values{}, false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.Err)
			assert.Nil(t, bundle.ErrResponse)
			_, _, err := bundle.WriteResources(io.Discard)
			assert.NoError(t, err)
			assert.NotNil(t, bundle.Stats)
		}
		assert.Equal(t, 2, bundles)
		assert.Equal(t, 2, requestCounter)
	})

	t.Run("MultiPageResponse with link Header", func(t *testing.T) {
		listen, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			t.Errorf("could not create listener for test server: %v\n", err)
		}

		testServerURL := fmt.Sprintf("http://%s", listen.Addr())

		var requestCounter int
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			total := 2
			searchMode := fm.SearchEntryModeMatch
			var response fm.Bundle

			if requestCounter == 0 {
				w.Header().Set("Link", fmt.Sprintf(`<something>;rel="self",<%s/something-else>;rel="next"`, testServerURL))
				response = fm.Bundle{
					Type:  fm.BundleTypeSearchset,
					Total: &total,
					Entry: []fm.BundleEntry{{
						Resource: []byte("{\"foo\": \"bar\"}"),
						Search: &fm.BundleEntrySearch{
							Mode: &searchMode,
						},
					}},
					Link: []fm.BundleLink{
						{
							Relation: "self",
							Url:      "something",
						},
						{
							Relation: "next",
							Url:      fmt.Sprintf("%s/something-else", testServerURL),
						},
					},
				}
			} else {
				w.Header().Set("Link", `<something-else>;rel="self"`)
				response = fm.Bundle{
					Type:  fm.BundleTypeSearchset,
					Total: &total,
					Entry: []fm.BundleEntry{{
						Resource: []byte("{\"foobar\": \"baz\"}"),
						Search: &fm.BundleEntrySearch{
							Mode: &searchMode,
						},
					}},
					Link: []fm.BundleLink{{
						Relation: "self",
						Url:      "something-else",
					}},
				}
			}

			requestCounter++
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()
		_ = server.Listener.Close()
		server.Listener = listen
		server.Start()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadResources(client, "foo", url.Values{}, false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.Err)
			assert.Nil(t, bundle.ErrResponse)
			_, _, err := bundle.WriteResources(io.Discard)
			assert.NoError(t, err)
			assert.NotNil(t, bundle.Stats)
		}
		assert.Equal(t, 2, bundles)
		assert.Equal(t, 2, requestCounter)
	})
}

// executeDownloadSplit executes the download command with the given args,
// writing resources to outputFile, and returns its stdout and stderr output
// alongside the error.
func executeDownloadSplit(outputFile string, args ...string) (string, string, error) {
	defer resetFlags(downloadCmd, "query", "count", "use-post", "output-file")

	return executeCommandSplit("download", append(args, "--output-file", outputFile)...)
}

// executeDownload executes the download command with the given args, writing
// resources to outputFile, and returns the combined output alongside the error.
func executeDownload(outputFile string, args ...string) (string, error) {
	stdout, stderr, err := executeDownloadSplit(outputFile, args...)
	return stdout + stderr, err
}

func TestDownloadCountFlag(t *testing.T) {
	for _, value := range []string{"foo", "0", "-1", "1.5", ""} {
		t.Run("rejects `"+value+"`", func(t *testing.T) {
			server, queries := newDownloadTestServer(t)
			outputFile := filepath.Join(t.TempDir(), "out.ndjson")

			out, err := executeDownload(outputFile, "--server", server.URL, "--count", value)

			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), "count")
			}
			assert.Contains(t, out, "Usage:")
			assert.Empty(t, *queries)
			assert.NoFileExists(t, outputFile)
		})
	}

	t.Run("sends _count on system level", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownload(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "--count", "100")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.Equal(t, "100", (*queries)[0].Get("_count"))
		}
	})

	t.Run("sends _count on type level", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownload(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "Patient", "--count", "100")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.Equal(t, "100", (*queries)[0].Get("_count"))
		}
	})

	t.Run("sends _count alongside the query", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownload(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "Patient", "-q", "gender=female", "--count", "100")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.Equal(t, "100", (*queries)[0].Get("_count"))
			assert.Equal(t, "female", (*queries)[0].Get("gender"))
		}
	})

	t.Run("sends _count in the body with POST", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownload(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "Patient", "-p", "--count", "100")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.Equal(t, "100", (*queries)[0].Get("_count"))
		}
	})

	t.Run("omits _count if the flag isn't given", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownload(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "Patient")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.False(t, (*queries)[0].Has("_count"))
		}
	})

	t.Run("keeps _count of the query if the flag isn't given", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownload(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "Patient", "-q", "_count=50")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.Equal(t, "50", (*queries)[0].Get("_count"))
		}
	})

	t.Run("fails if _count is also given in the query", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		outputFile := filepath.Join(t.TempDir(), "out.ndjson")

		out, err := executeDownload(outputFile, "--server", server.URL, "Patient", "-q", "gender=female&_count=50", "--count", "100")

		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "--count")
			assert.Contains(t, err.Error(), "_count")
		}
		assert.Contains(t, out, "Usage:")
		assert.Empty(t, *queries)
		assert.NoFileExists(t, outputFile)
	})

	t.Run("fails if _count is also given in the query file", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		outputFile := filepath.Join(t.TempDir(), "out.ndjson")
		queryFile := filepath.Join(t.TempDir(), "query.txt")
		assert.NoError(t, os.WriteFile(queryFile, []byte("_count=50\n"), 0o600))

		_, err := executeDownload(outputFile, "--server", server.URL, "Patient", "-q", "@"+queryFile, "--count", "100")

		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "--count")
		}
		assert.Empty(t, *queries)
		assert.NoFileExists(t, outputFile)
	})
}

// newErrorTestServer returns a test server that responds to every request
// with a 404 OperationOutcome.
func newErrorTestServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		outcome := fm.OperationOutcome{Issue: []fm.OperationOutcomeIssue{{
			Severity: fm.IssueSeverityError,
			Code:     fm.IssueTypeNotFound,
		}}}
		if err := json.NewEncoder(w).Encode(outcome); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestDownloadMetaOutput(t *testing.T) {
	t.Run("prints stats to stderr", func(t *testing.T) {
		server, _ := newDownloadTestServer(t)

		stdout, stderr, err := executeDownloadSplit(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL)

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Pages")
	})

	for _, flag := range []string{"-s", "--silent"} {
		t.Run(flag+" suppresses stats", func(t *testing.T) {
			server, queries := newDownloadTestServer(t)
			outputFile := filepath.Join(t.TempDir(), "out.ndjson")

			stdout, stderr, err := executeDownloadSplit(outputFile, "--server", server.URL, flag)

			assert.NoError(t, err)
			assert.Empty(t, stdout)
			assert.Empty(t, stderr)
			assert.Len(t, *queries, 1)
			assert.FileExists(t, outputFile)
		})
	}

	t.Run("--silent suppresses download errors", func(t *testing.T) {
		server := newErrorTestServer(t)

		stdout, stderr, err := executeDownloadSplit(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "-s")

		assert.ErrorContains(t, err, "failed to download resources")
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
	})

	t.Run("-sS shows download errors but no stats", func(t *testing.T) {
		server := newErrorTestServer(t)

		stdout, stderr, err := executeDownloadSplit(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "-sS")

		assert.Error(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Error: failed to download resources")
		assert.NotContains(t, stderr, "Pages")
	})
}

func TestDownloadExistingOutputFile(t *testing.T) {
	t.Run("fails with exit code 3 before any request", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		outputFile := filepath.Join(t.TempDir(), "out.ndjson")
		assert.NoError(t, os.WriteFile(outputFile, []byte("content"), 0o600))

		stdout, stderr, err := executeDownloadSplit(outputFile, "--server", server.URL)

		assert.Equal(t, 3, exitCode(err))
		assert.Empty(t, stdout)
		assert.Equal(t, "Error: the output file "+outputFile+" does already exist\n", stderr)
		assert.Empty(t, *queries)
		content, _ := os.ReadFile(outputFile)
		assert.Equal(t, "content", string(content))
	})

	t.Run("--silent suppresses the error", func(t *testing.T) {
		server, _ := newDownloadTestServer(t)
		outputFile := filepath.Join(t.TempDir(), "out.ndjson")
		assert.NoError(t, os.WriteFile(outputFile, []byte("content"), 0o600))

		stdout, stderr, err := executeDownloadSplit(outputFile, "--server", server.URL, "-s")

		assert.Equal(t, 3, exitCode(err))
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
	})
}

func TestDownloadQueryFlag(t *testing.T) {
	t.Run("fails with an invalid query before any request", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		outputFile := filepath.Join(t.TempDir(), "out.ndjson")

		_, err := executeDownload(outputFile, "--server", server.URL, "Patient", "-q", "%zz")

		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "could not parse the FHIR search query")
		}
		assert.Empty(t, *queries)
		assert.NoFileExists(t, outputFile)
	})

	t.Run("fails with a missing query file before any request", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		outputFile := filepath.Join(t.TempDir(), "out.ndjson")

		_, err := executeDownload(outputFile, "--server", server.URL, "Patient", "-q", "@"+filepath.Join(t.TempDir(), "missing.txt"))

		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "could not parse the FHIR search query")
		}
		assert.Empty(t, *queries)
		assert.NoFileExists(t, outputFile)
	})

	t.Run("reads the query from a file", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		queryFile := filepath.Join(t.TempDir(), "query.txt")
		assert.NoError(t, os.WriteFile(queryFile, []byte("gender=female\n"), 0o600))

		_, err := executeDownload(filepath.Join(t.TempDir(), "out.ndjson"), "--server", server.URL, "Patient", "-q", "@"+queryFile)

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.Equal(t, "female", (*queries)[0].Get("gender"))
		}
	})
}

func TestNewOutputSink(t *testing.T) {
	var buf bytes.Buffer
	sink := newOutputSink(&buf)

	assert.Equal(t, 64<<10, sink.Size())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

// downloadBundles downloads the bundles from a test server responding with
// body and returns them followed by extraBundles.
func downloadBundles(t *testing.T, body string, extraBundles ...fhir.DownloadBundle) <-chan fhir.DownloadBundle {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	baseURL, _ := url.ParseRequestURI(server.URL)
	downloadedBundles := make(chan fhir.DownloadBundle)
	go downloadResources(fhir.NewClient(*baseURL, nil), "Patient", url.Values{}, false, downloadedBundles)

	bundleChannel := make(chan fhir.DownloadBundle)
	go func() {
		defer close(bundleChannel)
		for bundle := range downloadedBundles {
			bundleChannel <- bundle
		}
		for _, bundle := range extraBundles {
			bundleChannel <- bundle
		}
	}()
	return bundleChannel
}

func TestProcessBundles(t *testing.T) {
	patientBundle := `{"resourceType":"Bundle","entry":[{"resource":{"resourceType":"Patient","id":"0"}}]}`

	t.Run("WritesAndFlushesResources", func(t *testing.T) {
		var buf bytes.Buffer
		var stats util.CommandStats

		err := processBundles(downloadBundles(t, patientBundle), &stats, newOutputSink(&buf))

		assert.NoError(t, err)
		assert.Equal(t, "{\"resourceType\":\"Patient\",\"id\":\"0\"}\n", buf.String())
	})

	t.Run("FlushesResourcesWrittenBeforeDownloadError", func(t *testing.T) {
		var buf bytes.Buffer
		var stats util.CommandStats

		err := processBundles(downloadBundles(t, patientBundle, fhir.DownloadBundleError("foo")),
			&stats, newOutputSink(&buf))

		assert.ErrorContains(t, err, "foo")
		assert.Equal(t, "{\"resourceType\":\"Patient\",\"id\":\"0\"}\n", buf.String())
	})

	t.Run("ReturnsInvalidBundleError", func(t *testing.T) {
		var buf bytes.Buffer
		var stats util.CommandStats
		err := processBundles(downloadBundles(t, `{"entry":{}}`), &stats, newOutputSink(&buf))

		assert.ErrorContains(t, err, "could not parse the bundle entries from JSON")
	})

	t.Run("ReturnsFlushError", func(t *testing.T) {
		var stats util.CommandStats

		err := processBundles(downloadBundles(t, patientBundle), &stats, newOutputSink(failingWriter{}))

		assert.ErrorContains(t, err, "disk full")
	})

	t.Run("ReturnsWriteErrorOnlyOnce", func(t *testing.T) {
		var stats util.CommandStats
		// the output has to exceed the sink, so that writing already fails
		resource := `{"resourceType":"Patient","id":"0"}`
		entry := `{"resource":` + resource + `}`
		numEntries := outputSinkSize/len(resource) + 1
		entries := strings.Repeat(entry+",", numEntries-1) + entry
		bigBundle := `{"resourceType":"Bundle","entry":[` + entries + `]}`

		err := processBundles(downloadBundles(t, bigBundle), &stats, newOutputSink(failingWriter{}))

		assert.ErrorContains(t, err, "disk full")
		assert.Equal(t, 1, strings.Count(err.Error(), "disk full"))
	})

	t.Run("ReturnsDownloadAndFlushError", func(t *testing.T) {
		var stats util.CommandStats

		err := processBundles(downloadBundles(t, patientBundle, fhir.DownloadBundleError("foo")),
			&stats, newOutputSink(failingWriter{}))

		assert.ErrorContains(t, err, "foo")
		assert.ErrorContains(t, err, "disk full")
	})
}
