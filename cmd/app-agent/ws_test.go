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

func TestBroadcastGroupMessageExcludesHumanSender(t *testing.T) {
	bridge := NewBridge(DefaultConfig())
	bridge.groups.groups["g1"] = &appGroup{
		ID:           "g1",
		Owner:        "ztt",
		HumanMembers: map[string]bool{"ztt": true, "alice": true},
		RobotAccount: "robot-g1",
	}

	err := bridge.broadcastGroupMessage("g1", "ztt", "hello", "text", map[string]any{})
	if err != nil {
		t.Fatalf("broadcastGroupMessage returned error: %v", err)
	}

	if got := len(bridge.pendingByUser["ztt"]); got != 0 {
		t.Fatalf("expected sender ztt to receive no queued messages, got %d", got)
	}
	if got := len(bridge.pendingByUser["alice"]); got != 1 {
		t.Fatalf("expected alice to receive 1 queued message, got %d", got)
	}
}

func TestBroadcastGroupMessageFromRobotStillReachesAllHumans(t *testing.T) {
	bridge := NewBridge(DefaultConfig())
	bridge.groups.groups["g1"] = &appGroup{
		ID:           "g1",
		Owner:        "ztt",
		HumanMembers: map[string]bool{"ztt": true, "alice": true},
		RobotAccount: "robot-g1",
	}

	err := bridge.broadcastGroupMessage("g1", "robot-g1", "robot reply", "text", map[string]any{})
	if err != nil {
		t.Fatalf("broadcastGroupMessage returned error: %v", err)
	}

	if got := len(bridge.pendingByUser["ztt"]); got != 1 {
		t.Fatalf("expected ztt to receive 1 robot message, got %d", got)
	}
	if got := len(bridge.pendingByUser["alice"]); got != 1 {
		t.Fatalf("expected alice to receive 1 robot message, got %d", got)
	}
}
