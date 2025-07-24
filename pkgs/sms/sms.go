package sms

import (
	"bytes"
	"config"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"

	log "mylog"
)

var sms_send_url = ""
var sms_name = ""
var sms_phone = ""

func Init() {
	sms_send_url = config.GetConfig("sms_send_url")
	sms_name = config.GetConfig("admin")
	sms_phone = config.GetConfig("sms_phone")
}

// 生成6位随机数字验证码（更安全的版本）
func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("生成验证码失败: %v", err)
	}
	return fmt.Sprintf("%06d", n), nil
}

// 发送验证码请求
func sendVerificationCode(url, name, code, target string) (map[string]interface{}, error) {
	// 构造请求数据
	data := map[string]interface{}{
		"name":    name,
		"code":    code,
		"targets": target,
	}

	// 将数据编码为JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("JSON编码失败: %v", err)
	}

	// 创建带超时的HTTP客户端
	client := &http.Client{Timeout: 10 * time.Second}

	// 创建POST请求
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("请求发送失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析JSON响应
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %v", err)
	}

	return result, nil
}

func SendSMS() (string, error) {
	code, err := generateCode()
	if err != nil {
		fmt.Println("生成验证码失败:", err)
		return "", err
	}
	if sms_send_url == "" {
		return "", fmt.Errorf("sms_send_url is not set")
	}
	if sms_name == "" {
		return "", fmt.Errorf("sms_name is not set")
	}
	if sms_phone == "" {
		return "", fmt.Errorf("sms_phone is not set")
	}
	log.InfoF("SendSMS  url=%s code=%s phone=%s name=%s", sms_send_url, code, sms_phone, sms_name)
	_, err = sendVerificationCode(sms_send_url, sms_name, code, sms_phone)
	if err != nil {
		return "", err
	}
	return code, nil
}

/*
func main() {
	// 配置参数
	const (
		apiUrl   = "https://push.spug.cc/send/Dw1GdmdQLAjl32qK"
		name     = "ztt"
		target   = "15210842209"
	)

	// 1. 生成验证码
	code, err := generateCode()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}
	fmt.Printf("生成的验证码: %s\n", code)

	// 2. 发送验证码
	result, err := sendVerificationCode(apiUrl, name, code, target)
	if err != nil {
		fmt.Printf("发送验证码失败: %v\n", err)
		return
	}

	// 3. 处理响应
	fmt.Println("\n响应结果:")
	for k, v := range result {
		fmt.Printf("%s: %v\n", k, v)
	}

	// 4. 简单判断是否发送成功
	if status, ok := result["status"]; ok && status == "success" {
		fmt.Println("\n验证码发送成功!")
	} else {
		fmt.Println("\n验证码发送可能未成功，请检查响应")
	}
}
*/
