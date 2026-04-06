# audio-agent

推荐配置：

- `SpeechToText`: `openai/default`
- `TextToSpeech`: `minimax/default`

原因：

- 当前仓库已经接入 OpenAI 的语音转文本 HTTP 接口
- 当前仓库已经接入 MiniMax 的文本转语音同步 HTTP 接口
- MiniMax 的公开文档里，当前明确可用的是 TTS HTTP；仓库里没有接入独立 STT API

## 快速开始

1. 生成配置：

```bash
cd cmd/audio-agent
./audio-agent -genconf -config audio-agent.json
```

2. 编辑 `audio-agent.json`：

- 填写 `server_url`
- 填写 `auth_token`
- 填写 `providers.openai.api_key`
- 填写 `providers.minimax.api_key`

3. 启动：

```bash
cd cmd/audio-agent
./audio-agent -config audio-agent.json
```

## 当前默认模型

### SpeechToText

- provider: `openai`
- model: `gpt-4o-mini-transcribe`

### TextToSpeech

- provider: `minimax`
- model: `speech-2.8-hd`
- voice: `female-tianmei`
- endpoint: `https://api.minimaxi.com/v1/t2a_v2`

## 工具

- `AudioToText`: 输入 `audio_base64`，返回识别文本
- `TextToAudio`: 输入 `text`，返回 `audio_base64`
