package exercise

import (
	"blog"
	"core"
	"encoding/json"
	"fmt"
	"module"
	"strings"
	"time"
)

/*
goroutine 线程安全
 goroutine 会被调度到任意一个线程上，因此会被任意一个线程执行接口
 线程安全原因
 原因1: 	actor使用chan通信，chan是线程安全的
 原因2: 	actor的mailbox是线程安全的

 添加一个功能需要的四个步骤:
  第一步: 实现功能逻辑
  第二步: 实现对应的cmd
  第三步: 在exercise.go中添加对应的接口
  第四步: 在http中添加对应的接口
*/

// actor
type ExerciseActor struct {
	*core.Actor
}

// generateBlogTitle generates a blog title for a specific date's exercise list
func (ea *ExerciseActor) generateBlogTitle(date string) string {
	return fmt.Sprintf("exercise-%s", date)
}

// generateTemplateBlogTitle generates a blog title for exercise templates
func (ea *ExerciseActor) generateTemplateBlogTitle() string {
	return "exercise-templates"
}

// generateCollectionBlogTitle generates a blog title for exercise template collections
func (ea *ExerciseActor) generateCollectionBlogTitle() string {
	return "exercise-template-collections"
}

// generateUserProfileBlogTitle generates a blog title for user profile
func (ea *ExerciseActor) generateUserProfileBlogTitle() string {
	return "exercise-user-profile"
}

// generateMETValuesBlogTitle generates a blog title for MET values
func (ea *ExerciseActor) generateMETValuesBlogTitle() string {
	return "exercise-met-values"
}

// getDateFromTitle extracts the date from an exercise blog title
func (ea *ExerciseActor) getDateFromTitle(title string) string {
	if strings.HasPrefix(title, "exercise-") && title != "exercise-templates" {
		return strings.TrimPrefix(title, "exercise-")
	}
	return ""
}

// AddExercise adds a new exercise item to a specific date's list
func (ea *ExerciseActor) addExercise(date, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) (*ExerciseItem, error) {
	// Get or create exercise list for the date
	exerciseList, err := ea.getExercisesByDate(date)
	if err != nil {
		exerciseList = ExerciseList{
			Date:  date,
			Items: []ExerciseItem{},
		}
	}

	// Auto-calculate calories if not provided or is 0
	if calories == 0 {
		profile, _ := ea.getUserProfile()
		if profile != nil && profile.Weight > 0 {
			totalWeight := profile.Weight + weight // Add exercise weight to body weight
			calories = ea.calculateCalories(exerciseType, intensity, duration, totalWeight)
		}
	}

	// Create new exercise item
	item := ExerciseItem{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:      name,
		Type:      exerciseType,
		Duration:  duration,
		Intensity: intensity,
		Calories:  calories,
		Notes:     notes,
		Completed: false,
		Weight:    weight,
		CreatedAt: time.Now(),
		BodyParts: bodyParts,
	}

	// Add item to list
	exerciseList.Items = append(exerciseList.Items, item)

	// Save to blog
	if err := ea.saveExercisesToBlog(exerciseList); err != nil {
		return nil, err
	}

	return &item, nil
}

// DeleteExercise removes an exercise item by ID
func (ea *ExerciseActor) deleteExercise(date, id string) error {
	// Get exercise list for the date
	exerciseList, err := ea.getExercisesByDate(date)
	if err != nil {
		return err
	}

	// Find and remove the item
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

	// Update list
	exerciseList.Items = updatedItems

	// Save to blog
	return ea.saveExercisesToBlog(exerciseList)
}

// UpdateExercise updates an existing exercise item
func (ea *ExerciseActor) updateExercise(date, id, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) error {
	// Get exercise list for the date
	exerciseList, err := ea.getExercisesByDate(date)
	if err != nil {
		return err
	}

	// Auto-calculate calories if not provided or is 0
	if calories == 0 {
		profile, _ := ea.getUserProfile()
		if profile != nil && profile.Weight > 0 {
			totalWeight := profile.Weight + weight // Add exercise weight to body weight
			calories = ea.calculateCalories(exerciseType, intensity, duration, totalWeight)
		}
	}

	// Find and update the item
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

	// Save to blog
	return ea.saveExercisesToBlog(exerciseList)
}

// ToggleExercise toggles the completion status of an exercise item
func (ea *ExerciseActor) toggleExercise(date, id string) error {
	// Get exercise list for the date
	exerciseList, err := ea.getExercisesByDate(date)
	if err != nil {
		return err
	}

	// Find and toggle the item
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

	// Save to blog
	return ea.saveExercisesToBlog(exerciseList)
}

// GetExercisesByDate retrieves the exercise list for a specific date
func (ea *ExerciseActor) getExercisesByDate(date string) (ExerciseList, error) {
	title := ea.generateBlogTitle(date)

	// Find blog by title
	b := blog.GetBlog(title)
	if b == nil {
		return ExerciseList{Date: date, Items: []ExerciseItem{}}, nil
	}

	// Parse content as JSON
	var exerciseList ExerciseList
	if err := json.Unmarshal([]byte(b.Content), &exerciseList); err != nil {
		return ExerciseList{Date: date, Items: []ExerciseItem{}}, fmt.Errorf("failed to parse exercise list: %w", err)
	}

	return exerciseList, nil
}

// GetAllExercises retrieves all exercise lists from the blog system
func (ea *ExerciseActor) getAllExercises() (map[string]ExerciseList, error) {
	result := make(map[string]ExerciseList)

	// Iterate through all blogs
	for _, b := range blog.Blogs {
		date := ea.getDateFromTitle(b.Title)
		if date != "" {
			var exerciseList ExerciseList
			if err := json.Unmarshal([]byte(b.Content), &exerciseList); err == nil {
				result[date] = exerciseList
			}
		}
	}

	return result, nil
}

// saveExercisesToBlog saves an ExerciseList as a blog post
func (ea *ExerciseActor) saveExercisesToBlog(exerciseList ExerciseList) error {
	title := ea.generateBlogTitle(exerciseList.Date)

	// Convert to JSON
	content, err := json.MarshalIndent(exerciseList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert exercise list to JSON: %w", err)
	}

	// Find existing blog or create new one
	b := blog.GetBlog(title)
	if b == nil {
		// Create new blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise",
			AuthType: module.EAuthType_private,
		}
		blog.AddBlog(ubd)
	} else {
		// Update existing blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise",
			AuthType: module.EAuthType_private,
		}
		blog.ModifyBlog(ubd)
	}

	return nil
}

// AddTemplate adds a new exercise template
func (ea *ExerciseActor) addTemplate(name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) (*ExerciseTemplate, error) {
	templates, err := ea.getTemplates()
	if err != nil {
		templates = []ExerciseTemplate{}
	}

	// Create new template
	template := ExerciseTemplate{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:      name,
		Type:      exerciseType,
		Duration:  duration,
		Intensity: intensity,
		Calories:  calories,
		Notes:     notes,
		Weight:    weight,
		BodyParts: bodyParts,
	}

	// Add template to list
	templates = append(templates, template)

	// Save templates
	if err := ea.saveTemplatesToBlog(templates); err != nil {
		return nil, err
	}

	return &template, nil
}

// DeleteTemplate removes a template by ID
func (ea *ExerciseActor) deleteTemplate(id string) error {
	templates, err := ea.getTemplates()
	if err != nil {
		return err
	}

	// Find and remove the template
	found := false
	updatedTemplates := make([]ExerciseTemplate, 0, len(templates))
	for _, template := range templates {
		if template.ID != id {
			updatedTemplates = append(updatedTemplates, template)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("template not found")
	}

	// Save updated templates
	return ea.saveTemplatesToBlog(updatedTemplates)
}

// UpdateTemplate updates an existing template
func (ea *ExerciseActor) updateTemplate(id, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64, bodyParts []string) error {
	templates, err := ea.getTemplates()
	if err != nil {
		return err
	}

	// Find and update the template
	found := false
	for i := range templates {
		if templates[i].ID == id {
			templates[i].Name = name
			templates[i].Type = exerciseType
			templates[i].Duration = duration
			templates[i].Intensity = intensity
			templates[i].Calories = calories
			templates[i].Notes = notes
			templates[i].Weight = weight
			templates[i].BodyParts = bodyParts
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("template not found")
	}

	// Save updated templates
	return ea.saveTemplatesToBlog(templates)
}

// GetTemplates retrieves all exercise templates
func (ea *ExerciseActor) getTemplates() ([]ExerciseTemplate, error) {
	title := ea.generateTemplateBlogTitle()

	// Find blog by title
	b := blog.GetBlog(title)
	if b == nil {
		return []ExerciseTemplate{}, nil
	}

	// Parse content as JSON
	var templates []ExerciseTemplate
	if err := json.Unmarshal([]byte(b.Content), &templates); err != nil {
		return []ExerciseTemplate{}, fmt.Errorf("failed to parse templates: %w", err)
	}

	return templates, nil
}

// saveTemplatesToBlog saves exercise templates as a blog post
func (ea *ExerciseActor) saveTemplatesToBlog(templates []ExerciseTemplate) error {
	title := ea.generateTemplateBlogTitle()

	// Convert to JSON
	content, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert templates to JSON: %w", err)
	}

	// Find existing blog or create new one
	b := blog.GetBlog(title)
	if b == nil {
		// Create new blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise-template",
			AuthType: module.EAuthType_private,
		}
		blog.AddBlog(ubd)
	} else {
		// Update existing blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise-template",
			AuthType: module.EAuthType_private,
		}
		blog.ModifyBlog(ubd)
	}

	return nil
}

// calculateStats calculates exercise statistics for a given period
func (ea *ExerciseActor) calculateStats(period, startDate, endDate string) (*ExerciseStats, error) {
	allExercises, err := ea.getAllExercises()
	if err != nil {
		return nil, err
	}

	stats := &ExerciseStats{
		Period:    period,
		StartDate: startDate,
		EndDate:   endDate,
		TypeStats: make(map[string]int),
	}

	// Parse dates for comparison
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)

	// Calculate total days
	stats.TotalDays = int(end.Sub(start).Hours()/24) + 1

	exerciseDaysSet := make(map[string]bool)

	// Iterate through all exercise data
	for date, exerciseList := range allExercises {
		exerciseDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			continue
		}

		// Check if date is in range
		if exerciseDate.Before(start) || exerciseDate.After(end) {
			continue
		}

		hasCompletedExercise := false
		for _, item := range exerciseList.Items {
			if item.Completed {
				hasCompletedExercise = true
				stats.TotalDuration += item.Duration
				stats.TotalCalories += item.Calories
				stats.TypeStats[item.Type]++
			}
		}

		if hasCompletedExercise {
			exerciseDaysSet[date] = true
		}
	}

	stats.ExerciseDays = len(exerciseDaysSet)

	// Calculate consistency (exercise days / total days)
	if stats.TotalDays > 0 {
		stats.Consistency = float64(stats.ExerciseDays) / float64(stats.TotalDays) * 100
	}

	// Calculate weekly average
	if period == "week" {
		stats.WeeklyAvg = float64(stats.TotalDuration)
	} else if period == "month" {
		weeks := float64(stats.TotalDays) / 7.0
		if weeks > 0 {
			stats.WeeklyAvg = float64(stats.TotalDuration) / weeks
		}
	} else if period == "year" {
		stats.WeeklyAvg = float64(stats.TotalDuration) / 52.0
	}

	return stats, nil
}

// GetWeeklyStats calculates weekly exercise statistics
func (ea *ExerciseActor) getWeeklyStats(startDate string) (*ExerciseStats, error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}

	// Calculate week range (Monday to Sunday)
	weekday := start.Weekday()
	if weekday == 0 { // Sunday
		weekday = 7
	}
	monday := start.AddDate(0, 0, -int(weekday-1))
	sunday := monday.AddDate(0, 0, 6)

	return ea.calculateStats("week", monday.Format("2006-01-02"), sunday.Format("2006-01-02"))
}

// GetMonthlyStats calculates monthly exercise statistics
func (ea *ExerciseActor) getMonthlyStats(year int, month int) (*ExerciseStats, error) {
	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid date: %w", err)
	}

	// Last day of month
	end := start.AddDate(0, 1, -1)

	return ea.calculateStats("month", start.Format("2006-01-02"), end.Format("2006-01-02"))
}

// GetYearlyStats calculates yearly exercise statistics
func (ea *ExerciseActor) getYearlyStats(year int) (*ExerciseStats, error) {
	startDate := fmt.Sprintf("%04d-01-01", year)
	endDate := fmt.Sprintf("%04d-12-31", year)

	return ea.calculateStats("year", startDate, endDate)
}

// AddCollection adds a new template collection
func (ea *ExerciseActor) addCollection(name, description string, templateIDs []string) (*ExerciseTemplateCollection, error) {
	collections, err := ea.getCollections()
	if err != nil {
		collections = []ExerciseTemplateCollection{}
	}

	// Create new collection
	collection := ExerciseTemplateCollection{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		TemplateIDs: templateIDs,
		CreatedAt:   time.Now(),
	}

	// Add collection to list
	collections = append(collections, collection)

	// Save collections
	if err := ea.saveCollectionsToBlog(collections); err != nil {
		return nil, err
	}

	return &collection, nil
}

// DeleteCollection removes a collection by ID
func (ea *ExerciseActor) deleteCollection(id string) error {
	collections, err := ea.getCollections()
	if err != nil {
		return err
	}

	// Find and remove the collection
	found := false
	updatedCollections := make([]ExerciseTemplateCollection, 0, len(collections))
	for _, collection := range collections {
		if collection.ID != id {
			updatedCollections = append(updatedCollections, collection)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("collection not found")
	}

	// Save updated collections
	return ea.saveCollectionsToBlog(updatedCollections)
}

// UpdateCollection updates an existing collection
func (ea *ExerciseActor) updateCollection(id, name, description string, templateIDs []string) error {
	collections, err := ea.getCollections()
	if err != nil {
		return err
	}

	// Find and update the collection
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

	// Save updated collections
	return ea.saveCollectionsToBlog(collections)
}

// GetCollections retrieves all template collections
func (ea *ExerciseActor) getCollections() ([]ExerciseTemplateCollection, error) {
	title := ea.generateCollectionBlogTitle()

	// Find blog by title
	b := blog.GetBlog(title)
	if b == nil {
		return []ExerciseTemplateCollection{}, nil
	}

	// Parse content as JSON
	var collections []ExerciseTemplateCollection
	if err := json.Unmarshal([]byte(b.Content), &collections); err != nil {
		return []ExerciseTemplateCollection{}, fmt.Errorf("failed to parse collections: %w", err)
	}

	return collections, nil
}

// GetCollectionWithTemplates retrieves a collection with its associated templates
func (ea *ExerciseActor) getCollectionWithTemplates(collectionID string) (*ExerciseTemplateCollection, []ExerciseTemplate, error) {
	collections, err := ea.getCollections()
	if err != nil {
		return nil, nil, err
	}

	// Find the collection
	var targetCollection *ExerciseTemplateCollection
	for _, collection := range collections {
		if collection.ID == collectionID {
			targetCollection = &collection
			break
		}
	}

	if targetCollection == nil {
		return nil, nil, fmt.Errorf("collection not found")
	}

	// Get all templates
	allTemplates, err := ea.getTemplates()
	if err != nil {
		return targetCollection, []ExerciseTemplate{}, nil
	}

	// Filter templates that belong to this collection
	var collectionTemplates []ExerciseTemplate
	for _, templateID := range targetCollection.TemplateIDs {
		for _, template := range allTemplates {
			if template.ID == templateID {
				collectionTemplates = append(collectionTemplates, template)
				break
			}
		}
	}

	return targetCollection, collectionTemplates, nil
}

// AddFromCollection adds all exercises from a collection to a specific date
func (ea *ExerciseActor) addFromCollection(date, collectionID string) error {
	_, templates, err := ea.getCollectionWithTemplates(collectionID)
	if err != nil {
		return err
	}
	// Add each template as an exercise
	for _, template := range templates {
		exerciseList, err := ea.getExercisesByDate(date)
		if err != nil {
			exerciseList = ExerciseList{
				Date:  date,
				Items: []ExerciseItem{},
			}
		}
		calories := template.Calories
		if calories == 0 {
			profile, _ := ea.getUserProfile()
			if profile != nil && profile.Weight > 0 {
				totalWeight := profile.Weight + template.Weight
				calories = ea.calculateCalories(template.Type, template.Intensity, template.Duration, totalWeight)
			}
		}
		item := ExerciseItem{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			Name:      template.Name,
			Type:      template.Type,
			Duration:  template.Duration,
			Intensity: template.Intensity,
			Calories:  calories,
			Notes:     template.Notes,
			Completed: false,
			Weight:    template.Weight,
			CreatedAt: time.Now(),
			BodyParts: template.BodyParts,
		}
		exerciseList.Items = append(exerciseList.Items, item)
		if err := ea.saveExercisesToBlog(exerciseList); err != nil {
			return err
		}
	}
	return nil
}

// saveCollectionsToBlog saves template collections as a blog post
func (ea *ExerciseActor) saveCollectionsToBlog(collections []ExerciseTemplateCollection) error {
	title := ea.generateCollectionBlogTitle()

	// Convert to JSON
	content, err := json.MarshalIndent(collections, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert collections to JSON: %w", err)
	}

	// Find existing blog or create new one
	b := blog.GetBlog(title)
	if b == nil {
		// Create new blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise-collection",
			AuthType: module.EAuthType_private,
		}
		blog.AddBlog(ubd)
	} else {
		// Update existing blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise-collection",
			AuthType: module.EAuthType_private,
		}
		blog.ModifyBlog(ubd)
	}

	return nil
}

// CalculateCalories calculates calories burned using MET formula
// Formula: Calories (kcal) = MET × Weight (kg) × Time (hours)
func (ea *ExerciseActor) calculateCalories(exerciseType, intensity string, duration int, weight float64) int {
	met := ea.getMETValue(exerciseType, intensity)
	hours := float64(duration) / 60.0 // Convert minutes to hours
	calories := met * weight * hours
	return int(calories)
}

// getMETValue returns the MET value for given exercise type and intensity
func (ea *ExerciseActor) getMETValue(exerciseType, intensity string) float64 {
	metValues := ea.getDefaultMETValues()

	for _, mv := range metValues {
		if mv.ExerciseType == exerciseType && mv.Intensity == intensity {
			return mv.MET
		}
	}

	// Default MET values if not found
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

// GetMETValueWithDescription returns the MET value and description for given exercise type and intensity
func (ea *ExerciseActor) getMETValueWithDescription(exerciseType, intensity string) (float64, string) {
	metValues := ea.getDefaultMETValues()

	for _, mv := range metValues {
		if mv.ExerciseType == exerciseType && mv.Intensity == intensity {
			return mv.MET, mv.Description
		}
	}

	// Default values if not found
	var description string
	var met float64

	switch intensity {
	case "low":
		met = 3.0
		description = "低强度活动"
	case "medium":
		met = 5.0
		description = "中等强度活动"
	case "high":
		met = 8.0
		description = "高强度活动"
	default:
		met = 4.0
		description = "一般强度活动"
	}

	return met, description
}

// getDefaultMETValues returns default MET values for common exercises
func (ea *ExerciseActor) getDefaultMETValues() []METValue {
	return []METValue{
		// Cardio exercises
		{"cardio", "low", 3.5, "慢走"},
		{"cardio", "medium", 6.0, "慢跑"},
		{"cardio", "high", 10.0, "快跑"},

		// Strength training
		{"strength", "low", 3.0, "轻度力量训练"},
		{"strength", "medium", 5.0, "中等力量训练"},
		{"strength", "high", 8.0, "高强度力量训练"},

		// Flexibility
		{"flexibility", "low", 2.5, "拉伸"},
		{"flexibility", "medium", 3.0, "瑜伽"},
		{"flexibility", "high", 4.0, "高强度瑜伽"},

		// Sports
		{"sports", "low", 4.0, "休闲运动"},
		{"sports", "medium", 7.0, "中等强度运动"},
		{"sports", "high", 10.0, "激烈运动"},

		// Other
		{"other", "low", 2.5, "其他轻度活动"},
		{"other", "medium", 4.0, "其他中等活动"},
		{"other", "high", 6.0, "其他高强度活动"},
	}
}

// SaveUserProfile saves or updates user profile
func (ea *ExerciseActor) saveUserProfile(name, gender string, weight, height float64, age int) (*UserProfile, error) {
	profile := &UserProfile{
		ID:        "default", // Single user profile
		Name:      name,
		Gender:    gender,
		Weight:    weight,
		Height:    height,
		Age:       age,
		UpdatedAt: time.Now(),
	}

	// Check if profile exists
	existingProfile, _ := ea.getUserProfile()
	if existingProfile == nil {
		profile.CreatedAt = time.Now()
	} else {
		profile.CreatedAt = existingProfile.CreatedAt
	}

	if err := ea.saveUserProfileToBlog(profile); err != nil {
		return nil, err
	}

	return profile, nil
}

// GetUserProfile retrieves user profile
func (ea *ExerciseActor) getUserProfile() (*UserProfile, error) {
	title := ea.generateUserProfileBlogTitle()

	// Find blog by title
	b := blog.GetBlog(title)
	if b == nil {
		return nil, nil // No profile found
	}

	// Parse content as JSON
	var profile UserProfile
	if err := json.Unmarshal([]byte(b.Content), &profile); err != nil {
		return nil, fmt.Errorf("failed to parse user profile: %w", err)
	}

	return &profile, nil
}

// saveUserProfileToBlog saves user profile as a blog post
func (ea *ExerciseActor) saveUserProfileToBlog(profile *UserProfile) error {
	title := ea.generateUserProfileBlogTitle()

	// Convert to JSON
	content, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert user profile to JSON: %w", err)
	}

	// Find existing blog or create new one
	b := blog.GetBlog(title)
	if b == nil {
		// Create new blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise-profile",
			AuthType: module.EAuthType_private,
		}
		blog.AddBlog(ubd)
	} else {
		// Update existing blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise-profile",
			AuthType: module.EAuthType_private,
		}
		blog.ModifyBlog(ubd)
	}

	return nil
}

// GetMETValues retrieves MET values
func (ea *ExerciseActor) getMETValues() ([]METValue, error) {
	title := ea.generateMETValuesBlogTitle()

	// Find blog by title
	b := blog.GetBlog(title)
	if b == nil {
		// Return default values and save them
		defaultValues := ea.getDefaultMETValues()
		ea.saveMETValuesToBlog(defaultValues)
		return defaultValues, nil
	}

	// Parse content as JSON
	var metValues []METValue
	if err := json.Unmarshal([]byte(b.Content), &metValues); err != nil {
		return ea.getDefaultMETValues(), nil
	}

	return metValues, nil
}

// saveMETValuesToBlog saves MET values as a blog post
func (ea *ExerciseActor) saveMETValuesToBlog(metValues []METValue) error {
	title := ea.generateMETValuesBlogTitle()

	// Convert to JSON
	content, err := json.MarshalIndent(metValues, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to convert MET values to JSON: %w", err)
	}

	// Find existing blog or create new one
	b := blog.GetBlog(title)
	if b == nil {
		// Create new blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise-met",
			AuthType: module.EAuthType_private,
		}
		blog.AddBlog(ubd)
	} else {
		// Update existing blog using UploadedBlogData
		ubd := &module.UploadedBlogData{
			Title:    title,
			Content:  string(content),
			Tags:     "exercise-met",
			AuthType: module.EAuthType_private,
		}
		blog.ModifyBlog(ubd)
	}

	return nil
}

// UpdateAllTemplateCalories updates calories for all existing templates based on current MET values
func (ea *ExerciseActor) updateAllTemplateCalories(weight float64) error {
	templates, err := ea.getTemplates()
	if err != nil {
		return err
	}

	updated := false
	for i := range templates {
		// Calculate new calories for each template, considering template weight (负重)
		totalWeight := weight + templates[i].Weight
		newCalories := ea.calculateCalories(templates[i].Type, templates[i].Intensity, templates[i].Duration, totalWeight)
		if newCalories != templates[i].Calories {
			templates[i].Calories = newCalories
			updated = true
		}
	}

	// Save updated templates if any changes were made
	if updated {
		return ea.saveTemplatesToBlog(templates)
	}

	return nil
}

// UpdateAllExerciseCalories updates calories for all existing exercise records based on current MET values
func (ea *ExerciseActor) updateAllExerciseCalories(weight float64) (int, error) {
	allExercises, err := ea.getAllExercises()
	if err != nil {
		return 0, err
	}

	updatedCount := 0

	for date, exerciseList := range allExercises {
		updated := false
		for i := range exerciseList.Items {
			// Calculate new calories for each exercise item
			newCalories := ea.calculateCalories(exerciseList.Items[i].Type, exerciseList.Items[i].Intensity, exerciseList.Items[i].Duration, weight)
			if newCalories != exerciseList.Items[i].Calories {
				exerciseList.Items[i].Calories = newCalories
				updated = true
				updatedCount++
			}
		}

		// Save updated exercise list if any changes were made
		if updated {
			if err := ea.saveExercisesToBlog(exerciseList); err != nil {
				return updatedCount, fmt.Errorf("failed to save exercises for date %s: %w", date, err)
			}
		}
	}

	return updatedCount, nil
}