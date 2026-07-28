package http

import (
	"auth"
	"blog"
	h "net/http"
)

// emitUsageHook is the single HTTP-to-hook bridge. It records request context
// but intentionally never passes a blog body into the event stream.
func emitUsageHook(r *h.Request, account string, eventType blog.HookType, feature, objectType, objectID, title, query string, context, result map[string]any) {
	blog.EmitHook(blog.HookEvent{
		Account:    account,
		SessionID:  auth.GetSessionFromRequest(r),
		Type:       eventType,
		Feature:    feature,
		ObjectType: objectType,
		ObjectID:   objectID,
		Title:      title,
		Query:      query,
		Context:    context,
		Result:     result,
	})
}
