package http

import (
	"control"
	"encoding/json"
	log "mylog"
	h "net/http"
	"time"
	"view"
)

// HandleTimeStamp handles timestamp page
func HandleTimeStamp(w h.ResponseWriter, r *h.Request) {
	view.PageTimeStamp(w)
}

// HandleTodolist handles todolist page
func HandleTodolist(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleTodolist", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		// If no date provided, use today's date
		date = time.Now().Format("2006-01-02")
	}

	view.PageTodolist(w, date)
}

// HandleGoal renders the unified goal management page
func HandleGoal(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleGoal", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	view.PageGoal(w)
}

// HandleStatistics renders the statistics page
func HandleStatistics(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleStatistics", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageStatistics(w)
}

// HandleExercise renders the exercise page
func HandleExercise(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExercise", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		// If no date provided, use today's date
		date = time.Now().Format("2006-01-02")
	}

	view.PageExercise(w)
}

// HandleStatisticsAPI returns statistics data as JSON
func HandleStatisticsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleStatisticsAPI", r)
	if checkLogin(r) != 0 {
		w.WriteHeader(h.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if r.Method != h.MethodGet {
		w.WriteHeader(h.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	account := getAccountFromRequest(r)
	stats := control.GetStatistics(account)
	if stats == nil {
		w.WriteHeader(h.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get statistics"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.StatusOK)

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.ErrorF(log.ModuleStatistics, "Failed to encode statistics: %v", err)
		w.WriteHeader(h.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to encode statistics"})
		return
	}
}

// HandleTools handles tools page
func HandleTools(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleTools", r)

	view.PageTools(w)
}

// HandleEnglishLearning renders the English learning tracker page
func HandleEnglishLearning(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleEnglishLearning", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageEnglishLearning(w)
}
