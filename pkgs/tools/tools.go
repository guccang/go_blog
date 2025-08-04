package tools

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	log "mylog"
)

// Info displays package version information
func Info() {
	log.Debug("info tools v1.0")
}

// TimeResult 时间工具结果
type TimeResult struct {
	CurrentTime   string `json:"current_time"`
	Timestamp     int64  `json:"timestamp"`
	Timezone      string `json:"timezone"`
	FormattedTime string `json:"formatted_time"`
}

// DataProcessResult 数据处理结果
type DataProcessResult struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Valid  bool   `json:"valid"`
	Error  string `json:"error,omitempty"`
}

// CalculatorResult 计算器结果
type CalculatorResult struct {
	Expression string  `json:"expression"`
	Result     float64 `json:"result"`
	Error      string  `json:"error,omitempty"`
}

// TextResult 文本工具结果
type TextResult struct {
	Characters         int `json:"characters"`
	Words              int `json:"words"`
	Lines              int `json:"lines"`
	CharactersNoSpaces int `json:"characters_no_spaces"`
}

// WeatherResult 天气结果
type WeatherResult struct {
	City        string  `json:"city"`
	Temperature float64 `json:"temperature"`
	Description string  `json:"description"`
	Humidity    int     `json:"humidity"`
	Error       string  `json:"error,omitempty"`
}

// GetCurrentTime 获取当前时间
func GetCurrentTime(timezone string) TimeResult {
	now := time.Now()
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			now = now.In(loc)
		}
	}

	return TimeResult{
		CurrentTime:   now.Format("2006-01-02 15:04:05"),
		Timestamp:     now.Unix(),
		Timezone:      now.Location().String(),
		FormattedTime: now.Format("Monday, January 2, 2006 at 3:04 PM"),
	}
}

// ConvertTimestamp 时间戳转换
func ConvertTimestamp(timestamp int64) TimeResult {
	t := time.Unix(timestamp, 0)
	return TimeResult{
		CurrentTime:   t.Format("2006-01-02 15:04:05"),
		Timestamp:     timestamp,
		Timezone:      t.Location().String(),
		FormattedTime: t.Format("Monday, January 2, 2006 at 3:04 PM"),
	}
}

// FormatJSON JSON格式化
func FormatJSON(input string) DataProcessResult {
	input = strings.TrimSpace(input)
	if input == "" {
		return DataProcessResult{
			Input: input,
			Valid: false,
			Error: "输入为空",
		}
	}

	var jsonData interface{}
	if err := json.Unmarshal([]byte(input), &jsonData); err != nil {
		return DataProcessResult{
			Input:  input,
			Output: input,
			Valid:  false,
			Error:  "无效的JSON格式: " + err.Error(),
		}
	}

	formatted, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return DataProcessResult{
			Input:  input,
			Output: input,
			Valid:  false,
			Error:  "格式化失败: " + err.Error(),
		}
	}

	return DataProcessResult{
		Input:  input,
		Output: string(formatted),
		Valid:  true,
	}
}

// EncodeBase64 Base64编码
func EncodeBase64(input string) DataProcessResult {
	encoded := base64.StdEncoding.EncodeToString([]byte(input))
	return DataProcessResult{
		Input:  input,
		Output: encoded,
		Valid:  true,
	}
}

// DecodeBase64 Base64解码
func DecodeBase64(input string) DataProcessResult {
	decoded, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return DataProcessResult{
			Input:  input,
			Output: input,
			Valid:  false,
			Error:  "Base64解码失败: " + err.Error(),
		}
	}

	return DataProcessResult{
		Input:  input,
		Output: string(decoded),
		Valid:  true,
	}
}

// EncodeURL URL编码
func EncodeURL(input string) DataProcessResult {
	encoded := url.QueryEscape(input)
	return DataProcessResult{
		Input:  input,
		Output: encoded,
		Valid:  true,
	}
}

// DecodeURL URL解码
func DecodeURL(input string) DataProcessResult {
	decoded, err := url.QueryUnescape(input)
	if err != nil {
		return DataProcessResult{
			Input:  input,
			Output: input,
			Valid:  false,
			Error:  "URL解码失败: " + err.Error(),
		}
	}

	return DataProcessResult{
		Input:  input,
		Output: decoded,
		Valid:  true,
	}
}

// GenerateHash 生成哈希值
func GenerateHash(input, hashType string) DataProcessResult {
	var hash string

	switch strings.ToLower(hashType) {
	case "md5":
		h := md5.Sum([]byte(input))
		hash = hex.EncodeToString(h[:])
	case "sha1":
		h := sha1.Sum([]byte(input))
		hash = hex.EncodeToString(h[:])
	case "sha256":
		h := sha256.Sum256([]byte(input))
		hash = hex.EncodeToString(h[:])
	default:
		return DataProcessResult{
			Input:  input,
			Output: input,
			Valid:  false,
			Error:  "不支持的哈希类型: " + hashType,
		}
	}

	return DataProcessResult{
		Input:  input,
		Output: hash,
		Valid:  true,
	}
}

// Calculate 计算器
func Calculate(expression string) CalculatorResult {
	// 简单的四则运算计算器实现
	expression = strings.ReplaceAll(expression, " ", "")

	// 基本的表达式验证
	if matched, _ := regexp.MatchString(`^[0-9+\-*/.()]+$`, expression); !matched {
		return CalculatorResult{
			Expression: expression,
			Error:      "表达式包含无效字符",
		}
	}

	// 这里简化处理，实际项目中可以使用更完善的表达式解析库
	result, err := evaluateExpression(expression)
	if err != nil {
		return CalculatorResult{
			Expression: expression,
			Error:      err.Error(),
		}
	}

	return CalculatorResult{
		Expression: expression,
		Result:     result,
	}
}

// evaluateExpression 简单的表达式计算（仅支持基本四则运算）
func evaluateExpression(expr string) (float64, error) {
	// 移除空格
	expr = strings.ReplaceAll(expr, " ", "")

	// 简单的计算逻辑，实际使用中建议使用专门的表达式解析库
	// 这里只做基本演示
	if strings.Contains(expr, "+") {
		parts := strings.Split(expr, "+")
		if len(parts) == 2 {
			a, err1 := strconv.ParseFloat(parts[0], 64)
			b, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 != nil || err2 != nil {
				return 0, fmt.Errorf("无效的数字")
			}
			return a + b, nil
		}
	}

	// 单个数字
	if result, err := strconv.ParseFloat(expr, 64); err == nil {
		return result, nil
	}

	return 0, fmt.Errorf("表达式格式错误")
}

// CalculateBMI BMI计算
func CalculateBMI(height, weight float64) (float64, string) {
	if height <= 0 || weight <= 0 {
		return 0, "身高和体重必须大于0"
	}

	// 将身高从cm转换为m
	heightInM := height / 100
	bmi := weight / (heightInM * heightInM)

	var category string
	switch {
	case bmi < 18.5:
		category = "偏瘦"
	case bmi < 24:
		category = "正常"
	case bmi < 28:
		category = "超重"
	default:
		category = "肥胖"
	}

	return math.Round(bmi*100) / 100, category
}

// CountText 文本统计
func CountText(text string) TextResult {
	lines := strings.Count(text, "\n") + 1
	if text == "" {
		lines = 0
	}

	words := len(strings.Fields(text))
	characters := utf8.RuneCountInString(text)
	charactersNoSpaces := utf8.RuneCountInString(strings.ReplaceAll(text, " ", ""))

	return TextResult{
		Characters:         characters,
		Words:              words,
		Lines:              lines,
		CharactersNoSpaces: charactersNoSpaces,
	}
}

// TestRegex 正则表达式测试
func TestRegex(pattern, text string) DataProcessResult {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return DataProcessResult{
			Input:  pattern,
			Output: text,
			Valid:  false,
			Error:  "正则表达式语法错误: " + err.Error(),
		}
	}

	matches := regex.FindAllString(text, -1)
	matchResult := fmt.Sprintf("匹配结果: %v\n匹配数量: %d", matches, len(matches))

	return DataProcessResult{
		Input:  pattern,
		Output: matchResult,
		Valid:  true,
	}
}

// GetWeather 获取天气信息（模拟实现）
func GetWeather(city string) WeatherResult {
	// 这里是模拟数据，实际项目中需要调用真实的天气API
	temperatures := map[string]float64{
		"北京": 25.5,
		"上海": 28.2,
		"广州": 32.1,
		"深圳": 31.8,
		"杭州": 26.9,
	}

	descriptions := map[string]string{
		"北京": "晴转多云",
		"上海": "多云",
		"广州": "雷阵雨",
		"深圳": "晴",
		"杭州": "小雨",
	}

	if temp, exists := temperatures[city]; exists {
		return WeatherResult{
			City:        city,
			Temperature: temp,
			Description: descriptions[city],
			Humidity:    65,
		}
	}

	return WeatherResult{
		City:  city,
		Error: "暂不支持该城市的天气查询",
	}
}

// ConvertUnit 单位转换
func ConvertUnit(value float64, fromUnit, toUnit, unitType string) (float64, error) {
	switch unitType {
	case "length":
		return convertLength(value, fromUnit, toUnit)
	case "weight":
		return convertWeight(value, fromUnit, toUnit)
	case "temperature":
		return convertTemperature(value, fromUnit, toUnit)
	default:
		return 0, fmt.Errorf("不支持的单位类型: %s", unitType)
	}
}

func convertLength(value float64, from, to string) (float64, error) {
	// 转换为米作为基准
	toMeter := map[string]float64{
		"mm": 0.001,
		"cm": 0.01,
		"m":  1.0,
		"km": 1000.0,
		"in": 0.0254,
		"ft": 0.3048,
	}

	fromFactor, fromExists := toMeter[from]
	toFactor, toExists := toMeter[to]

	if !fromExists || !toExists {
		return 0, fmt.Errorf("不支持的长度单位")
	}

	return value * fromFactor / toFactor, nil
}

func convertWeight(value float64, from, to string) (float64, error) {
	// 转换为克作为基准
	toGram := map[string]float64{
		"mg": 0.001,
		"g":  1.0,
		"kg": 1000.0,
		"oz": 28.3495,
		"lb": 453.592,
	}

	fromFactor, fromExists := toGram[from]
	toFactor, toExists := toGram[to]

	if !fromExists || !toExists {
		return 0, fmt.Errorf("不支持的重量单位")
	}

	return value * fromFactor / toFactor, nil
}

func convertTemperature(value float64, from, to string) (float64, error) {
	// 先转换为摄氏度
	var celsius float64
	switch from {
	case "C":
		celsius = value
	case "F":
		celsius = (value - 32) * 5 / 9
	case "K":
		celsius = value - 273.15
	default:
		return 0, fmt.Errorf("不支持的温度单位: %s", from)
	}

	// 再从摄氏度转换为目标单位
	switch to {
	case "C":
		return celsius, nil
	case "F":
		return celsius*9/5 + 32, nil
	case "K":
		return celsius + 273.15, nil
	default:
		return 0, fmt.Errorf("不支持的温度单位: %s", to)
	}
}
