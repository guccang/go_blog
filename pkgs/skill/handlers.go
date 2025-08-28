package skill

import (
	"blog"
	"encoding/json"
	"fmt"
	"io"
	log "mylog"
	"net/http"
	"strings"
)

// HTTP handlers for skill management

var skillManager = NewSkillManager()

// getAccountFromRequest extracts account from session cookie
func getAccountFromRequest(r *http.Request) string {
	sessionCookie, err := r.Cookie("session")
	if err != nil {
		log.DebugF("No session cookie found: %v", err)
		return ""
	}

	account := blog.GetAccountFromSession(sessionCookie.Value)
	if account == "" {
		return ""
	}
	return account
}

// AddSkillHandler handles adding a new skill
func AddSkillHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var skill Skill
	if err := json.Unmarshal(body, &skill); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	if err := skillManager.AddSkill(account, &skill); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add skill: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"skill":   skill,
	})
}

// GetSkillHandler handles retrieving a skill by ID
func GetSkillHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract skill ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Skill ID required", http.StatusBadRequest)
		return
	}
	skillID := pathParts[len(pathParts)-1]

	account := getAccountFromRequest(r)
	skill, err := skillManager.GetSkill(account, skillID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Skill not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"skill":   skill,
	})
}

// UpdateSkillHandler handles updating an existing skill
func UpdateSkillHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var skill Skill
	if err := json.Unmarshal(body, &skill); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	if err := skillManager.UpdateSkill(account, &skill); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update skill: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"skill":   skill,
	})
}

// DeleteSkillHandler handles deleting a skill
func DeleteSkillHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract skill ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Skill ID required", http.StatusBadRequest)
		return
	}
	skillID := pathParts[len(pathParts)-1]

	account := getAccountFromRequest(r)
	if err := skillManager.DeleteSkill(account, skillID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete skill: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Skill deleted successfully",
	})
}

// GetAllSkillsHandler handles retrieving all skills
func GetAllSkillsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	account := getAccountFromRequest(r)
	skills, err := skillManager.GetAllSkills(account)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get skills: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"skills":  skills,
		"count":   len(skills),
	})
}

// AddSkillContentHandler handles adding content to a skill
func AddSkillContentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract skill ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Skill ID required", http.StatusBadRequest)
		return
	}
	skillID := pathParts[len(pathParts)-2]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var content SkillContent
	if err := json.Unmarshal(body, &content); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	if err := skillManager.AddSkillContent(account, skillID, &content); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add content: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"content": content,
	})
}

// UpdateSkillContentHandler handles updating skill content
func UpdateSkillContentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract skill ID and content ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Error(w, "Skill ID and Content ID required", http.StatusBadRequest)
		return
	}
	skillID := pathParts[len(pathParts)-3]
	contentID := pathParts[len(pathParts)-1]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var content SkillContent
	if err := json.Unmarshal(body, &content); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	if err := skillManager.UpdateSkillContent(account, skillID, contentID, &content); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update content: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"content": content,
	})
}

// DeleteSkillContentHandler handles deleting skill content
func DeleteSkillContentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract skill ID and content ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Error(w, "Skill ID and Content ID required", http.StatusBadRequest)
		return
	}
	skillID := pathParts[len(pathParts)-3]
	contentID := pathParts[len(pathParts)-1]

	account := getAccountFromRequest(r)
	if err := skillManager.DeleteSkillContent(account, skillID, contentID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete content: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Content deleted successfully",
	})
}

// GetSkillsByCategoryHandler handles retrieving skills by category
func GetSkillsByCategoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	category := r.URL.Query().Get("category")
	if category == "" {
		http.Error(w, "Category parameter required", http.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	skills, err := skillManager.GetSkillsByCategory(account, category)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get skills: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"skills":  skills,
		"count":   len(skills),
	})
}

// GetSkillsByTagHandler handles retrieving skills by tag
func GetSkillsByTagHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "Tag parameter required", http.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	skills, err := skillManager.GetSkillsByTag(account, tag)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get skills: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"skills":  skills,
		"count":   len(skills),
	})
}

// GetActiveSkillsHandler handles retrieving active skills
func GetActiveSkillsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	account := getAccountFromRequest(r)
	skills, err := skillManager.GetActiveSkills(account)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get skills: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"skills":  skills,
		"count":   len(skills),
	})
}

// SkillStats represents statistics about skills
// 技能统计信息
type SkillStats struct {
	TotalSkills       int            `json:"total_skills"`
	ActiveSkills      int            `json:"active_skills"`
	CompletedSkills   int            `json:"completed_skills"`
	TotalContents     int            `json:"total_contents"`
	CompletedContents int            `json:"completed_contents"`
	AvgProgress       float64        `json:"avg_progress"`
	Categories        map[string]int `json:"categories"`
	Tags              map[string]int `json:"tags"`
}

// GetSkillStatsHandler handles retrieving skill statistics
func GetSkillStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	account := getAccountFromRequest(r)
	allSkills, err := skillManager.GetAllSkills(account)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get skills: %v", err), http.StatusInternalServerError)
		return
	}

	stats := SkillStats{
		Categories: make(map[string]int),
		Tags:       make(map[string]int),
	}

	totalProgress := 0.0
	for _, skill := range allSkills {
		stats.TotalSkills++
		if skill.IsActive {
			stats.ActiveSkills++
		}
		if skill.Progress >= 100 {
			stats.CompletedSkills++
		}

		stats.TotalContents += len(skill.Contents)
		for _, content := range skill.Contents {
			if content.Status == "completed" {
				stats.CompletedContents++
			}
		}

		totalProgress += skill.Progress

		// Count categories
		if skill.Category != "" {
			stats.Categories[skill.Category]++
		}

		// Count tags
		for _, tag := range skill.Tags {
			stats.Tags[tag]++
		}
	}

	if stats.TotalSkills > 0 {
		stats.AvgProgress = totalProgress / float64(stats.TotalSkills)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}
