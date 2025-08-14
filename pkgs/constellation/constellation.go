package constellation

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"blog"
	"control"
	"module"

	"github.com/google/uuid"
)

// ConstellationManager 星座管理器
type ConstellationManager struct {
	horoscopeGenerator    *HoroscopeGenerator
	divinationEngine      *DivinationEngine
	compatibilityAnalyzer *CompatibilityAnalyzer
}

// NewConstellationManager 创建新的星座管理器
func NewConstellationManager() *ConstellationManager {
	return &ConstellationManager{
		horoscopeGenerator:    NewHoroscopeGenerator(),
		divinationEngine:      NewDivinationEngine(),
		compatibilityAnalyzer: NewCompatibilityAnalyzer(),
	}
}

// GetConstellationByDate 根据日期获取星座
func (cm *ConstellationManager) GetConstellationByDate(birthDate string) (string, error) {
	// 解析日期 YYYY-MM-DD -> MM-DD
	parts := strings.Split(birthDate, "-")
	if len(parts) != 3 {
		return "", fmt.Errorf("日期格式错误，应为 YYYY-MM-DD")
	}

	monthDay := parts[1] + "-" + parts[2]

	// 遍历所有星座，找到匹配的日期范围
	for constellationID, info := range ConstellationData {
		if cm.isDateInRange(monthDay, info.DateRange) {
			return constellationID, nil
		}
	}

	return "", fmt.Errorf("无法确定星座")
}

// isDateInRange 检查日期是否在范围内
func (cm *ConstellationManager) isDateInRange(date string, dateRange DateRange) bool {
	// 处理跨年的情况（如摩羯座 12-22 到 01-19）
	if dateRange.Start > dateRange.End {
		return date >= dateRange.Start || date <= dateRange.End
	}
	return date >= dateRange.Start && date <= dateRange.End
}

// GetConstellationInfo 获取星座详细信息
func (cm *ConstellationManager) GetConstellationInfo(constellationID string) (*ConstellationInfo, error) {
	info, exists := ConstellationData[constellationID]
	if !exists {
		return nil, fmt.Errorf("星座不存在: %s", constellationID)
	}
	return &info, nil
}

// CreateBirthChart 创建个人星盘
func (cm *ConstellationManager) CreateBirthChart(userName, birthDate, birthTime, birthPlace string) (*BirthChart, error) {
	// 获取太阳星座
	sunSign, err := cm.GetConstellationByDate(birthDate)
	if err != nil {
		return nil, err
	}

	// 简化版星盘生成（实际应用中需要复杂的天文计算）
	chart := &BirthChart{
		ID:         generateID(),
		UserName:   userName,
		BirthDate:  birthDate,
		BirthTime:  birthTime,
		BirthPlace: birthPlace,
		SunSign:    sunSign,
		MoonSign:   cm.calculateMoonSign(birthDate, birthTime),
		RisingSign: cm.calculateRisingSign(birthDate, birthTime, birthPlace),
		Planetary:  cm.calculatePlanetaryPositions(birthDate, birthTime),
		Houses:     cm.calculateHouses(birthDate, birthTime, birthPlace),
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 保存星盘到博客系统
	err = cm.saveBirthChart(chart)
	if err != nil {
		return nil, err
	}

	return chart, nil
}

// GetDailyHoroscope 获取每日运势
func (cm *ConstellationManager) GetDailyHoroscope(constellationID, date string) (*DailyHoroscope, error) {
	// 先尝试从博客系统获取现有运势
	horoscope, err := cm.loadDailyHoroscope(constellationID, date)
	if err == nil {
		return horoscope, nil
	}

	// 如果不存在，则生成新的运势
	horoscope = cm.horoscopeGenerator.GenerateDailyHoroscope(constellationID, date)

	// 保存到博客系统
	err = cm.saveDailyHoroscope(horoscope)
	if err != nil {
		return nil, err
	}

	return horoscope, nil
}

// CreateDivination 创建占卜记录
func (cm *ConstellationManager) CreateDivination(userName, divinationType, question, method string) (*DivinationRecord, error) {
	// 生成占卜结果
	result, err := cm.divinationEngine.PerformDivination(divinationType, method, question)
	if err != nil {
		return nil, err
	}

	record := &DivinationRecord{
		ID:         generateID(),
		UserName:   userName,
		Type:       divinationType,
		Question:   question,
		Method:     method,
		Result:     *result,
		Accuracy:   0, // 初始为0，用户后续可以评价
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 保存占卜记录
	err = cm.saveDivinationRecord(record)
	if err != nil {
		return nil, err
	}

	return record, nil
}

// AnalyzeCompatibility 分析星座配对
func (cm *ConstellationManager) AnalyzeCompatibility(sign1, sign2 string) (*CompatibilityAnalysis, error) {
	// 检查星座是否存在
	if _, exists := ConstellationData[sign1]; !exists {
		return nil, fmt.Errorf("星座不存在: %s", sign1)
	}
	if _, exists := ConstellationData[sign2]; !exists {
		return nil, fmt.Errorf("星座不存在: %s", sign2)
	}

	analysis := cm.compatibilityAnalyzer.AnalyzeCompatibility(sign1, sign2)

	// 保存配对分析
	err := cm.saveCompatibilityAnalysis(analysis)
	if err != nil {
		return nil, err
	}

	return analysis, nil
}

// GetDivinationHistory 获取占卜历史
func (cm *ConstellationManager) GetDivinationHistory(userName string, limit int) ([]*DivinationRecord, error) {
	// 搜索用户的占卜记录
	blogList := make([]*module.Blog, 0)
	for _, b := range blog.GetBlogs() {
		if strings.HasPrefix(b.Title, "constellation-divination-") {
			blogList = append(blogList, b)
		}
	}

	var records []*DivinationRecord
	count := 0

	for _, b := range blogList {
		if count >= limit {
			break
		}

		// 解析占卜记录
		var record DivinationRecord
		err := json.Unmarshal([]byte(b.Content), &record)
		if err != nil {
			continue
		}

		// 筛选用户记录
		if record.UserName == userName {
			records = append(records, &record)
			count++
		}
	}

	return records, nil
}

// GetDivinationStats 获取占卜统计
func (cm *ConstellationManager) GetDivinationStats(userName string) (*DivinationStats, error) {
	records, err := cm.GetDivinationHistory(userName, 1000) // 获取所有记录
	if err != nil {
		return nil, err
	}

	stats := &DivinationStats{
		UserName:     userName,
		TotalCount:   len(records),
		TypeStats:    make(map[string]int),
		MonthlyStats: make(map[string]int),
	}

	var accuracySum float64
	accuracyCount := 0

	for _, record := range records {
		// 统计类型
		stats.TypeStats[record.Type]++

		// 统计月度数据
		month := record.CreateTime[:7] // YYYY-MM
		stats.MonthlyStats[month]++

		// 统计准确度
		if record.Accuracy > 0 {
			accuracySum += float64(record.Accuracy)
			accuracyCount++
		}

		// 记录最后占卜时间
		if record.CreateTime > stats.LastDivination {
			stats.LastDivination = record.CreateTime
		}
	}

	// 计算平均准确度
	if accuracyCount > 0 {
		stats.AccuracyAvg = accuracySum / float64(accuracyCount)
	}

	// 找出最常用的占卜类型
	maxCount := 0
	for divinationType, count := range stats.TypeStats {
		if count > maxCount {
			maxCount = count
			stats.FavoriteType = divinationType
		}
	}

	return stats, nil
}

// UpdateDivinationAccuracy 更新占卜准确度评价
func (cm *ConstellationManager) UpdateDivinationAccuracy(recordID string, accuracy int) error {
	// 查找对应的博客记录
	title := fmt.Sprintf("constellation-divination-%s", recordID)
	b := blog.GetBlog(title)
	if b == nil {
		return fmt.Errorf("占卜记录不存在")
	}

	// 解析记录
	var record DivinationRecord
	err := json.Unmarshal([]byte(b.Content), &record)
	if err != nil {
		return err
	}

	// 更新准确度
	record.Accuracy = accuracy

	// 重新保存
	return cm.saveDivinationRecord(&record)
}

// === 私有方法 ===

// 生成唯一ID
func generateID() string {
	return uuid.New().String()
}

// 计算月亮星座（简化版）
func (cm *ConstellationManager) calculateMoonSign(birthDate, birthTime string) string {
	// 实际应用需要复杂的天文计算，这里使用简化算法
	hash := sha256.Sum256([]byte(birthDate + birthTime + "moon"))
	constellations := []string{"aries", "taurus", "gemini", "cancer", "leo", "virgo",
		"libra", "scorpio", "sagittarius", "capricorn", "aquarius", "pisces"}
	return constellations[int(hash[0])%12]
}

// 计算上升星座（简化版）
func (cm *ConstellationManager) calculateRisingSign(birthDate, birthTime, birthPlace string) string {
	// 实际应用需要考虑出生地经纬度和时区
	hash := sha256.Sum256([]byte(birthDate + birthTime + birthPlace + "rising"))
	constellations := []string{"aries", "taurus", "gemini", "cancer", "leo", "virgo",
		"libra", "scorpio", "sagittarius", "capricorn", "aquarius", "pisces"}
	return constellations[int(hash[0])%12]
}

// 计算行星位置（简化版）
func (cm *ConstellationManager) calculatePlanetaryPositions(birthDate, birthTime string) PlanetaryPositions {
	// 实际应用需要精确的天体力学计算
	seed := birthDate + birthTime
	return PlanetaryPositions{
		Sun:     cm.getRandomConstellation(seed + "sun"),
		Moon:    cm.getRandomConstellation(seed + "moon"),
		Mercury: cm.getRandomConstellation(seed + "mercury"),
		Venus:   cm.getRandomConstellation(seed + "venus"),
		Mars:    cm.getRandomConstellation(seed + "mars"),
		Jupiter: cm.getRandomConstellation(seed + "jupiter"),
		Saturn:  cm.getRandomConstellation(seed + "saturn"),
		Uranus:  cm.getRandomConstellation(seed + "uranus"),
		Neptune: cm.getRandomConstellation(seed + "neptune"),
		Pluto:   cm.getRandomConstellation(seed + "pluto"),
	}
}

// 计算宫位（简化版）
func (cm *ConstellationManager) calculateHouses(birthDate, birthTime, birthPlace string) [12]HouseInfo {
	var houses [12]HouseInfo
	descriptions := []string{
		"第一宫：自我、外表、第一印象",
		"第二宫：金钱、价值观、天赋",
		"第三宫：沟通、兄弟姐妹、短途旅行",
		"第四宫：家庭、根基、内心世界",
		"第五宫：创造、恋爱、子女",
		"第六宫：工作、健康、日常习惯",
		"第七宫：伙伴、婚姻、合作",
		"第八宫：转化、共同资源、死亡",
		"第九宫：哲学、高等教育、远行",
		"第十宫：事业、声誉、社会地位",
		"第十一宫：朋友、团体、理想",
		"第十二宫：潜意识、精神、隐秘",
	}

	for i := 0; i < 12; i++ {
		houses[i] = HouseInfo{
			Number:      i + 1,
			Sign:        cm.getRandomConstellation(birthDate + birthTime + birthPlace + strconv.Itoa(i)),
			Degree:      rand.Intn(30), // 0-29度
			Description: descriptions[i],
		}
	}

	return houses
}

// 获取随机星座（基于种子）
func (cm *ConstellationManager) getRandomConstellation(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	constellations := []string{"aries", "taurus", "gemini", "cancer", "leo", "virgo",
		"libra", "scorpio", "sagittarius", "capricorn", "aquarius", "pisces"}
	return constellations[int(hash[0])%12]
}

// 保存个人星盘
func (cm *ConstellationManager) saveBirthChart(chart *BirthChart) error {
	title := fmt.Sprintf("constellation-birthchart-%s-%s", chart.UserName, chart.ID)
	content, _ := json.MarshalIndent(chart, "", "  ")

	ubd := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		Tags:     "constellation|birthchart",
		AuthType: module.EAuthType_private,
	}

	control.AddBlog(ubd)
	return nil
}

// 保存每日运势
func (cm *ConstellationManager) saveDailyHoroscope(horoscope *DailyHoroscope) error {
	title := fmt.Sprintf("horoscope-%s-%s", horoscope.Constellation, horoscope.Date)
	content, _ := json.MarshalIndent(horoscope, "", "  ")

	ubd := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		Tags:     "constellation|horoscope|daily",
		AuthType: module.EAuthType_public,
	}

	control.AddBlog(ubd)
	return nil
}

// 加载每日运势
func (cm *ConstellationManager) loadDailyHoroscope(constellationID, date string) (*DailyHoroscope, error) {
	title := fmt.Sprintf("horoscope-%s-%s", constellationID, date)
	b := blog.GetBlog(title)
	if b == nil {
		return nil, fmt.Errorf("运势不存在")
	}

	var horoscope DailyHoroscope
	err := json.Unmarshal([]byte(b.Content), &horoscope)
	if err != nil {
		return nil, err
	}

	return &horoscope, nil
}

// 保存占卜记录
func (cm *ConstellationManager) saveDivinationRecord(record *DivinationRecord) error {
	title := fmt.Sprintf("constellation-divination-%s-%s",
		record.UserName,
		time.Now().Format("2006-01-02-15-04-05"))
	content, _ := json.MarshalIndent(record, "", "  ")

	ubd := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		Tags:     fmt.Sprintf("constellation|divination|%s", record.Type),
		AuthType: module.EAuthType_private,
	}

	control.AddBlog(ubd)
	return nil
}

// 保存配对分析
func (cm *ConstellationManager) saveCompatibilityAnalysis(analysis *CompatibilityAnalysis) error {
	title := fmt.Sprintf("constellation-compatibility-%s-%s-%s",
		analysis.Person1, analysis.Person2, analysis.ID)
	content, _ := json.MarshalIndent(analysis, "", "  ")

	ubd := &module.UploadedBlogData{
		Title:    title,
		Content:  string(content),
		Tags:     "constellation|compatibility",
		AuthType: module.EAuthType_private,
	}

	control.AddBlog(ubd)
	return nil
}
