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
	"io"
	"io/ioutil"
	"net/http"
)

// A Client is a FHIR client which combines an HTTP client with the base URL of
// a FHIR server. At minimum, the Base has to be set. HttpClient can be left at
// its default value.
type Client struct {
	HttpClient http.Client
	Base       string
}

// NewCapabilitiesRequest creates a new capabilities interaction request. Uses
// the base URL from the FHIR client and sets JSON Accept header. Otherwise it's
// identical to http.NewRequest.
func (c *Client) NewCapabilitiesRequest() (*http.Request, error) {
	req, err := http.NewRequest("GET", c.Base+"/metadata", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	return req, nil
}

// NewTransactionRequest creates a new transaction/batch interaction request.
// Uses the base URL from the FHIR client and sets JSON Accept and Content-Type
// headers. Otherwise it's identical to http.NewRequest.
func (c *Client) NewTransactionRequest(body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", c.Base, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	req.Header.Add("Content-Type", "application/fhir+json")
	return req, nil
}

// NewBatchRequest creates a new transaction/batch interaction request.
// Uses the base URL from the FHIR client and sets JSON Accept and Content-Type
// headers. Otherwise it's identical to http.NewRequest.
func (c *Client) NewBatchRequest(body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", c.Base, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	req.Header.Add("Content-Type", "application/fhir+json")
	return req, nil
}

// NewSearchTypeRequest creates a new search-type interaction request. Uses the
// base URL from the FHIR client and sets JSON Accept header. Otherwise it's
// identical to http.NewRequest.
func (c *Client) NewSearchTypeRequest(resourceType string) (*http.Request, error) {
	req, err := http.NewRequest("GET", c.SearchTypeURL(resourceType), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	return req, nil
}

// SearchTypeURL generates the URL for a search-type interaction of the given
// resource type
func (c *Client) SearchTypeURL(resourceType string) string {
	return c.Base + "/" + resourceType
}

// Do calls Do on the HTTP client of the FHIR client.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.HttpClient.Do(req)
}

// CloseIdleConnections calls CloseIdleConnections on the HTTP client of the
// FHIR client.
func (c *Client) CloseIdleConnections() {
	c.HttpClient.CloseIdleConnections()
}

// ReadCapabilityStatement reads and unmarshals a capability statement.
func ReadCapabilityStatement(r io.Reader) (CapabilityStatement, error) {
	var capabilityStatement CapabilityStatement
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return capabilityStatement, err
	}
	if err := json.Unmarshal(body, &capabilityStatement); err != nil {
		return capabilityStatement, err
	}
	return capabilityStatement, nil
}

// ReadBundle reads and unmarshals a bundle.
func ReadBundle(r io.Reader) (Bundle, error) {
	var bundle Bundle
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return bundle, err
	}
	return UnmarshalBundle(body)
}

// UnmarshalBundle unmarshals a bundle.
func UnmarshalBundle(b []byte) (Bundle, error) {
	var bundle Bundle
	if err := json.Unmarshal(b, &bundle); err != nil {
		return bundle, err
	}
	return bundle, nil
}
