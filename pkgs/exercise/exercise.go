package exercise

import (
	"core"
	"encoding/json"
	log "mylog"
	"time"
)

// ExerciseItem represents a single exercise entry
type ExerciseItem struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`      // 锻炼项目名称
	Type        string     `json:"type"`      // 锻炼类型：cardio, strength, flexibility, sports
	Duration    int        `json:"duration"`  // 持续时间（分钟）
	Intensity   string     `json:"intensity"` // 强度：low, medium, high
	Calories    int        `json:"calories"`  // 消耗卡路里
	Notes       string     `json:"notes"`     // 备注
	Completed   bool       `json:"completed"` // 是否完成
	Weight      float64    `json:"weight"`    // 负重 (kg)
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	BodyParts   []string   `json:"body_parts"` // 新增，锻炼部位
}

// ExerciseTemplate represents an exercise template
type ExerciseTemplate struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Duration  int      `json:"duration"`
	Intensity string   `json:"intensity"`
	Calories  int      `json:"calories"`
	Notes     string   `json:"notes"`
	Weight    float64  `json:"weight"`     // 负重 (kg)
	BodyParts []string `json:"body_parts"` // 新增，锻炼部位
}

// ExerciseTemplateCollection represents a collection of exercise templates
type ExerciseTemplateCollection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	TemplateIDs []string  `json:"template_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserProfile represents user's basic information for exercise calculation
type UserProfile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Gender    string    `json:"gender"` // male, female
	Weight    float64   `json:"weight"` // kg
	Height    float64   `json:"height"` // cm
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// METValue represents metabolic equivalent values for different exercises
type METValue struct {
	ExerciseType string  `json:"exercise_type"`
	Intensity    string  `json:"intensity"`
	MET          float64 `json:"met"`
	Description  string  `json:"description"`
}

// ExerciseList represents a collection of exercises for a specific date
type ExerciseList struct {
	Date      string             `json:"date"`
	Items     []ExerciseItem     `json:"items"`
	Templates []ExerciseTemplate `json:"templates"`
}

// ExerciseStats represents exercise statistics
type ExerciseStats struct {
	Period        string         `json:"period"` // week, month, year
	StartDate     string         `json:"start_date"`
	EndDate       string         `json:"end_date"`
	TotalDays     int            `json:"total_days"`
	ExerciseDays  int            `json:"exercise_days"`
	TotalDuration int            `json:"total_duration"` // 总锻炼时间（分钟）
	TotalCalories int            `json:"total_calories"` // 总消耗卡路里
	TypeStats     map[string]int `json:"type_stats"`     // 各类型锻炼次数
	WeeklyAvg     float64        `json:"weekly_avg"`     // 周平均锻炼时间
	Consistency   float64        `json:"consistency"`    // 坚持率（锻炼天数/总天数）
}

// 锻炼模块actor
var exercise_module *ExerciseActor

func Info() {
	log.Debug("info exercise v1.0")
}

// 初始化exercise模块，用于锻炼管理
func Init() {
	exercise_module = &ExerciseActor{
		Actor: core.NewActor(),
	}
	exercise_module.Start(exercise_module)
}

// interface

func AddExercise(account, date, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) (*ExerciseItem, error) {
	cmd := &AddExerciseCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		Date:      date,
		Name:      name,
		Type:      exerciseType,
		Duration:  duration,
		Intensity: intensity,
		Calories:  calories,
		Notes:     notes,
		Weight:    weight,
		BodyParts: bodyParts,
	}
	exercise_module.Send(cmd)
	item := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return item.(*ExerciseItem), nil
}

func DeleteExercise(account, date, id string) error {
	cmd := &DeleteExerciseCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Date:    date,
		ID:      id,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func UpdateExercise(account, date, id, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) error {
	cmd := &UpdateExerciseCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		Date:      date,
		ID:        id,
		Name:      name,
		Type:      exerciseType,
		Duration:  duration,
		Intensity: intensity,
		Calories:  calories,
		Notes:     notes,
		Weight:    weight,
		BodyParts: bodyParts,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func ToggleExercise(account, date, id string) error {
	cmd := &ToggleExerciseCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Date:    date,
		ID:      id,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func GetExercisesByDate(account, date string) (ExerciseList, error) {
	cmd := &GetExercisesByDateCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Date:    date,
	}
	exercise_module.Send(cmd)
	exercises := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return ExerciseList{}, err.(error)
	}
	return exercises.(ExerciseList), nil
}

func GetAllExercises(account string) (map[string]ExerciseList, error) {
	cmd := &GetAllExercisesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	exercise_module.Send(cmd)
	exercises := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return exercises.(map[string]ExerciseList), nil
}

func AddTemplate(account, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) (*ExerciseTemplate, error) {
	cmd := &AddTemplateCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		Name:      name,
		Type:      exerciseType,
		Duration:  duration,
		Intensity: intensity,
		Calories:  calories,
		Notes:     notes,
		Weight:    weight,
		BodyParts: bodyParts,
	}
	exercise_module.Send(cmd)
	template := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return template.(*ExerciseTemplate), nil
}

func DeleteTemplate(account, id string) error {
	cmd := &DeleteTemplateCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		ID:      id,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func UpdateTemplate(account, id, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) error {
	cmd := &UpdateTemplateCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		ID:        id,
		Name:      name,
		Type:      exerciseType,
		Duration:  duration,
		Intensity: intensity,
		Calories:  calories,
		Notes:     notes,
		Weight:    weight,
		BodyParts: bodyParts,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func GetTemplates(account string) ([]ExerciseTemplate, error) {
	cmd := &GetTemplatesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	exercise_module.Send(cmd)
	templates := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return templates.([]ExerciseTemplate), nil
}

func GetWeeklyStats(account, startDate string) (*ExerciseStats, error) {
	cmd := &GetWeeklyStatsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:   account,
		StartDate: startDate,
	}
	exercise_module.Send(cmd)
	stats := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return stats.(*ExerciseStats), nil
}

func GetMonthlyStats(account string, year int, month int) (*ExerciseStats, error) {
	cmd := &GetMonthlyStatsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Year:    year,
		Month:   month,
	}
	exercise_module.Send(cmd)
	stats := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return stats.(*ExerciseStats), nil
}

func GetYearlyStats(account string, year int) (*ExerciseStats, error) {
	cmd := &GetYearlyStatsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Year:    year,
	}
	exercise_module.Send(cmd)
	stats := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return stats.(*ExerciseStats), nil
}

func AddCollection(account, name, description string, templateIDs []string) (*ExerciseTemplateCollection, error) {
	cmd := &AddCollectionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:     account,
		Name:        name,
		Description: description,
		TemplateIDs: templateIDs,
	}
	exercise_module.Send(cmd)
	collection := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return collection.(*ExerciseTemplateCollection), nil
}

func DeleteCollection(account, id string) error {
	cmd := &DeleteCollectionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		ID:      id,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func UpdateCollection(account, id, name, description string, templateIDs []string) error {
	cmd := &UpdateCollectionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:     account,
		ID:          id,
		Name:        name,
		Description: description,
		TemplateIDs: templateIDs,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func GetCollections(account string) ([]ExerciseTemplateCollection, error) {
	cmd := &GetCollectionsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	exercise_module.Send(cmd)
	collections := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return collections.([]ExerciseTemplateCollection), nil
}

func GetCollectionWithTemplates(account, collectionID string) (*ExerciseTemplateCollection, []ExerciseTemplate, error) {
	cmd := &GetCollectionWithTemplatesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:      account,
		CollectionID: collectionID,
	}
	exercise_module.Send(cmd)
	collection := <-cmd.Response()
	templates := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, nil, err.(error)
	}
	return collection.(*ExerciseTemplateCollection), templates.([]ExerciseTemplate), nil
}

func AddFromCollection(account, date, collectionID string) error {
	cmd := &AddFromCollectionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:      account,
		Date:         date,
		CollectionID: collectionID,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func CalculateCalories(account, exerciseType, intensity string, duration int, weight float64) int {
	cmd := &CalculateCaloriesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:      account,
		ExerciseType: exerciseType,
		Intensity:    intensity,
		Duration:     duration,
		Weight:       weight,
	}
	exercise_module.Send(cmd)
	calories := <-cmd.Response()
	return calories.(int)
}

func GetMETValueWithDescription(account, exerciseType, intensity string) (float64, string) {
	cmd := &GetMETValueWithDescriptionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:      account,
		ExerciseType: exerciseType,
		Intensity:    intensity,
	}
	exercise_module.Send(cmd)
	met := <-cmd.Response()
	description := <-cmd.Response()
	return met.(float64), description.(string)
}

func SaveUserProfile(account, name, gender string, weight, height float64, age int) (*UserProfile, error) {
	cmd := &SaveUserProfileCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Name:    name,
		Gender:  gender,
		Weight:  weight,
		Height:  height,
		Age:     age,
	}
	exercise_module.Send(cmd)
	profile := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return profile.(*UserProfile), nil
}

func GetUserProfile(account string) (*UserProfile, error) {
	cmd := &GetUserProfileCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	exercise_module.Send(cmd)
	profile := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	if profile == nil {
		return nil, nil
	}
	return profile.(*UserProfile), nil
}

func GetMETValues(account string) ([]METValue, error) {
	cmd := &GetMETValuesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
	}
	exercise_module.Send(cmd)
	metValues := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return nil, err.(error)
	}
	return metValues.([]METValue), nil
}

func UpdateAllTemplateCalories(account string, weight float64) error {
	cmd := &UpdateAllTemplateCaloriesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Weight:  weight,
	}
	exercise_module.Send(cmd)
	err := <-cmd.Response()
	if err != nil {
		return err.(error)
	}
	return nil
}

func UpdateAllExerciseCalories(account string, weight float64) (int, error) {
	cmd := &UpdateAllExerciseCaloriesCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account: account,
		Weight:  weight,
	}
	exercise_module.Send(cmd)
	updatedCount := <-cmd.Response()
	err := <-cmd.Response()
	if err != nil {
		return 0, err.(error)
	}
	return updatedCount.(int), nil
}

// ParseExerciseFromBlog parses a blog content string into an ExerciseList
func ParseExerciseFromBlog(content string) ExerciseList {
	var exerciseList ExerciseList
	if err := json.Unmarshal([]byte(content), &exerciseList); err != nil {
		// Return empty ExerciseList if parsing fails
		return ExerciseList{Items: []ExerciseItem{}}
	}
	return exerciseList
}
