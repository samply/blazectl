// Copyright 2019 - 2022 The Samply Community
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

package fhir

import (
	"encoding/json"
	. "github.com/samply/golang-fhir-models/fhir-models/fhir"
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
	if err := json.Unmarshal(bundle.Entry[0].Resource, &bundle); err != nil {
		t.Error(err)
	}
	assert.Equal(t, 23, *bundle.Total)
}
