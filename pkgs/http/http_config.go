package http

import (
	"config"
	"control"
	"encoding/json"
	h "net/http"
	t "html/template"
	"module"
	log "mylog"
	"path/filepath"
	"strings"
)

// HandleConfig handles the system configuration page
// 系统配置页面处理
func HandleConfig(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleConfig", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/login", h.StatusSeeOther)
		return
	}

	// 渲染配置页面模板
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "config.template"))
	if err != nil {
		log.ErrorF("Failed to parse config.template: %s", err.Error())
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
		log.ErrorF("Failed to render config.template: %s", err.Error())
		h.Error(w, "Failed to render config template", h.StatusInternalServerError)
		return
	}
}

// HandleConfigAPI handles the system configuration API
// 系统配置API处理
func HandleConfigAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleConfigAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case h.MethodGet:
		// 获取系统配置
		blog := control.GetBlog("", "sys_conf")
		if blog == nil {
			log.ErrorF("sys_conf文件不存在，创建默认配置文件")

			// 创建默认配置文件
			defaultConfigs := map[string]string{
				"port":                       "8888",
				"redis_ip":                   "127.0.0.1",
				"redis_port":                 "6666",
				"redis_pwd":                  "",
				"publictags":                 "public|share|demo",
				"sysfiles":                   "sys_conf",
				"title_auto_add_date_suffix": "日记",
				"diary_keywords":             "日记_",
			}

			// 构建默认配置内容和注释
			defaultComments := map[string]string{
				"port":                       "HTTP服务监听端口",
				"redis_ip":                   "Redis服务器IP地址",
				"redis_port":                 "Redis服务器端口",
				"redis_pwd":                  "Redis密码（留空表示无密码）",
				"publictags":                 "公开标签列表（用|分隔）",
				"sysfiles":                   "系统文件列表（用|分隔）",
				"title_auto_add_date_suffix": "自动添加日期后缀的标题前缀（用|分隔）",
				"diary_keywords":             "日记关键字（用|分隔）",
			}
			defaultContent := buildConfigContentWithComments(defaultConfigs, defaultComments)

			// 创建默认配置文件
			uploadData := &module.UploadedBlogData{
				Title:    "sys_conf",
				Content:  defaultContent,
				AuthType: module.EAuthType_private,
				Tags:     "system,config",
				Encrypt:  0,
			}

			result := control.AddBlog("", uploadData)
			if result != 0 {
				log.ErrorF("创建默认配置文件失败: result=%d", result)
				h.Error(w, "创建默认配置文件失败", h.StatusInternalServerError)
				return
			}

			log.DebugF("默认配置文件创建成功")

			// 返回默认配置
			response := map[string]interface{}{
				"success":     true,
				"configs":     defaultConfigs,
				"comments":    defaultComments,
				"raw_content": defaultContent,
				"is_default":  true,
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// 解析配置内容和注释
		configs, comments := parseConfigContentWithComments(blog.Content)

		response := map[string]interface{}{
			"success":     true,
			"configs":     configs,
			"comments":    comments,
			"raw_content": blog.Content,
			"is_default":  false,
		}
		json.NewEncoder(w).Encode(response)

	case h.MethodPost:
		// 更新系统配置
		var requestData struct {
			Configs  map[string]string `json:"configs"`
			Comments map[string]string `json:"comments"`
		}

		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			log.ErrorF("解析配置数据失败: %v", err)
			h.Error(w, "无效的JSON数据", h.StatusBadRequest)
			return
		}

		// 构建新的配置内容
		newContent := buildConfigContentWithComments(requestData.Configs, requestData.Comments)

		// 更新sys_conf文件
		uploadData := &module.UploadedBlogData{
			Title:    "sys_conf",
			Content:  newContent,
			AuthType: module.EAuthType_private,
			Tags:     "system,config",
			Encrypt:  0,
		}

		result := control.ModifyBlog("", uploadData)
		if result != 0 {
			log.ErrorF("更新配置文件失败: result=%d", result)
			h.Error(w, "更新配置失败", h.StatusInternalServerError)
			return
		}

		// 重新加载配置
		configPath := config.GetConfigPath()
		config.ReloadConfig(configPath)

		log.DebugF("系统配置更新成功")

		response := map[string]interface{}{
			"success": true,
			"message": "配置更新成功",
		}
		json.NewEncoder(w).Encode(response)

	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
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

	// 定义配置项的顺序
	configOrder := []string{
		"port",
		"redis_ip", 
		"redis_port",
		"redis_pwd",
		"publictags",
		"sysfiles",
		"title_auto_add_date_suffix",
		"diary_keywords",
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