package fhir

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_DoWithBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if len(authHeader) == 0 || !strings.HasPrefix(authHeader, "Basic") {
			t.FailNow()
		}
	}))
	defer server.Close()

	client := Client{Base: server.URL, BasicAuthUser: "foo", BasicAuthPassword: "bar"}

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	_, _ = client.Do(req)
}

func TestClient_DoWithBasicAuthWithoutPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if len(authHeader) == 0 || !strings.HasPrefix(authHeader, "Basic") {
			t.FailNow()
		}
	}))
	defer server.Close()

	client := Client{Base: server.URL, BasicAuthUser: "foo", BasicAuthPassword: ""}

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	_, _ = client.Do(req)
}

func TestClient_DoWithoutBasicAuth(t *testing.T) {
	// we need a handler to check whether the basic auth was NOT set
	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if len(authHeader) != 0 {
			t.FailNow()
		}
	}))
	defer server.Close()

	client := Client{Base: server.URL, BasicAuthUser: "", BasicAuthPassword: ""}

	req, _ := http.NewRequest("GET", "/", nil)
	_, _ = client.Do(req)
}
