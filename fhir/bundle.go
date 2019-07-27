// Copyright © 2019 Alexander Kiel <alexander.kiel@life.uni-leipzig.de>
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
)

// Bundle is documented here https://www.hl7.org/fhir/bundle.html
type Bundle struct {
	Type  string
	Total *int
	Entry []BundleEntry
}

// MarshalJSON marshals the given bundle as JSON into a byte slice
func (b Bundle) MarshalJSON() ([]byte, error) {
	x := make(map[string]interface{})
	x["resourceType"] = "Bundle"
	x["type"] = b.Type
	if b.Total != nil {
		x["total"] = b.Total
	}
	if len(b.Entry) > 0 {
		x["entry"] = b.Entry
	}
	return json.Marshal(x)
}

// BundleEntry represents the Bundle.entry BackboneElement
type BundleEntry struct {
	Resource json.RawMessage      `json:"resource,omitempty"`
	Request  *BundleEntryRequest  `json:"request,omitempty"`
	Response *BundleEntryResponse `json:"response,omitempty"`
}

// BundleEntryRequest represents the Bundle.entry.request BackboneElement
type BundleEntryRequest struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

// BundleEntryResponse represents the Bundle.entry.response BackboneElement
type BundleEntryResponse struct {
	Status string `json:"status"`
}
