package http

import (
	h "net/http"
)

func HandleGoal(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleGoal", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	view.PageGoal(w)
}

func HandleExercise(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExercise", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	view.PageExercise(w)
}

func HandleTools(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleTools", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	view.PageTools(w)
}
