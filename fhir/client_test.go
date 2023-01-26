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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if len(authHeader) == 0 || !strings.HasPrefix(authHeader, "Basic") {
			t.FailNow()
		}
	}))
	defer server.Close()

	auth := ClientAuth{BasicAuthUser: "foo", BasicAuthPassword: "bar"}
	baseURL, _ := url.ParseRequestURI(server.URL)
	client := NewClient(*baseURL, auth)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	_, _ = client.Do(req)
}

func TestBasicAuthWithoutPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if len(authHeader) == 0 || !strings.HasPrefix(authHeader, "Basic") {
			t.FailNow()
		}
	}))
	defer server.Close()

	auth := ClientAuth{BasicAuthUser: "foo", BasicAuthPassword: ""}
	baseURL, _ := url.ParseRequestURI(server.URL)
	client := NewClient(*baseURL, auth)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	_, _ = client.Do(req)
}

func TestWithoutBasicAuth(t *testing.T) {
	// we need a handler to check whether the basic auth was NOT set
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if len(authHeader) != 0 {
			t.FailNow()
		}
	}))
	defer server.Close()

	auth := ClientAuth{BasicAuthUser: "", BasicAuthPassword: ""}
	baseURL, _ := url.ParseRequestURI(server.URL)
	client := NewClient(*baseURL, auth)

	req, _ := http.NewRequest("GET", "/", nil)
	_, _ = client.Do(req)
}

func TestNewClient(t *testing.T) {
	t.Run("BaseUrlWithoutPath", func(t *testing.T) {
		parsedUrl, _ := url.ParseRequestURI("http://localhost:8080")
		client := NewClient(*parsedUrl, ClientAuth{})

		assert.Empty(t, client.baseURL.Path)
	})

	t.Run("BaseUrlWithPathWndingWithoutSlash", func(t *testing.T) {
		parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
		client := NewClient(*parsedUrl, ClientAuth{})

		assert.NotEmpty(t, client.baseURL.Path)
		assert.True(t, strings.HasSuffix(client.baseURL.Path, "some-path/"))
	})

	t.Run("BaseUrlWithPathEndingWithSlash", func(t *testing.T) {
		parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path/")
		client := NewClient(*parsedUrl, ClientAuth{})

		assert.NotEmpty(t, client.baseURL.Path)
		assert.True(t, strings.HasSuffix(client.baseURL.Path, "some-path/"))
	})
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
		client := NewClient(*baseUrl, ClientAuth{})
		_, err := client.Do(req)
		assert.NotNil(t, err, "expected request to fail")
	})

	t.Run("ClientWithDisabledSecuritySucceedsOnSelfSignedCertificate", func(t *testing.T) {
		client := NewClientInsecure(*baseUrl, ClientAuth{})
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
