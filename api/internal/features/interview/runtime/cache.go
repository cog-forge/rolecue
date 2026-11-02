package runtime

import (
	"container/list"
	"sync"

	"github.com/cog-forge/rolecue/api/internal/pkg/tts"
)

// maxCachedAudioBytes bounds the memory of the synthesis cache whatever the entry count.
const maxCachedAudioBytes = 64 << 20

// synthesisCache keeps recent successful syntheses per (provider voice, text). The same core question
// is asked of many Candidates of one posting, and a reconnect repeats the current turn, so caching saves
// provider quota and latency. It is a bounded LRU; the results are never modified after they are stored.
type synthesisCache struct {
	mu      sync.Mutex
	limit   int
	bytes   int
	order   *list.List
	entries map[cacheKey]*list.Element
}

type cacheKey struct {
	voice string
	text  string
}

type cacheEntry struct {
	key    cacheKey
	result tts.Result
}

// newSynthesisCache returns a cache of at most limit entries, or nil (no caching) when limit is zero.
func newSynthesisCache(limit int) *synthesisCache {
	if limit <= 0 {
		return nil
	}
	return &synthesisCache{limit: limit, order: list.New(), entries: make(map[cacheKey]*list.Element)}
}

func (c *synthesisCache) get(voice, text string) (tts.Result, bool) {
	if c == nil {
		return tts.Result{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.entries[cacheKey{voice, text}]
	if !ok {
		return tts.Result{}, false
	}
	c.order.MoveToFront(element)
	return element.Value.(*cacheEntry).result, true
}

func (c *synthesisCache) put(voice, text string, result tts.Result) {
	if c == nil || len(result.Audio) > maxCachedAudioBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey{voice, text}
	if element, ok := c.entries[key]; ok {
		c.order.MoveToFront(element)
		return
	}
	c.entries[key] = c.order.PushFront(&cacheEntry{key: key, result: result})
	c.bytes += len(result.Audio)
	for c.order.Len() > c.limit || c.bytes > maxCachedAudioBytes {
		oldest := c.order.Back()
		entry := oldest.Value.(*cacheEntry)
		c.order.Remove(oldest)
		delete(c.entries, entry.key)
		c.bytes -= len(entry.result.Audio)
	}
}
