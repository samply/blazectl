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
	"encoding/json/jsontext"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
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
	t.TLSClientConfig = tlsConfig

	return &Client{
		httpClient: http.Client{Transport: t},
		baseURL:    fhirServerBaseUrl,
		auth:       auth,
	}, nil
}

func createClient(fhirServerBaseUrl url.URL, auth Auth, insecure bool) *Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
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

// NewHistoryTypeRequest creates a new history request that will use GET on a resource type.
func (c *Client) NewHistoryTypeRequest(resourceType string) (*http.Request, error) {
	_url := c.baseURL.JoinPath(resourceType, "_history")
	req, err := http.NewRequest("GET", _url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
	return req, nil
}

// NewHistoryInstanceRequest creates a new history request that will use GET on a resource.
func (c *Client) NewHistoryInstanceRequest(resourceType string, resourceId string) (*http.Request, error) {
	_url := c.baseURL.JoinPath(resourceType, resourceId, "_history")
	req, err := http.NewRequest("GET", _url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
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

// newPostOperationRequest creates a new operation request that will POST the
// given parameters as a FHIR Parameters body to the given operation URL.
func (c *Client) newPostOperationRequest(operationUrl string, async bool, parameters fm.Parameters) (*http.Request, error) {
	payload, err := json.Marshal(parameters)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", operationUrl, bytes.NewReader(payload))
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

// NewPostSystemOperationRequest creates a new system-level operation request that will use POST with parameters.
func (c *Client) NewPostSystemOperationRequest(operationName string, async bool, parameters fm.Parameters) (*http.Request, error) {
	return c.newPostOperationRequest(c.baseURL.JoinPath("$"+operationName).String(), async, parameters)
}

// NewPostTypeOperationRequest creates a new type-level operation request that will use POST with parameters.
func (c *Client) NewPostTypeOperationRequest(resourceType string, operationName string, async bool, parameters fm.Parameters) (*http.Request, error) {
	return c.newPostOperationRequest(c.baseURL.JoinPath(resourceType, "$"+operationName).String(), async, parameters)
}

// NewPostInstanceOperationRequest creates a new instance-level operation request that will use POST with parameters.
func (c *Client) NewPostInstanceOperationRequest(resourceType string, resourceId string, operationName string, async bool, parameters fm.Parameters) (*http.Request, error) {
	return c.newPostOperationRequest(c.baseURL.JoinPath(resourceType, resourceId, "$"+operationName).String(), async, parameters)
}

// NewHistorySystemRequest creates a new history system interaction request that will use GET on a
// FHIR history endpoint.
func (c *Client) NewHistorySystemRequest() (*http.Request, error) {
	_url := c.baseURL.JoinPath("_history")
	req, err := http.NewRequest("GET", _url.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
	return req, nil
}

// newGetOperationRequest creates a new operation request that will use GET with
// parameters in the query params of the given operation URL.
func (c *Client) newGetOperationRequest(operationUrl *url.URL, async bool, parameters url.Values) (*http.Request, error) {
	operationUrl.RawQuery = parameters.Encode()
	req, err := http.NewRequest("GET", operationUrl.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)
	if async {
		req.Header.Add("Prefer", "respond-async")
	}
	return req, nil
}

// NewTypeOperationRequest creates a new operation request that will use GET with parameters in the query params of the URL.
func (c *Client) NewTypeOperationRequest(resourceType string, operationName string, async bool, parameters url.Values) (*http.Request, error) {
	return c.newGetOperationRequest(c.baseURL.JoinPath(resourceType, "$"+operationName), async, parameters)
}

// NewInstanceOperationRequest creates a new instance-level operation request that will use GET with parameters in the query params of the URL.
func (c *Client) NewInstanceOperationRequest(resourceType string, resourceId string, operationName string, async bool, parameters url.Values) (*http.Request, error) {
	return c.newGetOperationRequest(c.baseURL.JoinPath(resourceType, resourceId, "$"+operationName), async, parameters)
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
				return c.handlePollOkResponse(resp)
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

func (c *Client) handlePollOkResponse(resp *http.Response) ([]byte, error) {
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

		// A 2xx status indicates success. Blaze returns 200 for the GET form
		// and 201 (Created) for the POST form of an operation like
		// $evaluate-measure, because the latter persists its result resource.
		if !strings.HasPrefix(response.Status, "2") {
			if response.Outcome == nil {
				return nil, fmt.Errorf("error status: %s", response.Status)
			}

			var operationOutcome fm.OperationOutcome
			if err := json.Unmarshal(response.Outcome, &operationOutcome); err != nil {
				return nil, fmt.Errorf("error while reading the outcome of an error response in the async response bundle: %w", err)
			}

			return nil, fmt.Errorf("%w", &operationOutcomeError{outcome: &operationOutcome})
		}

		// The POST form of an operation like $evaluate-measure persists its
		// result resource and returns only its location (201 Created) instead
		// of an inline resource, so we have to fetch it.
		if len(bundle.Entry[0].Resource) == 0 && response.Location != nil {
			return c.fetchResource(*response.Location)
		}

		return bundle.Entry[0].Resource, nil
	} else {
		return nil, fmt.Errorf("non FHIR response")
	}
}

// fetchResource reads the resource at the given location using GET.
func (c *Client) fetchResource(location string) ([]byte, error) {
	req, err := http.NewRequest("GET", location, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add(HeaderAccept, MediaTypeFhirJson)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, handleErrorResponse(resp)
	}

	defer DiscardAndClose(resp.Body)
	return io.ReadAll(resp.Body)
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

// networkStats describes network statistics that arise when downloading resources from
// a FHIR server.
type networkStats struct {
	RequestDuration, ProcessingDuration float64
	TotalBytesIn                        int64
}

// DownloadBundle describes the result of downloading a single page of resources from a FHIR server.
// The response body of a successfully downloaded page is streamed and has to be
// consumed by WriteResources.
type DownloadBundle struct {
	AssociatedRequestURL url.URL
	Err                  error
	Stats                *networkStats
	ErrResponse          *util.ErrorResponse
	body                 *responseBody
}

// DownloadBundleError creates a downloadResource instance with an error attached to it.
// The error is formatted using the given format with all potential substitutions.
func DownloadBundleError(format string, a ...interface{}) DownloadBundle {
	return DownloadBundle{
		Err: fmt.Errorf(format, a...),
	}
}

// responseBody is the streamed response body of a downloaded page. It counts
// the bytes read and keeps the first read error, so that it can be told apart
// from an invalid bundle.
type responseBody struct {
	body         io.ReadCloser
	stats        *networkStats
	requestStart time.Time
	readErr      error
	// nextLink receives the next link of the bundle or nil if there is none.
	// It is nil itself if the next link was already known from the Link header
	// or was reported already.
	nextLink chan<- *url.URL
}

func (b *responseBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	b.stats.TotalBytesIn += int64(n)
	if err != nil && err != io.EOF && b.readErr == nil {
		b.readErr = err
	}
	return n, err
}

// reportNextLink reports nextLink as the next link of the bundle, if the next
// link wasn't known already.
func (b *responseBody) reportNextLink(nextLink *url.URL) {
	if b.nextLink != nil {
		b.nextLink <- nextLink
		b.nextLink = nil
	}
}

// WriteResources streams the response body of b and writes the resource of
// each bundle entry to sink, so that all information resembles a valid NDJSON
// stream. Has to be called for each bundle without error, because the request
// of the next page can depend on the links of this bundle.
//
// Always returns the number of written resources alongside all encountered
// inline operation outcomes, also if there is an error. An error can only occur
// if reading the response body or writing to sink fails or the bundle is
// invalid.
func (b DownloadBundle) WriteResources(sink io.Writer) (int, []*fm.OperationOutcome, error) {
	body := b.body
	resources, outcomes, err := writeResources(body, sink, body.reportNextLink)
	body.reportNextLink(nil)
	if err == nil {
		// reads the rest of the body, so that the connection can be reused
		_, err = io.Copy(io.Discard, body)
	}
	closeErr := body.body.Close()
	body.stats.RequestDuration = time.Since(body.requestStart).Seconds()
	if body.readErr != nil {
		return resources, outcomes, fmt.Errorf("could not read the response body: %w", body.readErr)
	}
	if err != nil {
		return resources, outcomes, err
	}
	if closeErr != nil {
		return resources, outcomes, fmt.Errorf("could not close the response body: %w", closeErr)
	}
	return resources, outcomes, nil
}

// ExpandPages downloads the page of initialRequest and all following pages and
// sends them in order to resChannel. The response body of each page is streamed
// by DownloadBundle.WriteResources. The next page is requested as soon as its
// link is known, either from the Link header or from the links of the bundle,
// while the current page is still being streamed. So with an unbuffered
// resChannel, at most one page is requested ahead. Stops after the first
// error, which is sent to resChannel.
func (c *Client) ExpandPages(initialRequest *http.Request, resChannel chan<- DownloadBundle) {
	request := initialRequest
	for {
		bundle, nextLinkChannel := c.downloadPage(request)
		resChannel <- bundle
		if bundle.Err != nil {
			return
		}
		nextLink := <-nextLinkChannel
		if nextLink == nil {
			return
		}
		var err error
		request, err = c.NewPaginatedRequest(nextLink)
		if err != nil {
			resChannel <- DownloadBundleError("could not create FHIR server request: %v\n", err)
			return
		}
	}
}

// downloadPage sends request and returns the downloaded page with its body
// still to be streamed. The returned channel receives the next link of the page
// or nil if there is none. It isn't used if the bundle has an error.
func (c *Client) downloadPage(request *http.Request) (DownloadBundle, <-chan *url.URL) {
	var stats networkStats
	var requestStart time.Time
	var processingStart time.Time

	trace := &httptrace.ClientTrace{
		GotConn: func(_ httptrace.GotConnInfo) {
			requestStart = time.Now()
		},
		WroteRequest: func(_ httptrace.WroteRequestInfo) {
			processingStart = time.Now()
		},
		GotFirstResponseByte: func() {
			stats.ProcessingDuration = time.Since(processingStart).Seconds()
		},
	}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))

	response, err := c.Do(request)
	if err != nil {
		return DownloadBundleError("could not request the FHIR server with URL %s: %v\n", request.URL, err), nil
	}

	if response.StatusCode != http.StatusOK {
		return errorResponseBundle(request, response, requestStart, &stats), nil
	}

	nextLink := make(chan *url.URL, 1)
	body := &responseBody{body: response.Body, stats: &stats, requestStart: requestStart}
	if linkHeader := response.Header.Get("Link"); linkHeader != "" {
		link, err := nextLinkFromHeader(linkHeader)
		if err != nil {
			_ = response.Body.Close()
			return DownloadBundleError("could not parse the self link from the Link header after request to URL %s: %v", request.URL, err), nil
		}
		nextLink <- link
	} else {
		body.nextLink = nextLink
	}

	return DownloadBundle{
		AssociatedRequestURL: *request.URL,
		Stats:                &stats,
		body:                 body,
	}, nextLink
}

// errorResponseBundle reads the operation outcome of the non-ok response and
// returns a bundle with the error.
func errorResponseBundle(request *http.Request, response *http.Response, requestStart time.Time, stats *networkStats) DownloadBundle {
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return DownloadBundleError("could not read FHIR server response after request to URL %s: %v\n", request.URL, err)
	}
	if err := response.Body.Close(); err != nil {
		return DownloadBundleError("could not close the response body: %v\n", err)
	}
	stats.RequestDuration = time.Since(requestStart).Seconds()
	stats.TotalBytesIn += int64(len(responseBody))

	outcome, err := fm.UnmarshalOperationOutcome(responseBody)
	if err != nil {
		bundle := DownloadBundleError("request to FHIR server with URL %s had a non-ok response status (%d) but the expected operation outcome could not be parsed: %v", request.URL, response.StatusCode, err)
		bundle.Stats = stats
		return bundle
	}

	bundle := DownloadBundleError("request to FHIR server with URL %s had a non-ok response status (%d)", request.URL, response.StatusCode)
	bundle.ErrResponse = &util.ErrorResponse{
		StatusCode:       response.StatusCode,
		OperationOutcome: &outcome,
	}
	bundle.Stats = stats
	return bundle
}

// nextLinkFromHeader extracts the URL to the next resource bundle page from a given
// HTTP Link header string.
// The extraction follows RFC 8288 (Web Linking) specification for parsing
// Link headers with relation types: https://tools.ietf.org/html/rfc8288
//
// Returns the URL to the next resource bundle page if there is any or nil.
// An error is returned if there is a URL, but it cannot be parsed.
func nextLinkFromHeader(linkHeader string) (*url.URL, error) {
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		parts := strings.Split(link, ";")
		if len(parts) == 2 && strings.TrimSpace(parts[1]) == `rel="next"` {
			linkUrl := strings.Trim(parts[0], "<> ")
			return url.ParseRequestURI(linkUrl)
		}
	}

	return nil, nil
}

// readNextLinks reads the links of a bundle from dec and returns the URL of the
// first link with relation next or nil if there is none.
//
// The extraction respects the FHIR specification with regard to how links are
// defined: https://www.iana.org/assignments/link-relations/link-relations.xhtml#link-relations-1
func readNextLinks(dec *jsontext.Decoder) (*url.URL, error) {
	var nextLink *url.URL
	for err := range elements(dec) {
		if err != nil {
			return nil, err
		}
		link, err := readNextLink(dec)
		if err != nil {
			return nil, err
		}
		if nextLink == nil {
			nextLink = link
		}
	}
	return nextLink, nil
}

// readNextLink reads a bundle link from dec and returns its URL if its
// relation is next or nil otherwise.
func readNextLink(dec *jsontext.Decoder) (*url.URL, error) {
	var next bool
	var link string
	for name, err := range members(dec) {
		if err != nil {
			return nil, err
		}
		switch string(name) {
		case "relation":
			var relation []byte
			relation, err = readString(dec)
			next = string(relation) == "next"
		case "url":
			var value []byte
			value, err = readString(dec)
			link = string(value)
		default:
			err = dec.SkipValue()
		}
		if err != nil {
			return nil, err
		}
	}
	if !next {
		return nil, nil
	}
	return url.ParseRequestURI(link)
}
