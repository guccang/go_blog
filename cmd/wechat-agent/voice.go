package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"uap"
)

const maxWeChatVoiceSize = 2 * 1024 * 1024

func (b *Bridge) sendNotifyPayload(payload uap.NotifyPayload) {
	if strings.EqualFold(strings.TrimSpace(payload.MessageType), "voice") {
		if err := b.sendAppVoiceMessage(payload.To, payload.Meta); err == nil {
			return
		} else {
			log.Printf("[Bridge] voice push failed for user=%s: %v, fallback to text", payload.To, err)
		}
	}

	content := strings.TrimSpace(payload.Content)
	if content == "" {
		content = "[voice reply unavailable]"
	}
	b.sendNotification(payload.To, content)
}

func (b *Bridge) sendAppVoiceMessage(toUser string, meta map[string]any) error {
	if !b.cfg.IsAppEnabled() {
		return fmt.Errorf("wechat app not configured")
	}

	voiceBytes, err := buildWeChatVoiceAudio(meta)
	if err != nil {
		return err
	}
	return b.sendAppVoiceMessageRetry(toUser, voiceBytes, true)
}

func (b *Bridge) sendAppVoiceMessageRetry(toUser string, voiceBytes []byte, allowRetry bool) error {
	token, err := b.getAccessToken()
	if err != nil {
		return fmt.Errorf("get token: %v", err)
	}

	mediaID, errCode, err := b.uploadVoiceMedia(token, voiceBytes)
	if err != nil {
		if allowRetry && isTokenExpiredErrCode(errCode) {
			b.clearCachedToken()
			log.Println("[Bridge] token expired during voice upload, retrying...")
			return b.sendAppVoiceMessageRetry(toUser, voiceBytes, false)
		}
		return err
	}

	errCode, err = b.sendVoiceMessage(token, toUser, mediaID)
	if err != nil {
		if allowRetry && isTokenExpiredErrCode(errCode) {
			b.clearCachedToken()
			log.Println("[Bridge] token expired during voice send, retrying...")
			return b.sendAppVoiceMessageRetry(toUser, voiceBytes, false)
		}
		return err
	}

	log.Printf("[Bridge] WeChat voice message sent to %s", toUser)
	return nil
}

func buildWeChatVoiceAudio(meta map[string]any) ([]byte, error) {
	audioBytes, audioFormat, err := extractVoiceReply(meta)
	if err != nil {
		return nil, err
	}

	if isAMRData(audioBytes) {
		if len(audioBytes) > maxWeChatVoiceSize {
			return nil, fmt.Errorf("amr voice exceeds 2MB limit")
		}
		return audioBytes, nil
	}

	converted, err := convertAudioToAMR(audioBytes, audioFormat)
	if err != nil {
		return nil, err
	}
	if len(converted) > maxWeChatVoiceSize {
		return nil, fmt.Errorf("amr voice exceeds 2MB limit after conversion")
	}
	if !isAMRData(converted) {
		return nil, fmt.Errorf("converted audio is not valid amr")
	}
	return converted, nil
}

func extractVoiceReply(meta map[string]any) ([]byte, string, error) {
	if len(meta) == 0 {
		return nil, "", fmt.Errorf("voice meta is required")
	}

	audioBase64 := stringMeta(meta, "audio_base64")
	if audioBase64 == "" {
		return nil, "", fmt.Errorf("voice meta missing audio_base64")
	}

	audioBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(audioBase64))
	if err != nil {
		return nil, "", fmt.Errorf("decode audio_base64: %w", err)
	}
	if len(audioBytes) == 0 {
		return nil, "", fmt.Errorf("voice payload is empty")
	}

	audioFormat := normalizeVoiceAudioFormat(stringMeta(meta, "audio_format"))
	if audioFormat == "" {
		audioFormat = "mp3"
	}
	return audioBytes, audioFormat, nil
}

func convertAudioToAMR(audioBytes []byte, sourceFormat string) ([]byte, error) {
	ffmpegPath, err := findFFmpegWithAMREncoder()
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "wechat-voice-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputFile := filepath.Join(tmpDir, "input."+sourceFileExt(sourceFormat))
	outputFile := filepath.Join(tmpDir, "output.amr")
	if err := os.WriteFile(inputFile, audioBytes, 0600); err != nil {
		return nil, fmt.Errorf("write temp input: %w", err)
	}

	commands := [][]string{
		{"-y", "-hide_banner", "-loglevel", "error", "-i", inputFile, "-ar", "8000", "-ac", "1", "-c:a", "libopencore_amrnb", "-b:a", "12.2k", outputFile},
		{"-y", "-hide_banner", "-loglevel", "error", "-i", inputFile, "-ar", "8000", "-ac", "1", "-c:a", "amr_nb", "-b:a", "12.2k", outputFile},
		{"-y", "-hide_banner", "-loglevel", "error", "-i", inputFile, "-ar", "8000", "-ac", "1", outputFile},
	}

	var lastErr error
	for _, args := range commands {
		_ = os.Remove(outputFile)
		cmd := exec.Command(ffmpegPath, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("ffmpeg convert failed: %w (%s)", err, strings.TrimSpace(string(output)))
			continue
		}

		converted, err := os.ReadFile(outputFile)
		if err != nil {
			lastErr = fmt.Errorf("read amr output: %w", err)
			continue
		}
		if !isAMRData(converted) {
			lastErr = fmt.Errorf("ffmpeg output is not amr")
			continue
		}
		return converted, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("ffmpeg conversion failed")
	}
	return nil, lastErr
}

func (b *Bridge) uploadVoiceMedia(token string, voiceBytes []byte) (string, int, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="media"; filename="voice.amr"; filelength=%d`, len(voiceBytes)))
	header.Set("Content-Type", "audio/amr")
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", 0, fmt.Errorf("create multipart part: %w", err)
	}
	if _, err := part.Write(voiceBytes); err != nil {
		return "", 0, fmt.Errorf("write voice data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", 0, fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=voice", token)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(url, writer.FormDataContentType(), &body)
	if err != nil {
		return "", 0, fmt.Errorf("upload voice media: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read upload response: %w", err)
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, fmt.Errorf("parse upload response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", result.ErrCode, fmt.Errorf("upload voice media error: %d %s", result.ErrCode, result.ErrMsg)
	}
	if strings.TrimSpace(result.MediaID) == "" {
		return "", 0, fmt.Errorf("upload voice media missing media_id")
	}

	return strings.TrimSpace(result.MediaID), 0, nil
}

func (b *Bridge) sendVoiceMessage(token, toUser, mediaID string) (int, error) {
	agentIDInt, _ := strconv.Atoi(b.cfg.AgentID)

	msg := map[string]any{
		"touser":  toUser,
		"msgtype": "voice",
		"agentid": agentIDInt,
		"voice": map[string]string{
			"media_id": mediaID,
		},
	}

	data, _ := json.Marshal(msg)
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("send voice message: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read voice send response: %w", err)
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("parse voice send response: %w", err)
	}
	if result.ErrCode != 0 {
		return result.ErrCode, fmt.Errorf("send voice message error: %d %s", result.ErrCode, result.ErrMsg)
	}

	return 0, nil
}

func (b *Bridge) clearCachedToken() {
	b.tokenMu.Lock()
	b.cachedToken = ""
	b.tokenExpireAt = time.Time{}
	b.tokenMu.Unlock()
}

func isTokenExpiredErrCode(code int) bool {
	return code == 40014 || code == 42001
}

func isAMRData(data []byte) bool {
	return len(data) >= 6 && bytes.Equal(data[:6], []byte("#!AMR\n"))
}

func normalizeVoiceAudioFormat(format string) string {
	format = strings.TrimSpace(strings.ToLower(format))
	format = strings.TrimPrefix(format, ".")
	switch format {
	case "mpeg":
		return "mp3"
	case "x-wav", "wave":
		return "wav"
	default:
		return format
	}
}

func sourceFileExt(format string) string {
	format = normalizeVoiceAudioFormat(format)
	if format == "" {
		return "bin"
	}
	return format
}

func findFFmpegWithAMREncoder() (string, error) {
	candidates := []string{
		"/usr/local/opt/ffmpeg-full/bin/ffmpeg",
		"/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg",
	}

	if path, err := exec.LookPath("ffmpeg"); err == nil {
		candidates = append(candidates, path)
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if ffmpegHasAMREncoder(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no ffmpeg with AMR-NB encoder found; install ffmpeg-full or provide amr audio upstream")
}

func ffmpegHasAMREncoder(ffmpegPath string) bool {
	output, err := exec.Command(ffmpegPath, "-hide_banner", "-encoders").CombinedOutput()
	if err != nil {
		return false
	}
	text := string(output)
	return strings.Contains(text, "libopencore_amrnb") || strings.Contains(text, " amr_nb")
}

func stringMeta(meta map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := meta[key]
		if !ok {
			continue
		}
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
			return text
		}
	}
	return ""
}
