package exercise

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "time"
    log "mylog"
)

var manager = NewExerciseManager()

// GetExerciseManager returns the global exercise manager instance
func GetExerciseManager() *ExerciseManager {
    return manager
}

// HandleExercises handles CRUD operations for exercises
func HandleExercises(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    switch r.Method {
    case http.MethodGet:
        handleGetExercises(w, r)
    case http.MethodPost:
        handleAddExercise(w, r)
    case http.MethodPut:
        handleUpdateExercise(w, r)
    case http.MethodDelete:
        handleDeleteExercise(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// HandleToggleExercise handles toggling exercise completion
func HandleToggleExercise(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    var req struct {
        Date string `json:"date"`
        ID   string `json:"id"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := manager.ToggleExercise(req.Date, req.ID); err != nil {
        log.ErrorF("Failed to toggle exercise: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleTemplates handles CRUD operations for exercise templates
func HandleTemplates(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    switch r.Method {
    case http.MethodGet:
        handleGetTemplates(w, r)
    case http.MethodPost:
        handleAddTemplate(w, r)
    case http.MethodPut:
        handleUpdateTemplate(w, r)
    case http.MethodDelete:
        handleDeleteTemplate(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// HandleExerciseStats handles exercise statistics requests
func HandleExerciseStats(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    period := r.URL.Query().Get("period")
    
    var stats *ExerciseStats
    var err error
    
    switch period {
    case "week":
        date := r.URL.Query().Get("date")
        if date == "" {
            date = time.Now().Format("2006-01-02")
        }
        stats, err = manager.GetWeeklyStats(date)
    case "month":
        yearStr := r.URL.Query().Get("year")
        monthStr := r.URL.Query().Get("month")
        
        year, err1 := strconv.Atoi(yearStr)
        month, err2 := strconv.Atoi(monthStr)
        
        if err1 != nil || err2 != nil {
            now := time.Now()
            year = now.Year()
            month = int(now.Month())
        }
        
        stats, err = manager.GetMonthlyStats(year, month)
    case "year":
        yearStr := r.URL.Query().Get("year")
        year, err1 := strconv.Atoi(yearStr)
        if err1 != nil {
            year = time.Now().Year()
        }
        
        stats, err = manager.GetYearlyStats(year)
    default:
        http.Error(w, "Invalid period. Use: week, month, or year", http.StatusBadRequest)
        return
    }
    
    if err != nil {
        log.ErrorF("Failed to get exercise stats: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(stats)
}

func handleGetExercises(w http.ResponseWriter, r *http.Request) {
    date := r.URL.Query().Get("date")
    if date == "" {
        date = time.Now().Format("2006-01-02")
    }
    
    exercises, err := manager.GetExercisesByDate(date)
    if err != nil {
        log.ErrorF("Failed to get exercises: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(exercises)
}

func handleAddExercise(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Date      string   `json:"date"`
        Name      string   `json:"name"`
        Type      string   `json:"type"`
        Duration  int      `json:"duration"`
        Intensity string   `json:"intensity"`
        Calories  int      `json:"calories"`
        Notes     string   `json:"notes"`
        Weight    float64  `json:"weight"`
        BodyParts []string `json:"body_parts"` // 新增
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if req.Date == "" {
        req.Date = time.Now().Format("2006-01-02")
    }
    
    // 新增：传递body_parts
    exerciseList, err := manager.GetExercisesByDate(req.Date)
    if err != nil {
        exerciseList = ExerciseList{
            Date:  req.Date,
            Items: []ExerciseItem{},
        }
    }
    // Auto-calculate calories if not provided or is 0
    calories := req.Calories
    if calories == 0 {
        profile, _ := manager.GetUserProfile()
        if profile != nil && profile.Weight > 0 {
            totalWeight := profile.Weight + req.Weight
            calories = manager.CalculateCalories(req.Type, req.Intensity, req.Duration, totalWeight)
        }
    }
    item := ExerciseItem{
        ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
        Name:      req.Name,
        Type:      req.Type,
        Duration:  req.Duration,
        Intensity: req.Intensity,
        Calories:  calories,
        Notes:     req.Notes,
        Completed: false,
        Weight:    req.Weight,
        CreatedAt: time.Now(),
        BodyParts: req.BodyParts, // 新增
    }
    exerciseList.Items = append(exerciseList.Items, item)
    if err := manager.saveExercisesToBlog(exerciseList); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(item)
}

func handleUpdateExercise(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Date      string   `json:"date"`
        ID        string   `json:"id"`
        Name      string   `json:"name"`
        Type      string   `json:"type"`
        Duration  int      `json:"duration"`
        Intensity string   `json:"intensity"`
        Calories  int      `json:"calories"`
        Notes     string   `json:"notes"`
        Weight    float64  `json:"weight"`
        BodyParts []string `json:"body_parts"` // 新增
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    exerciseList, err := manager.GetExercisesByDate(req.Date)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    // Auto-calculate calories if not provided or is 0
    calories := req.Calories
    if calories == 0 {
        profile, _ := manager.GetUserProfile()
        if profile != nil && profile.Weight > 0 {
            totalWeight := profile.Weight + req.Weight
            calories = manager.CalculateCalories(req.Type, req.Intensity, req.Duration, totalWeight)
        }
    }
    found := false
    for i := range exerciseList.Items {
        if exerciseList.Items[i].ID == req.ID {
            exerciseList.Items[i].Name = req.Name
            exerciseList.Items[i].Type = req.Type
            exerciseList.Items[i].Duration = req.Duration
            exerciseList.Items[i].Intensity = req.Intensity
            exerciseList.Items[i].Calories = calories
            exerciseList.Items[i].Notes = req.Notes
            exerciseList.Items[i].Weight = req.Weight
            exerciseList.Items[i].BodyParts = req.BodyParts // 新增
            found = true
            break
        }
    }
    if !found {
        http.Error(w, "exercise item not found", http.StatusNotFound)
        return
    }
    if err := manager.saveExercisesToBlog(exerciseList); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func handleDeleteExercise(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Date string `json:"date"`
        ID   string `json:"id"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := manager.DeleteExercise(req.Date, req.ID); err != nil {
        log.ErrorF("Failed to delete exercise: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func handleGetTemplates(w http.ResponseWriter, r *http.Request) {
    templates, err := manager.GetTemplates()
    if err != nil {
        log.ErrorF("Failed to get templates: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(templates)
}

func handleAddTemplate(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name      string   `json:"name"`
        Type      string   `json:"type"`
        Duration  int      `json:"duration"`
        Intensity string   `json:"intensity"`
        Calories  int      `json:"calories"`
        Notes     string   `json:"notes"`
        Weight    float64  `json:"weight"`
        BodyParts []string `json:"body_parts"` // 新增
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    // 新增：传递body_parts
    template := ExerciseTemplate{
        ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
        Name:      req.Name,
        Type:      req.Type,
        Duration:  req.Duration,
        Intensity: req.Intensity,
        Calories:  req.Calories,
        Notes:     req.Notes,
        Weight:    req.Weight,
        BodyParts: req.BodyParts,
    }
    
    templates, err := manager.GetTemplates()
    if err != nil {
        templates = []ExerciseTemplate{}
    }
    templates = append(templates, template)
    if err := manager.saveTemplatesToBlog(templates); err != nil {
        log.ErrorF("Failed to save templates: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(template)
}

func handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ID        string   `json:"id"`
        Name      string   `json:"name"`
        Type      string   `json:"type"`
        Duration  int      `json:"duration"`
        Intensity string   `json:"intensity"`
        Calories  int      `json:"calories"`
        Notes     string   `json:"notes"`
        Weight    float64  `json:"weight"`
        BodyParts []string `json:"body_parts"` // 新增
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    templates, err := manager.GetTemplates()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    found := false
    for i := range templates {
        if templates[i].ID == req.ID {
            templates[i].Name = req.Name
            templates[i].Type = req.Type
            templates[i].Duration = req.Duration
            templates[i].Intensity = req.Intensity
            templates[i].Calories = req.Calories
            templates[i].Notes = req.Notes
            templates[i].Weight = req.Weight
            templates[i].BodyParts = req.BodyParts // 新增
            found = true
            break
        }
    }
    if !found {
        http.Error(w, "template not found", http.StatusNotFound)
        return
    }
    if err := manager.saveTemplatesToBlog(templates); err != nil {
        log.ErrorF("Failed to save templates: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ID string `json:"id"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := manager.DeleteTemplate(req.ID); err != nil {
        log.ErrorF("Failed to delete template: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleCollections handles CRUD operations for template collections
func HandleCollections(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    switch r.Method {
    case http.MethodGet:
        handleGetCollections(w, r)
    case http.MethodPost:
        handleAddCollection(w, r)
    case http.MethodPut:
        handleUpdateCollection(w, r)
    case http.MethodDelete:
        handleDeleteCollection(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// HandleAddFromCollection handles adding exercises from a collection
func HandleAddFromCollection(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    var req struct {
        Date         string `json:"date"`
        CollectionID string `json:"collection_id"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := manager.AddFromCollection(req.Date, req.CollectionID); err != nil {
        log.ErrorF("Failed to add from collection: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleGetCollectionDetails handles getting collection with templates
func HandleGetCollectionDetails(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    collectionID := r.URL.Query().Get("id")
    if collectionID == "" {
        http.Error(w, "Collection ID is required", http.StatusBadRequest)
        return
    }
    
    collection, templates, err := manager.GetCollectionWithTemplates(collectionID)
    if err != nil {
        log.ErrorF("Failed to get collection details: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    response := map[string]interface{}{
        "collection": collection,
        "templates":  templates,
    }
    
    json.NewEncoder(w).Encode(response)
}

func handleGetCollections(w http.ResponseWriter, r *http.Request) {
    collections, err := manager.GetCollections()
    if err != nil {
        log.ErrorF("Failed to get collections: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(collections)
}

func handleAddCollection(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name        string   `json:"name"`
        Description string   `json:"description"`
        TemplateIDs []string `json:"template_ids"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    collection, err := manager.AddCollection(req.Name, req.Description, req.TemplateIDs)
    if err != nil {
        log.ErrorF("Failed to add collection: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(collection)
}

func handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ID          string   `json:"id"`
        Name        string   `json:"name"`
        Description string   `json:"description"`
        TemplateIDs []string `json:"template_ids"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := manager.UpdateCollection(req.ID, req.Name, req.Description, req.TemplateIDs); err != nil {
        log.ErrorF("Failed to update collection: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ID string `json:"id"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := manager.DeleteCollection(req.ID); err != nil {
        log.ErrorF("Failed to delete collection: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleUserProfile handles user profile operations
func HandleUserProfile(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    switch r.Method {
    case http.MethodGet:
        handleGetUserProfile(w, r)
    case http.MethodPost:
        handleSaveUserProfile(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// HandleCalculateCalories handles calorie calculation requests
func HandleCalculateCalories(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    var req struct {
        ExerciseType string  `json:"exercise_type"`
        Intensity    string  `json:"intensity"`
        Duration     int     `json:"duration"`
        Weight       float64 `json:"weight"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    calories := manager.CalculateCalories(req.ExerciseType, req.Intensity, req.Duration, req.Weight)
    
    response := map[string]interface{}{
        "calories": calories,
    }
    
    json.NewEncoder(w).Encode(response)
}

// HandleMETValues handles MET values requests
func HandleMETValues(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    metValues, err := manager.GetMETValues()
    if err != nil {
        log.ErrorF("Failed to get MET values: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(metValues)
}

func handleGetUserProfile(w http.ResponseWriter, r *http.Request) {
    profile, err := manager.GetUserProfile()
    if err != nil {
        log.ErrorF("Failed to get user profile: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    if profile == nil {
        // Return empty profile
        emptyProfile := UserProfile{}
        profile = &emptyProfile
    }
    
    json.NewEncoder(w).Encode(profile)
}

func handleSaveUserProfile(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name   string  `json:"name"`
        Gender string  `json:"gender"`
        Weight float64 `json:"weight"`
        Height float64 `json:"height"`
        Age    int     `json:"age"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    profile, err := manager.SaveUserProfile(req.Name, req.Gender, req.Weight, req.Height, req.Age)
    if err != nil {
        log.ErrorF("Failed to save user profile: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(profile)
}

// HandleUpdateTemplateCalories handles batch update of template calories
func HandleUpdateTemplateCalories(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    var req struct {
        Weight float64 `json:"weight"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    // Use default weight if not provided
    weight := req.Weight
    if weight <= 0 {
        weight = 70.0 // Default standard weight
    }
    
    err := manager.UpdateAllTemplateCalories(weight)
    if err != nil {
        log.ErrorF("Failed to update template calories: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    response := map[string]interface{}{
        "status": "success",
        "message": "所有模板卡路里已更新",
        "weight": weight,
    }
    
    json.NewEncoder(w).Encode(response)
}

// HandleUpdateExerciseCalories handles batch update of exercise record calories
func HandleUpdateExerciseCalories(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    var req struct {
        Weight float64 `json:"weight"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    // Use default weight if not provided
    weight := req.Weight
    if weight <= 0 {
        weight = 70.0 // Default standard weight
    }
    
    updatedCount, err := manager.UpdateAllExerciseCalories(weight)
    if err != nil {
        log.ErrorF("Failed to update exercise calories: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    response := map[string]interface{}{
        "status": "success",
        "message": fmt.Sprintf("已更新 %d 条锻炼记录的卡路里", updatedCount),
        "updated_count": updatedCount,
        "weight": weight,
    }
    
    json.NewEncoder(w).Encode(response)
}

// HandleGetMETValue handles getting specific MET value requests
func HandleGetMETValue(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    
    exerciseType := r.URL.Query().Get("type")
    intensity := r.URL.Query().Get("intensity")
    
    if exerciseType == "" || intensity == "" {
        http.Error(w, "exercise_type and intensity are required", http.StatusBadRequest)
        return
    }
    
    met, description := manager.GetMETValueWithDescription(exerciseType, intensity)
    
    response := map[string]interface{}{
        "met": met,
        "description": description,
        "exercise_type": exerciseType,
        "intensity": intensity,
    }
    
    json.NewEncoder(w).Encode(response)
} 