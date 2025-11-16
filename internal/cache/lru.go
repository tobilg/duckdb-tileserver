package cache

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
	"fmt"
	"strings"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
	log "github.com/sirupsen/logrus"
)

// TileCache provides thread-safe LRU caching for MVT tiles
type TileCache struct {
	cache       *lru.Cache[string, []byte]
	enabled     bool
	maxMemoryMB int64

	// Metrics (atomic counters for thread-safety)
	hits        atomic.Int64
	misses      atomic.Int64
	evictions   atomic.Int64
	currentSize atomic.Int64
	currentBytes atomic.Int64
}

// Stats represents cache statistics
type Stats struct {
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	Evictions   int64   `json:"evictions"`
	Size        int     `json:"size"`         // Number of items
	MemoryBytes int64   `json:"memory_bytes"`
	HitRate     float64 `json:"hit_rate"` // Percentage
}

// NewTileCache creates a new LRU tile cache
func NewTileCache(maxItems int, maxMemoryMB int) (*TileCache, error) {
	if maxItems <= 0 {
		return nil, fmt.Errorf("maxItems must be positive, got %d", maxItems)
	}

	tc := &TileCache{
		enabled:     true,
		maxMemoryMB: int64(maxMemoryMB),
	}

	// Create LRU cache with eviction callback
	cache, err := lru.NewWithEvict(maxItems, tc.onEvict)
	if err != nil {
		return nil, err
	}
	tc.cache = cache

	log.Infof("Initialized tile cache: max_items=%d max_memory=%dMB", maxItems, maxMemoryMB)
	return tc, nil
}

// NewDisabledCache returns a cache that's disabled (always misses)
func NewDisabledCache() *TileCache {
	return &TileCache{enabled: false}
}

// Get retrieves a tile from cache
func (tc *TileCache) Get(ctx context.Context, key string) ([]byte, bool) {
	if !tc.enabled {
		return nil, false
	}

	tile, ok := tc.cache.Get(key)
	if ok {
		tc.hits.Add(1)
		log.Debugf("Cache HIT: %s", key)
		return tile, true
	}

	tc.misses.Add(1)
	log.Debugf("Cache MISS: %s", key)
	return nil, false
}

// Set stores a tile in cache
func (tc *TileCache) Set(ctx context.Context, key string, data []byte) error {
	if !tc.enabled || len(data) == 0 {
		return nil
	}

	tileSize := int64(len(data))

	// Check memory limit before adding
	if tc.maxMemoryMB > 0 {
		currentMB := tc.currentBytes.Load() / 1024 / 1024
		tileMB := tileSize / 1024 / 1024

		if currentMB+tileMB > tc.maxMemoryMB {
			log.Debugf("Cache memory limit reached, evicting to make space")
			// LRU will automatically evict oldest items
		}
	}

	// Make a copy to avoid referencing request data
	tileCopy := make([]byte, len(data))
	copy(tileCopy, data)

	tc.cache.Add(key, tileCopy)
	tc.currentBytes.Add(tileSize)
	tc.currentSize.Add(1)

	log.Debugf("Cache SET: %s (%d bytes)", key, tileSize)
	return nil
}

// onEvict is called when an item is evicted from the LRU cache
func (tc *TileCache) onEvict(key string, value []byte) {
	tc.evictions.Add(1)
	tc.currentSize.Add(-1)
	tc.currentBytes.Add(-int64(len(value)))
	log.Debugf("Cache EVICT: %s", key)
}

// Clear removes all items from cache
func (tc *TileCache) Clear() {
	if !tc.enabled {
		return
	}

	tc.cache.Purge()
	tc.currentSize.Store(0)
	tc.currentBytes.Store(0)
	log.Info("Cache cleared")
}

// ClearLayer removes all tiles for a specific layer
func (tc *TileCache) ClearLayer(layerName string) int {
	if !tc.enabled {
		return 0
	}

	removed := 0
	keys := tc.cache.Keys()

	prefix := layerName + ":"
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			tc.cache.Remove(key)
			removed++
		}
	}

	log.Infof("Cleared %d tiles for layer %s", removed, layerName)
	return removed
}

// Stats returns current cache statistics
func (tc *TileCache) Stats() Stats {
	if !tc.enabled {
		return Stats{}
	}

	hits := tc.hits.Load()
	misses := tc.misses.Load()
	total := hits + misses

	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100.0
	}

	return Stats{
		Hits:        hits,
		Misses:      misses,
		Evictions:   tc.evictions.Load(),
		Size:        tc.cache.Len(),
		MemoryBytes: tc.currentBytes.Load(),
		HitRate:     hitRate,
	}
}

// Enabled returns whether the cache is enabled
func (tc *TileCache) Enabled() bool {
	return tc.enabled
}
