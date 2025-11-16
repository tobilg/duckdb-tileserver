package data

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
	"context"
	"database/sql"
	"fmt"
	"math"

	log "github.com/sirupsen/logrus"
)

const (
	SRID_3857 = 3857 // Web Mercator
)

// Layer represents a spatial layer that can serve MVT tiles
type Layer struct {
	Name           string            `json:"name"`
	Table          string            `json:"table"`
	GeometryColumn string            `json:"geometry_column"`
	GeometryType   string            `json:"geometry_type"`
	Srid           int               `json:"srid"`           // SRID of bounds (always 3857 for API responses)
	SourceSrid     int               `json:"-"`              // SRID of source data (not exposed in API)
	Bounds         *Extent           `json:"bounds,omitempty"`
	Properties     []string          `json:"properties,omitempty"`
	PropertyTypes  map[string]string `json:"-"`              // Column name -> data type mapping (not exposed in API)
}

// TileJSON represents the TileJSON specification metadata
type TileJSON struct {
	TileJSON    string   `json:"tilejson"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Scheme      string   `json:"scheme,omitempty"`
	Tiles       []string `json:"tiles"`
	MinZoom     int      `json:"minzoom,omitempty"`
	MaxZoom     int      `json:"maxzoom,omitempty"`
	Bounds      []float64 `json:"bounds,omitempty"`
	Center      []float64 `json:"center,omitempty"`
	VectorLayers []VectorLayer `json:"vector_layers,omitempty"`
}

// VectorLayer represents a layer in the TileJSON spec
type VectorLayer struct {
	ID          string            `json:"id"`
	Description string            `json:"description,omitempty"`
	MinZoom     int               `json:"minzoom,omitempty"`
	MaxZoom     int               `json:"maxzoom,omitempty"`
	Fields      map[string]string `json:"fields,omitempty"`
}

// GetLayers returns all tables with geometry columns
func (cat *CatalogDB) GetLayers() ([]*Layer, error) {
	query := `
		SELECT
			table_name,
			column_name as geometry_column
		FROM duckdb_columns
		WHERE data_type = 'GEOMETRY'
		ORDER BY table_name
	`

	rows, err := cat.dbconn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying layers: %w", err)
	}
	defer rows.Close()

	var layers []*Layer
	seenTables := make(map[string]bool)

	for rows.Next() {
		var tableName, geomColumn string
		if err := rows.Scan(&tableName, &geomColumn); err != nil {
			log.Warnf("Error scanning layer row: %v", err)
			continue
		}

		// Skip if we've already seen this table (first geometry column wins)
		if seenTables[tableName] {
			log.Warnf("Table %s has multiple geometry columns, using first one: %s", tableName, geomColumn)
			continue
		}
		seenTables[tableName] = true

		// Apply include/exclude filters
		if !cat.isTableIncluded(tableName) {
			continue
		}

		layer := &Layer{
			Name:           tableName,
			Table:          tableName,
			GeometryColumn: geomColumn,
		}

		// Get full metadata including bounds
		if err := cat.enrichLayerMetadata(layer); err != nil {
			log.Warnf("Error enriching layer %s metadata: %v", tableName, err)
			// Continue anyway with basic info
		}

		layers = append(layers, layer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating layers: %w", err)
	}

	log.Infof("Found %d layers with geometry columns", len(layers))
	return layers, nil
}

// enrichLayerMetadataLightweight adds only geometry type and properties (skips expensive bounds calculation)
func (cat *CatalogDB) enrichLayerMetadataLightweight(layer *Layer) error {
	// Get geometry type
	query := fmt.Sprintf(`
		SELECT ST_GeometryType(%s) as geom_type
		FROM %s
		WHERE %s IS NOT NULL
		LIMIT 1
	`, layer.GeometryColumn, layer.Table, layer.GeometryColumn)

	var geomType sql.NullString
	err := cat.dbconn.QueryRow(query).Scan(&geomType)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error getting geometry metadata: %w", err)
	}

	if geomType.Valid {
		layer.GeometryType = geomType.String
	}

	// Get property columns (non-geometry columns)
	propsQuery := fmt.Sprintf(`
		SELECT column_name
		FROM duckdb_columns
		WHERE table_name = '%s' AND data_type != 'GEOMETRY'
		ORDER BY column_name
	`, layer.Table)

	rows, err := cat.dbconn.Query(propsQuery)
	if err != nil {
		return fmt.Errorf("error getting properties: %w", err)
	}
	defer rows.Close()

	var properties []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			continue
		}
		properties = append(properties, col)
	}
	layer.Properties = properties

	return nil
}

// enrichLayerMetadata adds geometry type, SRID, bounds, and property list to a layer
func (cat *CatalogDB) enrichLayerMetadata(layer *Layer) error {
	// Get geometry type
	// Note: DuckDB Spatial doesn't support ST_SRID() - SRID is not stored per-geometry
	query := fmt.Sprintf(`
		SELECT ST_GeometryType(%s) as geom_type
		FROM %s
		WHERE %s IS NOT NULL
		LIMIT 1
	`, layer.GeometryColumn, layer.Table, layer.GeometryColumn)

	var geomType sql.NullString
	err := cat.dbconn.QueryRow(query).Scan(&geomType)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error getting geometry metadata: %w", err)
	}

	if geomType.Valid {
		layer.GeometryType = geomType.String
	}

	// Get native bounds - calculate extent ONCE to avoid expensive double table scan
	// DuckDB Spatial doesn't store SRID per-geometry, so we detect it from coordinate ranges
	nativeBoundsQuery := fmt.Sprintf(`
		WITH extent_calc AS (
			SELECT ST_Extent(%s) as extent
			FROM %s
			WHERE %s IS NOT NULL
		)
		SELECT
			ST_XMin(extent) as minx,
			ST_YMin(extent) as miny,
			ST_XMax(extent) as maxx,
			ST_YMax(extent) as maxy,
			-- Also get transformed bounds in one query to avoid double table scan
			ST_XMin(ST_Transform(extent, 'EPSG:4326', 'EPSG:3857')) as minx_3857,
			ST_YMin(ST_Transform(extent, 'EPSG:4326', 'EPSG:3857')) as miny_3857,
			ST_XMax(ST_Transform(extent, 'EPSG:4326', 'EPSG:3857')) as maxx_3857,
			ST_YMax(ST_Transform(extent, 'EPSG:4326', 'EPSG:3857')) as maxy_3857
		FROM extent_calc
	`, layer.GeometryColumn, layer.Table, layer.GeometryColumn)

	var nativeMinx, nativeMiny, nativeMaxx, nativeMaxy sql.NullFloat64
	var minx3857, miny3857, maxx3857, maxy3857 sql.NullFloat64
	err = cat.dbconn.QueryRow(nativeBoundsQuery).Scan(
		&nativeMinx, &nativeMiny, &nativeMaxx, &nativeMaxy,
		&minx3857, &miny3857, &maxx3857, &maxy3857)
	if err != nil && err != sql.ErrNoRows {
		log.Warnf("Error getting bounds for layer %s: %v", layer.Name, err)
		return err
	}

	// Detect coordinate system from bounds
	// EPSG:3857 (Web Mercator) has values roughly in range [-20037508, 20037508]
	// EPSG:4326 (WGS84) has values in range [-180, 180] for lon, [-90, 90] for lat
	sourceSrid := SRID_4326 // Default assumption
	if nativeMinx.Valid && nativeMaxx.Valid {
		maxAbsX := math.Max(math.Abs(nativeMinx.Float64), math.Abs(nativeMaxx.Float64))
		if maxAbsX > 360 {
			// Likely already in Web Mercator (EPSG:3857)
			sourceSrid = SRID_3857
		}
	}

	// Use appropriate bounds for EPSG:3857 output
	// If data is already in 3857, use native bounds; otherwise use transformed bounds
	var minx, miny, maxx, maxy sql.NullFloat64
	if sourceSrid == SRID_3857 {
		// Already in Web Mercator, use native bounds
		minx, miny, maxx, maxy = nativeMinx, nativeMiny, nativeMaxx, nativeMaxy
	} else {
		// Use pre-calculated transformed bounds
		minx, miny, maxx, maxy = minx3857, miny3857, maxx3857, maxy3857
	}

	if minx.Valid && miny.Valid && maxx.Valid && maxy.Valid {
		layer.Bounds = &Extent{
			Minx: minx.Float64,
			Miny: miny.Float64,
			Maxx: maxx.Float64,
			Maxy: maxy.Float64,
		}
		// Store source SRID for tile generation, set API SRID to 3857 since bounds are in Web Mercator
		layer.SourceSrid = sourceSrid
		layer.Srid = SRID_3857
	}

	// Get property columns (non-geometry columns)
	propsQuery := fmt.Sprintf(`
		SELECT column_name
		FROM duckdb_columns
		WHERE table_name = '%s' AND data_type != 'GEOMETRY'
		ORDER BY column_name
	`, layer.Table)

	rows, err := cat.dbconn.Query(propsQuery)
	if err != nil {
		return fmt.Errorf("error getting properties: %w", err)
	}
	defer rows.Close()

	var properties []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			continue
		}
		properties = append(properties, col)
	}
	layer.Properties = properties

	return nil
}

// isTableIncluded checks if a table should be included based on include/exclude lists
func (cat *CatalogDB) isTableIncluded(tableName string) bool {
	// If includes list is specified and table not in it, exclude
	if len(cat.tableIncludes) > 0 {
		if _, ok := cat.tableIncludes[tableName]; !ok {
			return false
		}
	}

	// If table is in excludes list, exclude
	if len(cat.tableExcludes) > 0 {
		if _, ok := cat.tableExcludes[tableName]; ok {
			return false
		}
	}

	return true
}

// GetLayerByName returns a single layer by name with lightweight metadata for tile generation
// This does NOT calculate bounds to avoid expensive ST_Extent queries on every tile request
func (cat *CatalogDB) GetLayerByName(name string) (*Layer, error) {
	// Query for just this specific table's geometry column
	query := `
		SELECT column_name as geometry_column
		FROM duckdb_columns
		WHERE table_name = $1 AND data_type = 'GEOMETRY'
		LIMIT 1
	`

	var geomColumn string
	err := cat.dbconn.QueryRow(query, name).Scan(&geomColumn)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("layer not found: %s", name)
		}
		return nil, fmt.Errorf("error querying layer %s: %w", name, err)
	}

	// Check if table is included
	if !cat.isTableIncluded(name) {
		return nil, fmt.Errorf("layer not included: %s", name)
	}

	layer := &Layer{
		Name:           name,
		Table:          name,
		GeometryColumn: geomColumn,
	}

	// Detect source SRID without calculating full bounds (lightweight check)
	// Sample one geometry to check coordinate range
	sridQuery := fmt.Sprintf(`
		SELECT ST_X(ST_Centroid(%s)) as x
		FROM %s
		WHERE %s IS NOT NULL
		LIMIT 1
	`, geomColumn, name, geomColumn)

	var sampleX sql.NullFloat64
	err = cat.dbconn.QueryRow(sridQuery).Scan(&sampleX)
	if err == nil && sampleX.Valid {
		// Detect based on coordinate range
		if math.Abs(sampleX.Float64) > 360 {
			layer.SourceSrid = SRID_3857
		} else {
			layer.SourceSrid = SRID_4326
		}
	} else {
		// Default to 4326 if we can't detect
		layer.SourceSrid = SRID_4326
	}

	// Get property columns (non-geometry columns) for MVT generation
	// This is lightweight and necessary to include properties in tiles
	// We also need data types to handle casting of unsupported types
	propsQuery := fmt.Sprintf(`
		SELECT column_name, data_type
		FROM duckdb_columns
		WHERE table_name = '%s' AND data_type != 'GEOMETRY'
		ORDER BY column_name
	`, name)

	rows, err := cat.dbconn.Query(propsQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting properties: %w", err)
	}
	defer rows.Close()

	var properties []string
	propertyTypes := make(map[string]string)
	for rows.Next() {
		var col, dataType string
		if err := rows.Scan(&col, &dataType); err != nil {
			continue
		}
		properties = append(properties, col)
		propertyTypes[col] = dataType
	}
	layer.Properties = properties
	layer.PropertyTypes = propertyTypes

	return layer, nil
}

// GenerateTile generates an MVT tile for the given layer and tile coordinates
// Uses a dedicated database connection per request to avoid connection pool contention
func (cat *CatalogDB) GenerateTile(ctx context.Context, layerName string, z, x, y int) ([]byte, error) {
	layer, err := cat.GetLayerByName(layerName)
	if err != nil {
		return nil, err
	}

	// Create a dedicated database connection for this tile request
	db, err := cat.createRequestConnection()
	if err != nil {
		return nil, fmt.Errorf("error creating request connection: %w", err)
	}
	defer db.Close()

	// Build the SQL query using ST_AsMVT following the Python reference implementation
	// https://github.com/bmcandr/fast-geoparquet-features/blob/main/app/main.py#L352-L418

	// Transform geometry to Web Mercator (EPSG:3857) for tiles if needed
	// DuckDB Spatial requires string CRS identifiers: ST_Transform(geom, 'source_crs', 'dest_crs', always_xy := true)
	geomExpr := layer.GeometryColumn
	if layer.SourceSrid != SRID_3857 && layer.SourceSrid != 0 {
		geomExpr = fmt.Sprintf("ST_Transform(%s, 'EPSG:4326', 'EPSG:3857', always_xy := true)", layer.GeometryColumn)
	}

	// Build column list for properties (all non-geometry columns)
	// We must not include the original geometry column since ST_AsMVT only allows one geometry column
	// Cast unsupported types to supported ones for MVT encoding
	// ST_AsMVT supports: VARCHAR, FLOAT, DOUBLE, INTEGER, BIGINT, BOOLEAN
	propertyColumns := ""
	if len(layer.Properties) > 0 {
		for i, prop := range layer.Properties {
			if i > 0 {
				propertyColumns += ", "
			}

			// Check if type needs casting
			dataType := layer.PropertyTypes[prop]
			needsCast := false

			// DECIMAL types need to be cast to DOUBLE
			if len(dataType) >= 7 && dataType[:7] == "DECIMAL" {
				needsCast = true
			}
			// Add other unsupported types here if needed
			// Examples: TIMESTAMP, DATE, TIME, UUID, etc.
			switch dataType {
			case "TIMESTAMP", "DATE", "TIME", "UUID", "BLOB":
				needsCast = true
			}

			if needsCast {
				// Cast to DOUBLE for numeric types, VARCHAR for others
				if dataType[:7] == "DECIMAL" {
					propertyColumns += fmt.Sprintf("CAST(%s AS DOUBLE) as %s", prop, prop)
				} else {
					propertyColumns += fmt.Sprintf("CAST(%s AS VARCHAR) as %s", prop, prop)
				}
			} else {
				propertyColumns += prop
			}
		}
		propertyColumns += ", "
	}

	// The MVT generation follows this pattern:
	// 1. Filter features that intersect the tile envelope
	// 2. Transform geometries to EPSG:3857 if needed
	// 3. Clip geometries to tile extent using ST_AsMVTGeom
	// 4. Aggregate into MVT format using ST_AsMVT
	query := fmt.Sprintf(`
		WITH tile_bounds AS (
			SELECT ST_TileEnvelope($1::INTEGER, $2::INTEGER, $3::INTEGER) as envelope,
			       ST_Extent(ST_TileEnvelope($1::INTEGER, $2::INTEGER, $3::INTEGER)) as extent
		),
		features AS (
			SELECT
				%sST_AsMVTGeom(
					%s,
					(SELECT extent FROM tile_bounds)
				) as geom
			FROM %s, tile_bounds
			WHERE ST_Intersects(%s, tile_bounds.envelope)
		)
		SELECT ST_AsMVT(features, '%s')
		FROM features
		WHERE geom IS NOT NULL
	`, propertyColumns, geomExpr, layer.Table, geomExpr, layerName)

	log.Debugf("Generating tile for layer=%s z=%d x=%d y=%d", layerName, z, x, y)

	var tileData []byte
	err = db.QueryRowContext(ctx, query, z, x, y).Scan(&tileData)
	if err != nil {
		return nil, fmt.Errorf("error generating tile: %w", err)
	}

	// Return empty tile if no data
	if tileData == nil {
		return []byte{}, nil
	}

	log.Debugf("Generated tile with %d bytes", len(tileData))
	return tileData, nil
}

// GetTileJSON returns TileJSON metadata for a layer
func (cat *CatalogDB) GetTileJSON(layerName string, baseURL string) (*TileJSON, error) {
	layer, err := cat.GetLayerByName(layerName)
	if err != nil {
		return nil, err
	}

	tileURL := fmt.Sprintf("%s/tiles/%s/{z}/{x}/{y}.mvt", baseURL, layerName)

	tj := &TileJSON{
		TileJSON: "3.0.0",
		Name:     layer.Name,
		Version:  "1.0.0",
		Scheme:   "xyz",
		Tiles:    []string{tileURL},
		MinZoom:  0,
		MaxZoom:  22,
	}

	// Add bounds if available
	if layer.Bounds != nil {
		tj.Bounds = []float64{
			layer.Bounds.Minx,
			layer.Bounds.Miny,
			layer.Bounds.Maxx,
			layer.Bounds.Maxy,
		}

		// Calculate center point
		centerX := (layer.Bounds.Minx + layer.Bounds.Maxx) / 2
		centerY := (layer.Bounds.Miny + layer.Bounds.Maxy) / 2
		tj.Center = []float64{centerX, centerY, 10} // default zoom 10
	}

	// Add vector layer metadata
	fields := make(map[string]string)
	for _, prop := range layer.Properties {
		fields[prop] = "string" // simplified - could determine actual type
	}

	tj.VectorLayers = []VectorLayer{
		{
			ID:      layerName,
			MinZoom: 0,
			MaxZoom: 22,
			Fields:  fields,
		},
	}

	return tj, nil
}
