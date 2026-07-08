# blog-agent 项目清理实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从 blog-agent 移除 11 个无用的 pkgs 子包及其所有引用，清理 go.mod，确保编译和测试通过。

**Architecture:** 分 8 个任务，自顶向下逐包解耦：先清理调用方引用，验证编译，最后删除包目录和 go.mod 条目。每步以 `go build ./...` 验证。

**Tech Stack:** Go 1.24, 纯删除操作，无新增依赖。

## 前置发现：调整移除范围

原设计文档计划移除 17 个包。深入分析后发现 `codegen`、`wechat`、`tools` 与保留模块深度耦合：

| 包 | 实际角色 | 耦合情况 |
|---|---|---|
| `codegen` | Gateway 桥接基础设施，不是"编码助手" | http_handler.go(assistant chat)、main.go(AIRouteHandler)、llm/agent_bridge.go、llm/sync_client.go 均依赖它 |
| `wechat` | 仅被 codegen/gateway.go 引用，随 codegen 保留 | 无独立影响 |
| `tools` | 被 codegen/remote.go 和 codegen/wechat_cmd.go 引用 | 随 codegen 保留 |
| `sms` | 被 login/login.go 引用（仅 GenerateSMSCode） | 可局部清理 |

**调整后实际移除 11 个包：** `encryption` `sms` `lifecountdown` `skill` `gomoku` `minesweeper` `tetris` `fruitcrush` `linkup` `constellation` `finance`

`codegen` `wechat` `tools` 保留。

## Global Constraints

- 每步必须以 `go build ./...` 验证通过
- `go.mod` replace/require 条目需同步清理
- 不深入保留包内部清理未使用函数（缩小变更范围）
- 不删除 templates/statics/scripts 中可能无用的资源

---

### Task 1: 清理 login/login.go — remove sms

**Files:**
- Modify: `pkgs/login/login.go`

- [ ] **Step 1: 删除 sms import**

移除 import 块中的 `"sms"`。

- [ ] **Step 2: 删除 GenerateSMSCode 函数**

删除 `func GenerateSMSCode(account string) (string, int)`（第 112-124 行），该函数调用 `sms.SendSMS()`。

- [ ] **Step 3: 验证编译**

```bash
cd cmd/blog-agent && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmd/blog-agent/pkgs/login/login.go
git commit -m "refactor(login): 移除 sms 短信验证码依赖

GenerateSMSCode 函数及其 sms.SendSMS 调用已移除。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 2: 清理 http/http_reading.go — remove lifecountdown

**Files:**
- Modify: `pkgs/http/http_reading.go`

- [ ] **Step 1: 删除 lifecountdown import**

移除 import 块中的 `"lifecountdown"`（第 7 行）。

- [ ] **Step 2: 删除全部 3 个 lifecountdown handler**

删除 `http_reading.go` 中的三个函数（第 1010-1130 行）：
- `HandleLifeCountdown` (line 1012) — 页面渲染，调用 `view.PageLifeCountdown`
- `HandleLifeCountdownAPI` (line 1024) — API，调用 `lifecountdown.UserConfig` 和 `lifecountdown.CalculateLifeCountdown`
- `HandleLifeCountdownConfigAPI` (line 1088) — 配置 API

- [ ] **Step 3: 删除 http_core.go 中的 3 条路由注册（第 339-341 行）**

```go
h.HandleFunc("/lifecountdown", HandleLifeCountdown)
h.HandleFunc("/api/lifecountdown", HandleLifeCountdownAPI)
h.HandleFunc("/api/lifecountdown/config", HandleLifeCountdownConfigAPI)
```

- [ ] **Step 4: 验证编译**

```bash
cd cmd/blog-agent && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/blog-agent/pkgs/http/http_reading.go cmd/blog-agent/pkgs/http/http_core.go
git commit -m "refactor(http): 移除 lifecountdown 人生倒计时模块

删除 HandleLifeCountdownAPI handler 及相关路由注册。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 3: 清理 http_core.go — 移除游戏/星座/金融路由和 import

**Files:**
- Modify: `pkgs/http/http_core.go`

- [ ] **Step 1: 删除 imports**

移除 import 块中的：
```go
"constellation"
"finance"
"fruitcrush"
"gomoku"
"linkup"
"minesweeper"
"tetris"
```

- [ ] **Step 2: 删除路由注册**

删除所有以下路由注册（约第 376-462 行）：
- Constellation 路由（`/constellation`, `/api/constellation/*`，~11 行）
- Gomoku 路由（`/gomoku`, `/api/gomoku/*`，~6 行）
- Linkup 路由（`/linkup`, `/api/linkup/*`，~14 行）
- Finance 路由（`/finance`, `/api/finance/*`，~3 行）
- Tetris 路由（`/tetris`, `/api/tetris/*`，~6 行）
- Minesweeper 路由（`/minesweeper`, `/api/minesweeper/*`，~6 行）
- Fruit Crush 路由（`/fruitcrush`, `/api/fruitcrush/*`，~6 行）

- [ ] **Step 3: 验证编译**

```bash
cd cmd/blog-agent && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmd/blog-agent/pkgs/http/http_core.go
git commit -m "refactor(http): 移除游戏/星座/金融模块路由注册

移除 gomoku/minesweeper/tetris/fruitcrush/linkup/constellation/finance 的 import 和 HandleFunc 注册。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 4: 删除 http_skill.go 和清理 http_lifecycle.go

**Files:**
- Delete: `pkgs/http/http_skill.go`
- Modify: `pkgs/http/http_core.go`
- Modify: `pkgs/http/http_lifecycle.go`

- [ ] **Step 1: 从 http_core.go 删除 skill 路由注册**

删除：
```go
h.HandleFunc("/skill", HandleSkill)
RegisterSkillRoutes()
```

- [ ] **Step 2: 从 http_lifecycle.go 删除 HandleSkill**

删除 `func HandleSkill`（第 161-169 行）。

- [ ] **Step 3: 删除 http_skill.go**

```bash
rm cmd/blog-agent/pkgs/http/http_skill.go
```

- [ ] **Step 4: 验证编译**

```bash
cd cmd/blog-agent && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/blog-agent/pkgs/http/
git commit -m "refactor(http): 移除 skill 技能学习模块

删除 http_skill.go、HandleSkill handler 及相关路由注册。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 5: 删除 11 个包目录

**Files:**
- Delete: `pkgs/encryption/`（全部文件）
- Delete: `pkgs/sms/`（全部文件）
- Delete: `pkgs/lifecountdown/`（全部文件）
- Delete: `pkgs/skill/`（全部文件）
- Delete: `pkgs/gomoku/`（全部文件）
- Delete: `pkgs/minesweeper/`（全部文件）
- Delete: `pkgs/tetris/`（全部文件）
- Delete: `pkgs/fruitcrush/`（全部文件）
- Delete: `pkgs/linkup/`（全部文件）
- Delete: `pkgs/constellation/`（全部文件）
- Delete: `pkgs/finance/`（全部文件）

- [ ] **Step 1: 删除目录**

```bash
cd cmd/blog-agent
rm -rf pkgs/encryption pkgs/sms pkgs/lifecountdown pkgs/skill \
       pkgs/gomoku pkgs/minesweeper pkgs/tetris pkgs/fruitcrush \
       pkgs/linkup pkgs/constellation pkgs/finance
```

- [ ] **Step 2: 验证编译（此时应失败，因为 go.mod 仍有 replace 指向不存在的目录）**

预期 `go build ./...` 报错：`directory pkgs/encryption does not exist` 等。这是预期行为——下一步清理 go.mod。

- [ ] **Step 3: 暂不 commit，等 go.mod 清理后一起提交**

---

### Task 6: 更新 go.mod

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: 移除 replace 指令**

删除 go.mod 中以下 replace 行：
```
replace encryption => ./pkgs/encryption
replace sms => ./pkgs/sms
replace lifecountdown => ./pkgs/lifecountdown
replace skill => ./pkgs/skill
replace gomoku => ./pkgs/gomoku
replace minesweeper => ./pkgs/minesweeper
replace tetris => ./pkgs/tetris
replace fruitcrush => ./pkgs/fruitcrush
replace linkup => ./pkgs/linkup
replace constellation => ./pkgs/constellation
replace finance => ./pkgs/finance
```

- [ ] **Step 2: 运行 go mod tidy 自动清理**

```bash
cd cmd/blog-agent && go mod tidy
```

这将自动从 require 块中移除不再需要的间接依赖。

- [ ] **Step 3: 验证编译**

```bash
cd cmd/blog-agent && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmd/blog-agent/pkgs/ cmd/blog-agent/go.mod cmd/blog-agent/go.sum
git commit -m "refactor: 删除 11 个无用包并清理 go.mod

移除包: encryption sms lifecountdown skill gomoku minesweeper
tetris fruitcrush linkup constellation finance

清理 go.mod replace 指令，go mod tidy 更新 require 块。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

### Task 7: 运行全量测试验证

**Files:** 无变更，仅验证

- [ ] **Step 1: 运行 Go 测试**

```bash
cd cmd/blog-agent && go test ./...
```

预期：全部通过（无失败的测试）。

- [ ] **Step 2: 最终编译验证**

```bash
cd cmd/blog-agent && go build ./...
```

预期：编译成功，无任何 warning。

- [ ] **Step 3: 若有测试失败，修复后重新验证**

检查失败原因。如果某个测试引用了已删除的包，该测试也需删除。

---

### Task 8: 最终检查

**Files:** 无变更

- [ ] **Step 1: 确认无残留引用**

```bash
cd cmd/blog-agent
for pkg in encryption sms lifecountdown skill gomoku minesweeper tetris fruitcrush linkup constellation finance; do
  if grep -rn "\"$pkg\"" pkgs/ main.go --include="*.go" 2>/dev/null; then
    echo "WARNING: $pkg still referenced!"
  fi
done
```

预期：无任何输出（所有残留引用已清除）。

- [ ] **Step 2: 检查 go.mod 干净度**

```bash
grep -E "encryption|sms|lifecountdown|skill|gomoku|minesweeper|tetris|fruitcrush|linkup|constellation|finance" cmd/blog-agent/go.mod
```

预期：无输出（go.mod 中已无这些包的条目）。
