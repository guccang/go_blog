package sms

import (
	"bytes"
	"core"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	http "net/http"
	"time"

	log "mylog"
)

// SmsActor holds SMS configuration and embeds the core Actor
type SmsActor struct {
	*core.Actor
	sendURL string
	name    string
	phone   string
}

// generateCode creates a 6-digit numeric verification code
func (a *SmsActor) generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("生成验证码失败: %v", err)
	}
	return fmt.Sprintf("%06d", n), nil
}

// sendVerificationCode sends the verification code to the configured endpoint
func (a *SmsActor) sendVerificationCode(code string) error {
	payload := map[string]interface{}{
		"name":    a.name,
		"code":    code,
		"targets": a.phone,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(a.sendURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("请求发送失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("JSON解析失败: %v", err)
	}

	// Basic success check if API returns {"status":"success"}
	if status, ok := result["status"].(string); ok && status != "success" {
		return fmt.Errorf("短信发送失败: status=%s", status)
	}
	return nil
}

// sendSMS validates config, generates code and sends it
func (a *SmsActor) sendSMS() (string, string) {
	if a.sendURL == "" {
		return "", "sms_send_url is not set"
	}
	if a.name == "" {
		return "", "sms_name is not set"
	}
	if a.phone == "" {
		return "", "sms_phone is not set"
	}

	code, err := a.generateCode()
	if err != nil {
		return "", err.Error()
	}

	log.InfoF("SendSMS url=%s code=%s phone=%s name=%s", a.sendURL, code, a.phone, a.name)
	if err := a.sendVerificationCode(code); err != nil {
		return "", err.Error()
	}
	return code, ""
}
