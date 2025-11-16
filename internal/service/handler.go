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
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/tobilg/duckdb-tileserver/internal/conf"
)

const (
	ContentTypeJSON = "application/json"
	ContentTypeHTML = "text/html; charset=utf-8"
	ContentTypeMVT  = "application/vnd.mapbox-vector-tile"
	ContentTypeText = "text/plain"
)

// initRouter sets up the HTTP routes
func initRouter(basePath string) *mux.Router {
	router := mux.NewRouter()

	// Apply base path if specified
	var r *mux.Router
	if basePath != "" {
		log.Infof("Using base path: %s", basePath)
		r = router.PathPrefix(basePath).Subrouter()
	} else {
		r = router
	}

	// Root endpoint - HTML map viewer
	r.Handle("/", appHandler(handleRoot)).Methods("GET")
	r.Handle("/index.html", appHandler(handleRoot)).Methods("GET")
	r.Handle("/home.html", appHandler(handleRoot)).Methods("GET")

	// Health check endpoint
	r.Handle("/health", appHandler(handleHealth)).Methods("GET")

	// Layers discovery endpoint
	r.Handle("/layers", appHandler(handleLayers)).Methods("GET")
	r.Handle("/layers.json", appHandler(handleLayers)).Methods("GET")

	// TileJSON metadata endpoint
	r.Handle("/tiles/{layer}.json", appHandler(handleTileJSON)).Methods("GET")

	// MVT tile endpoint (with cache middleware)
	r.Handle("/tiles/{layer}/{z:[0-9]+}/{x:[0-9]+}/{y:[0-9]+}.mvt", serviceInstance.tileCacheMiddleware(appHandler(handleTile))).Methods("GET")
	r.Handle("/tiles/{layer}/{z:[0-9]+}/{x:[0-9]+}/{y:[0-9]+}.pbf", serviceInstance.tileCacheMiddleware(appHandler(handleTile))).Methods("GET")

	// Cache management endpoints (conditionally registered)
	if !conf.Configuration.Cache.DisableApi {
		log.Info("Cache management endpoints enabled")
		// Apply authentication middleware if API key is configured
		r.Handle("/cache/stats", appHandler(cacheAuthMiddleware(serviceInstance.handleCacheStats))).Methods("GET")
		r.Handle("/cache/clear", appHandler(cacheAuthMiddleware(serviceInstance.handleCacheClear))).Methods("DELETE")
		r.Handle("/cache/layer/{layer}", appHandler(cacheAuthMiddleware(serviceInstance.handleCacheClearLayer))).Methods("DELETE")
	} else {
		log.Info("Cache management endpoints disabled")
	}

	// Log registered routes
	router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathTemplate, err := route.GetPathTemplate()
		if err == nil {
			log.Debugf("Registered route: %s", pathTemplate)
		}
		methods, err := route.GetMethods()
		if err == nil {
			log.Debugf("  Methods: %v", methods)
		}
		return nil
	})

	return router
}

// handleRoot serves the main HTML map viewer
func handleRoot(w http.ResponseWriter, r *http.Request) *appError {
	return serveMapViewer(w, r)
}

// getBaseURL constructs the base URL for the service
func getBaseURL(r *http.Request) string {
	// Remove trailing slash from serveURLBase
	base := serveURLBase(r)
	if len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	return base
}

// formatTileURL formats a tile URL pattern for use in map viewers
func formatTileURL(baseURL string, layer string) string {
	return fmt.Sprintf("%s/tiles/%s/{z}/{x}/{y}.mvt", baseURL, layer)
}
