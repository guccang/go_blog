package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const legacyToolCallMarker = "[TOOL_CALL]"

var (
	legacyToolNameRe = regexp.MustCompile(`(?is)\btool\s*=>\s*['"]([^'"]+)['"]`)
	legacyArgsHeadRe = regexp.MustCompile(`(?is)\bargs\s*=>\s*`)
	legacyArgFlagRe  = regexp.MustCompile(`(?m)(?:^|[\r\n])\s*--([a-zA-Z0-9_-]+)\b`)
)

// extractLegacyToolCallBlocks 解析旧式文本工具调用块：
//
//	[TOOL_CALL]
//	{tool => 'TextToAudio', args => {
//	  --content "你好"
//	  --voice "foo"
//	}}
//
// 解析成功后会移除文本块，并返回结构化 ToolCall。
func extractLegacyToolCallBlocks(content string) (string, []ToolCall) {
	if !strings.Contains(content, legacyToolCallMarker) {
		return strings.TrimSpace(content), nil
	}

	var (
		cleaned strings.Builder
		calls   []ToolCall
		rest    = content
	)

	for {
		idx := strings.Index(rest, legacyToolCallMarker)
		if idx < 0 {
			cleaned.WriteString(rest)
			break
		}

		cleaned.WriteString(rest[:idx])
		afterMarker := rest[idx+len(legacyToolCallMarker):]
		blockStart := skipLegacyWhitespace(afterMarker, 0)
		if blockStart >= len(afterMarker) || afterMarker[blockStart] != '{' {
			cleaned.WriteString(legacyToolCallMarker)
			rest = afterMarker
			continue
		}

		block, consumed, ok := extractBalancedBraceBlock(afterMarker, blockStart)
		if !ok {
			cleaned.WriteString(legacyToolCallMarker)
			rest = afterMarker
			continue
		}

		call, ok := parseLegacyToolCallBlock(block, len(calls)+1)
		if !ok {
			cleaned.WriteString(legacyToolCallMarker)
			cleaned.WriteString(block)
			rest = afterMarker[consumed:]
			continue
		}

		calls = append(calls, call)
		rest = afterMarker[consumed:]
	}

	return strings.TrimSpace(compactLegacyWhitespace(cleaned.String())), calls
}

func parseLegacyToolCallBlock(block string, seq int) (ToolCall, bool) {
	matches := legacyToolNameRe.FindStringSubmatch(block)
	if len(matches) < 2 {
		return ToolCall{}, false
	}

	toolName := strings.TrimSpace(matches[1])
	if toolName == "" {
		return ToolCall{}, false
	}

	args := map[string]any{}
	if argsBody, ok := extractLegacyArgsBody(block); ok {
		args = parseLegacyToolArgs(argsBody)
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return ToolCall{}, false
	}

	return ToolCall{
		ID:   fmt.Sprintf("legacy_text_%d", seq),
		Type: "function",
		Function: FunctionCall{
			Name:      toolName,
			Arguments: string(argsJSON),
		},
	}, true
}

func extractLegacyArgsBody(block string) (string, bool) {
	loc := legacyArgsHeadRe.FindStringIndex(block)
	if loc == nil {
		return "", false
	}

	start := skipLegacyWhitespace(block, loc[1])
	if start >= len(block) || block[start] != '{' {
		return "", false
	}

	argBlock, _, ok := extractBalancedBraceBlock(block, start)
	if !ok || len(argBlock) < 2 {
		return "", false
	}
	return strings.TrimSpace(argBlock[1 : len(argBlock)-1]), true
}

func parseLegacyToolArgs(body string) map[string]any {
	args := make(map[string]any)
	matches := legacyArgFlagRe.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		parseLegacyJSONArgs(body, args)
		return args
	}

	for i, match := range matches {
		if len(match) < 4 {
			continue
		}
		key := strings.TrimSpace(body[match[2]:match[3]])
		valueStart := match[1]
		valueEnd := len(body)
		if i+1 < len(matches) {
			valueEnd = matches[i+1][0]
		}
		value := decodeLegacyArgValue(body[valueStart:valueEnd])
		if value == "" {
			continue
		}
		setLegacyToolArg(args, key, value)
	}

	return args
}

func parseLegacyJSONArgs(body string, args map[string]any) {
	candidate := strings.TrimSpace(body)
	if candidate == "" {
		return
	}
	if !strings.HasPrefix(candidate, "{") {
		candidate = "{" + candidate + "}"
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(candidate), &raw); err != nil {
		return
	}

	for key, value := range raw {
		textValue := strings.TrimSpace(fmt.Sprint(value))
		if textValue == "" {
			continue
		}
		setLegacyToolArg(args, key, textValue)
	}
}

func setLegacyToolArg(args map[string]any, key, value string) {
	normalizedKey := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	if normalizedKey == "" {
		return
	}

	switch normalizedKey {
	case "content", "input":
		normalizedKey = "text"
	case "format":
		normalizedKey = "audio_format"
	case "voice_id":
		normalizedKey = "voice"
	}

	args[normalizedKey] = value
}

func decodeLegacyArgValue(raw string) string {
	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), ","))
	if raw == "" {
		return ""
	}

	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		if decoded, err := strconv.Unquote(raw); err == nil {
			return strings.TrimSpace(decoded)
		}
		return strings.TrimSpace(raw[1 : len(raw)-1])
	}

	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return strings.TrimSpace(unescapeLegacySingleQuoted(raw[1 : len(raw)-1]))
	}

	return strings.TrimSpace(raw)
}

func unescapeLegacySingleQuoted(raw string) string {
	replacer := strings.NewReplacer(
		`\\`, `\`,
		`\'`, `'`,
		`\"`, `"`,
		`\n`, "\n",
		`\r`, "\r",
		`\t`, "\t",
	)
	return replacer.Replace(raw)
}

func extractBalancedBraceBlock(s string, start int) (string, int, bool) {
	start = skipLegacyWhitespace(s, start)
	if start >= len(s) || s[start] != '{' {
		return "", 0, false
	}

	depth := 0
	inSingle := false
	inDouble := false
	escaped := false

	for i := start; i < len(s); i++ {
		ch := s[i]

		if escaped {
			escaped = false
			continue
		}

		if inSingle {
			switch ch {
			case '\\':
				escaped = true
			case '\'':
				inSingle = false
			}
			continue
		}

		if inDouble {
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inDouble = false
			}
			continue
		}

		switch ch {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], i + 1, true
			}
		}
	}

	return "", 0, false
}

func skipLegacyWhitespace(s string, start int) int {
	for start < len(s) {
		switch s[start] {
		case ' ', '\t', '\r', '\n':
			start++
		default:
			return start
		}
	}
	return start
}

func compactLegacyWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
