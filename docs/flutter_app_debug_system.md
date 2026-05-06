# Flutter App Debug 系统设计

## 背景

当前 `acp-agent` 可以启动 `codex` 和 `claudecode` 处理代码任务，但 Flutter App 问题定位仍依赖人工描述、局部日志和临时命令。仓库里已有三类基础能力：

- `cmd/acp-agent`：提供 `AcpListProjects`、`AcpStartSession`、`AcpStopSession`，负责启动 Codex/ClaudeCode 编码会话。
- `cmd/app-agent`：提供 `/api/app/logs/sources`、`/api/app/logs/files`、`/api/app/logs/content`，可以读取配置里的服务端日志源。
- `cmd/flutter-client-for-appagent/flutter_client_for_appagent`：Flutter 侧已有 `flutterClientLogs` 和 `addFlutterClientLog`，但只保存在进程内，缺少结构化导出和与代码会话的绑定。

目标是设计一个面向 ACP 编码智能体的 Debug 系统：用户报告 Flutter App 问题后，Codex/ClaudeCode 能快速拿到上下文、复现路径、日志、环境、最近改动和验证命令，从而定位并修复问题。

## 设计目标

1. 一键生成 Debug Bundle，减少“先问你要日志”的来回沟通。
2. Bundle 可被 Codex/ClaudeCode 直接消费，包含机器可读 JSON 和人类可读摘要。
3. 支持 Flutter Web、Android、Windows、macOS 的差异化诊断，但 MVP 先覆盖当前仓库最常见的本地 Flutter 客户端和 `app-agent`。
4. 调试链路只做分析、静态检查和必要单元测试；Flutter 客户端验证不得执行 APK 打包命令。
5. 默认保护敏感信息，输出前做 token、密码、cookie、Authorization 等字段脱敏。

## 总体架构

```
用户问题
  |
  v
app-agent / Flutter App
  |  采集客户端日志、设备状态、最近交互、服务端日志
  v
Debug Collector
  |  生成 debug bundle
  v
acp-agent Debug Tools
  |  注入 bundle 路径和修复约束
  v
Codex / ClaudeCode
  |  读取 bundle -> 定位 -> 修改 -> 运行允许的验证命令
  v
Debug Report
```

系统拆成四层：

- 客户端探针：Flutter 内收集运行时事件、网络请求摘要、WebSocket 状态、页面状态、异常堆栈、用户最近操作。
- 服务端采集：`app-agent` 聚合自身日志、`blog-agent`/`gateway`/`acp-agent` 日志、codegen stream 事件。
- Bundle 生成：输出固定目录结构，包含 `manifest.json`、`summary.md`、日志片段、截图、复现脚本、验证计划。
- ACP 工具层：给 `acp-agent` 增加 Debug 专用工具，使 LLM 不需要自己猜文件位置和命令。

## Debug Bundle 目录结构

建议放在项目内的临时运行目录，不提交：

```
.debug/flutter/<debug_id>/
  manifest.json
  summary.md
  issue.md
  environment.json
  app_state.json
  timeline.jsonl
  logs/
    flutter_client.log
    app_agent.log
    blog_agent.log
    acp_agent.log
  screenshots/
    current.png
  traces/
    network.jsonl
    websocket.jsonl
  repo/
    git_status.txt
    changed_files.txt
    dependency_snapshot.txt
  validation/
    allowed_commands.md
    last_results.json
```

`manifest.json` 是智能体入口，建议结构如下：

```json
{
  "debug_id": "dbg_20260505_153000_ab12",
  "project": "flutter-client-for-appagent",
  "project_dir": "cmd/flutter-client-for-appagent/flutter_client_for_appagent",
  "created_at": "2026-05-05T15:30:00+08:00",
  "issue": {
    "title": "Cortana 页面 WebSocket 断开后无法恢复",
    "user_description": "...",
    "expected": "...",
    "actual": "...",
    "repro_steps": ["..."]
  },
  "entrypoints": {
    "summary": "summary.md",
    "timeline": "timeline.jsonl",
    "client_log": "logs/flutter_client.log",
    "server_logs": ["logs/app_agent.log", "logs/blog_agent.log"]
  },
  "constraints": {
    "encoding": "UTF-8 without BOM",
    "forbidden_commands": ["flutter build apk", "build-apk.sh"],
    "allowed_validation": [
      "dart analyze",
      "flutter analyze",
      "dart format --set-exit-if-changed .",
      "flutter test"
    ]
  }
}
```

## Flutter 客户端探针

新增 `lib/debug/` 模块，先保持轻量，不引入复杂依赖：

- `debug_event.dart`：定义 `DebugEvent`，字段包括时间、级别、分类、页面、动作、消息、错误、堆栈、关联 ID。
- `debug_recorder.dart`：内存环形缓冲，默认 1000 条；同时桥接现有 `addFlutterClientLog`。
- `debug_http_client.dart`：包装关键 HTTP 调用，记录 method、path、status、耗时、错误类型；不记录 token 和正文敏感字段。
- `debug_ws_observer.dart`：记录 WebSocket connect、message、error、close、reconnect。
- `debug_exporter.dart`：导出 JSONL/文本摘要，供 App UI 或 `app-agent` 拉取。

启动时加全局错误捕获：

```dart
void main() {
  FlutterError.onError = (FlutterErrorDetails details) {
    DebugRecorder.instance.recordFlutterError(details);
    FlutterError.presentError(details);
  };
  PlatformDispatcher.instance.onError = (Object error, StackTrace stack) {
    DebugRecorder.instance.recordError('platform', error, stack);
    return false;
  };
  runApp(const AppAgentClientApp());
}
```

客户端 Debug 面板不需要做成复杂页面，先在现有设置/日志区域提供三个动作：

- 导出 Debug Bundle。
- 复制最近 200 条客户端日志。
- 上报当前页面状态到 `app-agent`。

## app-agent Debug API

在 `cmd/app-agent` 增加一组认证后的 API：

| Endpoint | 用途 |
| --- | --- |
| `POST /api/app/debug/bundles` | 创建 Debug Bundle，接收用户问题、复现步骤、客户端日志、页面状态 |
| `GET /api/app/debug/bundles/{id}` | 返回 bundle manifest 和摘要 |
| `GET /api/app/debug/bundles/{id}/file?path=...` | 读取 bundle 内单个文件 |
| `POST /api/app/debug/bundles/{id}/attach-client-log` | 追加 Flutter 客户端导出的 JSONL |
| `POST /api/app/debug/bundles/{id}/redact` | 对指定文件重新脱敏 |

Bundle 创建时 `app-agent` 负责：

1. 保存客户端提交的 `issue`、`app_state`、`timeline`。
2. 调用现有日志读取逻辑收集最近 N 行服务端日志。
3. 记录 `git status --short`、`pubspec.lock` 摘要、Flutter/Dart 版本。
4. 生成 `summary.md`，把最可能的入口文件、错误片段和建议验证命令列出来。

日志源继续复用现有 `log_agent_config_file`，但建议补齐：

```json
{
  "log_sources": {
    "app-agent": {"path": "./logs", "description": "app-agent logs"},
    "blog-agent": {"path": "../blog-agent/logs", "description": "blog-agent logs"},
    "acp-agent": {"path": "../acp-agent/logs", "description": "acp-agent logs"}
  }
}
```

## acp-agent Debug 工具

在 `cmd/acp-agent/connection.go` 的工具定义中新增三类工具：

| 工具 | 用途 |
| --- | --- |
| `AcpListDebugBundles` | 列出当前项目可用 Debug Bundle |
| `AcpReadDebugBundle` | 读取指定 bundle 的 `manifest.json`、`summary.md` 和关键日志 |
| `AcpStartDebugSession` | 启动 Codex/ClaudeCode，并自动把 bundle 路径、问题摘要、验证约束注入 prompt |

`AcpStartDebugSession` 的 prompt 模板应固定，避免上游 LLM 传参时丢关键信息：

```text
你正在修复 Flutter App 问题。

Debug Bundle: <absolute_bundle_path>
项目目录: <project_dir>

请先阅读 manifest.json 和 summary.md，再检查相关源码。
必须遵守：
1. 所有文件读写使用 UTF-8 无 BOM。
2. 不得执行 flutter build apk 或任何 APK 打包命令。
3. 验证优先级：dart analyze -> flutter analyze -> dart format --set-exit-if-changed . -> flutter test。
4. 只修改与问题相关的文件，不回滚用户未提交改动。

用户问题：
<issue>
```

这样 `llm-agent` 或其他调用方只需要传 `debug_id`、`project`、`tool`，不用拼复杂 prompt。

## Bundle 摘要规则

`summary.md` 面向智能体，必须短而密：

- 问题一句话。
- 复现步骤。
- 当前平台、App 版本、后端 URL、登录状态、WebSocket 状态。
- 最近异常堆栈前 3 条。
- 最近失败 HTTP/WebSocket 事件。
- 最近 codegen/acp 会话 ID。
- 相关文件候选，例如 `lib/main.dart`、`lib/cortana_page.dart`、`cmd/app-agent/bridge.go`。
- 允许执行的验证命令。

不要把完整日志塞进摘要；完整日志放 `logs/` 和 `timeline.jsonl`。

## 脱敏规则

采集阶段统一执行脱敏：

- Header：`Authorization`、`Cookie`、`Set-Cookie`、`X-Api-Key`。
- JSON 字段：`password`、`token`、`access_token`、`refresh_token`、`session_token`、`delegation_token`、`secret`、`private_key`。
- URL Query：`token`、`session_token`、`access_token`、`password`。
- 文件路径保留项目相对路径；用户主目录可替换为 `$HOME`。

脱敏后保留 hash 前缀，例如 `<redacted:sha256:ab12cd34>`，方便判断两个 token 是否相同但不泄露原值。

## 智能体工作流

1. 用户在 App 内点“导出 Debug Bundle”，或通过消息描述问题触发 `POST /api/app/debug/bundles`。
2. `llm-agent` 调用 `AcpStartDebugSession(project, debug_id, tool)`。
3. `acp-agent` 读取 bundle，启动 Codex/ClaudeCode。
4. 编码智能体先读 `manifest.json` 和 `summary.md`，再按候选文件定位。
5. 修改后只运行允许的 Flutter 验证命令。
6. 结果写回 `validation/last_results.json`，并在会话总结里给出改动、验证、残余风险。

## MVP 实施计划

第一阶段：只做文件 bundle 和 ACP prompt 注入。

- 新增 `docs/flutter_app_debug_system.md`。
- 在 `app-agent` 增加 bundle 创建 API。
- 复用现有 `/api/app/logs/*` 读取服务端日志。
- 新增 `AcpStartDebugSession`，先只接收 `debug_bundle_path`，不做远程下载。
- Flutter 侧先提供“复制/导出最近客户端日志”。

第二阶段：客户端结构化探针。

- 增加 `DebugRecorder`、全局错误捕获、HTTP/WebSocket 事件记录。
- App UI 增加 Debug 面板。
- 支持截图和页面状态导出。

第三阶段：自动诊断和回放。

- 生成 `timeline.jsonl` 后自动分类错误。
- 对常见问题生成候选修复区域，例如认证、WebSocket、附件下载、Live2D 资源、语音录制。
- 支持 Playwright/Flutter integration 测试脚本记录，但默认仍不执行 APK 打包。

## 验收标准

- 用户只提供一句问题描述时，也能生成包含日志、环境、最近状态的 bundle。
- Codex/ClaudeCode 通过 `AcpStartDebugSession` 能在首轮 prompt 中看到 bundle 路径和验证约束。
- Bundle 中不出现明文 token、密码、cookie。
- 修复 Flutter 代码后，验证命令不包含 `flutter build apk`。
- 对 WebSocket、HTTP、Flutter 异常、服务端错误至少能关联到同一个 `debug_id`。

