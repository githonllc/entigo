package entigo

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDummyCacheGet(t *testing.T) {
	cache := NewDummyCache()

	val, err := cache.Get("any_key")
	assert.ErrorIs(t, err, ErrCacheMiss, "DummyCache.Get should always return ErrCacheMiss")
	assert.Empty(t, val)
}

func TestDummyCacheSet(t *testing.T) {
	cache := NewDummyCache()

	err := cache.Set("key", "value")
	assert.NoError(t, err, "DummyCache.Set should not return error")

	// Verify that the value is not actually stored
	val, err := cache.Get("key")
	assert.ErrorIs(t, err, ErrCacheMiss)
	assert.Empty(t, val)
}

func TestDummyCacheDelete(t *testing.T) {
	cache := NewDummyCache()

	err := cache.Delete("key")
	assert.NoError(t, err, "DummyCache.Delete should not return error")
}

func TestInMemCacheSetGet(t *testing.T) {
	cache := NewInMemCache()

	err := cache.Set("greeting", "hello world")
	assert.NoError(t, err)

	val, err := cache.Get("greeting")
	assert.NoError(t, err)
	assert.Equal(t, "hello world", val)
}

func TestInMemCacheGetMiss(t *testing.T) {
	cache := NewInMemCache()

	val, err := cache.Get("nonexistent")
	assert.ErrorIs(t, err, ErrCacheMiss)
	assert.Empty(t, val)
}

func TestInMemCacheExpiration(t *testing.T) {
	cache := NewInMemCache()

	// Use the InMemCache directly to set with a very short TTL
	inMem := cache.(*InMemCache)
	err := inMem.SetWithExp("short_lived", "ephemeral", 10*time.Millisecond)
	assert.NoError(t, err)

	// Value should be available immediately
	val, err := cache.Get("short_lived")
	assert.NoError(t, err)
	assert.Equal(t, "ephemeral", val)

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Value should now be expired
	val, err = cache.Get("short_lived")
	assert.ErrorIs(t, err, ErrCacheMiss, "expired entry should return ErrCacheMiss")
	assert.Empty(t, val)
}

func TestInMemCacheDelete(t *testing.T) {
	cache := NewInMemCache()

	// Set a value
	err := cache.Set("to_delete", "some_value")
	assert.NoError(t, err)

	// Verify it exists
	val, err := cache.Get("to_delete")
	assert.NoError(t, err)
	assert.Equal(t, "some_value", val)

	// Delete it
	err = cache.Delete("to_delete")
	assert.NoError(t, err)

	// Verify it is gone
	val, err = cache.Get("to_delete")
	assert.ErrorIs(t, err, ErrCacheMiss)
	assert.Empty(t, val)
}

func TestInMemCacheDeleteNonexistent(t *testing.T) {
	cache := NewInMemCache()

	// Deleting a nonexistent key should not error
	err := cache.Delete("nonexistent")
	assert.NoError(t, err)
}

func TestInMemCacheOverwrite(t *testing.T) {
	cache := NewInMemCache()

	err := cache.Set("key", "first")
	assert.NoError(t, err)

	err = cache.Set("key", "second")
	assert.NoError(t, err)

	val, err := cache.Get("key")
	assert.NoError(t, err)
	assert.Equal(t, "second", val, "overwritten value should be returned")
}

func TestInMemCacheConcurrency(t *testing.T) {
	cache := NewInMemCache()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key"
			val := "value"
			_ = cache.Set(key, val)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _ = cache.Get("key")
		}(i)
	}

	// Concurrent deletes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = cache.Delete("key")
		}(i)
	}

	// This should not panic or race
	wg.Wait()
}

func TestGetObjectCache(t *testing.T) {
	cache := NewInMemCache()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	// Store an object
	err := SetObjectCache(cache, "user:1", &User{Name: "Alice", Age: 30})
	assert.NoError(t, err)

	// Retrieve and unmarshal
	var retrieved User
	err = GetObjectCache(cache, "user:1", &retrieved)
	assert.NoError(t, err)
	assert.Equal(t, "Alice", retrieved.Name)
	assert.Equal(t, 30, retrieved.Age)
}

func TestGetObjectCacheMiss(t *testing.T) {
	cache := NewInMemCache()

	var result string
	err := GetObjectCache(cache, "nonexistent", &result)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSetObjectCacheExp(t *testing.T) {
	cache := NewInMemCache()

	err := SetObjectCacheExp(cache, "temp", "data", 10*time.Millisecond)
	assert.NoError(t, err)

	// Should be available immediately
	var result string
	err = GetObjectCache(cache, "temp", &result)
	assert.NoError(t, err)
	assert.Equal(t, "data", result)

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	err = GetObjectCache(cache, "temp", &result)
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSetObjectCacheExpFallback(t *testing.T) {
	// DummyCache does not implement CacheServiceWithExp, so SetObjectCacheExp
	// should fall back to Set (which is a no-op for DummyCache)
	cache := NewDummyCache()

	err := SetObjectCacheExp(cache, "key", "value", time.Minute)
	assert.NoError(t, err)
}

func TestInMemCacheImplementsCacheServiceWithExp(t *testing.T) {
	cache := NewInMemCache()

	// Verify InMemCache implements CacheServiceWithExp
	_, ok := cache.(CacheServiceWithExp)
	assert.True(t, ok, "InMemCache should implement CacheServiceWithExp")
}
