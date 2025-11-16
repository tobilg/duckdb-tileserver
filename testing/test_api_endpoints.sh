#!/bin/bash

# Comprehensive API endpoint testing for duckdb-tileserver
# This script tests various API endpoints based on the test cases from pgfs_test.md

set -e

echo "🧪 DuckDB FeatureServ API Testing Suite"
echo "======================================="

# Configuration
HOST="localhost:9000"
BASE_URL="http://$HOST"
TEST_DB="test_spatial.duckdb"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper function to run a test
run_test() {
    local test_name="$1"
    local url="$2"
    local expected_pattern="$3"
    
    echo -e "${BLUE}Testing:${NC} $test_name"
    echo -e "${YELLOW}URL:${NC} $url"
    
    response=$(curl -s "$url" 2>/dev/null)
    status=$?
    
    if [ $status -eq 0 ]; then
        if [ -n "$expected_pattern" ]; then
            if echo "$response" | grep -q "$expected_pattern"; then
                echo -e "${GREEN}✓ PASS${NC} - Response contains expected pattern"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                echo -e "${RED}✗ FAIL${NC} - Response doesn't contain expected pattern: $expected_pattern"
                echo "Response: $(echo "$response" | head -c 200)..."
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
        else
            echo -e "${GREEN}✓ PASS${NC} - Request successful"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        fi
    else
        echo -e "${RED}✗ FAIL${NC} - Request failed"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    echo ""
}

# Check if server is running
echo "📡 Checking if server is running..."
if ! curl -s "$BASE_URL/collections" >/dev/null 2>&1; then
    echo "❌ Server not running at $BASE_URL"
    echo "Please start the server with: ./duckdb-tileserver --database-path $TEST_DB"
    exit 1
fi
echo "✅ Server is running"
echo ""

# Test Categories

echo "🏠 === BASIC ENDPOINT TESTS ==="
run_test "Service landing page" "$BASE_URL/" "duckdb-featureserv"
run_test "Collections endpoint" "$BASE_URL/collections" "test_"
run_test "Collections JSON format" "$BASE_URL/collections.json" "collections"
run_test "API documentation" "$BASE_URL/api" "openapi"

echo "📋 === COLLECTION METADATA TESTS ==="
run_test "Individual collection metadata" "$BASE_URL/collections/test_geom" "test_geom"
run_test "Collection with CRS data" "$BASE_URL/collections/test_crs" "test_crs"
run_test "Collection with JSON data" "$BASE_URL/collections/test_json" "test_json"

echo "🗺️ === FEATURE RETRIEVAL TESTS ==="
run_test "All features from test_geom" "$BASE_URL/collections/test_geom/items" "FeatureCollection"
run_test "Limited features (5)" "$BASE_URL/collections/test_crs/items?limit=5" "features"
run_test "Features with offset" "$BASE_URL/collections/test_crs/items?limit=5&offset=10" "features"
run_test "JSON format features" "$BASE_URL/collections/test_json/items.json" "Point"
run_test "HTML format features" "$BASE_URL/collections/test_geom/items.html" "html"
run_test "JSON format with properties" "$BASE_URL/collections/test_json/items.json?properties=id,val_json" "val_json"
run_test "HTML format with properties" "$BASE_URL/collections/test_crs/items.html?properties=id,name&limit=5" "html"

echo "🎯 === PROPERTY SELECTION TESTS ==="
run_test "Select specific properties" "$BASE_URL/collections/test_names/items?properties=id,colCamelCase" "colCamelCase"
run_test "Select geometry and ID only" "$BASE_URL/collections/test_geom/items?properties=id" "geometry"

echo "📦 === BOUNDING BOX TESTS ==="
run_test "Simple bbox filter" "$BASE_URL/collections/test_crs/items?bbox=1000000,400000,1010000,410000" "features"
run_test "Larger bbox filter" "$BASE_URL/collections/test_crs/items?bbox=1000000,400000,1030000,430000" "features"
run_test "Point data bbox" "$BASE_URL/collections/test_json/items?bbox=0.5,0.5,2.5,2.5" "Point"
run_test "Empty bbox result" "$BASE_URL/collections/test_json/items?bbox=100,100,101,101" "FeatureCollection"

echo "🔍 === CQL FILTER TESTS ==="
run_test "ID range filter" "$BASE_URL/collections/test_crs/items?filter=id%20BETWEEN%201%20AND%205" "features"
run_test "String LIKE filter" "$BASE_URL/collections/test_crs/items?filter=name%20LIKE%20%271_%25%27" "features"
run_test "NOT IN filter" "$BASE_URL/collections/test_crs/items?filter=NOT%20id%20IN%20(1,2,3)" "features"
run_test "NOT id In filter (original syntax)" "$BASE_URL/collections/test_crs/items?filter=NOT%20id%20In%20(1,2,3)" "features"
run_test "Equality filter" "$BASE_URL/collections/test_geom/items?filter=data%20=%20%27aaa%27" "Point"
run_test "String BETWEEN filter" "$BASE_URL/collections/test_crs/items?filter=name%20BETWEEN%20%270_0%27%20AND%20%272_2%27" "features"
run_test "String LIKE with wildcards" "$BASE_URL/collections/test_crs/items?filter=name%20LIKE%20%27%25_0%27" "features"

echo "🌍 === SPATIAL FILTER TESTS ==="
run_test "Point intersection" "$BASE_URL/collections/test_geom/items?filter=INTERSECTS(geom,%20POINT(1%201))" "features"
run_test "LineString intersection" "$BASE_URL/collections/test_geom/items?filter=INTERSECTS(geom,%20LINESTRING(1%201,%202%202))" "features"
run_test "Geometry within distance" "$BASE_URL/collections/test_json/items?filter=DWITHIN(geom,%20POINT(1.5%201.5),%201)" "features"
run_test "DWITHIN with Point coordinates" "$BASE_URL/collections/test_crs/items?filter=DWITHIN(geom,POINT(1005000%20405000),50000)" "features"
run_test "Large distance DWITHIN" "$BASE_URL/collections/test_crs/items?filter=DWITHIN(geom,POINT(1000000%20400000),60000)&limit=100" "features"
run_test "CROSSES spatial operator" "$BASE_URL/collections/test_crs/items?filter=CROSSES(geom,%20LINESTRING(1000000%20400000,1100000%20500000))" "FeatureCollection"
run_test "ENVELOPE intersection" "$BASE_URL/collections/test_crs/items?filter=INTERSECTS(geom,%20ENVELOPE(1000000,400000,1100000,500000))&limit=100" "features"
run_test "DWITHIN Point with encoding" "$BASE_URL/collections/test_json/items?filter=DWITHIN(geom,POINT(0%252%201),10)" "FeatureCollection"
run_test "DWITHIN LineString geometry" "$BASE_URL/collections/test_crs/items?filter=DWITHIN(geom,LINESTRING(1000000%20400000,1010000%20410000),10000)&limit=1000" "FeatureCollection"

echo "🔢 === ARRAY DATA TESTS ==="
run_test "Array properties" "$BASE_URL/collections/test_arr/items?properties=id,val_int,val_txt" "val_int"
run_test "All array data" "$BASE_URL/collections/test_arr/items" "val_bool"

echo "📄 === JSON DATA TESTS ==="
run_test "JSON properties" "$BASE_URL/collections/test_json/items?properties=id,val_json" "val_json"
run_test "JSON with geometry" "$BASE_URL/collections/test_json/items" "coordinates"

echo "🔀 === MIXED GEOMETRY TESTS ==="
run_test "Point and LineString" "$BASE_URL/collections/test_geom/items" "Point.*LineString"

echo "📐 === COORDINATE SYSTEM TESTS ==="
run_test "Default CRS features" "$BASE_URL/collections/test_json/items" "coordinates"
run_test "Custom CRS data" "$BASE_URL/collections/test_crs/items?limit=1" "1000000"
run_test "SRID 0 features" "$BASE_URL/collections/test_srid0/items?limit=3" "coordinates"
run_test "CRS parameter test" "$BASE_URL/collections/test_crs/items.json?crs=3005&limit=5" "features"
run_test "CRS with bbox-crs small area" "$BASE_URL/collections/test_crs/items.json?crs=3005&bbox-crs=3005&bbox=1000000,400000,1010000,410000" "FeatureCollection"
run_test "CRS with bbox-crs larger area" "$BASE_URL/collections/test_crs/items.json?crs=3005&bbox-crs=3005&bbox=1000000,400000,1030000,430000" "features"

echo "🗂️ === FILTER-CRS TESTS ==="
run_test "Filter-CRS with DWITHIN geo coords" "$BASE_URL/collections/test_crs/items.html?filter=DWITHIN(geom,POINT(-124.6%2049.3),40000)&limit=100" "html"
run_test "Filter-CRS with specific CRS" "$BASE_URL/collections/test_crs/items.html?filter=DWITHIN(geom,POINT(1000000%20400000),60000)&filter-crs=3005&limit=100" "html"
run_test "Filter-CRS with ENVELOPE" "$BASE_URL/collections/test_crs/items.html?filter=INTERSECTS(geom,%20ENVELOPE(1000000,400000,1100000,500000))&filter-crs=3005&limit=100" "html"

echo "📏 === COLUMN NAME TESTS ==="
run_test "Quoted column names" "$BASE_URL/collections/test_names/items?properties=id,\"colCamelCase\"" "colCamelCase"
run_test "Case sensitive filter" "$BASE_URL/collections/test_names/items?filter=\"colCamelCase\"%20=%201" "features"
run_test "Column names from original test" "$BASE_URL/collections/test_names/items.json?properties=id,colCamelCase" "colCamelCase"

echo "⚠️ === ERROR HANDLING TESTS ==="
run_test "Non-existent collection" "$BASE_URL/collections/nonexistent/items" "Collection not found"
run_test "Invalid property" "$BASE_URL/collections/test_geom/items?properties=invalid_column" "FeatureCollection"
run_test "Malformed filter" "$BASE_URL/collections/test_geom/items?filter=invalid%20syntax" "CQL syntax error"

echo "⚙️ === FUNCTIONS TESTS ==="
run_test "Functions endpoint" "$BASE_URL/functions" "functions"
run_test "Non-existent function" "$BASE_URL/functions/nonexistent_function/items.json" "Function not found"

echo "🚀 === PERFORMANCE TESTS ==="
run_test "Large result set" "$BASE_URL/collections/test_crs/items" "features"
run_test "Combined filter and pagination" "$BASE_URL/collections/test_crs/items?filter=id%20%3E%2050&limit=10" "features"
run_test "Large limit test" "$BASE_URL/collections/test_crs/items?limit=1000" "features"

echo ""
echo "📊 === TEST SUMMARY ==="
echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
echo "Total Tests: $((TESTS_PASSED + TESTS_FAILED))"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed.${NC}"
    exit 1
fi
