package constellation

import (
	"fmt"
	"time"
)

// CompatibilityAnalyzer 星座配对分析器
type CompatibilityAnalyzer struct {
	elementMatrix   map[string]map[string]float64 // 元素兼容性矩阵
	qualityMatrix   map[string]map[string]float64 // 性质兼容性矩阵
	rulerMatrix     map[string]map[string]float64 // 守护星兼容性矩阵
	statisticalData map[string]map[string]float64 // 统计数据兼容性
}

// NewCompatibilityAnalyzer 创建配对分析器
func NewCompatibilityAnalyzer() *CompatibilityAnalyzer {
	return &CompatibilityAnalyzer{
		elementMatrix:   initElementMatrix(),
		qualityMatrix:   initQualityMatrix(),
		rulerMatrix:     initRulerMatrix(),
		statisticalData: initStatisticalData(),
	}
}

// AnalyzeCompatibility 分析星座配对兼容性
func (ca *CompatibilityAnalyzer) AnalyzeCompatibility(sign1, sign2 string) *CompatibilityAnalysis {
	info1, _ := ConstellationData[sign1]
	info2, _ := ConstellationData[sign2]

	// 计算各维度兼容性分数
	elementScore := ca.calculateElementCompatibility(info1.Element, info2.Element)
	qualityScore := ca.calculateQualityCompatibility(info1.Quality, info2.Quality)
	rulerScore := ca.calculateRulerCompatibility(info1.Ruler, info2.Ruler)
	statisticalScore := ca.getStatisticalCompatibility(sign1, sign2)

	// 计算综合兼容性分数
	overallScore := ca.calculateOverallScore(elementScore, qualityScore, rulerScore, statisticalScore)
	loveScore := ca.calculateLoveScore(sign1, sign2, elementScore, qualityScore)
	friendScore := ca.calculateFriendScore(sign1, sign2, elementScore, qualityScore)
	workScore := ca.calculateWorkScore(sign1, sign2, qualityScore, rulerScore)

	// 生成详细分析
	analysis := ca.generateDetailedAnalysis(sign1, sign2, elementScore, qualityScore, rulerScore)
	advantages := ca.generateAdvantages(sign1, sign2, elementScore, qualityScore)
	challenges := ca.generateChallenges(sign1, sign2, elementScore, qualityScore)
	suggestions := ca.generateSuggestions(sign1, sign2, challenges)

	return &CompatibilityAnalysis{
		ID:           generateID(),
		Person1:      sign1,
		Person2:      sign2,
		OverallScore: overallScore,
		LoveScore:    loveScore,
		FriendScore:  friendScore,
		WorkScore:    workScore,
		Analysis:     analysis,
		Advantages:   advantages,
		Challenges:   challenges,
		Suggestions:  suggestions,
		CreateTime:   time.Now().Format("2006-01-02 15:04:05"),
	}
}

// === 兼容性计算方法 ===

// 计算元素兼容性
func (ca *CompatibilityAnalyzer) calculateElementCompatibility(element1, element2 string) float64 {
	if matrix, exists := ca.elementMatrix[element1]; exists {
		if score, exists := matrix[element2]; exists {
			return score
		}
	}
	return 0.5 // 默认中等兼容性
}

// 计算性质兼容性
func (ca *CompatibilityAnalyzer) calculateQualityCompatibility(quality1, quality2 string) float64 {
	if matrix, exists := ca.qualityMatrix[quality1]; exists {
		if score, exists := matrix[quality2]; exists {
			return score
		}
	}
	return 0.5 // 默认中等兼容性
}

// 计算守护星兼容性
func (ca *CompatibilityAnalyzer) calculateRulerCompatibility(ruler1, ruler2 string) float64 {
	if matrix, exists := ca.rulerMatrix[ruler1]; exists {
		if score, exists := matrix[ruler2]; exists {
			return score
		}
	}
	return 0.5 // 默认中等兼容性
}

// 获取统计数据兼容性
func (ca *CompatibilityAnalyzer) getStatisticalCompatibility(sign1, sign2 string) float64 {
	if matrix, exists := ca.statisticalData[sign1]; exists {
		if score, exists := matrix[sign2]; exists {
			return score
		}
	}
	// 尝试反向查找
	if matrix, exists := ca.statisticalData[sign2]; exists {
		if score, exists := matrix[sign1]; exists {
			return score
		}
	}
	return 0.5 // 默认中等兼容性
}

// 计算综合兼容性分数
func (ca *CompatibilityAnalyzer) calculateOverallScore(element, quality, ruler, statistical float64) float64 {
	// 加权平均
	score := element*0.35 + quality*0.25 + ruler*0.20 + statistical*0.20
	return score * 100 // 转换为百分制
}

// 计算爱情兼容性分数
func (ca *CompatibilityAnalyzer) calculateLoveScore(sign1, sign2 string, element, quality float64) float64 {
	base := (element*0.4 + quality*0.3) * 100

	// 特定星座组合的爱情加成/减成
	loveBonus := ca.getLoveBonus(sign1, sign2)

	score := base + loveBonus
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}

// 计算友情兼容性分数
func (ca *CompatibilityAnalyzer) calculateFriendScore(sign1, sign2 string, element, quality float64) float64 {
	base := (element*0.3 + quality*0.4) * 100

	// 友情通常比爱情更容易维持
	friendBonus := 5.0

	score := base + friendBonus
	if score > 100 {
		score = 100
	}

	return score
}

// 计算工作兼容性分数
func (ca *CompatibilityAnalyzer) calculateWorkScore(sign1, sign2 string, quality, ruler float64) float64 {
	base := (quality*0.5 + ruler*0.3) * 100

	// 工作兼容性更注重性质和守护星影响
	workBonus := ca.getWorkBonus(sign1, sign2)

	score := base + workBonus
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}

// === 详细分析生成 ===

func (ca *CompatibilityAnalyzer) generateDetailedAnalysis(sign1, sign2 string, element, quality, ruler float64) string {
	info1, _ := ConstellationData[sign1]
	info2, _ := ConstellationData[sign2]

	analysis := fmt.Sprintf("【%s与%s配对分析】\n\n", info1.ChineseName, info2.ChineseName)

	// 元素分析
	analysis += fmt.Sprintf("元素匹配：%s（%s）与%s（%s）\n",
		info1.ChineseName, info1.Element, info2.ChineseName, info2.Element)
	analysis += ca.getElementAnalysisText(info1.Element, info2.Element) + "\n\n"

	// 性质分析
	analysis += fmt.Sprintf("性质匹配：%s（%s）与%s（%s）\n",
		info1.ChineseName, info1.Quality, info2.ChineseName, info2.Quality)
	analysis += ca.getQualityAnalysisText(info1.Quality, info2.Quality) + "\n\n"

	// 守护星分析
	analysis += fmt.Sprintf("守护星影响：%s（%s）与%s（%s）\n",
		info1.ChineseName, info1.Ruler, info2.ChineseName, info2.Ruler)
	analysis += ca.getRulerAnalysisText(info1.Ruler, info2.Ruler) + "\n\n"

	// 性格特征分析
	analysis += "性格特征匹配：\n"
	analysis += ca.getTraitsAnalysis(info1.Traits, info2.Traits)

	return analysis
}

func (ca *CompatibilityAnalyzer) generateAdvantages(sign1, sign2 string, element, quality float64) []string {
	advantages := make([]string, 0)

	info1, _ := ConstellationData[sign1]
	info2, _ := ConstellationData[sign2]

	// 基于元素兼容性的优势
	if element >= 0.7 {
		advantages = append(advantages, ca.getElementAdvantage(info1.Element, info2.Element))
	}

	// 基于性质兼容性的优势
	if quality >= 0.7 {
		advantages = append(advantages, ca.getQualityAdvantage(info1.Quality, info2.Quality))
	}

	// 特定星座组合的优势
	specificAdvantages := ca.getSpecificAdvantages(sign1, sign2)
	advantages = append(advantages, specificAdvantages...)

	if len(advantages) == 0 {
		advantages = append(advantages, "互相学习，共同成长")
	}

	return advantages
}

func (ca *CompatibilityAnalyzer) generateChallenges(sign1, sign2 string, element, quality float64) []string {
	challenges := make([]string, 0)

	info1, _ := ConstellationData[sign1]
	info2, _ := ConstellationData[sign2]

	// 基于元素兼容性的挑战
	if element < 0.4 {
		challenges = append(challenges, ca.getElementChallenge(info1.Element, info2.Element))
	}

	// 基于性质兼容性的挑战
	if quality < 0.4 {
		challenges = append(challenges, ca.getQualityChallenge(info1.Quality, info2.Quality))
	}

	// 特定星座组合的挑战
	specificChallenges := ca.getSpecificChallenges(sign1, sign2)
	challenges = append(challenges, specificChallenges...)

	if len(challenges) == 0 {
		challenges = append(challenges, "需要更多时间相互了解")
	}

	return challenges
}

func (ca *CompatibilityAnalyzer) generateSuggestions(sign1, sign2 string, challenges []string) []string {
	suggestions := make([]string, 0)

	info1, _ := ConstellationData[sign1]
	info2, _ := ConstellationData[sign2]

	// 基于挑战生成建议
	for _, challenge := range challenges {
		suggestion := ca.getSuggestionForChallenge(challenge, info1, info2)
		if suggestion != "" {
			suggestions = append(suggestions, suggestion)
		}
	}

	// 通用建议
	generalSuggestions := []string{
		"保持开放和诚实的沟通",
		"尊重彼此的差异和独特性",
		"寻找共同兴趣和目标",
		"给彼此足够的个人空间",
		"学会欣赏对方的优点",
	}

	// 添加适合的通用建议
	for i, suggestion := range generalSuggestions {
		if i < 3-len(suggestions) { // 确保总建议数不超过5个
			suggestions = append(suggestions, suggestion)
		}
	}

	return suggestions
}

// === 辅助方法 ===

func (ca *CompatibilityAnalyzer) getCompatibilityKey(sign1, sign2 string) string {
	if sign1 < sign2 {
		return sign1 + "_" + sign2
	}
	return sign2 + "_" + sign1
}

func (ca *CompatibilityAnalyzer) getLoveBonus(sign1, sign2 string) float64 {
	// 特定星座组合的爱情加成
	loveBonus := map[string]map[string]float64{
		"aries": {
			"leo":         10.0, // 火火组合，激情四射
			"sagittarius": 8.0,
			"gemini":      5.0,
			"aquarius":    5.0,
		},
		"taurus": {
			"virgo":     12.0, // 土土组合，稳定可靠
			"capricorn": 10.0,
			"cancer":    8.0,
			"pisces":    6.0,
		},
		"gemini": {
			"libra":    10.0, // 风风组合，心灵相通
			"aquarius": 8.0,
			"aries":    5.0,
			"leo":      5.0,
		},
		"cancer": {
			"scorpio": 12.0, // 水水组合，情感深度
			"pisces":  10.0,
			"taurus":  8.0,
			"virgo":   6.0,
		},
	}

	if bonusMap, exists := loveBonus[sign1]; exists {
		if bonus, exists := bonusMap[sign2]; exists {
			return bonus
		}
	}

	// 尝试反向查找
	if bonusMap, exists := loveBonus[sign2]; exists {
		if bonus, exists := bonusMap[sign1]; exists {
			return bonus
		}
	}

	return 0.0
}

func (ca *CompatibilityAnalyzer) getWorkBonus(sign1, sign2 string) float64 {
	// 工作兼容性加成
	workBonus := map[string]map[string]float64{
		"capricorn": {
			"virgo":   15.0, // 两个土象星座，工作默契
			"taurus":  12.0,
			"scorpio": 8.0,
		},
		"leo": {
			"aries":       10.0, // 火象星座领导组合
			"sagittarius": 8.0,
		},
		"virgo": {
			"capricorn": 15.0,
			"taurus":    10.0,
		},
	}

	if bonusMap, exists := workBonus[sign1]; exists {
		if bonus, exists := bonusMap[sign2]; exists {
			return bonus
		}
	}

	if bonusMap, exists := workBonus[sign2]; exists {
		if bonus, exists := bonusMap[sign1]; exists {
			return bonus
		}
	}

	return 0.0
}

// === 分析文本生成 ===

func (ca *CompatibilityAnalyzer) getElementAnalysisText(element1, element2 string) string {
	analysisTexts := map[string]map[string]string{
		"火": {
			"火": "两个火象星座在一起充满激情和能量，但需要注意避免过度竞争。",
			"土": "火与土的组合，一个激情一个稳重，需要相互平衡。",
			"风": "火与风相互助长，创造力和行动力都很强。",
			"水": "火与水形成对比，需要学会相互理解和包容。",
		},
		"土": {
			"火": "土与火的组合，稳重与激情的碰撞，可以相互补充。",
			"土": "两个土象星座都很实际和稳重，关系稳定但可能缺乏激情。",
			"风": "土与风的差异较大，需要更多耐心和理解。",
			"水": "土与水的组合很和谐，都重视安全感和稳定性。",
		},
		"风": {
			"火": "风与火的组合充满活力，思维敏捷且行动力强。",
			"土": "风与土的组合需要平衡理想与现实。",
			"风": "两个风象星座心灵相通，但需要更多实际行动。",
			"水": "风与水的组合富有想象力，但需要更多实际执行力。",
		},
		"水": {
			"火": "水与火的对比强烈，需要学会欣赏彼此的不同。",
			"土": "水与土的组合很和谐，都重视情感和安全感。",
			"风": "水与风都很感性，富有想象力和直觉。",
			"水": "两个水象星座情感深度很强，但可能过于敏感。",
		},
	}

	if elementMap, exists := analysisTexts[element1]; exists {
		if text, exists := elementMap[element2]; exists {
			return text
		}
	}

	return "你们的元素组合有其独特的特点，需要相互理解和适应。"
}

func (ca *CompatibilityAnalyzer) getQualityAnalysisText(quality1, quality2 string) string {
	analysisTexts := map[string]map[string]string{
		"基本": {
			"基本": "两个基本星座都有很强的主导性，需要学会轮流领导。",
			"固定": "基本星座的开创性与固定星座的持久性形成很好的互补。",
			"变动": "基本星座的决断力与变动星座的灵活性结合，适应性很强。",
		},
		"固定": {
			"基本": "固定星座的稳定性为基本星座的计划提供坚实支持。",
			"固定": "两个固定星座都很执着，可能会产生固执的冲突。",
			"变动": "固定星座的坚持与变动星座的灵活形成有趣的对比。",
		},
		"变动": {
			"基本": "变动星座的适应性帮助基本星座实现目标。",
			"固定": "变动星座的灵活性与固定星座的坚持需要平衡。",
			"变动": "两个变动星座都很灵活，但可能缺乏方向性。",
		},
	}

	if qualityMap, exists := analysisTexts[quality1]; exists {
		if text, exists := qualityMap[quality2]; exists {
			return text
		}
	}

	return "你们的性质组合需要相互协调和理解。"
}

func (ca *CompatibilityAnalyzer) getRulerAnalysisText(ruler1, ruler2 string) string {
	// 简化的守护星分析
	if ruler1 == ruler2 {
		return fmt.Sprintf("你们拥有相同的守护星%s，在价值观和行为方式上有很多相似之处。", ruler1)
	}

	rulerAnalysis := map[string]string{
		"火星":  "火星守护的星座通常富有行动力和竞争精神。",
		"金星":  "金星守护的星座重视美感、和谐和人际关系。",
		"水星":  "水星守护的星座善于沟通和思考。",
		"月亮":  "月亮守护的星座情感丰富，重视内心感受。",
		"太阳":  "太阳守护的星座自信阳光，具有领导魅力。",
		"木星":  "木星守护的星座乐观开朗，富有哲学思维。",
		"土星":  "土星守护的星座务实可靠，有强烈的责任感。",
		"天王星": "天王星守护的星座独立创新，思维超前。",
		"海王星": "海王星守护的星座富有想象力和直觉。",
		"冥王星": "冥王星守护的星座具有强烈的洞察力和转化能力。",
	}

	analysis := ""
	if desc1, exists := rulerAnalysis[ruler1]; exists {
		analysis += desc1
	}
	if desc2, exists := rulerAnalysis[ruler2]; exists {
		analysis += " " + desc2
	}

	if analysis == "" {
		analysis = "你们的守护星组合带来独特的能量和特质。"
	}

	return analysis
}

func (ca *CompatibilityAnalyzer) getTraitsAnalysis(traits1, traits2 []string) string {
	// 分析性格特征的匹配度
	commonTraits := make([]string, 0)

	for _, trait1 := range traits1 {
		for _, trait2 := range traits2 {
			if trait1 == trait2 {
				commonTraits = append(commonTraits, trait1)
			}
		}
	}

	analysis := ""
	if len(commonTraits) > 0 {
		analysis += fmt.Sprintf("你们在%v等特质上有共同点，这有助于相互理解。\n", commonTraits)
	}

	analysis += "不同的性格特征可以形成有趣的互补，通过交流和理解可以学到更多。"

	return analysis
}

// === 优势、挑战和建议生成 ===

func (ca *CompatibilityAnalyzer) getElementAdvantage(element1, element2 string) string {
	advantages := map[string]map[string]string{
		"火": {
			"火": "共同的激情和行动力",
			"风": "相互激发创造力和活力",
		},
		"土": {
			"土": "共同的稳定性和可靠性",
			"水": "情感与现实的完美结合",
		},
		"风": {
			"风": "心灵相通，思维敏捷",
			"火": "理想与行动的结合",
		},
		"水": {
			"水": "深度的情感连接",
			"土": "感性与理性的平衡",
		},
	}

	if advantageMap, exists := advantages[element1]; exists {
		if advantage, exists := advantageMap[element2]; exists {
			return advantage
		}
	}

	return "独特的元素组合带来新的可能性"
}

func (ca *CompatibilityAnalyzer) getQualityAdvantage(quality1, quality2 string) string {
	advantages := map[string]map[string]string{
		"基本": {
			"固定": "开创精神与执行力的完美配合",
			"变动": "领导力与适应性的有机结合",
		},
		"固定": {
			"基本": "为新想法提供稳定的支持",
			"变动": "坚持与灵活的平衡",
		},
		"变动": {
			"基本": "为计划提供灵活的执行方案",
			"固定": "在稳定中保持适应性",
		},
	}

	if advantageMap, exists := advantages[quality1]; exists {
		if advantage, exists := advantageMap[quality2]; exists {
			return advantage
		}
	}

	return "不同性质的互补优势"
}

func (ca *CompatibilityAnalyzer) getSpecificAdvantages(sign1, sign2 string) []string {
	// 特定星座组合的优势
	specificAdvantages := map[string]map[string][]string{
		"aries": {
			"leo":         {"共同的领导欲望", "互相激励成长"},
			"sagittarius": {"冒险精神相合", "乐观积极的态度"},
		},
		"taurus": {
			"virgo":     {"共同的实用主义", "对细节的关注"},
			"capricorn": {"共同的目标导向", "稳重可靠的性格"},
		},
		// 可以继续添加更多特定组合
	}

	if advantageMap, exists := specificAdvantages[sign1]; exists {
		if advantages, exists := advantageMap[sign2]; exists {
			return advantages
		}
	}

	// 尝试反向查找
	if advantageMap, exists := specificAdvantages[sign2]; exists {
		if advantages, exists := advantageMap[sign1]; exists {
			return advantages
		}
	}

	return []string{}
}

func (ca *CompatibilityAnalyzer) getElementChallenge(element1, element2 string) string {
	challenges := map[string]map[string]string{
		"火": {
			"水": "激情与敏感的冲突",
			"土": "冲动与谨慎的差异",
		},
		"土": {
			"风": "现实与理想的分歧",
			"火": "保守与冒险的矛盾",
		},
		"风": {
			"水": "理性与感性的平衡困难",
			"土": "变化与稳定的矛盾",
		},
		"水": {
			"火": "情感与行动的不协调",
			"风": "直觉与逻辑的冲突",
		},
	}

	if challengeMap, exists := challenges[element1]; exists {
		if challenge, exists := challengeMap[element2]; exists {
			return challenge
		}
	}

	return "需要适应不同的表达方式"
}

func (ca *CompatibilityAnalyzer) getQualityChallenge(quality1, quality2 string) string {
	challenges := map[string]map[string]string{
		"基本": {
			"基本": "谁来主导的权力争夺",
		},
		"固定": {
			"固定": "双方都不愿妥协的僵局",
		},
		"变动": {
			"变动": "缺乏明确方向的迷茫",
		},
	}

	if challengeMap, exists := challenges[quality1]; exists {
		if challenge, exists := challengeMap[quality2]; exists {
			return challenge
		}
	}

	return "需要协调不同的行为模式"
}

func (ca *CompatibilityAnalyzer) getSpecificChallenges(sign1, sign2 string) []string {
	// 这里可以添加特定星座组合的挑战
	return []string{}
}

func (ca *CompatibilityAnalyzer) getSuggestionForChallenge(challenge string, info1, info2 ConstellationInfo) string {
	// 基于挑战类型生成具体建议
	suggestionMap := map[string]string{
		"激情与敏感的冲突":   "火象星座要学会温柔表达，水象星座要理解对方的直接",
		"冲动与谨慎的差异":   "相互学习，冲动方学会思考，谨慎方学会行动",
		"现实与理想的分歧":   "寻找理想与现实的平衡点，互相包容",
		"谁来主导的权力争夺":  "建立轮流决策机制，学会相互尊重",
		"双方都不愿妥协的僵局": "学会换位思考，寻找双赢的解决方案",
	}

	if suggestion, exists := suggestionMap[challenge]; exists {
		return suggestion
	}

	return "通过开放的沟通来解决分歧"
}

// === 初始化兼容性矩阵 ===

func initElementMatrix() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"火": {
			"火": 0.8, // 同元素高兼容
			"土": 0.4, // 火克土，兼容性低
			"风": 0.9, // 风助火，兼容性极高
			"水": 0.3, // 水克火，兼容性很低
		},
		"土": {
			"火": 0.4,
			"土": 0.8,
			"风": 0.3,
			"水": 0.7, // 土水相生
		},
		"风": {
			"火": 0.9,
			"土": 0.3,
			"风": 0.8,
			"水": 0.5,
		},
		"水": {
			"火": 0.3,
			"土": 0.7,
			"风": 0.5,
			"水": 0.8,
		},
	}
}

func initQualityMatrix() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"基本": {
			"基本": 0.4, // 同性质可能冲突
			"固定": 0.8, // 互补性强
			"变动": 0.7,
		},
		"固定": {
			"基本": 0.8,
			"固定": 0.4,
			"变动": 0.6,
		},
		"变动": {
			"基本": 0.7,
			"固定": 0.6,
			"变动": 0.5,
		},
	}
}

func initRulerMatrix() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"火星": {
			"火星":  0.9,
			"金星":  0.6,
			"水星":  0.5,
			"月亮":  0.4,
			"太阳":  0.7,
			"木星":  0.6,
			"土星":  0.3,
			"天王星": 0.5,
			"海王星": 0.4,
			"冥王星": 0.6,
		},
		"金星": {
			"火星":  0.6,
			"金星":  0.9,
			"水星":  0.7,
			"月亮":  0.8,
			"太阳":  0.6,
			"木星":  0.7,
			"土星":  0.5,
			"天王星": 0.4,
			"海王星": 0.8,
			"冥王星": 0.5,
		},
		// 可以继续添加其他守护星的兼容性矩阵
	}
}

func initStatisticalData() map[string]map[string]float64 {
	// 基于假想的统计数据（实际应用中应使用真实数据）
	return map[string]map[string]float64{
		"aries": {
			"leo":         0.85,
			"sagittarius": 0.78,
			"gemini":      0.65,
			"aquarius":    0.62,
			"taurus":      0.35,
			"cancer":      0.40,
			"virgo":       0.45,
			"scorpio":     0.55,
			"capricorn":   0.42,
			"pisces":      0.38,
			"libra":       0.58,
		},
		"taurus": {
			"virgo":       0.88,
			"capricorn":   0.82,
			"cancer":      0.75,
			"pisces":      0.68,
			"scorpio":     0.62,
			"aries":       0.35,
			"gemini":      0.45,
			"leo":         0.40,
			"libra":       0.55,
			"sagittarius": 0.38,
			"aquarius":    0.42,
		},
		// 可以继续添加其他星座的统计数据
	}
}
