package data

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

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

const forceTextTSVECTOR = "tsvector"

const sqlTables = `
SELECT 
    table_name AS id,
    'main' AS schema,
    table_name AS table,
    '' AS description,
    column_name AS geometry_column,
    4326 AS srid,
    'GEOMETRY' AS geometry_type,
    '' AS id_column,
    '[]' AS props
FROM information_schema.columns 
WHERE data_type = 'GEOMETRY'
ORDER BY table_name
`

const sqlFunctionsTemplate = `
SELECT 
    'spatial.ST_Area' AS id,
    'spatial' AS schema,
    'ST_Area' AS function,
    'Calculate area of geometry' AS description,
    '["geom"]' AS input_names,
    '["GEOMETRY"]' AS input_types,
    '[]' AS argdefaults,
    '["area"]' AS output_names,
    '["DOUBLE"]' AS output_types
UNION ALL
SELECT 
    'spatial.ST_AsGeoJSON' AS id,
    'spatial' AS schema,
    'ST_AsGeoJSON' AS function,
    'Convert geometry to GeoJSON' AS description,
    '["geom"]' AS input_names,
    '["GEOMETRY"]' AS input_types,
    '[]' AS argdefaults,
    '["geojson"]' AS output_names,
    '["VARCHAR"]' AS output_types
`

func sqlFunctions() string {
	return sqlFunctionsTemplate
}

func quotedList(names []string) string {
	itemsJoin := strings.Join(names, "','")
	return "'" + itemsJoin + "'"
}

const sqlFmtExtentEst = `SELECT ST_XMin(ext.geom) AS xmin, ST_YMin(ext.geom) AS ymin, ST_XMax(ext.geom) AS xmax, ST_YMax(ext.geom) AS ymax
FROM ( SELECT ST_Envelope_Agg("%s") AS geom FROM "%s" ) AS ext;`

func sqlExtentEstimated(tbl *Table) string {
	return fmt.Sprintf(sqlFmtExtentEst, tbl.GeometryColumn, tbl.Table)
}

const sqlFmtExtentExact = `SELECT ST_XMin(ext.geom) AS xmin, ST_YMin(ext.geom) AS ymin, ST_XMax(ext.geom) AS xmax, ST_YMax(ext.geom) AS ymax
FROM (SELECT COALESCE(ST_Envelope_Agg("%s"), ST_GeomFromText('POLYGON((-180 -90, 180 -90, 180 90, -180 90, -180 -90))', 4326)) AS geom FROM "%s" ) AS ext;`

func sqlExtentExact(tbl *Table) string {
	return fmt.Sprintf(sqlFmtExtentExact, tbl.GeometryColumn, tbl.Table)
}

const sqlFmtFeatures = "SELECT %v %v FROM \"%s\" %v %v %v %s;"

func sqlFeatures(tbl *Table, param *QueryParam) (string, []interface{}) {
	geomCol := sqlGeomCol(tbl.GeometryColumn, tbl.Srid, param)
	propCols := sqlColList(param.Columns, tbl.DbTypes, true)
	bboxFilter := sqlBBoxFilter(tbl.GeometryColumn, param.Bbox, param.BboxCrs)
	attrFilter, attrVals := sqlAttrFilter(param.Filter)
	cqlFilter := sqlCqlFilter(param.FilterSql)
	sqlWhere := sqlWhere(bboxFilter, attrFilter, cqlFilter)
	sqlGroupBy := sqlGroupBy(param.GroupBy)
	sqlOrderBy := sqlOrderBy(param.SortBy)
	sqlLimitOffset := sqlLimitOffset(param.Limit, param.Offset)
	sql := fmt.Sprintf(sqlFmtFeatures, geomCol, propCols, tbl.Table, sqlWhere, sqlGroupBy, sqlOrderBy, sqlLimitOffset)
	return sql, attrVals
}

// sqlColList creates a comma-separated column list, or blank if no columns
// If addLeadingComma is true, a leading comma is added, for use when the target SQL has columns defined before
func sqlColList(names []string, dbtypes map[string]string, addLeadingComma bool) string {
	if len(names) == 0 {
		return ""
	}

	var cols []string
	for _, col := range names {
		colExpr := sqlColExpr(col, dbtypes[col])
		cols = append(cols, colExpr)
	}
	colsStr := strings.Join(cols, ",")
	if addLeadingComma {
		return ", " + colsStr
	}
	return colsStr
}

// makeSQLColExpr casts a column to text if type is unknown to PGX
func sqlColExpr(name string, dbtype string) string {

	name = strconv.Quote(name)

	// TODO: make this more data-driven / configurable
	switch dbtype {
	case forceTextTSVECTOR:
		return fmt.Sprintf("%s::text", name)
	}

	// for properties that will be treated as a string in the JSON response,
	// cast to text.  This allows displaying data types that DuckDB
	// does not support out of the box, as long as it can be cast to text.
	if toJSONTypeFromDuckDB(dbtype) == JSONTypeString {
		return fmt.Sprintf("%s::VARCHAR", name)
	}

	return name
}

const sqlFmtFeature = "SELECT %v %v FROM \"%s\" WHERE \"%v\" = $1 LIMIT 1"

func sqlFeature(tbl *Table, param *QueryParam) string {
	geomCol := sqlGeomCol(tbl.GeometryColumn, tbl.Srid, param)
	propCols := sqlColList(param.Columns, tbl.DbTypes, true)
	sql := fmt.Sprintf(sqlFmtFeature, geomCol, propCols, tbl.Table, tbl.IDColumn)
	return sql
}

func sqlCqlFilter(sql string) string {
	//log.Debug("SQL = " + sql)
	if len(sql) == 0 {
		return ""
	}
	return "(" + sql + ")"
}

func sqlWhere(cond1 string, cond2 string, cond3 string) string {
	var condList []string
	if len(cond1) > 0 {
		condList = append(condList, cond1)
	}
	if len(cond2) > 0 {
		condList = append(condList, cond2)
	}
	if len(cond3) > 0 {
		condList = append(condList, cond3)
	}
	where := strings.Join(condList, " AND ")
	if len(where) > 0 {
		where = " WHERE " + where
	}
	return where
}

func sqlAttrFilter(filterConds []*PropertyFilter) (string, []interface{}) {
	var vals []interface{}
	var exprItems []string
	for i, cond := range filterConds {
		sqlCond := fmt.Sprintf("\"%v\" = $%v", cond.Name, i+1)
		exprItems = append(exprItems, sqlCond)
		vals = append(vals, cond.Value)
	}
	sql := strings.Join(exprItems, " AND ")
	return sql, vals
}

// DuckDB spatial doesn't support SRID parameter in ST_GeomFromText
const sqlFmtBBoxGeoFilter = ` ST_Intersects("%v", ST_GeomFromText('POLYGON((%v %v, %v %v, %v %v, %v %v, %v %v))')) `

func sqlBBoxFilter(geomCol string, bbox *Extent, bboxSRID int) string {
	if bbox == nil {
		return ""
	}
	// For DuckDB, use ST_GeomFromText without SRID parameter
	return fmt.Sprintf(sqlFmtBBoxGeoFilter, geomCol,
		bbox.Minx, bbox.Miny, bbox.Maxx, bbox.Miny, bbox.Maxx, bbox.Maxy, bbox.Minx, bbox.Maxy, bbox.Minx, bbox.Miny)
}

const sqlFmtGeomCol = `ST_AsGeoJSON( %v %v ) AS _geojson`

func sqlGeomCol(geomCol string, sourceSRID int, param *QueryParam) string {
	geomColSafe := strconv.Quote(geomCol)
	geomExpr := applyTransform(param.TransformFuns, geomColSafe)
	geomOutExpr := transformToOutCrs(geomExpr, sourceSRID, param.Crs)
	sql := fmt.Sprintf(sqlFmtGeomCol, geomOutExpr, sqlPrecisionArg(param.Precision))
	return sql
}

func transformToOutCrs(geomExpr string, sourceSRID, outSRID int) string {
	if sourceSRID == outSRID {
		return geomExpr
	}
	// For DuckDB spatial, we'll return the original for now since transform support is limited
	return geomExpr
}

func sqlPrecisionArg(precision int) string {
	if precision < 0 {
		return ""
	}
	sqlPrecision := fmt.Sprintf(",%v", precision)
	return sqlPrecision
}

const sqlFmtOrderBy = `ORDER BY "%v" %v`

func sqlOrderBy(ordering []Sorting) string {
	if len(ordering) <= 0 {
		return ""
	}
	// TODO: support more than one ordering
	col := ordering[0].Name
	dir := ""
	if ordering[0].IsDesc {
		dir = "DESC"
	}
	sql := fmt.Sprintf(sqlFmtOrderBy, col, dir)
	return sql
}

const sqlFmtGroupBy = `GROUP BY "%v"`

func sqlGroupBy(groupBy []string) string {
	if len(groupBy) <= 0 {
		return ""
	}
	// TODO: support more than one grouping
	col := groupBy[0]
	sql := fmt.Sprintf(sqlFmtGroupBy, col)
	log.Debugf("group by: %s", sql)
	return sql
}

func sqlLimitOffset(limit int, offset int) string {
	sqlLim := ""
	if limit >= 0 {
		sqlLim = fmt.Sprintf(" LIMIT %d", limit)
	}
	sqlOff := ""
	if offset > 0 {
		sqlOff = fmt.Sprintf(" OFFSET %d", offset)
	}
	return sqlLim + sqlOff
}

func applyTransform(funs []TransformFunction, expr string) string {
	if funs == nil {
		return expr
	}
	for _, fun := range funs {
		expr = fun.apply(expr)
	}
	return expr
}

const sqlFmtGeomFunction = "SELECT %s %s FROM \"%s\"( %v ) %v %v %s;"

func sqlGeomFunction(fn *Function, args map[string]string, propCols []string, param *QueryParam) (string, []interface{}) {
	sqlArgs, argVals := sqlFunctionArgs(args)
	sqlGeomCol := sqlGeomCol(fn.GeometryColumn, SRID_UNKNOWN, param)
	sqlPropCols := sqlColList(propCols, fn.Types, true)
	//-- SRS of function output is unknown, so have to assume 4326
	bboxFilter := sqlBBoxFilter(fn.GeometryColumn, param.Bbox, param.BboxCrs)
	cqlFilter := sqlCqlFilter(param.FilterSql)
	sqlWhere := sqlWhere(bboxFilter, cqlFilter, "")
	sqlOrderBy := sqlOrderBy(param.SortBy)
	sqlLimitOffset := sqlLimitOffset(param.Limit, param.Offset)
	sql := fmt.Sprintf(sqlFmtGeomFunction, sqlGeomCol, sqlPropCols, fn.Name, sqlArgs, sqlWhere, sqlOrderBy, sqlLimitOffset)
	return sql, argVals
}

const sqlFmtFunction = "SELECT %v FROM \"%s\"( %v ) %v %v %s;"

func sqlFunction(fn *Function, args map[string]string, propCols []string, param *QueryParam) (string, []interface{}) {
	sqlArgs, argVals := sqlFunctionArgs(args)
	sqlPropCols := sqlColList(propCols, fn.Types, false)
	cqlFilter := sqlCqlFilter(param.FilterSql)
	sqlWhere := sqlWhere(cqlFilter, "", "")
	sqlOrderBy := sqlOrderBy(param.SortBy)
	sqlLimitOffset := sqlLimitOffset(param.Limit, param.Offset)
	sql := fmt.Sprintf(sqlFmtFunction, sqlPropCols, fn.Name, sqlArgs, sqlWhere, sqlOrderBy, sqlLimitOffset)
	return sql, argVals
}

func sqlFunctionArgs(argValues map[string]string) (string, []interface{}) {
	var vals []interface{}
	var argItems []string
	i := 1
	for argName := range argValues {
		argItem := fmt.Sprintf("%v => $%v", argName, i)
		argItems = append(argItems, argItem)
		i++
		vals = append(vals, argValues[argName])
	}
	sql := strings.Join(argItems, ",")
	return sql, vals
}
