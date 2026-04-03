package projectmgmt

import (
	"blog"
	"encoding/json"
	"io"
	log "mylog"
	"net/http"
)

func getAccountFromRequest(r *http.Request) string {
	sessionCookie, err := r.Cookie("session")
	if err != nil {
		log.DebugF(log.ModuleBlog, "projectmgmt no session cookie: %v", err)
		return ""
	}
	return blog.GetAccountFromSession(sessionCookie.Value)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func decodeBody(r *http.Request, dst interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

func HandleProjects(w http.ResponseWriter, r *http.Request) {
	account := getAccountFromRequest(r)
	if account == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "unauthorized"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		projectID := r.URL.Query().Get("project_id")
		if projectID != "" {
			project, err := GetProjectWithAccount(account, projectID)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": project})
			return
		}
		status := r.URL.Query().Get("status")
		projects, err := ListProjectsWithAccount(account, status)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": projects})
	case http.MethodPost:
		var req Project
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		project, err := CreateProjectWithAccount(account, &req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": project})
	case http.MethodPut:
		var req Project
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		if err := UpdateProjectWithAccount(account, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		project, _ := GetProjectWithAccount(account, req.ID)
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": project})
	case http.MethodDelete:
		projectID := r.URL.Query().Get("project_id")
		if projectID == "" {
			var req struct {
				ProjectID string `json:"project_id"`
			}
			if err := decodeBody(r, &req); err == nil {
				projectID = req.ProjectID
			}
		}
		if projectID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "project_id is required"})
			return
		}
		if err := DeleteProjectWithAccount(account, projectID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"success": false, "message": "method not allowed"})
	}
}

func HandleProjectSummary(w http.ResponseWriter, r *http.Request) {
	account := getAccountFromRequest(r)
	if account == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "unauthorized"})
		return
	}
	summary, err := GetProjectSummaryWithAccount(account)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": summary})
}

func HandleProjectGoals(w http.ResponseWriter, r *http.Request) {
	account := getAccountFromRequest(r)
	if account == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "unauthorized"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		projectID := r.URL.Query().Get("project_id")
		project, err := GetProjectWithAccount(account, projectID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": project.Goals})
	case http.MethodPost:
		var req struct {
			ProjectID string `json:"project_id"`
			Goal      Goal   `json:"goal"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		goal, err := AddGoalWithAccount(account, req.ProjectID, req.Goal)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": goal})
	case http.MethodPut:
		var req struct {
			ProjectID string `json:"project_id"`
			Goal      Goal   `json:"goal"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		if err := UpdateGoalWithAccount(account, req.ProjectID, req.Goal); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	case http.MethodDelete:
		var req struct {
			ProjectID string `json:"project_id"`
			GoalID    string `json:"goal_id"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		if err := DeleteGoalWithAccount(account, req.ProjectID, req.GoalID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"success": false, "message": "method not allowed"})
	}
}

func HandleProjectOKRs(w http.ResponseWriter, r *http.Request) {
	account := getAccountFromRequest(r)
	if account == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "unauthorized"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		projectID := r.URL.Query().Get("project_id")
		project, err := GetProjectWithAccount(account, projectID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": project.OKRs})
	case http.MethodPost:
		var req struct {
			ProjectID string `json:"project_id"`
			OKR       OKR    `json:"okr"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		okr, err := AddOKRWithAccount(account, req.ProjectID, req.OKR)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": okr})
	case http.MethodPut:
		var req struct {
			ProjectID string `json:"project_id"`
			OKR       OKR    `json:"okr"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		if err := UpdateOKRWithAccount(account, req.ProjectID, req.OKR); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	case http.MethodDelete:
		var req struct {
			ProjectID string `json:"project_id"`
			OKRID     string `json:"okr_id"`
		}
		if err := decodeBody(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		if err := DeleteOKRWithAccount(account, req.ProjectID, req.OKRID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"success": false, "message": "method not allowed"})
	}
}

func HandleProjectKeyResults(w http.ResponseWriter, r *http.Request) {
	account := getAccountFromRequest(r)
	if account == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "message": "unauthorized"})
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"success": false, "message": "method not allowed"})
		return
	}
	var req struct {
		ProjectID string    `json:"project_id"`
		OKRID     string    `json:"okr_id"`
		KeyResult KeyResult `json:"key_result"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	if err := UpdateKeyResultWithAccount(account, req.ProjectID, req.OKRID, req.KeyResult); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}
