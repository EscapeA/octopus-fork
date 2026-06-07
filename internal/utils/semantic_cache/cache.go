package semantic_cache

import (
	"math"
	"sync"
	"time"
)

// CacheEntry holds a cached request/response pair with its embedding.
type CacheEntry struct {
	Namespace    string
	RequestKey   string
	ResponseJSON []byte
	Embedding    []float64
	CreatedAt    time.Time
	LastAccessAt time.Time
	HitCount     int64
}

// SemanticCache is an in-memory vector store with cosine similarity lookup.
type SemanticCache struct {
	mu         sync.RWMutex
	entries    []CacheEntry
	maxEntries int
	threshold  float64
	ttl        time.Duration
	hits       int64
	misses     int64
}

// globalCacheMu protects the globalCache pointer itself from concurrent read/write.
// The individual cache operations use their own internal mutex (SemanticCache.mu).
var globalCacheMu sync.RWMutex
var globalCache *SemanticCache

type RuntimeConfig struct {
	Enabled          bool
	MaxEntries       int
	Threshold        float64
	TTL              time.Duration
	EmbeddingBaseURL string
	EmbeddingAPIKey  string
	EmbeddingModel   string
	EmbeddingTimeout time.Duration
}

// Init creates or reconfigures the global semantic cache.
func Init(maxEntries int, threshold float64, ttlSec int) {
	if maxEntries <= 0 {
		globalCacheMu.Lock()
		globalCache = nil
		globalCacheMu.Unlock()
		return
	}
	globalCacheMu.Lock()
	defer globalCacheMu.Unlock()
	if globalCache == nil || globalCache.maxEntries != maxEntries || globalCache.threshold != threshold || globalCache.ttl != time.Duration(ttlSec)*time.Second {
		globalCache = &SemanticCache{
			entries:    make([]CacheEntry, 0, maxEntries),
			maxEntries: maxEntries,
			threshold:  threshold,
			ttl:        time.Duration(ttlSec) * time.Second,
		}
	}
}

// ApplyRuntimeConfig creates or reconfigures the global semantic cache from runtime settings.
// When the cache already exists and the size/threshold/TTL parameters are unchanged,
// the existing cache is reused so stored entries are not discarded.
func ApplyRuntimeConfig(cfg RuntimeConfig) {
	if !cfg.Enabled || cfg.MaxEntries <= 0 {
		Reset()
		return
	}

	ttl := cfg.TTL
	globalCacheMu.Lock()
	defer globalCacheMu.Unlock()
	if globalCache != nil &&
		globalCache.maxEntries == cfg.MaxEntries &&
		globalCache.threshold == cfg.Threshold &&
		globalCache.ttl == ttl {
		return
	}

	globalCache = &SemanticCache{
		entries:    make([]CacheEntry, 0, cfg.MaxEntries),
		maxEntries: cfg.MaxEntries,
		threshold:  cfg.Threshold,
		ttl:        ttl,
	}
}

// Reset clears the cache and runtime configuration.
func Reset() {
	globalCacheMu.Lock()
	globalCache = nil
	globalCacheMu.Unlock()
}

// Lookup finds the best matching cache entry for the given embedding.
// Returns the response JSON and true if a match above threshold is found.
func Lookup(namespace string, embedding []float64) (responseJSON []byte, found bool) {
	globalCacheMu.RLock()
	cache := globalCache
	globalCacheMu.RUnlock()
	if cache == nil {
		return nil, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.pruneExpiredLocked()

	if len(cache.entries) == 0 {
		cache.misses++
		return nil, false
	}

	bestIdx := -1
	bestSim := -1.0
	for i, entry := range cache.entries {
		if entry.Namespace != namespace {
			continue
		}
		sim := cosineSimilarity(embedding, entry.Embedding)
		if sim > bestSim {
			bestSim = sim
			bestIdx = i
		}
	}

	if bestIdx >= 0 && bestSim >= cache.threshold {
		cache.entries[bestIdx].HitCount++
		cache.entries[bestIdx].LastAccessAt = time.Now()
		cache.hits++
		return append([]byte(nil), cache.entries[bestIdx].ResponseJSON...), true
	}

	cache.misses++
	return nil, false
}

// Store adds a new entry to the cache. If the cache is full, the oldest entry is evicted.
func Store(namespace, requestKey string, responseJSON []byte, embedding []float64) {
	globalCacheMu.RLock()
	cache := globalCache
	globalCacheMu.RUnlock()
	if cache == nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry := CacheEntry{
		Namespace:    namespace,
		RequestKey:   requestKey,
		ResponseJSON: append([]byte(nil), responseJSON...),
		Embedding:    cloneEmbedding(embedding),
		CreatedAt:    time.Now(),
		LastAccessAt: time.Now(),
	}

	if len(cache.entries) >= cache.maxEntries {
		// Evict the oldest entry
		oldestIdx := 0
		for i, e := range cache.entries {
			if e.LastAccessAt.Before(cache.entries[oldestIdx].LastAccessAt) {
				oldestIdx = i
			}
		}
		cache.entries[oldestIdx] = entry
	} else {
		cache.entries = append(cache.entries, entry)
	}
}

// Stats returns hit/miss counts and current cache size.
func Stats() (hits, misses int64, size int) {
	globalCacheMu.RLock()
	cache := globalCache
	globalCacheMu.RUnlock()
	if cache == nil {
		return 0, 0, 0
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.pruneExpiredLocked()
	return cache.hits, cache.misses, len(cache.entries)
}

// Clear empties the cache.
func Clear() {
	globalCacheMu.RLock()
	cache := globalCache
	globalCacheMu.RUnlock()
	if cache == nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.entries = make([]CacheEntry, 0, cache.maxEntries)
	cache.hits = 0
	cache.misses = 0
}

// Enabled returns true if the semantic cache is initialized and active.
func Enabled() bool {
	globalCacheMu.RLock()
	cache := globalCache
	globalCacheMu.RUnlock()
	return cache != nil
}

func (sc *SemanticCache) pruneExpiredLocked() {
	if sc.ttl <= 0 {
		return
	}
	now := time.Now()
	n := 0
	for _, entry := range sc.entries {
		if now.Sub(entry.LastAccessAt) < sc.ttl {
			sc.entries[n] = entry
			n++
		}
	}
	sc.entries = sc.entries[:n]
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func cloneEmbedding(src []float64) []float64 {
	dst := make([]float64, len(src))
	copy(dst, src)
	return dst
}
