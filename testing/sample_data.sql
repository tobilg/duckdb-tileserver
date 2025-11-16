-- Sample test data for DuckDB Tileserver
-- Creates sample tables with spatial data in different SRIDs

-- Install and load spatial extension
INSTALL spatial;
LOAD spatial;

-- Create a sample buildings table in Web Mercator (EPSG:3857)
CREATE TABLE buildings AS
SELECT
    row_number() OVER () as id,
    'Building ' || row_number() OVER () as name,
    random() * 50 + 25 as height,
    ST_Transform(
        ST_GeomFromText(
            'POLYGON((' ||
            (lon - 0.001) || ' ' || (lat - 0.001) || ',' ||
            (lon + 0.001) || ' ' || (lat - 0.001) || ',' ||
            (lon + 0.001) || ' ' || (lat + 0.001) || ',' ||
            (lon - 0.001) || ' ' || (lat + 0.001) || ',' ||
            (lon - 0.001) || ' ' || (lat - 0.001) ||
            '))',
            4326
        ),
        3857
    ) as geometry
FROM (
    SELECT
        -74.0060 + (random() - 0.5) * 0.1 as lon,
        40.7128 + (random() - 0.5) * 0.1 as lat
    FROM range(1000)
);

-- Create a sample roads table in EPSG:4326 (WGS84)
CREATE TABLE roads AS
SELECT
    row_number() OVER () as id,
    'Road ' || row_number() OVER () as name,
    CASE
        WHEN random() < 0.3 THEN 'highway'
        WHEN random() < 0.6 THEN 'primary'
        WHEN random() < 0.8 THEN 'secondary'
        ELSE 'residential'
    END as road_type,
    ST_GeomFromText(
        'LINESTRING(' ||
        lon_start || ' ' || lat_start || ',' ||
        lon_end || ' ' || lat_end ||
        ')',
        4326
    ) as geometry
FROM (
    SELECT
        -74.0060 + (random() - 0.5) * 0.1 as lon_start,
        40.7128 + (random() - 0.5) * 0.1 as lat_start,
        -74.0060 + (random() - 0.5) * 0.1 as lon_end,
        40.7128 + (random() - 0.5) * 0.1 as lat_end
    FROM range(500)
);

-- Create a sample points of interest table
CREATE TABLE poi AS
SELECT
    row_number() OVER () as id,
    poi_name as name,
    poi_type as type,
    ST_Transform(
        ST_Point(lon, lat, 4326),
        3857
    ) as geometry
FROM (
    SELECT
        -74.0060 + (random() - 0.5) * 0.1 as lon,
        40.7128 + (random() - 0.5) * 0.1 as lat,
        CASE
            WHEN random() < 0.2 THEN 'restaurant'
            WHEN random() < 0.4 THEN 'cafe'
            WHEN random() < 0.6 THEN 'park'
            WHEN random() < 0.8 THEN 'museum'
            ELSE 'shop'
        END as poi_type,
        'POI ' || row_number() OVER () as poi_name
    FROM range(200)
);

-- Create a sample parcels table with more complex polygons
CREATE TABLE parcels AS
SELECT
    row_number() OVER () as id,
    'Parcel ' || row_number() OVER () as parcel_id,
    random() * 10000 + 1000 as area_sqm,
    CASE
        WHEN random() < 0.3 THEN 'residential'
        WHEN random() < 0.6 THEN 'commercial'
        WHEN random() < 0.8 THEN 'industrial'
        ELSE 'mixed'
    END as zoning,
    ST_Transform(
        ST_Buffer(ST_Point(lon, lat, 4326), 0.002),
        3857
    ) as geometry
FROM (
    SELECT
        -74.0060 + (random() - 0.5) * 0.1 as lon,
        40.7128 + (random() - 0.5) * 0.1 as lat
    FROM range(300)
);

-- Display summary
SELECT 'buildings' as table_name, COUNT(*) as count, ST_GeometryType(geometry) as geom_type, ST_SRID(geometry) as srid FROM buildings
UNION ALL
SELECT 'roads', COUNT(*), ST_GeometryType(geometry), ST_SRID(geometry) FROM roads
UNION ALL
SELECT 'poi', COUNT(*), ST_GeometryType(geometry), ST_SRID(geometry) FROM poi
UNION ALL
SELECT 'parcels', COUNT(*), ST_GeometryType(geometry), ST_SRID(geometry) FROM parcels;

-- Create spatial indexes for better performance
-- Note: DuckDB spatial extension automatically creates R-Tree indexes for geometry columns
ANALYZE;

-- Show extents
SELECT 'buildings' as layer, ST_AsText(ST_Extent(ST_Transform(geometry, 4326))) as extent_wgs84 FROM buildings
UNION ALL
SELECT 'roads', ST_AsText(ST_Extent(geometry)) FROM roads
UNION ALL
SELECT 'poi', ST_AsText(ST_Extent(ST_Transform(geometry, 4326))) FROM poi
UNION ALL
SELECT 'parcels', ST_AsText(ST_Extent(ST_Transform(geometry, 4326))) FROM parcels;
