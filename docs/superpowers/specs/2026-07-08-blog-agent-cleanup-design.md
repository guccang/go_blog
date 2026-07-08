# blog-agent 项目清理设计

## 目标

移除 blog-agent 项目中的 17 个无用包及其所有引用，清理 go.mod，确保编译和测试通过。

## 保留范围

- 基础设施：config http mylog module control view persistence ioutils auth login mcp llm delegation uap agentbase
- 博客核心：blog comment search share statistics reading
- 效率管理：exercise goal todolist yearplan projectmgmt taskbreakdown memory
- 经评估保留：account email（被保留模块深度依赖）

## 移除清单（17 个包）

### L1: 零引用，直接删除
- `encryption` — 无任何代码引用

### L2: 仅 http_core.go 路由注册引用，删除路由+包
- `gomoku` `minesweeper` `tetris` `fruitcrush` `linkup` `constellation` `finance`

### L3: 有深层依赖需解耦
| 包 | 引用点 |
|---|---|
| `codegen` | main.go(~70行初始化代码), pkgs/http/http_codegen.go(整文件), pkgs/llm/agent_bridge.go, pkgs/llm/sync_client.go |
| `wechat` | 仅被 codegen/gateway.go 引用，随 codegen 一起移除 |
| `tools` | pkgs/http/http_core.go(路由注册) |
| `sms` | pkgs/login/login.go |
| `lifecountdown` | pkgs/http/http_reading.go |
| `skill` | pkgs/http/http_skill.go(整文件), pkgs/llm/ai_skill.go |

## 实施步骤

1. **L3 解耦** — 逐包清理引用点，每改一批就验证编译
   - main.go: 移除 codegen 导入和 ~70 行初始化代码
   - llm/agent_bridge.go: 移除 codegen 引用
   - llm/sync_client.go: 移除 codegen 引用
   - llm/ai_skill.go: 移除 skill 引用
   - login/login.go: 移除 sms 引用
   - http/http_codegen.go: 删除整个文件
   - http/http_skill.go: 删除整个文件
   - http/http_core.go: 移除 tools/gomoku/minesweeper/tetris/fruitcrush/linkup/constellation/finance 路由注册
   - http/http_reading.go: 移除 lifecountdown 引用
2. **L2 路由清理** — http/http_core.go 中移除游戏/星座/金融等路由
3. **删除包目录** — 移除 17 个 pkgs/ 子目录
4. **更新 go.mod** — 移除对应 replace 和 require 条目
5. **验证** — go build + go test 全部通过

## 不做的事

- 不深入保留包内部清理未使用的函数/类型（缩小变更范围）
- 不删除 templates/statics/scripts 中可能无用的资源（缺乏判断依据，可后续单独清理）
