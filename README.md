# duckdb-tileserver

A lightweight MVT (Mapbox Vector Tile) tileserver for [DuckDB](https://duckdb.org/) with [duckdb-spatial](https://github.com/duckdb/duckdb-spatial) support, written in [Go](https://golang.org/).

Serves vector tiles directly from DuckDB databases using the new `ST_AsMVT` function from DuckDB Spatial v1.4+.

For a complete list of implemented features, see [FEATURES.md](FEATURES.md).

## Contents

- [Features](#features)
- [Download](#download)
- [Preparing Your Database](#preparing-your-database)
  - [Prerequisites](#prerequisites)
  - [Basic Setup](#basic-setup)
  - [Importing Data](#importing-data)
  - [Setting the Spatial Reference System (SRID)](#setting-the-spatial-reference-system-srid)
  - [Creating Spatial Indexes](#creating-spatial-indexes)
  - [Validating Your Data](#validating-your-data)
  - [Optimizing for Zoom Levels](#optimizing-for-zoom-levels)
  - [Example: Complete Database Setup](#example-complete-database-setup)
  - [Supported Geometry Types](#supported-geometry-types)
  - [Tips](#tips)
- [Quick Start](#quick-start)
- [Build from Source](#build-from-source)
  - [With Go installed](#with-go-installed)
  - [Without Go (Docker build)](#without-go-docker-build)
  - [Docker Image](#docker-image)
- [Configuration](#configuration)
  - [Configuration Using Environment Variables](#configuration-using-environment-variables)
  - [SSL Configuration](#ssl-configuration)
- [API Endpoints](#api-endpoints)
  - [Tile Endpoints](#tile-endpoints)
  - [Cache Management Endpoints](#cache-management-endpoints)
  - [Example Requests](#example-requests)
  - [Using with MapLibre GL JS](#using-with-maplibre-gl-js)
- [Data Requirements](#data-requirements)
- [Command-line Options](#command-line-options)
- [Sample Data](#sample-data)
- [Performance Tips](#performance-tips)
- [Troubleshooting](#troubleshooting)
  - [Enable Debug Logging](#enable-debug-logging)
  - [Common Issues](#common-issues)
- [Architecture](#architecture)
- [License](#license)
- [Credits](#credits)

## Features

* **MVT Tile Server**: Serves Mapbox Vector Tiles (MVT/PBF) for all spatial tables
* **Auto-discovery**: Automatically discovers all tables with geometry columns
* **Multi-SRID support**: Automatically transforms geometries to Web Mercator (EPSG:3857)
* **TileJSON metadata**: Provides TileJSON spec endpoints for each layer
* **Interactive map viewer**: Built-in HTML viewer with MapLibre GL JS
* **Fast & Efficient**: Leverages DuckDB's columnar storage and spatial indexing
* **Spatial extension**: Uses DuckDB Spatial's `ST_AsMVT` for efficient tile generation
* **RESTful API**:
  * `/` - Interactive map viewer
  * `/layers` - List all available layers
  * `/tiles/{layer}/{z}/{x}/{y}.mvt` - MVT tiles
  * `/tiles/{layer}.json` - TileJSON metadata
  * `/health` - Health check endpoint
* **Full HTTP support**:
  * CORS support with configurable origins
  * GZIP response encoding
  * HTTP and HTTPS support

## Download

Builds of the latest code:

* [GitHub Releases (Linux/MacOS/Windows binaries)](https://github.com/tobilg/duckdb-tileserver/releases)
* [Docker image](https://hub.docker.com/repository/docker/tobilg/duckdb-tileserver/general)

## Preparing Your Database

To use duckdb-tileserver, you need a DuckDB database with spatial data. This section covers how to prepare your database properly.

### Prerequisites

1. **Install DuckDB CLI**: Download from [duckdb.org](https://duckdb.org/docs/installation/)
2. **Load the Spatial Extension**: The spatial extension provides geometry types and functions

### Basic Setup

```sql
-- Load the spatial extension
INSTALL spatial;
LOAD spatial;

-- Create a table with a geometry column
CREATE TABLE buildings (
    id INTEGER PRIMARY KEY,
    name VARCHAR,
    geom GEOMETRY
);

-- Insert some data (example using WKT)
INSERT INTO buildings VALUES
    (1, 'Building A', ST_GeomFromText('POLYGON((0 0, 10 0, 10 10, 0 10, 0 0))'));
```

### Importing Data

#### From Shapefiles

```sql
LOAD spatial;

-- Import shapefile directly
COPY buildings FROM 'buildings.shp' WITH (FORMAT GDAL, DRIVER 'ESRI Shapefile');
```

#### From GeoJSON

```sql
LOAD spatial;

-- Import GeoJSON file
COPY buildings FROM 'buildings.geojson' WITH (FORMAT GDAL, DRIVER 'GeoJSON');
```

#### From GeoParquet

```sql
LOAD spatial;

-- Read GeoParquet file
CREATE TABLE buildings AS
    SELECT * FROM ST_Read('buildings.parquet');
```

#### From PostGIS

```sql
LOAD spatial;
INSTALL postgres;
LOAD postgres;

-- Attach PostgreSQL database
ATTACH 'dbname=mydb user=postgres host=localhost' AS pg (TYPE postgres);

-- Copy data from PostGIS
CREATE TABLE buildings AS
    SELECT id, name, geom::GEOMETRY as geom
    FROM pg.public.buildings;
```

### Setting the Spatial Reference System (SRID)

For best performance, set your geometries to EPSG:3857 (Web Mercator). If your data uses a different SRID, you can transform it:

```sql
-- Check current SRID
SELECT DISTINCT ST_SRID(geom) FROM buildings;

-- Transform to Web Mercator (EPSG:3857)
UPDATE buildings
SET geom = ST_Transform(geom, 'EPSG:4326', 'EPSG:3857');

-- Or create a new column with transformed geometries
ALTER TABLE buildings ADD COLUMN geom_3857 GEOMETRY;
UPDATE buildings
SET geom_3857 = ST_Transform(geom, 'EPSG:4326', 'EPSG:3857');
```

### Creating Spatial Indexes

R-Tree indexes significantly improve tile generation performance:

```sql
-- Create R-Tree spatial index on geometry column
CREATE INDEX buildings_geom_idx ON buildings USING RTREE (geom);

-- Verify index was created
SELECT * FROM duckdb_indexes() WHERE table_name = 'buildings';
```

### Validating Your Data

Ensure your geometries are valid before serving tiles:

```sql
-- Check for invalid geometries
SELECT id, ST_IsValid(geom) as is_valid, ST_IsValidReason(geom) as reason
FROM buildings
WHERE NOT ST_IsValid(geom);

-- Fix invalid geometries
UPDATE buildings
SET geom = ST_MakeValid(geom)
WHERE NOT ST_IsValid(geom);
```

### Optimizing for Zoom Levels

For large datasets, consider creating simplified versions for lower zoom levels:

```sql
-- Create simplified geometries for overview zoom levels
CREATE TABLE buildings_simplified AS
SELECT
    id,
    name,
    ST_Simplify(geom, 100) as geom  -- Simplify with 100m tolerance
FROM buildings
WHERE ST_Area(geom) > 10000;  -- Only keep larger features

-- Create spatial index on simplified table
CREATE INDEX buildings_simplified_geom_idx
    ON buildings_simplified USING RTREE (geom);
```

### Example: Complete Database Setup

```sql
-- Create database and load extensions
INSTALL spatial;
LOAD spatial;

-- Import data from GeoJSON
COPY buildings FROM 'buildings.geojson' WITH (FORMAT GDAL, DRIVER 'GeoJSON');

-- Transform to Web Mercator if needed
UPDATE buildings
SET geom = ST_Transform(geom, 'EPSG:4326', 'EPSG:3857')
WHERE ST_SRID(geom) = 4326;

-- Validate and fix geometries
UPDATE buildings
SET geom = ST_MakeValid(geom)
WHERE NOT ST_IsValid(geom);

-- Create spatial index
CREATE INDEX buildings_geom_idx ON buildings USING RTREE (geom);

-- Verify setup
SELECT
    COUNT(*) as feature_count,
    ST_SRID(geom) as srid,
    ST_GeometryType(geom) as geometry_type
FROM buildings
GROUP BY ST_SRID(geom), ST_GeometryType(geom);
```

### Supported Geometry Types

DuckDB Spatial supports all standard OGC geometry types:

* **POINT** / **MULTIPOINT** - For point features (POIs, markers)
* **LINESTRING** / **MULTILINESTRING** - For linear features (roads, rivers)
* **POLYGON** / **MULTIPOLYGON** - For area features (buildings, parcels)
* **GEOMETRYCOLLECTION** - Mixed geometry types

### Tips

* **Column naming**: The tileserver auto-detects geometry columns - use standard names like `geom`, `geometry`, or `wkb_geometry`
* **Multiple geometry columns**: If a table has multiple geometry columns, the tileserver uses the first one and logs a warning
* **Data types**: All standard DuckDB data types are preserved in tile properties
* **File size**: DuckDB uses columnar compression - expect significant space savings compared to shapefiles
* **Read-only mode**: The tileserver opens databases in read-only mode, so your data is safe

## Quick Start

### 1. Prepare Your Data

Create a DuckDB database with spatial data:

```bash
# Create sample data
duckdb tiles.db < testing/sample_data.sql
```

This creates sample tables: `buildings`, `roads`, `poi`, and `parcels` with geometry columns.

### 2. Run the Server

```bash
./duckdb-tileserver --database-path tiles.db
```

### 3. View the Map

Open http://localhost:9000/ in your browser to see an interactive map with all your spatial layers.

## Build from Source

`duckdb-tileserver` requires Go 1.24+ to support the latest DuckDB driver.

### With Go installed

```bash
cd duckdb-tileserver/
go build
```

This creates a `duckdb-tileserver` executable in the application directory.

### Without Go (Docker build)

```bash
make APPVERSION=<VERSION> clean build-in-docker
```

### Docker Image

#### Build the image

```bash
make APPVERSION=<VERSION> clean docker
```

#### Run the image

```bash
docker run --rm -dt \
  -v "$PWD/tiles.db:/data/tiles.db" \
  -e DUCKDBTS_DATABASE_PATH=/data/tiles.db \
  -p 9000:9000 \
  tobilg/duckdb-tileserver:<VERSION>
```

#### Complete example with sample data

```bash
# Build the Docker image
make APPVERSION=0.1.1 clean docker

# Create sample database with spatial data
duckdb tiles.db < testing/sample_data.sql

# Run the container
docker run --rm -dt \
  -v "$PWD/tiles.db:/data/tiles.db" \
  -e DUCKDBTS_DATABASE_PATH=/data/tiles.db \
  -p 9000:9000 \
  --name duckdb-tileserver \
  tobilg/duckdb-tileserver:0.1.1

# View logs
docker logs -f duckdb-tileserver

# Access the interactive map viewer at http://localhost:9000
# Stop the container when done
docker stop duckdb-tileserver
```

## Configuration

The configuration file (see [example](config/duckdb-tileserver.toml.example)) is automatically read from:

* `/etc/duckdb-tileserver.toml`
* `./config/duckdb-tileserver.toml`
* `/config/duckdb-tileserver.toml`

To specify a configuration file directly use the `--config` parameter.

### Configuration Using Environment Variables

All configuration options can be set via environment variables using the prefix `DUCKDBTS_` followed by the section and key name separated by underscores. For nested configuration like `Server.HttpPort`, use `DUCKDBTS_SERVER_HTTPPORT`.

#### Database Configuration

```bash
# Database path (required)
export DUCKDBTS_DATABASE_PATH="/path/to/database.db"

# Filter which tables to serve as tile layers (comma-separated)
export DUCKDBTS_DATABASE_TABLEINCLUDES="buildings,roads,parcels"
export DUCKDBTS_DATABASE_TABLEEXCLUDES="temp_table,staging"

# Function includes (comma-separated, default: "postgisftw")
export DUCKDBTS_DATABASE_FUNCTIONINCLUDES="func1,func2"

# Connection pool settings (for performance tuning)
export DUCKDBTS_DATABASE_MAXOPENCONNS=25        # Max open connections (default: 25)
export DUCKDBTS_DATABASE_MAXIDLECONNS=5         # Max idle connections (default: 5)
export DUCKDBTS_DATABASE_CONNMAXLIFETIME=3600   # Connection max lifetime in seconds (default: 3600)
export DUCKDBTS_DATABASE_CONNMAXIDLETIME=600    # Idle connection timeout in seconds (default: 600)
```

#### Server Configuration

```bash
# HTTP server settings
export DUCKDBTS_SERVER_HTTPHOST="0.0.0.0"
export DUCKDBTS_SERVER_HTTPPORT=9000
export DUCKDBTS_SERVER_HTTPSPORT=9001

# TLS/HTTPS configuration
export DUCKDBTS_SERVER_TLSSERVERCERTIFICATEFILE="/path/to/server.crt"
export DUCKDBTS_SERVER_TLSSERVERPRIVATEKEYFILE="/path/to/server.key"

# URL configuration
export DUCKDBTS_SERVER_URLBASE="https://example.com"
export DUCKDBTS_SERVER_BASEPATH="/tiles"

# CORS origins (default: "*")
export DUCKDBTS_SERVER_CORSORIGINS="https://example.com,https://app.example.com"

# Debug mode
export DUCKDBTS_SERVER_DEBUG=true

# Assets path for HTML templates
export DUCKDBTS_SERVER_ASSETSPATH="./assets"

# Timeouts in seconds
export DUCKDBTS_SERVER_READTIMEOUTSEC=5
export DUCKDBTS_SERVER_WRITETIMEOUTSEC=30

# Disable HTML UI
export DUCKDBTS_SERVER_DISABLEUI=false
```

#### Metadata Configuration

```bash
# Service metadata
export DUCKDBTS_METADATA_TITLE="My Tile Server"
export DUCKDBTS_METADATA_DESCRIPTION="Custom tile server description"
```

#### Website Configuration

```bash
# Custom basemap URL for the map viewer
export DUCKDBTS_WEBSITE_BASEMAPURL="https://tile.openstreetmap.org/{z}/{x}/{y}.png"
```

#### Cache Configuration

```bash
# Cache settings
export DUCKDBTS_CACHE_ENABLED=true
export DUCKDBTS_CACHE_MAXITEMS=10000
export DUCKDBTS_CACHE_MAXMEMORYMB=1024
export DUCKDBTS_CACHE_BROWSERCACHEMAXAGE=3600  # Browser cache max-age in seconds

# Cache management API endpoints
export DUCKDBTS_CACHE_DISABLEAPI=false  # Disable /cache/* endpoints
export DUCKDBTS_CACHE_APIKEY="your-secret-key"  # Require API key for cache endpoints
```

#### Paging Configuration

```bash
# Pagination settings for feature queries
export DUCKDBTS_PAGING_LIMITDEFAULT=10
export DUCKDBTS_PAGING_LIMITMAX=1000
```

### SSL Configuration

For SSL support, generate or provide a server certificate and private key:

```bash
# Generate self-signed certificate for testing
openssl req -nodes -new -x509 -keyout server.key -out server.crt
```

Configure in config file:
```toml
[Server]
TlsServerCertificateFile = "/path/server.crt"
TlsServerPrivateKeyFile = "/path/server.key"
HttpsPort = 9001
```

## API Endpoints

### Tile Endpoints

* **GET /** - Interactive map viewer (HTML)
* **GET /layers** - List all available spatial layers (JSON)
* **GET /tiles/{layer}.json** - TileJSON metadata for a layer
* **GET /tiles/{layer}/{z}/{x}/{y}.mvt** - MVT tile for a layer
* **GET /tiles/{layer}/{z}/{x}/{y}.pbf** - MVT tile (alternative extension)
* **GET /health** - Health check endpoint

### Cache Management Endpoints

These endpoints allow you to manage the tile cache. They can be disabled via configuration and optionally protected with an API key.

* **GET /cache/stats** - Get cache statistics (hits, misses, hit rate, size, memory usage)
* **DELETE /cache/clear** - Clear the entire tile cache
* **DELETE /cache/layer/{layer}** - Clear cache for a specific layer

**Authentication:** If `ApiKey` is configured in the `[Cache]` section, include the `X-API-Key` header:
```bash
curl -H "X-API-Key: your-secret-key" http://localhost:9000/cache/stats
```

**Configuration:**
- Set `DUCKDBTS_CACHE_DISABLEAPI=true` to disable these endpoints
- Set `DUCKDBTS_CACHE_APIKEY=your-secret-key` to require authentication

### Example Requests

```bash
# List available layers
curl http://localhost:9000/layers

# Get TileJSON metadata for buildings layer
curl http://localhost:9000/tiles/buildings.json

# Get a specific tile (zoom 12, x=1205, y=1539)
curl http://localhost:9000/tiles/buildings/12/1205/1539.mvt -o tile.mvt

# Check service health
curl http://localhost:9000/health

# Cache management (without authentication)
curl http://localhost:9000/cache/stats
curl -X DELETE http://localhost:9000/cache/clear
curl -X DELETE http://localhost:9000/cache/layer/buildings

# Cache management (with API key authentication)
curl -H "X-API-Key: your-secret-key" http://localhost:9000/cache/stats
curl -H "X-API-Key: your-secret-key" -X DELETE http://localhost:9000/cache/clear
```

### Using with MapLibre GL JS

```javascript
map.addSource('buildings', {
  type: 'vector',
  tiles: ['http://localhost:9000/tiles/buildings/{z}/{x}/{y}.mvt'],
  minzoom: 0,
  maxzoom: 22
});

map.addLayer({
  id: 'buildings-fill',
  type: 'fill',
  source: 'buildings',
  'source-layer': 'buildings',
  paint: {
    'fill-color': '#3388ff',
    'fill-opacity': 0.6
  }
});
```

## Data Requirements

Tables must have a geometry column to be served as tile layers. DuckDB Spatial supports various geometry types:

* POINT / MULTIPOINT
* LINESTRING / MULTILINESTRING
* POLYGON / MULTIPOLYGON
* GEOMETRYCOLLECTION

The tileserver will:
1. Auto-detect all tables with geometry columns
2. Use the first geometry column if multiple exist (with a warning)
3. Automatically transform geometries to EPSG:3857 (Web Mercator) for tiles
4. Apply include/exclude filters from configuration

## Command-line Options

* `-?` - Show command usage
* `--config file.toml` - Specify configuration file
* `--debug` - Set logging level to TRACE
* `--devel` - Run in development mode (reload templates on each request)
* `--disable-ui` - Disable HTML UI routes
* `--test` - Run with mock data for testing
* `--version` - Display version number
* `--database-path path` - Path to DuckDB database file

## Sample Data

Generate sample spatial data for testing:

```bash
duckdb tiles.db < testing/sample_data.sql
./duckdb-tileserver --database-path tiles.db
```

This creates four sample layers:
* **buildings** - 1000 polygon features (Web Mercator)
* **roads** - 500 linestring features (WGS84)
* **poi** - 200 point features (Web Mercator)
* **parcels** - 300 polygon features (Web Mercator)

## Performance Tips

1. **Spatial Indexes**: Create R-Tree indexes on geometry columns for better performance:
   ```sql
   CREATE INDEX buildings_geom_idx ON buildings USING RTREE (geom);
   ```
2. **Coordinate Reference System**: Store data in EPSG:3857 (Web Mercator) to avoid transformation overhead
3. **Connection Pooling**: Configure connection pool settings based on your workload:
   - `MaxOpenConns`: Set to 2-4x your CPU cores (default: 25)
   - `MaxIdleConns`: Keep warm connections available (default: 5)
   - The server uses a shared connection pool to efficiently handle concurrent tile requests
4. **Caching**: The built-in LRU cache significantly reduces database load:
   - Layer metadata is automatically cached to eliminate repeated queries
   - Tile cache can store up to 10,000 tiles (configurable)
   - Browser caching reduces server requests (default: 1 hour)
5. **Table Filtering**: Use `TableIncludes` to serve only necessary tables
6. **Zoom Levels**: Consider creating pre-aggregated tables for lower zoom levels

## Troubleshooting

### Enable Debug Logging

```bash
./duckdb-tileserver --debug --database-path tiles.db
```

Or in config file:
```toml
[Server]
Debug = true
```

### Common Issues

**No layers appear:**
- Check that tables have geometry columns: `SELECT * FROM duckdb_columns WHERE data_type = 'GEOMETRY'`
- Verify spatial extension is loaded: `LOAD spatial;`
- Check table filters in configuration

**Empty tiles:**
- Verify data has geometries in the tile extent
- Check SRID of geometries: `SELECT DISTINCT ST_SRID(geometry) FROM your_table`
- Ensure geometries are valid: `SELECT ST_IsValid(geometry) FROM your_table`

**Slow tile generation:**
- Create R-Tree spatial indexes on geometry columns if not already present
- Consider filtering data by zoom level
- Pre-aggregate data for lower zoom levels

## Architecture

```
┌─────────────────┐
│   HTTP Client   │
│ (Browser/QGIS)  │
└────────┬────────┘
         │
         ↓
┌─────────────────┐
│  HTTP Handlers  │
│  (Gorilla Mux)  │
└────────┬────────┘
         │
         ↓
┌─────────────────┐
│  Tile Generator │
│  (ST_AsMVT)     │
└────────┬────────┘
         │
         ↓
┌─────────────────┐
│   DuckDB + SQL  │
│  Spatial Ext.   │
└─────────────────┘
```
