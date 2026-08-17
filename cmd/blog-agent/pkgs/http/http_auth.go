package http

import (
	"config"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"login"
	"module"
	log "mylog"
	h "net/http"
	control "service"
	"strings"
	"time"
)

// HandleLoginSMSAPI handles SMS login code generation API (disabled)
func HandleLoginSMSAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLoginSMSAPI", r)
	h.Error(w, "短信功能暂时关闭", h.StatusBadRequest)
}

// HandleLoginSMS handles SMS login functionality
func HandleLoginSMS(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLoginSMS", r)
	h.Error(w, "短信功能暂时关闭", h.StatusBadRequest)
	return

	r.ParseMultipartForm(32 << 20) // 32MB

	code := r.FormValue("code")
	if code == "" {
		h.Error(w, "code parameter is missing", h.StatusBadRequest)
		return
	}

	device_id := r.FormValue("device_id")
	if device_id == "" {
		h.Error(w, "device_id parameter is missing", h.StatusBadRequest)
		return
	}

	// Validate device_id format
	if !strings.HasPrefix(device_id, "SK") {
		h.Error(w, "invalid device_id format", h.StatusBadRequest)
		return
	}
	// md5(admin+pwd)
	account := r.FormValue("account")
	if account == "" {
		h.Error(w, "account parameter is missing", h.StatusBadRequest)
		return
	}

	pwd := login.GetPwd(account)
	if pwd == "" {
		h.Error(w, "account not found", h.StatusBadRequest)
		return
	}

	hash := md5.Sum([]byte(account + pwd))
	inner_device_id := "SK" + hex.EncodeToString(hash[:])
	if inner_device_id != device_id {
		h.Error(w, "invalid device_id inner_device_id="+inner_device_id+" device_id="+device_id, h.StatusBadRequest)
		return
	}

	session, ret := login.LoginSMS(account, code)
	if ret != 0 {
		h.Error(w, "invalid SMS code or code expired", h.StatusBadRequest)
		return
	}
	log.InfoF(log.ModuleAuth, "LoginSMS add session=%s code=%s device_id=%s", session, code, device_id)

	config.ReloadConfigFromSQLite(account)
	migratePIProvidersToJSON(account)

	// set cookie
	cookie := &h.Cookie{
		Name:    "session",
		Value:   session,
		Expires: time.Now().Add(48 * time.Hour), // 过期时间为两天
		Path:    "/",
	}

	h.SetCookie(w, cookie)

	respondLoginSuccess(w, r)
}

// HandleLogin handles standard login functionality
func HandleLogin(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLogin", r)

	r.ParseMultipartForm(32 << 20) // 32MB

	account := r.FormValue("account")
	if account == "" {
		h.Error(w, "account parameter is missing", h.StatusBadRequest)
		return
	}

	pwd := r.FormValue("password")
	if pwd == "" {
		h.Error(w, "pwd parameter is missing", h.StatusBadRequest)
		return
	}

	device_id := r.FormValue("device_id")
	log.DebugF(log.ModuleAuth, "login request account=%s device_id_present=%t", account, device_id != "")

	session, ret := login.Login(account, pwd)
	if ret != 0 {
		// 记录失败的登录
		h.Error(w, "Error account or pwd", h.StatusBadRequest)
		return
	}

	// 记录成功的登录

	config.ReloadConfigFromSQLite(account)
	migratePIProvidersToJSON(account)

	// 重新加载提示词配置
	config.ReloadPrompts(account)

	// set cookie
	cookie := &h.Cookie{
		Name:    "session",
		Value:   session,
		Expires: time.Now().Add(48 * time.Hour), // 过期时间为两天
		Path:    "/",
	}
	h.SetCookie(w, cookie)

	respondLoginSuccess(w, r)
}

func respondLoginSuccess(w h.ResponseWriter, r *h.Request) {
	if strings.EqualFold(r.Header.Get("X-Requested-With"), "XMLHttpRequest") || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"redirect": "/main"})
		return
	}
	h.Redirect(w, r, "/main", h.StatusFound)
}

// migratePIProvidersToJSON performs the one-time SQLite data migration from
// legacy provider keys to the sole pi_providers JSON setting.
func migratePIProvidersToJSON(account string) {
	item := control.GetBlog(account, config.GetSysConfigTitle())
	if item == nil {
		return
	}
	configs, comments := parseConfigContentWithComments(item.Content)
	if !needsPIProviderMigration(configs) {
		return
	}
	ensurePIAgentConfig(configs, comments)
	content := buildConfigContentWithComments(configs, comments)
	if content == item.Content {
		return
	}
	result := control.ModifyBlog(account, &module.UploadedBlogData{Title: item.Title, Content: content, AuthType: item.AuthType, Tags: item.Tags, Encrypt: item.Encrypt})
	if result != 0 {
		log.ErrorF(log.ModuleConfig, "migrate PI provider JSON failed account=%s result=%d", account, result)
		return
	}
	config.UpdateConfigFromBlog(account, content)
}

func needsPIProviderMigration(configs map[string]string) bool {
	if _, exists := configs["pi_providers"]; !exists {
		return true
	}
	for key := range configs {
		if key == "pi_default_provider" || strings.HasPrefix(key, "pi_provider_") || key == "deepseek_api_key" || key == "deepseek_api_url" || key == "deepseek_model" {
			return true
		}
	}
	return false
}

// HandleRegister handles user registration
func HandleRegister(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleRegister", r)

	r.ParseMultipartForm(32 << 20) // 32MB

	account := r.FormValue("account")
	if account == "" {
		h.Error(w, "account parameter is missing", h.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	if password == "" {
		h.Error(w, "password parameter is missing", h.StatusBadRequest)
		return
	}

	ret := login.Register(account, password)

	switch ret {
	case 0:
		w.Write([]byte("注册成功"))
	case 1:
		h.Error(w, "账号已存在", h.StatusBadRequest)
	case 2:
		h.Error(w, "无效的账号或密码", h.StatusBadRequest)
	default:
		h.Error(w, "注册失败", h.StatusBadRequest)
	}
}

// HandleIndex handles the index/login page
func HandleIndex(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleIndex", r)
	view.PageIndex(w)
}

// basicAuth provides basic authentication middleware
func basicAuth(next h.Handler) h.Handler {
	return h.HandlerFunc(func(w h.ResponseWriter, r *h.Request) {
		if checkLogin(r) != 0 {
			h.Redirect(w, r, "/index", 302)
			return
		}
		next.ServeHTTP(w, r)
	})
}
