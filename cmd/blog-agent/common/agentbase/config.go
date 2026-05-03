package agentbase

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadKeyValueConfig 加载 key=value 格式的配置文件
// 支持 # 注释和空行
func LoadKeyValueConfig(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %v", err)
	}
	defer file.Close()

	config := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: %s", lineNum, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		config[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config file: %v", err)
	}

	return config, nil
}

// GetString 获取字符串配置值（带默认值）
func GetString(config map[string]string, key, defaultValue string) string {
	if value, exists := config[key]; exists {
		return value
	}
	return defaultValue
}

// GetInt 获取整数配置值（带默认值）
func GetInt(config map[string]string, key string, defaultValue int) int {
	if value, exists := config[key]; exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetBool 获取布尔配置值（带默认值）
func GetBool(config map[string]string, key string, defaultValue bool) bool {
	if value, exists := config[key]; exists {
		lowerValue := strings.ToLower(value)
		if lowerValue == "true" || lowerValue == "1" || lowerValue == "yes" {
			return true
		}
		if lowerValue == "false" || lowerValue == "0" || lowerValue == "no" {
			return false
		}
	}
	return defaultValue
}

// MustGetString 获取字符串配置值（必需）
func MustGetString(config map[string]string, key string) (string, error) {
	if value, exists := config[key]; exists && value != "" {
		return value, nil
	}
	return "", fmt.Errorf("required config key missing: %s", key)
}

// MustGetInt 获取整数配置值（必需）
func MustGetInt(config map[string]string, key string) (int, error) {
	value, err := MustGetString(config, key)
	if err != nil {
		return 0, err
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %s", key, value)
	}
	return intValue, nil
}

// WriteDefaultConfig 将默认配置序列化为 JSON 并写入指定路径
// 如果文件已存在则报错，不会覆盖
func WriteDefaultConfig(path string, cfg interface{}) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("配置文件已存在: %s（不会覆盖）", path)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	fmt.Printf("已生成默认配置文件: %s\n", path)
	return nil
}
