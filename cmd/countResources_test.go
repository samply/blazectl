// Copyright 2019 - 2024 The Samply Community
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
	"github.com/samply/blazectl/fhir"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFetchResourcesTotal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/fhir+json", r.Header.Get("Accept"))
		assert.Equal(t, "application/fhir+json", r.Header.Get("Content-Type"))
		defer r.Body.Close()
		bundle, err := fhir.ReadBundle(r.Body)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, fm.BundleTypeBatch, bundle.Type)
		if !assert.NotNil(t, bundle.Entry[0].Request) {
			return
		}
		assert.Equal(t, fm.HTTPVerbGET, bundle.Entry[0].Request.Method)
		assert.Equal(t, "Patient?_summary=count", bundle.Entry[0].Request.Url)

		total := 23
		resource := fm.Bundle{
			Type:  fm.BundleTypeSearchset,
			Total: &total,
		}
		resourceBytes, err := json.Marshal(resource)
		if err != nil {
			t.Error(err)
		}
		response := fm.Bundle{
			Type: fm.BundleTypeBatchResponse,
			Entry: []fm.BundleEntry{{
				Resource: json.RawMessage(resourceBytes),
				Response: &fm.BundleEntryResponse{
					Status: "200 OK",
				},
			}},
		}
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer ts.Close()

	baseURL, _ := url.ParseRequestURI(ts.URL)
	client := fhir.NewClient(*baseURL, nil)
	result, err := fetchResourcesTotal(client, []fm.ResourceType{fm.ResourceTypePatient})
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 23, result[fm.ResourceTypePatient])
}
