package entigo

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// defaultCacheTTL is the default time-to-live for cache entries when no explicit
// expiration is provided.
const defaultCacheTTL = 5 * time.Minute

// CacheService defines the interface for a string-based key-value cache.
type CacheService interface {
	// Get retrieves the value for the given key.
	// Returns ErrCacheMiss if the key does not exist or has expired.
	Get(key string) (string, error)

	// Set stores a value with the default expiration (5 minutes).
	Set(key string, value string) error

	// Delete removes the value for the given key.
	Delete(key string) error
}

// GetObjectCache retrieves a JSON-serialized object from the cache and unmarshals it into dest.
func GetObjectCache[T any](cache CacheService, key string, dest *T) error {
	data, err := cache.Get(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// SetObjectCache stores an object in the cache by marshaling it to JSON,
// using the default expiration.
func SetObjectCache(cache CacheService, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %w", err)
	}
	return cache.Set(key, string(data))
}

// SetObjectCacheExp stores an object in the cache with a specific TTL.
// It requires a CacheServiceWithExp implementation; otherwise it falls back to Set.
func SetObjectCacheExp(cache CacheService, key string, value any, exp time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %w", err)
	}
	if c, ok := cache.(CacheServiceWithExp); ok {
		return c.SetWithExp(key, string(data), exp)
	}
	return cache.Set(key, string(data))
}

// CacheServiceWithExp extends CacheService with explicit expiration support.
type CacheServiceWithExp interface {
	CacheService
	SetWithExp(key string, value string, exp time.Duration) error
}

// --- DummyCache ---

// DummyCache is a no-operation cache that never stores anything.
// Get always returns ErrCacheMiss; Set and Delete are silent no-ops.
type DummyCache struct{}

// NewDummyCache creates a new DummyCache.
func NewDummyCache() CacheService {
	return &DummyCache{}
}

func (c *DummyCache) Get(key string) (string, error) {
	return "", ErrCacheMiss
}

func (c *DummyCache) Set(key string, value string) error {
	return nil
}

func (c *DummyCache) Delete(key string) error {
	return nil
}

// --- InMemCache ---

// cacheEntry holds a cached value along with its expiration time.
type cacheEntry struct {
	value     string
	expiresAt time.Time
}

// InMemCache is a thread-safe in-memory cache with lazy TTL expiration.
// Expired entries are removed on read access.
type InMemCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

// NewInMemCache creates a new in-memory cache.
func NewInMemCache() CacheService {
	return &InMemCache{
		entries: make(map[string]cacheEntry),
	}
}

// Get retrieves a value by key. If the entry has expired, it is lazily deleted
// and ErrCacheMiss is returned.
func (c *InMemCache) Get(key string) (string, error) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return "", ErrCacheMiss
	}

	if time.Now().After(entry.expiresAt) {
		// Lazy delete expired entry
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return "", ErrCacheMiss
	}

	return entry.value, nil
}

// Set stores a value with the default expiration (5 minutes).
func (c *InMemCache) Set(key string, value string) error {
	return c.SetWithExp(key, value, defaultCacheTTL)
}

// SetWithExp stores a value with a specific expiration duration.
func (c *InMemCache) SetWithExp(key string, value string, exp time.Duration) error {
	c.mu.Lock()
	c.entries[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(exp),
	}
	c.mu.Unlock()
	return nil
}

// Delete removes a value by key.
func (c *InMemCache) Delete(key string) error {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
	return nil
}
