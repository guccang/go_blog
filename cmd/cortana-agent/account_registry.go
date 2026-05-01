package main

import (
	"sort"
	"strings"
	"sync"
)

// AccountRegistry 管理当前已注册的账号集合。
type AccountRegistry struct {
	mu       sync.RWMutex
	accounts map[string]struct{}
}

func NewAccountRegistry() *AccountRegistry {
	return &AccountRegistry{
		accounts: make(map[string]struct{}),
	}
}

func (r *AccountRegistry) RegisterAccount(account string) bool {
	account = strings.TrimSpace(account)
	if account == "" {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, existed := r.accounts[account]
	r.accounts[account] = struct{}{}
	return !existed
}

func (r *AccountRegistry) UnregisterAccount(account string) bool {
	account = strings.TrimSpace(account)
	if account == "" {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, existed := r.accounts[account]
	delete(r.accounts, account)
	return existed
}

func (r *AccountRegistry) HasAccount(account string) bool {
	account = strings.TrimSpace(account)
	if account == "" {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.accounts[account]
	return ok
}

func (r *AccountRegistry) ListAccounts() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	accounts := make([]string, 0, len(r.accounts))
	for account := range r.accounts {
		accounts = append(accounts, account)
	}
	sort.Strings(accounts)
	return accounts
}
