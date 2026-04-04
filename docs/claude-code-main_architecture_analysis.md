# Claude Code Main 架构深度拆解

本文基于对 `/Users/guccang/github_repo/claude-code-main` 源码的直接阅读整理，目标不是做功能清单，而是解释 Claude Code 这套 CLI 智能体系统“怎么启动、怎么形成上下文、怎么跑 agentic loop、怎么做权限、怎么派生 subagent、怎么把 UI/bridge/remote 串起来”。

本文重点参考的核心文件：

- `src/main.tsx`
- `src/setup.ts`
- `src/context.ts`
- `src/constants/prompts.ts`
- `src/utils/claudemd.ts`
- `src/query.ts`
- `src/Tool.ts`
- `src/tools.ts`
- `src/services/tools/toolOrchestration.ts`
- `src/services/tools/StreamingToolExecutor.ts`
- `src/hooks/useCanUseTool.tsx`
- `src/utils/permissions/permissionSetup.ts`
- `src/skills/loadSkillsDir.ts`
- `src/tools/AgentTool/AgentTool.tsx`
- `src/tools/AgentTool/runAgent.ts`
- `src/utils/systemPrompt.ts`
- `src/memdir/memdir.ts`
- `src/screens/REPL.tsx`
- `src/bridge/bridgeMain.ts`
- `src/remote/RemoteSessionManager.ts`
- `src/commands.ts`

---

## 1. 总体判断

Claude Code 不是一个“简单的聊天 CLI”，而是一套完整的 agent runtime。它的本质是：

1. 一个可组合的系统 prompt 装配器。
2. 一个可流式运行的 query loop。
3. 一个高度工程化的 tool runtime。
4. 一个按权限模式裁剪的执行环境。
5. 一个支持 skills / MCP / commands / subagents / teammates / remote bridge 的多层扩展系统。
6. 一个由 React + Ink 驱动的 TUI 外壳。

从源码结构看，它不是“以模型为中心，再补一点工具”，而是“以 agent runtime 为中心，模型只是其中一层”。

也就是说，Claude Code 的真正核心不是某个 prompt 文件，而是下面这条链：

`启动 -> 建立上下文 -> 装配 system prompt -> 进入 query loop -> 解析 streaming message/tool_use -> 权限判定 -> 工具执行 -> 把 tool_result 写回消息流 -> 继续 loop -> 渲染 UI / 推送 bridge / 驱动后台 agent`

---

## 2. 顶层目录怎么分工

从 `src/` 看，Claude Code 大致可拆成 10 层：

### 2.1 入口与初始化层

- `main.tsx`
- `setup.ts`
- `entrypoints/`
- `bootstrap/`

作用：

- 解析 CLI 参数。
- 提前启动耗时预取。
- 初始化 session / cwd / model / settings / telemetry / remote / plugins / MCP。
- 最终进入 REPL、print mode、bridge mode、remote mode 等具体运行形态。

### 2.2 Prompt / Context 层

- `context.ts`
- `constants/prompts.ts`
- `utils/systemPrompt.ts`
- `utils/claudemd.ts`
- `memdir/`

作用：

- 收集 git 状态、日期、用户/项目指令、memory。
- 根据模式、模型、工具、MCP、skills、feature flags 组装系统提示词。
- 管理 `CLAUDE.md` / `CLAUDE.local.md` / `.claude/rules/*.md` 等长期指令。

### 2.3 Query Engine 层

- `query.ts`
- `query/`
- `services/api/`

作用：

- 执行一轮或多轮 agentic loop。
- 驱动 streaming assistant output。
- 处理 thinking、tool_use、tool_result、compact、retry、fallback。

### 2.4 Tool Runtime 层

- `Tool.ts`
- `tools.ts`
- `tools/*`
- `services/tools/*`

作用：

- 定义工具协议。
- 注册全部工具。
- 执行串行/并行工具调用。
- 做 tool progress、interrupt、concurrency、安全边界。

### 2.5 Permission / Safety 层

- `hooks/useCanUseTool.tsx`
- `utils/permissions/*`
- `components/permissions/*`

作用：

- 在每次 tool invocation 前做 allow / deny / ask。
- 支持 default / auto / plan / bypass 等权限模式。
- 支持 classifier、规则、队列、交互确认、bridge 远端确认。

### 2.6 Agent 扩展层

- `tools/AgentTool/*`
- `tasks/*`
- `utils/forkedAgent.ts`
- `utils/swarm/*`

作用：

- 生成 fork subagent。
- 生成本地后台 agent。
- 生成 in-process teammate。
- 生成 remote agent / worktree agent。

### 2.7 Skill / Command / Plugin 层

- `commands.ts`
- `commands/*`
- `skills/*`
- `plugins/*`

作用：

- `/xxx` 命令是用户显式入口。
- skill 是 prompt 型工作流单元。
- plugin/bundled skill 是可插拔扩展面。

### 2.8 MCP / Bridge / Remote 层

- `services/mcp/*`
- `bridge/*`
- `remote/*`

作用：

- 连接外部 MCP server。
- 与 IDE / remote daemon 建立通信。
- 远端 session 通过 websocket + HTTP 控制。

### 2.9 UI 层

- `screens/REPL.tsx`
- `components/*`
- `hooks/*`

作用：

- 渲染消息、权限弹窗、task 列表、teammate 视图、MCP 面板等。

### 2.10 状态与持久化层

- `state/*`
- `bootstrap/state.ts`
- `utils/sessionStorage.ts`
- `assistant/sessionHistory.ts`

作用：

- session 级状态。
- query chain 状态。
- permission mode 状态。
- transcript / metadata / compact 边界 / agent lineage。

---

## 3. 启动链路：Claude Code 怎么跑起来

### 3.1 `main.tsx` 的角色不是“薄入口”，而是启动编排器

`src/main.tsx` 顶部有非常明确的优化思路：

- 最早启动 `startupProfiler`
- 提前启动 MDM 读取
- 提前启动 Keychain 预取

这说明 Claude Code 把 CLI 启动延迟当成核心工程问题在处理。它不是先把所有依赖都 import 完再说，而是在 import 初期就把慢 IO 并发打出去。

这里的设计特征有 3 个：

1. 顶层 side effect 是经过设计的，不是随手写的。
2. 很多模块通过 `bun:bundle` feature gate 懒加载，减小不同构建目标的体积和初始化代价。
3. `main.tsx` 既做 CLI option 初始化，也做环境级能力拼装，例如 MCP、plugins、telemetry、permissions、remote、session 恢复。

### 3.2 `setup.ts` 负责“把运行环境变成 agent 可工作环境”

`src/setup.ts` 做的不是普通项目里的“读配置”，而是 session runtime 的真正初始化：

- 校验 Node 版本。
- 切换 session id。
- 启动 UDS messaging。
- 捕获 teammate mode snapshot。
- 恢复 iTerm2 / Terminal 备份。
- `setCwd(cwd)`。
- 捕获 hooks 配置快照。
- 初始化 FileChanged watcher。
- 按需要创建 worktree / tmux session。

这意味着 Claude Code 的 setup 是“执行环境准备器”，而不仅是“应用配置加载器”。

对于智能体系统来说，这很关键：模型真正看到 prompt 前，系统已经把“协作通信、终端恢复、文件变化观察、worktree 隔离”这些运行时设施搭好了。

---

## 4. Context 层：Claude Code 如何构造长期指令与环境上下文

### 4.1 `context.ts` 把上下文分成 `systemContext` 与 `userContext`

`src/context.ts` 有两个核心 memoized 函数：

- `getSystemContext()`
- `getUserContext()`

#### `getSystemContext()`

主要包含：

- git status 快照
- branch / default branch / 最近提交
- cache breaker 注入

它强调的是“会话开始时的系统级环境快照”。

#### `getUserContext()`

主要包含：

- `CLAUDE.md` 系列文件的汇总结果
- `currentDate`

它强调的是“用户/项目层长期指令”。

这两个 context 都被 memoize，说明 Claude Code 强调：

- 同一会话内尽量复用上下文，减少重复 IO。
- system prompt 的静态部分要稳定，以提升 cache 命中率。

### 4.2 `utils/claudemd.ts` 是长期指令系统的核心

这是 Claude Code 最值得借鉴的模块之一。

它解决的问题不是简单的“读一个 `CLAUDE.md` 文件”，而是完整的“多来源 instruction discovery + include + 去重 + 限流”。

#### 发现顺序

源码注释已经写得非常清楚：

1. Managed memory，例如 `/etc/claude-code/CLAUDE.md`
2. User memory，例如 `~/.claude/CLAUDE.md`
3. Project memory，例如：
   - `CLAUDE.md`
   - `.claude/CLAUDE.md`
   - `.claude/rules/*.md`
4. Local memory，例如 `CLAUDE.local.md`

#### 目录遍历策略

- 从当前目录向上遍历到根目录。
- 越靠近当前工作目录的指令优先级越高。
- `.claude/rules/*.md` 作为规则碎片文件自动纳入。

#### 能力不是“读文件”，而是“读指令图”

`utils/claudemd.ts` 还支持：

- `@include` 语法
- frontmatter `paths`
- 文件类型白名单
- frontmatter 解析
- 避免循环包含
- 去重与变更检测

这使得 Claude Code 的 instruction system 本质上已经接近“小型规则编译器”，不是死板的文档拼接。

### 4.3 `memdir/memdir.ts` 不是聊天记忆，而是结构化长期记忆设施

Claude Code 的 memory 设计比较成熟，和很多“随便存一段摘要”的 agent 不同。

它定义了：

- 独立 memory directory
- `MEMORY.md` 作为索引入口
- 单个 memory file 存具体主题
- 对 entrypoint 做行数和字节截断
- typed memory taxonomy

从源码看，它的设计意图非常明确：

- memory 不等于本轮任务计划
- memory 不等于当前代码状态
- memory 用来记录未来对话仍然有价值的信息

也就是说，它把“memory / task / plan / codebase facts”严格区分开了。

这是很多 agent 系统缺失的一层。

---

## 5. System Prompt 组装：Claude Code 的 prompt 不是一段字符串，而是一套可缓存的 section graph

### 5.1 `constants/prompts.ts` 是 prompt 工厂

`getSystemPrompt()` 返回的不是单个字符串，而是 `string[]`。

这背后的工程含义非常重要：

- prompt 被拆成 section。
- section 可静态或动态计算。
- 动态段可单独缓存或失效。
- 存在明显的 cache boundary。

### 5.2 Prompt 结构分成“静态缓存段”和“动态段”

`SYSTEM_PROMPT_DYNAMIC_BOUNDARY` 是一个非常关键的设计点。

在它之前的是尽量可跨会话共享缓存的内容，例如：

- intro
- system rules
- coding style
- actions with care
- tool usage instructions
- tone/style

在它之后是动态内容，例如：

- session guidance
- memory prompt
- env info
- language
- output style
- MCP server instructions

这说明 Claude Code 的 prompt 设计目标不仅是“表达正确”，还包括：

- 尽量维持 prompt cache key 稳定
- 把波动部分压缩到尾部
- 给 late MCP connect / mode switch / memory 变化留出单独失效空间

### 5.3 `utils/systemPrompt.ts` 决定最终生效的 prompt 来源优先级

这个模块把 prompt 优先级写得很清楚：

1. override system prompt
2. coordinator prompt
3. agent prompt
4. custom system prompt
5. default system prompt
6. append system prompt

也就是说，Claude Code 不是“默认 prompt + 少量补丁”，而是“多种 agent 身份/模式下的 prompt 选择器”。

它还区分：

- 主线程 agent
- coordinator mode
- proactive mode
- custom agent
- append-only instruction

这使系统能在不破坏主 loop 的前提下切换行为人格。

---

## 6. Command 系统：`/xxx` 不是 shell alias，而是第一层产品级交互面

### 6.1 `commands.ts` 是命令注册中心

`src/commands.ts` 做的事情有：

- 注册 built-in commands
- 条件加载 feature-gated commands
- 加载 skill dir commands
- 加载 bundled skills
- 加载 plugin skills
- 合并命令与 skill
- 进行缓存

Claude Code 里的命令不是“顺手做几个 slash command”，而是整个交互体系的重要入口。

### 6.2 命令和 skill 的关系

源码里 skill 被建模成一种 command-like prompt source。

也就是说，系统不是把 skill 看成“外挂提示词”，而是：

- skill 可以进入命令列表
- skill 可以有 frontmatter
- skill 可以声明 allowed tools
- skill 可以声明 argument schema
- skill 可以决定是否 fork
- skill 可以声明 hooks

这使 skill 变成了一等公民。

---

## 7. Skill 系统：本质是“带 frontmatter 的 prompt workflow”

### 7.1 `skills/loadSkillsDir.ts` 的作用

这个模块做了 5 类事情：

1. 搜索 skills / commands 目录。
2. 解析 markdown frontmatter。
3. 生成内部 `Command` 表示。
4. 支持参数替换与 shell 前处理。
5. 把 skill 挂接到命令/模型执行体系。

### 7.2 Claude Code 的 skill 远强于普通模板

frontmatter 支持很多字段：

- `name`
- `description`
- `when_to_use`
- `allowed-tools`
- `arguments`
- `effort`
- `model`
- `hooks`
- `context: fork`
- `agent`
- `user-invocable`

这意味着 skill 不只是“一段提示词说明文档”，而是：

- 有元数据
- 有权限边界
- 有执行上下文
- 有模型偏好
- 有 hooks
- 有参数系统

### 7.3 skill 体系和 CLAUDE.md 体系是互补关系

从设计上看：

- `CLAUDE.md` 负责持久、广域、稳定规则。
- `skill` 负责可调用、具任务边界、具执行策略的工作流模板。

这是成熟智能体系统常见的分层：

- policy layer
- workflow layer

Claude Code 在这点上是清楚的。

---

## 8. Tool 协议：Claude Code 的工具不是一堆函数，而是一套统一 runtime contract

### 8.1 `Tool.ts` 定义整个工具世界的协议

这里最重要的不是某个字段，而是整体抽象：

- `ToolPermissionContext`
- `ToolUseContext`
- `ToolInputJSONSchema`
- 各类 progress type
- 消息/通知/UI/状态更新钩子

其中 `ToolUseContext` 非常关键，它基本等于“当前 agent runtime 句柄”。

里面包含：

- `commands`
- `tools`
- `mainLoopModel`
- `thinkingConfig`
- `mcpClients`
- `getAppState / setAppState`
- `readFileState`
- `abortController`
- `appendSystemMessage`
- `setToolJSX`
- `sendOSNotification`
- `agentId / agentType`
- `messages`
- 多种 UI 回调

也就是说，工具不是“一个输入一个输出”，而是运行在一个完整 session runtime 里。

### 8.2 工具是强状态化的

很多 CLI agent 会把工具设计成 stateless RPC。Claude Code 不是。

在 Claude Code 里，工具能：

- 改 AppState
- 发 UI
- 读取消息历史
- 影响权限队列
- 受 abort signal 控制
- 触发 hooks
- 写入 session persistence

这让工具真正成为 agent 运行时的一部分。

---

## 9. Tool 注册：`tools.ts` 是完整能力面装配器

### 9.1 `getAllBaseTools()` 是能力源头

`src/tools.ts` 的 `getAllBaseTools()` 列出完整基础工具集：

- AgentTool
- BashTool
- FileRead/Edit/Write
- WebFetch/WebSearch
- Todo/Task 系列
- SkillTool
- AskUserQuestion
- MCP resource tool
- Team / Message / Worktree / Plan mode
- 可选的 REPL/LSP/Workflow/Sleep/PowerShell 等

这里有两个关键设计：

1. feature flag 控制构建时是否包含某类工具。
2. 工具集会根据运行环境、权限、平台和模式进一步过滤。

### 9.2 工具可见性不是静态的

Claude Code 并不是“注册就给模型看”，而是经过多层过滤：

- feature gate
- env gate
- permission deny rule
- MCP availability
- mode-specific filtering
- agent-specific tool resolution

所以 Claude Code 的“tool list”本质上是运行时计算结果，不是静态常量。

---

## 10. Query Loop：Claude Code 的真正核心

### 10.1 `query.ts` 是总控循环

`query()` / `queryLoop()` 是 Claude Code 最核心的逻辑之一。

它的职责包括：

- 接收消息历史、system prompt、contexts、toolUseContext。
- 进入 streaming LLM 请求。
- 把返回的 assistant chunks 解析成消息。
- 识别 tool_use。
- 通过 tool orchestration 执行工具。
- 把 tool_result 重新喂回模型。
- 处理 compact、fallback、max_output_tokens recovery、budget continuation。

从架构上说，它是一个带恢复逻辑的“消息状态机 + 工具编排器”。

### 10.2 它不是简单 `while tool_calls`

它额外处理了很多复杂性：

- thinking block 规则
- compact boundary
- auto compact / reactive compact
- prompt too long 恢复
- max_output_tokens 恢复
- tool use summary
- stop hooks
- token budget
- skill prefetch
- memory attachment prefetch
- query source 分类

也就是说，Claude Code 的 query loop 已经不是“模型返回 tool_call 就执行”的低阶实现，而是一个面向真实产品环境的鲁棒循环。

### 10.3 query loop 的真正输入不只是 messages

`QueryParams` 里有：

- `messages`
- `systemPrompt`
- `userContext`
- `systemContext`
- `canUseTool`
- `toolUseContext`
- `fallbackModel`
- `querySource`
- `maxTurns`
- `taskBudget`

这说明 Claude Code 的 loop 不是单纯以 prompt 为中心，而是以“运行时执行上下文”为中心。

---

## 11. Tool 执行：Claude Code 对并发、安全和消息顺序做了认真设计

### 11.1 `toolOrchestration.ts` 负责批执行

它会先根据 `isConcurrencySafe()` 对工具调用分批：

- 连续的只读/并发安全工具可以并行。
- 有副作用或不安全的工具串行。

这个点非常重要。

很多 agent 会简单串行执行所有工具，导致性能差；或者简单并发执行，导致状态错乱。Claude Code 在两者之间做了非常明确的调度分层。

### 11.2 并发执行仍然保证消息顺序与 context modifier 应用顺序

在并发安全 batch 中：

- 结果可以并发跑出来。
- context modifier 会收集起来。
- 最后再按 tool 原始顺序应用。

这说明 Claude Code 区分了两件事：

1. 执行可以并发。
2. 状态变更提交要有顺序。

这就是成熟 runtime 的做法。

### 11.3 `StreamingToolExecutor.ts` 解决的是“流式到一半就出现 tool_use”的问题

这个类负责边流边执行工具。

特点：

- 工具 streaming 到来即可入队。
- 可并发工具并行执行。
- 非并发工具独占执行。
- 结果按工具出现顺序输出。
- 如果某个 bash sibling 出错，可中断其他 sibling。
- 支持 streaming fallback 后丢弃 pending 工具。

它本质上是一个“流式工具执行调度器”，不是普通的 `Promise.all` 包装。

---

## 12. Permission 系统：Claude Code 不是在 tool call 时“顺便问一下”，而是有完整判定引擎

### 12.1 `ToolPermissionContext` 是权限状态快照

里面有：

- 当前 mode
- 额外工作目录
- alwaysAllow / alwaysDeny / alwaysAsk rules
- bypass 是否可用
- auto mode 是否可用
- prePlanMode
- 是否应避免弹框
- 是否先等待自动检查

这说明权限不只是“允许/拒绝”，而是一个模式化 runtime。

### 12.2 `permissionSetup.ts` 负责初始化与危险规则裁剪

这个模块处理：

- 权限模式初始化
- 从磁盘加载权限规则
- 应用规则到 permission context
- 判定危险 bash / PowerShell / Agent permission
- plan mode / auto mode 相关的状态切换

其中有个很重要的理念：

不是所有 allow rule 都能直接用于 auto mode。

比如：

- `Bash(*)`
- `python:*`
- `node:*`
- Agent tool allow

这些会绕过分类器，因此会被视为危险规则。

### 12.3 `useCanUseTool.tsx` 才是实际判定调度器

它的流程大致是：

1. 构建 permission context。
2. 调 `hasPermissionsToUseTool()` 得到 allow / deny / ask。
3. allow 直接放行。
4. deny 直接拒绝，并记录 auto-mode denial。
5. ask 时再进入多分支：
   - coordinator handler
   - swarm worker handler
   - speculative classifier 快速批准
   - interactive permission dialog

也就是说，Claude Code 的权限系统不是简单 if/else，而是：

- 规则层
- classifier 层
- worker/team 协同层
- UI 交互层
- bridge 远端层

多层共同作用。

### 12.4 权限系统和 UI/bridge 是解耦的

`useCanUseTool.tsx` 并不直接依赖某种单一界面。它通过 context 和 callback 把权限请求发给：

- REPL interactive dialog
- bridge callback
- channel callback
- swarm permission bridge

这个抽象很好，因为同一套权限核心可以跑在：

- 本地 TUI
- IDE bridge
- remote session
- teammate mode

---

## 13. AgentTool：Claude Code 最关键的高级能力

### 13.1 AgentTool 不是“另一个工具”，而是 runtime 内再生成 runtime

`src/tools/AgentTool/AgentTool.tsx` 体现了 Claude Code 最强的一层：它允许模型继续生成新 agent。

这个工具支持：

- `subagent_type`
- `model`
- `run_in_background`
- `name`
- `team_name`
- `mode`
- `isolation`
- `cwd`

这意味着 AgentTool 既能：

- 开一个普通子 agent
- 开 worktree 隔离 agent
- 开 remote agent
- 开带权限模式的 teammate
- 开后台 agent

### 13.2 AgentTool 的 prompt 会根据 agent definition 动态生成

它不是固定 prompt。

流程上会：

- 从 agent definitions 中筛选可用 agent
- 结合 MCP requirement 过滤
- 结合 deny rule 过滤
- 通过 `getPrompt()` 告诉模型有哪些 agent 可以用

所以 AgentTool 也是一个“动态可见能力集”。

### 13.3 `runAgent.ts` 真正执行子 agent

这是 subagent 运行核心。

它会做：

- 初始化 agent 自己的 MCP servers
- 组装 agent 自己的 tool set
- 组装 agent 的 system prompt
- 继承或克隆部分父上下文
- 创建子级 `ToolUseContext`
- 调用 `query()`
- 记录 sidechain transcript
- 清理 agent 专属资源

这里一个非常关键的点是：

Claude Code 不是把子 agent 当成“函数调用”，而是完整复用主 query runtime。

也就是说，subagent 和主 agent 基本是同一执行机理，只是上下文、工具、权限和可见性不同。

### 13.4 `forkedAgent.ts` 的核心思想是“隔离可变状态，但共享 cache-safe 参数”

它明确考虑了 prompt cache：

- system prompt
- user context
- system context
- toolUseContext
- fork context messages

如果这些关键参数一致，就能让 fork agent 复用 prompt cache。

这说明 Claude Code 的 subagent 不是只考虑“能不能跑”，而是考虑“怎么跑得便宜、快、稳定”。

---

## 14. Teammate / Swarm：Claude Code 不只支持子代理，还支持协作型代理

从代码组织看，Claude Code 至少有三种代理形态：

1. 主线程 agent
2. subagent / fork agent
3. teammate / swarm agent

它们不是一回事。

### 14.1 subagent 更像“派生执行单元”

特点：

- 可以前台或后台
- 主要通过 AgentTool 创建
- 常常不直接共享 UI
- 更偏任务委派

### 14.2 teammate 更像“协作成员”

特点：

- 有 team context
- 可互发消息
- 可能有独立 mailbox
- 可能需要 plan approval
- 在 UI 中有 teammate view

### 14.3 这是 Claude Code 很强的一点

很多 agent 产品只做了“spawn worker”。Claude Code 进一步区分了：

- background task
- async local agent
- in-process teammate
- process-based teammate
- remote agent

这使它的协作模式更接近一个完整多 agent runtime，而不是单一的 task fork。

---

## 15. REPL：界面层其实是总控层之一

### 15.1 `screens/REPL.tsx` 不是单纯 UI 组件，而是主 session 容器

这个文件非常大，原因很简单：它承担的职责非常多。

它集成了：

- 输入框
- 消息列表
- 权限弹窗
- queue processor
- useCanUseTool
- query 调度
- remote/direct-connect/SSH session
- MCP 管理
- background task 导航
- teammate 视图
- file history
- cost tracking
- hook message
- compact/survey/notification

也就是说，REPL 在架构上是“交互式 session orchestrator”，不只是展示层。

### 15.2 REPL 负责将 query runtime 接到用户交互闭环

它做的关键工作有：

- 收集 prompt submit
- 生成 `ToolUseContext`
- 获取 system prompt / user context / system context
- 启动 `query()`
- 消费 streaming messages
- 把 tool progress、assistant 消息、compact 边界等写入 message state
- 响应 Ctrl+C、Escape、任务切换

这说明 Claude Code 的 UI 并不是 query loop 外面随便包一层，而是深度参与 runtime。

---

## 16. Bridge：Claude Code 不是只在本地终端跑

### 16.1 `bridge/bridgeMain.ts` 是一个长期运行的 session worker 管理器

它负责：

- 轮询 bridge API
- 维护 active sessions
- 发送 heartbeat
- spawn child session
- 管理 session timeout
- worktree 清理
- token refresh

这说明 bridge 模式下，Claude Code 实际上可以作为“受控 worker 守护进程”运行。

### 16.2 桥接不是消息转发，而是 session orchestration

bridge loop 要解决的问题包括：

- 环境标识
- work dispatch
- auth/token 续期
- active session registry
- reconnect
- session ingress token
- worker 与桥服务器之间的 work secret

这已经是一个小型 agent control plane 了。

---

## 17. Remote：远程会话是独立的一层协议

### 17.1 `remote/RemoteSessionManager.ts` 管远程 CCR 会话

它同时处理：

- websocket 订阅消息
- HTTP POST 发送消息
- control request/response
- 远端 permission request
- 断线回调

### 17.2 Remote session 把 permission 也远程化了

远端会话里，工具权限不是直接在本地 `useCanUseTool` 弹窗就结束，而是：

- CCR 发 control_request
- 本地记录 pending permission request
- 用户在本地批准/拒绝
- 通过 control_response 回给远端

这表明 Claude Code 的权限协议已经被抽象成跨连接边界可传输的控制面消息。

这很重要，因为一旦系统想支持：

- browser session
- IDE remote
- daemon mode
- cloud worker

权限流必须先协议化。

---

## 18. MCP：Claude Code 把外部工具生态做成了一等集成层

虽然本文没有逐个展开 `services/mcp/*`，但从 `tools.ts`、`commands.ts`、`prompts.ts`、`runAgent.ts`、`REPL.tsx` 能看出 MCP 的地位非常高：

- MCP tool 会并入主 tool pool。
- MCP resource 有独立工具。
- MCP server instruction 会被注入 system prompt。
- Agent 可声明 agent-specific MCP servers。
- REPL 有 MCP 管理 UI。

这说明 Claude Code 把 MCP 视为“能力扩展协议”，不是“顺便支持一下外部工具”。

---

## 19. Feature Flag 策略：源码不是单版本产品，而是多构建形态共存

Claude Code 大量使用 `bun:bundle` 的 `feature()`。

作用包括：

- 构建时裁剪功能
- 外部构建剔除内部能力
- ant-only 功能不进入 external build
- 降低 bundle 体积
- 防止内部字符串泄漏

常见被 feature gate 的能力有：

- proactive / kairos
- coordinator mode
- bridge mode
- daemon
- voice
- workflow scripts
- agent triggers
- fork subagent
- context collapse
- MCP delta

这意味着 Claude Code 本身其实是“一套 agent platform”，而不是单一 SKU。

---

## 20. Claude Code 的几个关键设计哲学

### 20.1 Prompt 只是 runtime 的一部分，不是全部

很多项目把“智能体”理解成一大段 system prompt。Claude Code 明显不是。

它真正的智能体能力来自：

- prompt section graph
- permission runtime
- tool runtime
- state/persistence
- skill/plugin system
- agent spawning
- UI/bridge/remote integration

### 20.2 它把“规则”拆成多层

Claude Code 的规则来源至少有：

- 静态系统规则
- `CLAUDE.md` / `.claude/rules`
- memory
- skill frontmatter
- permission rules
- MCP server instructions
- agent-specific prompt
- append system prompt
- feature gated system behavior

这比“单层提示词叠加”要成熟得多。

### 20.3 它认真区分了几种持久化

- `CLAUDE.md`：项目/用户长期规则
- memory：未来仍有价值的记忆
- tasks：本会话任务管理
- plans：当前任务计划
- transcripts：可恢复执行历史
- hooks snapshot / file state：运行时环境状态

这种区分让系统不容易把所有信息都粗暴塞进一个上下文桶里。

### 20.4 它认真对待“真正的工程问题”

从源码能明显看到他们在处理：

- 启动性能
- prompt cache 命中
- 动态 section 缓存
- 工具并发与顺序一致性
- 远端权限协议
- subagent 状态隔离
- worktree 隔离
- 恢复/重连

这说明 Claude Code 已经不是“研究 demo”，而是产品级 agent runtime。

---

## 21. 如果把 Claude Code 的核心抽象成一张图

可以这样理解：

### 21.1 控制流

`CLI / REPL / Bridge / Remote`
-> `setup`
-> `load settings / permissions / commands / tools / skills / plugins / mcp`
-> `build contexts`
-> `build system prompt`
-> `query loop`
-> `permission check`
-> `tool orchestration`
-> `stream messages to UI / remote / sdk`
-> `persist transcript / state`

### 21.2 数据流

`User input`
-> `Message queue`
-> `QueryParams`
-> `Anthropic streaming response`
-> `assistant message / tool_use / progress / compact`
-> `tool_result`
-> `new messages`
-> `UI state + session storage + sidechain transcript`

### 21.3 权限流

`tool_use`
-> `hasPermissionsToUseTool`
-> `allow / deny / ask`
-> `classifier / worker / interactive / bridge`
-> `permission decision`
-> `tool execution or rejection message`

### 21.4 agent 扩展流

`AgentTool`
-> `select agent definition`
-> `resolve tools + prompt + MCP + model`
-> `create subagent context`
-> `run query() again`
-> `return result / async task / teammate state`

---

## 22. 对你现在这个 go_blog/llm-agent 改造最有价值的借鉴点

如果目标是“重新彻底设置智能体”，我认为 Claude Code 最值得迁移的不是某几句 prompt，而是下面 8 个机制：

### 22.1 指令发现机制

把 `AGENTS.md / CLAUDE.md / .claude/rules/*.md / 用户级规则` 变成统一 discovery 层，而不是只读一个固定文件。

### 22.2 system prompt 分段缓存机制

把静态段和动态段拆开，不要每轮都重新生成所有 prompt 内容。

### 22.3 权限模式机制

至少要有：

- 默认交互
- 自动安全模式
- 计划模式
- 危险绕过模式

而不是只有 allow/deny。

### 22.4 Tool runtime contract

工具需要运行在统一 `ToolUseContext` 中，能访问：

- 状态
- 消息
- abort
- UI
- app state

否则后期能力会碎掉。

### 22.5 并发安全工具调度

工具不应永远串行；但并发也不能粗暴 `Promise.all`。

### 22.6 Skill 前台/后台/fork 语义

skill 不应只是“把内容贴进 prompt”，而应能表达：

- 使用哪些工具
- 是否可 fork
- 是否绑定 agent
- 是否带 hooks

### 22.7 Subagent 与主 agent 复用同一 query engine

不要单独为子任务再写一套简化循环。Claude Code 的重要优点是主子 agent 基本复用同一套 runtime。

### 22.8 transcript / session / plan / memory 分层

不要把“当前会话消息”“长期规则”“任务计划”“错误经验”全塞进一个文件或一个 context 拼接器里。

---

## 23. 结论

Claude Code 的源码显示，它的真正价值不在某个单点，而在于它把下面这些东西耦合成了一套完整系统：

- 指令系统
- 记忆系统
- 命令系统
- 工具系统
- 权限系统
- agent 递归系统
- UI/bridge/remote 系统
- 会话恢复与持久化系统

所以如果要“对齐 Claude Code”，正确路径不是：

- 抄一份 prompt
- 增加几个命令
- 加一个 subagent 工具

而是要按层次重构：

1. 先把 instruction/context 层补齐。
2. 再把 permission/runtime 层补齐。
3. 再把 tool orchestration 与 subagent 统一到同一 query loop。
4. 最后再接 skill / bridge / remote / UI。

从源码成熟度来看，Claude Code 更像一个“agent 操作系统”，而不是一个“会调工具的聊天壳”。

