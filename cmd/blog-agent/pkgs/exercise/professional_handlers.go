package exercise

import (
	"auth"
	"blog"
	"encoding/json"
	"errors"
	"net/http"
)

func HandleProfessionalCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeProfessionalJSON(w, http.StatusOK, ProfessionalCatalogData())
}

func HandleProfessionalProfile(w http.ResponseWriter, r *http.Request) {
	account := getAccountFromRequest(r)
	switch r.Method {
	case http.MethodGet:
		profile, err := GetProfessionalProfile(account)
		if err != nil {
			writeProfessionalError(w, http.StatusInternalServerError, err)
			return
		}
		writeProfessionalJSON(w, http.StatusOK, profile)
	case http.MethodPut:
		var profile ProfessionalProfile
		if err := decodeProfessionalJSON(r, &profile); err != nil {
			writeProfessionalError(w, http.StatusBadRequest, err)
			return
		}
		saved, err := SaveProfessionalProfile(account, profile)
		if err != nil {
			writeProfessionalError(w, http.StatusBadRequest, err)
			return
		}
		emitProfessionalHook(r, blog.HookFeatureUsed, "level_selection", map[string]any{
			"days_per_week": saved.DaysPerWeek,
			"levels":        saved.Levels,
		}, map[string]any{"status": "success"})
		writeProfessionalJSON(w, http.StatusOK, saved)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func HandleProfessionalPlanPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ProfessionalPlanRequest
	if err := decodeProfessionalJSON(r, &req); err != nil {
		writeProfessionalError(w, http.StatusBadRequest, err)
		return
	}
	plan, err := PreviewProfessionalPlan(req)
	if err != nil {
		writeProfessionalError(w, http.StatusBadRequest, err)
		return
	}
	emitProfessionalHook(r, blog.HookFeatureUsed, "plan_previewed", map[string]any{
		"start_date": req.StartDate, "days_per_week": req.DaysPerWeek,
	}, map[string]any{"status": "success", "session_count": len(plan.Sessions)})
	writeProfessionalJSON(w, http.StatusOK, plan)
}

func HandleProfessionalPlanApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ProfessionalPlanRequest
	if err := decodeProfessionalJSON(r, &req); err != nil {
		writeProfessionalError(w, http.StatusBadRequest, err)
		return
	}
	result, err := ApplyProfessionalPlan(getAccountFromRequest(r), req)
	if errors.Is(err, ErrProfessionalPlanConflict) {
		writeProfessionalJSON(w, http.StatusConflict, map[string]any{
			"error": "所选七天内已有专业训练计划", "code": "professional_plan_conflict",
		})
		return
	}
	if err != nil {
		writeProfessionalError(w, http.StatusBadRequest, err)
		return
	}
	action := "plan_applied"
	if req.Replace {
		action = "plan_replaced"
	}
	emitProfessionalHook(r, blog.HookFeatureUsed, action, map[string]any{
		"start_date": req.StartDate, "days_per_week": req.DaysPerWeek, "replace": req.Replace,
	}, map[string]any{
		"status": "success", "created": result.Created, "skipped": result.Skipped, "preserved": result.Preserved,
	})
	writeProfessionalJSON(w, http.StatusOK, result)
}

func decodeProfessionalJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeProfessionalJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProfessionalError(w http.ResponseWriter, status int, err error) {
	writeProfessionalJSON(w, status, map[string]string{"error": err.Error()})
}

func emitProfessionalHook(r *http.Request, eventType blog.HookType, action string, context, result map[string]any) {
	blog.EmitHook(blog.HookEvent{
		Account: getAccountFromRequest(r), SessionID: auth.GetSessionFromRequest(r),
		Type: eventType, Feature: "exercise_professional", ObjectType: "exercise_plan",
		ObjectID: action, Title: action, Context: context, Result: result,
	})
}
