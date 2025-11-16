# DuckDB Tileserver Features

A lightweight MVT (Mapbox Vector Tile) server for DuckDB with spatial extension support.

This project is adapted from [`pg_featureserv`](https://github.com/CrunchyData/pg_featureserv) by Crunchy Data Solutions, refactored to work with DuckDB Spatial instead of PostgreSQL/PostGIS.

## API

- [x] Determine response format from request headers `Content-Type`, `Accept`
- [x] CORS support (configurable origins)
- [x] GZIP encoding (via compress handler)
- [x] HTTPS support (optional TLS configuration)
- [x] Proxy support via configurable base URL and base path

## Endpoints

### Core Endpoints
- [x] `/` - Interactive map viewer (HTML landing page)
- [x] `/index.html`, `/home.html` - Alternative routes to landing page
- [x] `/health` - Health check endpoint
- [x] `/layers` - List all available spatial layers (JSON)
- [x] `/layers.json` - Alternative route to layers endpoint

### Tile Endpoints
- [x] `/tiles/{layer}.json` - TileJSON metadata endpoint
- [x] `/tiles/{layer}/{z}/{x}/{y}.mvt` - MVT tile endpoint
- [x] `/tiles/{layer}/{z}/{x}/{y}.pbf` - MVT tile endpoint (alternative extension)

### Cache Management Endpoints
- [x] `/cache/stats` - GET cache statistics (hits, misses, hit rate, size, memory, evictions)
- [x] `/cache/clear` - DELETE entire cache
- [x] `/cache/layer/{layer}` - DELETE layer-specific tiles
- [x] Optional API key authentication via `X-API-Key` header
- [x] Configurable enable/disable of cache management endpoints

## Tile Features

- [x] MVT (Mapbox Vector Tile) generation using DuckDB's `ST_AsMVT` function
- [x] Auto-discovery of all tables with geometry columns
- [x] Multi-SRID support with automatic transformation to Web Mercator (EPSG:3857)
- [x] Validation of tile coordinates (z, x, y ranges)
- [x] Empty tile handling (returns 204 No Content when no features in tile)
- [x] TileJSON 2.2.0 specification support
- [x] Geometry type detection and metadata
- [x] Table schema and column metadata in TileJSON

## Cache Features

- [x] LRU tile cache with configurable size and memory limits
- [x] Cache statistics tracking (hits, misses, hit rate, evictions)
- [x] Periodic cache statistics logging (every 5 minutes)
- [x] Cache middleware for tile endpoints
- [x] Browser cache control via `Cache-Control` headers
- [x] Configurable cache enable/disable
- [x] Memory-based eviction when cache exceeds limits
- [x] Layer-specific cache clearing
- [x] Full cache clearing
- [x] Cache management API authentication with configurable API key
- [x] Public and authenticated modes for cache endpoints

## Configuration

- [x] Read config from TOML file
- [x] Configuration file search paths (`/etc`, `./config`, `/config`)
- [x] Environment variable configuration with `DUCKDBTS_` prefix
- [x] Database connection path configuration
- [x] Table include/exclude filters
- [x] HTTP/HTTPS server settings (host, ports)
- [x] TLS certificate and key file paths
- [x] URL base and base path configuration
- [x] CORS origins configuration
- [x] Debug mode
- [x] Assets path for HTML templates
- [x] Request/write timeout configuration
- [x] UI enable/disable toggle
- [x] Cache configuration (enabled, max items, max memory, browser cache max-age)
- [x] Cache endpoint security (disable routes, API key authentication)
- [x] Metadata configuration (title, description)
- [x] Website basemap URL configuration

## Operational

- [x] Graceful shutdown with signal handling
- [x] Request timeouts with context cancellation
- [x] Database connection pooling
- [x] Concurrent HTTP and HTTPS servers
- [x] Timeout handler for long-running requests (returns 503 on timeout)
- [x] Abort timeout on shutdown to prevent hanging

## Data Types

- [x] All geometry types via DuckDB Spatial (POINT, LINESTRING, POLYGON, MULTIPOINT, MULTILINESTRING, MULTIPOLYGON, GEOMETRYCOLLECTION)
- [x] Common scalar types: text, int, float, numeric
- [x] Automatic detection of geometry columns
- [x] First geometry column used when multiple exist (with warning)
- [x] Support for tables with and without primary keys

## Tables / Views

- [x] Table column schema discovery
- [x] Support tables with geometry columns
- [x] Support views with geometry columns
- [x] Include/exclude published tables via configuration
- [x] Automatic SRID detection
- [x] Multi-SRID table support with transformation

## User Interface (HTML)

- [x] Interactive HTML landing page with map viewer
- [x] MapLibre GL JS-based map display
- [x] Automatic loading of all available layers
- [x] Layer visibility toggles
- [x] Feature attribute display on click
- [x] Layer list with geometry type indicators
- [x] Configurable basemap URL
- [x] Responsive design
- [x] Development mode for template reloading

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
│  - CORS         │
│  - GZIP         │
│  - Timeout      │
└────────┬────────┘
         │
         ↓
┌─────────────────┐
│  Cache Layer    │
│  (LRU Cache)    │
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

## Performance Features

1. **Tile Caching**: LRU cache with configurable size and memory limits
2. **Spatial Indexing**: DuckDB Spatial automatically creates R-Tree indexes
3. **Connection Pooling**: Go database/sql package handles connection pooling
4. **Efficient Tile Generation**: Uses DuckDB's native `ST_AsMVT` function
5. **GZIP Compression**: Automatic response compression
6. **Request Timeouts**: Prevents long-running queries from blocking
7. **Browser Caching**: Configurable `Cache-Control` headers

## Technology Stack

- **Language**: Go 1.24+
- **Database**: DuckDB with Spatial extension v1.4+
- **HTTP Framework**: Gorilla Mux
- **Map Viewer**: MapLibre GL JS
- **Tile Format**: Mapbox Vector Tiles (MVT/PBF)
- **Configuration**: Viper (TOML + environment variables)
- **Logging**: Logrus
