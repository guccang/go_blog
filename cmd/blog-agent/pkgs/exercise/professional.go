package exercise

import (
	"blog"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"module"
	"sort"
	"strings"
	"time"
)

const (
	professionalProfileTitle = "exercise-professional-profile"
	professionalSource       = "professional"
	professionalCatalogVer   = 1
)

var ErrProfessionalPlanConflict = errors.New("professional plan already exists in this date range")

type ProfessionalLevel struct {
	Level        int      `json:"level"`
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Sets         int      `json:"sets"`
	Reps         int      `json:"reps,omitempty"`
	HoldSeconds  int      `json:"hold_seconds,omitempty"`
	RestSeconds  int      `json:"rest_seconds"`
	Tempo        string   `json:"tempo"`
	Duration     int      `json:"duration"`
	Cues         []string `json:"cues"`
	Mistakes     []string `json:"mistakes"`
	Advance      string   `json:"advance"`
	Equipment    string   `json:"equipment"`
	Target       string   `json:"target"`
	Difficulty   string   `json:"difficulty"`
	MovementID   string   `json:"movement_id"`
	MovementName string   `json:"movement_name"`
}

type ProfessionalMovement struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Icon      string              `json:"icon"`
	Target    string              `json:"target"`
	Equipment string              `json:"equipment"`
	Summary   string              `json:"summary"`
	Levels    []ProfessionalLevel `json:"levels"`
}

type ProfessionalCatalog struct {
	Version   int                    `json:"version"`
	Notice    string                 `json:"notice"`
	Movements []ProfessionalMovement `json:"movements"`
}

type ProfessionalProfile struct {
	Version     int            `json:"version"`
	Levels      map[string]int `json:"levels"`
	DaysPerWeek int            `json:"days_per_week"`
	StartDate   string         `json:"start_date"`
	UpdatedAt   string         `json:"updated_at"`
}

type ProfessionalPlanRequest struct {
	StartDate   string         `json:"start_date"`
	DaysPerWeek int            `json:"days_per_week"`
	Levels      map[string]int `json:"levels"`
	Replace     bool           `json:"replace"`
}

type ProfessionalPlanItem struct {
	MovementID       string `json:"movement_id"`
	MovementName     string `json:"movement_name"`
	ProgressionLevel int    `json:"progression_level"`
	Name             string `json:"name"`
	Sets             int    `json:"sets"`
	Reps             int    `json:"reps,omitempty"`
	HoldSeconds      int    `json:"hold_seconds,omitempty"`
	RestSeconds      int    `json:"rest_seconds"`
	Tempo            string `json:"tempo"`
	Duration         int    `json:"duration"`
	Target           string `json:"target"`
}

type ProfessionalPlanSession struct {
	Date  string                 `json:"date"`
	Day   int                    `json:"day"`
	Items []ProfessionalPlanItem `json:"items"`
}

type ProfessionalPlan struct {
	ID          string                    `json:"id"`
	StartDate   string                    `json:"start_date"`
	EndDate     string                    `json:"end_date"`
	DaysPerWeek int                       `json:"days_per_week"`
	Sessions    []ProfessionalPlanSession `json:"sessions"`
}

type ProfessionalApplyResult struct {
	PlanID    string `json:"plan_id"`
	Created   int    `json:"created"`
	Skipped   int    `json:"skipped"`
	Preserved int    `json:"preserved"`
	Replaced  bool   `json:"replaced"`
}

type levelSpec struct {
	name        string
	sets        int
	reps        int
	holdSeconds int
	restSeconds int
	tempo       string
	duration    int
	equipment   string
	advance     string
}

type movementSpec struct {
	id        string
	name      string
	icon      string
	target    string
	equipment string
	summary   string
	cues      []string
	mistakes  []string
	levels    []levelSpec
}

func ProfessionalCatalogData() ProfessionalCatalog {
	specs := []movementSpec{
		{
			id: "pushup", name: "俯卧撑", icon: "↘", target: "胸、肩、肱三头肌", equipment: "墙面或稳固支撑物",
			summary:  "从垂直推墙逐步降低支撑高度，最终建立单侧推力与躯干稳定。",
			cues:     []string{"头、躯干与髋保持一条直线", "肩胛自然移动，肘部约向后四十五度"},
			mistakes: []string{"塌腰或抬臀借力", "只移动头部，没有让胸口接近支撑面"},
			levels: []levelSpec{
				{"墙壁俯卧撑", 2, 15, 0, 45, "3-1-2", 6, "墙面", "两组各完成 25 次，动作全程稳定"},
				{"高位俯卧撑", 2, 12, 0, 60, "3-1-2", 7, "胸口高度支撑物", "两组各完成 20 次"},
				{"低位俯卧撑", 2, 10, 0, 60, "3-1-2", 8, "腰部高度支撑物", "两组各完成 18 次"},
				{"跪姿俯卧撑", 2, 10, 0, 60, "3-1-2", 8, "软垫", "两组各完成 15 次"},
				{"半程俯卧撑", 2, 8, 0, 75, "3-1-2", 9, "瑜伽砖或软垫", "两组各完成 15 次并保持相同深度"},
				{"标准俯卧撑", 3, 8, 0, 90, "3-1-2", 12, "地面", "三组各完成 15 次"},
				{"窄距俯卧撑", 3, 6, 0, 90, "3-1-2", 12, "地面", "三组各完成 12 次"},
				{"偏重俯卧撑", 3, 5, 0, 105, "3-1-2", 14, "篮球或瑜伽砖", "每侧三组各完成 10 次"},
				{"弓箭俯卧撑", 3, 4, 0, 120, "4-1-2", 15, "地面", "每侧三组各完成 8 次"},
				{"辅助单臂俯卧撑", 3, 3, 0, 150, "4-1-2", 16, "高位辅助支撑", "每侧三组各完成 6 次，无躯干旋转"},
			},
		},
		{
			id: "squat", name: "深蹲", icon: "↓", target: "股四头肌、臀肌、腿后侧", equipment: "门框、长凳或台阶",
			summary:  "以可控深度和膝髋协同为核心，逐步过渡到单腿力量。",
			cues:     []string{"脚掌三点均匀受力，膝盖跟随脚尖方向", "先屈髋再屈膝，下降与起身同样可控"},
			mistakes: []string{"脚跟抬起或膝盖向内塌", "为了追求深度而失去腰背中立"},
			levels: []levelSpec{
				{"坐站练习", 2, 12, 0, 45, "3-1-2", 7, "稳固椅子", "两组各完成 20 次，不用手支撑"},
				{"辅助半蹲", 2, 12, 0, 45, "3-1-2", 7, "门框", "两组各完成 20 次"},
				{"自重半蹲", 2, 10, 0, 60, "3-1-2", 8, "无", "两组各完成 20 次且膝盖稳定"},
				{"箱式深蹲", 2, 10, 0, 60, "3-1-2", 9, "低凳", "两组各完成 18 次，轻触凳面即起"},
				{"标准深蹲", 3, 10, 0, 75, "3-1-2", 12, "无", "三组各完成 20 次"},
				{"窄距深蹲", 3, 8, 0, 90, "3-1-2", 12, "无", "三组各完成 15 次"},
				{"分腿蹲", 3, 8, 0, 90, "3-1-2", 14, "可选扶手", "每侧三组各完成 12 次"},
				{"保加利亚分腿蹲", 3, 6, 0, 105, "3-1-2", 15, "长凳", "每侧三组各完成 10 次"},
				{"箱式单腿蹲", 3, 5, 0, 120, "4-1-2", 16, "长凳或箱子", "每侧三组各完成 8 次"},
				{"辅助手枪深蹲", 3, 3, 0, 150, "4-1-2", 18, "门框或吊带", "每侧三组各完成 6 次，逐步减少辅助"},
			},
		},
		{
			id: "pullup", name: "引体向上", icon: "↑", target: "背阔肌、肱二头肌、握力", equipment: "稳固横杆或吊环",
			summary:  "先学会肩胛控制与水平拉，再逐渐承担完整体重。",
			cues:     []string{"先下沉肩胛再屈肘拉起", "保持肋骨收紧，不用摆腿制造惯性"},
			mistakes: []string{"耸肩并用颈部追杆", "快速坠落，失去离心控制"},
			levels: []levelSpec{
				{"肩胛下沉悬挂", 3, 0, 15, 45, "稳定保持", 6, "横杆", "三组各稳定保持 30 秒"},
				{"垂直划船", 2, 12, 0, 60, "3-1-2", 8, "门框或立柱", "两组各完成 20 次"},
				{"高位斜身划船", 3, 10, 0, 75, "3-1-2", 10, "腰高横杆", "三组各完成 15 次"},
				{"低位水平划船", 3, 8, 0, 90, "3-1-2", 12, "低横杆或吊环", "三组各完成 12 次"},
				{"脚踏辅助引体", 3, 6, 0, 90, "3-1-2", 12, "横杆与踏箱", "三组各完成 10 次，逐步减少腿部辅助"},
				{"离心引体向上", 3, 5, 0, 120, "5 秒下降", 14, "横杆与踏箱", "三组各完成 6 次，每次下降不少于 5 秒"},
				{"标准引体向上", 3, 4, 0, 120, "2-1-3", 15, "横杆", "三组各完成 8 次"},
				{"窄距引体向上", 3, 4, 0, 135, "2-1-3", 15, "横杆", "三组各完成 8 次"},
				{"偏重引体向上", 3, 3, 0, 150, "2-1-3", 17, "横杆与毛巾", "每侧三组各完成 5 次"},
				{"辅助单臂引体", 3, 2, 0, 180, "2-1-4", 18, "横杆与弹力带", "每侧三组各完成 4 次，并逐步降低辅助"},
			},
		},
		{
			id: "legraise", name: "举腿", icon: "⌁", target: "腹直肌、髋屈肌、躯干稳定", equipment: "地面、长凳或横杆",
			summary:  "从骨盆后倾与屈膝动作开始，逐步增加腿部杠杆和悬垂难度。",
			cues:     []string{"先收紧腹部并让骨盆轻微后倾", "动作范围以腰背稳定、不摆动为准"},
			mistakes: []string{"用惯性甩腿", "腰部过度拱起并出现不适"},
			levels: []levelSpec{
				{"仰卧骨盆卷动", 2, 12, 0, 45, "2-1-2", 6, "软垫", "两组各完成 20 次"},
				{"坐姿屈膝", 2, 12, 0, 45, "2-1-2", 7, "长凳", "两组各完成 20 次"},
				{"仰卧屈膝抬腿", 3, 10, 0, 60, "3-1-2", 9, "软垫", "三组各完成 15 次"},
				{"仰卧直腿抬腿", 3, 8, 0, 75, "3-1-2", 10, "软垫", "三组各完成 12 次且腰背稳定"},
				{"悬垂屈膝", 3, 8, 0, 90, "2-1-3", 12, "横杆", "三组各完成 12 次，无摆动"},
				{"悬垂高抬膝", 3, 6, 0, 105, "2-1-3", 13, "横杆", "三组各完成 10 次，膝盖接近胸口"},
				{"悬垂半直腿举腿", 3, 6, 0, 120, "2-1-3", 14, "横杆", "三组各完成 8 次"},
				{"悬垂直腿举腿", 3, 5, 0, 135, "2-1-3", 15, "横杆", "三组各完成 8 次至水平"},
				{"悬垂高位举腿", 3, 4, 0, 150, "2-1-4", 16, "横杆", "三组各完成 6 次，脚尖高于横杆"},
				{"慢速脚尖触杆", 3, 3, 0, 180, "3-1-5", 18, "横杆", "三组各完成 5 次，全程无摆动"},
			},
		},
		{
			id: "bridge", name: "桥", icon: "⌒", target: "臀肌、腿后侧、脊柱伸展肌群", equipment: "软垫、墙面或长凳",
			summary:  "先建立臀部伸展力量，再逐步增加肩部活动度和全身后链控制。",
			cues:     []string{"先收紧臀部再抬髋", "增加幅度时保持均匀呼吸，不挤压颈部"},
			mistakes: []string{"全部压力落在腰椎", "手脚打滑或在疼痛中强行增加幅度"},
			levels: []levelSpec{
				{"臀桥保持", 3, 0, 20, 45, "稳定保持", 7, "软垫", "三组各保持 40 秒"},
				{"臀桥", 3, 12, 0, 60, "3-1-2", 9, "软垫", "三组各完成 20 次"},
				{"直腿桥", 3, 10, 0, 60, "3-1-2", 10, "软垫", "三组各完成 15 次"},
				{"抬脚臀桥", 3, 8, 0, 75, "3-1-2", 11, "长凳", "三组各完成 12 次"},
				{"单腿臀桥", 3, 8, 0, 90, "3-1-2", 12, "软垫", "每侧三组各完成 12 次"},
				{"桌式桥", 3, 8, 0, 90, "3-1-2", 12, "地面", "三组各完成 12 次，肩髋充分伸展"},
				{"高位轮式桥", 3, 6, 0, 105, "3-2-3", 14, "稳固长凳", "三组各完成 10 次"},
				{"地面轮式桥", 3, 4, 0, 120, "3-2-3", 15, "软垫", "三组各完成 8 次，无腰部疼痛"},
				{"墙面下桥", 3, 3, 0, 150, "缓慢分段", 17, "墙面", "三组各完成 5 次，能按原路返回"},
				{"辅助站立下桥", 3, 2, 0, 180, "缓慢分段", 18, "训练伙伴或吊带", "三组各完成 3 次，全程可控"},
			},
		},
		{
			id: "handstand", name: "倒立撑", icon: "⇅", target: "肩、肱三头肌、上背与躯干", equipment: "墙面、箱子或软垫",
			summary:  "先建立倒置承重和肩胛上旋，再逐步增加垂直推举幅度。",
			cues:     []string{"双手主动推地，肩膀远离耳朵", "腹臀收紧，先掌握退出动作"},
			mistakes: []string{"颈部直接承重", "未掌握安全下法就追求更大幅度"},
			levels: []levelSpec{
				{"俯身肩胛推", 2, 12, 0, 45, "2-1-2", 7, "地面", "两组各完成 20 次"},
				{"下犬式保持", 3, 0, 20, 45, "稳定保持", 7, "软垫", "三组各保持 40 秒"},
				{"派克俯卧撑", 3, 8, 0, 75, "3-1-2", 10, "地面", "三组各完成 12 次"},
				{"高脚派克俯卧撑", 3, 6, 0, 90, "3-1-2", 12, "稳固箱子", "三组各完成 10 次"},
				{"面墙倒立保持", 3, 0, 20, 90, "稳定保持", 10, "墙面与软垫", "三组各保持 45 秒，并能安全退出"},
				{"倒立肩胛推", 3, 6, 0, 105, "2-1-2", 12, "墙面与软垫", "三组各完成 10 次"},
				{"半程靠墙倒立撑", 3, 4, 0, 120, "3-1-2", 14, "墙面与软垫", "三组各完成 8 次，深度一致"},
				{"靠墙倒立撑", 3, 3, 0, 150, "3-1-3", 16, "墙面与软垫", "三组各完成 6 次"},
				{"加深靠墙倒立撑", 3, 2, 0, 180, "4-1-3", 18, "墙面、推举把或瑜伽砖", "三组各完成 5 次"},
				{"辅助自由倒立撑", 3, 2, 0, 210, "4-1-3", 20, "训练伙伴或墙面", "三组各完成 3 次，辅助逐步减少"},
			},
		},
	}

	movements := make([]ProfessionalMovement, 0, len(specs))
	for _, spec := range specs {
		movement := ProfessionalMovement{
			ID: spec.id, Name: spec.name, Icon: spec.icon, Target: spec.target,
			Equipment: spec.equipment, Summary: spec.summary,
			Levels: make([]ProfessionalLevel, 0, len(spec.levels)),
		}
		for index, level := range spec.levels {
			difficulty := "基础"
			if index >= 3 {
				difficulty = "进阶"
			}
			if index >= 7 {
				difficulty = "挑战"
			}
			movement.Levels = append(movement.Levels, ProfessionalLevel{
				Level: index + 1, ID: fmt.Sprintf("%s-%02d", spec.id, index+1), Name: level.name,
				Sets: level.sets, Reps: level.reps, HoldSeconds: level.holdSeconds,
				RestSeconds: level.restSeconds, Tempo: level.tempo, Duration: level.duration,
				Cues: append([]string(nil), spec.cues...), Mistakes: append([]string(nil), spec.mistakes...),
				Advance: level.advance, Equipment: level.equipment, Target: spec.target,
				Difficulty: difficulty, MovementID: spec.id, MovementName: spec.name,
			})
		}
		movements = append(movements, movement)
	}
	return ProfessionalCatalog{
		Version:   professionalCatalogVer,
		Notice:    "训练参考不替代医疗建议。出现疼痛、眩晕或异常不适时立即停止；存在伤病或健康风险时先咨询专业人士。",
		Movements: movements,
	}
}

func defaultProfessionalProfile() ProfessionalProfile {
	levels := make(map[string]int)
	for _, movement := range ProfessionalCatalogData().Movements {
		levels[movement.ID] = 1
	}
	return ProfessionalProfile{Version: professionalCatalogVer, Levels: levels, DaysPerWeek: 3}
}

func normalizeProfessionalProfile(profile ProfessionalProfile) (ProfessionalProfile, error) {
	defaults := defaultProfessionalProfile()
	if profile.Levels == nil {
		profile.Levels = make(map[string]int)
	}
	for movementID := range profile.Levels {
		if _, ok := defaults.Levels[movementID]; !ok {
			return ProfessionalProfile{}, fmt.Errorf("unknown movement: %s", movementID)
		}
	}
	normalizedLevels := make(map[string]int, len(defaults.Levels))
	for movementID := range defaults.Levels {
		level := profile.Levels[movementID]
		if level == 0 {
			level = 1
		}
		if level < 1 || level > 10 {
			return ProfessionalProfile{}, fmt.Errorf("%s level must be between 1 and 10", movementID)
		}
		normalizedLevels[movementID] = level
	}
	profile.Levels = normalizedLevels
	if profile.DaysPerWeek == 0 {
		profile.DaysPerWeek = 3
	}
	if profile.DaysPerWeek != 2 && profile.DaysPerWeek != 3 && profile.DaysPerWeek != 4 {
		return ProfessionalProfile{}, errors.New("days_per_week must be 2, 3 or 4")
	}
	if profile.StartDate != "" {
		if _, err := time.Parse("2006-01-02", profile.StartDate); err != nil {
			return ProfessionalProfile{}, errors.New("start_date must use YYYY-MM-DD")
		}
	}
	profile.Version = professionalCatalogVer
	return profile, nil
}

func GetProfessionalProfile(acc string) (ProfessionalProfile, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()
	return getProfessionalProfileInternal(acc)
}

func getProfessionalProfileInternal(acc string) (ProfessionalProfile, error) {
	profile := defaultProfessionalProfile()
	stored := blog.GetBlogWithAccount(acc, professionalProfileTitle)
	if stored == nil || strings.TrimSpace(stored.Content) == "" {
		return profile, nil
	}
	if err := json.Unmarshal([]byte(stored.Content), &profile); err != nil {
		return ProfessionalProfile{}, fmt.Errorf("invalid professional profile: %w", err)
	}
	return normalizeProfessionalProfile(profile)
}

func SaveProfessionalProfile(acc string, profile ProfessionalProfile) (ProfessionalProfile, error) {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()
	return saveProfessionalProfileInternal(acc, profile)
}

func saveProfessionalProfileInternal(acc string, profile ProfessionalProfile) (ProfessionalProfile, error) {
	normalized, err := normalizeProfessionalProfile(profile)
	if err != nil {
		return ProfessionalProfile{}, err
	}
	normalized.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	content, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return ProfessionalProfile{}, err
	}
	data := &module.UploadedBlogData{
		Title: professionalProfileTitle, Content: string(content), Tags: "exercise-system",
		AuthType: module.EAuthType_private, Account: acc,
	}
	if blog.GetBlogWithAccount(acc, professionalProfileTitle) == nil {
		if blog.AddBlogWithAccount(acc, data) != 0 {
			return ProfessionalProfile{}, errors.New("failed to create professional profile")
		}
	} else if blog.ModifyBlogWithAccount(acc, data) != 0 {
		return ProfessionalProfile{}, errors.New("failed to update professional profile")
	}
	return normalized, nil
}

func PreviewProfessionalPlan(req ProfessionalPlanRequest) (ProfessionalPlan, error) {
	profile, err := normalizeProfessionalProfile(ProfessionalProfile{
		Levels: req.Levels, DaysPerWeek: req.DaysPerWeek, StartDate: req.StartDate,
	})
	if err != nil {
		return ProfessionalPlan{}, err
	}
	if profile.StartDate == "" {
		return ProfessionalPlan{}, errors.New("start_date is required")
	}
	start, _ := time.Parse("2006-01-02", profile.StartDate)
	catalog := ProfessionalCatalogData()
	byID := make(map[string]ProfessionalMovement, len(catalog.Movements))
	for _, movement := range catalog.Movements {
		byID[movement.ID] = movement
	}

	var schedule []struct {
		offset    int
		movements []string
	}
	switch profile.DaysPerWeek {
	case 2:
		schedule = []struct {
			offset    int
			movements []string
		}{{0, []string{"pushup", "squat", "legraise"}}, {3, []string{"pullup", "bridge", "handstand"}}}
	case 3:
		schedule = []struct {
			offset    int
			movements []string
		}{{0, []string{"pushup", "legraise"}}, {2, []string{"squat", "bridge"}}, {4, []string{"pullup", "handstand"}}}
	case 4:
		schedule = []struct {
			offset    int
			movements []string
		}{{0, []string{"pushup", "legraise"}}, {1, []string{"squat"}}, {3, []string{"pullup", "bridge"}}, {5, []string{"handstand"}}}
	}

	plan := ProfessionalPlan{
		StartDate: profile.StartDate, EndDate: start.AddDate(0, 0, 6).Format("2006-01-02"),
		DaysPerWeek: profile.DaysPerWeek, Sessions: make([]ProfessionalPlanSession, 0, len(schedule)),
	}
	for _, scheduled := range schedule {
		session := ProfessionalPlanSession{
			Date: start.AddDate(0, 0, scheduled.offset).Format("2006-01-02"),
			Day:  scheduled.offset + 1, Items: make([]ProfessionalPlanItem, 0, len(scheduled.movements)),
		}
		for _, movementID := range scheduled.movements {
			movement := byID[movementID]
			level := movement.Levels[profile.Levels[movementID]-1]
			session.Items = append(session.Items, ProfessionalPlanItem{
				MovementID: movementID, MovementName: movement.Name, ProgressionLevel: level.Level,
				Name: level.Name, Sets: level.Sets, Reps: level.Reps, HoldSeconds: level.HoldSeconds,
				RestSeconds: level.RestSeconds, Tempo: level.Tempo, Duration: level.Duration, Target: level.Target,
			})
		}
		plan.Sessions = append(plan.Sessions, session)
	}
	plan.ID = professionalPlanID(profile)
	return plan, nil
}

func professionalPlanID(profile ProfessionalProfile) string {
	keys := make([]string, 0, len(profile.Levels))
	for key := range profile.Levels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	fmt.Fprintf(&builder, "v%d|%s|%d", professionalCatalogVer, profile.StartDate, profile.DaysPerWeek)
	for _, key := range keys {
		fmt.Fprintf(&builder, "|%s=%d", key, profile.Levels[key])
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:8])
}

func ApplyProfessionalPlan(acc string, req ProfessionalPlanRequest) (ProfessionalApplyResult, error) {
	plan, err := PreviewProfessionalPlan(req)
	if err != nil {
		return ProfessionalApplyResult{}, err
	}
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	planDates := professionalPlanDates(plan)
	lists := make(map[string]ExerciseList, len(planDates))
	beforeLengths := make(map[string]int, len(planDates))
	for _, date := range planDates {
		list, err := getExercisesByDateInternal(acc, date)
		if err != nil {
			return ProfessionalApplyResult{}, err
		}
		lists[date] = list
		beforeLengths[date] = len(list.Items)
	}
	result, err := applyProfessionalPlanToLists(lists, plan, req.Replace, time.Now())
	if err != nil {
		return result, err
	}
	scheduledDates := make(map[string]bool, len(plan.Sessions))
	for _, session := range plan.Sessions {
		scheduledDates[session.Date] = true
	}
	for _, date := range planDates {
		if !scheduledDates[date] && beforeLengths[date] == len(lists[date].Items) {
			continue
		}
		if err := saveExercisesToBlog(acc, lists[date]); err != nil {
			return ProfessionalApplyResult{}, err
		}
	}
	_, err = saveProfessionalProfileInternal(acc, ProfessionalProfile{
		Levels: req.Levels, DaysPerWeek: req.DaysPerWeek, StartDate: req.StartDate,
	})
	return result, err
}

func applyProfessionalPlanToLists(lists map[string]ExerciseList, plan ProfessionalPlan, replace bool, now time.Time) (ProfessionalApplyResult, error) {
	result := ProfessionalApplyResult{PlanID: plan.ID, Replaced: replace}
	hasExisting := false
	for _, date := range professionalPlanDates(plan) {
		list := lists[date]
		for _, item := range list.Items {
			if item.Source == professionalSource {
				hasExisting = true
				break
			}
		}
	}
	if hasExisting && !replace {
		return result, ErrProfessionalPlanConflict
	}

	if replace {
		for _, date := range professionalPlanDates(plan) {
			list := lists[date]
			kept := make([]ExerciseItem, 0, len(list.Items))
			for _, item := range list.Items {
				if item.Source == professionalSource && !item.Completed {
					continue
				}
				if item.Source == professionalSource && item.Completed {
					result.Preserved++
				}
				kept = append(kept, item)
			}
			list.Items = kept
			lists[date] = list
		}
	}

	sequence := int64(0)
	for _, session := range plan.Sessions {
		list := lists[session.Date]
		if list.Date == "" {
			list.Date = session.Date
		}
		for _, planned := range session.Items {
			alreadyCompleted := false
			for _, item := range list.Items {
				if item.Source == professionalSource && item.MovementID == planned.MovementID && item.Completed {
					alreadyCompleted = true
					break
				}
				if item.PlanID == plan.ID && item.MovementID == planned.MovementID {
					alreadyCompleted = true
					break
				}
			}
			if alreadyCompleted {
				result.Skipped++
				continue
			}
			sequence++
			intensity := "low"
			if planned.ProgressionLevel >= 4 {
				intensity = "medium"
			}
			if planned.ProgressionLevel >= 8 {
				intensity = "high"
			}
			item := ExerciseItem{
				ID: fmt.Sprintf("%d", now.UnixNano()+sequence), Name: planned.Name, Type: "strength",
				Duration: planned.Duration, Intensity: intensity, Completed: false, CreatedAt: now,
				BodyParts: []string{planned.Target}, Source: professionalSource, PlanID: plan.ID,
				MovementID: planned.MovementID, ProgressionLevel: planned.ProgressionLevel,
				Sets: planned.Sets, Reps: planned.Reps, HoldSeconds: planned.HoldSeconds,
				RestSeconds: planned.RestSeconds, Tempo: planned.Tempo,
				Notes: professionalPrescription(planned),
			}
			list.Items = append(list.Items, item)
			result.Created++
		}
		lists[session.Date] = list
	}
	return result, nil
}

func professionalPlanDates(plan ProfessionalPlan) []string {
	start, err := time.Parse("2006-01-02", plan.StartDate)
	if err != nil {
		return nil
	}
	dates := make([]string, 0, 7)
	for offset := 0; offset < 7; offset++ {
		dates = append(dates, start.AddDate(0, 0, offset).Format("2006-01-02"))
	}
	return dates
}

func professionalPrescription(item ProfessionalPlanItem) string {
	target := fmt.Sprintf("%d 组 × %d 次", item.Sets, item.Reps)
	if item.HoldSeconds > 0 {
		target = fmt.Sprintf("%d 组 × %d 秒", item.Sets, item.HoldSeconds)
	}
	return fmt.Sprintf("专业训练：%s；节奏 %s；组间休息 %d 秒。", target, item.Tempo, item.RestSeconds)
}
