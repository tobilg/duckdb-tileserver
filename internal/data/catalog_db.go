package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	log "github.com/sirupsen/logrus"
	"github.com/tobilg/duckdb-tileserver/internal/conf"
)

// Constants
const (
	JSONTypeString       = "string"
	JSONTypeNumber       = "number"
	JSONTypeBoolean      = "boolean"
	JSONTypeJSON         = "json"
	JSONTypeBooleanArray = "boolean[]"
	JSONTypeStringArray  = "string[]"
	JSONTypeNumberArray  = "number[]"

	DuckDBTypeBool     = "BOOLEAN"
	DuckDBTypeNumeric  = "DOUBLE"
	DuckDBTypeJSON     = "JSON"
	DuckDBTypeGeometry = "GEOMETRY"
	DuckDBTypeText     = "VARCHAR"
)

// CatalogDB is the DuckDB catalog implementation
type CatalogDB struct {
	dbconn        *sql.DB
	dbPath        string // Store database path for per-request connections
	tableIncludes map[string]string
	tableExcludes map[string]string
	tables        []*Table
	tableMap      map[string]*Table
	functions     []*Function
	functionMap   map[string]*Function

	// Layer metadata cache (infinite cache - no expiration)
	layerMetadataCache map[string]*Layer
	layerCacheMutex    sync.RWMutex
}

var isStartup bool
var isFunctionsLoaded bool
var instanceDB *CatalogDB

const fmtQueryStats = "Database query result: %v rows in %v"

func init() {
	isStartup = true
}

// CatDBInstance tbd
func CatDBInstance() Catalog {
	// TODO: make a singleton
	if instanceDB == nil {
		instanceDB = newCatalogDB()
	}
	return instanceDB
}

func newCatalogDB() *CatalogDB {
	dbPath := conf.Configuration.Database.DatabasePath
	conn := dbConnect()
	cat := &CatalogDB{
		dbconn:             conn,
		dbPath:             dbPath,
		layerMetadataCache: make(map[string]*Layer),
	}
	log.Info("Layer metadata cache initialized")
	return cat
}

func dbConnect() *sql.DB {
	dbPath := conf.Configuration.Database.DatabasePath

	// disallow blank config for safety
	if dbPath == "" {
		log.Fatal("Blank DuckDB path is disallowed for security reasons")
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(conf.Configuration.Database.MaxOpenConns)
	db.SetMaxIdleConns(conf.Configuration.Database.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(conf.Configuration.Database.ConnMaxLifetime) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(conf.Configuration.Database.ConnMaxIdleTime) * time.Second)

	log.Infof("Connection pool configured: MaxOpenConns=%d, MaxIdleConns=%d, ConnMaxLifetime=%ds, ConnMaxIdleTime=%ds",
		conf.Configuration.Database.MaxOpenConns,
		conf.Configuration.Database.MaxIdleConns,
		conf.Configuration.Database.ConnMaxLifetime,
		conf.Configuration.Database.ConnMaxIdleTime)

	// Test the connection
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// Load spatial extension
	_, err = db.Exec("INSTALL spatial; LOAD spatial;")
	if err != nil {
		log.Warnf("Failed to load spatial extension: %v", err)
	}

	log.Infof("Connected to DuckDB: %s", dbPath)
	return db
}

// GetDB returns the underlying database connection
func (cat *CatalogDB) GetDB() *sql.DB {
	return cat.dbconn
}

func (cat *CatalogDB) SetIncludeExclude(includeList []string, excludeList []string) {
	//-- include schemas / tables
	cat.tableIncludes = make(map[string]string)
	for _, name := range includeList {
		nameLow := strings.ToLower(name)
		cat.tableIncludes[nameLow] = nameLow
	}
	//-- excluded schemas / tables
	cat.tableExcludes = make(map[string]string)
	for _, name := range excludeList {
		nameLow := strings.ToLower(name)
		cat.tableExcludes[nameLow] = nameLow
	}
}

func (cat *CatalogDB) Close() {
	cat.dbconn.Close()
}

// InvalidateLayerMetadataCache clears the layer metadata cache
// If layerName is empty, clears the entire cache; otherwise clears specific layer
func (cat *CatalogDB) InvalidateLayerMetadataCache(layerName string) {
	cat.layerCacheMutex.Lock()
	defer cat.layerCacheMutex.Unlock()

	if layerName == "" {
		// Clear entire cache
		cat.layerMetadataCache = make(map[string]*Layer)
		log.Info("Layer metadata cache cleared (all layers)")
	} else {
		// Clear specific layer
		delete(cat.layerMetadataCache, layerName)
		log.Infof("Layer metadata cache cleared for: %s", layerName)
	}
}

// GetLayerMetadataCacheStats returns statistics about the layer metadata cache
func (cat *CatalogDB) GetLayerMetadataCacheStats() map[string]interface{} {
	cat.layerCacheMutex.RLock()
	defer cat.layerCacheMutex.RUnlock()

	return map[string]interface{}{
		"cached_layers": len(cat.layerMetadataCache),
		"layers":        getLayerNames(cat.layerMetadataCache),
	}
}

// Helper function to get layer names from cache
func getLayerNames(cache map[string]*Layer) []string {
	names := make([]string, 0, len(cache))
	for name := range cache {
		names = append(names, name)
	}
	return names
}

func (cat *CatalogDB) Tables() ([]*Table, error) {
	cat.refreshTables(true)
	return cat.tables, nil
}

func (cat *CatalogDB) TableReload(name string) {
	tbl, ok := cat.tableMap[name]
	if !ok {
		return
	}
	// load extent (which may change over time
	sqlExtentEst := sqlExtentEstimated(tbl)
	isExtentLoaded := cat.loadExtent(sqlExtentEst, tbl)
	if !isExtentLoaded {
		log.Debugf("Can't get estimated extent for %s", name)
		sqlExtentExact := sqlExtentExact(tbl)
		cat.loadExtent(sqlExtentExact, tbl)
	}
}

func (cat *CatalogDB) loadExtent(sql string, tbl *Table) bool {
	var (
		xmin *float64
		xmax *float64
		ymin *float64
		ymax *float64
	)
	log.Debug("Extent query: " + sql)
	err := cat.dbconn.QueryRow(sql).Scan(&xmin, &ymin, &xmax, &ymax)
	if err != nil {
		log.Debugf("Error querying Extent for %s: %v", tbl.ID, err)
		return false
	}
	// no extent was read (perhaps a view...)
	if xmin == nil {
		return false
	}
	tbl.Extent.Minx = *xmin
	tbl.Extent.Miny = *ymin
	tbl.Extent.Maxx = *xmax
	tbl.Extent.Maxy = *ymax
	return true
}

func (cat *CatalogDB) TableByName(name string) (*Table, error) {
	cat.refreshTables(false)
	tbl, ok := cat.tableMap[name]
	if !ok {
		return nil, nil
	}
	return tbl, nil
}

func (cat *CatalogDB) TableFeatures(ctx context.Context, name string, param *QueryParam) ([]string, error) {
	tbl, err := cat.TableByName(name)
	if err != nil || tbl == nil {
		return nil, err
	}
	cols := param.Columns
	sql, argValues := sqlFeatures(tbl, param)
	log.Debug("Features query: " + sql)
	idColIndex := indexOfName(cols, tbl.IDColumn)

	features, err := readFeaturesWithArgs(ctx, cat.dbconn, sql, argValues, idColIndex, cols)
	return features, err
}

func (cat *CatalogDB) TableFeature(ctx context.Context, name string, id string, param *QueryParam) (string, error) {
	tbl, err := cat.TableByName(name)
	if err != nil {
		return "", err
	}
	cols := param.Columns
	sql := sqlFeature(tbl, param)
	log.Debug("Feature query: " + sql)
	idColIndex := indexOfName(cols, tbl.IDColumn)

	//--- Add a SQL arg for the feature ID
	argValues := make([]interface{}, 0)
	argValues = append(argValues, id)
	features, err := readFeaturesWithArgs(ctx, cat.dbconn, sql, argValues, idColIndex, cols)

	if len(features) == 0 {
		return "", err
	}
	return features[0], nil
}

func (cat *CatalogDB) refreshTables(force bool) {
	// TODO: refresh on timed basis?
	if force || isStartup {
		cat.loadTables()
		isStartup = false
	}
}

func (cat *CatalogDB) loadTables() {
	cat.tableMap = cat.readTables(cat.dbconn)
	cat.tables = tablesSorted(cat.tableMap)
}

func tablesSorted(tableMap map[string]*Table) []*Table {
	// TODO: use database order instead of sorting here
	var lsort []*Table
	for key := range tableMap {
		lsort = append(lsort, tableMap[key])
	}
	sort.SliceStable(lsort, func(i, j int) bool {
		return lsort[i].Title < lsort[j].Title
	})
	return lsort
}

func (cat *CatalogDB) readTables(db *sql.DB) map[string]*Table {
	// Discover all tables with geometry columns
	log.Info("Discovering all tables with geometry columns")
	rows, err := db.Query(sqlTables)

	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	tables := make(map[string]*Table)
	for rows.Next() {
		tbl := scanTable(cat.dbconn, rows)
		if cat.isIncluded(tbl) {
			tables[tbl.ID] = tbl
			log.Infof("Added table collection: %s (geometry column: %s)", tbl.ID, tbl.GeometryColumn)
		}
	}
	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	if len(tables) == 0 {
		log.Warn("No tables with geometry columns found in database")
	}

	return tables
}

func (cat *CatalogDB) isIncluded(tbl *Table) bool {
	//--- if no includes defined, always include
	isIncluded := true
	if len(cat.tableIncludes) > 0 {
		isIncluded = isMatchSchemaTable(tbl, cat.tableIncludes)
	}
	isExcluded := false
	if len(cat.tableExcludes) > 0 {
		isExcluded = isMatchSchemaTable(tbl, cat.tableExcludes)
	}
	return isIncluded && !isExcluded
}

func isMatchSchemaTable(tbl *Table, list map[string]string) bool {
	schemaLow := strings.ToLower(tbl.Schema)
	if _, ok := list[schemaLow]; ok {
		return true
	}
	idLow := strings.ToLower(tbl.ID)
	if _, ok := list[idLow]; ok {
		return true
	}
	return false
}

func scanTable(db *sql.DB, rows *sql.Rows) *Table {
	var (
		id, schema, table, description, geometryCol string
		srid                                        int
		geometryType, idColumn                      string
		propsStr                                    string
	)

	err := rows.Scan(&id, &schema, &table, &description, &geometryCol,
		&srid, &geometryType, &idColumn, &propsStr)
	if err != nil {
		log.Fatal(err)
	}

	// For DuckDB, we'll get column information through a separate query
	columns, datatypes, jsontypes, colDesc := getTableColumns(db, table)

	// Synthesize a title for now
	title := id
	// synthesize a description if none provided
	if description == "" {
		description = fmt.Sprintf("Data for table %v", id)
	}

	return &Table{
		ID:             id,
		Schema:         schema,
		Table:          table,
		Title:          title,
		Description:    description,
		GeometryColumn: geometryCol,
		Srid:           srid,
		GeometryType:   geometryType,
		IDColumn:       idColumn,
		Columns:        columns,
		DbTypes:        datatypes,
		JSONTypes:      jsontypes,
		ColDesc:        colDesc,
	}
}

func getTableColumns(db *sql.DB, tableName string) ([]string, map[string]string, []string, []string) {
	query := `SELECT column_name, data_type 
	          FROM information_schema.columns 
	          WHERE table_name = ? 
	          AND column_name != 'geom'
	          ORDER BY ordinal_position`

	rows, err := db.Query(query, tableName)
	if err != nil {
		log.Warnf("Error getting columns for table %s: %v", tableName, err)
		// Return minimal fallback
		return []string{"id"}, map[string]string{"id": "INTEGER"}, []string{"number"}, []string{"Identifier column"}
	}
	defer rows.Close()

	var columns []string
	datatypes := make(map[string]string)
	var jsontypes []string
	var colDesc []string

	for rows.Next() {
		var columnName, dataType string
		err := rows.Scan(&columnName, &dataType)
		if err != nil {
			log.Warnf("Error scanning column info: %v", err)
			continue
		}

		columns = append(columns, columnName)
		datatypes[columnName] = dataType
		jsontypes = append(jsontypes, toJSONTypeFromDuckDB(dataType))
		colDesc = append(colDesc, fmt.Sprintf("Column %s of type %s", columnName, dataType))
	}

	// Ensure we have at least one column
	if len(columns) == 0 {
		columns = []string{"id"}
		datatypes["id"] = "INTEGER"
		jsontypes = []string{"number"}
		colDesc = []string{"Identifier column"}
	}

	log.Debugf("Table %s columns: %v", tableName, columns)
	return columns, datatypes, jsontypes, colDesc
}

//=================================================

//nolint:unused
func readFeatures(ctx context.Context, db *sql.DB, sql string, idColIndex int, propCols []string) ([]string, error) {
	return readFeaturesWithArgs(ctx, db, sql, nil, idColIndex, propCols)
}

//nolint:unused
func readFeaturesWithArgs(ctx context.Context, db *sql.DB, sql string, args []interface{}, idColIndex int, propCols []string) ([]string, error) {
	start := time.Now()
	rows, err := db.QueryContext(ctx, sql, args...)
	if err != nil {
		log.Warnf("Error running Features query: %v", err)
		return nil, err
	}
	defer rows.Close()

	data, err := scanFeatures(ctx, rows, idColIndex, propCols)
	if err != nil {
		return data, err
	}
	log.Debugf(fmtQueryStats, len(data), time.Since(start))
	return data, nil
}

func scanFeatures(ctx context.Context, rows *sql.Rows, idColIndex int, propCols []string) ([]string, error) {
	// init features array to empty (not nil)
	var features []string = []string{}
	for rows.Next() {
		feature := scanFeature(rows, idColIndex, propCols)
		//log.Println(feature)
		features = append(features, feature)
	}
	// context check done outside rows loop,
	// because a long-running function might not produce any rows before timeout
	if err := ctx.Err(); err != nil {
		//log.Debugf("Context error scanning Features: %v", err)
		return features, err
	}
	// Check for errors from scanning rows.
	if err := rows.Err(); err != nil {
		log.Warnf("Error scanning rows for Features: %v", err)
		// TODO: return nil here ?
		return features, err
	}
	return features, nil
}

func scanFeature(rows *sql.Rows, idColIndex int, propNames []string) string {
	var id, geom string

	// Get column names to dynamically scan
	columns, err := rows.Columns()
	if err != nil {
		log.Warnf("Error getting columns: %v", err)
		return ""
	}

	// Create a slice to hold values
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	err = rows.Scan(valuePtrs...)
	if err != nil {
		log.Warnf("Error scanning row for Feature: %v", err)
		return ""
	}

	//--- geom value is expected to be a GeoJSON string
	//--- convert NULL to an empty string
	if values[0] != nil {
		if geomStr, ok := values[0].(string); ok {
			geom = geomStr
		} else if geomBytes, ok := values[0].([]byte); ok {
			// Handle binary geometry data by converting to string
			geom = string(geomBytes)
		} else {
			// Handle case where DuckDB returns geometry as a map structure
			// Convert it to JSON string
			if geomJSON, err := json.Marshal(values[0]); err == nil {
				geom = string(geomJSON)
				log.Debugf("Converted geometry map to JSON: %s", geom)
			} else {
				log.Warnf("Failed to convert geometry to JSON: %v, error: %v", values[0], err)
				geom = ""
			}
		}
		// Additional debugging info
		log.Debugf("Raw geometry data (first 100 chars): %s", truncateString(geom, 100))
	} else {
		geom = ""
	}

	propOffset := 1
	if idColIndex >= 0 {
		id = fmt.Sprintf("%v", values[idColIndex+propOffset])
	}

	//fmt.Println(geom)
	props := extractProperties(values, propOffset, propNames)
	return makeFeatureJSON(id, geom, props)
}

func extractProperties(vals []interface{}, propOffset int, propNames []string) map[string]interface{} {
	props := make(map[string]interface{})
	for i, name := range propNames {
		// offset vals index by 2 to skip geom, id
		props[name] = toJSONValue(vals[i+propOffset])
		//fmt.Printf("%v: %v\n", name, val)
	}
	return props
}

// toJSONValue converts DuckDB types to JSON values
func toJSONValue(value interface{}) interface{} {
	// Handle NULL values
	if value == nil {
		return nil
	}

	// For DuckDB, most values can be used directly as JSON
	// since they're already in Go native types
	switch v := value.(type) {
	case []byte:
		// Convert byte arrays to strings
		return string(v)
	case sql.NullString:
		if v.Valid {
			return v.String
		}
		return nil
	case sql.NullInt64:
		if v.Valid {
			return v.Int64
		}
		return nil
	case sql.NullFloat64:
		if v.Valid {
			return v.Float64
		}
		return nil
	case sql.NullBool:
		if v.Valid {
			return v.Bool
		}
		return nil
	default:
		// For most Go native types, return as-is
		return value
	}
}

func toJSONTypeFromDuckDBArray(duckdbTypes []string) []string {
	jsonTypes := make([]string, len(duckdbTypes))
	for i, duckdbType := range duckdbTypes {
		jsonTypes[i] = toJSONTypeFromDuckDB(duckdbType)
	}
	return jsonTypes
}

func toJSONTypeFromDuckDB(duckdbType string) string {
	//fmt.Printf("toJSONTypeFromDuckDB: %v\n", duckdbType)
	switch strings.ToUpper(duckdbType) {
	case "INTEGER", "BIGINT", "SMALLINT", "TINYINT":
		return JSONTypeNumber
	case "DOUBLE", "REAL", "DECIMAL", "NUMERIC":
		return JSONTypeNumber
	case "BOOLEAN":
		return JSONTypeBoolean
	case "JSON":
		return JSONTypeJSON
	case "VARCHAR", "TEXT", "CHAR":
		return JSONTypeString
	case "GEOMETRY":
		return JSONTypeString // GeoJSON is represented as string
	default:
		// For arrays and other complex types, default to string
		if strings.Contains(duckdbType, "[]") {
			if strings.Contains(duckdbType, "INTEGER") || strings.Contains(duckdbType, "DOUBLE") {
				return JSONTypeNumberArray
			}
			if strings.Contains(duckdbType, "BOOLEAN") {
				return JSONTypeBooleanArray
			}
			return JSONTypeStringArray
		}
		return JSONTypeString
	}
}

type featureData struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id,omitempty"`
	Geom  *json.RawMessage       `json:"geometry"`
	Props map[string]interface{} `json:"properties"`
}

func makeFeatureJSON(id string, geom string, props map[string]interface{}) string {
	//--- convert empty geom string to JSON null
	var geomRaw json.RawMessage
	if geom != "" {
		// Validate that geom is valid JSON before using it
		if json.Valid([]byte(geom)) {
			geomRaw = json.RawMessage(geom)
		} else {
			log.Warnf("Invalid geometry JSON, using null: %s", geom)
			geomRaw = json.RawMessage("null")
		}
	} else {
		geomRaw = json.RawMessage("null")
	}

	featData := featureData{
		Type:  "Feature",
		ID:    id,
		Geom:  &geomRaw,
		Props: props,
	}
	jsonBytes, err := json.Marshal(featData)
	if err != nil {
		log.Errorf("Error marshalling feature into JSON: %v", err)
		return ""
	}
	jsonStr := string(jsonBytes)
	//fmt.Println(jsonStr)
	return jsonStr
}

// indexOfName finds the index of a name in an array of names
// It returns the index or -1 if not found
func indexOfName(names []string, name string) int {
	for i, nm := range names {
		if nm == name {
			return i
		}
	}
	return -1
}

// truncateString truncates a string to maxLen characters for debugging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
