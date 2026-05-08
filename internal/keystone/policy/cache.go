package policy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type CacheEntry struct {
	Allowed   bool
	ExpiresAt time.Time
}

type Cache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
	go c.cleanupLoop()
	return c
}

func (c *Cache) Get(rule string, target, credentials map[string]interface{}) (bool, bool) {
	key := c.generateKey(rule, target, credentials)
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return false, false
	}
	return entry.Allowed, true
}

func (c *Cache) Set(rule string, target, credentials map[string]interface{}, allowed bool) {
	key := c.generateKey(rule, target, credentials)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = &CacheEntry{Allowed: allowed, ExpiresAt: time.Now().Add(c.ttl)}
}

func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
}

func (c *Cache) generateKey(rule string, target, credentials map[string]interface{}) string {
	h := sha256.New()
	h.Write([]byte(rule))
	h.Write(deterministicJSON(target))
	h.Write(deterministicJSON(credentials))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func deterministicJSON(m map[string]interface{}) []byte {
	if len(m) == 0 {
		return []byte("{}")
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	buf.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(`"`)
		buf.WriteString(k)
		buf.WriteString(`":`)

		switch val := m[k].(type) {
		case string:
			buf.WriteString(`"`)
			buf.WriteString(val)
			buf.WriteString(`"`)
		case []string:
			sorted := make([]string, len(val))
			copy(sorted, val)
			sort.Strings(sorted)
			jsonVal, _ := json.Marshal(sorted)
			buf.Write(jsonVal)
		default:
			jsonVal, _ := json.Marshal(m[k])
			buf.Write(jsonVal)
		}
	}
	buf.WriteString("}")
	return []byte(buf.String())
}

func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, entry := range c.entries {
			if now.After(entry.ExpiresAt) {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}
