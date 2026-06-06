# App-Agent TUI

用于快速测试 `app-agent` 的终端客户端，覆盖：

- 账号密码登录
- WebSocket 连接、消息接收与 ACK
- HTTP 消息发送
- 推送消息类型与最近一条消息 `meta` 查看
- 全部 HTTP/WebSocket 收发消息写入 UTF-8 JSONL 调试日志

## 运行

```bash
cd cmd/app-agent-tui
go run .
```

也可以通过参数预填连接信息：

```bash
go run . \
  -url http://blog.guccang.cn:8883 \
  -user ztt \
  -password your-password \
  -token 123456 \
  -log logs/app-agent-tui.jsonl
```

默认地址为 `http://blog.guccang.cn:8883`，默认用户为 `ztt`，默认接收令牌为 `123456`。
`-token` 对应 `app-agent.json` 中可选的 `receive_token`。

## 消息日志

全部 HTTP 请求/响应、WebSocket 推送、ACK、连接和错误事件默认追加写入
`logs/app-agent-tui.jsonl`。每行是一条 UTF-8 JSON 记录，密码、接收令牌、
session/access/refresh/delegation token 会自动脱敏。可使用 `-log` 指定其他路径。

## 快捷键与命令

- 登录页：`Tab`、方向键切换字段，最后一项按 `Enter` 连接。
- 聊天页：`Enter` 发送，`PgUp`/`PgDn` 滚动，`Esc` 返回登录页。
- `/clear`：清空消息。
- `/meta`：显示最近一条带元数据消息的 `meta`。
- `/reconnect`：重新登录并连接。
- `/quit`：退出。
