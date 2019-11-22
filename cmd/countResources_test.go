package cmd

import (
	"encoding/json"
	"github.com/samply/blazectl/fhir"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
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
		assert.Equal(t, "batch", bundle.Type)
		if !assert.NotNil(t, bundle.Entry[0].Request) {
			return
		}
		assert.Equal(t, "GET", bundle.Entry[0].Request.Method)
		assert.Equal(t, "Patient?_summary=count", bundle.Entry[0].Request.URL)

		total := 23
		resource := fhir.Bundle{
			Type:  "searchset",
			Total: &total,
		}
		resourceBytes, err := json.Marshal(resource)
		if err != nil {
			t.Error(err)
		}
		response := fhir.Bundle{
			Type: "batch-response",
			Entry: []fhir.BundleEntry{{
				Resource: json.RawMessage(resourceBytes),
				Response: &fhir.BundleEntryResponse{
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

	client := &fhir.Client{Base: ts.URL}
	result, err := fetchResourcesTotal(client, []string{"Patient"})
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 23, result["Patient"])
}
