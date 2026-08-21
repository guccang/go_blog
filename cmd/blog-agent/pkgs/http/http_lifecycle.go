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

func HandleExerciseManage(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExerciseManage", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	h.Redirect(w, r, "/exercise", h.StatusFound)
}

func HandleExercisePro(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExercisePro", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	emitUsageHook(r, getAccountFromRequest(r), blog.HookPageOpened, "exercise_professional", "page", "exercise_pro", "专业训练", "", nil, map[string]any{"status": "success"})
	view.PageExercisePro(w)
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

func HandleVisualThemes(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleVisualThemes", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	emitUsageHook(r, getAccountFromRequest(r), blog.HookPageOpened, "visual_themes", "page", "visual_themes", "视觉主题图鉴", "", nil, map[string]any{"status": "success"})
	view.PageVisualThemes(w)
}

func HandleNaturalMotionLab(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleNaturalMotionLab", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	emitUsageHook(r, getAccountFromRequest(r), blog.HookPageOpened, "natural_motion_lab", "page", "natural_motion_lab", "青瓷雨动效实验室", "", nil, map[string]any{"status": "success"})
	view.PageNaturalMotionLab(w)
}
