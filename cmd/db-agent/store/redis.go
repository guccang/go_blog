package store

import (
	"fmt"
)

// redisStore implements Store using Redis.
// NOTE: This is a stub. Full implementation requires "github.com/redis/go-redis/v9".
type redisStore struct {
	addr string
}

// NewRedisStore creates a new Redis-backed Store.
func NewRedisStore(addr string) (Store, error) {
	if addr == "" {
		addr = "localhost:6379"
	}
	return nil, fmt.Errorf("redis driver not yet implemented (addr=%s)", addr)
}

// Stub methods to satisfy the Store interface (never called since New returns error).
func (s *redisStore) Insert(collection string, record Record) (string, error) {
	return "", fmt.Errorf("redis: not implemented")
}

func (s *redisStore) Find(collection string, query Query) (*QueryResult, error) {
	return nil, fmt.Errorf("redis: not implemented")
}

func (s *redisStore) Update(collection string, query Query, updates map[string]any) (int64, error) {
	return 0, fmt.Errorf("redis: not implemented")
}

func (s *redisStore) Delete(collection string, query Query) (int64, error) {
	return 0, fmt.Errorf("redis: not implemented")
}

func (s *redisStore) Count(collection string, query Query) (int64, error) {
	return 0, fmt.Errorf("redis: not implemented")
}

func (s *redisStore) ListCollections() ([]string, error) {
	return nil, fmt.Errorf("redis: not implemented")
}

func (s *redisStore) Close() error {
	return nil
}
