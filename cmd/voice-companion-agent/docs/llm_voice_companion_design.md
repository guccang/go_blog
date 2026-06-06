# LLM 语音陪伴助手程序设计

## 1. 目标

设计一个基于 LLM 的语音陪伴助手，能够运行在 PC 和手机端，为用户提供自然语音对话、长期陪伴、日常提醒、情绪支持和轻量任务协助。

本设计先定义后端服务、端侧能力、协议和迭代路径，不在本阶段直接实现完整代码。

## 2. 产品定位

语音陪伴助手不是单纯的语音命令入口，而是一个持续存在的个人陪伴体：

1. 能记住用户偏好、习惯、重要事项和近期状态。
2. 能在合适时机主动问候或提醒，但不打扰用户。
3. 能以语音为主、文本为辅完成自然对话。
4. 能运行在 PC 和手机端，并保持账号级上下文一致。
5. 能明确区分陪伴、任务执行、信息查询和系统控制的边界。

## 3. 总体架构

```text
┌─────────────────────────────────────────────────────────┐
│                    PC / Mobile Client                    │
│  麦克风  播放器  唤醒词  UI 状态  本地缓存  通知权限        │
└───────────────────────┬─────────────────────────────────┘
                        │ WebSocket / HTTP
┌───────────────────────▼─────────────────────────────────┐
│                 app-agent / gateway                      │
│  账号认证  设备注册  消息转发  推送通道  多端连接管理        │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│              voice-companion-agent                       │
│  会话状态机  LLM 编排  记忆编排  主动关怀  语音响应编排       │
└───────┬───────────────┬────────────────┬────────────────┘
        │               │                │
┌───────▼──────┐ ┌──────▼──────┐ ┌───────▼────────────────┐
│ audio-agent  │ │ llm-agent   │ │ persistence / Redis     │
│ STT / TTS    │ │ 对话和工具   │ │ 画像 记忆 会话 事件      │
└──────────────┘ └─────────────┘ └────────────────────────┘
```

### 3.1 端侧职责

PC 和手机端负责低延迟、强交互和平台权限相关能力：

- 麦克风采集。
- 本地唤醒词或前台连续监听。
- 音频播放和播放状态回传。
- 用户打断、静音、重听、取消。
- 前台 UI 状态展示。
- 手机通知、PC 托盘或桌面通知。
- 弱网时的文本兜底和重试。

### 3.2 后端职责

`voice-companion-agent` 负责陪伴助手的核心行为：

- 管理一轮语音对话的状态机。
- 调用 `audio-agent` 完成 STT 和 TTS。
- 调用 `llm-agent` 完成回复、工具选择和动作规划。
- 维护短期上下文、长期记忆和用户画像。
- 决定是否主动关怀、何时推送、用什么语气。
- 对 PC 和手机端输出统一事件协议。

## 4. 核心模块

### 4.1 Session Manager

管理设备连接和会话生命周期。

主要职责：

- 为每个账号维护当前活跃会话。
- 区分 PC、手机和 Web 设备。
- 管理用户是否在线、是否可被打扰、是否处于语音播放中。
- 支持同账号多端接续，例如手机发起、PC 继续。

建议核心结构：

```go
type CompanionSession struct {
    AccountID     string
    DeviceID      string
    DeviceType    string
    State         SessionState
    LastActiveAt  time.Time
    CurrentTurnID string
}
```

### 4.2 Voice State Machine

语音交互使用显式状态机，避免语音识别、LLM 回复和音频播放互相抢状态。

```text
Idle
  ↓
ListeningWakeWord
  ↓
RecordingUserSpeech
  ↓
Transcribing
  ↓
Thinking
  ↓
Speaking
  ↓
Idle
```

额外状态：

- `Interrupted`：用户打断播放。
- `Muted`：用户暂停语音输出。
- `Retrying`：网络或供应商失败后重试。
- `FallbackTextOnly`：语音不可用时只返回文本。

### 4.3 LLM Orchestrator

LLM 编排层不是简单转发消息，而是把陪伴目标、上下文、记忆和工具结果组织为一次可控请求。

输入：

- 当前用户文本。
- 语音元信息，例如音量、语速、打断次数。
- 近期对话摘要。
- 用户画像和长期记忆。
- 当前设备和场景，例如 PC 工作中、手机夜间。

输出：

- `reply_text`：用于展示和 TTS 的最终回复。
- `reply_style`：语气，例如温和、简短、鼓励、提醒。
- `memory_ops`：需要写入、更新或忽略的记忆。
- `next_action`：是否设置提醒、打开工具、继续追问。
- `safety_level`：是否触发安全降级。

### 4.4 Memory Manager

记忆分三层，避免把所有内容都塞进上下文。

| 类型 | 生命周期 | 示例 | 存储建议 |
| --- | --- | --- | --- |
| 短期上下文 | 当前会话 | 刚刚聊到的主题 | 内存 + Redis |
| 近期摘要 | 数小时到数天 | 最近在准备考试 | Redis / DB |
| 长期记忆 | 长期 | 喜欢安静语气、每周跑步 | DB + 可审计记录 |

记忆写入需要可解释：

```json
{
  "account_id": "u_001",
  "memory_type": "preference",
  "content": "用户更喜欢简短、直接的回复",
  "source_turn_id": "turn_20260527_001",
  "confidence": 0.86,
  "created_at": "2026-05-27T10:00:00+08:00"
}
```

### 4.5 Proactive Care Engine

主动关怀需要严格限流，默认保守。

触发来源：

- 用户设置的提醒。
- 长时间未互动后的轻问候。
- 重要日程前的准备提醒。
- 用户近期状态中的明确诉求，例如希望被督促喝水。

限制规则：

- 夜间默认不主动语音打扰。
- 每日主动消息数量有上限。
- 用户可关闭所有主动关怀。
- 涉及健康、金融、法律等高风险主题时只做低风险提示，不替代专业建议。

## 5. 端侧方案

### 5.1 手机端

手机端优先复用现有 Flutter 客户端。

推荐能力：

- 前台唤醒词监听。
- 按住说话和点击说话。
- 后台只做通知，不默认常驻录音。
- 使用系统音频焦点处理播放和打断。
- 上传短音频片段到后端 STT。
- 接收 `audio` 富消息并自动播放。

手机端第一阶段不做锁屏常驻唤醒，避免权限、电量和隐私风险扩大。

### 5.2 PC 端

PC 端可以按两种形态实现：

1. 桌面托盘程序：常驻、低打扰、适合陪伴和提醒。
2. Web 页面：开发快，适合验证语音对话主链路。

推荐能力：

- Push-to-talk 快捷键。
- 托盘状态：待机、聆听、思考、播放。
- 可选前台唤醒词。
- 本地录音缓存和失败重传。
- 播放时支持 Esc 或快捷键打断。

## 6. 统一消息协议

客户端和 `voice-companion-agent` 使用事件流协议。底层可走 WebSocket，HTTP 适合作为兜底。

### 6.1 客户端上行事件

```json
{
  "event": "voice.input.completed",
  "account_id": "u_001",
  "device_id": "phone_001",
  "turn_id": "turn_001",
  "audio_ref": "upload://audio/turn_001.m4a",
  "locale": "zh-CN",
  "input_mode": "push_to_talk"
}
```

文本兜底：

```json
{
  "event": "text.input",
  "account_id": "u_001",
  "device_id": "pc_001",
  "turn_id": "turn_002",
  "text": "我今天有点累，陪我聊会儿"
}
```

用户打断：

```json
{
  "event": "voice.playback.interrupted",
  "account_id": "u_001",
  "device_id": "phone_001",
  "turn_id": "turn_001"
}
```

### 6.2 后端下行事件

状态事件：

```json
{
  "event": "assistant.state",
  "turn_id": "turn_001",
  "state": "thinking"
}
```

文本回复：

```json
{
  "event": "assistant.reply.text",
  "turn_id": "turn_001",
  "text": "听起来你今天消耗不少。要不要先说说最累的是哪一段？"
}
```

语音回复：

```json
{
  "event": "assistant.reply.audio",
  "turn_id": "turn_001",
  "text": "听起来你今天消耗不少。要不要先说说最累的是哪一段？",
  "audio_ref": "download://audio/turn_001.mp3",
  "audio_format": "mp3",
  "voice": "warm_female"
}
```

记忆提示：

```json
{
  "event": "assistant.memory.candidate",
  "turn_id": "turn_001",
  "summary": "用户最近下班后容易疲惫，希望助手用更轻柔的语气陪伴",
  "requires_user_confirm": true
}
```

## 7. 配置设计

建议配置文件为 `voice-companion-agent.json`。

```json
{
  "server_url": "http://127.0.0.1:10086",
  "auth_token": "",
  "audio_agent_url": "http://127.0.0.1:10087",
  "llm_agent_url": "http://127.0.0.1:10088",
  "redis_addr": "127.0.0.1:6379",
  "default_locale": "zh-CN",
  "voice": {
    "stt_provider": "openai/default",
    "tts_provider": "minimax/default",
    "default_voice": "female-tianmei",
    "enable_text_fallback": true
  },
  "companion": {
    "enable_memory": true,
    "enable_proactive_care": false,
    "max_daily_proactive_messages": 3,
    "quiet_hours_start": "22:30",
    "quiet_hours_end": "08:00"
  }
}
```

## 8. 安全和隐私

必须默认遵循以下规则：

1. 录音只在用户明确触发或开启前台唤醒时采集。
2. 长期记忆默认可查看、可删除、可关闭。
3. 主动关怀默认不在夜间发语音。
4. 高风险内容必须降级为支持性沟通和求助建议。
5. 不把 API Key、Token、私有配置写入仓库。
6. 端侧上传音频前应带账号和设备鉴权。

## 9. 日志和可观测性

日志需要能还原完整语音链路，但不能记录敏感原始音频。

建议记录：

- 会话创建和关闭。
- 设备连接和断开。
- 状态机迁移。
- STT、LLM、TTS 调用耗时。
- 打断、重试、降级和失败原因。
- 记忆候选生成、确认和删除。

不建议默认记录：

- 原始音频文件。
- 未脱敏的长期私密信息。
- API Key、Token、Cookie。

## 10. 失败回退

| 失败点 | 回退策略 |
| --- | --- |
| STT 失败 | 提示用户重说，保留本轮 turn_id |
| LLM 超时 | 返回简短安抚文本，允许稍后继续 |
| TTS 失败 | 只展示文本，并提示语音暂不可用 |
| WebSocket 断开 | 客户端切 HTTP 轮询或重新连接 |
| 记忆写入失败 | 不影响当前回复，后台重试 |
| 多端冲突 | 最近活跃设备优先，其他设备只同步文本 |

## 11. 实施阶段

### Phase 1：最小可用语音陪伴

1. 建立 `voice-companion-agent` 服务骨架。
2. 支持文本输入和语音输入转文本。
3. 调用 `llm-agent` 生成陪伴式回复。
4. 调用 `audio-agent` 生成语音。
5. 手机端和 PC 端均可完成一轮语音对话。

### Phase 2：会话状态和打断

1. 引入显式语音状态机。
2. 支持播放中打断。
3. 支持文本兜底。
4. 增加 WebSocket 事件流。
5. 增加基础日志和耗时统计。

### Phase 3：记忆和画像

1. 加入短期上下文摘要。
2. 加入长期记忆候选。
3. 提供用户确认和删除入口。
4. 将记忆注入 LLM 编排层。

### Phase 4：主动关怀

1. 加入低频主动问候。
2. 接入提醒和日程类事件。
3. 增加免打扰和频率控制。
4. 为 PC 托盘和手机通知做差异化策略。

## 12. 与现有仓库模块的关系

- `cmd/audio-agent`：复用 STT、TTS、音乐生成能力。
- `cmd/llm-agent`：复用对话、工具调用和模型编排能力。
- `cmd/app-agent`：复用账号、客户端消息和 Flutter 桥接能力。
- `cmd/cortana-agent`：可参考主动提醒、广播和账号注册逻辑。
- `cmd/flutter-client-for-appagent`：手机端优先复用现有 Flutter 客户端。
- `cmd/voice-input`：可参考端侧语音输入封装方式。

## 13. 首轮代码落地建议

首轮实现只做最小闭环，避免一开始就加入复杂主动关怀：

1. 新增 `cmd/voice-companion-agent/main.go` 和配置加载。
2. 提供 `/api/companion/text` 和 `/api/companion/voice` 两个入口。
3. `/api/companion/voice` 接收音频引用，先调用 STT，再复用文本链路。
4. 文本链路调用 `llm-agent`，得到 `reply_text`。
5. 如果客户端请求语音，调用 `audio-agent` 生成 `audio_ref`。
6. 返回统一响应：

```json
{
  "turn_id": "turn_001",
  "reply_text": "我在。你可以慢慢说。",
  "audio_ref": "download://audio/turn_001.mp3",
  "state": "completed"
}
```

这样可以先验证 PC 和手机端的主体验，再逐步增加状态机、记忆和主动关怀。

