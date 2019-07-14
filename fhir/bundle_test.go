package fhir

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnmarshalBundleEntryResource(t *testing.T) {
	var bundle Bundle
	if err := json.Unmarshal([]byte(`{
"resourceType": "Bundle",
"type": "batch-response",
"entry": [{
  "resource": {
    "resourceType": "Bundle",
    "type": "searchset",
    "total": 23
}}]}`), &bundle); err != nil {
		t.Error(err)
	}
	if err := json.Unmarshal(bundle.Entry[0].Resource.Json, &bundle); err != nil {
		t.Error(err)
	}
	assert.Equal(t, 23, *bundle.Total)
}
