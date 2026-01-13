package sms

import (
	"bytes"
	"config"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	log "mylog"
	http "net/http"
	"sync"
	"time"
)

// ========== Simple SMS 模块 ==========
// 无 Actor、无 Channel，使用 sync.Mutex

var (
	smsConfig struct {
		sendURL string
		name    string
		phone   string
	}
	smsMu sync.Mutex
)

func Info() {
	log.Debug(log.ModuleSMS, "info sms v2.0 (simple)")
}

// Init 初始化 SMS 模块
func Init() {
	smsMu.Lock()
	defer smsMu.Unlock()
	smsConfig.sendURL = config.GetConfigWithAccount(config.GetAdminAccount(), "sms_send_url")
	smsConfig.name = config.GetAdminAccount()
	smsConfig.phone = config.GetConfigWithAccount(config.GetAdminAccount(), "sms_phone")
}

// SendSMS 发送短信验证码
func SendSMS() (string, error) {
	smsMu.Lock()
	defer smsMu.Unlock()

	// 验证配置
	if smsConfig.sendURL == "" {
		return "", fmt.Errorf("sms_send_url is not set")
	}
	if smsConfig.name == "" {
		return "", fmt.Errorf("sms_name is not set")
	}
	if smsConfig.phone == "" {
		return "", fmt.Errorf("sms_phone is not set")
	}

	// 生成 6 位验证码
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("生成验证码失败: %v", err)
	}
	code := fmt.Sprintf("%06d", n)

	// 构建请求
	payload := map[string]interface{}{
		"name":    smsConfig.name,
		"code":    code,
		"targets": smsConfig.phone,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %v", err)
	}

	// 发送 HTTP 请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(smsConfig.sendURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("请求发送失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("JSON解析失败: %v", err)
	}

	// 检查响应状态
	if status, ok := result["status"].(string); ok && status != "success" {
		return "", fmt.Errorf("短信发送失败: status=%s", status)
	}

	log.InfoF(log.ModuleSMS, "SendSMS url=%s code=%s phone=%s name=%s",
		smsConfig.sendURL, code, smsConfig.phone, smsConfig.name)
	return code, nil
}
