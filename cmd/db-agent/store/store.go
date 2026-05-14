package store

import (
	"fmt"
)

// Record represents a database record as a JSON-compatible map.
type Record map[string]any

// SortField defines sort order for queries.
type SortField struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc"`
}

// Query represents a database query with optional filters, sorting, and pagination.
type Query struct {
	Filter map[string]any    `json:"filter"` // Field equality filter
	Sort   []SortField       `json:"sort"`   // Sort specification
	Limit  int64             `json:"limit"`  // Max results (0 = unlimited)
	Offset int64             `json:"offset"` // Pagination offset
	Regex  map[string]string `json:"regex"`  // Field → regex pattern
}

// QueryResult holds query results with total count.
type QueryResult struct {
	Data  []Record `json:"data"`
	Total int64    `json:"total"`
}

// Store is the database abstraction interface.
type Store interface {
	Insert(collection string, record Record) (string, error)
	Find(collection string, query Query) (*QueryResult, error)
	Update(collection string, query Query, updates map[string]any) (int64, error)
	Delete(collection string, query Query) (int64, error)
	Count(collection string, query Query) (int64, error)
	ListCollections() ([]string, error)
	Close() error
}

// NewStore creates a Store based on the driver type.
func NewStore(driver, dsn, dataDir string) (Store, error) {
	switch driver {
	case "sqlite", "":
		return NewSQLiteStore(dsn, dataDir)
	case "mongodb":
		return NewMongoDBStore(dsn)
	case "redis":
		return NewRedisStore(dsn)
	default:
		return nil, fmt.Errorf("unsupported driver: %q (available: sqlite, mongodb, redis)", driver)
	}
}
