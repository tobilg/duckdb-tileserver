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
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tobilg/duckdb-tileserver/internal/ui"
)

// serveMapViewer serves the HTML map viewer page
func serveMapViewer(w http.ResponseWriter, r *http.Request) *appError {
	log.Debug("Map viewer request")

	// Load the template
	templ, err := ui.LoadTemplate("index.gohtml")
	if err != nil {
		return appErrorInternal(err, "Error loading map viewer template")
	}

	// Execute the standalone template directly (it's a complete HTML page)
	w.Header().Set("Content-Type", ContentTypeHTML)
	w.WriteHeader(http.StatusOK)
	err = templ.Execute(w, nil)
	if err != nil {
		return appErrorInternal(err, "Error rendering map viewer")
	}

	return nil
}
