# voice-companion-agent

`voice-companion-agent` 是面向 PC 和手机端的 LLM 语音陪伴助手设计目录。

当前目录先沉淀程序设计文档，不直接实现服务代码。目标是把语音输入、LLM 对话、长期记忆、情绪陪伴、语音输出、多端同步等职责拆清楚，后续再按阶段接入 `audio-agent`、`llm-agent`、`app-agent` 和 Flutter 客户端。

## 设计目标

1. 支持 PC 和手机端使用同一套后端会话能力。
2. 支持文本、按住说话、唤醒词、连续语音四种交互入口。
3. 通过 LLM 提供陪伴式对话，而不是一次性问答工具。
4. 支持用户画像、短期上下文、长期记忆和主动关怀。
5. 支持语音回复、打断、静音、重听和文本兜底。
6. 保持隐私可控，用户可以查看、禁用和删除记忆。

## 目录

- [docs/llm_voice_companion_design.md](docs/llm_voice_companion_design.md)：完整程序设计文档。

## 推荐服务边界

```text
PC / Mobile Client
        ↓
app-agent / gateway
        ↓
voice-companion-agent
        ↓
audio-agent + llm-agent + persistence
```

`voice-companion-agent` 不直接绑定某一种前端技术。PC 端可以是桌面客户端、Web 或命令行托盘程序；手机端优先复用现有 Flutter 客户端能力。

