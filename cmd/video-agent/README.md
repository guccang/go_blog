# video-agent

`video-agent` 通过 UAP 接入 MiniMax Hailuo 视频生成能力，支持文生视频和图生视频。

## 配置

```bash
cd cmd/video-agent
./video-agent -genconf -config video-agent.json
```

编辑 `video-agent.json`，至少填写：

- `providers.minimax.api_key`
- 按需调整 `video_generation.model`，默认 `default` 为 `MiniMax-Hailuo-2.3`

## 运行

```bash
cd cmd/video-agent
./video-agent -config video-agent.json
```

## 工具

- `TextToVideo`: 输入 `prompt`，返回 MP4 的 `video_base64`
- `ImageToVideo`: 输入 `prompt` 和 `first_frame_image`，返回 MP4 的 `video_base64`

工具结果包含 `app_message`，可直接传给 `app.SendRichMessage` 推送到 Flutter：

```json
{
  "message_type": "video",
  "content": "视频说明",
  "meta": {
    "video_base64": "...",
    "video_format": "mp4",
    "file_name": "output_aigc.mp4",
    "file_format": "mp4",
    "mime_type": "video/mp4"
  }
}
```
