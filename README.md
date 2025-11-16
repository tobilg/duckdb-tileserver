# duckdb-tileserver

A lightweight MVT (Mapbox Vector Tile) tileserver for [DuckDB](https://duckdb.org/) with [duckdb-spatial](https://github.com/duckdb/duckdb-spatial) support, written in [Go](https://golang.org/).

Serves vector tiles directly from DuckDB databases using the new `ST_AsMVT` function from DuckDB Spatial v1.4+.

For a complete list of implemented features, see [FEATURES.md](FEATURES.md).

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

* [Linux](https://artifacts.serverless-duckdb.com/duckdb-tileserver_latest_linux.zip)
* [Windows](https://artifacts.serverless-duckdb.com/duckdb-tileserver_latest_windows.zip)
* [MacOS](https://artifacts.serverless-duckdb.com/duckdb-tileserver_latest_macos.zip)
* [Docker image](https://hub.docker.com/repository/docker/tobilg/duckdb-tileserver/general)

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
make APPVERSION=0.1.0 clean docker

# Create sample database with spatial data
duckdb tiles.db < testing/sample_data.sql

# Run the container
docker run --rm -dt \
  -v "$PWD/tiles.db:/data/tiles.db" \
  -e DUCKDBTS_DATABASE_PATH=/data/tiles.db \
  -p 9000:9000 \
  --name duckdb-tileserver \
  tobilg/duckdb-tileserver:0.1.0

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

1. **Spatial Indexes**: DuckDB Spatial automatically creates R-Tree indexes for geometry columns
2. **Coordinate Reference System**: Store data in EPSG:3857 (Web Mercator) to avoid transformation overhead
3. **Connection Pooling**: The Go database/sql package handles connection pooling automatically
4. **Table Filtering**: Use `TableIncludes` to serve only necessary tables
5. **Zoom Levels**: Consider creating pre-aggregated tables for lower zoom levels

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
- Check if spatial indexes exist (automatic in DuckDB Spatial)
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

## License

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

## Credits

This project is adapted from [`pg_featureserv`](https://github.com/CrunchyData/pg_featureserv)
by Crunchy Data Solutions, refactored to work with DuckDB Spatial instead of PostgreSQL/PostGIS.
