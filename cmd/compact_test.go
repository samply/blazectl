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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samply/blazectl/fhir"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
)

func TestCreateParameters(t *testing.T) {
	parameters := createParameters("index", "resource-as-of-index")

	assert.Equal(t, "database", parameters.Parameter[0].Name)
	assert.Equal(t, "index", *parameters.Parameter[0].ValueCode)

	assert.Equal(t, "column-family", parameters.Parameter[1].Name)
	assert.Equal(t, "resource-as-of-index", *parameters.Parameter[1].ValueCode)
}

// newAsyncTestServer returns a test server that serves the capability
// statement of a Blaze server supporting the $disk-perf operation and answers
// POST requests of system operations with 202 and a status endpoint that
// returns result.
func newAsyncTestServer(t *testing.T, result string) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/metadata":
			capabilityStatement := fm.CapabilityStatement{
				Software: &fm.CapabilityStatementSoftware{Name: "Blaze"},
				Rest: []fm.CapabilityStatementRest{{
					Mode:      fm.RestfulCapabilityModeServer,
					Operation: []fm.CapabilityStatementRestResourceOperation{{Name: "disk-perf"}},
				}},
			}
			if err := json.NewEncoder(w).Encode(capabilityStatement); err != nil {
				t.Error(err)
			}
		case r.URL.Path == "/async-poll":
			response := fm.Bundle{
				Type: fm.BundleTypeBatchResponse,
				Entry: []fm.BundleEntry{{
					Response: &fm.BundleEntryResponse{Status: "200"},
					Resource: json.RawMessage(result),
				}},
			}
			w.Header().Set(fhir.HeaderContentType, fhir.MediaTypeFhirJson)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Error(err)
			}
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Location", fmt.Sprintf("http://%s/async-poll", r.Host))
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestCompactMetaOutput(t *testing.T) {
	args := []string{"index", "resource-as-of-index"}

	t.Run("prints messages to stderr", func(t *testing.T) {
		server := newAsyncTestServer(t, `{"resourceType":"Parameters"}`)

		stdout, stderr, err := executeCommandSplit("compact", append(args, "--server", server.URL)...)

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Start compacting column family `resource-as-of-index` in database `index`...")
		assert.Contains(t, stderr, "Start polling status endpoint")
		assert.Contains(t, stderr, "Successfully compacted column family `resource-as-of-index` in database `index`.")
	})

	t.Run("--silent suppresses messages", func(t *testing.T) {
		server := newAsyncTestServer(t, `{"resourceType":"Parameters"}`)

		stdout, stderr, err := executeCommandSplit("compact", append(args, "--server", server.URL, "-s")...)

		assert.NoError(t, err)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
	})

	t.Run("fails if the server doesn't accept the compaction", func(t *testing.T) {
		server := newErrorTestServer(t)

		stdout, stderr, err := executeCommandSplit("compact", append(args, "--server", server.URL)...)

		assert.ErrorContains(t, err, "error while compacting column family `resource-as-of-index` in database `index`")
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "Error: error while compacting")
	})

	t.Run("--silent suppresses the failure", func(t *testing.T) {
		server := newErrorTestServer(t)

		stdout, stderr, err := executeCommandSplit("compact", append(args, "--server", server.URL, "-s")...)

		assert.Error(t, err)
		assert.Empty(t, stdout)
		assert.Empty(t, stderr)
	})
}
