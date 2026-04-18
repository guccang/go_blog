# Flutter Cortana 页面优化说明

本文档总结本轮 `Flutter Cortana` 页面与 `llm-agent` 协同链路的优化内容，重点覆盖：

- 文本发送到 `llm-agent` 的回复链路
- 语音自动播放与回退策略
- Live2D 表情/动作协议与前端消费方式
- 口型同步策略
- 当前边界与后续扩展建议

涉及的核心文件：

- `cmd/flutter-client-for-appagent/flutter_client_for_appagent/lib/cortana_page.dart`
- `cmd/flutter-client-for-appagent/flutter_client_for_appagent/lib/main.dart`
- `cmd/flutter-client-for-appagent/flutter_client_for_appagent/assets/cortana/index.html`
- `cmd/llm-agent/app_chat.go`
- `cmd/llm-agent/app_chat_test.go`

## 1. 目标

本轮优化目标不是做“完整语音助手平台”，而是先把 Cortana 页的主路径打通，并把协议层留出足够扩展空间：

1. Flutter Cortana 页面继续使用文字输入。
2. 文字消息发送到 `llm-agent`。
3. 如果后端返回语音富消息，前端自动播放。
4. 播放时驱动 Live2D 表情、动作、口型。
5. 如果后端没有返回结构化动作计划，前端仍能使用本地规则回退。
6. 如果后端没有返回语音，前端仍能本地 TTS 兜底。

## 2. 优化前问题

优化前主要存在以下问题：

- Cortana 页面虽然能“说话”，但语音并不是来自 `llm-agent` 的语音富消息，而是 Flutter 本地再次调用 `/api/tts`。
- Cortana 动作规划依赖第二次 LLM 请求，页面会额外再问一次“动作规划提示词”，增加了时延和不确定性。
- `_sendCortanaMessage()` 只拿文本消息，无法优先等待语音富消息。
- 进度消息和最终回复没有严格区分，容易误把“思考中”“任务完成”等系统提示当作最终答案。
- Live2D 动作名和页面实现耦合较重，不利于后续更换模型或增加动作。
- 口型同步仅使用随机振幅，观感不稳定。

## 3. 当前链路

当前 Cortana 页面链路如下：

1. 用户在 Cortana 页面输入文本。
2. Flutter 调用 `onSendMessage`，最终通过 `main.dart` 走 `sendAppMessage(...)`。
3. 请求附带：
   - `input_mode = cortana_text`
   - `reply_mode = audio_preferred`
4. `llm-agent` 检测到该模式后：
   - 优先走语音回复路径
   - 在 system prompt 中追加 Cortana 专用输出协议
5. 如果模型按协议输出了 `[CORTANA_ACTION_PLAN]`，后端会提取该结构化动作计划。
6. 如果后端合成了语音，语音富消息会带上：
   - `speech_text`
   - `audio_format`
   - `cortana_action_plan`
7. Flutter Cortana 页面优先等待语音富消息。
8. 收到语音后自动播放，并根据 `cortana_action_plan` 驱动表情和动作。
9. 如果没有动作计划，前端按本地规则生成动作计划。
10. 如果没有语音，前端回退到本地 TTS。

## 4. Flutter 侧优化

### 4.1 CortanaReplyPayload

为了支持“文本 + 可选语音 + 可选动作计划”的统一传递，新增了 `CortanaReplyPayload`：

- `text`
- `audioPath`
- `audioFormat`
- `actionPlan`

这样 Cortana 页面不再只接收一个字符串，而是能同时拿到：

- 最终展示/口播文本
- 自动播放的本地音频路径
- 可选的结构化表情动作计划

### 4.2 Cortana 消息等待策略

`main.dart` 中 `_sendCortanaMessage()` 的行为进行了调整：

- 发送时不再走简单 `sendMessage()`，而是走 `sendAppMessage(...)` 并显式附带 `cortana_text` 标记。
- 等待回复时会过滤掉进度消息，例如：
  - `收到消息`
  - `思考:`
  - `工具调用:`
  - `任务完成:`
- 优先等待 `audio` 类型消息。
- 如果先收到文本，会短暂等待语音；若超时仍无语音，则回退使用文本。

这使 Cortana 页面具备了“语音优先、文本兜底”的行为。

### 4.3 语音自动播放

`cortana_page.dart` 中 `_speak()` 的播放逻辑变为：

1. 发消息拿到 `CortanaReplyPayload`
2. 确定最终动作计划
3. 先设置表情
4. 再调度动作
5. 启动口型同步
6. 若有 `audioPath` 则直接播放语音文件
7. 否则走本地 TTS 回退

当前默认优先使用后端语音，只有后端未返回语音时才走：

- `http://blog.guccang.cn:10086/api/tts`

### 4.4 本地动作规划回退

当后端没有提供 `cortana_action_plan` 时，Flutter 本地会根据文本内容生成默认动作计划：

- 问候优先 `IdleWave`
- 道歉类优先 `sad`
- 强调类优先 `surprised` + `Tap`
- 长文本在 `Idle / IdleAlt / Tap` 间进行轻量编排

这保证了即使模型暂时没输出动作协议，Cortana 仍然可以正常演示。

### 4.5 Live2D 语义映射层

为了后续更换模型，前端引入了“语义动作/表情 -> 当前模型动作/表情”的别名映射层。

当前支持的语义表情：

- `happy`
- `sad`
- `surprised`

当前支持的语义动作：

- `Idle`
- `IdleAlt`
- `IdleWave`
- `Tap`

并兼容一些别名，例如：

- `Greeting -> IdleWave`
- `Explain -> IdleAlt`
- `TapBody -> Tap`

这层的作用是：

- 模型只需要输出语义动作名
- Flutter/JS 决定如何映射到当前 Live2D 模型的真实动作组

## 5. Live2D 页面优化

### 5.1 JS 桥接接口

当前页面暴露的核心接口包括：

- `window.setExpression(name)`
- `window.setMotion(group, index)`
- `window.startLipSync(amp)`
- `window.stopLipSync()`

新增了：

- `window.setExpressionFor(name, holdMs, fallbackName)`

这个接口可以实现：

- 先切换一个短暂表情
- 保持一段时间
- 再自动回落到基础表情

适用于：

- 惊讶后回到平静
- 轻微开心后回到日常表情
- 强调句后的表情收束

### 5.2 动作调度增强

Flutter 前端现在能识别动作中的更多字段：

- `motion`
- `delay`
- `index`
- `hold_ms`
- `resume_to_idle`

其中：

- `index` 用于指定动作组变体
- `hold_ms` 用于描述动作持续窗口
- `resume_to_idle` 用于表达动作结束后自动回待机

这让动作协议具备了更明确的执行语义，而不是只靠一个动作名 + 时间点。

## 6. llm-agent 侧优化

### 6.1 Cortana 专用输出协议

`llm-agent` 在识别到 `input_mode = cortana_text` 时，会为该轮 App 会话追加 Cortana 专用 system prompt，要求模型在正文后输出：

```text
[CORTANA_ACTION_PLAN]
{
  "speech_text": "...",
  "expression": "happy",
  "fallback_expression": "happy",
  "expression_hold_ms": 1600,
  "mood": "warm",
  "actions": [
    {"motion": "IdleWave", "delay": 0, "index": 0, "hold_ms": 1800},
    {"motion": "Tap", "delay": 1800, "index": 0, "hold_ms": 900, "resume_to_idle": true},
    {"motion": "Idle", "delay": 3600, "index": 0}
  ]
}
```

该协议只对 Cortana 入口生效，不会污染普通 App / 微信 / Web 对话。

### 6.2 动作计划提取

后端当前支持从模型输出中提取两类结构：

1. 直接标签块：

```text
[CORTANA_ACTION_PLAN]
{ ... }
```

2. fenced json / cortana 代码块：

```text
```json
{ ... }
```
```

提取后会：

- 清理正文中的动作计划块
- 将 `speech_text` 与正文对齐
- 把动作计划透传到语音富消息的 `meta.cortana_action_plan`

### 6.3 语音富消息透传

当 `llm-agent` 发送音频富消息时，当前 `meta` 中可包含：

- `audio_base64`
- `audio_format`
- `input_mode = tts_reply`
- `speech_text`
- `cortana_action_plan`

这样前端收到一条语音消息时，不仅能播放，还能直接做角色编排。

### 6.4 兼容 TextToAudio 工具输出

如果未来 `TextToAudio` 工具本身直接返回：

- `cortana_action_plan`
- 或顶层 `expression / actions`

当前后端也会尝试透传，不要求再改中间协议。

## 7. 口型同步优化

### 7.1 优化前

优化前的口型同步只是：

- 每 120ms 生成一个随机振幅
- 调 `window.startLipSync(amp)`

问题是：

- 同一句话每次播放口型都不同
- 停顿与强调没有节奏
- 与文本内容没有关系

### 7.2 当前实现

当前实现改为“基于播放进度 + 文本节奏”的确定性曲线：

1. 根据文本长度估算总时长
2. 根据标点和分段生成口型 profile
3. 监听播放器位置流 `onPositionChanged`
4. 按当前位置映射到 profile 上计算 mouth amplitude

优点：

- 同一句话的口型节奏可复现
- 标点停顿会影响开合节奏
- 观感明显稳定于随机方案

当前仍然不是“真实音频能量驱动”，只是一个更稳定的过渡方案。

## 8. 当前支持的动作协议

前端当前可消费的 `cortana_action_plan` 字段如下：

### 顶层字段

- `speech_text`
- `expression`
- `fallback_expression`
- `expression_hold_ms`
- `mood`
- `actions`

### actions[] 字段

- `motion`
- `delay`
- `index`
- `hold_ms`
- `resume_to_idle`

### 示例

```json
{
  "speech_text": "今天我先帮你梳理重点，然后我们一步一步处理。",
  "expression": "happy",
  "fallback_expression": "happy",
  "expression_hold_ms": 1400,
  "mood": "calm",
  "actions": [
    {"motion": "IdleWave", "delay": 0, "index": 0, "hold_ms": 1600},
    {"motion": "Tap", "delay": 1700, "index": 0, "hold_ms": 700, "resume_to_idle": true},
    {"motion": "IdleAlt", "delay": 2900, "index": 0}
  ]
}
```

## 9. 测试与验证

已完成的验证包括：

- `cd cmd/llm-agent && go test ./...`
- `cd cmd/flutter-client-for-appagent/flutter_client_for_appagent && flutter analyze lib/cortana_page.dart lib/main.dart`

并增加了 `llm-agent` 回归测试，验证：

- `[CORTANA_ACTION_PLAN]` 提取
- fenced json 提取
- 非法 payload 不误伤正文
- Cortana 输出协议提示词包含关键字段

## 10. 当前边界

当前版本已经能支撑较稳定的展示链路，但仍有明确边界：

- 口型仍不是基于真实音频能量/viseme
- `mood` 字段已保留，但前端当前仅透传，未做单独演出逻辑
- 当前 Live2D 模型的动作组仍然比较少，语义动作虽然已抽象，但可发挥空间仍受模型资源限制
- 表情回退现在基于简单 `hold_ms`，尚未实现更复杂的状态机

## 11. 后续建议

建议优先级如下：

1. 将 `motionMap` / `expressionMap` 抽为配置文件，降低模型切换成本。
2. 接入真实音频振幅或 viseme，替换当前文本驱动口型。
3. 为 `mood` 增加前端演出策略，例如：
   - `warm`
   - `calm`
   - `alert`
   - `playful`
4. 增加动作中断/覆盖规则，解决长回复中多个动作冲突问题。
5. 在 Live2D 资源扩充后，加入更多语义动作名，例如：
   - `Nod`
   - `Explain`
   - `Listen`
   - `Celebrate`

## 12. 总结

本轮优化的核心价值在于把 Cortana 从“页面本地拼装的演示逻辑”推进成了“前后端有明确协议的可扩展角色播放链路”：

- 消息链路上，支持 `llm-agent` 语音优先回复。
- 协议层上，支持结构化动作计划透传。
- 前端层上，支持自动播放、动作消费、表情回退和本地兜底。
- 动画层上，支持比随机方案更稳定的口型同步。

这意味着后续继续扩展 Live2D 能力时，主要工作会从“重写链路”变成“丰富协议 + 增加资源 + 调整映射”，整体改造成本已经明显下降。
