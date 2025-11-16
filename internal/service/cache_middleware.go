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
	"bytes"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/tobilg/duckdb-tileserver/internal/conf"
)

// tileCacheMiddleware wraps the tile handler to check cache first
func (s *Service) tileCacheMiddleware(next appHandler) appHandler {
	return func(w http.ResponseWriter, r *http.Request) *appError {
		// Skip cache if service or cache is not initialized
		if s == nil || s.cache == nil || !s.cache.Enabled() {
			return next(w, r)
		}

		// Extract tile coordinates from URL
		vars := mux.Vars(r)
		layer := vars["layer"]
		z := vars["z"]
		x := vars["x"]
		y := vars["y"]

		// Build cache key
		cacheKey := fmt.Sprintf("%s:%s:%s:%s", layer, z, x, y)

		// Try cache first
		if cachedTile, found := s.cache.Get(r.Context(), cacheKey); found {
			// Cache hit - return immediately
			w.Header().Set("Content-Type", "application/vnd.mapbox-vector-tile")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("X-Cache", "HIT")
			// Allow browser caching
			maxAge := conf.Configuration.Cache.BrowserCacheMaxAge
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))

			if len(cachedTile) == 0 {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write(cachedTile)
			}
			return nil
		}

		// Cache miss - set headers before calling next handler
		w.Header().Set("X-Cache", "MISS")
		// Allow browser caching
		maxAge := conf.Configuration.Cache.BrowserCacheMaxAge
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))

		// Capture the response to store it
		recorder := &responseCapturer{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
		}

		// Call original handler
		appErr := next(recorder, r)

		// If successful, store in cache (async to not block response)
		if appErr == nil && recorder.statusCode == http.StatusOK {
			go s.cache.Set(r.Context(), cacheKey, recorder.body.Bytes())
		}

		// Also cache empty tiles (204 No Content)
		if appErr == nil && recorder.statusCode == http.StatusNoContent {
			go s.cache.Set(r.Context(), cacheKey, []byte{})
		}

		return appErr
	}
}

// responseCapturer captures the response body to store in cache
type responseCapturer struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (rc *responseCapturer) Write(b []byte) (int, error) {
	// If WriteHeader wasn't called explicitly, assume 200 OK
	if rc.statusCode == 0 {
		rc.statusCode = http.StatusOK
	}

	// Capture body
	rc.body.Write(b)

	// Write to original response
	return rc.ResponseWriter.Write(b)
}

func (rc *responseCapturer) WriteHeader(statusCode int) {
	rc.statusCode = statusCode
	rc.ResponseWriter.WriteHeader(statusCode)
}
