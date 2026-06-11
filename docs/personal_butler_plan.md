# 私人管家（Butler）实现方案

> 参考：[MiMo Code: Scaling Coding Agents to Long-Horizon Tasks](https://mimo.xiaomi.com/blog/mimo-code-long-horizon)
> 目标：以 Flutter App 为终端（输入 + 形象展示），打造接管工作/生活/锻炼方方面面的私人管家：
> 1. 管理 blog-agent（博客、待办、目标、年计划等数据与操作）
> 2. 管理玩家设定的目标（长程跟踪，而非一次性记录）
> 3. 提醒玩家（定时 + 情境触发）
> 4. 安慰陪伴玩家（共情、人格、养成）

---

## 1. 来自 MiMo Code 的三个设计借鉴

MiMo Code 解决的是"编码 agent 跑几百步不迷失"，其方法论同样适用于"管家陪伴用户几个月不失忆"：

| MiMo 主题 | MiMo 做法 | 管家映射 |
|-----------|----------|----------|
| **记忆** | 项目级 Markdown 记忆文件（MEMORY.md / checkpoint.md / 进度日志），上层 FTS 全文索引；选文件而非向量库，**用户可审阅、可编辑、可删除** | 每用户 Markdown 记忆库：`MEMORY.md`（长期事实）+ `journal/`（每日日志）+ `checkpoint.md`（当前关注点）；Flutter 提供"记忆审阅页"，用户能看到管家记住了什么并修改 |
| **计算** | Max Mode：每轮并行生成 N 个候选方案，同模型裁判选优后执行 | 高敏感决策（要不要此刻打扰、如何安慰低落的用户）可用"多候选 + 自裁判"；常规决策单候选省成本 |
| **进化** | 任务树（T1/T1.1）、独立判官判断目标是否真完成（防"乐观停止"）、人在环反馈 | 目标树（G1/G1.1）+ 每个目标带"完成判据"；hermes 定期用**独立判官 prompt** 评审判据是否达成；用户反馈（有用/太频繁）回写记忆调整行为 |

核心取舍：**记忆用可审阅的 Markdown 文件 + 关键词/FTS 检索，不引入向量库**。私人管家的记忆必须可信、可纠错——用户能看到"它为什么这么说"。

---

## 2. 现状分析（cortana-agent vs hermes-agent）

### 2.1 能力对比

| 维度 | cortana-agent | hermes-agent |
|------|---------------|--------------|
| 定位 | 感知 + 表达（监控数据、决定播报、TTS、表情动作） | 通用推理运行时（工具调用、多步执行） |
| 长期记忆 | ❌ 仅上轮摘要（companion_state ≤100 字） | ✅ 有 memory provider 框架（未启用）、会话历史 |
| 主动性 | ✅ 120s 监控循环 + 事件驱动 + 4 层冷却（全局 600s / 场景每日上限 / 免打扰时段 / 权限） | ✅ cron 系统（cron 表达式，origin 回传，输出落盘） |
| 决策方式 | 每次独立快照 → llm-agent `cortana_proactive` 单轮判断 | 多步推理直到任务完成（max_iterations=90） |
| 工具 | 6 个跨 agent 查询工具（拼快照用） | 已接通 blog-agent 全部 30+ MCP 工具（上期 go_blog 工具桥） |
| 人格/表达 | ✅ persona + 3 表情 + 4 动作 + TTS + Live2D | ❌ 无 |
| 与 Flutter | ✅ 播报推送 + 形象联动 + 设置页 | ✅ 对话路由（上期"对话 Agent 切换"） |

### 2.2 cortana-agent "做的事情简单了"的根因

1. **无记忆**：每次播报决策只看当下快照，不知道"上周用户说过最近失眠""昨天已经提醒过同一件事"
2. **单轮决策**：无法执行"观察 → 计划 → 多日跟进"的长程行为，只会"此刻要不要说一句话"
3. **无养成**：交互一百次和第一次没有区别，没有理解度/亲密度的积累
4. **目标管理缺位**：blog-agent 有 16 个 goal 工具，但没有谁在长期跟踪"目标是否真的在推进"

### 2.3 结论：不是二选一，而是分工

- **hermes-agent = 管家大脑**：长程推理、目标评审、复杂任务、创建提醒（已具备 cron + blog-agent 工具桥）
- **cortana-agent = 管家心脏**：何时开口、用什么情绪开口、表情/语音/形象（保留其打扰控制和表达链路）
- **blog-agent = 管家数据中枢**：目标/待办/年计划已有，**新增记忆存储工具**，让所有 agent 共享同一份用户记忆
- **Flutter = 管家面孔**：对话输入、Live2D 形象、记忆审阅、目标树、提醒管理

---

## 3. 总体架构

```
                         Flutter App（面孔）
        对话输入 │ Live2D 形象 │ 管家面板（目标树/记忆审阅/提醒/养成度）
                │                    ▲ /ws/app 推送（含表情+语音）
                ▼                    │
            app-agent ──────────────┘
                │ notify / preferences
     ┌──────────┼──────────────────────────┐
     ▼          ▼                          ▼
 hermes-agent  llm-agent             cortana-agent
 （大脑）      （快速对话）          （心脏：监控+表达）
     │ tool_call    │ tool_call            │ 快照 + 记忆 → 播报决策
     └──────┬───────┴───────────┬──────────┘
            ▼                   ▼
        blog-agent（数据中枢）
   goal/todo/yearplan 工具 + 【新】memory 工具（MEMORY.md/journal/checkpoint + FTS 检索）
```

记忆是三个 agent 的**共享底座**：hermes 处理对话时写记忆，cortana 播报前读记忆，llm-agent 闲聊时也能引用。

---

## 4. 记忆系统设计（核心新增）

### 4.1 存储：blog-agent 新增 `pkgs/mcp/inner_memory_tools.go`

按 account 隔离，文件落在 blog-agent 工作区 `memory/{account}/`：

```
memory/alice/
├── MEMORY.md          # 长期事实：喜好、习惯、忌讳、重要的人、纪念日、健康状况
├── checkpoint.md      # 当前关注点：本周重点、进行中的事、管家的下一步
├── goals.md           # 目标树（见 §5），与 goal 工具数据互为视图
└── journal/
    └── 2026-06-11.md  # 当日日志：交互摘要、情绪观察、完成事项、播报记录
```

新增 MCP 工具（自动被 hermes 工具桥发现，无需改 hermes）：

| 工具 | 功能 |
|------|------|
| `RawMemoryAppend(account, file, section, content)` | 追加记忆条目（带时间戳） |
| `RawMemorySearch(account, query, limit)` | 关键词/FTS 检索，返回片段+出处 |
| `RawMemoryReadFile(account, file)` | 读整个记忆文件（checkpoint/MEMORY） |
| `RawMemoryRewriteSection(account, file, section, content)` | 重写小节（记忆整理用） |
| `RawMemoryJournal(account, entry)` | 写当日 journal |

检索先用 Go 实现的关键词倒排（按行分词 + 简单评分）即可，后续可换 SQLite FTS5。

### 4.2 写入策略（谁在什么时候写）

- **hermes-agent**：对话结束后，若出现"值得长期记住的事实"（用户偏好、承诺、重要事件），调 `RawMemoryAppend`；系统提示词中加入记忆规范（什么该记/不该记、写入格式）
- **cortana-agent**：每次播报后写 journal（播报了什么、用户反馈）；每日终了把 companion_state 摘要落 journal
- **记忆整理（防膨胀）**：hermes cron 每周一次"记忆整理"job——读 journal 合并提炼进 MEMORY.md，过期内容归档，对应 MiMo 的上下文重建

### 4.3 读取策略（token 预算注入，借鉴 MiMo）

- cortana 构建快照时：`RawMemorySearch(当前事件关键词)` 取 top-5 片段 + `checkpoint.md` 全文，**限定 ≤1500 tokens** 注入决策 prompt
- hermes 处理对话时：检索相关记忆注入 system prompt（同样限预算）
- Flutter 记忆审阅页：经 blog-agent HTTP API 直接读写文件（可信、可纠错）

---

## 5. 目标管理（管理玩家设定的目标）

### 5.1 目标树 + 完成判据

扩展 `goals.md` / goal 工具数据结构（blog-agent 已有 16 个 goal 工具，补字段不重写）：

```markdown
## G1 三个月减重 4kg [active] (judge: 体重记录 ≤72kg)
- G1.1 每周锻炼 3 次 [tracking] (judge: 本周 exercise 记录 ≥3)
- G1.2 晚餐控糖 [habit]
## G2 完成博客系统重构 [active] (judge: 重构 PR 全部合并)
```

### 5.2 独立判官（防"乐观停止"，MiMo /goal 思想）

hermes cron 周期 job（每周日晚）：
1. 读 goals.md + 调 blog-agent 工具拉实际数据（exercise 记录、todo 完成率）
2. **判官 prompt**：只回答"判据是否客观达成"，不允许"差不多就算完成"
3. 评审结果写 journal + 更新 goals.md 状态
4. `deliver: origin → cortana-agent`，由 cortana 决定用什么语气播报（达成→庆祝表情；落后→鼓励而非指责）

### 5.3 目标设定入口

- 对话式：用户对 hermes 说"我想三个月减 4kg"→ hermes 拆解为目标树（追问判据）→ 写入 goal 工具 + goals.md
- Flutter 管家面板：目标树 UI 直接增删改

---

## 6. 提醒系统

复用 hermes cron（已有 origin 回传机制），统一三类提醒：

| 类型 | 创建方式 | 触发链路 |
|------|---------|---------|
| 显式提醒 | 用户说"明早 9 点提醒我交报告"→ hermes 调 cronjob 工具建 job | cron 到点 → deliver origin → cortana 表达层 → TTS 播报 |
| 习惯提醒 | 目标树中的 habit 节点自动生成（如 G1.1 → 周三未锻炼则提醒） | 周期 job 检查数据后有条件触发 |
| 情境提醒 | cortana 监控循环发现异常（待办积压、连续 3 天没锻炼） | 现有 proactive 链路 + 记忆去重（"昨天已提醒过"则跳过） |

关键改进：**所有提醒在播报前过 cortana 的打扰控制 + 记忆查重**，避免轰炸——这是"管家"和"闹钟"的区别。

## 7. 安慰与陪伴（养成系统）

1. **情绪感知**：hermes/llm 对话后把"用户情绪观察"写 journal（如"今天加班到 11 点，语气低落"）
2. **共情播报**：cortana 决策 prompt 注入最近 7 天情绪轨迹——低落期收紧提醒、改为安慰；高峰期可以活泼
3. **养成度（0-100）**：交互频率 + 正反馈 − 负反馈，存 companion_state；影响：播报语气亲密度、主动频率阈值、Flutter 形象徽章
4. **反馈闭环（MiMo"进化"）**：Flutter 播报卡片加反馈按钮「有帮助 / 太频繁 / 时机不对」→ 回传 cortana 调冷却参数 + 写 MEMORY.md（"用户不喜欢午休时间被打扰"）

## 8. Flutter 端改造

| 模块 | 内容 | 基础 |
|------|------|------|
| 管家面板（新 Tab） | 目标树（进度环+判据）、今日 checkpoint、提醒列表、养成度 | 复用 codegen 面板模式 |
| 记忆审阅页 | 查看/编辑/删除 MEMORY.md 与 journal（管家透明化） | 经 blog-agent API |
| 播报反馈 | 播报卡片三按钮反馈 | 现有 cortana 播报 UI |
| 形象增强 | 表情扩充（comfort/cheer/proud/sleepy）、按养成度换装饰 | 现有 Live2D 链路 |
| 对话输入 | 已有（上期完成 agent 切换 + 命令面板） | — |

## 9. 实施路线（按依赖排序）

| 阶段 | 内容 | 改动点 | 工作量 |
|------|------|--------|--------|
| **P1 记忆基建** | blog-agent memory MCP 工具 + HTTP API；hermes 系统提示词加记忆规范 | blog-agent 新增 1 文件；hermes 配置 | 小 ✅ |
| **P2 cortana 接记忆** | 快照注入记忆检索结果；播报后写 journal；提醒查重 | cortana-agent + llm-agent 的 cortana_proactive | 中 ✅ |
| **P3 目标树+判官** | goal 判据字段；hermes 周评审 cron job；对话式目标拆解 | blog-agent goal 工具 + hermes job 模板 | 中 ✅ |
| **P4 提醒统一** | 显式/习惯/情境三类提醒走统一链路 + 打扰控制 | hermes cronjob 工具暴露 + cortana | 中 ✅ |
| **P5 Flutter 管家面板** | 目标树/记忆审阅/提醒/反馈按钮/养成度 | Flutter 新 Tab + blog-agent API | 大 ✅（提醒列表视图暂缓） |
| **P6 进化优化** | 反馈调参、记忆周整理 job、多候选决策（可选） | 各组件小改 | ✅（多候选可选，暂缓） |

每阶段独立可用：P1+P2 完成后管家就"有记性"了；P3 后目标管理闭环；P5 后体验完整。

> 实现备注（P3–P6）：
> - **P3**：goal 判据采用约定写法（overview/description 的「判据:」段，不改数据 schema）；`hermes-agent/butler.py` 含独立判官 `GOAL_REVIEW_JOB`（周日评审）与对话式拆解规范（BUTLER_RULES 第 2 条）。
> - **P4**：显式提醒走 cron（BUTLER_RULES 第 3 条）；习惯提醒新增 `HABIT_REMINDER_JOB`（每日检查 + 记忆查重，落下才提醒）；情境提醒走 P2 的 cortana 链路。三类均经 deliver=origin → cortana 表达层做打扰控制。
> - **P5**：`app-agent/butler.go` 代理记忆/目标工具，`butler_feedback.go` 新增反馈与养成度端点；Flutter 管家 Tab 含目标环、checkpoint、记忆审阅、养成度与反馈按钮。
> - **P6**：反馈闭环采用「写记忆驱动收敛」——负反馈写入 MEMORY，经 cortana 快照注入（rule 25）与每周判官长期收敛主动性，比直接改冷却参数更可审阅（贴合 MiMo 透明化）；记忆周整理见 `MEMORY_CONSOLIDATION_JOB`；多候选决策为可选项，暂缓。

## 10. 风险与边界

- **隐私**：记忆文件含敏感生活信息——保持在私有部署 blog-agent 内，沿用 account 隔离 + delegation token 鉴权；记忆审阅页让用户随时删除
- **打扰疲劳**：所有主动行为必须过 cortana 冷却 + 记忆查重，宁可少说不刷存在感
- **记忆膨胀**：journal 只保留 90 天，周整理 job 负责提炼归档
- **LLM 成本**：判官/整理类 job 低频（周级）；播报决策维持现有单轮；多候选模式只在情绪敏感场景启用
