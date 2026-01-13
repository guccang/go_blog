package exercise

import (
	"account"
	"blog"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"strings"
	"sync"
	"time"
)

// ========== Simple Exercise 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

var exerciseMu sync.RWMutex

// ========== 数据结构 ==========

type ExerciseItem struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Duration    int        `json:"duration"`
	Intensity   string     `json:"intensity"`
	Calories    int        `json:"calories"`
	Notes       string     `json:"notes"`
	Completed   bool       `json:"completed"`
	Weight      float64    `json:"weight"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	BodyParts   []string   `json:"body_parts"`
}

type ExerciseTemplate struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Duration  int      `json:"duration"`
	Intensity string   `json:"intensity"`
	Calories  int      `json:"calories"`
	Notes     string   `json:"notes"`
	Weight    float64  `json:"weight"`
	BodyParts []string `json:"body_parts"`
}

type ExerciseTemplateCollection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	TemplateIDs []string  `json:"template_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserProfile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Gender    string    `json:"gender"`
	Weight    float64   `json:"weight"`
	Height    float64   `json:"height"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type METValue struct {
	ExerciseType string  `json:"exercise_type"`
	Intensity    string  `json:"intensity"`
	MET          float64 `json:"met"`
	Description  string  `json:"description"`
}

type ExerciseList struct {
	Date      string             `json:"date"`
	Items     []ExerciseItem     `json:"items"`
	Templates []ExerciseTemplate `json:"templates"`
}

type ExerciseStats struct {
	Period        string         `json:"period"`
	StartDate     string         `json:"start_date"`
	EndDate       string         `json:"end_date"`
	TotalDays     int            `json:"total_days"`
	ExerciseDays  int            `json:"exercise_days"`
	TotalDuration int            `json:"total_duration"`
	TotalCalories int            `json:"total_calories"`
	TypeStats     map[string]int `json:"type_stats"`
	WeeklyAvg     float64        `json:"weekly_avg"`
	Consistency   float64        `json:"consistency"`
}

func Info() {
	log.Debug(log.ModuleExercise, "info exercise v2.0 (simple)")
}

func Init() {
	log.Debug(log.ModuleExercise, "exercise module initialized")
}

// ========== 辅助函数 ==========

func generateBlogTitle(date string) string { return fmt.Sprintf("exercise-%s", date) }
func generateTemplateBlogTitle() string    { return "exercise-templates" }
func generateCollectionBlogTitle() string  { return "exercise-template-collections" }
func generateUserProfileBlogTitle() string { return "exercise-user-profile" }
func generateMETValuesBlogTitle() string   { return "exercise-met-values" }

func getDateFromTitle(title string) string {
	if strings.HasPrefix(title, "exercise-") && title != "exercise-templates" {
		return strings.TrimPrefix(title, "exercise-")
	}
	return ""
}

func getDefaultMETValues() []METValue {
	return []METValue{
		{"cardio", "low", 3.5, "慢走"}, {"cardio", "medium", 6.0, "慢跑"}, {"cardio", "high", 10.0, "快跑"},
		{"strength", "low", 3.0, "轻度力量训练"}, {"strength", "medium", 5.0, "中等力量训练"}, {"strength", "high", 8.0, "高强度力量训练"},
		{"flexibility", "low", 2.5, "拉伸"}, {"flexibility", "medium", 3.0, "瑜伽"}, {"flexibility", "high", 4.0, "高强度瑜伽"},
		{"sports", "low", 4.0, "休闲运动"}, {"sports", "medium", 7.0, "中等强度运动"}, {"sports", "high", 10.0, "激烈运动"},
		{"other", "low", 2.5, "其他轻度活动"}, {"other", "medium", 4.0, "其他中等活动"}, {"other", "high", 6.0, "其他高强度活动"},
	}
}

func getMETValue(exerciseType, intensity string) float64 {
	for _, mv := range getDefaultMETValues() {
		if mv.ExerciseType == exerciseType && mv.Intensity == intensity {
			return mv.MET
		}
	}
	switch intensity {
	case "low":
		return 3.0
	case "medium":
		return 5.0
	case "high":
		return 8.0
	default:
		return 4.0
	}
}

func calculateCaloriesInternal(exerciseType, intensity string, duration int, weight float64) int {
	met := getMETValue(exerciseType, intensity)
	hours := float64(duration) / 60.0
	return int(met * weight * hours)
}

// ========== 对外接口 ==========

func AddExercise(acc, date, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) (*ExerciseItem, error) {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	exerciseList, _ := getExercisesByDateInternal(acc, date)
	if exerciseList.Date == "" {
		exerciseList = ExerciseList{Date: date, Items: []ExerciseItem{}}
	}

	if calories == 0 {
		if profile, _ := getUserProfileInternal(acc); profile != nil && profile.Weight > 0 {
			calories = calculateCaloriesInternal(exerciseType, intensity, duration, profile.Weight+weight)
		}
	}

	item := ExerciseItem{
		ID: fmt.Sprintf("%d", time.Now().UnixNano()), Name: name, Type: exerciseType,
		Duration: duration, Intensity: intensity, Calories: calories, Notes: notes,
		Completed: false, Weight: weight, CreatedAt: time.Now(), BodyParts: bodyParts,
	}
	exerciseList.Items = append(exerciseList.Items, item)

	if err := saveExercisesToBlog(acc, exerciseList); err != nil {
		return nil, err
	}
	return &item, nil
}

func DeleteExercise(acc, date, id string) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	exerciseList, err := getExercisesByDateInternal(acc, date)
	if err != nil {
		return err
	}

	found := false
	updatedItems := make([]ExerciseItem, 0, len(exerciseList.Items))
	for _, item := range exerciseList.Items {
		if item.ID != id {
			updatedItems = append(updatedItems, item)
		} else {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("exercise item not found")
	}
	exerciseList.Items = updatedItems
	return saveExercisesToBlog(acc, exerciseList)
}

func UpdateExercise(acc, date, id, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	exerciseList, err := getExercisesByDateInternal(acc, date)
	if err != nil {
		return err
	}

	if calories == 0 {
		if profile, _ := getUserProfileInternal(acc); profile != nil && profile.Weight > 0 {
			calories = calculateCaloriesInternal(exerciseType, intensity, duration, profile.Weight+weight)
		}
	}

	found := false
	for i := range exerciseList.Items {
		if exerciseList.Items[i].ID == id {
			exerciseList.Items[i].Name = name
			exerciseList.Items[i].Type = exerciseType
			exerciseList.Items[i].Duration = duration
			exerciseList.Items[i].Intensity = intensity
			exerciseList.Items[i].Calories = calories
			exerciseList.Items[i].Notes = notes
			exerciseList.Items[i].Weight = weight
			exerciseList.Items[i].BodyParts = bodyParts
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("exercise item not found")
	}
	return saveExercisesToBlog(acc, exerciseList)
}

func ToggleExercise(acc, date, id string) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	exerciseList, err := getExercisesByDateInternal(acc, date)
	if err != nil {
		return err
	}

	found := false
	for i := range exerciseList.Items {
		if exerciseList.Items[i].ID == id {
			exerciseList.Items[i].Completed = !exerciseList.Items[i].Completed
			if exerciseList.Items[i].Completed {
				now := time.Now()
				exerciseList.Items[i].CompletedAt = &now
			} else {
				exerciseList.Items[i].CompletedAt = nil
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("exercise item not found")
	}
	return saveExercisesToBlog(acc, exerciseList)
}

func GetExercisesByDate(acc, date string) (ExerciseList, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()
	return getExercisesByDateInternal(acc, date)
}

func getExercisesByDateInternal(acc, date string) (ExerciseList, error) {
	b := blog.GetBlogWithAccount(acc, generateBlogTitle(date))
	if b == nil {
		return ExerciseList{Date: date, Items: []ExerciseItem{}}, nil
	}
	var exerciseList ExerciseList
	if err := json.Unmarshal([]byte(b.Content), &exerciseList); err != nil {
		return ExerciseList{Date: date, Items: []ExerciseItem{}}, nil
	}
	return exerciseList, nil
}

func GetAllExercises(acc string) (map[string]ExerciseList, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()
	return getAllExercisesInternal(acc)
}

func getAllExercisesInternal(acc string) (map[string]ExerciseList, error) {
	result := make(map[string]ExerciseList)
	for _, b := range blog.GetBlogsWithAccount(acc) {
		date := getDateFromTitle(b.Title)
		if date != "" {
			var exerciseList ExerciseList
			if err := json.Unmarshal([]byte(b.Content), &exerciseList); err == nil {
				result[date] = exerciseList
			}
		}
	}
	return result, nil
}

func saveExercisesToBlog(acc string, exerciseList ExerciseList) error {
	title := generateBlogTitle(exerciseList.Date)
	content, err := json.MarshalIndent(exerciseList, "", "  ")
	if err != nil {
		return err
	}
	ubd := &module.UploadedBlogData{
		Title: title, Content: string(content), Tags: "exercise", AuthType: module.EAuthType_private, Account: acc,
	}
	if blog.GetBlogWithAccount(acc, title) == nil {
		blog.AddBlogWithAccount(acc, ubd)
	} else {
		blog.ModifyBlogWithAccount(acc, ubd)
	}
	return nil
}

// ========== Template 接口 ==========

func AddTemplate(acc, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) (*ExerciseTemplate, error) {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	templates, _ := getTemplatesInternal(acc)
	template := ExerciseTemplate{
		ID: fmt.Sprintf("%d", time.Now().UnixNano()), Name: name, Type: exerciseType,
		Duration: duration, Intensity: intensity, Calories: calories, Notes: notes, Weight: weight, BodyParts: bodyParts,
	}
	templates = append(templates, template)
	if err := saveTemplatesToBlog(acc, templates); err != nil {
		return nil, err
	}
	return &template, nil
}

func DeleteTemplate(acc, id string) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	templates, _ := getTemplatesInternal(acc)
	found := false
	updated := make([]ExerciseTemplate, 0, len(templates))
	for _, t := range templates {
		if t.ID != id {
			updated = append(updated, t)
		} else {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("template not found")
	}
	return saveTemplatesToBlog(acc, updated)
}

func UpdateTemplate(acc, id, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	templates, _ := getTemplatesInternal(acc)
	found := false
	for i := range templates {
		if templates[i].ID == id {
			templates[i] = ExerciseTemplate{ID: id, Name: name, Type: exerciseType, Duration: duration, Intensity: intensity, Calories: calories, Notes: notes, Weight: weight, BodyParts: bodyParts}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("template not found")
	}
	return saveTemplatesToBlog(acc, templates)
}

func GetTemplates(acc string) ([]ExerciseTemplate, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()
	return getTemplatesInternal(acc)
}

func getTemplatesInternal(acc string) ([]ExerciseTemplate, error) {
	b := blog.GetBlogWithAccount(acc, generateTemplateBlogTitle())
	if b == nil {
		return []ExerciseTemplate{}, nil
	}
	var templates []ExerciseTemplate
	if err := json.Unmarshal([]byte(b.Content), &templates); err != nil {
		return []ExerciseTemplate{}, nil
	}
	return templates, nil
}

func saveTemplatesToBlog(acc string, templates []ExerciseTemplate) error {
	title := generateTemplateBlogTitle()
	content, _ := json.MarshalIndent(templates, "", "  ")
	ubd := &module.UploadedBlogData{Title: title, Content: string(content), Tags: "exercise-template", AuthType: module.EAuthType_private}
	if blog.GetBlogWithAccount(acc, title) == nil {
		blog.AddBlogWithAccount(acc, ubd)
	} else {
		blog.ModifyBlogWithAccount(acc, ubd)
	}
	return nil
}

// ========== Collection 接口 ==========

func AddCollection(acc, name, description string, templateIDs []string) (*ExerciseTemplateCollection, error) {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	collections, _ := getCollectionsInternal(acc)
	collection := ExerciseTemplateCollection{
		ID: fmt.Sprintf("%d", time.Now().UnixNano()), Name: name, Description: description, TemplateIDs: templateIDs, CreatedAt: time.Now(),
	}
	collections = append(collections, collection)
	if err := saveCollectionsToBlog(acc, collections); err != nil {
		return nil, err
	}
	return &collection, nil
}

func DeleteCollection(acc, id string) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	collections, _ := getCollectionsInternal(acc)
	found := false
	updated := make([]ExerciseTemplateCollection, 0, len(collections))
	for _, c := range collections {
		if c.ID != id {
			updated = append(updated, c)
		} else {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("collection not found")
	}
	return saveCollectionsToBlog(acc, updated)
}

func UpdateCollection(acc, id, name, description string, templateIDs []string) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	collections, _ := getCollectionsInternal(acc)
	found := false
	for i := range collections {
		if collections[i].ID == id {
			collections[i].Name = name
			collections[i].Description = description
			collections[i].TemplateIDs = templateIDs
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("collection not found")
	}
	return saveCollectionsToBlog(acc, collections)
}

func GetCollections(acc string) ([]ExerciseTemplateCollection, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()
	return getCollectionsInternal(acc)
}

func getCollectionsInternal(acc string) ([]ExerciseTemplateCollection, error) {
	b := blog.GetBlogWithAccount(acc, generateCollectionBlogTitle())
	if b == nil {
		return []ExerciseTemplateCollection{}, nil
	}
	var collections []ExerciseTemplateCollection
	json.Unmarshal([]byte(b.Content), &collections)
	return collections, nil
}

func saveCollectionsToBlog(acc string, collections []ExerciseTemplateCollection) error {
	title := generateCollectionBlogTitle()
	content, _ := json.MarshalIndent(collections, "", "  ")
	ubd := &module.UploadedBlogData{Title: title, Content: string(content), Tags: "exercise-collection", AuthType: module.EAuthType_private}
	if blog.GetBlogWithAccount(acc, title) == nil {
		blog.AddBlogWithAccount(acc, ubd)
	} else {
		blog.ModifyBlogWithAccount(acc, ubd)
	}
	return nil
}

func GetCollectionWithTemplates(acc, collectionID string) (*ExerciseTemplateCollection, []ExerciseTemplate, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()

	collections, _ := getCollectionsInternal(acc)
	var targetCollection *ExerciseTemplateCollection
	for _, c := range collections {
		if c.ID == collectionID {
			targetCollection = &c
			break
		}
	}
	if targetCollection == nil {
		return nil, nil, fmt.Errorf("collection not found")
	}

	allTemplates, _ := getTemplatesInternal(acc)
	var collectionTemplates []ExerciseTemplate
	for _, templateID := range targetCollection.TemplateIDs {
		for _, t := range allTemplates {
			if t.ID == templateID {
				collectionTemplates = append(collectionTemplates, t)
				break
			}
		}
	}
	return targetCollection, collectionTemplates, nil
}

func AddFromCollection(acc, date, collectionID string) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	collections, _ := getCollectionsInternal(acc)
	var targetCollection *ExerciseTemplateCollection
	for _, c := range collections {
		if c.ID == collectionID {
			targetCollection = &c
			break
		}
	}
	if targetCollection == nil {
		return fmt.Errorf("collection not found")
	}

	allTemplates, _ := getTemplatesInternal(acc)
	for _, templateID := range targetCollection.TemplateIDs {
		for _, template := range allTemplates {
			if template.ID == templateID {
				exerciseList, _ := getExercisesByDateInternal(acc, date)
				if exerciseList.Date == "" {
					exerciseList = ExerciseList{Date: date, Items: []ExerciseItem{}}
				}
				calories := template.Calories
				if calories == 0 {
					if profile, _ := getUserProfileInternal(acc); profile != nil && profile.Weight > 0 {
						calories = calculateCaloriesInternal(template.Type, template.Intensity, template.Duration, profile.Weight+template.Weight)
					}
				}
				item := ExerciseItem{
					ID: fmt.Sprintf("%d", time.Now().UnixNano()), Name: template.Name, Type: template.Type,
					Duration: template.Duration, Intensity: template.Intensity, Calories: calories, Notes: template.Notes,
					Completed: false, Weight: template.Weight, CreatedAt: time.Now(), BodyParts: template.BodyParts,
				}
				exerciseList.Items = append(exerciseList.Items, item)
				saveExercisesToBlog(acc, exerciseList)
				break
			}
		}
	}
	return nil
}

// ========== Stats 接口 ==========

func GetWeeklyStats(acc, startDate string) (*ExerciseStats, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, err
	}
	weekday := int(start.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := start.AddDate(0, 0, -weekday+1)
	sunday := monday.AddDate(0, 0, 6)
	return calculateStats(acc, "week", monday.Format("2006-01-02"), sunday.Format("2006-01-02"))
}

func GetMonthlyStats(acc string, year, month int) (*ExerciseStats, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()

	start, _ := time.Parse("2006-01-02", fmt.Sprintf("%04d-%02d-01", year, month))
	end := start.AddDate(0, 1, -1)
	return calculateStats(acc, "month", start.Format("2006-01-02"), end.Format("2006-01-02"))
}

func GetYearlyStats(acc string, year int) (*ExerciseStats, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()
	return calculateStats(acc, "year", fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year))
}

func calculateStats(acc, period, startDate, endDate string) (*ExerciseStats, error) {
	allExercises, _ := getAllExercisesInternal(acc)
	stats := &ExerciseStats{Period: period, StartDate: startDate, EndDate: endDate, TypeStats: make(map[string]int)}

	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	stats.TotalDays = int(end.Sub(start).Hours()/24) + 1

	exerciseDaysSet := make(map[string]bool)
	for date, exerciseList := range allExercises {
		exerciseDate, err := time.Parse("2006-01-02", date)
		if err != nil || exerciseDate.Before(start) || exerciseDate.After(end) {
			continue
		}
		hasCompleted := false
		for _, item := range exerciseList.Items {
			if item.Completed {
				hasCompleted = true
				stats.TotalDuration += item.Duration
				stats.TotalCalories += item.Calories
				stats.TypeStats[item.Type]++
			}
		}
		if hasCompleted {
			exerciseDaysSet[date] = true
		}
	}
	stats.ExerciseDays = len(exerciseDaysSet)
	if stats.TotalDays > 0 {
		stats.Consistency = float64(stats.ExerciseDays) / float64(stats.TotalDays) * 100
	}
	if period == "week" {
		stats.WeeklyAvg = float64(stats.TotalDuration)
	} else if period == "month" {
		stats.WeeklyAvg = float64(stats.TotalDuration) / (float64(stats.TotalDays) / 7.0)
	} else {
		stats.WeeklyAvg = float64(stats.TotalDuration) / 52.0
	}
	return stats, nil
}

// ========== Profile & MET 接口 ==========

func CalculateCalories(acc, exerciseType, intensity string, duration int, weight float64) int {
	if weight <= 0 && acc != "" {
		if accountInfo, err := account.GetAccountInfo(acc); err == nil && accountInfo != nil {
			weight = accountInfo.Weight
		}
	}
	return calculateCaloriesInternal(exerciseType, intensity, duration, weight)
}

func GetMETValueWithDescription(acc, exerciseType, intensity string) (float64, string) {
	for _, mv := range getDefaultMETValues() {
		if mv.ExerciseType == exerciseType && mv.Intensity == intensity {
			return mv.MET, mv.Description
		}
	}
	switch intensity {
	case "low":
		return 3.0, "低强度活动"
	case "medium":
		return 5.0, "中等强度活动"
	case "high":
		return 8.0, "高强度活动"
	default:
		return 4.0, "一般强度活动"
	}
}

func SaveUserProfile(acc, name, gender string, weight, height float64, age int) (*UserProfile, error) {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	profile := &UserProfile{ID: "default", Name: name, Gender: gender, Weight: weight, Height: height, Age: age, UpdatedAt: time.Now()}
	existing, _ := getUserProfileInternal(acc)
	if existing == nil {
		profile.CreatedAt = time.Now()
	} else {
		profile.CreatedAt = existing.CreatedAt
	}
	if err := saveUserProfileToBlog(acc, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func GetUserProfile(acc string) (*UserProfile, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()
	return getUserProfileInternal(acc)
}

func getUserProfileInternal(acc string) (*UserProfile, error) {
	b := blog.GetBlogWithAccount(acc, generateUserProfileBlogTitle())
	if b == nil {
		if accountInfo, err := account.GetAccountInfo(acc); err == nil && accountInfo != nil && accountInfo.Weight > 0 {
			return &UserProfile{ID: acc, Name: accountInfo.Name, Weight: accountInfo.Weight, Height: accountInfo.Height, Age: accountInfo.GetAge(), CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
		}
		return nil, nil
	}
	var profile UserProfile
	if err := json.Unmarshal([]byte(b.Content), &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func saveUserProfileToBlog(acc string, profile *UserProfile) error {
	title := generateUserProfileBlogTitle()
	content, _ := json.MarshalIndent(profile, "", "  ")
	ubd := &module.UploadedBlogData{Title: title, Content: string(content), Tags: "exercise-profile", AuthType: module.EAuthType_private}
	if blog.GetBlogWithAccount(acc, title) == nil {
		blog.AddBlogWithAccount(acc, ubd)
	} else {
		blog.ModifyBlogWithAccount(acc, ubd)
	}
	return nil
}

func GetMETValues(acc string) ([]METValue, error) {
	exerciseMu.RLock()
	defer exerciseMu.RUnlock()

	b := blog.GetBlogWithAccount(acc, generateMETValuesBlogTitle())
	if b == nil {
		return getDefaultMETValues(), nil
	}
	var metValues []METValue
	if err := json.Unmarshal([]byte(b.Content), &metValues); err != nil {
		return getDefaultMETValues(), nil
	}
	return metValues, nil
}

func UpdateAllTemplateCalories(acc string, weight float64) error {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	if weight <= 0 && acc != "" {
		if accountInfo, err := account.GetAccountInfo(acc); err == nil && accountInfo != nil {
			weight = accountInfo.Weight
		}
	}

	templates, _ := getTemplatesInternal(acc)
	updated := false
	for i := range templates {
		newCalories := calculateCaloriesInternal(templates[i].Type, templates[i].Intensity, templates[i].Duration, weight+templates[i].Weight)
		if newCalories != templates[i].Calories {
			templates[i].Calories = newCalories
			updated = true
		}
	}
	if updated {
		return saveTemplatesToBlog(acc, templates)
	}
	return nil
}

func UpdateAllExerciseCalories(acc string, weight float64) (int, error) {
	exerciseMu.Lock()
	defer exerciseMu.Unlock()

	if weight <= 0 && acc != "" {
		if accountInfo, err := account.GetAccountInfo(acc); err == nil && accountInfo != nil {
			weight = accountInfo.Weight
		}
	}

	allExercises, _ := getAllExercisesInternal(acc)
	updatedCount := 0
	for date, exerciseList := range allExercises {
		updated := false
		for i := range exerciseList.Items {
			newCalories := calculateCaloriesInternal(exerciseList.Items[i].Type, exerciseList.Items[i].Intensity, exerciseList.Items[i].Duration, weight)
			if newCalories != exerciseList.Items[i].Calories {
				exerciseList.Items[i].Calories = newCalories
				updated = true
				updatedCount++
			}
		}
		if updated {
			saveExercisesToBlog(acc, exerciseList)
		}
		_ = date
	}
	return updatedCount, nil
}

func ParseExerciseFromBlog(content string) ExerciseList {
	var exerciseList ExerciseList
	if err := json.Unmarshal([]byte(content), &exerciseList); err != nil {
		return ExerciseList{Items: []ExerciseItem{}}
	}
	return exerciseList
}
