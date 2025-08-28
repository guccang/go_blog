package blog

import (
	"sync"
)

// BlogManager manages multiple blog actors for different accounts
type BlogManager struct {
	actors map[string]*BlogActor // account -> BlogActor
	mu     sync.RWMutex
}

var blogManager *BlogManager
