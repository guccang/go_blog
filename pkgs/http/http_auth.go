package http

import (
	"comment"
	"config"
	"control"
	"crypto/md5"
	"encoding/hex"
	"login"
	log "mylog"
	h "net/http"
	db "persistence"
	"strings"
	"time"
	"view"
)

// HandleLoginSMSAPI handles SMS login code generation API
func HandleLoginSMSAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLoginSMSAPI", r)

	h.Error(w, "短信功能暂时关闭", h.StatusBadRequest)
	return

	device_id := r.FormValue("device_id")
	if device_id == "" {
		h.Error(w, "device_id parameter is missing", h.StatusBadRequest)
		return
	}

	// Check if device_id exists in config or validate format (starts with SK)
	if !strings.HasPrefix(device_id, "SK") || len(device_id) != 34 {
		h.Error(w, "invalid device_id format", h.StatusBadRequest)
		return
	}

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

	code, ret := login.GenerateSMSCode(account)
	log.InfoF(log.ModuleAuth, "SMS Generate code=%s for device_id=%s", code, device_id)
	if ret != 0 {
		h.Error(w, "SMS generation failed", h.StatusBadRequest)
		return
	}

	// 提示 短信已发送
	w.Write([]byte("短信已发送 请注意查收"))
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

	// 获取用户IP
	remoteAddr := r.RemoteAddr
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		remoteAddr = xForwardedFor
	}
	control.RecordUserLogin(account, remoteAddr, true)

	// 加载数据
	blogs_txt_dir := config.GetBlogsPath(account)
	control.ImportBlogsFromPath(account, blogs_txt_dir)
	db.SaveBlogs(account, control.GetBlogs(account))

	// 加载comment
	comment.LoadComments(account)

	// config
	sys_conf_path := config.GetSysConfigPath(account)
	config.ReloadConfigWithAccount(account, sys_conf_path)

	// set cookie
	cookie := &h.Cookie{
		Name:    "session",
		Value:   session,
		Expires: time.Now().Add(48 * time.Hour), // 过期时间为两天
		Path:    "/",
	}
	h.SetCookie(w, cookie)

	h.Redirect(w, r, "/main", 302)
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
	log.DebugF(log.ModuleAuth, "account=%s pwd=%s device_id=%s", account, pwd, device_id)

	// 获取用户IP
	remoteAddr := r.RemoteAddr
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		remoteAddr = xForwardedFor
	}

	session, ret := login.Login(account, pwd)
	if ret != 0 {
		// 记录失败的登录
		control.RecordUserLogin(account, remoteAddr, false)
		h.Error(w, "Error account or pwd", h.StatusBadRequest)
		return
	}

	// 记录成功的登录
	control.RecordUserLogin(account, remoteAddr, true)

	// 加载数据
	blogs_txt_dir := config.GetBlogsPath(account)
	control.ImportBlogsFromPath(account, blogs_txt_dir)
	db.SaveBlogs(account, control.GetBlogs(account))

	// 加载comment
	comment.LoadComments(account)

	// config
	sys_conf_path := config.GetSysConfigPath(account)
	config.ReloadConfigWithAccount(account, sys_conf_path)

	// set cookie
	cookie := &h.Cookie{
		Name:    "session",
		Value:   session,
		Expires: time.Now().Add(48 * time.Hour), // 过期时间为两天
		Path:    "/",
	}
	h.SetCookie(w, cookie)

	log.DebugF(log.ModuleAuth, "login success account=%s pwd=%s session=%s", account, pwd, session)
	h.Redirect(w, r, "/main", 302)
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
