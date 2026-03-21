package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ValidateField validates a single field value against its type and rules.
func ValidateField(field ConfigField, value string) error {
	if value == "" {
		if field.Required {
			return fmt.Errorf("字段 %q 是必填项", field.Label)
		}
		return nil
	}

	switch field.Type {
	case FieldURL:
		return validateURL(value)
	case FieldPort:
		return validatePort(value)
	case FieldPath:
		return validatePath(value)
	case FieldInt:
		return validateInt(value)
	case FieldBool:
		return validateBool(value)
	case FieldStringSlice:
		// Accept comma-separated values
		return nil
	case FieldMap:
		// Maps are complex; basic JSON check or skip
		return nil
	default:
		return nil
	}
}

func validateURL(value string) error {
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("无效的 URL: %v", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("URL 缺少协议 (如 http://, ws://)")
	}
	if u.Host == "" {
		return fmt.Errorf("URL 缺少主机地址")
	}
	return nil
}

func validatePort(value string) error {
	// May be prefixed with ":" for listen format
	value = strings.TrimPrefix(value, ":")
	p, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("端口必须是数字: %v", err)
	}
	if p < 1 || p > 65535 {
		return fmt.Errorf("端口范围: 1-65535, 当前: %d", p)
	}
	return nil
}

func validatePath(value string) error {
	// Allow relative paths and non-existent paths (they may be created later)
	// Just check for obviously invalid characters
	if strings.ContainsAny(value, "<>|\"") {
		return fmt.Errorf("路径包含无效字符")
	}
	return nil
}

func validateInt(value string) error {
	_, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("必须是整数: %v", err)
	}
	return nil
}

func validateBool(value string) error {
	v := strings.ToLower(value)
	if v != "true" && v != "false" && v != "yes" && v != "no" && v != "1" && v != "0" {
		return fmt.Errorf("布尔值应为 true/false/yes/no")
	}
	return nil
}

// CheckPortAvailable checks if a port is available for binding.
func CheckPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ParseBoolValue normalizes various bool representations to bool.
func ParseBoolValue(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	return v == "true" || v == "yes" || v == "1"
}

// ParseStringSlice splits a comma-separated string into a slice.
func ParseStringSlice(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
