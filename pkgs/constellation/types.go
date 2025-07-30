package constellation

// 日期范围
type DateRange struct {
	Start string `json:"start"` // MM-DD 格式
	End   string `json:"end"`   // MM-DD 格式
}

// 星座基本信息
type ConstellationInfo struct {
	ID          string    `json:"id"`           // 星座ID (aries, taurus, etc.)
	Name        string    `json:"name"`         // 英文名称
	ChineseName string    `json:"chinese_name"` // 中文名称
	Symbol      string    `json:"symbol"`       // 星座符号 ♈♉♊
	Element     string    `json:"element"`      // 元素：火、土、风、水
	Quality     string    `json:"quality"`      // 性质：基本、固定、变动
	Ruler       string    `json:"ruler"`        // 守护星
	DateRange   DateRange `json:"date_range"`   // 日期范围
	Colors      []string  `json:"colors"`       // 幸运色
	Numbers     []int     `json:"numbers"`      // 幸运数字
	Traits      []string  `json:"traits"`       // 性格特征
	Description string    `json:"description"`  // 详细描述
}

// 行星位置信息
type PlanetaryPositions struct {
	Sun     string `json:"sun"`     // 太阳位置
	Moon    string `json:"moon"`    // 月亮位置
	Mercury string `json:"mercury"` // 水星位置
	Venus   string `json:"venus"`   // 金星位置
	Mars    string `json:"mars"`    // 火星位置
	Jupiter string `json:"jupiter"` // 木星位置
	Saturn  string `json:"saturn"`  // 土星位置
	Uranus  string `json:"uranus"`  // 天王星位置
	Neptune string `json:"neptune"` // 海王星位置
	Pluto   string `json:"pluto"`   // 冥王星位置
}

// 宫位信息
type HouseInfo struct {
	Number      int    `json:"number"`      // 宫位号码 1-12
	Sign        string `json:"sign"`        // 所在星座
	Degree      int    `json:"degree"`      // 度数
	Description string `json:"description"` // 宫位含义
}

// 个人星盘
type BirthChart struct {
	ID         string             `json:"id"`
	UserName   string             `json:"user_name"`
	BirthDate  string             `json:"birth_date"`  // 出生日期 YYYY-MM-DD
	BirthTime  string             `json:"birth_time"`  // 出生时间 HH:MM
	BirthPlace string             `json:"birth_place"` // 出生地点
	SunSign    string             `json:"sun_sign"`    // 太阳星座
	MoonSign   string             `json:"moon_sign"`   // 月亮星座
	RisingSign string             `json:"rising_sign"` // 上升星座
	Planetary  PlanetaryPositions `json:"planetary"`   // 行星位置
	Houses     [12]HouseInfo      `json:"houses"`      // 12宫位信息
	CreateTime string             `json:"create_time"`
}

// 每日运势
type DailyHoroscope struct {
	ID            string `json:"id"`
	Constellation string `json:"constellation"` // 星座ID
	Date          string `json:"date"`          // 日期 YYYY-MM-DD
	Overall       int    `json:"overall"`       // 综合运势 1-5星
	Love          int    `json:"love"`          // 爱情运势 1-5星
	Career        int    `json:"career"`        // 事业运势 1-5星
	Money         int    `json:"money"`         // 财运 1-5星
	Health        int    `json:"health"`        // 健康运势 1-5星
	LuckyColor    string `json:"lucky_color"`   // 幸运色
	LuckyNumber   int    `json:"lucky_number"`  // 幸运数字
	Advice        string `json:"advice"`        // 运势建议
	Description   string `json:"description"`   // 详细描述
	CreateTime    string `json:"create_time"`
}

// 占卜结果
type DivinationResult struct {
	Type           string            `json:"type"`           // 占卜类型
	Cards          []TarotCard       `json:"cards"`          // 抽到的牌
	Interpretation string            `json:"interpretation"` // 解读结果
	Advice         string            `json:"advice"`         // 建议
	LuckyNumbers   []int             `json:"lucky_numbers"`  // 幸运数字
	LuckyColors    []string          `json:"lucky_colors"`   // 幸运色
	Probability    float64           `json:"probability"`    // 准确度预测
	Details        map[string]string `json:"details"`        // 详细信息
}

// 占卜记录
type DivinationRecord struct {
	ID         string           `json:"id"`
	UserName   string           `json:"user_name"`
	Type       string           `json:"type"`     // 占卜类型：tarot、astrology、numerology
	Question   string           `json:"question"` // 占卜问题
	Method     string           `json:"method"`   // 占卜方法：single_card、three_card、celtic_cross等
	Result     DivinationResult `json:"result"`   // 占卜结果
	Accuracy   int              `json:"accuracy"` // 用户评价准确度 1-5
	CreateTime string           `json:"create_time"`
}

// 塔罗牌
type TarotCard struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`        // 牌名
	NameCN     string   `json:"name_cn"`     // 中文名
	Suit       string   `json:"suit"`        // 牌组：Major Arcana、Wands、Cups、Swords、Pentacles
	Number     int      `json:"number"`      // 牌号
	Upright    []string `json:"upright"`     // 正位含义
	Reversed   []string `json:"reversed"`    // 逆位含义
	Keywords   []string `json:"keywords"`    // 关键词
	Element    string   `json:"element"`     // 对应元素
	Planet     string   `json:"planet"`      // 对应行星
	IsReversed bool     `json:"is_reversed"` // 是否逆位
}

// 星座配对分析
type CompatibilityAnalysis struct {
	ID           string   `json:"id"`
	Person1      string   `json:"person1"`       // 第一人星座
	Person2      string   `json:"person2"`       // 第二人星座
	OverallScore float64  `json:"overall_score"` // 总体配对指数 0-100
	LoveScore    float64  `json:"love_score"`    // 爱情配对 0-100
	FriendScore  float64  `json:"friend_score"`  // 友情配对 0-100
	WorkScore    float64  `json:"work_score"`    // 工作配对 0-100
	Analysis     string   `json:"analysis"`      // 详细分析
	Advantages   []string `json:"advantages"`    // 配对优势
	Challenges   []string `json:"challenges"`    // 潜在挑战
	Suggestions  []string `json:"suggestions"`   // 相处建议
	CreateTime   string   `json:"create_time"`
}

// 占卜历史统计
type DivinationStats struct {
	UserName       string         `json:"user_name"`
	TotalCount     int            `json:"total_count"`     // 总占卜次数
	TypeStats      map[string]int `json:"type_stats"`      // 各类型占卜次数
	AccuracyAvg    float64        `json:"accuracy_avg"`    // 平均准确度
	MonthlyStats   map[string]int `json:"monthly_stats"`   // 月度统计
	FavoriteType   string         `json:"favorite_type"`   // 最常用占卜类型
	LastDivination string         `json:"last_divination"` // 最后占卜时间
}

// 12星座基础数据
var ConstellationData = map[string]ConstellationInfo{
	"aries": {
		ID:          "aries",
		Name:        "Aries",
		ChineseName: "白羊座",
		Symbol:      "♈",
		Element:     "火",
		Quality:     "基本",
		Ruler:       "火星",
		DateRange:   DateRange{Start: "03-21", End: "04-19"},
		Colors:      []string{"红色", "橙色"},
		Numbers:     []int{6, 7, 9},
		Traits:      []string{"热情", "冲动", "勇敢", "领导力强"},
		Description: "白羊座是十二星座中的第一个星座，象征着新的开始。白羊座的人热情开朗，勇于冒险，具有强烈的领导欲望。",
	},
	"taurus": {
		ID:          "taurus",
		Name:        "Taurus",
		ChineseName: "金牛座",
		Symbol:      "♉",
		Element:     "土",
		Quality:     "固定",
		Ruler:       "金星",
		DateRange:   DateRange{Start: "04-20", End: "05-20"},
		Colors:      []string{"绿色", "粉色"},
		Numbers:     []int{2, 6, 9, 12, 24},
		Traits:      []string{"踏实", "固执", "享受", "保守"},
		Description: "金牛座的人性格稳重，做事踏实，对美食和美物有着天然的鉴赏能力。",
	},
	"gemini": {
		ID:          "gemini",
		Name:        "Gemini",
		ChineseName: "双子座",
		Symbol:      "♊",
		Element:     "风",
		Quality:     "变动",
		Ruler:       "水星",
		DateRange:   DateRange{Start: "05-21", End: "06-21"},
		Colors:      []string{"黄色", "橙色"},
		Numbers:     []int{5, 7, 14, 23},
		Traits:      []string{"聪明", "多变", "好奇", "善于沟通"},
		Description: "双子座的人聪明机智，善于交际，对新鲜事物充满好奇心。",
	},
	"cancer": {
		ID:          "cancer",
		Name:        "Cancer",
		ChineseName: "巨蟹座",
		Symbol:      "♋",
		Element:     "水",
		Quality:     "基本",
		Ruler:       "月亮",
		DateRange:   DateRange{Start: "06-22", End: "07-22"},
		Colors:      []string{"白色", "银色"},
		Numbers:     []int{2, 7, 11, 16, 20, 25},
		Traits:      []string{"敏感", "顾家", "情绪化", "富有同情心"},
		Description: "巨蟹座的人感情丰富，非常重视家庭，具有强烈的保护欲。",
	},
	"leo": {
		ID:          "leo",
		Name:        "Leo",
		ChineseName: "狮子座",
		Symbol:      "♌",
		Element:     "火",
		Quality:     "固定",
		Ruler:       "太阳",
		DateRange:   DateRange{Start: "07-23", End: "08-22"},
		Colors:      []string{"金色", "橙色"},
		Numbers:     []int{1, 3, 10, 19},
		Traits:      []string{"自信", "慷慨", "戏剧性", "领导力"},
		Description: "狮子座的人自信大方，具有天生的领袖气质，喜欢成为关注的焦点。",
	},
	"virgo": {
		ID:          "virgo",
		Name:        "Virgo",
		ChineseName: "处女座",
		Symbol:      "♍",
		Element:     "土",
		Quality:     "变动",
		Ruler:       "水星",
		DateRange:   DateRange{Start: "08-23", End: "09-22"},
		Colors:      []string{"海军蓝", "灰色"},
		Numbers:     []int{3, 27, 35},
		Traits:      []string{"完美主义", "细心", "分析能力强", "服务精神"},
		Description: "处女座的人追求完美，注重细节，具有很强的分析能力和服务精神。",
	},
	"libra": {
		ID:          "libra",
		Name:        "Libra",
		ChineseName: "天秤座",
		Symbol:      "♎",
		Element:     "风",
		Quality:     "基本",
		Ruler:       "金星",
		DateRange:   DateRange{Start: "09-23", End: "10-23"},
		Colors:      []string{"蓝色", "绿色"},
		Numbers:     []int{4, 6, 13, 15, 24},
		Traits:      []string{"平衡", "优雅", "犹豫", "追求和谐"},
		Description: "天秤座的人追求平衡与和谐，具有很好的审美能力和社交技巧。",
	},
	"scorpio": {
		ID:          "scorpio",
		Name:        "Scorpio",
		ChineseName: "天蝎座",
		Symbol:      "♏",
		Element:     "水",
		Quality:     "固定",
		Ruler:       "冥王星",
		DateRange:   DateRange{Start: "10-24", End: "11-22"},
		Colors:      []string{"深红色", "黑色"},
		Numbers:     []int{4, 13, 21},
		Traits:      []string{"神秘", "专注", "复仇心强", "洞察力"},
		Description: "天蝎座的人神秘深邃，具有强烈的直觉和洞察力，对感兴趣的事物非常专注。",
	},
	"sagittarius": {
		ID:          "sagittarius",
		Name:        "Sagittarius",
		ChineseName: "射手座",
		Symbol:      "♐",
		Element:     "火",
		Quality:     "变动",
		Ruler:       "木星",
		DateRange:   DateRange{Start: "11-23", End: "12-21"},
		Colors:      []string{"紫色", "红色"},
		Numbers:     []int{3, 9, 15, 21, 33},
		Traits:      []string{"自由", "乐观", "哲学", "爱冒险"},
		Description: "射手座的人热爱自由，乐观向上，对哲学和远方有着天然的向往。",
	},
	"capricorn": {
		ID:          "capricorn",
		Name:        "Capricorn",
		ChineseName: "摩羯座",
		Symbol:      "♑",
		Element:     "土",
		Quality:     "基本",
		Ruler:       "土星",
		DateRange:   DateRange{Start: "12-22", End: "01-19"},
		Colors:      []string{"棕色", "黑色"},
		Numbers:     []int{8, 10, 26, 35},
		Traits:      []string{"踏实", "有责任心", "保守", "野心勃勃"},
		Description: "摩羯座的人踏实可靠，具有强烈的责任心和事业心，是天生的管理者。",
	},
	"aquarius": {
		ID:          "aquarius",
		Name:        "Aquarius",
		ChineseName: "水瓶座",
		Symbol:      "♒",
		Element:     "风",
		Quality:     "固定",
		Ruler:       "天王星",
		DateRange:   DateRange{Start: "01-20", End: "02-18"},
		Colors:      []string{"蓝色", "银色"},
		Numbers:     []int{4, 7, 11, 22, 29},
		Traits:      []string{"独立", "创新", "人道主义", "固执"},
		Description: "水瓶座的人独立自主，具有创新精神和人道主义关怀，思维超前。",
	},
	"pisces": {
		ID:          "pisces",
		Name:        "Pisces",
		ChineseName: "双鱼座",
		Symbol:      "♓",
		Element:     "水",
		Quality:     "变动",
		Ruler:       "海王星",
		DateRange:   DateRange{Start: "02-19", End: "03-20"},
		Colors:      []string{"海蓝色", "紫色"},
		Numbers:     []int{3, 9, 12, 15, 18, 24},
		Traits:      []string{"浪漫", "直觉强", "同情心", "逃避现实"},
		Description: "双鱼座的人富有想象力和同情心，直觉敏锐，但有时容易逃避现实。",
	},
}

// 版本信息
func Info() string {
	return "constellation v1.0.0 - 星座占卜运势模块"
}
