// Package cache provides caching functionality for scan results.
package cache

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lautaror/real-disk-map/core/models"
)

// Cache provides disk-based caching of scan results.
type Cache struct {
	Dir     string
	entries map[string]*CachedEntry
	dirty   bool
}

// CachedEntry represents a cached scan entry with metadata.
type CachedEntry struct {
	Entry     *models.Entry `json:"entry"`
	ScannedAt time.Time     `json:"scanned_at"`
	ModTime   time.Time     `json:"mod_time"`
	Size      int64         `json:"size"`
	PathHash  string        `json:"path_hash"`
}

// NewCache creates a new cache instance.
func NewCache(cacheDir string) (*Cache, error) {
	if cacheDir == "" {
		// Use default cache directory
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		cacheDir = filepath.Join(home, ".cache", "real-disk-map")
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	c := &Cache{
		Dir:     cacheDir,
		entries: make(map[string]*CachedEntry),
	}

	// Load existing cache
	if err := c.Load(); err != nil {
		// It's OK if loading fails, we'll start fresh
		c.entries = make(map[string]*CachedEntry)
	}

	return c, nil
}

// Get retrieves an entry from the cache.
func (c *Cache) Get(path string) (*CachedEntry, bool) {
	hash := hashPath(path)
	entry, ok := c.entries[hash]
	if !ok {
		return nil, false
	}

	// Validate the cached entry is still valid
	if !c.isValid(entry, path) {
		delete(c.entries, hash)
		c.dirty = true
		return nil, false
	}

	return entry, true
}

// Set stores an entry in the cache.
func (c *Cache) Set(path string, entry *models.Entry, modTime time.Time) {
	hash := hashPath(path)
	c.entries[hash] = &CachedEntry{
		Entry:     entry,
		ScannedAt: time.Now(),
		ModTime:   modTime,
		Size:      entry.TotalLogicalSize(),
		PathHash:  hash,
	}
	c.dirty = true
}

// Invalidate removes an entry from the cache.
func (c *Cache) Invalidate(path string) {
	hash := hashPath(path)
	delete(c.entries, hash)
	c.dirty = true
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() error {
	c.entries = make(map[string]*CachedEntry)
	c.dirty = true
	return c.Save()
}

// isValid checks if a cached entry is still valid.
func (c *Cache) isValid(entry *CachedEntry, path string) bool {
	// Check if file still exists and hasn't changed
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// If modification time changed, cache is invalid
	if info.ModTime() != entry.ModTime {
		return false
	}

	// If size changed, cache is invalid
	if info.Size() != entry.Size {
		return false
	}

	return true
}

// Load loads the cache from disk.
func (c *Cache) Load() error {
	cacheFile := filepath.Join(c.Dir, "cache.json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet
		}
		return err
	}

	var entries map[string]*CachedEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal cache: %w", err)
	}

	c.entries = entries
	return nil
}

// Save saves the cache to disk.
func (c *Cache) Save() error {
	if !c.dirty {
		return nil
	}

	cacheFile := filepath.Join(c.Dir, "cache.json")

	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	c.dirty = false
	return nil
}

// Close saves the cache and cleans up.
func (c *Cache) Close() error {
	return c.Save()
}

// Stats returns statistics about the cache.
func (c *Cache) Stats() CacheStats {
	return CacheStats{
		Entries:  len(c.entries),
		CacheDir: c.Dir,
		IsDirty:  c.dirty,
	}
}

// CacheStats holds statistics about the cache.
type CacheStats struct {
	Entries  int
	CacheDir string
	IsDirty  bool
}

// hashPath creates a hash of a path for use as a cache key.
func hashPath(path string) string {
	h := md5.New()
	h.Write([]byte(path))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// MemoryCache provides an in-memory cache that doesn't persist to disk.
type MemoryCache struct {
	entries map[string]*models.Entry
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		entries: make(map[string]*models.Entry),
	}
}

// Get retrieves an entry from the memory cache.
func (m *MemoryCache) Get(path string) (*models.Entry, bool) {
	entry, ok := m.entries[path]
	return entry, ok
}

// Set stores an entry in the memory cache.
func (m *MemoryCache) Set(path string, entry *models.Entry) {
	m.entries[path] = entry
}

// Clear removes all entries from the memory cache.
func (m *MemoryCache) Clear() {
	m.entries = make(map[string]*models.Entry)
}

// Len returns the number of entries in the cache.
func (m *MemoryCache) Len() int {
	return len(m.entries)
}
