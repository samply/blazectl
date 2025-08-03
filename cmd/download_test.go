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
	"github.com/samply/blazectl/fhir"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDownloadResources(t *testing.T) {

	t.Run("RequestToFHIRServerFails", func(t *testing.T) {
		baseURL, _ := url.ParseRequestURI("http://localhost")
		client := fhir.NewClient(*baseURL, nil)

		var bundles int
		bundleChannel := make(chan downloadBundle)

		go downloadResources(client, "foo", "", false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.NotNil(t, bundle.err)
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
		bundleChannel := make(chan downloadBundle)

		go downloadResources(client, "foo", "", false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.NotNil(t, bundle.err)
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
		bundleChannel := make(chan downloadBundle)

		go downloadResources(client, "foo", "", false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.err)
			assert.NotNil(t, bundle.responseBody)
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
		bundleChannel := make(chan downloadBundle)

		go downloadResources(client, "foo", "", false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.NotNil(t, bundle.err)
			assert.NotNil(t, bundle.errResponse)
			assert.NotNil(t, bundle.stats)
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
		bundleChannel := make(chan downloadBundle)

		go downloadResources(client, "foo", "", false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.err)
			assert.Nil(t, bundle.errResponse)
			assert.NotNil(t, bundle.responseBody)
			assert.NotNil(t, bundle.stats)
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
		bundleChannel := make(chan downloadBundle)

		go downloadResources(client, "foo", "", false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.err)
			assert.Nil(t, bundle.errResponse)
			assert.NotNil(t, bundle.responseBody)
			assert.NotNil(t, bundle.stats)
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
		bundleChannel := make(chan downloadBundle)

		go downloadResources(client, "foo", "", false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.err)
			assert.Nil(t, bundle.errResponse)
			assert.NotNil(t, bundle.responseBody)
			assert.NotNil(t, bundle.stats)
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
		bundleChannel := make(chan downloadBundle)

		go downloadResources(client, "foo", "", false, bundleChannel)
		for bundle := range bundleChannel {
			bundles++
			assert.Nil(t, bundle.err)
			assert.Nil(t, bundle.errResponse)
			assert.NotNil(t, bundle.responseBody)
			assert.NotNil(t, bundle.stats)
		}
		assert.Equal(t, 2, bundles)
		assert.Equal(t, 2, requestCounter)
	})
}

func TestWriteResource(t *testing.T) {
	t.Run("EmptyData", func(t *testing.T) {
		resources, outcomes, err := writeResources([]byte{}, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 0, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("EmptyBundleEntry", func(t *testing.T) {
		data := []byte(`{"entry":[{}]}`)
		resources, outcomes, err := writeResources(data, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 0, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("SingleBundleEntry", func(t *testing.T) {
		data := []byte(`{"entry": [{"resource": {}, "search": {"mode": "match"}}]}`)
		resources, outcomes, err := writeResources(data, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 1, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("SingleBundleEntryWithInlineOutcome", func(t *testing.T) {
		outcome := fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{{
				Severity: fm.IssueSeverityWarning,
				Code:     fm.IssueTypeTooLong,
			}},
		}

		outcomeRawJSON, _ := json.Marshal(outcome)

		searchMode := fm.SearchEntryModeOutcome

		var bundleEntry fm.BundleEntry
		bundleEntry.Resource = outcomeRawJSON
		bundleEntry.Search = &fm.BundleEntrySearch{
			Mode: &searchMode,
		}
		var bundle fm.Bundle
		bundle.Entry = []fm.BundleEntry{bundleEntry}

		bundleRawJSON, _ := json.Marshal(bundle)
		resources, outcomes, err := writeResources(bundleRawJSON, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 0, resources)
		assert.NotEmpty(t, outcomes)
	})

	t.Run("MultipleBundleEntries", func(t *testing.T) {
		searchMode := fm.SearchEntryModeMatch

		var bundleEntryA fm.BundleEntry
		bundleEntryA.Resource = []byte("{}")
		bundleEntryA.Search = &fm.BundleEntrySearch{
			Mode: &searchMode,
		}
		var bundleEntryB fm.BundleEntry
		bundleEntryB.Resource = []byte("{}")
		bundleEntryB.Search = &fm.BundleEntrySearch{
			Mode: &searchMode,
		}
		var bundle fm.Bundle
		bundle.Entry = []fm.BundleEntry{bundleEntryA, bundleEntryB}

		bundleRawJSON, _ := json.Marshal(bundle)
		resources, outcomes, err := writeResources(bundleRawJSON, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 2, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("MultipleBundleEntriesWithSingleInlineOutcome", func(t *testing.T) {
		searchModeA := fm.SearchEntryModeMatch
		searchModeB := fm.SearchEntryModeOutcome

		outcome := fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{{
				Severity: fm.IssueSeverityWarning,
				Code:     fm.IssueTypeTooLong,
			}},
		}
		outcomeRawJSON, _ := json.Marshal(outcome)

		var bundleEntryA fm.BundleEntry
		bundleEntryA.Resource = []byte("{}")
		bundleEntryA.Search = &fm.BundleEntrySearch{
			Mode: &searchModeA,
		}
		var bundleEntryB fm.BundleEntry
		bundleEntryB.Resource = outcomeRawJSON
		bundleEntryB.Search = &fm.BundleEntrySearch{
			Mode: &searchModeB,
		}
		var bundle fm.Bundle
		bundle.Entry = []fm.BundleEntry{bundleEntryA, bundleEntryB}

		bundleRawJSON, _ := json.Marshal(bundle)
		resources, outcomes, err := writeResources(bundleRawJSON, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 1, resources)
		assert.NotEmpty(t, outcomes)
	})
}
