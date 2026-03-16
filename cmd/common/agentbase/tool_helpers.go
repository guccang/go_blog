package agentbase

import "fmt"

// ============================================================================
// 安全参数提取辅助函数（供工具回调使用）
// ============================================================================

// GetStringParam 安全提取字符串参数
func GetStringParam(arguments map[string]interface{}, key string) (string, error) {
	val, ok := arguments[key]
	if !ok {
		return "", fmt.Errorf("缺少参数: %s", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("参数类型错误: %s 应为字符串", key)
	}
	return str, nil
}

// GetIntParam 安全提取整数参数 (JSON数字默认为float64)
func GetIntParam(arguments map[string]interface{}, key string) (int, error) {
	val, ok := arguments[key]
	if !ok {
		return 0, fmt.Errorf("缺少参数: %s", key)
	}
	switch v := val.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("参数类型错误: %s 应为数字", key)
	}
}

// GetOptionalIntParam 安全提取可选整数参数
func GetOptionalIntParam(arguments map[string]interface{}, key string, defaultVal int) int {
	val, ok := arguments[key]
	if !ok {
		return defaultVal
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return defaultVal
	}
}

// ErrorJSON 返回JSON格式的错误消息
func ErrorJSON(msg string) string {
	return fmt.Sprintf(`{"error": "%s"}`, msg)
}
