package http

import (
	"config"
	"control"
	"encoding/json"
	t "html/template"
	"module"
	log "mylog"
	h "net/http"
	"path/filepath"
	"strings"
)

// HandleConfig handles the system configuration page
// 系统配置页面处理
func HandleConfig(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleConfig", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/main", h.StatusSeeOther)
		return
	}

	// 渲染配置页面模板
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "config.template"))
	if err != nil {
		log.ErrorF(log.ModuleConfig, "Failed to parse config.template: %s", err.Error())
		h.Error(w, "Failed to parse config template", h.StatusInternalServerError)
		return
	}

	data := struct {
		Title string
	}{
		Title: "系统配置管理",
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.ErrorF(log.ModuleConfig, "Failed to render config.template: %s", err.Error())
		h.Error(w, "Failed to render config template", h.StatusInternalServerError)
		return
	}
}

// HandleConfigAPI handles the system configuration API with per-account support
// 系统配置API处理 - 支持多账户
func HandleConfigAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleConfigAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	account := getAccountFromRequest(r)
	isAdmin := isAdminUser(account)

	switch r.Method {
	case h.MethodGet:
		handleGetConfig(w, account, isAdmin)
	case h.MethodPost:
		handleUpdateConfig(w, r, account, isAdmin)
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// handleGetConfig handles GET requests for configuration
func handleGetConfig(w h.ResponseWriter, account string, isAdmin bool) {
	// 获取账户专属的系统配置
	configTitle := config.GetSysConfigTitle()
	blog := control.GetBlog(account, configTitle)

	if blog == nil {
		// 创建默认配置
		createDefaultConfig(w, account, configTitle, isAdmin)
		return
	}

	// 解析现有配置内容和注释
	configs, comments := parseConfigContentWithComments(blog.Content)

	response := map[string]interface{}{
		"success":     true,
		"configs":     configs,
		"comments":    comments,
		"raw_content": blog.Content,
		"is_default":  false,
		"is_admin":    isAdmin,
		"account":     account,
	}
	json.NewEncoder(w).Encode(response)
}

// createDefaultConfig creates default configuration for an account
func createDefaultConfig(w h.ResponseWriter, account, configTitle string, isAdmin bool) {
	log.InfoF(log.ModuleConfig, "%s配置文件不存在，创建默认配置文件", configTitle)

	// 根据用户类型创建不同的默认配置文件
	var defaultConfigs map[string]string
	var defaultComments map[string]string

	if isAdmin {
		// 管理员获得完整的系统配置
		defaultConfigs = map[string]string{
			"port":                       "8888",
			"redis_ip":                   "127.0.0.1",
			"redis_port":                 "6666",
			"redis_pwd":                  "",
			"publictags":                 "public|share|demo",
			"sysfiles":                   configTitle,
			"title_auto_add_date_suffix": "日记",
			"diary_keywords":             "日记_",
			"diary_password":             "",
			"main_show_blogs":            "10",
		}

		defaultComments = map[string]string{
			"port":                       "HTTP服务监听端口",
			"redis_ip":                   "Redis服务器IP地址",
			"redis_port":                 "Redis服务器端口",
			"redis_pwd":                  "Redis密码（留空表示无密码）",
			"publictags":                 "公开标签列表（用|分隔）",
			"sysfiles":                   "系统文件列表（用|分隔）",
			"title_auto_add_date_suffix": "自动添加日期后缀的标题前缀（用|分隔）",
			"diary_keywords":             "日记关键字（用|分隔）",
			"diary_password":             "日记密码保护",
			"main_show_blogs":            "主页显示博客数量",
		}
	} else {
		// 非管理员只获得个人配置
		defaultConfigs = map[string]string{
			"publictags":                 "public|share|demo",
			"title_auto_add_date_suffix": "日记",
			"diary_keywords":             "日记_",
			"diary_password":             "",
			"main_show_blogs":            "10",
		}

		defaultComments = map[string]string{
			"publictags":                 "公开标签列表（用|分隔）",
			"title_auto_add_date_suffix": "自动添加日期后缀的标题前缀（用|分隔）",
			"diary_keywords":             "日记关键字（用|分隔）",
			"diary_password":             "日记密码保护",
			"main_show_blogs":            "主页显示博客数量",
		}
	}

	// 构建默认配置内容
	defaultContent := buildConfigContentWithComments(defaultConfigs, defaultComments)

	// 创建默认配置文件
	uploadData := &module.UploadedBlogData{
		Title:    configTitle,
		Content:  defaultContent,
		AuthType: module.EAuthType_private,
		Tags:     "system,config",
		Encrypt:  0,
	}

	result := control.AddBlog(account, uploadData)
	if result != 0 {
		log.ErrorF(log.ModuleConfig, "创建默认配置文件失败: result=%d", result)
		h.Error(w, "创建默认配置文件失败", h.StatusInternalServerError)
		return
	}

	// 更新账户的配置actor
	config.UpdateConfigFromBlog(account, defaultContent)

	log.InfoF(log.ModuleConfig, "账户 %s 的默认配置文件创建成功", account)

	// 返回默认配置
	response := map[string]interface{}{
		"success":     true,
		"configs":     defaultConfigs,
		"comments":    defaultComments,
		"raw_content": defaultContent,
		"is_default":  true,
		"is_admin":    isAdmin,
		"account":     account,
	}
	json.NewEncoder(w).Encode(response)
}

// handleUpdateConfig handles POST requests for configuration updates
func handleUpdateConfig(w h.ResponseWriter, r *h.Request, account string, isAdmin bool) {
	configTitle := config.GetSysConfigTitle()
	var requestData struct {
		Configs  map[string]string `json:"configs"`
		Comments map[string]string `json:"comments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.ErrorF(log.ModuleConfig, "解析配置数据失败: %v", err)
		h.Error(w, "无效的JSON数据", h.StatusBadRequest)
		return
	}

	// 构建新的配置内容
	newContent := buildConfigContentWithComments(requestData.Configs, requestData.Comments)

	// 更新账户专属的配置文件
	uploadData := &module.UploadedBlogData{
		Title:    configTitle,
		Content:  newContent,
		AuthType: module.EAuthType_private,
		Tags:     "system,config",
		Encrypt:  0,
	}

	result := control.ModifyBlog(account, uploadData)
	if result != 0 {
		log.ErrorF(log.ModuleConfig, "更新配置文件失败: result=%d", result)
		h.Error(w, "更新配置失败", h.StatusInternalServerError)
		return
	}

	// 更新账户的配置actor
	config.UpdateConfigFromBlog(account, newContent)

	log.InfoF(log.ModuleConfig, "用户 %s 的配置更新成功", account)

	response := map[string]interface{}{
		"success":  true,
		"message":  "配置更新成功",
		"is_admin": isAdmin,
		"account":  account,
	}
	json.NewEncoder(w).Encode(response)
}

// isAdminUser 检查用户是否为管理员
func isAdminUser(account string) bool {
	adminAccount := config.GetAdminAccount()
	return account == adminAccount
}

// parseConfigContent parses configuration content into key-value pairs
func parseConfigContent(content string) map[string]string {
	configs := make(map[string]string)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过注释和空行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析key=value格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			configs[key] = value
		}
	}

	return configs
}

// buildConfigContent builds configuration content from key-value pairs
func buildConfigContent(configs map[string]string) string {
	var content strings.Builder

	content.WriteString("# 系统配置文件\n")
	content.WriteString("# 修改后需要重启服务生效\n\n")

	for key, value := range configs {
		content.WriteString(key + "=" + value + "\n")
	}

	return content.String()
}

// parseConfigContentWithComments parses configuration content with comments
func parseConfigContentWithComments(content string) (map[string]string, map[string]string) {
	configs := make(map[string]string)
	comments := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentComment string
	var currentGroup string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 处理注释行
		if strings.HasPrefix(line, "#") {
			commentText := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if commentText != "" {
				// 检查是否是组注释
				if isGroupComment(commentText) {
					currentGroup = commentText
				} else {
					currentComment = commentText
				}
			}
			continue
		}

		// 跳过空行
		if line == "" {
			continue
		}

		// 解析key=value格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			configs[key] = value

			// 保存注释
			if currentComment != "" {
				comments[key] = currentComment
				currentComment = ""
			} else if currentGroup != "" {
				comments[key] = currentGroup
			}
		}
	}

	return configs, comments
}

// isGroupComment checks if a comment is a group comment
func isGroupComment(comment string) bool {
	groupKeywords := []string{"配置", "设置", "参数", "选项"}
	for _, keyword := range groupKeywords {
		if strings.Contains(comment, keyword) && !strings.Contains(comment, "=") {
			return true
		}
	}
	return false
}

// buildConfigContentWithComments builds configuration content with comments
func buildConfigContentWithComments(configs map[string]string, comments map[string]string) string {
	var content strings.Builder

	content.WriteString("# 系统配置文件\n")
	content.WriteString("# 修改后需要重启服务生效\n\n")

	// 定义配置项的顺序（按逻辑分类排列）
	configOrder := []string{
		// 基础设置
		"port", "pwd", "admin", "logs_dir", "statics_path", "templates_path", "download_path", "recycle_path",
		// Redis 缓存
		"redis_ip", "redis_port", "redis_pwd",
		// 博客设置
		"publictags", "sysfiles", "main_show_blogs", "max_blog_comments", "share_days", "help_blog_name",
		// 日记设置
		"title_auto_add_date_suffix", "diary_keywords", "diary_password",
		// AI / LLM
		"openai_api_key", "openai_api_url", "deepseek_api_key", "deepseek_api_url",
		"qwen_api_key", "qwen_api_url", "llm_fallback_models", "assistant_save_mcp_result",
		// CodeGen 编码
		"codegen_workspace", "codegen_claude_path", "codegen_max_turns", "codegen_mode", "codegen_agent_token",
		// 企业微信
		"wechat_corp_id", "wechat_secret", "wechat_agent_id", "wechat_token", "wechat_encoding_aes_key", "wechat_webhook",
		// 邮件通知
		"smtp_host", "smtp_port", "email_from", "email_password", "email_to", "sms_phone", "sms_send_url",
	}

	// 按顺序输出配置项
	for _, key := range configOrder {
		if value, exists := configs[key]; exists {
			// 写入注释
			if comment, hasComment := comments[key]; hasComment {
				content.WriteString("# " + comment + "\n")
			}
			// 写入配置项
			content.WriteString(key + "=" + value + "\n\n")
		}
	}

	// 输出其他配置项（不在预定义顺序中的）
	for key, value := range configs {
		found := false
		for _, orderedKey := range configOrder {
			if key == orderedKey {
				found = true
				break
			}
		}
		if !found {
			// 写入注释
			if comment, hasComment := comments[key]; hasComment {
				content.WriteString("# " + comment + "\n")
			}
			// 写入配置项
			content.WriteString(key + "=" + value + "\n\n")
		}
	}

	return content.String()
}
