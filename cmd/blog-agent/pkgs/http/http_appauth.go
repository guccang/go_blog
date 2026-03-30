package http

import (
	"encoding/json"
	"login"
	log "mylog"
	h "net/http"
)

// HandleAppAuthLogin 供 app-agent 调用，校验账号密码是否有效
func HandleAppAuthLogin(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAppAuthLogin", r)

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Error(w, "invalid json", h.StatusBadRequest)
		return
	}

	if req.Account == "" || req.Password == "" {
		h.Error(w, "account and password are required", h.StatusBadRequest)
		return
	}

	ret := login.VerifyCredentials(req.Account, req.Password)
	if ret != 0 {
		log.InfoF(log.ModuleAuth, "app auth verify failed account=%s ret=%d", req.Account, ret)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "invalid account or password",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"account": req.Account,
	})
}

// HandleAppAuthRegister 供 app-agent 调用，确保账号存在且密码匹配。
func HandleAppAuthRegister(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAppAuthRegister", r)

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Error(w, "invalid json", h.StatusBadRequest)
		return
	}

	if req.Account == "" || req.Password == "" {
		h.Error(w, "account and password are required", h.StatusBadRequest)
		return
	}

	switch ret := login.VerifyCredentials(req.Account, req.Password); ret {
	case 0:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"account": req.Account,
			"status":  "exists",
		})
		return
	case 1:
		if regRet := login.Register(req.Account, req.Password); regRet != 0 {
			log.InfoF(log.ModuleAuth, "app auth register failed account=%s ret=%d", req.Account, regRet)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(h.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "register failed",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"account": req.Account,
			"status":  "created",
		})
		return
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "account exists but password mismatch",
		})
		return
	}
}
