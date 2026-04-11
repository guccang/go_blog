package main

import (
	"context"
	"testing"
)

func TestEnsureAuthenticatedContextUsesAccountFallback(t *testing.T) {
	ctx := ensureAuthenticatedContext(context.Background(), "wechat-user")
	if got := GetAuthenticatedUser(ctx); got != "wechat-user" {
		t.Fatalf("expected authenticated user from account fallback, got %q", got)
	}
}

func TestEnsureAuthenticatedContextPreservesExistingUser(t *testing.T) {
	base := WithAuthenticatedUser(context.Background(), "explicit-user")
	ctx := ensureAuthenticatedContext(base, "wechat-user")
	if got := GetAuthenticatedUser(ctx); got != "explicit-user" {
		t.Fatalf("expected existing authenticated user to win, got %q", got)
	}
}

func TestEnsureAuthenticatedContextAllowsEmptyAccount(t *testing.T) {
	ctx := ensureAuthenticatedContext(context.Background(), "")
	if got := GetAuthenticatedUser(ctx); got != "" {
		t.Fatalf("expected empty authenticated user, got %q", got)
	}
}
