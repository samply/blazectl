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
	"io"
	"net/http"
)

// A FHIR client which combines an HTTP client with the base URL of a FHIR
// server.
type Client struct {
	HttpClient http.Client
	Base       string
}

// Creates a new capabilities interaction request. Uses the base URL from
// the FHIR client and sets JSON Accept header. Otherwise it's identical to
// http.NewRequest.
func (c *Client) NewCapabilitiesRequest() (*http.Request, error) {
	req, err := http.NewRequest("GET", c.Base+"/metadata", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	return req, nil
}

// Creates a new transaction/batch interaction request. Uses the base URL from
// the FHIR client and sets JSON Accept and Content-Type headers. Otherwise it's
// identical to http.NewRequest.
func (c *Client) NewTransactionRequest(body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", c.Base, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	req.Header.Add("Content-Type", "application/fhir+json")
	return req, nil
}

// Creates a new search-type interaction request. Uses the base URL from
// the FHIR client and sets JSON Accept header. Otherwise it's identical to
// http.NewRequest.
func (c *Client) NewSearchTypeRequest(resourceType string) (*http.Request, error) {
	req, err := http.NewRequest("GET", c.Base+"/"+resourceType, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/fhir+json")
	return req, nil
}

// Calls Do on the HTTP client of the FHIR client
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.HttpClient.Do(req)
}

// Calls CloseIdleConnections on the HTTP client of the FHIR client
func (c *Client) CloseIdleConnections() {
	c.HttpClient.CloseIdleConnections()
}
