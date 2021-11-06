// Copyright 2019 - 2021 The Samply Community
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
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestClient_withBasicAuth(t *testing.T) {
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

func TestClient_withBasicAuthWithoutPassword(t *testing.T) {
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

func TestClient_withoutBasicAuth(t *testing.T) {
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
	t.Run("BaseURL without path", func(t *testing.T) {
		parsedUrl, _ := url.ParseRequestURI("http://localhost:8080")
		client := NewClient(*parsedUrl, ClientAuth{})

		assert.Empty(t, client.baseURL.Path)
	})

	t.Run("BaseURL with path ending without slash", func(t *testing.T) {
		parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path")
		client := NewClient(*parsedUrl, ClientAuth{})

		assert.NotEmpty(t, client.baseURL.Path)
		assert.True(t, strings.HasSuffix(client.baseURL.Path, "some-path/"))
	})

	t.Run("BaseURL with path ending with slash", func(t *testing.T) {
		parsedUrl, _ := url.ParseRequestURI("http://localhost:8080/some-path/")
		client := NewClient(*parsedUrl, ClientAuth{})

		assert.NotEmpty(t, client.baseURL.Path)
		assert.True(t, strings.HasSuffix(client.baseURL.Path, "some-path/"))
	})
}
