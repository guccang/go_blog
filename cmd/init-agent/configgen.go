package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// AgentConfigValues holds the user-provided values for an agent's config.
type AgentConfigValues struct {
	AgentName string
	Values    map[string]any
}

// AgentConfigInfo holds a dynamically discovered agent configuration.
type AgentConfigInfo struct {
	Name       string         `json:"name"`        // Agent name (directory name)
	Dir        string         `json:"dir"`         // Relative directory path (e.g., "cmd/acp-agent")
	ConfigPath string         `json:"config_path"` // Relative config file path
	Values     map[string]any `json:"values"`      // Parsed JSON key-value pairs
}

// DiscoverAgentConfigs scans cmd/*/ directories for JSON config files.
// It skips cmd/common and cmd/init-agent.
func DiscoverAgentConfigs(rootDir string) []AgentConfigInfo {
	cmdDir := filepath.Join(rootDir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil
	}

	var results []AgentConfigInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "common" || name == "init-agent" {
			continue
		}

		agentDir := filepath.Join(cmdDir, name)
		configPath := ""

		// Strategy 1: look for {dir-name}.json
		candidate := filepath.Join(agentDir, name+".json")
		if fileExistsAt(candidate) {
			configPath = candidate
		}

		// Strategy 2: look for any *.json file in the directory
		if configPath == "" {
			jsons, _ := filepath.Glob(filepath.Join(agentDir, "*.json"))
			if len(jsons) > 0 {
				configPath = jsons[0]
			}
		}

		if configPath == "" {
			continue
		}

		// Read and parse the JSON config
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		var values map[string]any
		if err := json.Unmarshal(data, &values); err != nil {
			continue
		}

		relDir := "cmd/" + name
		relConfig := relDir + "/" + filepath.Base(configPath)

		results = append(results, AgentConfigInfo{
			Name:       name,
			Dir:        relDir,
			ConfigPath: relConfig,
			Values:     values,
		})
	}

	return results
}

func fileExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// InferFieldType returns the inferred form field type from a JSON value.
// Returns: "string", "number", "bool", "array", "object"
func InferFieldType(value any) string {
	if value == nil {
		return "string"
	}
	switch value.(type) {
	case float64, int:
		return "number"
	case bool:
		return "bool"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "string"
	}
}

// FormatValueForDisplay converts a JSON value to a display string for the form.
func FormatValueForDisplay(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case float64:
		if v == float64(int(v)) {
			return strconv.Itoa(int(v))
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ",")
	case map[string]any:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ParseInputValue parses user input back to the original JSON type based on originalValue's type.
func ParseInputValue(input string, originalValue any) any {
	if originalValue == nil {
		return input
	}
	switch originalValue.(type) {
	case float64:
		if f, err := strconv.ParseFloat(input, 64); err == nil {
			return f
		}
		return input
	case int:
		if i, err := strconv.Atoi(input); err == nil {
			return i
		}
		return input
	case bool:
		return ParseBoolValue(input)
	case []any:
		parts := ParseStringSlice(input)
		result := make([]any, len(parts))
		for i, p := range parts {
			result[i] = p
		}
		return result
	case map[string]any:
		var m map[string]any
		if err := json.Unmarshal([]byte(input), &m); err == nil {
			return m
		}
		return input
	default:
		return input
	}
}

// SortedKeys returns the keys of a map in sorted order.
func SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// WriteDiscoveredConfig writes values directly back to the agent's JSON config file.
func WriteDiscoveredConfig(rootDir string, info AgentConfigInfo, values map[string]any) (string, error) {
	path := filepath.Join(rootDir, info.ConfigPath)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	data, err := json.MarshalIndent(values, "", "    ")
	if err != nil {
		return "", fmt.Errorf("序列化 JSON 失败: %v", err)
	}
	return path, os.WriteFile(path, append(data, '\n'), 0644)
}

// PreviewDiscoveredConfig returns a formatted JSON preview string.
func PreviewDiscoveredConfig(values map[string]any) string {
	data, err := json.MarshalIndent(values, "  ", "    ")
	if err != nil {
		return fmt.Sprintf("  (preview error: %v)", err)
	}
	return "  " + string(data)
}

// LoadExistingConfig reads an existing config file and returns its values as a map.
func LoadExistingConfig(rootDir string, schema *AgentSchema) (map[string]any, error) {
	path := filepath.Join(rootDir, schema.Dir, schema.ConfigFileName)
	if schema.ConfigFormat == "keyvalue" {
		return loadKeyValueConfig(path)
	}
	return loadJSONConfig(path)
}

func loadJSONConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 JSON 配置失败: %v", err)
	}
	return m, nil
}

func loadKeyValueConfig(path string) (map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	m := make(map[string]any)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		m[key] = val
	}
	return m, scanner.Err()
}

// MergeConfigs merges existing config values with shared values and user input.
// Priority: userValues > existing > defaults
func MergeConfigs(schema *AgentSchema, existing map[string]any, sharedValues map[string]string, userValues map[string]string) map[string]any {
	result := make(map[string]any)

	for _, field := range schema.Fields {
		key := field.Key

		// 1. Check user-provided value
		if v, ok := userValues[key]; ok && v != "" {
			result[key] = convertValue(field, v)
			continue
		}

		// 2. Check shared values (for shared fields)
		if field.Shared {
			sharedKey := normalizeSharedKey(key)
			if v, ok := sharedValues[sharedKey]; ok && v != "" {
				result[key] = convertValue(field, v)
				continue
			}
		}

		// 3. Check existing config
		if existing != nil {
			if v, ok := existing[key]; ok {
				result[key] = v
				continue
			}
		}

		// 4. Use default value
		if field.DefaultValue != nil {
			result[key] = field.DefaultValue
		}
	}

	return result
}

// normalizeSharedKey maps agent-specific key names to shared key names.
func normalizeSharedKey(key string) string {
	switch key {
	case "gateway_url":
		return "server_url"
	case "gateway_token":
		return "auth_token"
	default:
		return key
	}
}

// convertValue converts a string value to the appropriate Go type based on FieldType.
func convertValue(field ConfigField, value string) any {
	switch field.Type {
	case FieldInt:
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
		return value
	case FieldBool:
		return ParseBoolValue(value)
	case FieldPort:
		if v, err := strconv.Atoi(strings.TrimPrefix(value, ":")); err == nil {
			return v
		}
		return value
	case FieldStringSlice:
		return ParseStringSlice(value)
	default:
		return value
	}
}

// WriteAgentConfig writes the config file for an agent.
func WriteAgentConfig(rootDir string, schema *AgentSchema, values map[string]any) (string, error) {
	path := filepath.Join(rootDir, schema.Dir, schema.ConfigFileName)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	if schema.ConfigFormat == "keyvalue" {
		return path, writeKeyValueConfig(path, values)
	}
	return path, writeJSONConfig(path, schema, values)
}

func writeJSONConfig(path string, schema *AgentSchema, values map[string]any) error {
	// Build the config respecting nested keys (e.g., "llm.model")
	output := buildNestedMap(values)

	data, err := json.MarshalIndent(output, "", "    ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %v", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// buildNestedMap converts flat dotted keys into nested maps.
// e.g., {"llm.model": "gpt-4"} → {"llm": {"model": "gpt-4"}}
func buildNestedMap(flat map[string]any) map[string]any {
	result := make(map[string]any)

	for key, value := range flat {
		parts := strings.Split(key, ".")
		if len(parts) == 1 {
			result[key] = value
			continue
		}

		// Navigate/create nested structure
		current := result
		for i, part := range parts {
			if i == len(parts)-1 {
				current[part] = value
			} else {
				if _, ok := current[part]; !ok {
					current[part] = make(map[string]any)
				}
				if m, ok := current[part].(map[string]any); ok {
					current = m
				}
			}
		}
	}

	return result
}

func writeKeyValueConfig(path string, values map[string]any) error {
	var lines []string
	lines = append(lines, "# 由 init-agent 自动生成")
	lines = append(lines, "")

	for key, val := range values {
		if val == nil {
			continue
		}
		var strVal string
		switch v := val.(type) {
		case string:
			strVal = v
		case int:
			strVal = strconv.Itoa(v)
		case float64:
			strVal = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			if v {
				strVal = "true"
			} else {
				strVal = "false"
			}
		default:
			strVal = fmt.Sprintf("%v", v)
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, strVal))
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// PreviewConfig returns a formatted preview of the config that would be written.
func PreviewConfig(schema *AgentSchema, values map[string]any) string {
	if schema.ConfigFormat == "keyvalue" {
		return previewKeyValue(values)
	}
	return previewJSON(values)
}

func previewJSON(values map[string]any) string {
	output := buildNestedMap(values)
	data, err := json.MarshalIndent(output, "  ", "    ")
	if err != nil {
		return fmt.Sprintf("  (preview error: %v)", err)
	}
	return "  " + string(data)
}

func previewKeyValue(values map[string]any) string {
	var lines []string
	for key, val := range values {
		if val == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s=%v", key, val))
	}
	return strings.Join(lines, "\n")
}

// GetDefaultValueString returns the default value as a display string.
func GetDefaultValueString(field ConfigField) string {
	if field.DefaultValue == nil {
		return ""
	}
	switch v := field.DefaultValue.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}
