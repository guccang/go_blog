package http

import (
	"account"
	"blog"
	"config"
	"encoding/json"
	log "mylog"
	h "net/http"
	"path/filepath"
	"strconv"
	"strings"
	t "text/template"
)

// HandleAccount 处理账户信息页面请求
func HandleAccount(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAccount", r)

	// 检查登录状态
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	session := getsession(r)
	userAccount := blog.GetAccountFromSession(session)

	if userAccount == "" {
		h.Error(w, "获取用户信息失败", h.StatusUnauthorized)
		return
	}

	// 获取账户信息
	accountInfo, err := account.GetAccountInfo(userAccount)
	if err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to get account info: %v", err)
		h.Error(w, "获取账户信息失败", h.StatusInternalServerError)
		return
	}

	// 准备模板数据
	data := struct {
		UserAccount string
		AccountInfo *account.AccountInfo
		BMI         float64
		BMIStatus   string
		Age         int
		HobbiesStr  string
	}{
		UserAccount: userAccount,
		AccountInfo: accountInfo,
		BMI:         accountInfo.GetBMI(),
		BMIStatus:   accountInfo.GetBMIStatus(),
		Age:         accountInfo.GetAge(),
		HobbiesStr:  account.HobbiesToString(accountInfo.Hobbies),
	}

	// 渲染模板
	exeDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(exeDir, "account.template"))
	if err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to parse account.template: %v", err)
		h.Error(w, "模板解析失败", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to render account template: %v", err)
		h.Error(w, "模板渲染失败", h.StatusInternalServerError)
	}
}

// HandleAccountAPI 处理账户信息API请求
func HandleAccountAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAccountAPI", r)

	// 检查登录状态
	if checkLogin(r) != 0 {
		sendJSONError(w, "未登录", 401)
		return
	}

	session := getsession(r)
	userAccount := blog.GetAccountFromSession(session)

	if userAccount == "" {
		sendJSONError(w, "获取用户信息失败", 401)
		return
	}

	switch r.Method {
	case "GET":
		handleGetAccountInfo(w, r, userAccount)
	case "POST", "PUT":
		handleUpdateAccountInfo(w, r, userAccount)
	default:
		sendJSONError(w, "不支持的请求方法", 405)
	}
}

// handleGetAccountInfo 处理获取账户信息
func handleGetAccountInfo(w h.ResponseWriter, r *h.Request, userAccount string) {
	accountInfo, err := account.GetAccountInfo(userAccount)
	if err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to get account info: %v", err)
		sendJSONError(w, "获取账户信息失败", 500)
		return
	}

	response := map[string]interface{}{
		"success":     true,
		"account":     userAccount,
		"accountInfo": accountInfo,
		"bmi":         accountInfo.GetBMI(),
		"bmiStatus":   accountInfo.GetBMIStatus(),
		"age":         accountInfo.GetAge(),
		"hobbiesStr":  account.HobbiesToString(accountInfo.Hobbies),
	}

	sendJSONResponse(w, response)
}

// handleUpdateAccountInfo 处理更新账户信息
func handleUpdateAccountInfo(w h.ResponseWriter, r *h.Request, userAccount string) {
	// 解析表单数据 (支持 multipart/form-data)
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max memory
		if err := r.ParseForm(); err != nil { // 回退到普通表单解析
			log.ErrorF(log.ModuleAccount, "Failed to parse form: %v", err)
			sendJSONError(w, "表单解析失败", 400)
			return
		}
	}
	
	// 调试日志：记录接收到的表单数据
	log.DebugF(log.ModuleAccount, "Received form data for account %s: %+v", userAccount, r.Form)

	// 获取当前账户信息
	currentInfo, err := account.GetAccountInfo(userAccount)
	if err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to get current account info: %v", err)
		sendJSONError(w, "获取当前账户信息失败", 500)
		return
	}

	// 更新账户信息 - 只更新表单中提供的字段
	updatedInfo := &account.AccountInfo{
		// 保留原有值作为默认值
		Name:     currentInfo.Name,
		Phone:    currentInfo.Phone,
		Email:    currentInfo.Email,
		Bio:      currentInfo.Bio,
		Location: currentInfo.Location,
		Website:  currentInfo.Website,
		Birthday: currentInfo.Birthday,
		Avatar:   currentInfo.Avatar,
		Age:      currentInfo.Age,
		Height:   currentInfo.Height,
		Weight:   currentInfo.Weight,
		Hobbies:  currentInfo.Hobbies,
	}

	// 只更新表单中提供的字段
	if name := strings.TrimSpace(r.FormValue("name")); name != "" {
		updatedInfo.Name = name
	}
	
	if phone := strings.TrimSpace(r.FormValue("phone")); phone != "" {
		updatedInfo.Phone = phone
	}
	
	if email := strings.TrimSpace(r.FormValue("email")); email != "" {
		updatedInfo.Email = email
	}
	
	if bio := strings.TrimSpace(r.FormValue("bio")); bio != "" {
		updatedInfo.Bio = bio
	}
	
	if location := strings.TrimSpace(r.FormValue("location")); location != "" {
		updatedInfo.Location = location
	}
	
	if website := strings.TrimSpace(r.FormValue("website")); website != "" {
		updatedInfo.Website = website
	}
	
	if birthday := strings.TrimSpace(r.FormValue("birthday")); birthday != "" {
		updatedInfo.Birthday = birthday
	}
	
	if avatar := strings.TrimSpace(r.FormValue("avatar")); avatar != "" {
		updatedInfo.Avatar = avatar
	}

	// 解析数值字段
	if ageStr := strings.TrimSpace(r.FormValue("age")); ageStr != "" {
		if age, err := strconv.Atoi(ageStr); err == nil {
			updatedInfo.Age = age
		}
	}

	if heightStr := strings.TrimSpace(r.FormValue("height")); heightStr != "" {
		if height, err := strconv.ParseFloat(heightStr, 64); err == nil {
			updatedInfo.Height = height
		}
	}

	if weightStr := strings.TrimSpace(r.FormValue("weight")); weightStr != "" {
		if weight, err := strconv.ParseFloat(weightStr, 64); err == nil {
			updatedInfo.Weight = weight
		}
	}

	// 解析爱好
	if hobbiesStr := strings.TrimSpace(r.FormValue("hobbies")); hobbiesStr != "" {
		updatedInfo.Hobbies = account.ParseHobbies(hobbiesStr)
	}

	// 如果某些字段为空，保留原有值
	if updatedInfo.Name == "" {
		updatedInfo.Name = currentInfo.Name
	}
	if updatedInfo.Avatar == "" {
		updatedInfo.Avatar = currentInfo.Avatar
	}

	// 验证账户信息
	if errors := account.ValidateAccountInfo(updatedInfo); len(errors) > 0 {
		response := map[string]interface{}{
			"success": false,
			"message": "数据验证失败",
			"errors":  errors,
		}
		sendJSONResponse(w, response)
		return
	}

	// 保存账户信息
	if err := account.SaveAccountInfo(userAccount, updatedInfo); err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to save account info: %v", err)
		sendJSONError(w, "保存账户信息失败", 500)
		return
	}

	// 返回成功响应
	response := map[string]interface{}{
		"success":     true,
		"message":     "账户信息更新成功",
		"accountInfo": updatedInfo,
		"bmi":         updatedInfo.GetBMI(),
		"bmiStatus":   updatedInfo.GetBMIStatus(),
		"age":         updatedInfo.GetAge(),
	}

	sendJSONResponse(w, response)
}

// sendJSONResponse 发送JSON响应
func sendJSONResponse(w h.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to marshal JSON: %v", err)
		sendJSONError(w, "JSON序列化失败", 500)
		return
	}

	w.Write(jsonData)
}

// sendJSONError 发送JSON错误响应
func sendJSONError(w h.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"success": false,
		"message": message,
	}

	jsonData, _ := json.Marshal(response)
	w.Write(jsonData)
}

// HandleAccountAvatar 处理头像上传/更新
func HandleAccountAvatar(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAccountAvatar", r)

	// 检查登录状态
	if checkLogin(r) != 0 {
		sendJSONError(w, "未登录", 401)
		return
	}

	session := getsession(r)
	userAccount := blog.GetAccountFromSession(session)

	if userAccount == "" {
		sendJSONError(w, "获取用户信息失败", 401)
		return
	}

	if r.Method != "POST" {
		sendJSONError(w, "不支持的请求方法", 405)
		return
	}

	// 解析表单数据 (支持 multipart/form-data)
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max memory
		if err := r.ParseForm(); err != nil { // 回退到普通表单解析
			sendJSONError(w, "表单解析失败", 400)
			return
		}
	}
	
	// 调试日志：记录接收到的头像数据
	log.DebugF(log.ModuleAccount, "Received avatar form data: %+v", r.Form)

	avatar := strings.TrimSpace(r.FormValue("avatar"))
	if avatar == "" {
		sendJSONError(w, "头像不能为空", 400)
		return
	}

	// 头像只能是单个字符
	if len([]rune(avatar)) != 1 {
		sendJSONError(w, "头像必须是单个字符", 400)
		return
	}

	// 获取当前账户信息
	accountInfo, err := account.GetAccountInfo(userAccount)
	if err != nil {
		sendJSONError(w, "获取账户信息失败", 500)
		return
	}

	// 更新头像
	accountInfo.Avatar = avatar

	// 保存更新后的信息
	if err := account.SaveAccountInfo(userAccount, accountInfo); err != nil {
		sendJSONError(w, "保存头像失败", 500)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "头像更新成功",
		"avatar":  avatar,
	}

	sendJSONResponse(w, response)
}

// HandleAccountSettings 处理账户设置页面
func HandleAccountSettings(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAccountSettings", r)

	// 检查登录状态
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	// 重定向到账户页面
	h.Redirect(w, r, "/account", 302)
}

// init 函数用于注册路由（这个函数需要在http_core.go中调用）
func InitAccountRoutes() {
	// 这个函数的内容将在http_core.go中手动添加路由
	log.InfoF(log.ModuleAccount, "Account routes initialized")
}
