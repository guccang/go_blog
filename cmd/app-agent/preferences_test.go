package main

import (
	"path/filepath"
	"testing"
)

func TestAppPreferencesStoreDefaults(t *testing.T) {
	store := NewAppPreferencesStore(filepath.Join(t.TempDir(), "prefs.json"))
	got := store.Get("alice")
	if got.PreferredChatAgent != ChatAgentLLM {
		t.Fatalf("default preferred_chat_agent = %q, want %q", got.PreferredChatAgent, ChatAgentLLM)
	}
}

func TestAppPreferencesStoreSetAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	store := NewAppPreferencesStore(path)
	store.Set("alice", AppPreferences{PreferredChatAgent: "HERMES"})

	reloaded := NewAppPreferencesStore(path)
	got := reloaded.Get("alice")
	if got.PreferredChatAgent != ChatAgentHermes {
		t.Fatalf("reloaded preferred_chat_agent = %q, want %q", got.PreferredChatAgent, ChatAgentHermes)
	}
	if reloaded.Get("bob").PreferredChatAgent != ChatAgentLLM {
		t.Fatalf("unknown user should fall back to llm")
	}
}

func TestNormalizeAppPreferencesRejectsUnknownAgent(t *testing.T) {
	got := NormalizeAppPreferences(AppPreferences{PreferredChatAgent: "something-else"})
	if got.PreferredChatAgent != ChatAgentLLM {
		t.Fatalf("unknown agent should normalize to llm, got %q", got.PreferredChatAgent)
	}
}

func TestChatAgentForRouting(t *testing.T) {
	cfg := DefaultConfig()
	bridge := NewBridge(cfg)
	store := NewAppPreferencesStore(filepath.Join(t.TempDir(), "prefs.json"))
	bridge.SetPreferences(store)

	if got := bridge.chatAgentFor("alice"); got != cfg.LLMAgentID {
		t.Fatalf("default route = %q, want %q", got, cfg.LLMAgentID)
	}

	store.Set("alice", AppPreferences{PreferredChatAgent: ChatAgentHermes})
	if got := bridge.chatAgentFor("alice"); got != cfg.HermesAgentID {
		t.Fatalf("hermes route = %q, want %q", got, cfg.HermesAgentID)
	}
	if got := bridge.chatAgentFor("bob"); got != cfg.LLMAgentID {
		t.Fatalf("other user route = %q, want %q", got, cfg.LLMAgentID)
	}
}
