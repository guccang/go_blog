package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/zalando/go-keyring"
)

// credentialStore 凭据存储，keyring 优先，文件加密 fallback
type credentialStore struct {
	filePath string
}

// newCredentialStore 创建凭据存储
func newCredentialStore() *credentialStore {
	return &credentialStore{filePath: credentialFilePath()}
}

// Get 获取密码：keyring → 文件
func (cs *credentialStore) Get(account string) (string, error) {
	// 优先 keyring
	if pwd, err := keyring.Get(keyringService, account); err == nil && pwd != "" {
		return pwd, nil
	}

	// fallback: 加密文件
	return cs.fileGet(account)
}

// Set 保存密码：keyring + 文件（双写，确保至少一个可用）
func (cs *credentialStore) Set(account, password string) error {
	var keyringErr, fileErr error

	// 尝试 keyring
	keyringErr = keyring.Set(keyringService, account, password)

	// 同时写文件（macOS keyring 可能本次成功但下次读取失败）
	fileErr = cs.fileSet(account, password)

	if keyringErr != nil && fileErr != nil {
		return fmt.Errorf("keyring: %v; file: %v", keyringErr, fileErr)
	}
	return nil
}

// --- 文件加密存储 ---

type credentialFile struct {
	Credentials map[string]string `json:"credentials"` // account → hex(encrypted_password)
}

func (cs *credentialStore) fileGet(account string) (string, error) {
	data, err := os.ReadFile(cs.filePath)
	if err != nil {
		return "", fmt.Errorf("read credential file: %v", err)
	}

	var cf credentialFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return "", fmt.Errorf("parse credential file: %v", err)
	}

	encrypted, ok := cf.Credentials[account]
	if !ok {
		return "", fmt.Errorf("account %q not found", account)
	}

	ciphertext, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("decode: %v", err)
	}

	pwd, err := decrypt(ciphertext, deriveKey())
	if err != nil {
		return "", fmt.Errorf("decrypt: %v", err)
	}

	return string(pwd), nil
}

func (cs *credentialStore) fileSet(account, password string) error {
	// 读取已有文件
	var cf credentialFile
	if data, err := os.ReadFile(cs.filePath); err == nil {
		json.Unmarshal(data, &cf)
	}
	if cf.Credentials == nil {
		cf.Credentials = make(map[string]string)
	}

	// 加密密码
	ciphertext, err := encrypt([]byte(password), deriveKey())
	if err != nil {
		return fmt.Errorf("encrypt: %v", err)
	}
	cf.Credentials[account] = hex.EncodeToString(ciphertext)

	// 写入文件
	dir := filepath.Dir(cs.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir: %v", err)
	}

	data, _ := json.MarshalIndent(cf, "", "  ")
	return os.WriteFile(cs.filePath, data, 0600)
}

// credentialFilePath 返回凭据文件路径
func credentialFilePath() string {
	if runtime.GOOS == "windows" {
		// %APPDATA%\deploy-agent\credentials.json
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
		return filepath.Join(appData, "deploy-agent", "credentials.json")
	}
	// ~/.config/deploy-agent/credentials.json
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "deploy-agent", "credentials.json")
}

// deriveKey 从机器信息派生 AES-256 密钥
// 不是高安全级别，但足够保护 SSH 密码不被明文存储
func deriveKey() []byte {
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	home, _ := os.UserHomeDir()

	// 组合多个机器特征
	material := fmt.Sprintf("deploy-agent:%s:%s:%s:v1", hostname, username, home)
	hash := sha256.Sum256([]byte(material))
	return hash[:]
}

// encrypt AES-256-GCM 加密
func encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt AES-256-GCM 解密
func decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
