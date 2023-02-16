// Copyright 2019 - 2023 The Samply Community
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
	"fmt"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// A Client is a FHIR client which combines an HTTP client with the base URL of
// a FHIR server. At minimum, the BaseURL has to be set. HttpClient can be left at
// its default value.
type Client struct {
	httpClient http.Client
	baseURL    url.URL
	auth       ClientAuth
}

// ClientAuth comprises the authentication information used by the Client in
// order to communicate with a FHIR server.
type ClientAuth struct {
	BasicAuthUser     string
	BasicAuthPassword string
}

// NewClient creates a new Client with the given base URL and ClientAuth configuration.
func NewClient(fhirServerBaseUrl url.URL, auth ClientAuth) *Client {
	return createClient(fhirServerBaseUrl, auth, false)
}

// NewClientInsecure creates a new Client as NewClient does but disables TLS security checks. I.e. the client will
// accept any connection to a servers without verifying its certificate.
// Use this with great caution as it opens up man-in-the-middle attacks.
func NewClientInsecure(fhirServerBaseUrl url.URL, auth ClientAuth) *Client {
	return createClient(fhirServerBaseUrl, auth, true)
}

func createClient(fhirServerBaseUrl url.URL, auth ClientAuth, insecure bool) *Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100
	t.TLSClientConfig.InsecureSkipVerify = insecure

	return &Client{
		httpClient: http.Client{Transport: t},
		baseURL:    fhirServerBaseUrl,
		auth:       auth,
	}
}

const fhirJson = "application/fhir+json"

// NewCapabilitiesRequest creates a new capabilities interaction request. Uses
// the base URL from the FHIR client and sets JSON Accept header. Otherwise it's
// identical to http.NewRequest.
func (c *Client) NewCapabilitiesRequest() (*http.Request, error) {
	req, err := http.NewRequest("GET", c.baseURL.JoinPath("metadata").String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", fhirJson)
	return req, nil
}

// NewTransactionRequest creates a new transaction/batch interaction request.
// Uses the base URL from the FHIR client and sets JSON Accept and Content-Type
// headers. Otherwise, it's identical to http.NewRequest.
func (c *Client) NewTransactionRequest(body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", c.baseURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("error while creating a transaction request: %w", err)
	}
	req.Header.Add("Accept", fhirJson)
	req.Header.Add("Content-Type", fhirJson)
	return req, nil
}

// NewSearchTypeRequest creates a new search type interaction request that will use GET with a
// FHIR search query in the query params of the URL.
func (c *Client) NewSearchTypeRequest(resourceType string, searchQuery url.Values) (*http.Request, error) {
	_url := c.baseURL.JoinPath(resourceType)
	_url.RawQuery = searchQuery.Encode()
	req, err := http.NewRequest("GET", _url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", fhirJson)
	return req, nil
}

// NewPostSearchTypeRequest creates a new search type interaction request that will use POST with a
// FHIR search query in the body.
func (c *Client) NewPostSearchTypeRequest(resourceType string, searchQuery url.Values) (*http.Request, error) {
	req, err := http.NewRequest("POST", c.baseURL.JoinPath(resourceType, "_search").String(),
		strings.NewReader(searchQuery.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", fhirJson)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

// NewSearchSystemRequest creates a new search system interaction request that will use GET with a
// FHIR search query in the query params of the URL.
func (c *Client) NewSearchSystemRequest(searchQuery url.Values) (*http.Request, error) {
	_url := c.baseURL.JoinPath("")
	_url.RawQuery = searchQuery.Encode()
	req, err := http.NewRequest("GET", _url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", fhirJson)
	return req, nil
}

// NewPaginatedRequest creates a new resource interaction request based on
// a pagination link received from a FHIR server. It sets JSON Accept header and is
// otherwise identical to http.NewRequest.
func (c *Client) NewPaginatedRequest(paginationURL *url.URL) (*http.Request, error) {
	req, err := http.NewRequest("GET", paginationURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", fhirJson)
	return req, nil
}

// NewTypeOperationRequest creates a new operation request that will use GET with parameters in the query params of the URL.
func (c *Client) NewTypeOperationRequest(resourceType string, operationName string, parameters url.Values) (*http.Request, error) {
	_url := c.baseURL.JoinPath(resourceType, "/$"+operationName)
	_url.RawQuery = parameters.Encode()
	req, err := http.NewRequest("GET", _url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", fhirJson)
	return req, nil
}

// Do calls Do on the HTTP client of the FHIR client.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if len(c.auth.BasicAuthUser) != 0 {
		req.SetBasicAuth(c.auth.BasicAuthUser, c.auth.BasicAuthPassword)
	}

	return c.httpClient.Do(req)
}

// CloseIdleConnections calls CloseIdleConnections on the HTTP client of the
// FHIR client.
func (c *Client) CloseIdleConnections() {
	c.httpClient.CloseIdleConnections()
}

// ReadCapabilityStatement reads and unmarshals a capability statement.
func ReadCapabilityStatement(r io.Reader) (fm.CapabilityStatement, error) {
	var capabilityStatement fm.CapabilityStatement
	body, err := io.ReadAll(r)
	if err != nil {
		return capabilityStatement, err
	}
	if err := json.Unmarshal(body, &capabilityStatement); err != nil {
		return capabilityStatement, err
	}
	return capabilityStatement, nil
}

// ReadBundle reads and unmarshals a bundle.
func ReadBundle(r io.Reader) (fm.Bundle, error) {
	var bundle fm.Bundle
	body, err := io.ReadAll(r)
	if err != nil {
		return bundle, err
	}
	return fm.UnmarshalBundle(body)
}
