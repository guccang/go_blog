package constellation

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// HoroscopeGenerator 运势生成器
type HoroscopeGenerator struct {
	dateFactors      map[string]float64 // 日期影响因子
	planetaryAspects map[string]float64 // 行星相位影响
	seasonalBonus    map[string]float64 // 季节加成
	weekdayModifier  map[string]float64 // 星期影响
	randomSource     *rand.Rand
}

// NewHoroscopeGenerator 创建运势生成器
func NewHoroscopeGenerator() *HoroscopeGenerator {
	return &HoroscopeGenerator{
		dateFactors:      initDateFactors(),
		planetaryAspects: initPlanetaryAspects(),
		seasonalBonus:    initSeasonalBonus(),
		weekdayModifier:  initWeekdayModifier(),
		randomSource:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateDailyHoroscope 生成每日运势
func (hg *HoroscopeGenerator) GenerateDailyHoroscope(constellationID, date string) *DailyHoroscope {
	parsedDate, _ := time.Parse("2006-01-02", date)

	// 计算各项运势分数
	overall := hg.calculateOverallScore(constellationID, parsedDate)
	love := hg.calculateLoveScore(constellationID, parsedDate)
	career := hg.calculateCareerScore(constellationID, parsedDate)
	money := hg.calculateMoneyScore(constellationID, parsedDate)
	health := hg.calculateHealthScore(constellationID, parsedDate)

	// 生成幸运元素
	luckyColor := hg.generateLuckyColor(constellationID, parsedDate)
	luckyNumber := hg.generateLuckyNumber(constellationID, parsedDate)

	// 生成运势建议和描述
	advice := hg.generateAdvice(constellationID, overall, parsedDate)
	description := hg.generateDescription(constellationID, overall, love, career, money, health, parsedDate)

	return &DailyHoroscope{
		ID:            generateID(),
		Constellation: constellationID,
		Date:          date,
		Overall:       overall,
		Love:          love,
		Career:        career,
		Money:         money,
		Health:        health,
		LuckyColor:    luckyColor,
		LuckyNumber:   luckyNumber,
		Advice:        advice,
		Description:   description,
		CreateTime:    time.Now().Format("2006-01-02 15:04:05"),
	}
}

// === 运势计算方法 ===

// 计算综合运势
func (hg *HoroscopeGenerator) calculateOverallScore(constellationID string, date time.Time) int {
	base := hg.getBaseScore(constellationID)
	planetary := hg.getPlanetaryInfluence(constellationID, date)
	seasonal := hg.getSeasonalBonus(constellationID, date)
	weekday := hg.getWeekdayModifier(date)
	random := (hg.randomSource.Float64() - 0.5) * 0.4 // ±20%随机波动

	score := base + planetary + seasonal + weekday + random
	return hg.normalizeScore(score)
}

// 计算爱情运势
func (hg *HoroscopeGenerator) calculateLoveScore(constellationID string, date time.Time) int {
	base := hg.getLoveBaseScore(constellationID)

	// 金星影响（爱情主星）
	venusInfluence := hg.getVenusInfluence(date)

	// 月相影响
	moonPhase := hg.getMoonPhaseInfluence(date)

	// 星期影响（周五和周末爱情运势更好）
	weekdayBonus := 0.0
	weekday := date.Weekday()
	if weekday == time.Friday || weekday == time.Saturday || weekday == time.Sunday {
		weekdayBonus = 0.3
	}

	random := (hg.randomSource.Float64() - 0.5) * 0.3

	score := base + venusInfluence + moonPhase + weekdayBonus + random
	return hg.normalizeScore(score)
}

// 计算事业运势
func (hg *HoroscopeGenerator) calculateCareerScore(constellationID string, date time.Time) int {
	base := hg.getCareerBaseScore(constellationID)

	// 土星影响（事业主星）
	saturnInfluence := hg.getSaturnInfluence(date)

	// 工作日影响
	weekdayBonus := 0.0
	weekday := date.Weekday()
	if weekday >= time.Monday && weekday <= time.Friday {
		weekdayBonus = 0.2
	}

	// 月初月末影响（通常工作压力不同）
	monthPeriod := hg.getMonthPeriodInfluence(date)

	random := (hg.randomSource.Float64() - 0.5) * 0.3

	score := base + saturnInfluence + weekdayBonus + monthPeriod + random
	return hg.normalizeScore(score)
}

// 计算财运
func (hg *HoroscopeGenerator) calculateMoneyScore(constellationID string, date time.Time) int {
	base := hg.getMoneyBaseScore(constellationID)

	// 木星影响（财富主星）
	jupiterInfluence := hg.getJupiterInfluence(date)

	// 季节影响（年底通常财运相关活动多）
	seasonalInfluence := hg.getFinancialSeasonalInfluence(date)

	random := (hg.randomSource.Float64() - 0.5) * 0.4

	score := base + jupiterInfluence + seasonalInfluence + random
	return hg.normalizeScore(score)
}

// 计算健康运势
func (hg *HoroscopeGenerator) calculateHealthScore(constellationID string, date time.Time) int {
	base := hg.getHealthBaseScore(constellationID)

	// 季节影响（不同季节健康关注点不同）
	seasonalHealth := hg.getHealthSeasonalInfluence(date)

	// 星期影响（周末通常休息更好）
	weekdayInfluence := 0.0
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		weekdayInfluence = 0.2
	}

	random := (hg.randomSource.Float64() - 0.5) * 0.2

	score := base + seasonalHealth + weekdayInfluence + random
	return hg.normalizeScore(score)
}

// === 基础分数计算 ===

func (hg *HoroscopeGenerator) getBaseScore(constellationID string) float64 {
	// 基于星座特性的基础分数
	baseScores := map[string]float64{
		"aries":       3.5, // 白羊座，精力充沛
		"taurus":      3.2, // 金牛座，稳定但缓慢
		"gemini":      3.6, // 双子座，变化多端
		"cancer":      3.1, // 巨蟹座，情绪化
		"leo":         3.8, // 狮子座，自信阳光
		"virgo":       3.3, // 处女座，谨慎细心
		"libra":       3.4, // 天秤座，寻求平衡
		"scorpio":     3.7, // 天蝎座，强度高
		"sagittarius": 3.9, // 射手座，乐观冒险
		"capricorn":   3.2, // 摩羯座，务实稳重
		"aquarius":    3.5, // 水瓶座，独立创新
		"pisces":      3.0, // 双鱼座，敏感梦幻
	}

	if score, exists := baseScores[constellationID]; exists {
		return score
	}
	return 3.0
}

func (hg *HoroscopeGenerator) getLoveBaseScore(constellationID string) float64 {
	// 基于星座在爱情方面的特性
	loveScores := map[string]float64{
		"aries":       3.4, // 热情但冲动
		"taurus":      3.8, // 忠诚稳定
		"gemini":      3.2, // 多变不定
		"cancer":      4.0, // 重感情
		"leo":         3.6, // 需要被赞美
		"virgo":       3.1, // 挑剔谨慎
		"libra":       4.2, // 天生浪漫
		"scorpio":     3.9, // 深情专一
		"sagittarius": 3.0, // 渴望自由
		"capricorn":   3.3, // 保守传统
		"aquarius":    2.8, // 独立理性
		"pisces":      4.5, // 浪漫梦幻
	}

	if score, exists := loveScores[constellationID]; exists {
		return score
	}
	return 3.0
}

func (hg *HoroscopeGenerator) getCareerBaseScore(constellationID string) float64 {
	careerScores := map[string]float64{
		"aries":       4.0, // 领导力强
		"taurus":      3.6, // 稳定可靠
		"gemini":      3.4, // 沟通能力强
		"cancer":      3.2, // 关怀他人
		"leo":         4.2, // 天生领袖
		"virgo":       4.0, // 细致专业
		"libra":       3.5, // 协调能力
		"scorpio":     3.8, // 专注执着
		"sagittarius": 3.3, // 视野开阔
		"capricorn":   4.5, // 事业心强
		"aquarius":    3.7, // 创新思维
		"pisces":      2.9, // 理想主义
	}

	if score, exists := careerScores[constellationID]; exists {
		return score
	}
	return 3.0
}

func (hg *HoroscopeGenerator) getMoneyBaseScore(constellationID string) float64 {
	moneyScores := map[string]float64{
		"aries":       3.2, // 冲动消费
		"taurus":      4.2, // 理财能力强
		"gemini":      3.0, // 花钱随性
		"cancer":      3.8, // 节俭储蓄
		"leo":         2.8, // 爱面子消费
		"virgo":       4.0, // 精打细算
		"libra":       3.1, // 为美花钱
		"scorpio":     3.9, // 投资敏锐
		"sagittarius": 2.7, // 花钱大手大脚
		"capricorn":   4.5, // 财务规划强
		"aquarius":    3.3, // 理性消费
		"pisces":      2.9, // 情绪化消费
	}

	if score, exists := moneyScores[constellationID]; exists {
		return score
	}
	return 3.0
}

func (hg *HoroscopeGenerator) getHealthBaseScore(constellationID string) float64 {
	healthScores := map[string]float64{
		"aries":       3.8, // 精力充沛但易受伤
		"taurus":      3.5, // 体质好但懒散
		"gemini":      3.2, // 神经紧张
		"cancer":      3.0, // 情绪影响健康
		"leo":         3.9, // 活力四射
		"virgo":       4.2, // 注重健康养生
		"libra":       3.4, // 追求平衡
		"scorpio":     3.7, // 恢复力强
		"sagittarius": 4.0, // 爱运动
		"capricorn":   3.3, // 容易过劳
		"aquarius":    3.6, // 精神健康重要
		"pisces":      3.1, // 敏感体质
	}

	if score, exists := healthScores[constellationID]; exists {
		return score
	}
	return 3.0
}

// === 行星影响计算 ===

func (hg *HoroscopeGenerator) getPlanetaryInfluence(constellationID string, date time.Time) float64 {
	// 模拟行星影响（实际应用需要真实的天体计算）
	dayOfYear := date.YearDay()

	// 不同行星的周期影响
	sunCycle := math.Sin(float64(dayOfYear)*2*math.Pi/365) * 0.2
	moonCycle := math.Sin(float64(date.Day())*2*math.Pi/29.5) * 0.15
	mercuryCycle := math.Sin(float64(dayOfYear)*2*math.Pi/88) * 0.1

	return sunCycle + moonCycle + mercuryCycle
}

func (hg *HoroscopeGenerator) getVenusInfluence(date time.Time) float64 {
	// 金星周期约225天
	dayOfYear := date.YearDay()
	return math.Sin(float64(dayOfYear)*2*math.Pi/225) * 0.3
}

func (hg *HoroscopeGenerator) getSaturnInfluence(date time.Time) float64 {
	// 土星长周期影响，更多基于月份
	month := int(date.Month())
	saturnPhases := []float64{0.1, 0.2, 0.0, -0.1, 0.15, 0.25, 0.3, 0.2, 0.1, -0.05, 0.0, 0.1}
	return saturnPhases[month-1]
}

func (hg *HoroscopeGenerator) getJupiterInfluence(date time.Time) float64 {
	// 木星12年周期，但这里简化为月度影响
	month := int(date.Month())
	jupiterPhases := []float64{0.2, 0.15, 0.25, 0.3, 0.35, 0.2, 0.1, 0.05, 0.15, 0.25, 0.3, 0.4}
	return jupiterPhases[month-1]
}

func (hg *HoroscopeGenerator) getMoonPhaseInfluence(date time.Time) float64 {
	// 简化的月相计算
	day := date.Day()
	phase := float64(day) / 29.5

	// 新月到满月的影响
	if phase < 0.5 {
		return phase * 0.4 // 新月到满月，递增影响
	} else {
		return (1.0 - phase) * 0.4 // 满月到新月，递减影响
	}
}

// === 季节和时间影响 ===

func (hg *HoroscopeGenerator) getSeasonalBonus(constellationID string, date time.Time) float64 {
	month := int(date.Month())

	// 每个星座在对应月份获得额外加成
	seasonalMap := map[string][]int{
		"aries":       {3, 4},   // 春分时节
		"taurus":      {4, 5},   // 春末
		"gemini":      {5, 6},   // 初夏
		"cancer":      {6, 7},   // 夏至
		"leo":         {7, 8},   // 盛夏
		"virgo":       {8, 9},   // 夏末
		"libra":       {9, 10},  // 秋分
		"scorpio":     {10, 11}, // 深秋
		"sagittarius": {11, 12}, // 初冬
		"capricorn":   {12, 1},  // 冬至
		"aquarius":    {1, 2},   // 深冬
		"pisces":      {2, 3},   // 冬末
	}

	if months, exists := seasonalMap[constellationID]; exists {
		for _, m := range months {
			if m == month {
				return 0.3 // 本命月加成
			}
		}
	}

	return 0.0
}

func (hg *HoroscopeGenerator) getWeekdayModifier(date time.Time) float64 {
	weekday := date.Weekday()
	modifiers := map[time.Weekday]float64{
		time.Monday:    -0.1, // 周一综合运势略低
		time.Tuesday:   0.0,
		time.Wednesday: 0.1,
		time.Thursday:  0.15,
		time.Friday:    0.2,  // 周五运势较好
		time.Saturday:  0.25, // 周末运势好
		time.Sunday:    0.15,
	}

	return modifiers[weekday]
}

func (hg *HoroscopeGenerator) getFinancialSeasonalInfluence(date time.Time) float64 {
	month := int(date.Month())

	// 年底财运相关活动多，春节前后消费多
	financialSeasons := map[int]float64{
		1:  0.2,  // 年初规划
		2:  -0.1, // 春节消费
		3:  0.1,  // 恢复期
		4:  0.0,
		5:  0.0,
		6:  0.1, // 年中
		7:  0.0,
		8:  0.0,
		9:  0.1,
		10: 0.1,
		11: 0.2, // 年底冲刺
		12: 0.3, // 年终奖金
	}

	return financialSeasons[month]
}

func (hg *HoroscopeGenerator) getHealthSeasonalInfluence(date time.Time) float64 {
	month := int(date.Month())

	// 季节对健康的影响
	healthSeasons := map[int]float64{
		1:  -0.1, // 冬季流感
		2:  -0.1, // 冬季抑郁
		3:  0.2,  // 春季复苏
		4:  0.3,  // 春暖花开
		5:  0.2,  // 春夏之交
		6:  0.1,  // 初夏
		7:  0.0,  // 盛夏炎热
		8:  -0.1, // 酷暑
		9:  0.2,  // 秋高气爽
		10: 0.3,  // 秋季最佳
		11: 0.1,  // 深秋
		12: -0.1, // 冬季开始
	}

	return healthSeasons[month]
}

func (hg *HoroscopeGenerator) getMonthPeriodInfluence(date time.Time) float64 {
	day := date.Day()

	if day <= 7 {
		return 0.1 // 月初，新开始
	} else if day <= 15 {
		return 0.15 // 月中，进展期
	} else if day <= 23 {
		return 0.05 // 月中后期
	} else {
		return -0.05 // 月末，总结期
	}
}

// === 幸运元素生成 ===

func (hg *HoroscopeGenerator) generateLuckyColor(constellationID string, date time.Time) string {
	info, exists := ConstellationData[constellationID]
	if !exists || len(info.Colors) == 0 {
		return "白色"
	}

	// 基于日期选择幸运色
	colorIndex := (date.Day() + int(date.Month())) % len(info.Colors)
	return info.Colors[colorIndex]
}

func (hg *HoroscopeGenerator) generateLuckyNumber(constellationID string, date time.Time) int {
	info, exists := ConstellationData[constellationID]
	if !exists || len(info.Numbers) == 0 {
		return date.Day()%50 + 1
	}

	// 基于日期选择幸运数字
	numberIndex := (date.Day() + int(date.Weekday())) % len(info.Numbers)
	return info.Numbers[numberIndex]
}

// === 建议和描述生成 ===

func (hg *HoroscopeGenerator) generateAdvice(constellationID string, overall int, date time.Time) string {
	adviceTemplates := map[int][]string{
		1: { // 运势较低
			"今天适合低调行事，避免重大决策",
			"多关注内心感受，给灵魂一些休息时间",
			"困难是暂时的，保持耐心等待转机",
		},
		2: {
			"今天需要更多耐心，避免急躁情绪",
			"适合整理思绪，为未来做准备",
			"与朋友交流可能会带来意外收获",
		},
		3: {
			"保持平衡心态，稳步前进",
			"今天适合处理日常事务",
			"关注细节，避免粗心大意",
		},
		4: {
			"今天是展现能力的好时机",
			"自信面对挑战，相信自己的判断",
			"积极与人合作，会有不错的成果",
		},
		5: { // 运势极佳
			"今天是行动的最佳时机，勇敢追求目标",
			"运势极佳，可以考虑重要决策",
			"把握机会，今天的努力会有丰厚回报",
		},
	}

	templates := adviceTemplates[overall]
	if len(templates) == 0 {
		return "保持积极心态，相信美好会到来"
	}

	// 基于日期选择建议
	index := date.Day() % len(templates)
	return templates[index]
}

func (hg *HoroscopeGenerator) generateDescription(constellationID string, overall, love, career, money, health int, date time.Time) string {
	info, _ := ConstellationData[constellationID]

	description := fmt.Sprintf("【%s今日运势】\n\n", info.ChineseName)

	// 综合运势描述
	overallDesc := hg.getScoreDescription(overall, "综合")
	description += fmt.Sprintf("综合运势：%s (%d/5星)\n", overallDesc, overall)

	// 各项运势描述
	loveDesc := hg.getScoreDescription(love, "爱情")
	description += fmt.Sprintf("爱情运势：%s (%d/5星)\n", loveDesc, love)

	careerDesc := hg.getScoreDescription(career, "事业")
	description += fmt.Sprintf("事业运势：%s (%d/5星)\n", careerDesc, career)

	moneyDesc := hg.getScoreDescription(money, "财运")
	description += fmt.Sprintf("财运指数：%s (%d/5星)\n", moneyDesc, money)

	healthDesc := hg.getScoreDescription(health, "健康")
	description += fmt.Sprintf("健康运势：%s (%d/5星)\n\n", healthDesc, health)

	// 添加星座特色描述
	description += hg.generateConstellationSpecificDescription(constellationID, date)

	return description
}

func (hg *HoroscopeGenerator) getScoreDescription(score int, category string) string {
	descriptions := map[int]map[string]string{
		1: {
			"综合": "需要格外小心",
			"爱情": "感情波动较大",
			"事业": "工作压力山大",
			"财运": "理财需谨慎",
			"健康": "注意休息调养",
		},
		2: {
			"综合": "平平淡淡",
			"爱情": "感情较为平淡",
			"事业": "工作进展缓慢",
			"财运": "财务状况一般",
			"健康": "身体状态平常",
		},
		3: {
			"综合": "中规中矩",
			"爱情": "感情稳定发展",
			"事业": "工作稳步前进",
			"财运": "收支基本平衡",
			"健康": "身心状态良好",
		},
		4: {
			"综合": "运势不错",
			"爱情": "桃花运旺盛",
			"事业": "工作表现出色",
			"财运": "财运亨通",
			"健康": "精神饱满",
		},
		5: {
			"综合": "运势爆棚",
			"爱情": "爱情甜蜜如蜜",
			"事业": "事业如虹",
			"财运": "财源滚滚",
			"健康": "活力四射",
		},
	}

	if categoryDesc, exists := descriptions[score]; exists {
		if desc, exists := categoryDesc[category]; exists {
			return desc
		}
	}

	return "运势平常"
}

func (hg *HoroscopeGenerator) generateConstellationSpecificDescription(constellationID string, date time.Time) string {
	specificDesc := map[string][]string{
		"aries": {
			"白羊座的你今天充满行动力，但要注意控制脾气",
			"勇敢追求目标，但记得听取他人建议",
			"领导才能今天特别突出，适合主导项目",
		},
		"taurus": {
			"金牛座的稳重今天特别有用，坚持就是胜利",
			"物质享受和美食可能带来额外快乐",
			"财务直觉敏锐，适合理财规划",
		},
		"gemini": {
			"双子座的沟通能力今天大放异彩",
			"好奇心驱使你探索新领域",
			"多变的思维今天带来创意灵感",
		},
		"cancer": {
			"巨蟹座的直觉今天特别准确",
			"家庭和情感需求更加突出",
			"照顾他人的天性今天得到认可",
		},
		"leo": {
			"狮子座的魅力今天无人能挡",
			"表现欲强烈，适合展示才华",
			"慷慨的本性为你带来好人缘",
		},
		"virgo": {
			"处女座的细致今天帮你避免错误",
			"完美主义倾向要适度控制",
			"分析能力出众，适合解决复杂问题",
		},
		"libra": {
			"天秤座的平衡感今天特别重要",
			"美感和艺术天赋得到发挥",
			"协调能力帮助化解冲突",
		},
		"scorpio": {
			"天蝎座的直觉今天特别敏锐",
			"深度思考带来重要洞察",
			"神秘魅力吸引他人注意",
		},
		"sagittarius": {
			"射手座的乐观今天感染周围的人",
			"探险精神驱使你尝试新事物",
			"哲学思考带来人生感悟",
		},
		"capricorn": {
			"摩羯座的责任感今天得到重视",
			"务实的方法帮助达成目标",
			"长远规划能力今天特别有用",
		},
		"aquarius": {
			"水瓶座的独特视角今天很有价值",
			"创新思维带来突破性想法",
			"人道主义关怀温暖他人",
		},
		"pisces": {
			"双鱼座的想象力今天特别丰富",
			"艺术灵感和创作欲望强烈",
			"同情心和直觉帮助理解他人",
		},
	}

	if descriptions, exists := specificDesc[constellationID]; exists {
		index := date.Day() % len(descriptions)
		return descriptions[index]
	}

	return "今天是展现你独特魅力的好日子。"
}

// === 辅助方法 ===

// 归一化分数到1-5范围
func (hg *HoroscopeGenerator) normalizeScore(score float64) int {
	normalized := int(math.Round(score))
	if normalized < 1 {
		return 1
	}
	if normalized > 5 {
		return 5
	}
	return normalized
}

// === 初始化配置数据 ===

func initDateFactors() map[string]float64 {
	return map[string]float64{
		"new_year": 0.3,
		"spring":   0.2,
		"summer":   0.1,
		"autumn":   0.15,
		"winter":   -0.1,
	}
}

func initPlanetaryAspects() map[string]float64 {
	return map[string]float64{
		"mercury_retrograde": -0.2,
		"venus_favorable":    0.3,
		"mars_strong":        0.2,
		"jupiter_blessing":   0.4,
		"saturn_challenge":   -0.1,
	}
}

func initSeasonalBonus() map[string]float64 {
	return map[string]float64{
		"spring_equinox":  0.25,
		"summer_solstice": 0.2,
		"autumn_equinox":  0.15,
		"winter_solstice": 0.1,
	}
}

func initWeekdayModifier() map[string]float64 {
	return map[string]float64{
		"monday":    -0.1,
		"tuesday":   0.0,
		"wednesday": 0.1,
		"thursday":  0.1,
		"friday":    0.2,
		"saturday":  0.25,
		"sunday":    0.15,
	}
}
