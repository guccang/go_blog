package constellation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"view"
)

var manager *ConstellationManager

func init() {
	manager = NewConstellationManager()
}

// HandleConstellation 星座占卜主页
func HandleConstellation(w http.ResponseWriter, r *http.Request) {
	// 渲染星座占卜主页
	view.PageConstellation(w)
}

// HandleDailyHoroscope 获取每日运势
func HandleDailyHoroscope(w http.ResponseWriter, r *http.Request) {
	constellationID := r.URL.Query().Get("constellation")
	date := r.URL.Query().Get("date")

	if constellationID == "" {
		http.Error(w, "缺少星座参数", http.StatusBadRequest)
		return
	}

	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	horoscope, err := manager.GetDailyHoroscope(constellationID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(horoscope)
}

// HandleBirthChart 创建个人星盘
func HandleBirthChart(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		var request struct {
			UserName   string `json:"user_name"`
			BirthDate  string `json:"birth_date"`
			BirthTime  string `json:"birth_time"`
			BirthPlace string `json:"birth_place"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "请求格式错误", http.StatusBadRequest)
			return
		}

		chart, err := manager.CreateBirthChart(
			request.UserName,
			request.BirthDate,
			request.BirthTime,
			request.BirthPlace,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chart)
	} else {
		http.Error(w, "仅支持POST请求", http.StatusMethodNotAllowed)
	}
}

// HandleDivination 占卜功能
func HandleDivination(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		var request struct {
			UserName string `json:"user_name"`
			Type     string `json:"type"` // tarot, astrology, numerology
			Question string `json:"question"`
			Method   string `json:"method"` // single_card, three_card, celtic_cross
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "请求格式错误", http.StatusBadRequest)
			return
		}

		record, err := manager.CreateDivination(
			request.UserName,
			request.Type,
			request.Question,
			request.Method,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(record)
	} else {
		http.Error(w, "仅支持POST请求", http.StatusMethodNotAllowed)
	}
}

// HandleCompatibility 星座配对分析
func HandleCompatibility(w http.ResponseWriter, r *http.Request) {

	sign1 := r.URL.Query().Get("sign1")
	sign2 := r.URL.Query().Get("sign2")

	if sign1 == "" || sign2 == "" {
		http.Error(w, "缺少星座参数", http.StatusBadRequest)
		return
	}

	analysis, err := manager.AnalyzeCompatibility(sign1, sign2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}

// HandleDivinationHistory 获取占卜历史
func HandleDivinationHistory(w http.ResponseWriter, r *http.Request) {

	userName := r.URL.Query().Get("user_name")
	limitStr := r.URL.Query().Get("limit")

	if userName == "" {
		http.Error(w, "缺少用户名参数", http.StatusBadRequest)
		return
	}

	limit := 10 // 默认返回10条记录
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	records, err := manager.GetDivinationHistory(userName, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// HandleDivinationStats 获取占卜统计
func HandleDivinationStats(w http.ResponseWriter, r *http.Request) {

	userName := r.URL.Query().Get("user_name")
	if userName == "" {
		http.Error(w, "缺少用户名参数", http.StatusBadRequest)
		return
	}

	stats, err := manager.GetDivinationStats(userName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleConstellationInfo 获取星座信息
func HandleConstellationInfo(w http.ResponseWriter, r *http.Request) {
	constellationID := r.URL.Query().Get("id")

	if constellationID == "" {
		// 返回所有星座信息
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ConstellationData)
		return
	}

	info, err := manager.GetConstellationInfo(constellationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// HandleGetConstellationByDate 根据出生日期获取星座
func HandleGetConstellationByDate(w http.ResponseWriter, r *http.Request) {
	birthDate := r.URL.Query().Get("birth_date")
	if birthDate == "" {
		http.Error(w, "缺少出生日期参数", http.StatusBadRequest)
		return
	}

	constellationID, err := manager.GetConstellationByDate(birthDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := manager.GetConstellationInfo(constellationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"constellation_id":   constellationID,
		"constellation_info": info,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleUpdateAccuracy 更新占卜准确度
func HandleUpdateAccuracy(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		var request struct {
			RecordID string `json:"record_id"`
			Accuracy int    `json:"accuracy"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "请求格式错误", http.StatusBadRequest)
			return
		}

		if request.Accuracy < 1 || request.Accuracy > 5 {
			http.Error(w, "准确度评分应在1-5之间", http.StatusBadRequest)
			return
		}

		err := manager.UpdateDivinationAccuracy(request.RecordID, request.Accuracy)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	} else {
		http.Error(w, "仅支持POST请求", http.StatusMethodNotAllowed)
	}
}

// HandleBatchHoroscope 批量获取多个星座的每日运势
func HandleBatchHoroscope(w http.ResponseWriter, r *http.Request) {

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 获取所有星座的运势
	allHoroscopes := make(map[string]*DailyHoroscope)

	for constellationID := range ConstellationData {
		horoscope, err := manager.GetDailyHoroscope(constellationID, date)
		if err != nil {
			// 记录错误但继续处理其他星座
			fmt.Printf("获取%s运势失败: %v\n", constellationID, err)
			continue
		}
		allHoroscopes[constellationID] = horoscope
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allHoroscopes)
}
