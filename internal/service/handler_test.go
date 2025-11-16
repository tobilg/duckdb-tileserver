package service

/*
 Copyright 2019 - 2025 Crunchy Data Solutions, Inc.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/tobilg/duckdb-tileserver/internal/cache"
	"github.com/tobilg/duckdb-tileserver/internal/conf"
	"github.com/tobilg/duckdb-tileserver/internal/data"
)

func init() {
	// Initialize minimal config for testing
	conf.Configuration.Server.AssetsPath = "../../assets"
	conf.Configuration.Metadata.Title = "Test Tileserver"
	conf.Configuration.Metadata.Description = "Test Description"
	conf.Configuration.Cache.Enabled = false // Disable cache for tests
}

func setupTestCatalog() {
	catalogInstance = data.CatMockInstance()

	// Initialize a test service instance with disabled cache
	serviceInstance = &Service{
		cache: cache.NewDisabledCache(),
	}
}

func TestHandleHealth(t *testing.T) {
	setupTestCatalog()

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := appHandler(handleHealth)
	handler.ServeHTTP(rr, req)

	var response HealthResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse health response: %v", err)
	}

	// Mock catalog returns error status (503) because it doesn't have a real DB connection
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusServiceUnavailable)
	}

	if response.Status != "error" {
		t.Errorf("Expected status 'error' for mock catalog, got '%s'", response.Status)
	}
}

func TestHandleRoot(t *testing.T) {
	setupTestCatalog()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := appHandler(handleRoot)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != ContentTypeHTML {
		t.Errorf("Expected Content-Type %s, got %s", ContentTypeHTML, contentType)
	}
}

func TestHandleTileInvalidCoordinates(t *testing.T) {
	setupTestCatalog()

	tests := []struct {
		name string
		url  string
		code int
	}{
		{"Invalid zoom", "/tiles/test/99/0/0.mvt", http.StatusBadRequest},
		{"Negative zoom", "/tiles/test/-1/0/0.mvt", http.StatusNotFound}, // Regex pattern doesn't match negative numbers
		{"Invalid x", "/tiles/test/10/9999/0.mvt", http.StatusBadRequest},
		{"Invalid y", "/tiles/test/10/0/9999.mvt", http.StatusBadRequest},
		{"Negative x", "/tiles/test/10/-1/0.mvt", http.StatusNotFound}, // Regex pattern doesn't match negative numbers
		{"Negative y", "/tiles/test/10/0/-1.mvt", http.StatusNotFound}, // Regex pattern doesn't match negative numbers
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.url, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			router := initRouter("")
			router.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.code {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.code)
			}
		})
	}
}

func TestRouter(t *testing.T) {
	router := initRouter("")

	tests := []struct {
		method string
		path   string
		match  bool
	}{
		{"GET", "/", true},
		{"GET", "/index.html", true},
		{"GET", "/health", true},
		{"GET", "/layers", true},
		{"GET", "/tiles/buildings.json", true},
		{"GET", "/tiles/buildings/10/512/384.mvt", true},
		{"GET", "/tiles/buildings/10/512/384.pbf", true},
		{"POST", "/", false},
		{"GET", "/invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			var match bool
			router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
				if route.Match(req, &mux.RouteMatch{}) {
					match = true
				}
				return nil
			})

			if match != tt.match {
				t.Errorf("Expected route match %v for %s %s, got %v", tt.match, tt.method, tt.path, match)
			}
		})
	}
}

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		scheme   string
		expected string
	}{
		{
			name:     "Simple HTTP",
			host:     "localhost:9000",
			scheme:   "http",
			expected: "http://localhost:9000",
		},
		{
			name:     "HTTPS",
			host:     "example.com",
			scheme:   "https",
			expected: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host
			if tt.scheme == "https" {
				req.TLS = &tls.ConnectionState{}
			}

			baseURL := getBaseURL(req)
			if baseURL != tt.expected {
				t.Errorf("Expected base URL %s, got %s", tt.expected, baseURL)
			}
		})
	}
}

func TestFormatTileURL(t *testing.T) {
	tests := []struct {
		baseURL  string
		layer    string
		expected string
	}{
		{
			baseURL:  "http://localhost:9000",
			layer:    "buildings",
			expected: "http://localhost:9000/tiles/buildings/{z}/{x}/{y}.mvt",
		},
		{
			baseURL:  "https://example.com",
			layer:    "roads",
			expected: "https://example.com/tiles/roads/{z}/{x}/{y}.mvt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.layer, func(t *testing.T) {
			result := formatTileURL(tt.baseURL, tt.layer)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
