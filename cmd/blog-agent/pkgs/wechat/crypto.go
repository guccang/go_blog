package wechat

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

// VerifyURL 验证企业微信回调 URL
// 返回解密后的 echostr
func VerifyURL(token, encodingAESKey, corpID, msgSignature, timestamp, nonce, echoStr string) (string, error) {
	// 验证签名
	if !verifySignature(token, timestamp, nonce, echoStr, msgSignature) {
		return "", fmt.Errorf("signature verification failed")
	}

	// 解密 echostr
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return "", fmt.Errorf("decode AES key: %v", err)
	}

	cipherText, err := base64.StdEncoding.DecodeString(echoStr)
	if err != nil {
		return "", fmt.Errorf("decode echostr: %v", err)
	}

	plainText, err := aesDecrypt(cipherText, aesKey)
	if err != nil {
		return "", fmt.Errorf("decrypt echostr: %v", err)
	}

	// 提取消息内容（去掉随机字符串和长度前缀）
	content, _, err := extractMessage(plainText)
	if err != nil {
		return "", fmt.Errorf("extract message: %v", err)
	}

	return content, nil
}

// DecryptMessage 解密企业微信推送的消息
func DecryptMessage(token, encodingAESKey, corpID, msgSignature, timestamp, nonce, xmlBody string) (string, error) {
	// 从 XML 中提取加密内容
	var encMsg struct {
		XMLName    xml.Name `xml:"xml"`
		ToUserName string   `xml:"ToUserName"`
		Encrypt    string   `xml:"Encrypt"`
		AgentID    string   `xml:"AgentID"`
	}
	if err := xml.Unmarshal([]byte(xmlBody), &encMsg); err != nil {
		return "", fmt.Errorf("parse xml: %v", err)
	}

	// 验证签名
	if !verifySignature(token, timestamp, nonce, encMsg.Encrypt, msgSignature) {
		return "", fmt.Errorf("signature verification failed")
	}

	// 解密
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return "", fmt.Errorf("decode AES key: %v", err)
	}

	cipherText, err := base64.StdEncoding.DecodeString(encMsg.Encrypt)
	if err != nil {
		return "", fmt.Errorf("decode cipher text: %v", err)
	}

	plainText, err := aesDecrypt(cipherText, aesKey)
	if err != nil {
		return "", fmt.Errorf("decrypt: %v", err)
	}

	content, receivedCorpID, err := extractMessage(plainText)
	if err != nil {
		return "", fmt.Errorf("extract: %v", err)
	}

	// 验证 CorpID
	if receivedCorpID != corpID {
		return "", fmt.Errorf("corp_id mismatch: got %s, want %s", receivedCorpID, corpID)
	}

	return content, nil
}

// ParseMessage 解析明文 XML 消息
func ParseMessage(xmlContent string) (*WechatMessage, error) {
	var msg WechatMessage
	if err := xml.Unmarshal([]byte(xmlContent), &msg); err != nil {
		return nil, fmt.Errorf("parse message xml: %v", err)
	}
	return &msg, nil
}

// ========================= 内部工具函数 =========================

// verifySignature 验证消息签名
func verifySignature(token, timestamp, nonce, encrypt, msgSignature string) bool {
	strs := []string{token, timestamp, nonce, encrypt}
	sort.Strings(strs)
	combined := strings.Join(strs, "")

	hash := sha1.New()
	hash.Write([]byte(combined))
	computed := fmt.Sprintf("%x", hash.Sum(nil))

	return computed == msgSignature
}

// aesDecrypt AES-CBC 解密
func aesDecrypt(cipherText, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(cipherText) < aes.BlockSize {
		return nil, fmt.Errorf("cipher text too short")
	}

	// IV 是 AES Key 的前 16 字节
	iv := key[:aes.BlockSize]

	mode := cipher.NewCBCDecrypter(block, iv)
	plainText := make([]byte, len(cipherText))
	mode.CryptBlocks(plainText, cipherText)

	// PKCS#7 去填充
	plainText = pkcs7Unpad(plainText)

	return plainText, nil
}

// pkcs7Unpad PKCS#7 去填充
func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding > aes.BlockSize {
		return data
	}
	return data[:len(data)-padding]
}

// extractMessage 从解密后的明文中提取消息内容
// 格式: 16字节随机串 + 4字节消息长度(网络字节序) + 消息内容 + CorpID
func extractMessage(plainText []byte) (string, string, error) {
	if len(plainText) < 20 {
		return "", "", fmt.Errorf("plain text too short: %d", len(plainText))
	}

	// 跳过16字节随机串
	msgLenBytes := plainText[16:20]
	msgLen := int(binary.BigEndian.Uint32(msgLenBytes))

	if 20+msgLen > len(plainText) {
		return "", "", fmt.Errorf("invalid message length: %d (total: %d)", msgLen, len(plainText))
	}

	content := string(plainText[20 : 20+msgLen])
	corpID := string(plainText[20+msgLen:])

	// 清除可能残留的填充字符（控制字符）
	corpID = strings.TrimRight(corpID, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f")

	return content, corpID, nil
}
