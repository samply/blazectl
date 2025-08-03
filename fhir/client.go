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

package fhir

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// A Client is a FHIR client which combines an HTTP client with the base URL of
// a FHIR server. At minimum, the BaseURL has to be set. HttpClient can be left at
// its default value.
type Client struct {
	httpClient http.Client
	baseURL    url.URL
	auth       Auth
}

type Auth interface {
	setAuth(req *http.Request)
}

// BasicAuth comprises basic authentication information used by the Client in
// order to communicate with a FHIR server.
type BasicAuth struct {
	User     string
	Password string
}

func (auth BasicAuth) setAuth(req *http.Request) {
	req.SetBasicAuth(auth.User, auth.Password)
}

// TokenAuth comprises bearer token authentication information used by the Client in
// order to communicate with a FHIR server.
type TokenAuth struct {
	Token string
}

func (auth TokenAuth) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+auth.Token)
}

// NewClient creates a new Client with the given base URL and BasicAuth configuration.
func NewClient(fhirServerBaseUrl url.URL, auth Auth) *Client {
	return createClient(fhirServerBaseUrl, auth, false)
}

// NewClientInsecure creates a new Client as NewClient does but disables TLS security checks. I.e. the client will
// accept any connection to a servers without verifying its certificate.
// Use this with great caution as it opens up man-in-the-middle attacks.
func NewClientInsecure(fhirServerBaseUrl url.URL, auth Auth) *Client {
	return createClient(fhirServerBaseUrl, auth, true)
}

func NewClientCa(fhirServerBaseUrl url.URL, auth Auth, caCertFilename string) (*Client, error) {
	caCert, err := os.ReadFile(caCertFilename)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100
	t.TLSClientConfig = tlsConfig

	return &Client{
		httpClient: http.Client{Transport: t},
		baseURL:    fhirServerBaseUrl,
		auth:       auth,
	}, nil
}

func createClient(fhirServerBaseUrl url.URL, auth Auth, insecure bool) *Client {
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

const HeaderAccept = "Accept"
const HeaderContentType = "Content-Type"
const MediaTypeFhirJson = "application/fhir+json"
const mediaTypeForm = "application/x-www-form-urlencoded"

// NewCapabilitiesRequest creates a new capabilities interaction request. Uses
// the base URL from the FHIR client and sets JSON Accept header. Otherwise it's
// identical to http.NewRequest.
func (c *Client) NewCapabilitiesRequest() (*http.Request, error) {
	req, err := http.NewRequest("GET", c.baseURL.JoinPath("metadata").String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
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
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
	req.Header.Add(HeaderContentType, MediaTypeFhirJson)
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
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
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
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
	req.Header.Add(HeaderContentType, mediaTypeForm)
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
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
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
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
	return req, nil
}

// NewPostSystemOperationRequest creates a new operation request that will use POST with parameters.
func (c *Client) NewPostSystemOperationRequest(operationName string, async bool, parameters fm.Parameters) (*http.Request, error) {
	payload, err := json.Marshal(parameters)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.baseURL.JoinPath("$"+operationName).String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
	req.Header.Add(HeaderContentType, MediaTypeFhirJson)
	if async {
		req.Header.Add("Prefer", "respond-async")
	}
	return req, nil
}

// NewTypeOperationRequest creates a new operation request that will use GET with parameters in the query params of the URL.
func (c *Client) NewTypeOperationRequest(resourceType string, operationName string, async bool, parameters url.Values) (*http.Request, error) {
	_url := c.baseURL.JoinPath(resourceType, "$"+operationName)
	_url.RawQuery = parameters.Encode()
	req, err := http.NewRequest("GET", _url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
	if async {
		req.Header.Add("Prefer", "respond-async")
	}
	return req, nil
}

// Do calls Do on the HTTP client of the FHIR client.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.auth != nil {
		c.auth.setAuth(req)
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

type operationOutcomeError struct {
	outcome *fm.OperationOutcome
}

func (err *operationOutcomeError) Error() string {
	return util.FmtOperationOutcomes([]*fm.OperationOutcome{err.outcome})
}

func handleErrorResponse(resp *http.Response) error {
	defer func() {
		// Read and discard any remaining body content
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if IsFhirResponse(resp) {
		var operationOutcome fm.OperationOutcome
		if err := json.NewDecoder(resp.Body).Decode(&operationOutcome); err != nil {
			return err
		}

		return fmt.Errorf("%w", &operationOutcomeError{outcome: &operationOutcome})
	} else {
		return fmt.Errorf("non FHIR response")
	}
}

func IsFhirResponse(resp *http.Response) bool {
	return strings.HasPrefix(resp.Header.Get(HeaderContentType), MediaTypeFhirJson)
}

// PollAsyncStatus polls the async status location until a 200 is returned.
// Can be interrupted by putting a signal on the interruptChan.
// Starts polling after 100 ms. Increases polling gap exponentially if still under 10 seconds.
// Keeps the polling gap constant after that.
// Prints eclipsed time from start on STDERR.
func (c *Client) PollAsyncStatus(location string, interruptChan chan os.Signal) ([]byte, error) {
	wait := 100 * time.Millisecond
	start := time.Now()
	req, err := http.NewRequest("GET", location, nil)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "Start polling status endpoint at %s...\n", location)
	for {
		select {
		case <-interruptChan:
			fmt.Fprintf(os.Stderr, "Cancel async request...\n")

			req, err := http.NewRequest("DELETE", location, nil)
			if err != nil {
				return nil, err
			}

			resp, err := c.Do(req)
			if err != nil {
				return nil, err
			}

			return nil, handlePollCancelResponse(location, resp)
		case <-time.After(wait):
			fmt.Fprintf(os.Stderr, "eclipsed time %.1f s\n", time.Since(start).Seconds())

			resp, err := c.Do(req)
			if err != nil {
				return nil, err
			}

			if resp.StatusCode == 200 {
				return handlePollOkResponse(resp)
			} else if resp.StatusCode == 202 {
				if err := DiscardAndClose(resp.Body); err != nil {
					return nil, err
				}

				// exponential wait up to 10 seconds
				if wait < 10*time.Second {
					wait *= 2
				}

				// Continue the loop to poll again
				continue
			} else {
				return nil, handleErrorResponse(resp)
			}
		}
	}
}

func handlePollCancelResponse(location string, resp *http.Response) error {
	defer DiscardAndClose(resp.Body)

	if resp.StatusCode == 202 {
		return fmt.Errorf("sucessfully cancelled the async request at status endpoint %s", location)
	} else {
		return fmt.Errorf("Error while cancelling the async request at status endpoint %s:\n\n%w",
			location, handleErrorResponse(resp))
	}
}

func handlePollOkResponse(resp *http.Response) ([]byte, error) {
	defer DiscardAndClose(resp.Body)

	if IsFhirResponse(resp) {
		var bundle fm.Bundle
		if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
			return nil, fmt.Errorf("error while reading the async response bundle: %w", err)
		}

		if bundle.Type != fm.BundleTypeBatchResponse {
			return nil, fmt.Errorf("expected batch-response bundle but the bundle type is: %s", bundle.Type)
		}

		if len(bundle.Entry) != 1 {
			return nil, fmt.Errorf("expected one entry in async response bundle but was %d entries", len(bundle.Entry))
		}

		if bundle.Entry[0].Response == nil {
			return nil, fmt.Errorf("missing response in bundle entry")
		}

		response := bundle.Entry[0].Response

		if !strings.HasPrefix(response.Status, "200") {
			if response.Outcome == nil {
				return nil, fmt.Errorf("error status: %s", response.Status)
			}

			var operationOutcome fm.OperationOutcome
			if err := json.Unmarshal(response.Outcome, &operationOutcome); err != nil {
				return nil, fmt.Errorf("error while reading the outcome of an error response in the async response bundle: %w", err)
			}

			return nil, fmt.Errorf("%w", &operationOutcomeError{outcome: &operationOutcome})
		}

		return bundle.Entry[0].Resource, nil
	} else {
		return nil, fmt.Errorf("non FHIR response")
	}
}

func DiscardAndClose(r io.ReadCloser) error {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	return nil
}
