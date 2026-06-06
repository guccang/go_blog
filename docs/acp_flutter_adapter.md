# ACP Agent 与 Flutter 适配设计

## 问题

旧链路把 Flutter 表单拼成 `/cg` 文本命令，随后又把 ACP 结构化事件拼成累计文本返回。
这会让客户端依赖命令语法，并迫使客户端通过文本前缀和增量算法恢复会话事件。

## 当前适配边界

Flutter 只依赖 app-agent 提供的版本化接口：

```text
POST /api/app/codegen/actions
```

请求使用 `CodegenActionRequest`，明确表达 `start`、`debug`、`deploy`、项目、工具、配置、
续聊、自动发布和调试包等字段。app-agent 负责校验请求，并在内部适配当前 cmd-agent 的
`/cg` 命令协议。旧 Flutter 客户端仍可继续发送 `/cg` 命令。

ACP 流式消息继续保留累计文本作为兼容展示，同时在 WebSocket `meta.codegen_event` 中携带
原始事件类型、文本、工具名和完成状态。新 UI 应优先读取结构化事件。

## 目标链路

```text
Flutter CodegenAction
  -> app-agent Codegen API
  -> cmd-agent structured action
  -> acp-agent tool call

acp-agent session event
  -> app-agent durable event stream
  -> Flutter task reducer
  -> task timeline UI
```

## 后续迁移

1. cmd-agent 增加结构化 action 消息处理，app-agent 删除内部 `/cg` 命令转换。
2. 下行事件增加稳定的 `event_id`、`sequence` 和 `occurred_at`，按会话持久化并支持断线补拉。
3. Flutter 使用独立 task reducer 管理 `queued/running/waiting/completed/failed/cancelled` 状态。
4. 权限确认、取消、续聊和模式切换改为显式 action，不再依赖聊天文本。
5. 所有活跃客户端迁移后，删除累计文本和 `/cg` 兼容路径。
