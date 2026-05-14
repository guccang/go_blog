package store

import (
	"fmt"
)

// mongoDBStore implements Store using MongoDB.
// NOTE: This is a stub. Full implementation requires "go.mongodb.org/mongo-driver".
type mongoDBStore struct {
	uri string
}

// NewMongoDBStore creates a new MongoDB-backed Store.
func NewMongoDBStore(uri string) (Store, error) {
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	return nil, fmt.Errorf("mongodb driver not yet implemented (dsn=%s)", uri)
}

// Stub methods to satisfy the Store interface (never called since New returns error).
func (s *mongoDBStore) Insert(collection string, record Record) (string, error) {
	return "", fmt.Errorf("mongodb: not implemented")
}

func (s *mongoDBStore) Find(collection string, query Query) (*QueryResult, error) {
	return nil, fmt.Errorf("mongodb: not implemented")
}

func (s *mongoDBStore) Update(collection string, query Query, updates map[string]any) (int64, error) {
	return 0, fmt.Errorf("mongodb: not implemented")
}

func (s *mongoDBStore) Delete(collection string, query Query) (int64, error) {
	return 0, fmt.Errorf("mongodb: not implemented")
}

func (s *mongoDBStore) Count(collection string, query Query) (int64, error) {
	return 0, fmt.Errorf("mongodb: not implemented")
}

func (s *mongoDBStore) ListCollections() ([]string, error) {
	return nil, fmt.Errorf("mongodb: not implemented")
}

func (s *mongoDBStore) Close() error {
	return nil
}
