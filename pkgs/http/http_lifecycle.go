package http

import (
	"control"
	"encoding/json"
	log "mylog"
	h "net/http"
	"strconv"
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

// HandleYearPlan renders the year plan page
func HandleYearPlan(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleYearPlan", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	// Get the current year
	year := r.URL.Query().Get("year")
	// string to int
	yearInt, err := strconv.Atoi(year)
	if err != nil {
		yearInt = time.Now().Year()
	}

	// Render the yearplan template
	view.PageYearPlan(w, yearInt)
}

// HandleMonthGoal renders the month goal page
func HandleMonthGoal(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleMonthGoal", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	// Get the current year and month
	year := r.URL.Query().Get("year")
	month := r.URL.Query().Get("month")

	yearInt, err := strconv.Atoi(year)
	if err != nil {
		yearInt = time.Now().Year()
	}

	monthInt, err := strconv.Atoi(month)
	if err != nil {
		monthInt = int(time.Now().Month())
	}

	// Render the monthgoal template
	view.PageMonthGoal(w, yearInt, monthInt)
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

	stats := control.GetStatistics()
	if stats == nil {
		w.WriteHeader(h.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get statistics"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.StatusOK)

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.ErrorF("Failed to encode statistics: %v", err)
		w.WriteHeader(h.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to encode statistics"})
		return
	}
}

// HandleTools handles tools page
func HandleTools(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleTools", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageTools(w)
}

// HandleSkill renders the skill learning page
func HandleSkill(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleSkill", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageSkill(w)
}
