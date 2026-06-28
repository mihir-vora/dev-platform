package auth

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var errKVNotFound = errors.New("key not found")

type kvStore interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key string) error
}

type redisStore struct {
	client *redis.Client
}

func (s *redisStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *redisStore) Get(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", errKVNotFound
	}
	return val, err
}

func (s *redisStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

type memoryStore struct {
	mu    sync.Mutex
	items map[string]memEntry
}

type memEntry struct {
	value   string
	expires time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: make(map[string]memEntry)}
}

func (m *memoryStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = memEntry{value: value, expires: time.Now().Add(ttl)}
	return nil
}

func (m *memoryStore) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.items[key]
	if !ok || time.Now().After(entry.expires) {
		delete(m.items, key)
		return "", errKVNotFound
	}
	return entry.value, nil
}

func (m *memoryStore) Del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

// NewKVStore uses Redis when REDIS_URL is set and reachable; otherwise in-memory.
func NewKVStore(ctx context.Context, redisURL string) kvStore {
	if redisURL == "" {
		slog.Warn("REDIS_URL not set; using in-memory sessions (not suitable for multiple API instances)")
		return newMemoryStore()
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Warn("invalid REDIS_URL; using in-memory sessions", "error", err)
		return newMemoryStore()
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("redis unavailable; using in-memory sessions", "error", err)
		return newMemoryStore()
	}

	slog.Info("redis connected; using redis for sessions")
	return &redisStore{client: client}
}
