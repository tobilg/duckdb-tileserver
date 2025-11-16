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
	"database/sql"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tobilg/duckdb-tileserver/internal/cache"
	"github.com/tobilg/duckdb-tileserver/internal/data"
)

// HealthResponse represents the JSON response for the /health endpoint
type HealthResponse struct {
	Status           string       `json:"status"`
	Database         string       `json:"database"`
	SpatialExtension string       `json:"spatial_extension"`
	Cache            CacheStatus  `json:"cache"`
}

// CacheStatus represents cache health information
type CacheStatus struct {
	Enabled bool          `json:"enabled"`
	Stats   *cache.Stats  `json:"stats,omitempty"`
}

// handleHealth returns health status of the service
func handleHealth(w http.ResponseWriter, r *http.Request) *appError {
	log.Debug("Health check request")

	health := HealthResponse{
		Status:           "ok",
		Database:         "unknown",
		SpatialExtension: "unknown",
	}

	// Check database connection
	catDB, ok := catalogInstance.(*data.CatalogDB)
	if !ok {
		health.Status = "error"
		health.Database = "disconnected"
		w.WriteHeader(http.StatusServiceUnavailable)
		return writeJSON(w, ContentTypeJSON, health)
	}

	// Get the underlying database connection to test it
	db := catDB.GetDB()
	if db == nil {
		health.Status = "error"
		health.Database = "disconnected"
		w.WriteHeader(http.StatusServiceUnavailable)
		return writeJSON(w, ContentTypeJSON, health)
	}

	// Ping database
	err := db.Ping()
	if err != nil {
		log.Warnf("Database ping failed: %v", err)
		health.Status = "error"
		health.Database = "disconnected"
		w.WriteHeader(http.StatusServiceUnavailable)
		return writeJSON(w, ContentTypeJSON, health)
	}
	health.Database = "connected"

	// Check if spatial extension is loaded
	var result sql.NullString
	err = db.QueryRow("SELECT ST_Point(0, 0)").Scan(&result)
	if err != nil {
		log.Warnf("Spatial extension check failed: %v", err)
		health.SpatialExtension = "not loaded"
		health.Status = "degraded"
	} else {
		health.SpatialExtension = "loaded"
	}

	// Add cache status
	cacheStatus := CacheStatus{
		Enabled: serviceInstance != nil && serviceInstance.cache != nil && serviceInstance.cache.Enabled(),
	}
	if cacheStatus.Enabled {
		stats := serviceInstance.cache.Stats()
		cacheStatus.Stats = &stats
	}
	health.Cache = cacheStatus

	// Set response status code based on health
	if health.Status == "ok" {
		w.WriteHeader(http.StatusOK)
	} else if health.Status == "degraded" {
		w.WriteHeader(http.StatusOK) // Still operational but degraded
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	return writeJSON(w, ContentTypeJSON, health)
}
