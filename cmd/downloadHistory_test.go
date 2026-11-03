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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/samply/blazectl/fhir"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
)

func TestDownloadHistory(t *testing.T) {

	t.Run("RequestToFHIRServerFails", func(t *testing.T) {
		baseURL, _ := url.ParseRequestURI("http://localhost")
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadHistory(client, "foo", "", "", 0, bundleChannel)
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

		go downloadHistory(client, "foo", "", "", 0, bundleChannel)
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

		go downloadHistory(client, "foo", "", "", 0, bundleChannel)
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

		go downloadHistory(client, "foo", "", "", 0, bundleChannel)
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

		go downloadHistory(client, "foo", "", "", 0, bundleChannel)
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

		go downloadHistory(client, "foo", "", "", 0, bundleChannel)
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

		go downloadHistory(client, "foo", "", "", 0, bundleChannel)
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

		go downloadHistory(client, "foo", "", "", 0, bundleChannel)
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

	t.Run("ResourceTypeAndIdSpecified", func(t *testing.T) {
		var requestCounter int
		var requestPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCounter++
			requestPath = r.URL.Path
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

		go downloadHistory(client, "Patient", "123", "", 0, bundleChannel)
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
		assert.Contains(t, requestPath, "Patient/123")
	})

	t.Run("OnlyResourceTypeSpecified", func(t *testing.T) {
		var requestCounter int
		var requestPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCounter++
			requestPath = r.URL.Path
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

		go downloadHistory(client, "Patient", "", "", 0, bundleChannel)
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
		assert.Contains(t, requestPath, "Patient")
		assert.Contains(t, requestPath, "_history")
	})

	t.Run("NoResourceTypeSpecified", func(t *testing.T) {
		var requestCounter int
		var requestPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCounter++
			requestPath = r.URL.Path
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

		go downloadHistory(client, "", "", "", 0, bundleChannel)
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
		assert.Contains(t, requestPath, "_history")
	})
}

// newDownloadTestServer returns a server that responds with an empty bundle to
// every request and records the queries of the requests it receives, including
// the ones sent as POST in the body.
func newDownloadTestServer(t *testing.T) (*httptest.Server, *[]url.Values) {
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		queries = append(queries, r.Form)
		if err := json.NewEncoder(w).Encode(fm.Bundle{Type: fm.BundleTypeHistory}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server, &queries
}

func TestDownloadHistorySince(t *testing.T) {
	levels := []struct {
		name         string
		resourceType string
		resourceId   string
	}{
		{"system", "", ""},
		{"type", "Patient", ""},
		{"instance", "Patient", "123"},
	}

	for _, level := range levels {
		t.Run(level.name+" level", func(t *testing.T) {
			server, queries := newDownloadTestServer(t)
			baseURL, _ := url.ParseRequestURI(server.URL)
			client := fhir.NewClient(*baseURL, nil)
			bundleChannel := make(chan fhir.DownloadBundle)

			go downloadHistory(client, level.resourceType, level.resourceId, "2026-01-01T00:00:00+01:00", 0, bundleChannel)
			for bundle := range bundleChannel {
				assert.Nil(t, bundle.Err)
				_, _, err := bundle.WriteResources(io.Discard)
				assert.NoError(t, err)
			}

			if assert.Len(t, *queries, 1) {
				assert.Equal(t, "2026-01-01T00:00:00+01:00", (*queries)[0].Get("_since"))
			}
		})
	}

	t.Run("omitted if empty", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		baseURL, _ := url.ParseRequestURI(server.URL)
		client := fhir.NewClient(*baseURL, nil)
		bundleChannel := make(chan fhir.DownloadBundle)

		go downloadHistory(client, "Patient", "", "", 0, bundleChannel)
		for bundle := range bundleChannel {
			assert.Nil(t, bundle.Err)
			_, _, err := bundle.WriteResources(io.Discard)
			assert.NoError(t, err)
		}

		if assert.Len(t, *queries, 1) {
			assert.False(t, (*queries)[0].Has("_since"))
		}
	})
}

// executeDownloadHistory executes the download-history command with the given
// args, writing resources to a temporary file, and returns the combined output
// alongside the error.
func executeDownloadHistory(t *testing.T, args ...string) (string, error) {
	stdout, stderr, err := executeDownloadHistorySplit(filepath.Join(t.TempDir(), "history.ndjson"), args...)
	return stdout + stderr, err
}

// executeDownloadHistorySplit executes the download-history command with the
// given args, writing resources to outputFile, and returns its stdout and
// stderr output alongside the error.
func executeDownloadHistorySplit(outputFile string, args ...string) (string, string, error) {
	defer resetFlags(downloadHistoryCmd, "since", "count", "output-file")

	return executeCommandSplit("download-history", append(args, "--output-file", outputFile)...)
}

func TestDownloadHistoryMetaOutput(t *testing.T) {
	t.Run("prints stats to stderr", func(t *testing.T) {
		server, _ := newDownloadTestServer(t)

		stdout, stderr, err := executeDownloadHistorySplit(filepath.Join(t.TempDir(), "history.ndjson"), "--server", server.URL)

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Pages")
	})

	t.Run("--silent suppresses stats", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		outputFile := filepath.Join(t.TempDir(), "history.ndjson")

		stdout, stderr, err := executeDownloadHistorySplit(outputFile, "--server", server.URL, "-s")

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
		assert.Len(t, *queries, 1)
		assert.FileExists(t, outputFile)
	})

	t.Run("fails with exit code 3 on an existing output file", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)
		outputFile := filepath.Join(t.TempDir(), "history.ndjson")
		assert.NoError(t, os.WriteFile(outputFile, []byte("content"), 0o600))

		stdout, stderr, err := executeDownloadHistorySplit(outputFile, "--server", server.URL)

		assert.Equal(t, 3, exitCode(err))
		assert.Empty(t, stdout)
		assert.Equal(t, "Error: the output file "+outputFile+" does already exist\n", stderr)
		assert.Empty(t, *queries)
	})
}

func TestDownloadHistorySinceFlag(t *testing.T) {
	for _, value := range []string{
		"foo",
		"2026-01-01",
		"2026-01-01T00:00:00",
		"",
		"2026-01-01T0:00:00Z",
		"2026-01-01T00:00:00,5Z",
		"2026-01-01T00:00:00+24:00",
		"2026-01-01T00:00:00+14:01",
		"2026-02-30T00:00:00Z",
		"0000-01-01T00:00:00Z",
	} {
		t.Run("rejects `"+value+"`", func(t *testing.T) {
			server, queries := newDownloadTestServer(t)

			out, err := executeDownloadHistory(t, "--server", server.URL, "--since", value)

			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), "since")
			}
			assert.Contains(t, out, "Usage:")
			assert.Empty(t, *queries)
		})
	}

	for _, value := range []string{"2026-01-01T00:00:00Z", "2026-01-01T00:00:00.123+01:00"} {
		t.Run("sends "+value, func(t *testing.T) {
			server, queries := newDownloadTestServer(t)

			_, err := executeDownloadHistory(t, "--server", server.URL, "Patient", "--since", value)

			assert.NoError(t, err)
			if assert.Len(t, *queries, 1) {
				assert.Equal(t, value, (*queries)[0].Get("_since"))
			}
		})
	}

	t.Run("omits _since if the flag isn't given", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownloadHistory(t, "--server", server.URL, "Patient")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.False(t, (*queries)[0].Has("_since"))
		}
	})
}

func TestDownloadHistoryCountFlag(t *testing.T) {
	for _, value := range []string{"foo", "0", "-1", "1.5", ""} {
		t.Run("rejects `"+value+"`", func(t *testing.T) {
			server, queries := newDownloadTestServer(t)

			out, err := executeDownloadHistory(t, "--server", server.URL, "--count", value)

			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), "count")
			}
			assert.Contains(t, out, "Usage:")
			assert.Empty(t, *queries)
		})
	}

	levels := []struct {
		name string
		args []string
	}{
		{"system", nil},
		{"type", []string{"Patient"}},
		{"instance", []string{"Patient", "123"}},
	}

	for _, level := range levels {
		t.Run("sends _count on "+level.name+" level", func(t *testing.T) {
			server, queries := newDownloadTestServer(t)

			_, err := executeDownloadHistory(t, append(level.args, "--server", server.URL, "--count", "100")...)

			assert.NoError(t, err)
			if assert.Len(t, *queries, 1) {
				assert.Equal(t, "100", (*queries)[0].Get("_count"))
			}
		})
	}

	t.Run("sends _count alongside _since", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownloadHistory(t, "--server", server.URL, "--count", "100", "--since", "2026-01-01T00:00:00Z")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.Equal(t, "100", (*queries)[0].Get("_count"))
			assert.Equal(t, "2026-01-01T00:00:00Z", (*queries)[0].Get("_since"))
		}
	})

	t.Run("omits _count if the flag isn't given", func(t *testing.T) {
		server, queries := newDownloadTestServer(t)

		_, err := executeDownloadHistory(t, "--server", server.URL, "Patient")

		assert.NoError(t, err)
		if assert.Len(t, *queries, 1) {
			assert.False(t, (*queries)[0].Has("_count"))
		}
	})
}

// We don't need to test writeResources again since it's already tested in download_test.go
// and both commands use the same function.
