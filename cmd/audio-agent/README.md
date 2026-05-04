# audio-agent

推荐配置：

- `SpeechToText`: `openai/default`
- `TextToSpeech`: `minimax/default`

原因：

- 当前仓库已经接入 OpenAI 的语音转文本 HTTP 接口
- 当前仓库已经接入 MiniMax 的文本转语音同步 HTTP 接口
- 当前仓库已经接入 MiniMax 的音乐生成同步 HTTP 接口

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
- 如需限免模型，可把 `providers.minimax.music_generation_models.default.model` 改成 `music-2.6-free`

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

### TextToMusic

- provider: `minimax`
- model: `music-2.6`
- endpoint: `https://api.minimaxi.com/v1/music_generation`
- 默认 `lyrics_optimizer=true`，只传提示词时由 MiniMax 根据提示词生成歌词

## 工具

- `AudioToText`: 输入 `audio_base64`，返回识别文本
- `TextToAudio`: 输入 `text`，返回 `audio_base64`
- `TextToMusic`: 输入 `prompt`，可选 `lyrics` / `is_instrumental`，返回可播放的 `audio_base64`
