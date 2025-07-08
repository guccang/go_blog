package exercise

import (
    "encoding/json"
    "fmt"
    "strings"
    "time"
    "blog"
    "module"
)

// ExerciseItem represents a single exercise entry
type ExerciseItem struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`        // 锻炼项目名称
    Type        string    `json:"type"`        // 锻炼类型：cardio, strength, flexibility, sports
    Duration    int       `json:"duration"`    // 持续时间（分钟）
    Intensity   string    `json:"intensity"`   // 强度：low, medium, high
    Calories    int       `json:"calories"`    // 消耗卡路里
    Notes       string    `json:"notes"`       // 备注
    Completed   bool      `json:"completed"`   // 是否完成
    Weight      float64   `json:"weight"`      // 负重 (kg)
    CreatedAt   time.Time `json:"created_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ExerciseTemplate represents an exercise template
type ExerciseTemplate struct {
    ID        string  `json:"id"`
    Name      string  `json:"name"`
    Type      string  `json:"type"`
    Duration  int     `json:"duration"`
    Intensity string  `json:"intensity"`
    Calories  int     `json:"calories"`
    Notes     string  `json:"notes"`
    Weight    float64 `json:"weight"` // 负重 (kg)
}

// ExerciseTemplateCollection represents a collection of exercise templates
type ExerciseTemplateCollection struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    TemplateIDs []string `json:"template_ids"`
    CreatedAt   time.Time `json:"created_at"`
}

// UserProfile represents user's basic information for exercise calculation
type UserProfile struct {
    ID        string  `json:"id"`
    Name      string  `json:"name"`
    Gender    string  `json:"gender"`    // male, female
    Weight    float64 `json:"weight"`    // kg
    Height    float64 `json:"height"`    // cm
    Age       int     `json:"age"`
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
    Date      string         `json:"date"`
    Items     []ExerciseItem `json:"items"`
    Templates []ExerciseTemplate `json:"templates"`
}

// ExerciseStats represents exercise statistics
type ExerciseStats struct {
    Period        string            `json:"period"`        // week, month, year
    StartDate     string            `json:"start_date"`
    EndDate       string            `json:"end_date"`
    TotalDays     int               `json:"total_days"`
    ExerciseDays  int               `json:"exercise_days"`
    TotalDuration int               `json:"total_duration"` // 总锻炼时间（分钟）
    TotalCalories int               `json:"total_calories"` // 总消耗卡路里
    TypeStats     map[string]int    `json:"type_stats"`     // 各类型锻炼次数
    WeeklyAvg     float64           `json:"weekly_avg"`     // 周平均锻炼时间
    Consistency   float64           `json:"consistency"`    // 坚持率（锻炼天数/总天数）
}

// ExerciseManager handles exercise operations
type ExerciseManager struct{}

// NewExerciseManager creates a new ExerciseManager instance
func NewExerciseManager() *ExerciseManager {
    return &ExerciseManager{}
}

// generateBlogTitle generates a blog title for a specific date's exercise list
func generateBlogTitle(date string) string {
    return fmt.Sprintf("exercise-%s", date)
}

// generateTemplateBlogTitle generates a blog title for exercise templates
func generateTemplateBlogTitle() string {
    return "exercise-templates"
}

// generateCollectionBlogTitle generates a blog title for exercise template collections
func generateCollectionBlogTitle() string {
    return "exercise-template-collections"
}

// generateUserProfileBlogTitle generates a blog title for user profile
func generateUserProfileBlogTitle() string {
    return "exercise-user-profile"
}

// generateMETValuesBlogTitle generates a blog title for MET values
func generateMETValuesBlogTitle() string {
    return "exercise-met-values"
}

// getDateFromTitle extracts the date from an exercise blog title
func getDateFromTitle(title string) string {
    if strings.HasPrefix(title, "exercise-") && title != "exercise-templates" {
        return strings.TrimPrefix(title, "exercise-")
    }
    return ""
}

// AddExercise adds a new exercise item to a specific date's list
func (em *ExerciseManager) AddExercise(date, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64) (*ExerciseItem, error) {
    // Get or create exercise list for the date
    exerciseList, err := em.GetExercisesByDate(date)
    if err != nil {
        exerciseList = ExerciseList{
            Date:  date,
            Items: []ExerciseItem{},
        }
    }
    
    // Auto-calculate calories if not provided or is 0
    if calories == 0 {
        profile, _ := em.GetUserProfile()
        if profile != nil && profile.Weight > 0 {
            totalWeight := profile.Weight + weight // Add exercise weight to body weight
            calories = em.CalculateCalories(exerciseType, intensity, duration, totalWeight)
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
    }
    
    // Add item to list
    exerciseList.Items = append(exerciseList.Items, item)
    
    // Save to blog
    if err := em.saveExercisesToBlog(exerciseList); err != nil {
        return nil, err
    }
    
    return &item, nil
}

// DeleteExercise removes an exercise item by ID
func (em *ExerciseManager) DeleteExercise(date, id string) error {
    // Get exercise list for the date
    exerciseList, err := em.GetExercisesByDate(date)
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
    return em.saveExercisesToBlog(exerciseList)
}

// UpdateExercise updates an existing exercise item
func (em *ExerciseManager) UpdateExercise(date, id, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64) error {
    // Get exercise list for the date
    exerciseList, err := em.GetExercisesByDate(date)
    if err != nil {
        return err
    }
    
    // Auto-calculate calories if not provided or is 0
    if calories == 0 {
        profile, _ := em.GetUserProfile()
        if profile != nil && profile.Weight > 0 {
            totalWeight := profile.Weight + weight // Add exercise weight to body weight
            calories = em.CalculateCalories(exerciseType, intensity, duration, totalWeight)
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
            found = true
            break
        }
    }
    
    if !found {
        return fmt.Errorf("exercise item not found")
    }
    
    // Save to blog
    return em.saveExercisesToBlog(exerciseList)
}

// ToggleExercise toggles the completion status of an exercise item
func (em *ExerciseManager) ToggleExercise(date, id string) error {
    // Get exercise list for the date
    exerciseList, err := em.GetExercisesByDate(date)
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
    return em.saveExercisesToBlog(exerciseList)
}

// GetExercisesByDate retrieves the exercise list for a specific date
func (em *ExerciseManager) GetExercisesByDate(date string) (ExerciseList, error) {
    title := generateBlogTitle(date)
    
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
func (em *ExerciseManager) GetAllExercises() (map[string]ExerciseList, error) {
    result := make(map[string]ExerciseList)
    
    // Iterate through all blogs
    for _, b := range blog.Blogs {
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

// saveExercisesToBlog saves an ExerciseList as a blog post
func (em *ExerciseManager) saveExercisesToBlog(exerciseList ExerciseList) error {
    title := generateBlogTitle(exerciseList.Date)
    
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
func (em *ExerciseManager) AddTemplate(name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64) (*ExerciseTemplate, error) {
    templates, err := em.GetTemplates()
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
    }
    
    // Add template to list
    templates = append(templates, template)
    
    // Save templates
    if err := em.saveTemplatesToBlog(templates); err != nil {
        return nil, err
    }
    
    return &template, nil
}

// DeleteTemplate removes a template by ID
func (em *ExerciseManager) DeleteTemplate(id string) error {
    templates, err := em.GetTemplates()
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
    return em.saveTemplatesToBlog(updatedTemplates)
}

// UpdateTemplate updates an existing template
func (em *ExerciseManager) UpdateTemplate(id, name, exerciseType string, duration int, intensity string, calories int, notes string, weight float64) error {
    templates, err := em.GetTemplates()
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
            found = true
            break
        }
    }
    
    if !found {
        return fmt.Errorf("template not found")
    }
    
    // Save updated templates
    return em.saveTemplatesToBlog(templates)
}

// GetTemplates retrieves all exercise templates
func (em *ExerciseManager) GetTemplates() ([]ExerciseTemplate, error) {
    title := generateTemplateBlogTitle()
    
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
func (em *ExerciseManager) saveTemplatesToBlog(templates []ExerciseTemplate) error {
    title := generateTemplateBlogTitle()
    
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

// GetWeeklyStats calculates weekly exercise statistics
func (em *ExerciseManager) GetWeeklyStats(startDate string) (*ExerciseStats, error) {
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
    
    return em.calculateStats("week", monday.Format("2006-01-02"), sunday.Format("2006-01-02"))
}

// GetMonthlyStats calculates monthly exercise statistics
func (em *ExerciseManager) GetMonthlyStats(year int, month int) (*ExerciseStats, error) {
    startDate := fmt.Sprintf("%04d-%02d-01", year, month)
    start, err := time.Parse("2006-01-02", startDate)
    if err != nil {
        return nil, fmt.Errorf("invalid date: %w", err)
    }
    
    // Last day of month
    end := start.AddDate(0, 1, -1)
    
    return em.calculateStats("month", start.Format("2006-01-02"), end.Format("2006-01-02"))
}

// GetYearlyStats calculates yearly exercise statistics
func (em *ExerciseManager) GetYearlyStats(year int) (*ExerciseStats, error) {
    startDate := fmt.Sprintf("%04d-01-01", year)
    endDate := fmt.Sprintf("%04d-12-31", year)
    
    return em.calculateStats("year", startDate, endDate)
}

// calculateStats calculates exercise statistics for a given period
func (em *ExerciseManager) calculateStats(period, startDate, endDate string) (*ExerciseStats, error) {
    allExercises, err := em.GetAllExercises()
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

// AddCollection adds a new template collection
func (em *ExerciseManager) AddCollection(name, description string, templateIDs []string) (*ExerciseTemplateCollection, error) {
    collections, err := em.GetCollections()
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
    if err := em.saveCollectionsToBlog(collections); err != nil {
        return nil, err
    }
    
    return &collection, nil
}

// DeleteCollection removes a collection by ID
func (em *ExerciseManager) DeleteCollection(id string) error {
    collections, err := em.GetCollections()
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
    return em.saveCollectionsToBlog(updatedCollections)
}

// UpdateCollection updates an existing collection
func (em *ExerciseManager) UpdateCollection(id, name, description string, templateIDs []string) error {
    collections, err := em.GetCollections()
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
    return em.saveCollectionsToBlog(collections)
}

// GetCollections retrieves all template collections
func (em *ExerciseManager) GetCollections() ([]ExerciseTemplateCollection, error) {
    title := generateCollectionBlogTitle()
    
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
func (em *ExerciseManager) GetCollectionWithTemplates(collectionID string) (*ExerciseTemplateCollection, []ExerciseTemplate, error) {
    collections, err := em.GetCollections()
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
    allTemplates, err := em.GetTemplates()
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
func (em *ExerciseManager) AddFromCollection(date, collectionID string) error {
    _, templates, err := em.GetCollectionWithTemplates(collectionID)
    if err != nil {
        return err
    }
    
    // Add each template as an exercise
    for _, template := range templates {
        _, err := em.AddExercise(date, template.Name, template.Type, template.Duration, template.Intensity, template.Calories, template.Notes, template.Weight)
        if err != nil {
            return fmt.Errorf("failed to add exercise from template %s: %w", template.Name, err)
        }
    }
    
    return nil
}

// saveCollectionsToBlog saves template collections as a blog post
func (em *ExerciseManager) saveCollectionsToBlog(collections []ExerciseTemplateCollection) error {
    title := generateCollectionBlogTitle()
    
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
func (em *ExerciseManager) CalculateCalories(exerciseType, intensity string, duration int, weight float64) int {
    met := em.getMETValue(exerciseType, intensity)
    hours := float64(duration) / 60.0 // Convert minutes to hours
    calories := met * weight * hours
    return int(calories)
}

// getMETValue returns the MET value for given exercise type and intensity
func (em *ExerciseManager) getMETValue(exerciseType, intensity string) float64 {
    metValues := em.getDefaultMETValues()
    
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
func (em *ExerciseManager) GetMETValueWithDescription(exerciseType, intensity string) (float64, string) {
    metValues := em.getDefaultMETValues()
    
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
func (em *ExerciseManager) getDefaultMETValues() []METValue {
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
func (em *ExerciseManager) SaveUserProfile(name, gender string, weight, height float64, age int) (*UserProfile, error) {
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
    existingProfile, _ := em.GetUserProfile()
    if existingProfile == nil {
        profile.CreatedAt = time.Now()
    } else {
        profile.CreatedAt = existingProfile.CreatedAt
    }
    
    if err := em.saveUserProfileToBlog(profile); err != nil {
        return nil, err
    }
    
    return profile, nil
}

// GetUserProfile retrieves user profile
func (em *ExerciseManager) GetUserProfile() (*UserProfile, error) {
    title := generateUserProfileBlogTitle()
    
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
func (em *ExerciseManager) saveUserProfileToBlog(profile *UserProfile) error {
    title := generateUserProfileBlogTitle()
    
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
func (em *ExerciseManager) GetMETValues() ([]METValue, error) {
    title := generateMETValuesBlogTitle()
    
    // Find blog by title
    b := blog.GetBlog(title)
    if b == nil {
        // Return default values and save them
        defaultValues := em.getDefaultMETValues()
        em.saveMETValuesToBlog(defaultValues)
        return defaultValues, nil
    }
    
    // Parse content as JSON
    var metValues []METValue
    if err := json.Unmarshal([]byte(b.Content), &metValues); err != nil {
        return em.getDefaultMETValues(), nil
    }
    
    return metValues, nil
}

// saveMETValuesToBlog saves MET values as a blog post
func (em *ExerciseManager) saveMETValuesToBlog(metValues []METValue) error {
    title := generateMETValuesBlogTitle()
    
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
func (em *ExerciseManager) UpdateAllTemplateCalories(weight float64) error {
    templates, err := em.GetTemplates()
    if err != nil {
        return err
    }
    
    updated := false
    for i := range templates {
        // Calculate new calories for each template, considering template weight (负重)
        totalWeight := weight + templates[i].Weight
        newCalories := em.CalculateCalories(templates[i].Type, templates[i].Intensity, templates[i].Duration, totalWeight)
        if newCalories != templates[i].Calories {
            templates[i].Calories = newCalories
            updated = true
        }
    }
    
    // Save updated templates if any changes were made
    if updated {
        return em.saveTemplatesToBlog(templates)
    }
    
    return nil
}

// UpdateAllExerciseCalories updates calories for all existing exercise records based on current MET values
func (em *ExerciseManager) UpdateAllExerciseCalories(weight float64) (int, error) {
    allExercises, err := em.GetAllExercises()
    if err != nil {
        return 0, err
    }
    
    updatedCount := 0
    
    for date, exerciseList := range allExercises {
        updated := false
        for i := range exerciseList.Items {
            // Calculate new calories for each exercise item
            newCalories := em.CalculateCalories(exerciseList.Items[i].Type, exerciseList.Items[i].Intensity, exerciseList.Items[i].Duration, weight)
            if newCalories != exerciseList.Items[i].Calories {
                exerciseList.Items[i].Calories = newCalories
                updated = true
                updatedCount++
            }
        }
        
        // Save updated exercise list if any changes were made
        if updated {
            if err := em.saveExercisesToBlog(exerciseList); err != nil {
                return updatedCount, fmt.Errorf("failed to save exercises for date %s: %w", date, err)
            }
        }
    }
    
    return updatedCount, nil
} 