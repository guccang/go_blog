package http

import (
	"blog"
	h "net/http"
)

func HandleGoal(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleGoal", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	emitUsageHook(r, getAccountFromRequest(r), blog.HookPageOpened, "goal_management", "page", "goal", "goal", "", nil, map[string]any{"status": "success"})
	view.PageGoal(w)
}

func HandleGoalManage(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleGoalManage", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	view.PageGoalManage(w)
}

func HandleExercise(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExercise", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	emitUsageHook(r, getAccountFromRequest(r), blog.HookPageOpened, "exercise_management", "page", "exercise", "exercise", "", nil, map[string]any{"status": "success"})
	view.PageExercise(w)
}

// HandleExerciseManage keeps the detailed template, collection and profile tools
// available without making the daily exercise page carry all of their complexity.
func HandleExerciseManage(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExerciseManage", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	view.PageExerciseManage(w)
}

func HandleReadingManage(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingManage", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	view.PageReadingManage(w)
}

func HandleTools(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleTools", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	emitUsageHook(r, getAccountFromRequest(r), blog.HookPageOpened, "tools", "page", "tools", "tools", "", nil, map[string]any{"status": "success"})
	view.PageTools(w)
}
