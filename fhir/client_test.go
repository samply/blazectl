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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
)

func TestBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		username, password, ok := req.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "foo", username)
		assert.Equal(t, "bar", password)
	}))
	defer server.Close()

	auth := BasicAuth{User: "foo", Password: "bar"}
	baseURL, _ := url.ParseRequestURI(server.URL)
	client := NewClient(*baseURL, auth)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	_, _ = client.Do(req)
}

func TestTokenAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		header := req.Header.Get("Authorization")
		assert.Equal(t, "Bearer foo", header)
	}))
	defer server.Close()

	auth := TokenAuth{Token: "foo"}
	baseURL, _ := url.ParseRequestURI(server.URL)
	client := NewClient(*baseURL, auth)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	_, _ = client.Do(req)
}

func TestWithoutBasicAuth(t *testing.T) {
	// we need a handler to check whether the basic auth was NOT set
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		_, _, ok := req.BasicAuth()
		assert.False(t, ok)
	}))
	defer server.Close()

	baseURL, _ := url.ParseRequestURI(server.URL)
	client := NewClient(*baseURL, nil)

	req, _ := http.NewRequest("GET", "/", nil)
	_, _ = client.Do(req)
}

func TestNewCapabilitiesRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	req, err := client.NewCapabilitiesRequest()
	if err != nil {
		t.Fatalf("could not create a capabilities request: %v", err)
	}

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/some-path/metadata", req.URL.Path)
}

func TestNewTransactionRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	req, err := client.NewTransactionRequest(bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("could not create a transaction request: %v", err)
	}

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/some-path", req.URL.Path)
}

func TestNewSearchTypeRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	query, _ := url.ParseQuery("")
	req, err := client.NewSearchTypeRequest("some-type", query)
	if err != nil {
		t.Fatalf("could not create a search-type request: %v", err)
	}

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/some-path/some-type", req.URL.Path)
}

func TestNewPostSearchTypeRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	query, _ := url.ParseQuery("")
	req, err := client.NewPostSearchTypeRequest("some-type", query)
	if err != nil {
		t.Fatalf("could not create a search-type request: %v", err)
	}

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "/some-path/some-type/_search", req.URL.Path)
}

func TestNewSearchSystemRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	query, _ := url.ParseQuery("")
	req, err := client.NewSearchSystemRequest(query)
	if err != nil {
		t.Fatalf("could not create a search-system request: %v", err)
	}

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/some-path", req.URL.Path)
}

func TestNewTypeOperationRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	parameters, _ := url.ParseQuery("")
	req, err := client.NewTypeOperationRequest("some-type", "some-operation", false, parameters)
	if err != nil {
		t.Fatalf("could not create a search-system request: %v", err)
	}

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/some-path/some-type/$some-operation", req.URL.Path)
	assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderAccept))
}

func TestNewAsyncTypeOperationRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	parameters, _ := url.ParseQuery("")
	req, err := client.NewTypeOperationRequest("some-type", "some-operation", true, parameters)
	if err != nil {
		t.Fatalf("could not create a search-system request: %v", err)
	}

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/some-path/some-type/$some-operation", req.URL.Path)
	assert.Equal(t, "respond-async", req.Header.Get("Prefer"))
	assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderAccept))
}

func TestClientSecurity(t *testing.T) {
	crt, key, err := createSelfSignedCertificate()
	if err != nil {
		t.Fatalf("could not create self-signed certificate: %v", err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
	}))

	tlsCrt := tls.Certificate{
		Certificate: [][]byte{crt.Raw},
		Leaf:        crt,
		PrivateKey:  key,
	}

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{tlsCrt},
	}
	server.StartTLS()
	defer server.Close()

	log.Println(server.URL)
	baseUrl, _ := url.ParseRequestURI(server.URL)
	req, _ := http.NewRequest("GET", server.URL, nil)

	t.Run("ClientWithEnabledSecurityFailsOnSelfSignedCertificate", func(t *testing.T) {
		client := NewClient(*baseUrl, nil)
		_, err := client.Do(req)
		assert.NotNil(t, err, "expected request to fail")
	})

	t.Run("ClientWithDisabledSecuritySucceedsOnSelfSignedCertificate", func(t *testing.T) {
		client := NewClientInsecure(*baseUrl, nil)
		_, err := client.Do(req)
		assert.Nil(t, err, "expected request to succeed")
	})
}

func createSelfSignedCertificate() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("could not generate a key pair: %v", err)
	}

	certificateTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Samply Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Minute * 10),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certificate, err := x509.CreateCertificate(rand.Reader, &certificateTemplate, &certificateTemplate,
		&privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("could not generate self-signed certificate: %v", err)
	}

	selfSignedCertificate, err := x509.ParseCertificate(certificate)
	if err != nil {
		return nil, nil, fmt.Errorf("could not parse parse self-signed certificate: %v", err)
	}

	return selfSignedCertificate, privateKey, nil
}

func TestNewClientCa(t *testing.T) {
	// Create a temporary CA certificate file
	crt, _, err := createSelfSignedCertificate()
	if err != nil {
		t.Fatalf("could not create self-signed certificate: %v", err)
	}

	// Write certificate to a temporary file
	certFile, err := os.CreateTemp("", "ca-cert-*.pem")
	if err != nil {
		t.Fatalf("could not create temporary file: %v", err)
	}
	defer os.Remove(certFile.Name())

	// Write certificate in PEM format
	if err := os.WriteFile(certFile.Name(), crt.Raw, 0644); err != nil {
		t.Fatalf("could not write to temporary file: %v", err)
	}

	// Create client with CA certificate
	baseURL, _ := url.ParseRequestURI("https://example.com")
	client, err := NewClientCa(*baseURL, nil, certFile.Name())

	// Verify client was created successfully
	assert.Nil(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "https://example.com", client.baseURL.String())
}

func TestNewHistoryTypeRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	req, err := client.NewHistoryTypeRequest("some-type")
	if err != nil {
		t.Fatalf("could not create a history-type request: %v", err)
	}

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/some-path/some-type/_history", req.URL.Path)
	assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderAccept))
}

func TestNewHistoryResourceRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	req, err := client.NewHistoryInstanceRequest("some-type", "some-id")
	if err != nil {
		t.Fatalf("could not create a history-resource request: %v", err)
	}

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/some-path/some-type/some-id/_history", req.URL.Path)
	assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderAccept))
}

func TestNewPostSystemOperationRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	testValue := "test-value"
	parameters := fm.Parameters{
		Parameter: []fm.ParametersParameter{
			{
				Name:        "test-param",
				ValueString: &testValue,
			},
		},
	}

	t.Run("synchronous request", func(t *testing.T) {
		req, err := client.NewPostSystemOperationRequest("some-operation", false, parameters)
		if err != nil {
			t.Fatalf("could not create a system operation request: %v", err)
		}

		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "/some-path/$some-operation", req.URL.Path)
		assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderAccept))
		assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderContentType))
		assert.Equal(t, "", req.Header.Get("Prefer"))

		// Verify request body contains the parameters
		body, err := io.ReadAll(req.Body)
		assert.Nil(t, err)
		var decodedParams fm.Parameters
		err = json.Unmarshal(body, &decodedParams)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(decodedParams.Parameter))
		assert.Equal(t, "test-param", decodedParams.Parameter[0].Name)
		assert.NotNil(t, decodedParams.Parameter[0].ValueString)
		assert.Equal(t, "test-value", *decodedParams.Parameter[0].ValueString)
	})

	t.Run("asynchronous request", func(t *testing.T) {
		req, err := client.NewPostSystemOperationRequest("some-operation", true, parameters)
		if err != nil {
			t.Fatalf("could not create an async system operation request: %v", err)
		}

		assert.Equal(t, "POST", req.Method)
		assert.Equal(t, "/some-path/$some-operation", req.URL.Path)
		assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderAccept))
		assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderContentType))
		assert.Equal(t, "respond-async", req.Header.Get("Prefer"))
	})
}

func TestNewHistorySystemRequest(t *testing.T) {
	parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
	client := NewClient(*parsedUrl, nil)

	req, err := client.NewHistorySystemRequest()
	if err != nil {
		t.Fatalf("could not create a history-system request: %v", err)
	}

	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "/some-path/_history", req.URL.Path)
	assert.Equal(t, MediaTypeFhirJson, req.Header.Get(HeaderAccept))
}

func TestPollAsyncStatus(t *testing.T) {
	pollAsyncStatus := func(server *httptest.Server) ([]byte, error) {
		baseURL, _ := url.ParseRequestURI(server.URL)
		client := NewClient(*baseURL, nil)
		interruptChan := make(chan os.Signal, 1)

		return client.PollAsyncStatus(server.URL+"/foo", interruptChan)
	}

	t.Run("async response with non FHIR response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		_, err := pollAsyncStatus(server)

		assert.Equal(t, "non FHIR response", err.Error())
	})

	t.Run("async response with invalid FHIR response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HeaderContentType, MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte{'{'})
			if err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		_, err := pollAsyncStatus(server)

		assert.Equal(t, "error while reading the async response bundle: unexpected EOF", err.Error())
	})

	t.Run("async response with bundle of different type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HeaderContentType, MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
			response := fm.Bundle{Type: fm.BundleTypeBatch}
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		_, err := pollAsyncStatus(server)

		assert.Equal(t, "expected batch-response bundle but the bundle type is: batch", err.Error())
	})

	t.Run("async response with missing bundle entry", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HeaderContentType, MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
			response := fm.Bundle{Type: fm.BundleTypeBatchResponse}
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		_, err := pollAsyncStatus(server)

		assert.Equal(t, "expected one entry in async response bundle but was 0 entries", err.Error())
	})

	t.Run("async response with error bundle entry without outcome", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HeaderContentType, MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
			response := fm.Bundle{
				Type: fm.BundleTypeBatchResponse,
				Entry: []fm.BundleEntry{{
					Response: &fm.BundleEntryResponse{
						Status: "400 Bad Request",
					},
				}},
			}
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		_, err := pollAsyncStatus(server)

		assert.Equal(t, "error status: 400 Bad Request", err.Error())
	})

	t.Run("async response with error bundle entry with invalid outcome", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HeaderContentType, MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
			response := fm.Bundle{
				Type: fm.BundleTypeBatchResponse,
				Entry: []fm.BundleEntry{{
					Response: &fm.BundleEntryResponse{
						Status:  "400 Bad Request",
						Outcome: json.RawMessage("[]"),
					},
				}},
			}
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		_, err := pollAsyncStatus(server)

		assert.Equal(t, "error while reading the outcome of an error response in the async response bundle: json: cannot unmarshal JSON array into Go type fhir.OperationOutcome", err.Error())
	})

	t.Run("async response with error bundle entry with outcome", func(t *testing.T) {
		outcome := fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{{
				Severity: fm.IssueSeverityError,
				Code:     fm.IssueTypeValue,
			}},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HeaderContentType, MediaTypeFhirJson)
			w.WriteHeader(http.StatusOK)
			outcomeBytes, err := json.Marshal(outcome)
			if err != nil {
				t.Error(err)
			}
			response := fm.Bundle{
				Type: fm.BundleTypeBatchResponse,
				Entry: []fm.BundleEntry{{
					Response: &fm.BundleEntryResponse{
						Status:  "400 Bad Request",
						Outcome: outcomeBytes,
					},
				}},
			}
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		_, err := pollAsyncStatus(server)

		assert.Equal(t, "Severity    : Error\nCode        : An element or header value is invalid.\n", err.Error())
	})

	t.Run("async error response with non FHIR response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		baseURL, _ := url.ParseRequestURI(server.URL)
		client := NewClient(*baseURL, nil)
		interruptChan := make(chan os.Signal, 1)

		_, err := client.PollAsyncStatus(server.URL+"/foo", interruptChan)

		assert.Equal(t, "non FHIR response", err.Error())
	})

	t.Run("async error response with FHIR OperationOutcome response", func(t *testing.T) {
		outcome := fm.OperationOutcome{
			Issue: []fm.OperationOutcomeIssue{{
				Severity: fm.IssueSeverityError,
				Code:     fm.IssueTypeConflict,
			}},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(HeaderContentType, MediaTypeFhirJson)
			w.WriteHeader(http.StatusConflict)
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(outcome); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()

		_, err := pollAsyncStatus(server)

		assert.Equal(t, "Severity    : Error\nCode        : Content could not be accepted because of an edit conflict (i.e. version aware updates). (In a pure RESTful environment, this would be an HTTP 409 error, but this code may be used where the conflict is discovered further into the application architecture.).\n", err.Error())
	})
}
