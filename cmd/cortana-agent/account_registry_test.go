package main

import "testing"

func TestAccountRegistryOperations(t *testing.T) {
	registry := NewAccountRegistry()

	if added := registry.RegisterAccount("ztt"); !added {
		t.Fatalf("expected first register to add account")
	}
	if added := registry.RegisterAccount("alice"); !added {
		t.Fatalf("expected second register to add account")
	}
	if added := registry.RegisterAccount("ztt"); added {
		t.Fatalf("expected duplicate register to be idempotent")
	}
	if !registry.HasAccount("ztt") || !registry.HasAccount("alice") {
		t.Fatalf("expected accounts to be present")
	}

	accounts := registry.ListAccounts()
	if len(accounts) != 2 || accounts[0] != "alice" || accounts[1] != "ztt" {
		t.Fatalf("unexpected accounts: %#v", accounts)
	}

	if removed := registry.UnregisterAccount("missing"); removed {
		t.Fatalf("expected missing unregister to be idempotent")
	}
	if removed := registry.UnregisterAccount("ztt"); !removed {
		t.Fatalf("expected unregister to remove existing account")
	}
	if registry.HasAccount("ztt") {
		t.Fatalf("expected account to be removed")
	}
}

func TestAccountRegistrySyncUserSession(t *testing.T) {
	registry := NewAccountRegistry()
	session := registry.SyncUserSession(CortanaSyncUserPayload{
		Account:    "alice",
		Registered: true,
		Online:     true,
		Settings: CortanaUserSettings{
			Enabled:         true,
			AllowFullAccess: true,
			AutoPlay:        false,
			ProactiveMode:   "high",
			UpdatedAt:       123,
		},
	})
	if session == nil {
		t.Fatalf("expected session")
	}
	if !session.Online || !session.AllowFullAccess || session.AutoPlay {
		t.Fatalf("unexpected session: %#v", session)
	}
}

func TestAppendInteractionKeepsLatestEntries(t *testing.T) {
	registry := NewAccountRegistry()
	registry.SyncUserSession(CortanaSyncUserPayload{
		Account:    "demo",
		Registered: true,
		Online:     true,
		Settings:   defaultCortanaUserSettings(),
	})

	for i := 0; i < 10; i++ {
		registry.AppendInteraction("demo", map[string]any{
			"summary": i,
		})
	}

	session := registry.GetSession("demo")
	if session == nil {
		t.Fatalf("expected session")
	}
	if got := len(session.RecentInteractions); got != 8 {
		t.Fatalf("expected 8 recent interactions, got %d", got)
	}
	if session.RecentInteractions[0]["summary"] != 2 {
		t.Fatalf("expected oldest retained interaction to be 2, got %#v", session.RecentInteractions[0]["summary"])
	}
	if session.RecentInteractions[7]["summary"] != 9 {
		t.Fatalf("expected latest retained interaction to be 9, got %#v", session.RecentInteractions[7]["summary"])
	}
}
