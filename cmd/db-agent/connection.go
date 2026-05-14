package main

import (
	"encoding/json"
	"fmt"
	"log"

	"agentbase"
	"db-agent/store"
	"uap"
)

// Connection manages the UAP gateway connection for db-agent.
type Connection struct {
	*agentbase.AgentBase

	cfg   *DBConfig
	store store.Store
}

// NewConnection creates a new Connection.
func NewConnection(cfg *DBConfig, agentID string) (*Connection, error) {
	st, err := store.NewStore(cfg.Driver, cfg.DSN, cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	baseCfg := &agentbase.Config{
		ServerURL:   cfg.ServerURL,
		AgentID:     agentID,
		AgentType:   "db",
		AgentName:   cfg.AgentName,
		Description: "数据库存储代理 — 提供 SQLite/MongoDB/Redis 数据持久化能力",
		AuthToken:   cfg.AuthToken,
		Capacity:    cfg.MaxConcurrent,
		Tools:       buildDBToolDefs(),
		Meta: map[string]any{
			"driver": cfg.Driver,
		},
	}

	c := &Connection{
		AgentBase: agentbase.NewAgentBase(baseCfg),
		cfg:       cfg,
		store:     st,
	}

	c.RegisterToolCallHandler(c.handleToolCall)

	return c, nil
}

// Stop gracefully stops the connection and closes the store.
func (c *Connection) Stop() {
	if c.store != nil {
		c.store.Close()
	}
	c.AgentBase.Stop()
}

// ========================= Tool Definitions =========================

func buildDBToolDefs() []uap.ToolDef {
	return []uap.ToolDef{
		{
			Name:        "db.Insert",
			Description: "向指定 collection 插入一条 JSON 记录，返回自动生成的记录 _id",
			Parameters: mustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "集合/表名"},
					"record":     map[string]any{"type": "object", "description": "要插入的 JSON 记录"},
				},
				"required": []string{"collection", "record"},
			}),
		},
		{
			Name:        "db.Find",
			Description: "查询 collection 中的记录，支持字段过滤、正则匹配、排序和分页",
			Parameters: mustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "集合/表名"},
					"filter":     map[string]any{"type": "object", "description": "字段等值过滤条件"},
					"regex":      map[string]any{"type": "object", "description": "字段正则匹配条件，如 {\"name\": \"^hello\"}"},
					"sort":       map[string]any{"type": "array", "description": "排序字段列表 [{\"field\": \"name\", \"desc\": false}]"},
					"limit":      map[string]any{"type": "integer", "description": "返回记录数上限，默认无限制"},
					"offset":     map[string]any{"type": "integer", "description": "分页偏移量，默认 0"},
				},
				"required": []string{"collection"},
			}),
		},
		{
			Name:        "db.Update",
			Description: "更新匹配过滤条件的记录，返回受影响行数",
			Parameters: mustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "集合/表名"},
					"filter":     map[string]any{"type": "object", "description": "字段等值过滤条件"},
					"regex":      map[string]any{"type": "object", "description": "字段正则匹配条件"},
					"updates":    map[string]any{"type": "object", "description": "要更新的字段和值"},
				},
				"required": []string{"collection", "updates"},
			}),
		},
		{
			Name:        "db.Delete",
			Description: "删除匹配过滤条件的记录，返回删除行数",
			Parameters: mustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "集合/表名"},
					"filter":     map[string]any{"type": "object", "description": "字段等值过滤条件"},
					"regex":      map[string]any{"type": "object", "description": "字段正则匹配条件"},
				},
				"required": []string{"collection"},
			}),
		},
		{
			Name:        "db.Count",
			Description: "统计匹配过滤条件的记录数",
			Parameters: mustMarshalJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"collection": map[string]any{"type": "string", "description": "集合/表名"},
					"filter":     map[string]any{"type": "object", "description": "字段等值过滤条件"},
					"regex":      map[string]any{"type": "object", "description": "字段正则匹配条件"},
				},
				"required": []string{"collection"},
			}),
		},
		{
			Name:        "db.ListCollections",
			Description: "列出数据库中所有的 collection/表",
			Parameters:  mustMarshalJSON(map[string]any{"type": "object", "properties": map[string]any{}}),
		},
	}
}

// ========================= Tool Call Handler =========================

func (c *Connection) handleToolCall(msg *uap.Message) {
	var payload uap.ToolCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[WARN] invalid tool_call payload: %v", err)
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     "invalid tool_call payload",
		})
		return
	}

	var args map[string]any
	if len(payload.Arguments) > 0 {
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			log.Printf("[WARN] invalid tool_call arguments: %v", err)
			c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
				RequestID: msg.ID,
				Success:   false,
				Error:     "invalid arguments: " + err.Error(),
			})
			return
		}
	} else {
		args = make(map[string]any)
	}

	log.Printf("[INFO] tool_call from=%s tool=%s", msg.From, payload.ToolName)

	var result string
	switch payload.ToolName {
	case "db.Insert":
		result = c.toolInsert(msg.ID, args)
	case "db.Find":
		result = c.toolFind(msg.ID, args)
	case "db.Update":
		result = c.toolUpdate(msg.ID, args)
	case "db.Delete":
		result = c.toolDelete(msg.ID, args)
	case "db.Count":
		result = c.toolCount(msg.ID, args)
	case "db.ListCollections":
		result = c.toolListCollections(msg.ID)
	default:
		c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
			RequestID: msg.ID,
			Success:   false,
			Error:     fmt.Sprintf("unknown tool: %s", payload.ToolName),
		})
		return
	}

	c.Client.SendTo(msg.From, uap.MsgToolResult, uap.ToolResultPayload{
		RequestID: msg.ID,
		Success:   true,
		Result:    result,
	})
}

// ========================= Tool Implementations =========================

func (c *Connection) toolInsert(msgID string, args map[string]any) string {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return buildError("collection is required")
	}

	record, ok := args["record"].(map[string]any)
	if !ok || record == nil {
		return buildError("record must be a non-null JSON object")
	}

	id, err := c.store.Insert(collection, store.Record(record))
	if err != nil {
		return buildError(err.Error())
	}

	tr := uap.BuildToolResult(msgID, map[string]any{"_id": id}, "inserted")
	return tr.Result
}

func (c *Connection) toolFind(msgID string, args map[string]any) string {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return buildError("collection is required")
	}

	q := parseQuery(args)
	result, err := c.store.Find(collection, q)
	if err != nil {
		return buildError(err.Error())
	}

	tr := uap.BuildToolResult(msgID, result, fmt.Sprintf("%d records", len(result.Data)))
	return tr.Result
}

func (c *Connection) toolUpdate(msgID string, args map[string]any) string {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return buildError("collection is required")
	}

	updates, ok := args["updates"].(map[string]any)
	if !ok || updates == nil {
		return buildError("updates must be a non-null JSON object")
	}

	q := parseQuery(args)
	affected, err := c.store.Update(collection, q, updates)
	if err != nil {
		return buildError(err.Error())
	}

	tr := uap.BuildToolResult(msgID, map[string]any{"affected": affected}, fmt.Sprintf("updated %d records", affected))
	return tr.Result
}

func (c *Connection) toolDelete(msgID string, args map[string]any) string {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return buildError("collection is required")
	}

	q := parseQuery(args)
	affected, err := c.store.Delete(collection, q)
	if err != nil {
		return buildError(err.Error())
	}

	tr := uap.BuildToolResult(msgID, map[string]any{"affected": affected}, fmt.Sprintf("deleted %d records", affected))
	return tr.Result
}

func (c *Connection) toolCount(msgID string, args map[string]any) string {
	collection, _ := args["collection"].(string)
	if collection == "" {
		return buildError("collection is required")
	}

	q := parseQuery(args)
	count, err := c.store.Count(collection, q)
	if err != nil {
		return buildError(err.Error())
	}

	tr := uap.BuildToolResult(msgID, map[string]any{"count": count}, fmt.Sprintf("%d records", count))
	return tr.Result
}

func (c *Connection) toolListCollections(msgID string) string {
	names, err := c.store.ListCollections()
	if err != nil {
		return buildError(err.Error())
	}

	tr := uap.BuildToolResult(msgID, map[string]any{"collections": names}, fmt.Sprintf("%d collections", len(names)))
	return tr.Result
}

// ========================= Helpers =========================

func parseQuery(args map[string]any) store.Query {
	q := store.Query{}

	if filter, ok := args["filter"].(map[string]any); ok {
		q.Filter = filter
	}
	if regex, ok := args["regex"].(map[string]any); ok {
		q.Regex = make(map[string]string)
		for k, v := range regex {
			if s, ok := v.(string); ok {
				q.Regex[k] = s
			}
		}
	}
	if sortRaw, ok := args["sort"].([]any); ok {
		for _, item := range sortRaw {
			if sm, ok := item.(map[string]any); ok {
				field, _ := sm["field"].(string)
				desc, _ := sm["desc"].(bool)
				if field != "" {
					q.Sort = append(q.Sort, store.SortField{Field: field, Desc: desc})
				}
			}
		}
	}
	if limit, ok := args["limit"].(float64); ok {
		q.Limit = int64(limit)
	}
	if offset, ok := args["offset"].(float64); ok {
		q.Offset = int64(offset)
	}

	return q
}

func buildError(errMsg string) string {
	return fmt.Sprintf(`{"success":false,"error":%q}`, errMsg)
}

func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}
