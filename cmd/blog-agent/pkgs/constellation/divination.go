package constellation

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// DivinationEngine 占卜算法引擎
type DivinationEngine struct {
	tarotDeck    []TarotCard
	randomSource *rand.Rand
}

// NewDivinationEngine 创建占卜引擎
func NewDivinationEngine() *DivinationEngine {
	return &DivinationEngine{
		tarotDeck:    initTarotDeck(),
		randomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// PerformDivination 执行占卜
func (de *DivinationEngine) PerformDivination(divinationType, method, question string) (*DivinationResult, error) {
	switch divinationType {
	case "tarot":
		return de.performTarotDivination(method, question)
	case "astrology":
		return de.performAstrologyDivination(method, question)
	case "numerology":
		return de.performNumerologyDivination(method, question)
	default:
		return nil, fmt.Errorf("不支持的占卜类型: %s", divinationType)
	}
}

// 塔罗占卜
func (de *DivinationEngine) performTarotDivination(method, question string) (*DivinationResult, error) {
	var cards []TarotCard
	var interpretation string

	switch method {
	case "single_card":
		cards = de.drawCards(1)
		interpretation = de.interpretSingleCard(cards[0], question)

	case "three_card":
		cards = de.drawCards(3)
		interpretation = de.interpretThreeCard(cards, question)

	case "celtic_cross":
		cards = de.drawCards(10)
		interpretation = de.interpretCelticCross(cards, question)

	default:
		return nil, fmt.Errorf("不支持的塔罗占卜方法: %s", method)
	}

	return &DivinationResult{
		Type:           "tarot",
		Cards:          cards,
		Interpretation: interpretation,
		Advice:         de.generateTarotAdvice(cards),
		LuckyNumbers:   de.generateLuckyNumbers(cards),
		LuckyColors:    de.generateLuckyColors(cards),
		Probability:    de.calculateProbability(cards),
		Details:        de.generateTarotDetails(cards, method),
	}, nil
}

// 星座占卜
func (de *DivinationEngine) performAstrologyDivination(method, question string) (*DivinationResult, error) {
	// 基于当前星象的占卜
	currentDate := time.Now()

	interpretation := de.generateAstrologyInterpretation(currentDate, question)
	advice := de.generateAstrologyAdvice(currentDate)

	return &DivinationResult{
		Type:           "astrology",
		Cards:          []TarotCard{}, // 星座占卜不使用卡牌
		Interpretation: interpretation,
		Advice:         advice,
		LuckyNumbers:   de.generateAstrologyLuckyNumbers(currentDate),
		LuckyColors:    de.generateAstrologyLuckyColors(currentDate),
		Probability:    0.75 + de.randomSource.Float64()*0.2, // 75-95%
		Details:        de.generateAstrologyDetails(currentDate, method),
	}, nil
}

// 数字占卜
func (de *DivinationEngine) performNumerologyDivination(method, question string) (*DivinationResult, error) {
	// 基于问题和时间生成数字
	numbers := de.generateMeaningfulNumbers(question)

	interpretation := de.generateNumerologyInterpretation(numbers, question)
	advice := de.generateNumerologyAdvice(numbers)

	return &DivinationResult{
		Type:           "numerology",
		Cards:          []TarotCard{}, // 数字占卜不使用卡牌
		Interpretation: interpretation,
		Advice:         advice,
		LuckyNumbers:   numbers,
		LuckyColors:    de.generateNumerologyColors(numbers),
		Probability:    0.70 + de.randomSource.Float64()*0.25, // 70-95%
		Details:        de.generateNumerologyDetails(numbers, method),
	}, nil
}

// === 塔罗牌相关方法 ===

// 抽卡
func (de *DivinationEngine) drawCards(count int) []TarotCard {
	// 洗牌
	deck := make([]TarotCard, len(de.tarotDeck))
	copy(deck, de.tarotDeck)

	for i := len(deck) - 1; i > 0; i-- {
		j := de.randomSource.Intn(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}

	// 抽取指定数量的卡牌
	cards := make([]TarotCard, count)
	for i := 0; i < count; i++ {
		cards[i] = deck[i]
		// 随机决定是否逆位
		cards[i].IsReversed = de.randomSource.Float64() < 0.3 // 30%概率逆位
	}

	return cards
}

// 单卡解读
func (de *DivinationEngine) interpretSingleCard(card TarotCard, question string) string {
	var meaning []string

	if card.IsReversed {
		meaning = card.Reversed
	} else {
		meaning = card.Upright
	}

	interpretation := fmt.Sprintf("你抽到的是：%s (%s)\n\n", card.NameCN, card.Name)

	if card.IsReversed {
		interpretation += "【逆位】\n"
	} else {
		interpretation += "【正位】\n"
	}

	interpretation += fmt.Sprintf("牌意：%s\n\n", strings.Join(meaning, "、"))

	interpretation += fmt.Sprintf("针对你的问题「%s」，这张牌建议你：\n", question)
	interpretation += de.generateCardAdviceForQuestion(card, question, card.IsReversed)

	return interpretation
}

// 三卡解读（过去-现在-未来）
func (de *DivinationEngine) interpretThreeCard(cards []TarotCard, question string) string {
	positions := []string{"过去", "现在", "未来"}
	interpretation := fmt.Sprintf("针对你的问题「%s」，三卡牌阵显示：\n\n", question)

	for i, card := range cards {
		interpretation += fmt.Sprintf("【%s】%s (%s) %s\n",
			positions[i],
			card.NameCN,
			card.Name,
			func() string {
				if card.IsReversed {
					return "逆位"
				} else {
					return "正位"
				}
			}())

		var meaning []string
		if card.IsReversed {
			meaning = card.Reversed
		} else {
			meaning = card.Upright
		}

		interpretation += fmt.Sprintf("含义：%s\n\n", strings.Join(meaning, "、"))
	}

	// 综合解读
	interpretation += "【综合解读】\n"
	interpretation += de.generateThreeCardSynthesis(cards, question)

	return interpretation
}

// 凯尔特十字牌阵解读
func (de *DivinationEngine) interpretCelticCross(cards []TarotCard, question string) string {
	positions := []string{
		"现状", "障碍/机会", "远程目标", "近期过去",
		"可能结果", "近期未来", "你的方法", "外在影响",
		"内在感受", "最终结果",
	}

	interpretation := fmt.Sprintf("针对你的问题「%s」，凯尔特十字牌阵显示：\n\n", question)

	for i, card := range cards {
		interpretation += fmt.Sprintf("【%s】%s (%s) %s\n",
			positions[i],
			card.NameCN,
			card.Name,
			func() string {
				if card.IsReversed {
					return "逆位"
				} else {
					return "正位"
				}
			}())

		var meaning []string
		if card.IsReversed {
			meaning = card.Reversed
		} else {
			meaning = card.Upright
		}

		interpretation += fmt.Sprintf("%s\n\n", strings.Join(meaning, "、"))
	}

	// 综合解读
	interpretation += "【综合指导】\n"
	interpretation += de.generateCelticCrossSynthesis(cards, question)

	return interpretation
}

// === 星座占卜相关方法 ===

func (de *DivinationEngine) generateAstrologyInterpretation(date time.Time, question string) string {
	// 获取当前月份对应的星座能量
	month := date.Month()
	var dominantSign string

	switch month {
	case 3:
		dominantSign = "白羊座"
	case 4:
		dominantSign = "金牛座"
	case 5:
		dominantSign = "双子座"
	case 6:
		dominantSign = "巨蟹座"
	case 7:
		dominantSign = "狮子座"
	case 8:
		dominantSign = "处女座"
	case 9:
		dominantSign = "天秤座"
	case 10:
		dominantSign = "天蝎座"
	case 11:
		dominantSign = "射手座"
	case 12:
		dominantSign = "摩羯座"
	case 1:
		dominantSign = "水瓶座"
	case 2:
		dominantSign = "双鱼座"
	}

	interpretation := fmt.Sprintf("当前星象分析（%s影响期）：\n\n", dominantSign)
	interpretation += fmt.Sprintf("针对你的问题「%s」：\n\n", question)

	// 基于星座特性生成解读
	signInfo := de.getSignEnergyDescription(dominantSign)
	interpretation += signInfo + "\n\n"

	// 行星影响
	interpretation += "当前主要行星影响：\n"
	interpretation += de.generatePlanetaryInfluence(date)

	return interpretation
}

// === 数字占卜相关方法 ===

func (de *DivinationEngine) generateMeaningfulNumbers(question string) []int {
	// 基于问题文本生成有意义的数字
	questionBytes := []byte(question)
	numbers := make([]int, 0, 7)

	// 计算文本数字特征
	sum := 0
	for _, b := range questionBytes {
		sum += int(b)
	}

	// 生成7个幸运数字
	for i := 0; i < 7; i++ {
		num := (sum + i*7 + int(time.Now().Unix())) % 100
		if num == 0 {
			num = 1
		}
		numbers = append(numbers, num)
	}

	return numbers
}

func (de *DivinationEngine) generateNumerologyInterpretation(numbers []int, question string) string {
	interpretation := fmt.Sprintf("数字占卜解读：\n\n")
	interpretation += fmt.Sprintf("针对你的问题「%s」，数字显示：\n\n", question)

	// 主要数字分析
	mainNumber := numbers[0] % 9
	if mainNumber == 0 {
		mainNumber = 9
	}

	numberMeanings := map[int]string{
		1: "新的开始、领导力、独立",
		2: "合作、平衡、关系",
		3: "创造力、表达、乐观",
		4: "稳定、务实、努力",
		5: "变化、自由、冒险",
		6: "关爱、责任、家庭",
		7: "精神、智慧、内省",
		8: "成功、权力、物质",
		9: "完成、智慧、普世关爱",
	}

	interpretation += fmt.Sprintf("主导数字：%d\n", mainNumber)
	interpretation += fmt.Sprintf("含义：%s\n\n", numberMeanings[mainNumber])

	// 数字组合分析
	interpretation += "数字组合显示：\n"
	interpretation += de.generateNumberCombinationAnalysis(numbers)

	return interpretation
}

// === 辅助方法 ===

func (de *DivinationEngine) generateCardAdviceForQuestion(card TarotCard, question string, isReversed bool) string {
	adviceTemplates := []string{
		"现在是时候%s，专注于%s的力量。",
		"建议你保持%s的态度，同时注意%s的影响。",
		"这张牌提醒你要%s，特别是在%s方面。",
	}

	template := adviceTemplates[de.randomSource.Intn(len(adviceTemplates))]

	var action, focus string
	if isReversed {
		action = "谨慎行事"
		focus = "内在调整"
	} else {
		action = "积极行动"
		focus = "外在表现"
	}

	return fmt.Sprintf(template, action, focus)
}

func (de *DivinationEngine) generateTarotAdvice(cards []TarotCard) string {
	adviceList := []string{
		"保持开放的心态面对变化",
		"相信自己的直觉和内在智慧",
		"专注于当下，不要过分担忧未来",
		"与他人保持真诚的沟通",
		"寻找生活中的平衡点",
	}

	return adviceList[de.randomSource.Intn(len(adviceList))]
}

func (de *DivinationEngine) generateLuckyNumbers(cards []TarotCard) []int {
	numbers := make([]int, 0, 7)

	for _, card := range cards {
		if card.Number > 0 {
			numbers = append(numbers, card.Number)
		}
	}

	// 补充随机数字到7个
	for len(numbers) < 7 {
		num := de.randomSource.Intn(50) + 1
		numbers = append(numbers, num)
	}

	return numbers[:7]
}

func (de *DivinationEngine) generateLuckyColors(cards []TarotCard) []string {
	colorsByElement := map[string][]string{
		"fire":  {"红色", "橙色", "金色"},
		"water": {"蓝色", "海蓝色", "银色"},
		"air":   {"黄色", "白色", "淡蓝色"},
		"earth": {"绿色", "棕色", "黑色"},
	}

	colors := make([]string, 0)

	for _, card := range cards {
		if elementColors, exists := colorsByElement[card.Element]; exists {
			colors = append(colors, elementColors[de.randomSource.Intn(len(elementColors))])
		}
	}

	if len(colors) == 0 {
		colors = append(colors, "紫色", "白色")
	}

	return colors
}

func (de *DivinationEngine) calculateProbability(cards []TarotCard) float64 {
	// 基于卡牌组合计算准确度
	base := 0.7

	for _, card := range cards {
		if card.Suit == "Major Arcana" {
			base += 0.05 // 大阿卡纳增加准确度
		}
		if !card.IsReversed {
			base += 0.02 // 正位增加准确度
		}
	}

	if base > 0.95 {
		base = 0.95
	}

	return base
}

func (de *DivinationEngine) generateTarotDetails(cards []TarotCard, method string) map[string]string {
	details := make(map[string]string)

	details["牌阵类型"] = method
	details["抽卡数量"] = fmt.Sprintf("%d张", len(cards))
	details["逆位数量"] = fmt.Sprintf("%d张", de.countReversedCards(cards))
	details["大阿卡纳"] = fmt.Sprintf("%d张", de.countMajorArcana(cards))

	return details
}

func (de *DivinationEngine) countReversedCards(cards []TarotCard) int {
	count := 0
	for _, card := range cards {
		if card.IsReversed {
			count++
		}
	}
	return count
}

func (de *DivinationEngine) countMajorArcana(cards []TarotCard) int {
	count := 0
	for _, card := range cards {
		if card.Suit == "Major Arcana" {
			count++
		}
	}
	return count
}

// 更多辅助方法的实现...
func (de *DivinationEngine) generateThreeCardSynthesis(cards []TarotCard, question string) string {
	return "三张卡牌组合显示了一个完整的时间线。过去的经历为现在提供了基础，而当前的选择将影响未来的发展。建议综合考虑三个时期的信息，做出明智的决定。"
}

func (de *DivinationEngine) generateCelticCrossSynthesis(cards []TarotCard, question string) string {
	return "凯尔特十字牌阵展现了问题的全貌。内在与外在因素相互作用，过去的经历与未来的可能性交织在一起。关键在于找到内心的平衡，同时积极应对外在环境的变化。"
}

func (de *DivinationEngine) getSignEnergyDescription(sign string) string {
	descriptions := map[string]string{
		"白羊座": "当前充满行动力和开创精神的能量。适合开始新项目，但要注意耐心。",
		"金牛座": "稳定和务实的能量主导。专注于具体目标，享受简单的美好。",
		"双子座": "沟通和学习的能量旺盛。多元化思考，保持好奇心。",
		"巨蟹座": "情感和直觉的能量突出。关注内心声音，重视人际关系。",
		"狮子座": "创造和表现的能量强劲。展现自信，追求认可和赞赏。",
		"处女座": "分析和完善的能量显著。注重细节，追求完美和效率。",
		"天秤座": "和谐和平衡的能量盛行。寻求公正，重视美感和关系。",
		"天蝎座": "转化和深度的能量强烈。探索隐藏真相，面对内在阴影。",
		"射手座": "探索和扩展的能量活跃。追求真理，渴望新体验。",
		"摩羯座": "目标和成就的能量稳定。制定长期计划，承担责任。",
		"水瓶座": "创新和独立的能量突出。打破常规，追求独特性。",
		"双鱼座": "直觉和精神的能量丰富。连接内在智慧，发挥想象力。",
	}

	return descriptions[sign]
}

func (de *DivinationEngine) generatePlanetaryInfluence(date time.Time) string {
	influences := []string{
		"水星影响：沟通和思维活跃，适合学习和交流",
		"金星影响：爱情和美感增强，关注人际关系",
		"火星影响：行动力和竞争意识提升，积极进取",
		"木星影响：扩展和机遇增加，保持乐观态度",
		"土星影响：责任和纪律性增强，脚踏实地",
	}

	return influences[date.Day()%len(influences)]
}

func (de *DivinationEngine) generateAstrologyAdvice(date time.Time) string {
	advice := []string{
		"顺应当前星象能量，把握时机",
		"平衡理性思考与直觉感受",
		"关注内在成长与外在表现的协调",
		"保持开放心态，迎接变化",
		"专注于长期目标，稳步前进",
	}

	return advice[date.Hour()%len(advice)]
}

func (de *DivinationEngine) generateAstrologyLuckyNumbers(date time.Time) []int {
	base := date.Day()
	numbers := make([]int, 7)

	for i := 0; i < 7; i++ {
		numbers[i] = (base*7+i*3)%50 + 1
	}

	return numbers
}

func (de *DivinationEngine) generateAstrologyLuckyColors(date time.Time) []string {
	colorSets := [][]string{
		{"金色", "橙色"},
		{"蓝色", "银色"},
		{"绿色", "棕色"},
		{"红色", "紫色"},
		{"白色", "灰色"},
	}

	return colorSets[date.Weekday()%time.Weekday(len(colorSets))]
}

func (de *DivinationEngine) generateAstrologyDetails(date time.Time, method string) map[string]string {
	details := make(map[string]string)

	details["占卜日期"] = date.Format("2006-01-02")
	details["星象类型"] = method
	details["主导行星"] = de.getDominantPlanet(date)
	details["月相影响"] = de.getMoonPhase(date)

	return details
}

func (de *DivinationEngine) getDominantPlanet(date time.Time) string {
	planets := []string{"太阳", "月亮", "水星", "金星", "火星", "木星", "土星"}
	return planets[date.Day()%len(planets)]
}

func (de *DivinationEngine) getMoonPhase(date time.Time) string {
	phases := []string{"新月", "上弦月", "满月", "下弦月"}
	return phases[date.Day()%len(phases)]
}

func (de *DivinationEngine) generateNumerologyAdvice(numbers []int) string {
	mainNumber := numbers[0] % 9
	if mainNumber == 0 {
		mainNumber = 9
	}

	advice := map[int]string{
		1: "相信自己的能力，勇敢迈出第一步",
		2: "寻求合作与平衡，倾听他人意见",
		3: "发挥创造力，用积极态度面对挑战",
		4: "脚踏实地，通过努力实现目标",
		5: "拥抱变化，保持灵活性和适应性",
		6: "承担责任，关爱身边的人",
		7: "静心思考，寻求内在的智慧",
		8: "专注于实际成果，发挥领导能力",
		9: "放眼大局，以服务他人的心态行事",
	}

	return advice[mainNumber]
}

func (de *DivinationEngine) generateNumerologyColors(numbers []int) []string {
	colorMap := map[int]string{
		1: "红色", 2: "橙色", 3: "黄色", 4: "绿色", 5: "蓝色",
		6: "靛蓝", 7: "紫色", 8: "金色", 9: "白色", 0: "银色",
	}

	colors := make([]string, 0, 3)
	for _, num := range numbers[:3] {
		if color, exists := colorMap[num%10]; exists {
			colors = append(colors, color)
		}
	}

	return colors
}

func (de *DivinationEngine) generateNumberCombinationAnalysis(numbers []int) string {
	sum := 0
	for _, num := range numbers {
		sum += num
	}

	reduced := sum
	for reduced >= 10 {
		newSum := 0
		for reduced > 0 {
			newSum += reduced % 10
			reduced /= 10
		}
		reduced = newSum
	}

	analysis := fmt.Sprintf("数字和为 %d，最终简化为 %d。", sum, reduced)

	meanings := map[int]string{
		1: "这代表新的开始和领导力",
		2: "这象征着合作和平衡",
		3: "这表示创造和表达的能量",
		4: "这显示稳定和务实的特质",
		5: "这暗示变化和自由的需求",
		6: "这反映关爱和责任感",
		7: "这指向精神成长和内在智慧",
		8: "这展现成功和物质成就的潜力",
		9: "这体现完成和普世关爱的境界",
	}

	if meaning, exists := meanings[reduced]; exists {
		analysis += meaning + "。"
	}

	return analysis
}

func (de *DivinationEngine) generateNumerologyDetails(numbers []int, method string) map[string]string {
	details := make(map[string]string)

	details["占卜方法"] = method
	details["主要数字"] = fmt.Sprintf("%v", numbers[:3])
	details["数字总和"] = fmt.Sprintf("%d", de.sumNumbers(numbers))
	details["生命数字"] = fmt.Sprintf("%d", de.calculateLifeNumber(numbers))

	return details
}

func (de *DivinationEngine) sumNumbers(numbers []int) int {
	sum := 0
	for _, num := range numbers {
		sum += num
	}
	return sum
}

func (de *DivinationEngine) calculateLifeNumber(numbers []int) int {
	sum := de.sumNumbers(numbers)
	for sum >= 10 {
		newSum := 0
		for sum > 0 {
			newSum += sum % 10
			sum /= 10
		}
		sum = newSum
	}
	return sum
}

// 初始化塔罗牌组
func initTarotDeck() []TarotCard {
	return []TarotCard{
		// 大阿卡纳 (Major Arcana)
		{0, "The Fool", "愚者", "Major Arcana", 0, []string{"新开始", "冒险", "潜力"}, []string{"鲁莽", "风险", "愚蠢"}, []string{"开始", "冒险", "自由"}, "air", "天王星", false},
		{1, "The Magician", "魔术师", "Major Arcana", 1, []string{"意志力", "技能", "专注"}, []string{"操控", "缺乏能力", "延迟"}, []string{"意志", "技能", "力量"}, "air", "水星", false},
		{2, "The High Priestess", "女祭司", "Major Arcana", 2, []string{"直觉", "神秘", "智慧"}, []string{"隐藏秘密", "缺乏中心", "认知偏差"}, []string{"直觉", "智慧", "神秘"}, "water", "月亮", false},
		{3, "The Empress", "皇后", "Major Arcana", 3, []string{"女性力量", "创造", "自然"}, []string{"依赖", "空虚", "不育"}, []string{"母性", "创造", "自然"}, "earth", "金星", false},
		{4, "The Emperor", "皇帝", "Major Arcana", 4, []string{"权威", "结构", "控制"}, []string{"专制", "缺乏纪律", "不灵活"}, []string{"权威", "父性", "结构"}, "fire", "白羊座", false},

		// 权杖牌组 (Wands) - 火元素
		{5, "Ace of Wands", "权杖一", "Wands", 1, []string{"创造力", "灵感", "新项目"}, []string{"缺乏方向", "创意枯竭", "延迟"}, []string{"创造", "灵感", "开始"}, "fire", "火星", false},
		{6, "Two of Wands", "权杖二", "Wands", 2, []string{"计划", "决策", "个人力量"}, []string{"缺乏规划", "害怕未知", "缺乏控制"}, []string{"计划", "控制", "决策"}, "fire", "火星", false},
		{7, "Three of Wands", "权杖三", "Wands", 3, []string{"扩展", "远见", "领导力"}, []string{"缺乏远见", "意外障碍", "缺乏规划"}, []string{"扩展", "远见", "进展"}, "fire", "太阳", false},

		// 圣杯牌组 (Cups) - 水元素
		{8, "Ace of Cups", "圣杯一", "Cups", 1, []string{"新的爱情", "直觉", "精神成长"}, []string{"情感封闭", "压抑感情", "精神枯竭"}, []string{"爱情", "情感", "直觉"}, "water", "海王星", false},
		{9, "Two of Cups", "圣杯二", "Cups", 2, []string{"伙伴关系", "爱情", "和谐"}, []string{"关系不平衡", "缺乏信任", "误解"}, []string{"伙伴", "爱情", "和谐"}, "water", "金星", false},

		// 宝剑牌组 (Swords) - 风元素
		{10, "Ace of Swords", "宝剑一", "Swords", 1, []string{"新想法", "清晰思维", "突破"}, []string{"思维混乱", "缺乏清晰", "错误信息"}, []string{"思维", "真相", "清晰"}, "air", "水星", false},
		{11, "Two of Swords", "宝剑二", "Swords", 2, []string{"艰难决择", "僵局", "需要信息"}, []string{"犹豫不决", "过度分析", "缺乏信息"}, []string{"选择", "平衡", "和平"}, "air", "月亮", false},

		// 金币牌组 (Pentacles) - 土元素
		{12, "Ace of Pentacles", "金币一", "Pentacles", 1, []string{"新机会", "物质显化", "财富"}, []string{"错失机会", "缺乏规划", "贪婪"}, []string{"机会", "财富", "显化"}, "earth", "土元素", false},
		{13, "Two of Pentacles", "金币二", "Pentacles", 2, []string{"平衡", "适应性", "时间管理"}, []string{"失去平衡", "压力过大", "无法应对"}, []string{"平衡", "灵活", "管理"}, "earth", "木星", false},
	}
}
