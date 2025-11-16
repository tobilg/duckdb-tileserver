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
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/tobilg/duckdb-tileserver/internal/data"
)

// handleTile serves MVT tiles for a given layer and tile coordinates
func handleTile(w http.ResponseWriter, r *http.Request) *appError {
	vars := mux.Vars(r)
	layer := vars["layer"]
	zStr := vars["z"]
	xStr := vars["x"]
	yStr := vars["y"]

	// Parse tile coordinates
	z, err := strconv.Atoi(zStr)
	if err != nil {
		return appErrorBadRequest(err, fmt.Sprintf("Invalid zoom level: %s", zStr))
	}

	x, err := strconv.Atoi(xStr)
	if err != nil {
		return appErrorBadRequest(err, fmt.Sprintf("Invalid x coordinate: %s", xStr))
	}

	y, err := strconv.Atoi(yStr)
	if err != nil {
		return appErrorBadRequest(err, fmt.Sprintf("Invalid y coordinate: %s", yStr))
	}

	// Validate tile coordinates
	if z < 0 || z > 30 {
		return appErrorBadRequest(nil, fmt.Sprintf("Zoom level out of range: %d", z))
	}

	maxCoord := 1 << uint(z) // 2^z
	if x < 0 || x >= maxCoord {
		return appErrorBadRequest(nil, fmt.Sprintf("X coordinate out of range: %d (max: %d)", x, maxCoord-1))
	}

	if y < 0 || y >= maxCoord {
		return appErrorBadRequest(nil, fmt.Sprintf("Y coordinate out of range: %d (max: %d)", y, maxCoord-1))
	}

	log.Debugf("Tile request: layer=%s z=%d x=%d y=%d", layer, z, x, y)

	// Get catalog instance (cast to access tile methods)
	catDB, ok := catalogInstance.(*data.CatalogDB)
	if !ok {
		return appErrorInternal(nil, "Invalid catalog type")
	}

	// Generate the tile
	tileData, err := catDB.GenerateTile(r.Context(), layer, z, x, y)
	if err != nil {
		if err.Error() == fmt.Sprintf("layer not found: %s", layer) {
			return appErrorNotFound(err, fmt.Sprintf("Layer not found: %s", layer))
		}
		return appErrorInternal(err, fmt.Sprintf("Error generating tile: %v", err))
	}

	// Return empty tile as 204 No Content if there's no data
	if len(tileData) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	// Write the tile data
	w.Header().Set("Content-Type", ContentTypeMVT)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(tileData)
	if err != nil {
		return appErrorInternal(err, "Error writing tile data")
	}

	return nil
}

// handleTileJSON serves TileJSON metadata for a layer
func handleTileJSON(w http.ResponseWriter, r *http.Request) *appError {
	vars := mux.Vars(r)
	layer := vars["layer"]

	log.Debugf("TileJSON request for layer: %s", layer)

	// Get catalog instance
	catDB, ok := catalogInstance.(*data.CatalogDB)
	if !ok {
		return appErrorInternal(nil, "Invalid catalog type")
	}

	// Get base URL for tile URLs
	baseURL := getBaseURL(r)

	// Generate TileJSON
	tileJSON, err := catDB.GetTileJSON(layer, baseURL)
	if err != nil {
		if err.Error() == fmt.Sprintf("layer not found: %s", layer) {
			return appErrorNotFound(err, fmt.Sprintf("Layer not found: %s", layer))
		}
		return appErrorInternal(err, fmt.Sprintf("Error generating TileJSON: %v", err))
	}

	// Return JSON response
	return writeJSON(w, ContentTypeJSON, tileJSON)
}
