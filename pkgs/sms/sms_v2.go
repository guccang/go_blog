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

	"config"
	log "mylog"
)

// ========== 新版 SMS Actor (基于泛型框架) ==========

// SmsActorV2 使用新版 ActorV2 框架
type SmsActorV2 struct {
	*core.ActorV2
	sendURL string
	name    string
	phone   string
}

var sms_actor_v2 *SmsActorV2

// InitV2 初始化新版 SMS 模块
func InitV2() {
	sms_actor_v2 = &SmsActorV2{
		ActorV2: core.NewActorV2(),
		sendURL: config.GetConfigWithAccount(config.GetAdminAccount(), "sms_send_url"),
		name:    config.GetAdminAccount(),
		phone:   config.GetConfigWithAccount(config.GetAdminAccount(), "sms_phone"),
	}
}

// SendSMSV2 发送短信验证码 - 新版实现
// 使用 Execute2 泛型函数，类型安全，无需类型断言
func SendSMSV2() (string, error) {
	return core.Execute2(sms_actor_v2.ActorV2, func() (string, error) {
		// 验证配置
		if sms_actor_v2.sendURL == "" {
			return "", fmt.Errorf("sms_send_url is not set")
		}
		if sms_actor_v2.name == "" {
			return "", fmt.Errorf("sms_name is not set")
		}
		if sms_actor_v2.phone == "" {
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
			"name":    sms_actor_v2.name,
			"code":    code,
			"targets": sms_actor_v2.phone,
		}
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("JSON编码失败: %v", err)
		}

		// 发送 HTTP 请求
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post(sms_actor_v2.sendURL, "application/json", bytes.NewBuffer(jsonData))
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
			sms_actor_v2.sendURL, code, sms_actor_v2.phone, sms_actor_v2.name)
		return code, nil
	})
}

// SendSMSAsyncV2 异步发送短信验证码
// 返回一个 channel，调用方可以非阻塞地等待结果
func SendSMSAsyncV2() <-chan core.Result2[string, error] {
	return core.ExecuteAsync(sms_actor_v2.ActorV2, func() core.Result2[string, error] {
		code, err := sendSMSInternal()
		return core.Result2[string, error]{V1: code, V2: err}
	})
}

// SendSMSWithTimeoutV2 带超时的发送短信
func SendSMSWithTimeoutV2(timeout time.Duration) (string, error, bool) {
	result, ok := core.ExecuteWithTimeout(sms_actor_v2.ActorV2, timeout, func() core.Result2[string, error] {
		code, err := sendSMSInternal()
		return core.Result2[string, error]{V1: code, V2: err}
	})
	return result.V1, result.V2, ok
}

// sendSMSInternal 内部发送逻辑，避免代码重复
func sendSMSInternal() (string, error) {
	if sms_actor_v2.sendURL == "" {
		return "", fmt.Errorf("sms_send_url is not set")
	}
	if sms_actor_v2.phone == "" {
		return "", fmt.Errorf("sms_phone is not set")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("生成验证码失败: %v", err)
	}
	code := fmt.Sprintf("%06d", n)

	payload, _ := json.Marshal(map[string]interface{}{
		"name":    sms_actor_v2.name,
		"code":    code,
		"targets": sms_actor_v2.phone,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(sms_actor_v2.sendURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if status, _ := result["status"].(string); status != "success" {
		return "", fmt.Errorf("发送失败: %s", status)
	}

	log.InfoF(log.ModuleSMS, "SendSMS code=%s phone=%s", code, sms_actor_v2.phone)
	return code, nil
}
