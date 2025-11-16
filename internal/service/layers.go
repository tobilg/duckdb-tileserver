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

	log "github.com/sirupsen/logrus"
	"github.com/tobilg/duckdb-tileserver/internal/data"
)

// LayersResponse represents the JSON response for the /layers endpoint
type LayersResponse struct {
	Layers []*data.Layer `json:"layers"`
}

// handleLayers returns a list of all available spatial layers
func handleLayers(w http.ResponseWriter, r *http.Request) *appError {
	log.Debug("Layers request")

	// Get catalog instance
	catDB, ok := catalogInstance.(*data.CatalogDB)
	if !ok {
		return appErrorInternal(nil, "Invalid catalog type")
	}

	// Get all layers
	layers, err := catDB.GetLayers()
	if err != nil {
		return appErrorInternal(err, fmt.Sprintf("Error retrieving layers: %v", err))
	}

	response := LayersResponse{
		Layers: layers,
	}

	return writeJSON(w, ContentTypeJSON, response)
}
