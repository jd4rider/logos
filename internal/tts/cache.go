package tts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// CacheMeta is persisted alongside each cached PCM file.
type CacheMeta struct {
	Hash        string    `json:"hash"`
	Engine      string    `json:"engine"`
	VoiceID     string    `json:"voice_id"`
	VoiceName   string    `json:"voice_name"`
	Rate        int       `json:"rate"`
	SampleRate  int       `json:"sample_rate"`
	TextPreview string    `json:"text_preview"`    // first 80 chars of text
	TextLen     int       `json:"text_len"`        // full text length
	WordCount   int       `json:"word_count"`
	DurationMs  int64     `json:"duration_ms"`
	PCMBytes    int       `json:"pcm_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	LastAccess  time.Time `json:"last_access"`
	AccessCount int       `json:"access_count"`
}

// CacheStats summarises the current state of the audio cache.
type CacheStats struct {
	Entries      int
	TotalBytes   int64
	TotalHits    int
	OldestEntry  time.Time
	NewestEntry  time.Time
	MaxBytes     int64
}

// AudioCache stores synthesised PCM audio keyed by a hash of
// (engine, voiceID, rate, text). Entries are evicted LRU-style
// when the cache exceeds MaxBytes.
type AudioCache struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
}

const defaultMaxCacheBytes = 500 * 1024 * 1024 // 500 MB

// NewAudioCache creates (or re-opens) the cache in ~/.cache/logos/tts/.
func NewAudioCache() *AudioCache {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cache", "logos", "tts")
	_ = os.MkdirAll(dir, 0o755)
	return &AudioCache{dir: dir, maxBytes: defaultMaxCacheBytes}
}

// CacheKey returns the lookup key for the given synthesis parameters.
func CacheKey(engine, voiceID string, rate int, text string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%d\x00%s", engine, voiceID, rate, text)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (c *AudioCache) pcmPath(hash string) string  { return filepath.Join(c.dir, hash+".pcm") }
func (c *AudioCache) metaPath(hash string) string { return filepath.Join(c.dir, hash+".json") }

// Get retrieves cached PCM and metadata. Returns (nil, nil, false) on miss.
func (c *AudioCache) Get(hash string) ([]byte, *CacheMeta, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pcm, err := os.ReadFile(c.pcmPath(hash))
	if err != nil {
		return nil, nil, false
	}
	raw, err := os.ReadFile(c.metaPath(hash))
	if err != nil {
		return nil, nil, false
	}
	var meta CacheMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, nil, false
	}

	// Update access stats without blocking the caller.
	meta.LastAccess = time.Now()
	meta.AccessCount++
	if b, err2 := json.MarshalIndent(meta, "", "  "); err2 == nil {
		_ = os.WriteFile(c.metaPath(hash), b, 0o644)
	}

	return pcm, &meta, true
}

// Put stores PCM + metadata in the cache, then evicts oldest entries if
// total cache size exceeds MaxBytes.
func (c *AudioCache) Put(hash string, pcm []byte, meta CacheMeta) {
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = os.WriteFile(c.pcmPath(hash), pcm, 0o644)
	if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(c.metaPath(hash), b, 0o644)
	}

	c.evictLocked()
}

// evictLocked removes the oldest (LRU) entries until total PCM bytes ≤ MaxBytes.
// Caller must hold c.mu.
func (c *AudioCache) evictLocked() {
	entries, err := c.loadAllMetaLocked()
	if err != nil || len(entries) == 0 {
		return
	}

	var total int64
	for _, m := range entries {
		total += int64(m.PCMBytes)
	}
	if total <= c.maxBytes {
		return
	}

	// Sort by last access ascending (oldest first).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastAccess.Before(entries[j].LastAccess)
	})

	for _, m := range entries {
		if total <= c.maxBytes {
			break
		}
		_ = os.Remove(c.pcmPath(m.Hash))
		_ = os.Remove(c.metaPath(m.Hash))
		total -= int64(m.PCMBytes)
	}
}

// Stats returns a summary of the current cache state.
func (c *AudioCache) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, _ := c.loadAllMetaLocked()
	var stats CacheStats
	stats.MaxBytes = c.maxBytes
	stats.Entries = len(entries)
	for _, m := range entries {
		stats.TotalBytes += int64(m.PCMBytes)
		stats.TotalHits += m.AccessCount
		if stats.OldestEntry.IsZero() || m.CreatedAt.Before(stats.OldestEntry) {
			stats.OldestEntry = m.CreatedAt
		}
		if m.CreatedAt.After(stats.NewestEntry) {
			stats.NewestEntry = m.CreatedAt
		}
	}
	return stats
}

// List returns all cache entries sorted by last-access descending (most recent first).
func (c *AudioCache) List() []CacheMeta {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, _ := c.loadAllMetaLocked()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastAccess.After(entries[j].LastAccess)
	})
	return entries
}

// Clear removes all cached entries.
func (c *AudioCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := c.loadAllMetaLocked()
	if err != nil {
		return err
	}
	for _, m := range entries {
		_ = os.Remove(c.pcmPath(m.Hash))
		_ = os.Remove(c.metaPath(m.Hash))
	}
	return nil
}

// SetMaxBytes changes the cache size cap (in bytes) and triggers immediate eviction.
func (c *AudioCache) SetMaxBytes(n int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxBytes = n
	c.evictLocked()
}

// Dir returns the cache directory path.
func (c *AudioCache) Dir() string { return c.dir }

// loadAllMetaLocked reads every .json file in c.dir.
// Caller must hold c.mu.
func (c *AudioCache) loadAllMetaLocked() ([]CacheMeta, error) {
	glob, err := filepath.Glob(filepath.Join(c.dir, "*.json"))
	if err != nil {
		return nil, err
	}
	out := make([]CacheMeta, 0, len(glob))
	for _, path := range glob {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m CacheMeta
		if json.Unmarshal(raw, &m) == nil {
			out = append(out, m)
		}
	}
	return out, nil
}
