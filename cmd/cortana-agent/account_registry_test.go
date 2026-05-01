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
