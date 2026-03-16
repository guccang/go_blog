package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
)

// Handler 微信回调处理器
type Handler struct {
	cfg    *Config
	bridge *Bridge
}

// NewHandler 创建处理器
func NewHandler(cfg *Config, bridge *Bridge) *Handler {
	return &Handler{cfg: cfg, bridge: bridge}
}

// HandleCallback 处理微信回调（GET=验证, POST=消息）
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.IsCallbackEnabled() {
		http.Error(w, "WeChat callback not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.handleVerify(w, r)
	case http.MethodPost:
		h.handleMessage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVerify URL 验证
func (h *Handler) handleVerify(w http.ResponseWriter, r *http.Request) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	echoStr := r.URL.Query().Get("echostr")

	log.Printf("[Handler] WeChat verify: timestamp=%s nonce=%s", timestamp, nonce)

	decrypted, err := verifyURL(h.cfg.Token, h.cfg.EncodingAESKey, h.cfg.CorpID,
		msgSignature, timestamp, nonce, echoStr)
	if err != nil {
		log.Printf("[Handler] verify failed: %v", err)
		http.Error(w, "Verification failed", http.StatusForbidden)
		return
	}
	log.Println("[Handler] WeChat URL verification successful")
	w.Write([]byte(decrypted))
}

// handleMessage 处理加密消息
func (h *Handler) handleMessage(w http.ResponseWriter, r *http.Request) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	decrypted, err := decryptMessage(h.cfg.Token, h.cfg.EncodingAESKey, h.cfg.CorpID,
		msgSignature, timestamp, nonce, string(body))
	if err != nil {
		log.Printf("[Handler] decrypt failed: %v", err)
		http.Error(w, "Decrypt failed", http.StatusForbidden)
		return
	}

	msg, err := parseMessage(decrypted)
	if err != nil {
		log.Printf("[Handler] parse failed: %v", err)
		w.Write([]byte("success"))
		return
	}

	log.Printf("[Handler] WeChat message from %s: %s", msg.FromUserName, msg.Content)

	// 异步处理：通过 bridge 发送到 gateway
	go h.bridge.HandleWechatMessage(msg)

	// 立即返回 "success" 给微信服务器
	w.Write([]byte("success"))
}

// ========================= 微信消息结构 =========================

// WechatMessage 企业微信消息
type WechatMessage struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	MsgId        string `xml:"MsgId"`
	AgentID      string `xml:"AgentID"`
}

// ========================= 加解密函数（从 pkgs/wechat/crypto.go 复用） =========================

func verifyURL(token, encodingAESKey, corpID, msgSignature, timestamp, nonce, echoStr string) (string, error) {
	if !verifySignature(token, timestamp, nonce, echoStr, msgSignature) {
		return "", fmt.Errorf("signature verification failed")
	}

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

	content, _, err := extractMessage(plainText)
	if err != nil {
		return "", fmt.Errorf("extract message: %v", err)
	}

	return content, nil
}

func decryptMessage(token, encodingAESKey, corpID, msgSignature, timestamp, nonce, xmlBody string) (string, error) {
	var encMsg struct {
		XMLName    xml.Name `xml:"xml"`
		ToUserName string   `xml:"ToUserName"`
		Encrypt    string   `xml:"Encrypt"`
		AgentID    string   `xml:"AgentID"`
	}
	if err := xml.Unmarshal([]byte(xmlBody), &encMsg); err != nil {
		return "", fmt.Errorf("parse xml: %v", err)
	}

	if !verifySignature(token, timestamp, nonce, encMsg.Encrypt, msgSignature) {
		return "", fmt.Errorf("signature verification failed")
	}

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

	if receivedCorpID != corpID {
		return "", fmt.Errorf("corp_id mismatch: got %s, want %s", receivedCorpID, corpID)
	}

	return content, nil
}

func parseMessage(xmlContent string) (*WechatMessage, error) {
	var msg WechatMessage
	if err := xml.Unmarshal([]byte(xmlContent), &msg); err != nil {
		return nil, fmt.Errorf("parse message xml: %v", err)
	}
	return &msg, nil
}

func verifySignature(token, timestamp, nonce, encrypt, msgSignature string) bool {
	strs := []string{token, timestamp, nonce, encrypt}
	sort.Strings(strs)
	combined := strings.Join(strs, "")
	hash := sha1.New()
	hash.Write([]byte(combined))
	computed := fmt.Sprintf("%x", hash.Sum(nil))
	return computed == msgSignature
}

func aesDecrypt(cipherText, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(cipherText) < aes.BlockSize {
		return nil, fmt.Errorf("cipher text too short")
	}
	iv := key[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	plainText := make([]byte, len(cipherText))
	mode.CryptBlocks(plainText, cipherText)
	plainText = pkcs7Unpad(plainText)
	return plainText, nil
}

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

func extractMessage(plainText []byte) (string, string, error) {
	if len(plainText) < 20 {
		return "", "", fmt.Errorf("plain text too short: %d", len(plainText))
	}
	msgLenBytes := plainText[16:20]
	msgLen := int(binary.BigEndian.Uint32(msgLenBytes))
	if 20+msgLen > len(plainText) {
		return "", "", fmt.Errorf("invalid message length: %d (total: %d)", msgLen, len(plainText))
	}
	content := string(plainText[20 : 20+msgLen])
	corpID := string(plainText[20+msgLen:])
	corpID = strings.TrimRight(corpID, " \x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x20")
	return content, corpID, nil
}
