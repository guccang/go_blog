package tools

import (
	"encoding/json"
	"net/http"
	"strconv"
	"view"
)

// ToolsHandler 工具页面处理器
func ToolsHandler(w http.ResponseWriter, r *http.Request) {
	view.PageTools(w)
}

// TimeToolHandler 时间工具API
func TimeToolHandler(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	timezone := r.URL.Query().Get("timezone")
	
	w.Header().Set("Content-Type", "application/json")
	
	switch action {
	case "current":
		result := GetCurrentTime(timezone)
		json.NewEncoder(w).Encode(result)
	case "convert":
		timestampStr := r.URL.Query().Get("timestamp")
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "无效的时间戳"})
			return
		}
		result := ConvertTimestamp(timestamp)
		json.NewEncoder(w).Encode(result)
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的操作"})
	}
}

// DataProcessHandler 数据处理工具API
func DataProcessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	r.ParseForm()
	action := r.PostForm.Get("action")
	input := r.PostForm.Get("input")
	
	w.Header().Set("Content-Type", "application/json")
	
	var result DataProcessResult
	
	switch action {
	case "json_format":
		result = FormatJSON(input)
	case "base64_encode":
		result = EncodeBase64(input)
	case "base64_decode":
		result = DecodeBase64(input)
	case "url_encode":
		result = EncodeURL(input)
	case "url_decode":
		result = DecodeURL(input)
	case "md5":
		result = GenerateHash(input, "md5")
	case "sha1":
		result = GenerateHash(input, "sha1")
	case "sha256":
		result = GenerateHash(input, "sha256")
	default:
		result = DataProcessResult{
			Input:  input,
			Output: input,
			Valid:  false,
			Error:  "无效的操作",
		}
	}
	
	json.NewEncoder(w).Encode(result)
}

// CalculatorHandler 计算器API
func CalculatorHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	r.ParseForm()
	expression := r.PostForm.Get("expression")
	
	w.Header().Set("Content-Type", "application/json")
	
	result := Calculate(expression)
	json.NewEncoder(w).Encode(result)
}

// BMIHandler BMI计算API
func BMIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	r.ParseForm()
	heightStr := r.PostForm.Get("height")
	weightStr := r.PostForm.Get("weight")
	
	w.Header().Set("Content-Type", "application/json")
	
	height, err1 := strconv.ParseFloat(heightStr, 64)
	weight, err2 := strconv.ParseFloat(weightStr, 64)
	
	if err1 != nil || err2 != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "请输入有效的身高和体重"})
		return
	}
	
	bmi, category := CalculateBMI(height, weight)
	result := map[string]interface{}{
		"bmi":      bmi,
		"category": category,
		"height":   height,
		"weight":   weight,
	}
	json.NewEncoder(w).Encode(result)
}

// TextToolHandler 文本工具API
func TextToolHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	r.ParseForm()
	action := r.PostForm.Get("action")
	text := r.PostForm.Get("text")
	
	w.Header().Set("Content-Type", "application/json")
	
	switch action {
	case "count":
		result := CountText(text)
		json.NewEncoder(w).Encode(result)
	case "regex":
		pattern := r.PostForm.Get("pattern")
		result := TestRegex(pattern, text)
		json.NewEncoder(w).Encode(result)
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的操作"})
	}
}

// WeatherHandler 天气查询API
func WeatherHandler(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	
	w.Header().Set("Content-Type", "application/json")
	
	if city == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "请提供城市名称"})
		return
	}
	
	result := GetWeather(city)
	json.NewEncoder(w).Encode(result)
}

// UnitConvertHandler 单位转换API
func UnitConvertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	r.ParseForm()
	valueStr := r.PostForm.Get("value")
	fromUnit := r.PostForm.Get("from_unit")
	toUnit := r.PostForm.Get("to_unit")
	unitType := r.PostForm.Get("unit_type")
	
	w.Header().Set("Content-Type", "application/json")
	
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "请输入有效的数值"})
		return
	}
	
	result, err := ConvertUnit(value, fromUnit, toUnit, unitType)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	
	response := map[string]interface{}{
		"original_value": value,
		"from_unit":      fromUnit,
		"to_unit":        toUnit,
		"converted_value": result,
		"unit_type":      unitType,
	}
	json.NewEncoder(w).Encode(response)
}