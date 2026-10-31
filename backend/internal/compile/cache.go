package compile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
)

// CacheEntry is one row of the pdf_cache table (in-memory until phase 05
// swaps in the sqlc-backed implementation).
type CacheEntry struct {
	Key       string
	PDFKey    string
	PageCount int
}

// PDFCache stores cache key → rendered PDF metadata. Phase 05 provides the
// sqlc-backed implementation against the pdf_cache table.
type PDFCache interface {
	Get(ctx context.Context, key string) (CacheEntry, bool, error)
	Put(ctx context.Context, entry CacheEntry) error
}

// ObjectStore stores blobs by key (R2 in production; see internal/storage,
// phase 04.8/05). Implementations must treat keys as opaque.
type ObjectStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
}

// CacheKey = sha256(tex + sorted (asset path, asset sha256) pairs + compiler).
// NUL separators keep concatenation unambiguous.
func CacheKey(tex string, assetHashes map[string]string, compiler string) string {
	paths := make([]string, 0, len(assetHashes))
	for p := range assetHashes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	h.Write([]byte(tex))
	for _, p := range paths {
		h.Write([]byte{0})
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(assetHashes[p]))
	}
	h.Write([]byte{0})
	h.Write([]byte(compiler))
	return hex.EncodeToString(h.Sum(nil))
}

// MemoryCache is an in-memory PDFCache for tests and for running before the
// phase 05 migration exists. Not shared across processes — a restart simply
// recompiles.
type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]CacheEntry
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{entries: make(map[string]CacheEntry)}
}

func (m *MemoryCache) Get(_ context.Context, key string) (CacheEntry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	return e, ok, nil
}

func (m *MemoryCache) Put(_ context.Context, entry CacheEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[entry.Key] = entry
	return nil
}

// MemoryStore is an in-memory ObjectStore for tests.
type MemoryStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: make(map[string][]byte)}
}

func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (m *MemoryStore) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored := make([]byte, len(data))
	copy(stored, data)
	m.objects[key] = stored
	return nil
}
