package blog

import (
	"encoding/json"
	log "mylog"
	db "persistence"
	"sync"
)

// HookType describes an action that can later provide context to an AI feature.
// Bodies are never included in hook events.
type HookType string

const (
	HookBlogCreated  HookType = "blog.created"
	HookDiaryWritten HookType = "diary.written"
	HookDiaryDeleted HookType = "diary.deleted"
	HookPageOpened   HookType = "page.opened"
	HookFeatureUsed  HookType = "feature.used"

	// Reserved AI lifecycle events. They are intentionally defined before the
	// AI feature exists so consumers share one stable event vocabulary.
	HookAIAsked      HookType = "ai.asked"
	HookAIAnswered   HookType = "ai.answered"
	HookAIAccepted   HookType = "ai.accepted"
	HookAISummarized HookType = "ai.summarized"
)

type HookEvent struct {
	Account    string
	SessionID  string
	Type       HookType
	Feature    string
	ObjectType string
	ObjectID   string
	Title      string // Legacy-friendly display title; never contains body content.
	Query      string // Local search phrase, when the event is a search.
	Context    map[string]any
	Result     map[string]any
}

type HookHandler func(HookEvent)

var hookHandlers = struct {
	sync.RWMutex
	items []HookHandler
}{}

// SubscribeHook allows a future AI module to observe blog actions without
// adding dependencies back into HTTP handlers.
func SubscribeHook(handler HookHandler) {
	if handler == nil {
		return
	}
	hookHandlers.Lock()
	hookHandlers.items = append(hookHandlers.items, handler)
	hookHandlers.Unlock()
}

// EmitHook durably records a metadata-only event, then notifies subscribers.
func EmitHook(event HookEvent) {
	if event.Account == "" || event.Type == "" {
		return
	}
	if event.ObjectID == "" {
		event.ObjectID = event.Title
	}
	contextJSON := []byte("{}")
	if event.Context != nil {
		contextJSON, _ = json.Marshal(event.Context)
	}
	resultJSON := []byte("{}")
	if event.Result != nil {
		resultJSON, _ = json.Marshal(event.Result)
	}
	if err := db.RecordBlogHook(db.BlogHookEvent{Account: event.Account, SessionID: event.SessionID, EventType: string(event.Type), Feature: event.Feature, ObjectType: event.ObjectType, ObjectID: event.ObjectID, Title: event.Title, Query: event.Query, ContextJSON: string(contextJSON), ResultJSON: string(resultJSON)}); err != nil {
		log.ErrorF(log.ModuleBlog, "record blog hook failed: %v", err)
	}
	hookHandlers.RLock()
	handlers := append([]HookHandler(nil), hookHandlers.items...)
	hookHandlers.RUnlock()
	for _, handler := range handlers {
		go handler(event)
	}
}

func RecentHooks(account string, limit int) ([]db.BlogHookEvent, error) {
	return db.ListBlogHooks(account, limit)
}
