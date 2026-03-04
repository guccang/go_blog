package http

import (
	"skill"
	"net/http"
)

// RegisterSkillRoutes registers skill-related HTTP routes
func RegisterSkillRoutes() {
	// Skill management routes
	http.HandleFunc("/api/skill", skill.AddSkillHandler)
	http.HandleFunc("/api/skills", skill.GetAllSkillsHandler)
	http.HandleFunc("/api/skills/category", skill.GetSkillsByCategoryHandler)
	http.HandleFunc("/api/skills/tag", skill.GetSkillsByTagHandler)
	http.HandleFunc("/api/skills/active", skill.GetActiveSkillsHandler)
	http.HandleFunc("/api/skills/stats", skill.GetSkillStatsHandler)

	// Skill content management routes
	http.HandleFunc("/api/skill/", func(w http.ResponseWriter, r *http.Request) {
		// Handle nested content routes
		path := r.URL.Path
		if len(path) > len("/api/skill/") {
			remainingPath := path[len("/api/skill/"):]
			pathParts := splitPath(remainingPath)
			
			if len(pathParts) >= 2 && pathParts[1] == "content" {
				if len(pathParts) == 2 && r.Method == "POST" {
					skill.AddSkillContentHandler(w, r)
					return
				} else if len(pathParts) == 3 {
					switch r.Method {
					case "PUT":
						skill.UpdateSkillContentHandler(w, r)
					case "DELETE":
						skill.DeleteSkillContentHandler(w, r)
					default:
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					}
					return
				}
			}
		}
		
		// Fallback to regular skill handling
		switch r.Method {
		case "GET":
			skill.GetSkillHandler(w, r)
		case "PUT":
			skill.UpdateSkillHandler(w, r)
		case "DELETE":
			skill.DeleteSkillHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// splitPath splits a URL path into parts
func splitPath(path string) []string {
	var parts []string
	current := ""
	
	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		parts = append(parts, current)
	}
	
	return parts
}