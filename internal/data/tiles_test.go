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
	"testing"
)

func TestLayerStruct(t *testing.T) {
	layer := &Layer{
		Name:           "buildings",
		Table:          "buildings",
		GeometryColumn: "geom",
		GeometryType:   "POLYGON",
		Srid:           3857,
		Properties:     []string{"id", "name", "height"},
	}

	if layer.Name != "buildings" {
		t.Errorf("Expected layer name 'buildings', got '%s'", layer.Name)
	}

	if layer.Srid != 3857 {
		t.Errorf("Expected SRID 3857, got %d", layer.Srid)
	}

	if len(layer.Properties) != 3 {
		t.Errorf("Expected 3 properties, got %d", len(layer.Properties))
	}
}

func TestTileJSONStruct(t *testing.T) {
	tj := &TileJSON{
		TileJSON: "3.0.0",
		Name:     "test",
		Tiles:    []string{"http://localhost:9000/tiles/test/{z}/{x}/{y}.mvt"},
		MinZoom:  0,
		MaxZoom:  22,
		Bounds:   []float64{-180, -85, 180, 85},
		VectorLayers: []VectorLayer{
			{
				ID:      "test",
				MinZoom: 0,
				MaxZoom: 22,
				Fields:  map[string]string{"id": "string"},
			},
		},
	}

	if tj.TileJSON != "3.0.0" {
		t.Errorf("Expected TileJSON version 3.0.0, got %s", tj.TileJSON)
	}

	if len(tj.Tiles) != 1 {
		t.Errorf("Expected 1 tile URL, got %d", len(tj.Tiles))
	}

	if len(tj.VectorLayers) != 1 {
		t.Errorf("Expected 1 vector layer, got %d", len(tj.VectorLayers))
	}

	if tj.VectorLayers[0].ID != "test" {
		t.Errorf("Expected vector layer ID 'test', got '%s'", tj.VectorLayers[0].ID)
	}
}

func TestExtentStruct(t *testing.T) {
	extent := &Extent{
		Minx: -180,
		Miny: -85,
		Maxx: 180,
		Maxy: 85,
	}

	if extent.Minx != -180 {
		t.Errorf("Expected Minx -180, got %f", extent.Minx)
	}

	if extent.Maxy != 85 {
		t.Errorf("Expected Maxy 85, got %f", extent.Maxy)
	}
}

func TestVectorLayerFields(t *testing.T) {
	fields := map[string]string{
		"id":     "number",
		"name":   "string",
		"active": "boolean",
	}

	vl := VectorLayer{
		ID:      "test",
		Fields:  fields,
		MinZoom: 0,
		MaxZoom: 14,
	}

	if len(vl.Fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(vl.Fields))
	}

	if vl.Fields["name"] != "string" {
		t.Errorf("Expected name field to be 'string', got '%s'", vl.Fields["name"])
	}
}
