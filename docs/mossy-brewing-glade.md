## 计划

将已完成的 MCP 工具分析文档输出到 `cmd/llm-agent/docs/MCP-TOOL-OPTIMIZATION.md`

---

# MCP 工具高效使用指南 — 原理分析与 llm-agent 优化实践

> 基于 Anthropic 官方博客《Code Execution with MCP》和《Code Execution Tool》的深度解读，
> 结合 `cmd/llm-agent` 实际代码的系统性分析。

---

## 目录

1. [MCP 工具使用的两大效率瓶颈](#一mcp-工具使用的两大效率瓶颈)
2. [Code Execution 模式：核心原理与数据流](#二code-execution-模式核心原理与数据流)
3. [Claude API code_execution 工具详解](#三claude-api-code_execution-工具详解)
4. [llm-agent 现状分析](#四llm-agent-现状分析)
5. [差距分析与优化方向](#五差距分析与优化方向)
6. [可落地的优化方案](#六可落地的优化方案)

---

## 一、MCP 工具使用的两大效率瓶颈

MCP (Model Context Protocol) 是连接 AI Agent 与外部系统的开放标准。随着一个 Agent 连接的工具数量从几个增长到几十甚至上百个，两个核心效率问题逐渐暴露。

### 1.1 瓶颈一：工具定义占满上下文窗口

**问题描述**：大多数 MCP 客户端在启动时将**所有工具的完整定义**（包含名称、描述、参数 schema）一次性加载到 LLM 的上下文中。

**数据量估算**：
- 每个工具定义约 200-500 tokens（含参数 JSON Schema）
- 20 个工具 ≈ 4,000-10,000 tokens
- 100 个工具 ≈ 20,000-50,000 tokens
- 1000 个工具 ≈ 200,000-500,000 tokens

**后果**：
- LLM 在读到用户问题之前，先消耗大量 tokens 处理工具定义
- 增加首次响应延迟（TTFT）
- 增加 API 调用成本（按 input tokens 计费）
- 可能逼近甚至超过模型的上下文窗口限制

**示例**：一个工具定义的典型结构：
```json
{
  "type": "function",
  "function": {
    "name": "CodegenStartSession",
    "description": "启动一个代码生成会话，在指定项目中执行代码编写任务",
    "parameters": {
      "type": "object",
      "properties": {
        "project_name": {
          "type": "string",
          "description": "目标项目名称"
        },
        "task_description": {
          "type": "string",
          "description": "需要执行的代码任务描述"
        },
        "language": {
          "type": "string",
          "enum": ["go", "python", "javascript"],
          "description": "编程语言"
        }
      },
      "required": ["project_name", "task_description"]
    }
  }
}
```

即使 LLM 本次任务只需要 1 个工具，它也必须"阅读"全部 20 个工具的完整定义。

### 1.2 瓶颈二：中间结果反复穿越模型上下文

**问题描述**：在传统 MCP 工具调用模式中，每次工具调用的结果必须回到 LLM 的上下文窗口，LLM 读取后才能决定下一步操作。

**典型场景 — 文档搬运**：

> 用户指令："把 Google Drive 中的会议记录附加到 Salesforce 的客户信息中"

传统调用流程：
```
第1轮 LLM 调用:
  输入: [system_prompt + 工具定义 + 用户问题]
  输出: "调用 gdrive.getDocument(id: 'abc123')"

工具执行:
  → 返回 50,000 字符的会议记录全文

第2轮 LLM 调用:
  输入: [system_prompt + 工具定义 + 用户问题 + 第1轮assistant消息 + 50,000字符工具结果]
  输出: "调用 salesforce.updateRecord(data: {Notes: '50,000字符的会议记录...'})"
  ↑ LLM 把完整会议记录写进了工具调用参数（又消耗输出 tokens）

工具执行:
  → 返回更新成功
```

**问题**：
- 50,000 字符的会议记录在 LLM 上下文中出现了 **至少 2 次**（作为工具结果 + 作为下一个工具的参数）
- LLM 的角色只是"数据搬运工"——它不需要理解会议内容，只需要把 A 的输出传给 B
- 对于 2 小时的会议记录，这可能意味着多消耗 **10万+ tokens**
- 超大文档可能超过上下文窗口限制，直接导致任务失败
- LLM 在复制大量数据时更容易出错（遗漏、截断、格式损坏）

---

## 二、Code Execution 模式：核心原理与数据流

### 2.1 核心思想

> **让 LLM 不再充当数据搬运工。**

Code Execution 模式的本质是：LLM 不再逐个调用工具，而是**写一段代码**来编排多个工具调用。工具结果存在代码变量中，而不是 LLM 的上下文窗口中。

### 2.2 数据流对比：传统模式 vs Code Execution 模式

#### 传统模式（Direct Tool Calling）

```
┌─────────────────────────────────────────────────────────────┐
│                    LLM 上下文窗口                             │
│                                                             │
│  第1轮: [system + tools定义 + user问题]                       │
│  →  LLM输出: "调用工具A"                                     │
│                                                             │
│  第2轮: [system + tools定义 + user问题                        │
│          + assistant消息 + 工具A结果(50,000字符)]             │
│  →  LLM输出: "调用工具B(参数=工具A的结果)"                    │
│          ↑ 数据在上下文中出现了 2 次                           │
│                                                             │
│  第3轮: [所有之前的内容 + 工具B结果]                           │
│  →  LLM输出: "完成，结果是..."                                │
│                                                             │
│  总 token 消耗: 巨大（数据反复穿越）                           │
│  总 LLM 调用次数: 3 次                                        │
│  总延迟: 3次LLM推理 + 2次工具执行                              │
└─────────────────────────────────────────────────────────────┘
```

#### Code Execution 模式

```
┌────────────────────────────────────────┐
│           LLM 上下文窗口                │
│                                        │
│  第1轮: [system + user问题]             │
│  →  LLM输出: 一段代码脚本 (200 tokens)   │
│                                        │
│  第2轮: [代码执行结果摘要 (50 tokens)]    │
│  →  LLM输出: "完成，结果是..."           │
│                                        │
│  总 token 消耗: 极少                     │
│  总 LLM 调用次数: 1-2 次                 │
│  总延迟: 1次LLM推理 + 代码执行时间        │
└────────────────────────────────────────┘

┌────────────────────────────────────────┐
│        代码执行沙箱（独立于LLM）         │
│                                        │
│  result_a = call_tool("工具A", {...})   │
│  // result_a 在变量中，50,000字符        │
│  // 不进入 LLM 上下文！                  │
│                                        │
│  filtered = process(result_a)           │
│  // 可以过滤、转换、聚合                  │
│                                        │
│  call_tool("工具B", {data: filtered})   │
│  // 数据直接从变量传给工具B               │
│                                        │
│  print("更新了 5 条记录")                │
│  // 只有这一行回到 LLM 上下文             │
└────────────────────────────────────────┘
```

### 2.3 关键澄清：code_execution 的目的不是"换个地方调用工具"

**常见误解**：code_execution 把代码放在 Claude 服务器端运行，但工具调用的结果不管在哪里调用都是一样的，那意义何在？

**正确理解**：code_execution 的目的不是改变工具的执行结果，而是改变**数据的流动路径**：

| 维度 | 传统模式 | Code Execution 模式 |
|------|---------|-------------------|
| 中间数据存储位置 | LLM 上下文窗口（消耗 tokens） | 代码变量（零 token 成本） |
| 数据处理能力 | LLM 逐步"思考"处理（每步一轮推理） | 代码直接过滤/转换/聚合 |
| 多工具编排 | 每个工具调用需一轮 LLM 推理 | 一段代码批量执行 N 个工具调用 |
| 工具间传参 | LLM 读取结果A → 写入工具B参数 | 代码变量直接传递 |
| 出错概率 | LLM 复制大数据时可能遗漏/出错 | 代码精确传递，不会丢失数据 |

**一句话总结**：工具在哪里执行不重要，重要的是工具的**结果**不需要经过 LLM 的上下文窗口就能到达下一个工具。

### 2.4 Code Execution 的五大收益

#### 收益一：渐进式工具加载（Progressive Disclosure）

不再全量加载所有工具定义。工具以文件系统形式呈现，LLM 按需浏览：

```
servers/
├── google-drive/
│   ├── getDocument.ts      ← LLM 需要时才读取这个文件
│   ├── listFiles.ts
│   └── index.ts
├── salesforce/
│   ├── updateRecord.ts
│   └── index.ts
└── ...
```

LLM 先 `ls servers/` 看有哪些服务，再读取需要的工具文件。从加载全部工具的 150,000 tokens 降为按需加载的 2,000 tokens — **节省 98.7%**。

#### 收益二：上下文高效的结果处理

工具返回 10,000 行数据时：
```javascript
const allRows = await gdrive.getSheet({ sheetId: 'abc123' });
// 10,000 行在变量中，不进入 LLM 上下文

const pending = allRows.filter(row => row["Status"] === 'pending');
console.log(`Found ${pending.length} pending orders`);
console.log(pending.slice(0, 5)); // 只打印前 5 行给 LLM 看
```
LLM 看到的只是 "Found 42 pending orders" + 5 行样例数据，而不是 10,000 行原始数据。

#### 收益三：更强大的控制流

用代码实现循环、条件判断、错误重试，替代多轮 LLM→工具往返：
```javascript
// 一段代码 = 替代了 N 轮 LLM 推理
let found = false;
while (!found) {
  const messages = await slack.getChannelHistory({ channel: 'C123456' });
  found = messages.some(m => m.text.includes('deployment complete'));
  if (!found) await sleep(5000);
}
console.log('Deployment notification received');
```

#### 收益四：隐私保护

敏感数据（邮箱、电话、姓名）在代码变量中流转，可被自动脱敏后才（如果需要的话）展示给 LLM：
```javascript
// LLM 看到的是:
[{ email: '[EMAIL_1]', phone: '[PHONE_1]', name: '[NAME_1]' }]
// 但实际传给工具B的是真实数据（在沙箱中自动还原）
```

#### 收益五：状态持久化与技能复用

Agent 可以把常用操作保存为可复用的函数（Skills）：
```javascript
// 第一次: Agent 编写并保存
// ./skills/save-sheet-as-csv.ts

// 以后: Agent 直接调用已有的 skill
import { saveSheetAsCsv } from './skills/save-sheet-as-csv';
```

---

## 三、Claude API code_execution 工具详解

### 3.1 工具定义与使用

Claude API 提供的 `code_execution_20250825` 是一个内置工具类型，提供 Bash 命令执行和文件操作能力：

```json
{
  "type": "code_execution_20250825",
  "name": "code_execution"
}
```

启用后，Claude 自动获得两个子工具：
- `bash_code_execution`: 执行 Shell 命令
- `text_editor_code_execution`: 查看、创建、编辑文件

### 3.2 执行环境

- **运行位置**: Anthropic 服务器端的安全沙箱容器
- **操作系统**: Linux 容器
- **资源限制**: 5GiB 内存 + 5GiB 磁盘 + 1 CPU
- **网络**: 完全禁用（无法访问外部网络）
- **容器复用**: 同一容器可跨多个 API 请求保持状态（通过 container ID）
- **过期**: 容器创建后 30 天过期

### 3.3 Programmatic Tool Calling — 核心机制

这是 code_execution 最关键的能力。通过 `allowed_callers` 参数，让 Claude 在沙箱中写代码来调用自定义工具：

```python
response = client.messages.create(
    model="claude-opus-4-6",
    max_tokens=4096,
    messages=[
        {"role": "user", "content": "获取5个城市的天气，找出最暖的"}
    ],
    tools=[
        {"type": "code_execution_20250825", "name": "code_execution"},
        {
            "name": "get_weather",
            "description": "获取城市天气",
            "input_schema": {...},
            "allowed_callers": ["code_execution_20250825"],  # 关键！
        },
    ],
)
```

**执行流程**：
```
1. Claude 在沙箱中写代码:
   ┌──────────────────────────────────────────┐
   │ results = []                              │
   │ for city in ["北京","上海","广州","深圳","杭州"]: │
   │   weather = call_tool("get_weather",      │
   │                        {"city": city})     │
   │   results.append(weather)                 │
   │ warmest = max(results, key=lambda x:      │
   │              x["temperature"])             │
   │ print(f"最暖的城市: {warmest['city']}")     │
   └──────────────────────────────────────────┘

2. 沙箱遇到 call_tool → 转发回客户端
3. 客户端执行真正的 get_weather API 调用
4. 结果返回沙箱 → 存入代码变量
5. 循环 5 次，结果都在代码变量中
6. 只有 print() 的输出回到 Claude 上下文
```

**对比传统方式**：
- 传统: 5 轮 LLM 推理（每次决定调用哪个城市的天气），5 个工具结果全部进入上下文
- Programmatic: 1 轮 LLM 推理（写代码），5 个工具结果只在代码变量中，LLM 只看到最终结论

### 3.4 适用性与限制

| 特性 | 说明 |
|------|------|
| 支持模型 | Claude 全系列（Opus/Sonnet/Haiku 4.x+） |
| API 平台 | Anthropic API、Azure AI Foundry（不支持 Bedrock/Vertex） |
| 计费 | 与 web_search/web_fetch 搭配时免费；否则按执行时间计费（最低 5 分钟） |
| 免费额度 | 每组织 1,550 小时/月 |
| 沙箱网络 | 完全禁用 — 沙箱代码无法直接访问外部 API |
| 工具执行位置 | 工具仍在客户端执行，沙箱只是编排层 |

**关键限制**：沙箱没有网络，所以它不能直接调用 MCP Server 的 HTTP API。所有工具调用都是通过 API 协议转发回客户端执行的。这意味着 code_execution 本质上是一个**客户端编排代理**，而不是独立的执行引擎。

### 3.5 深入理解：沙箱如何在没有网络的情况下获取工具数据？

沙箱中的代码**并不真正执行**工具调用。`call_tool()` 不是一个 HTTP 请求，而是一个 **暂停信号（yield）**——它让代码暂停执行，把工具调用请求通过 API 协议交还给你的客户端程序。

**Claude API 在这里扮演的角色就是一个转发代理**：

```
沙箱代码  ←──转发代理(Claude API)──→  你的客户端(MCP Client)  ──→  MCP Server
```

**完整数据流（以5个城市天气为例）**：

```
你的客户端程序                    Claude API                     沙箱容器
     │                              │                              │
     │  1. 发送 messages + tools    │                              │
     │ ──────────────────────────►  │                              │
     │                              │  2. Claude 决定写代码         │
     │                              │ ────────────────────────────► │
     │                              │                              │ 3. 执行代码...
     │                              │                              │    遇到 call_tool("get_weather",
     │                              │                              │                    {city:"北京"})
     │                              │                              │    ──► 代码暂停！
     │                              │  4. 沙箱把调用请求传回API      │
     │  5. API 返回 response        │ ◄──────────────────────────── │
     │     stop_reason: "tool_use"  │                              │
     │ ◄──────────────────────────  │                              │
     │     内容: {tool: "get_weather", input: {city: "北京"}}       │
     │                              │                              │
     │  6. 你的客户端执行            │                              │
     │     get_weather("北京")      │                              │
     │     → 调用真正的天气API       │                              │
     │     → 得到结果: {temp: 22}   │                              │
     │                              │                              │
     │  7. 把结果发回 Claude API     │                              │
     │     tool_result: {temp: 22}  │                              │
     │ ──────────────────────────►  │                              │
     │                              │  8. 结果注入沙箱变量          │
     │                              │ ────────────────────────────► │
     │                              │                              │ 9. 代码继续执行...
     │                              │                              │    weather = {temp: 22}  ← 变量已赋值
     │                              │                              │    继续循环下一个城市...
     │                              │                              │    遇到 call_tool("get_weather",
     │                              │                              │                    {city:"上海"})
     │                              │                              │    ──► 代码又暂停！
     │                              │                              │
     │         ... 重复步骤 5-9，共 5 次 ...                        │
     │                              │                              │
     │                              │                              │ 10. 所有循环完成
     │                              │                              │     print("最暖: 广州 28°C")
     │                              │  11. 沙箱返回最终输出         │
     │  12. API 返回 final response │ ◄──────────────────────────── │
     │      stop_reason: "end_turn" │                              │
     │      内容: "最暖的是广州28°C" │                              │
     │ ◄──────────────────────────  │                              │
```

**关键点**：

1. `call_tool()` 本质上是一个 `yield`（挂起）操作
2. Claude API 把请求转发给你的客户端（`stop_reason: "tool_use"`）
3. 你的客户端在自己的环境中执行真正的工具调用
4. 结果发回 Claude API → 注入沙箱变量 → 代码从暂停处恢复
5. **沙箱不需要网络**——通过 API 请求-响应协议间接获取数据

**与传统模式的本质区别**：

```
传统模式:
  客户端 → API → Claude说"调用工具A" → 客户端执行 → 结果进LLM上下文(tokens!)
                 Claude读取结果 → 说"调用工具B" → 客户端执行 → 结果又进上下文(tokens!)

  数据路径: 工具A结果 →→→ LLM上下文 →→→ 工具B     （经过LLM，消耗tokens）

Code Execution:
  客户端 → API → 沙箱代码 call_tool("A") → 暂停 → 客户端执行 → 结果注入沙箱变量
                 沙箱代码继续,变量传给 call_tool("B") → 暂停 → 客户端执行
                 沙箱 print(摘要) → 摘要进入LLM上下文

  数据路径: 工具A结果 →→→ 沙箱变量 →→→ 工具B     （不经过LLM，零token成本）
```

**网络请求次数其实是一样的**（都是客户端执行工具），区别在于工具结果是存入 **LLM 上下文**（消耗 tokens）还是存入**沙箱代码变量**（零成本）。Claude API 这个代理只是多了一个**选择性暴露**的能力——沙箱代码决定哪些数据 `print()` 给 LLM 看，哪些只存在变量里。

### 3.6 LLM 如何生成 code_execution 代码？流程变化时怎么办？

#### 代码生成方式

LLM 生成代码的依据和生成 `tool_call` 的依据是一样的——**基于工具定义（schema）和用户意图**。区别只是输出格式：

```
传统模式 LLM 输出:
  tool_call: get_weather({city: "北京"})

Code Execution 模式 LLM 输出:
  代码: weather = call_tool("get_weather", {city: "北京"})
        print(weather["temp"])
```

LLM 不需要"特殊能力"来写这种代码——它本来就擅长写代码。工具的 schema 告诉它参数格式，它就能生成对应的调用代码。

#### 流程需要改变时的处理

分两种情况：

**情况一：可预见的分支 — 代码能处理（不需要重写）**

如果 LLM 事先能预判可能的结果，它会在代码中写好条件分支：

```python
result = call_tool("query_order", {id: "123"})

if result["status"] == "shipped":
    tracking = call_tool("get_tracking", {id: result["tracking_id"]})
    print(f"已发货，物流单号: {tracking['number']}")
elif result["status"] == "pending":
    call_tool("send_reminder", {order_id: "123"})
    print("已发送催单提醒")
else:
    print(f"订单状态: {result['status']}，需人工处理")
```

**情况二：不可预见的结果 — 需要新一轮 LLM 推理（代码会重写）**

如果工具返回了完全出乎预料的结果，代码无法处理：

```
第1轮:
  LLM 写代码 → 沙箱执行 → call_tool() → 客户端执行 → 结果注入沙箱
  → 代码 print() 出关键信息或报错
  → API 返回给客户端

第2轮:
  LLM 看到 print() 的输出 → 重新理解情况 → 写新的代码片段
  → 沙箱执行新代码（容器复用，之前的文件和状态还在）
```

**重写的成本很低**，因为：
1. LLM 只看到 `print()` 的摘要信息（几十 tokens），不是原始大数据
2. 沙箱容器可以复用（container ID），之前代码写入的文件、变量状态还在
3. 新代码可以读取之前保存的中间结果文件

#### 实际工作模式：渐进式代码生成

真实场景中，code_execution **不是一次性生成一个完美脚本**，而是多轮渐进式代码生成：

```
┌──────────────────────────────────────────────────────┐
│  LLM 推理轮次1:                                       │
│    写代码: 查询数据 + 过滤 + print(摘要)                │
│    → 沙箱执行 → LLM 看到摘要                           │
│                                                       │
│  LLM 推理轮次2:                                       │
│    根据摘要，写代码: 处理数据 + 调用下一批工具            │
│    → 沙箱执行 → LLM 看到新的摘要                        │
│                                                       │
│  LLM 推理轮次3:                                       │
│    写最终代码: 汇总 + 输出                               │
└──────────────────────────────────────────────────────┘
```

**和传统模式的关键对比**：

| | 传统模式 | Code Execution |
|--|---------|---------------|
| 每轮 LLM 看到的 | 工具完整原始结果 | 代码 `print()` 的摘要 |
| 每轮能做的事 | 调用 1 个工具 | 执行一段代码（可含 N 个工具调用 + 过滤 + 循环） |
| 需要新一轮 LLM 的条件 | 每次工具调用后都需要 | 只有遇到无法预判的情况才需要 |

**总结**：code_execution 不是让 LLM 一次写出完美的全流程代码，而是让 LLM **每轮能做更多事、看更少数据**。可预见的分支在代码的 if/else 里处理，不可预见的结果才触发新一轮 LLM 推理，但 LLM 看到的依然只是摘要，token 开销远小于传统模式。

---

## 四、llm-agent 现状分析

### 4.1 架构概览

```
用户输入 (Web/企业微信)
       ↓
  Bridge (UAP Gateway 客户端)
       ↓
  processTask() — 任务入口
       ├─ 简单任务 → Agentic Loop（直接工具调用）
       └─ 复杂任务 → plan_and_execute:
            ① Plan    — LLM 分解任务为子任务 DAG
            ② Review  — LLM 验证计划质量
            ③ Execute — DAG 拓扑排序执行各子任务
            └─ Synthesize — LLM 整合所有结果
```

### 4.2 关键组件与文件

| 文件 | 职责 | 与 MCP 工具的关系 |
|------|------|----------------|
| `bridge.go` | UAP 客户端、工具发现、工具调用 | **工具定义加载 + 工具调用执行** |
| `processor.go` | 任务入口、简单/复杂路由、Agentic Loop | **工具结果处理 + 消息历史管理** |
| `orchestrator.go` | DAG 执行、子任务管理、异步检测 | **子任务间结果传递** |
| `planner.go` | 任务分解、计划审查、失败决策 | 工具提示（tools_hint）生成 |
| `llm_client.go` | LLM API 调用（同步/流式） | 工具定义序列化发送 |
| `config.go` | 配置管理 | 超时、迭代上限等参数 |
| `session.go` | 会话历史持久化 | 工具调用记录存储 |

### 4.3 工具生命周期详解

#### 阶段一：工具发现（bridge.go:84-184）

```go
// 每 60 秒执行
DiscoverTools()
  → GET /api/gateway/tools          // 获取所有在线 agent 的工具
  → 解析工具定义（name, description, parameters schema）
  → 构建 toolCatalog: map[toolName]agentID    // 路由表
  → 构建 llmTools: []LLMTool                  // LLM 函数调用格式
  → 工具名转换: "codegen.start_session" → "codegen_start_session"
```

**当前行为**：全量获取所有在线 agent 的全部工具定义，包含完整 parameters JSON Schema。

#### 阶段二：工具筛选（bridge.go:258-326, processor.go:164-196）

```go
// processor.go:192-196
if len(tools) > 15 && query != "" {
    tools = b.routeTools(query, tools)  // LLM 智能路由
}
```

`routeTools()` 实现：
1. 构建工具目录（仅名称 + 描述，不含 schema）
2. 用 LLM 根据用户问题选择相关工具名
3. 根据选中的名称加载完整工具定义

**已有优化**：
- 路由时只传名称+描述，节省 schema tokens
- "宁多勿少"策略，避免遗漏
- 路由失败时（LLM 调用出错、解析失败）返回 nil，让 LLM 直接回答

**未覆盖的场景**：
- 工具 ≤15 个时不触发路由，全量加载
- 路由本身消耗一次 LLM 调用

#### 阶段三：Agentic Loop（processor.go:222-400）

```
for i := 0; i < maxIter; i++ {           // maxIter = 32
    text, toolCalls, err := SendLLMRequest(messages, tools)

    if len(toolCalls) == 0 { break }      // 无工具调用 → 结束

    messages = append(messages, assistantMsg)

    for _, tc := range toolCalls {
        result := bridge.CallTool(tc.Name, tc.Args)     // 工具执行
        messages = append(messages, Message{             // 结果全量入 messages
            Role: "tool",
            Content: result,    // ← 原始结果，可能数千到数万字符
            ToolCallID: tc.ID,
        })
    }
}
```

**关键问题**：每轮迭代 messages 只增不减，工具结果原始全量进入上下文。

#### 阶段四：工具调用执行（bridge.go:344-399）

```go
CallTool(toolName, args)
  → 查找 toolCatalog 获取 agentID
  → 创建 pending channel
  → 通过 UAP WebSocket 发送 MsgToolCall
  → 等待 MsgToolResult（超时机制）

超时配置：
  普通工具: ToolCallTimeoutSec = 120s
  长时间工具 (Codegen, Deploy): LongToolTimeoutSec = 600s
```

#### 阶段五：子任务编排（orchestrator.go）

复杂任务通过 DAG 拓扑排序执行：
```
SubTask t1 (无依赖)
    ↓ 结果通过 siblingContext 传递
SubTask t2 (依赖 t1)
    ↓
SubTask t3 (依赖 t1, t2)
```

`buildSiblingContext()` 实现（orchestrator.go:942-961）：
```go
func buildSiblingContext(dependsOn []string, completedResults map[string]string) string {
    for _, depID := range dependsOn {
        result := completedResults[depID]
        if len(result) > 3000 {
            result = result[:3000] + "\n...(已截断)"  // 已有截断，但阈值较高
        }
        sb.WriteString(fmt.Sprintf("### 任务 %s 的结果:\n%s\n\n", depID, result))
    }
    return sb.String()
}
```

**已有优化**：siblingContext 对超过 3000 字符的结果做了截断。

---

## 五、差距分析与优化方向

### 5.1 总览对比

| 维度 | 博客推荐做法 | llm-agent 现状 | 差距评估 |
|------|------------|-------------------|---------|
| 工具定义加载 | 渐进式按需加载 | 全量加载 + >15时LLM路由 | **中** |
| 工具结果处理 | 在执行环境中过滤后再返回模型 | 原始结果全量进入 messages | **大** |
| 多工具编排 | 代码批量执行多工具 | 每个工具调用一轮LLM往返 | 中 |
| 工具间数据传递 | 代码变量直接传递，不经模型 | 全部经过模型上下文中转 | **大** |
| 会话历史管理 | 摘要化、滑动窗口 | 只增不减直到 maxIter(32) | **大** |
| 并行工具执行 | 无依赖的工具并行调用 | 顺序执行 | 低 |

### 5.2 差距详解与影响评估

#### 差距一：工具结果全量进入上下文（影响：大）

**代码位置**：`processor.go:392-398`, `orchestrator.go:457-463`

**问题**：工具返回的原始 JSON 全量作为 tool message 追加到 messages 中。例如 Codegen 工具可能返回数千字符的代码内容，查询工具可能返回完整数据列表。

**影响**：
- 后续每轮 LLM 调用都要重新发送全部历史 messages（含之前所有工具结果）
- 多轮迭代后 messages 累积可达数万 tokens
- 直接增加 API 成本和响应延迟

**对比博客建议**：博客指出应在执行环境中过滤数据，只将摘要返回模型。当前系统无此机制。

#### 差距二：消息历史只增不减（影响：大）

**代码位置**：`processor.go:222-400` 的 Agentic Loop

**问题**：messages 列表在循环中**只有 append，没有压缩**。直到 maxIter（默认32轮）才强制收敛（移除工具，要求文本总结）。

**影响**：
- 假设每轮工具结果 2000 字符，10 轮后 messages 累积 20,000+ 字符
- DeepSeek 按 input tokens 计费，每轮调用的 input 成本递增
- 长对话（如微信多轮）问题更加严重

#### 差距三：工具间数据必须经过 LLM 中转（影响：中，受限于LLM能力）

**问题**：当 LLM 需要用工具A的结果作为工具B的输入时，数据流是：
```
工具A结果 → LLM上下文 → LLM生成工具B参数(引用A的结果) → 工具B
```

**限制**：这是使用 DeepSeek API 的固有限制。DeepSeek 不支持 code_execution 类型工具，无法实现 Programmatic Tool Calling。

**应用层缓解方案**：优化工具端返回值 + 消息压缩，间接减少数据经过 LLM 的量。

---

## 六、可落地的优化方案

> 以下方案基于当前使用 DeepSeek API 的架构限制，不依赖 Claude 的 code_execution 能力，在**应用层**模拟博客推荐的核心理念。

### 方案 1：工具端返回值精炼（从源头减少 token）

**原理**：与其在 llm-agent 层截断工具结果（可能丢失关键信息），不如在各 MCP agent（codegen-agent、deploy-agent 等）的返回值设计上做优化，让工具本身返回精炼的结构化结果。

**优势**：这是最安全的优化方式——工具最清楚哪些信息是关键的，哪些是可以省略的。不存在"信息不完整"的风险。

**设计原则**：
```
工具返回值 = 状态信息 + 关键标识符 + 摘要 + (可选)详情链接

{
  "success": true,
  "status": "completed",
  "session_id": "sess_abc123",          ← 后续工具调用需要的标识符
  "summary": "已创建 calculator 项目，包含 main.go 和 go.mod",  ← 人可读摘要
  "files_created": ["main.go", "go.mod"],  ← 结构化关键数据
  // 不返回文件的完整内容！
}
```

**对比当前可能的返回值**：
```
{
  "success": true,
  "session_id": "sess_abc123",
  "output": "创建文件 main.go:\npackage main\n\nimport (\n\t\"fmt\"\n)\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n\n创建文件 go.mod:\nmodule calculator\n\ngo 1.21\n..."
  // output 包含了完整的文件内容，可能数千字符
}
```

**涉及文件**：各 MCP agent 的工具返回逻辑（非 llm-agent 本身）

### 方案 2：历史工具结果摘要替换（消息压缩）

**原理**：在 Agentic Loop 中，当 messages 累积量超过阈值时，对**历史**工具结果（非最近一轮的）进行摘要替换。最近一轮的结果保持完整，因为 LLM 可能正在基于它做决策。

**实现思路**：

```go
// 在每轮 LLM 调用前执行
func compactMessages(messages []Message, currentIter int) []Message {
    // 估算总字符量
    totalChars := 0
    for _, m := range messages {
        totalChars += len(m.Content)
    }

    // 未超阈值，不压缩
    if totalChars <= 16000 {  // 约 4000 tokens
        return messages
    }

    // 压缩策略：
    // 1. 始终保留: messages[0] (system prompt)
    // 2. 始终保留: 最后 4 条消息（最近一轮的 assistant + tool results）
    // 3. 中间的 tool 消息: 提取关键信息后替换
    //    原始: {"success": true, "session_id": "abc", "output": "...长文本..."}
    //    替换: "[工具 X 返回成功, session_id=abc, 详情已省略]"
}
```

**关键设计决策**：
- 压缩只影响传给 LLM 的 messages 副本
- SessionStore 中保留完整历史（用于调试和审计）
- 结构化结果（JSON）：提取 `success`、`status`、`session_id` 等关键字段
- 纯文本结果：保留前 200 字符作为摘要

**涉及文件**：`processor.go`（简单任务循环）, `orchestrator.go`（子任务循环）

### 方案 3：子任务间结果智能传递

**现状**：`buildSiblingContext()` 已有 3000 字符截断（orchestrator.go:955-957），这是合理的。

**优化方向**：不是简单截断，而是**智能提取**——从前置任务结果中提取后续任务真正需要的关键信息：

```go
func buildSmartSiblingContext(dependsOn []string, completedResults map[string]string) string {
    for _, depID := range dependsOn {
        result := completedResults[depID]

        // 尝试解析为 JSON，提取关键字段
        var parsed map[string]interface{}
        if json.Unmarshal([]byte(result), &parsed) == nil {
            // 提取标识符类字段
            keys := extractKeyFields(parsed, []string{
                "session_id", "project_name", "task_id", "status", "message",
            })
            sb.WriteString(fmt.Sprintf("### 任务 %s 的结果:\n%s\n\n", depID, keys))
        } else {
            // 纯文本：截断
            if len(result) > 1500 {
                result = result[:1500] + "\n...(已截断)"
            }
            sb.WriteString(fmt.Sprintf("### 任务 %s 的结果:\n%s\n\n", depID, result))
        }
    }
}
```

**涉及文件**：`orchestrator.go`

### 方案 4：工具定义分层加载（渐进式披露）

**原理**：模拟博客提出的文件系统式工具发现。LLM 初始只看到工具的名称和一句话描述（不含完整 schema），需要调用某个工具时再获取完整参数定义。

**实现方式**：注入一个虚拟工具 `describe_tool`：

```go
// 第一层：所有工具的摘要注入 system prompt
"你可以使用以下工具（调用前请先用 describe_tool 查看参数格式）：
1. CodegenStartSession - 启动代码生成会话
2. CodegenSendMessage - 在会话中发送消息
3. DeployProject - 部署项目
4. RawCurrentDate - 获取当前日期
..."

// 第二层：LLM 调用 describe_tool 获取完整 schema
describe_tool("CodegenStartSession")
→ 返回完整的 parameters JSON Schema
```

**权衡**：
- 优势：初始 context 从 O(N × schema_size) 降为 O(N × 一行)
- 劣势：增加 1 轮 LLM→工具往返（LLM 需要先查 schema 再调用工具）
- 适用场景：工具数 > 20 时收益显著

**涉及文件**：`bridge.go`, `processor.go`

### 方案 5：多工具并行执行

**现状**：LLM 一次返回多个 tool_calls 时，当前代码顺序执行（processor.go:358-399 的 for 循环）。

**优化**：对无依赖关系的工具调用并行执行：

```go
// 并行执行工具调用
var wg sync.WaitGroup
type toolResult struct {
    result  string
    err     error
    success bool
}
results := make([]toolResult, len(toolCalls))

for i, tc := range toolCalls {
    wg.Add(1)
    go func(idx int, tc ToolCall) {
        defer wg.Done()
        r, e := b.CallTool(unsanitizeToolName(tc.Function.Name),
                          json.RawMessage(tc.Function.Arguments))
        results[idx] = toolResult{result: r, err: e, success: e == nil}
    }(i, tc)
}
wg.Wait()

// 按原始顺序处理结果
for i, tc := range toolCalls {
    // 使用 results[i]...
}
```

**效果**：当 LLM 同时调用 3 个独立工具时，总耗时从 3x 降为 max(1x)。

**涉及文件**：`processor.go`, `orchestrator.go`

### 方案 6：execute-code-agent — 本地实现 Code Execution 能力

Claude 的 code_execution 沙箱本质上并不复杂——它就是一个转发代理 + 代码执行环境。我们完全可以构建一个本地的 `execute-code-agent`，让**任意 LLM**（DeepSeek、GPT、Qwen 等）都能使用 Code Execution 模式高效调用 MCP 工具。

#### 6.1 为什么必须用代码沙箱？

曾考虑过用"结构化批量调用协议"（JSON 定义工具调用链）代替代码沙箱，但这个方案有一个致命缺陷——**无法对数据进行处理**。

```
结构化协议:   工具A → 原始数据(10,000行) → 还是得全部回到LLM上下文   ← 没解决问题
代码沙箱:     工具A → 原始数据(10,000行) → 代码过滤为5行 → 5行回到LLM  ← 解决了问题
```

code_execution 的灵魂是**代码可以处理数据**——过滤、转换、聚合、计算。砍掉代码执行能力就砍掉了全部意义，和并行调用工具返回原始数据没有区别。

#### 6.2 沙箱语言选择：Python

| 维度 | Python | Go | JavaScript |
|------|--------|----|-----------|
| LLM 生成代码简洁度 | 最简洁 | 较啰嗦（package/import/func main/类型断言） | 简洁 |
| 启动延迟 | ~50ms | ~500ms-1s（需编译） | ~100ms |
| 数据处理生态 | 最强（list comprehension、dict 操作） | 较弱 | 中等 |
| 错误处理 | try/except 一行 | if err != nil 每次都要写 | try/catch |
| LLM 训练数据中的脚本占比 | 最高 | 低 | 中 |

Go 做沙箱语言的代码对比：

```go
// Go 版本 — LLM 需要生成的代码
package main
import "fmt"
func main() {
    result, _ := callTool("query_order", map[string]interface{}{"id": "123"})
    status := result["status"].(string)
    if status == "shipped" {
        tracking, _ := callTool("get_tracking", map[string]interface{}{
            "id": result["tracking_id"].(string),
        })
        fmt.Printf("已发货，物流: %s\n", tracking["number"])
    }
}
```

```python
# Python 版本 — LLM 需要生成的代码
result = call_tool("query_order", {"id": "123"})
if result["status"] == "shipped":
    tracking = call_tool("get_tracking", {"id": result["tracking_id"]})
    print(f"已发货，物流: {tracking['number']}")
```

**结论**：execute-code-agent 本身用 Go 实现（与项目一致），但沙箱子进程用 Python（LLM 生成代码更简洁高效）。这和 Claude 的做法一样——Claude 的 API 服务不是 Python 写的，但沙箱容器里跑的是 Python/Bash。

#### 6.3 整体架构

```
DeepSeek LLM
    │
    │  tool_call: execute_code({code: "...", tools_hint: [...]})
    ▼
llm-agent (现有)
    │
    │  通过 UAP 调用 execute-code-agent 的工具
    ▼
execute-code-agent (新建，Go 实现)
    │
    │  启动 Python 子进程，注入 call_tool() 桥接函数
    ▼
Python 子进程（沙箱）
    │  代码执行...
    │  遇到 call_tool("get_weather", {"city": "北京"})
    │  ──► 写特殊格式到 stdout
    ▼
execute-code-agent（读取 stdout，识别工具调用请求）
    │
    │  通过 UAP Gateway 调用真正的 MCP 工具
    │  等待结果返回
    │  将结果写入子进程 stdin
    ▼
Python 子进程（从 stdin 读取结果，赋值给变量，代码继续执行）
    │
    │  数据处理: filtered = [x for x in data if x["status"] == "pending"]
    │  print(f"找到 {len(filtered)} 条待处理记录")  ← 只有这行最终返回
    ▼
execute-code-agent → 返回 stdout 给 llm-agent → 进入 LLM 上下文
```

#### 6.4 关键组件实现

**组件一：注入到 Python 沙箱的 `call_tool()` 桥接函数**

```python
# 在子进程启动时自动注入，用户代码之前执行
import sys, json

def call_tool(tool_name, arguments=None):
    """调用 MCP 工具 — 通过 stdin/stdout 协议与 agent 通信"""
    request = json.dumps({
        "type": "tool_call",
        "tool": tool_name,
        "args": arguments or {}
    })
    # 写请求到 stdout（agent 在监听）
    print(f"__TOOL_CALL__{request}__END__", flush=True)

    # 从 stdin 读取结果（agent 注入）
    line = sys.stdin.readline().strip()
    result = json.loads(line)
    if not result.get("success"):
        raise Exception(f"Tool {tool_name} failed: {result.get('error')}")
    return result.get("data")

# ===== 以下是 LLM 生成的用户代码 =====
```

**组件二：execute-code-agent 的核心逻辑（Go 实现）**

```go
func (a *Agent) executeCode(code string) (string, error) {
    // 1. 拼接桥接代码 + 用户代码
    fullCode := bridgeCode + "\n" + code

    // 2. 启动 Python 子进程（带资源限制）
    ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, "python3", "-c", fullCode)
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    cmd.Start()

    // 3. 逐行读取 stdout
    scanner := bufio.NewScanner(stdout)
    var output strings.Builder

    for scanner.Scan() {
        line := scanner.Text()

        if strings.Contains(line, "__TOOL_CALL__") {
            // 4. 解析工具调用请求
            toolCall := parseToolCallFromLine(line)

            // 5. 通过 UAP 调用真正的 MCP 工具
            result, err := a.callMCPTool(toolCall.Tool, toolCall.Args)

            // 6. 将结果写入子进程 stdin
            response, _ := json.Marshal(map[string]interface{}{
                "success": err == nil,
                "data":    result,
                "error":   errorString(err),
            })
            stdin.Write(append(response, '\n'))
        } else {
            // 7. 普通 print 输出 → 收集为最终返回值
            output.WriteString(line + "\n")
        }
    }

    cmd.Wait()
    return output.String(), nil  // 只返回 print() 的内容
}
```

**组件三：llm-agent 的改造**

在 system prompt 中引导 LLM 优先使用代码执行模式：

```
当你需要连续调用多个工具，或需要对工具返回的数据进行过滤/转换/聚合时，
优先使用 execute_code 工具编写 Python 代码。

代码中使用 call_tool(tool_name, args) 调用 MCP 工具。
只有 print() 的输出会返回给你，中间数据不会进入对话历史。

可用工具列表（在代码中通过 call_tool 调用）：
1. codegen.start_session(project_name, task_description) - 启动代码生成会话
2. deploy.project(project_name) - 部署项目
3. raw.current_date() - 获取当前日期
...
```

#### 6.5 与 Claude code_execution 的对比

| 维度 | Claude code_execution | execute-code-agent |
|------|----------------------|-------------------|
| 沙箱位置 | Anthropic 服务器 | 本地（Python 子进程） |
| 通信协议 | API 请求-响应 | stdin/stdout |
| 转发代理 | Claude API | execute-code-agent + UAP |
| 适用 LLM | 仅 Claude | **任意 LLM**（DeepSeek/GPT/Qwen/...） |
| 网络隔离 | 完全禁用 | 可配置 |
| 安全性 | Anthropic 托管 | 需自行沙箱化 |
| 容器复用 | 30天有效 | 按需配置 |
| 额外成本 | 按执行时间计费 | **免费**（本地执行） |

#### 6.6 安全性考虑

LLM 生成的代码可能包含危险操作，需要防护：

| 风险 | 防护措施 |
|------|---------|
| 无限循环 | `context.WithTimeout` 设置执行超时（如 120s） |
| 内存耗尽 | 通过 cgroup 或 Docker 限制内存上限 |
| 文件系统破坏 | 限制工作目录为临时目录，只读挂载系统目录 |
| 网络滥用 | 可选：通过 Docker 网络策略禁用外部访问 |
| 恶意代码 | 可选：Docker 容器化完全隔离 |

**最小方案**（快速上线）：`context.WithTimeout` + 临时工作目录
**完整方案**（生产环境）：Docker 容器化 + 资源限制 + 网络隔离

#### 6.7 核心价值

> **把 Claude 的专有能力变成通用能力，任意 LLM 都能用 Code Execution 模式高效调用 MCP 工具。**

---

## 附录 A：实施优先级矩阵

| 优先级 | 方案 | 预期收益 | 实施复杂度 | 信息丢失风险 |
|--------|------|---------|-----------|------------|
| **P0** | 工具端返回值精炼 | 从源头减少 30-50% token | 中（需改各 agent） | **无** |
| **P1** | 历史工具结果摘要替换 | 防止长对话 token 爆炸 | 中 | **低** |
| **P1** | 子任务结果智能传递 | 减少 siblingContext 开销 | 低 | **低** |
| **P2** | execute-code-agent | 架构级提升，任意 LLM 可用 | 中高 | **无** |
| **P2** | 工具定义分层加载 | 减少初始 context 占用 | 中 | **无** |
| **P2** | 多工具并行执行 | 减少工具执行总耗时 | 低 | **无** |

## 附录 B：验证方法

1. **Token 消耗监控**：在 `SendLLMRequest` 中记录每次请求的 messages 总字符数，对比优化前后
2. **延迟监控**：记录 `processTask` 总耗时和各阶段耗时
3. **功能回归测试**：
   - 简单任务："查询今天的运动记录" — 验证工具返回值精炼后 LLM 仍能正确解读
   - 复杂任务："创建一个计算器项目并部署" — 验证消息压缩后子任务链仍能正确执行
   - 长对话：微信多轮对话 15+ 轮 — 验证历史压缩不影响对话连贯性
   - 代码执行："查询所有待处理订单并按金额排序" — 验证 execute-code-agent 的数据过滤能力
4. **A/B 测试**：对同一批任务，分别用传统模式和 Code Execution 模式处理，对比 token 消耗和成功率
