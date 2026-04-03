package main

import "testing"

func TestRegisterClientAllowsMultipleConnectionsPerUser(t *testing.T) {
	bridge := NewBridge(DefaultConfig())

	client1 := &appClientConn{userID: "demo-user"}
	client2 := &appClientConn{userID: "demo-user"}

	bridge.registerClient(client1)
	bridge.registerClient(client2)

	if got := bridge.OnlineClientCount(); got != 2 {
		t.Fatalf("expected 2 online clients, got %d", got)
	}

	userClients := bridge.clients["demo-user"]
	if len(userClients) != 2 {
		t.Fatalf("expected 2 stored connections for demo-user, got %d", len(userClients))
	}

	bridge.unregisterClient(client1)
	if got := bridge.OnlineClientCount(); got != 1 {
		t.Fatalf("expected 1 online client after removing one connection, got %d", got)
	}

	bridge.unregisterClient(client2)
	if got := bridge.OnlineClientCount(); got != 0 {
		t.Fatalf("expected 0 online clients after removing all connections, got %d", got)
	}
}
