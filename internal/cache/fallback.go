package cache

import (
	"sync"
	"time"
)

type fallbackEntry struct {
	data      []byte
	expiresAt time.Time
}

type FallbackCache struct {
	mu      sync.RWMutex
	entries map[string]fallbackEntry
	maxSize int
}

func NewFallbackCache(maxSize int) *FallbackCache {
	fc := &FallbackCache{
		entries: make(map[string]fallbackEntry),
		maxSize: maxSize,
	}
	go fc.cleanup()
	return fc
}

func (fc *FallbackCache) Get(key string) ([]byte, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	entry, ok := fc.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.data, true
}

func (fc *FallbackCache) Set(key string, data []byte, ttl time.Duration) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if len(fc.entries) >= fc.maxSize {
		fc.evictOldest()
	}

	fc.entries[key] = fallbackEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}

func (fc *FallbackCache) Delete(key string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	delete(fc.entries, key)
}

func (fc *FallbackCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range fc.entries {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(fc.entries, oldestKey)
	}
}

func (fc *FallbackCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		fc.mu.Lock()
		now := time.Now()
		for key, entry := range fc.entries {
			if now.After(entry.expiresAt) {
				delete(fc.entries, key)
			}
		}
		fc.mu.Unlock()
	}
}
