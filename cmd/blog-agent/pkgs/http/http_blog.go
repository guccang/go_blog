package http

import (
	"blog"
	"config"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	h "net/http"
	"net/url"
	"persistence"
	"regexp"
	control "service"
	"share"
	"strconv"
	"strings"
)

// sanitizeBlogTitle 强制格式化博客标题，将不符合规范的字符替换为下划线
// 只保留中文、字母、数字、下划线、点、连字符
func sanitizeBlogTitle(title string) string {
	if title == "" {
		return ""
	}
	// 替换所有不符合规范的字符为下划线
	invalidChars := regexp.MustCompile(`[^\p{Han}a-zA-Z0-9\._-]`)
	title = invalidChars.ReplaceAllString(title, "_")
	// 合并连续下划线
	multiUnderscores := regexp.MustCompile(`_+`)
	title = multiUnderscores.ReplaceAllString(title, "_")
	// 去除首尾下划线
	title = strings.Trim(title, "_")
	return title
}

// HandleSave handles blog saving functionality
func HandleSave(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleSave", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 设置请求体大小限制
	r.ParseMultipartForm(32 << 20) // 32MB

	// 获取单个字段值
	title := r.FormValue("title")
	// 强制格式化博客标题，不符合规范的字符替换为下划线
	original := title
	title = sanitizeBlogTitle(title)
	if title == "" {
		h.Error(w, "save failed! title is empty after sanitization!", h.StatusBadRequest)
		return
	}
	if original != title {
		log.InfoF(log.ModuleBlog, "blog title sanitized: [%s] -> [%s]", original, title)
	}

	log.DebugF(log.ModuleBlog, "title:%s", title)

	content := r.FormValue("content")
	// 在这里，您可以处理或保存content到数据库等
	log.DebugF(log.ModuleBlog, "Received content:%s", content)

	// 解析权限设置
	auth_type_string := r.FormValue("authtype")
	log.DebugF(log.ModuleBlog, "Received authtype:%s", auth_type_string)

	// 解析权限组合
	auth_type := parseAuthTypeString(auth_type_string)

	// tags
	tags := r.FormValue("tags")
	log.DebugF(log.ModuleBlog, "Received tags:%s", tags)

	// encrypt
	encryptionKey := r.FormValue("encrypt")
	encrypt := 0
	log.DebugF(log.ModuleBlog, "Received title=%s encrypt:%s", title, encryptionKey)

	//
	if encryptionKey != "" {
		encrypt = 1
		/*
			// aes加密
			log.DebugF(log.ModuleBlog, "encryption key=%s",encryptionKey)
			content_encrypt  := encryption.AesSimpleEncrypt(content, encryptionKey);

			content_decrypt := encryption.AesSimpleDecrypt(content_encrypt, encryptionKey);
			log.DebugF(log.ModuleBlog, "encryption content_decrypt=%s",content_encrypt)
			if content_decrypt != content {
				h.Error(w, "save failed! aes not match error!", h.StatusBadRequest)
				return
			}
			log.DebugF(log.ModuleBlog, "content encrypt=%s\n",content)
			// 邮件备份密码,todo
			content = content_encrypt
		*/
	}

	account := getAccountFromRequest(r)

	ubd := module.UploadedBlogData{
		Title:    title,
		Content:  content,
		AuthType: auth_type,
		Tags:     tags,
		Encrypt:  encrypt,
		Account:  account,
	}

	ret := control.AddBlog(account, &ubd)

	// 如果保存的是提示词配置，自动重载
	if ret == 0 && title == config.GetSysPromptsConfigTitle() {
		config.ReloadPrompts(account)
	}

	// 响应客户端
	if ret == 0 {
		hookType := blog.HookBlogCreated
		if (auth_type&module.EAuthType_diary) != 0 || config.IsDiaryBlogWithAccount(account, title) {
			hookType = blog.HookDiaryWritten
		}
		emitUsageHook(r, account, hookType, "blog_editor", "blog", title, title, "", nil, map[string]any{"status": "success"})
		w.Write([]byte(fmt.Sprintf("save successfully! ret=%d", ret)))
	} else {
		h.Error(w, "save failed! has same title blog", h.StatusBadRequest)
	}
}

// HandleHelp handles help page requests
func HandleHelp(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleHelp", r)
	blogname := config.GetHelpBlogName()
	if blogname == "" {
		blogname = "help"
	}

	log.DebugF(log.ModuleBlog, "help blogname=%s", blogname)

	account := getAccountFromRequest(r)
	usepublic := 0
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) != 0 {
		// 判定blog访问权限
		auth_type := control.GetBlogAuthType(account, blogname)
		if auth_type == module.EAuthType_private {
			h.Redirect(w, r, "/index", 302)
			return
		} else {
			usepublic = 1
		}
	}

	emitUsageHook(r, account, blog.HookPageOpened, "help", "page", blogname, blogname, "", nil, map[string]any{"status": "success"})
	emitUsageHook(r, account, blog.HookPageOpened, "blog_reader", "blog", blogname, blogname, "", map[string]any{"public": usepublic != 0}, map[string]any{"status": "success"})
	view.PageGetBlog(blogname, w, usepublic, account)
}

// HandleGetShare handles shared blog/tag access
// 使用@share c blogname 标签获取分享链接和密码
// 访问分享，使用链接和密码
func HandleGetShare(w h.ResponseWriter, r *h.Request) {
	r.ParseMultipartForm(32 << 20) // 32MB
	// t
	t, _ := strconv.Atoi(r.URL.Query().Get("t"))
	name := r.URL.Query().Get("name")
	pwd := r.URL.Query().Get("pwd")
	account := getAccountFromRequest(r)

	if t == 0 {
		// blog
		blog := share.GetSharedBlog(name)
		if blog == nil {
			h.Error(w, "HandleGetShared error blogname", h.StatusBadRequest)
			return
		}
		if blog.Pwd != pwd {
			h.Error(w, "HandleGetShared error pwd", h.StatusBadRequest)
			return
		}
		cnt := share.ModifyCntSharedBlog(name, -1)
		if cnt < 0 {
			h.Error(w, "HandleGetShared error cnt < 0", h.StatusBadRequest)
			return
		}
		usepublic := 1
		view.PageGetBlog(name, w, usepublic, account)
	} else if t == 1 {
		// tag
		tag := share.GetSharedTag(name)
		if tag == nil {
			h.Error(w, "HandleGetShared error tagname", h.StatusBadRequest)
			return
		}
		if tag.Pwd != pwd {
			h.Error(w, "HandleGetShared error pwd", h.StatusBadRequest)
			return
		}
		cnt := share.ModifyCntSharedTag(name, -1)
		if cnt < 0 {
			h.Error(w, "HandleGetShared error cnt < 0", h.StatusBadRequest)
			return
		}
		view.PageTags(w, name, "")
	}
}

// HandleGet handles blog retrieval with various redirections and permissions
func HandleGet(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleGet", r)
	blogname := r.URL.Query().Get("blogname")
	if blogname == "" {
		h.Error(w, "blogname parameter is missing", h.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)

	// Check if account is specified in URL (for public blog access)
	urlAccount := r.URL.Query().Get("account")
	if urlAccount != "" {
		account = urlAccount
	}

	// 首先获取博客信息以检查权限
	var blog *module.Blog
	if account != "" {
		blog = control.GetBlog(account, blogname)
	}
	// 兼容未携带 account 的公开链接：当前账号找不到时，只跨账号查找可公开访问的同名文章。
	if blog == nil && urlAccount == "" {
		accounts, err := persistence.FindPublicBlogAccountsByTitle(blogname)
		if err != nil {
			log.ErrorF(log.ModuleBlog, "resolve public blog account failed title=%s: %v", blogname, err)
			h.Error(w, "failed to resolve public blog", h.StatusInternalServerError)
			return
		}
		if len(accounts) > 1 {
			h.Error(w, "multiple public blogs use this title; add the account parameter", h.StatusConflict)
			return
		}
		if len(accounts) == 1 {
			account = accounts[0]
			blog = control.GetBlog(account, blogname)
		}
	}
	if blog == nil {
		h.Error(w, fmt.Sprintf("blogname=%s not find", blogname), h.StatusNotFound)
		return
	}

	// 检查是否设置了日记权限，如果是则需要密码验证
	if (blog.AuthType & module.EAuthType_diary) != 0 {
		// 检查是否提供了密码
		diaryPassword := r.URL.Query().Get("diary_pwd")
		if diaryPassword == "" {
			// 没有提供密码，显示密码输入页面
			view.PageDiaryPasswordInput(w, blogname)
			return
		}

		// 验证密码
		expectedPassword := config.GetConfigWithAccount(account, "diary_password")
		if expectedPassword == "" {
			expectedPassword = "diary123" // 默认密码
		}

		if diaryPassword != expectedPassword {
			// 密码错误，返回错误页面
			view.PageDiaryPasswordError(w, blogname)
			return
		}

		// 密码正确，继续处理
		log.DebugF(log.ModuleBlog, "日记博客密码验证成功: %s (AuthType: %d)", blogname, blog.AuthType)
	}

	// 兼容性：同时检查基于名称的日记博客（向后兼容）
	if config.IsDiaryBlogWithAccount(account, blogname) && (blog.AuthType&module.EAuthType_diary) == 0 {
		// 检查是否提供了密码
		diaryPassword := r.URL.Query().Get("diary_pwd")
		if diaryPassword == "" {
			// 没有提供密码，显示密码输入页面
			view.PageDiaryPasswordInput(w, blogname)
			return
		}

		// 验证密码
		expectedPassword := config.GetConfigWithAccount(account, "diary_password")
		if expectedPassword == "" {
			expectedPassword = "diary123" // 默认密码
		}

		if diaryPassword != expectedPassword {
			// 密码错误，返回错误页面
			view.PageDiaryPasswordError(w, blogname)
			return
		}

		// 密码正确，继续处理
		log.DebugF(log.ModuleBlog, "传统日记博客密码验证成功: %s", blogname)
	}

	// 保留旧待办博客的访问兼容性，统一跳转到同一天的日目标。
	if strings.HasPrefix(blogname, "todolist-") {
		// 从blogname中解析出日期，格式为todolist-YYYY-MM-DD
		date := strings.TrimPrefix(blogname, "todolist-")
		// 验证日期格式是否正确
		if len(date) == 10 && date[4] == '-' && date[7] == '-' {
			h.Redirect(w, r, fmt.Sprintf("/goal?level=daily&period=%s", date), 302)
			return
		}
		h.Redirect(w, r, "/goal", 302)
		return
	}

	// 检查是否是 yearplan 博客，如果是则重定向到 goal 页面
	if strings.HasPrefix(blogname, "年计划_") {
		h.Redirect(w, r, "/goal", 302)
		return
	}

	// 检查是否是 exercise 博客，如果是则重定向到 exercise 页面
	if strings.HasPrefix(blogname, "exercise-") {
		// 从blogname中解析出日期，格式为exercise-YYYY-MM-DD
		date := strings.TrimPrefix(blogname, "exercise-")
		// 验证日期格式是否正确
		if len(date) == 10 && date[4] == '-' && date[7] == '-' {
			// 重定向到exercise页面，并传递date参数
			h.Redirect(w, r, fmt.Sprintf("/exercise?date=%s", date), 302)
			return
		}
		// 如果日期格式不正确，则使用默认重定向
		h.Redirect(w, r, "/exercise", 302)
		return
	}

	// 检查是否是 月度目标 博客，如果是则重定向到 goal 页面
	if strings.HasPrefix(blogname, "月度目标_") {
		h.Redirect(w, r, "/goal", 302)
		return
	}

	// 检查是否是 reading_book 博客，如果是则重定向到 reading 页面
	if strings.HasPrefix(blogname, "reading_book_") {
		// 从blogname中解析出书名，格式为reading_book_书名.md
		bookTitle := strings.TrimSuffix(strings.TrimPrefix(blogname, "reading_book_"), ".md")
		// 重定向到reading页面，并传递book参数
		h.Redirect(w, r, fmt.Sprintf("/reading?book=%s", bookTitle), 302)
		return
	}

	// 检查是否是 constellation 博客，如果是则重定向到 constellation 页面
	if strings.HasPrefix(blogname, "horoscope-") || strings.HasPrefix(blogname, "constellation-") {
		// 重定向到constellation页面
		h.Redirect(w, r, "/constellation", 302)
		return
	}

	usepublic := 0
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) != 0 {
		// 判定blog访问权限 - 直接使用已获取的blog对象
		auth_type := blog.AuthType
		// 如果博客有公开权限，允许访问（只读模式）
		if (auth_type & module.EAuthType_public) != 0 {
			usepublic = 1
		} else if (auth_type & module.EAuthType_private) != 0 {
			// 只有私有权限，未登录用户重定向到登录页面
			h.Redirect(w, r, "/index", 302)
			return
		} else {
			// 既不是公开也不是私有权限，重定向
			h.Redirect(w, r, "/index", 302)
			return
		}
	}

	// 记录博客访问
	if blogname != "" {
		remoteAddr := r.RemoteAddr
		xForwardedFor := r.Header.Get("X-Forwarded-For")
		if xForwardedFor != "" {
			remoteAddr = xForwardedFor
		}
		userAgent := r.Header.Get("User-Agent")
		_ = remoteAddr
		_ = userAgent
	}

	view.PageGetBlog(blogname, w, usepublic, account)
}

// HandleDelete handles blog deletion
func HandleDelete(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleDelete", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 设置请求体大小限制
	r.ParseMultipartForm(32 << 20) // 32MB

	// 获取单个字段值
	title := r.FormValue("title")
	log.DebugF(log.ModuleBlog, "delete title:%s", title)

	account := getAccountFromRequest(r)

	current := control.GetBlog(account, title)
	ret := control.DeleteBlog(account, title)
	if ret == 0 {
		if current != nil && ((current.AuthType&module.EAuthType_diary) != 0 || config.IsDiaryBlogWithAccount(account, title)) {
			emitUsageHook(r, account, blog.HookDiaryDeleted, "blog_editor", "diary", title, title, "", nil, map[string]any{"status": "success"})
		}
		w.Write([]byte(fmt.Sprintf("Content received successfully! ret=%d", ret)))
	} else {
		w.Write([]byte(fmt.Sprintf("Content received failed! ret=%d", ret)))
	}
}

// HandleModify handles blog modification
func HandleModify(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleModify", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	account := getAccountFromRequest(r)

	// 设置请求体大小限制
	r.ParseMultipartForm(32 << 20) // 32MB

	// 获取单个字段值
	title := r.FormValue("title")
	log.DebugF(log.ModuleBlog, "title:%s", title)

	// 解析权限设置
	auth_type_string := r.FormValue("auth_type")
	log.DebugF(log.ModuleBlog, "Received auth_type:%s", auth_type_string)

	// 解析权限组合
	auth_type := parseAuthTypeString(auth_type_string)

	// tags
	tags := r.FormValue("tags")
	log.DebugF(log.ModuleBlog, "Received tags:%s", tags)

	// 内容
	content := r.FormValue("content")
	// 在这里，您可以处理或保存content到数据库等
	//log.DebugF("Received content:%s", content)

	// 加密
	encryptionKey := r.FormValue("encrypt")
	encrypt := 0
	log.DebugF(log.ModuleBlog, "Received title=%s encrypt:%t", title, encryptionKey != "")

	if encryptionKey != "" {
		encrypt = 1
		/*
			// aes加密
			log.DebugF("encryption key=%s",encryptionKey)
			content_encrypt  := encryption.AesSimpleEncrypt(content, encryptionKey);

			content_decrypt := encryption.AesSimpleDecrypt(content_encrypt, encryptionKey);
			log.DebugF(log.ModuleBlog, "encryption content_decrypt=%s",content_encrypt)
			if content_decrypt != content {
				h.Error(w, "save failed! aes not match error!", h.StatusBadRequest)
				return
			}
			log.DebugF(log.ModuleBlog, "content encrypt=%s\n",content)

			content = content_encrypt
		*/
		// 邮件备份密码,todo
	}

	ubd := module.UploadedBlogData{
		Title:    title,
		Content:  content,
		AuthType: auth_type,
		Tags:     tags,
		Encrypt:  encrypt,
		Account:  account,
	}

	ret := control.ModifyBlog(account, &ubd)
	if ret == 0 && ((auth_type&module.EAuthType_diary) != 0 || config.IsDiaryBlogWithAccount(account, title)) {
		emitUsageHook(r, account, blog.HookDiaryWritten, "blog_editor", "diary", title, title, "", nil, map[string]any{"status": "success"})
	}

	// 响应客户端
	w.Write([]byte(fmt.Sprintf("Content received successfully! ret=%d", ret)))

}

// HandleSearch handles blog search functionality
func HandleSearch(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleSearch", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	match := r.URL.Query().Get("match")
	account := getAccountFromRequest(r)
	if strings.HasPrefix(strings.TrimSpace(match), "@") {
		ret := view.PageSearchNormal(match, w, r)
		if ret != 0 {
			view.PageSearch(match, w, account)
		}
		return
	}
	PageSearchResults(w, account, match)
}

// HandleTag handles tag-based blog listing
func HandleTag(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleTag", r)

	r.ParseMultipartForm(32 << 20) // 32MB

	tag := r.FormValue("tag")

	isTagPublic := config.IsPublicTag(tag)
	log.DebugF(log.ModuleBlog, "HandleTag %s %d", tag, isTagPublic)
	if isTagPublic != 1 {
		if checkLogin(r) != 0 {
			h.Redirect(w, r, "/index", 302)
			return
		}
	}

	// 展示所有public tag
	view.PageTags(w, tag, getAccountFromRequest(r))
}

// HandlePublic renders the public blogs page
func HandlePublic(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandlePublic", r)
	account := r.URL.Query().Get("account")
	if account == "" {
		account = getAccountFromRequest(r)
	}
	if account == "" {
		account = config.GetAdminAccount()
	}
	view.PagePublic(w, account)
}

// HandleCreateShare creates a share link for a blog
func HandleCreateShare(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleCreateShare", r)

	// 检查登录状态
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 解析请求
	r.ParseForm()
	blogname := r.FormValue("blogname")
	if blogname == "" {
		h.Error(w, "blogname parameter is missing", h.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	// 检查博客是否存在
	blog := control.GetBlog(account, blogname)
	if blog == nil {
		h.Error(w, fmt.Sprintf("Blog %s not found", blogname), h.StatusBadRequest)
		return
	}

	// 构建完整的URL（包含域名和协议）
	host := r.Host
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwardedProto == "http" || forwardedProto == "https" {
		scheme = forwardedProto
	}

	// 公开、未加密、非日记博客直接生成带账号的永久访问链接。
	if blog.Encrypt == 0 && (blog.AuthType&module.EAuthType_public) != 0 && (blog.AuthType&(module.EAuthType_diary|module.EAuthType_encrypt)) == 0 {
		fullURL := publicBlogShareURL(scheme, host, blogname, account)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(h.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true, "url": fullURL, "pwd": "", "blogname": blogname, "account": account, "mode": "public",
		})
		return
	}

	// 非公开内容沿用有密码和次数限制的临时分享链接。
	shareURL, pwd := share.AddSharedBlog(blogname)
	fullURL := fmt.Sprintf("%s://%s%s", scheme, host, shareURL)

	// 返回JSON响应
	response := map[string]interface{}{
		"success":  true,
		"url":      fullURL,
		"pwd":      pwd,
		"blogname": blogname,
		"account":  account,
		"mode":     "protected",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func publicBlogShareURL(scheme, host, blogname, account string) string {
	return fmt.Sprintf("%s://%s/get?blogname=%s&account=%s", scheme, host, url.QueryEscape(blogname), url.QueryEscape(account))
}

// HandleMigration handles migration page display
func HandleMigration(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleMigration", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	view.PageMigration(w)
}

// HandleMigrationExport handles blog data export
func HandleMigrationExport(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleMigrationExport", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	account := getAccountFromRequest(r)

	// 获取所有博客数据
	blogs := control.GetBlogs(account)
	if blogs == nil {
		h.Error(w, "Failed to get blogs", h.StatusInternalServerError)
		return
	}

	// 构建导出数据结构
	exportData := make([]map[string]interface{}, 0)

	for _, blog := range blogs {
		blogData := map[string]interface{}{
			"title":       blog.Title,
			"create_time": blog.CreateTime,
			"modify_time": blog.ModifyTime,
			"access_time": blog.AccessTime,
			"modify_num":  blog.ModifyNum,
			"access_num":  blog.AccessNum,
			"auth_type":   blog.AuthType,
			"tags":        blog.Tags,
			"encrypt":     blog.Encrypt,
			"account":     blog.Account,
		}
		exportData = append(exportData, blogData)
	}

	// 转换为JSON
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		h.Error(w, "Failed to marshal JSON: "+err.Error(), h.StatusInternalServerError)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=sys_blog_exdata.md")
	w.WriteHeader(h.StatusOK)

	// 写入文件内容
	w.Write([]byte("# 博客元数据导出文件\n\n"))
	w.Write([]byte("此文件包含博客的元数据信息，不包含内容\n\n"))
	w.Write([]byte("```json\n"))
	w.Write(jsonData)
	w.Write([]byte("\n```\n"))
}

// HandleMigrationImport handles blog data import
func HandleMigrationImport(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleMigrationImport", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}

	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 解析上传的文件
	err := r.ParseMultipartForm(10 << 20) // 10MB limit
	if err != nil {
		h.Error(w, "Failed to parse form: "+err.Error(), h.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		h.Error(w, "Failed to get uploaded file: "+err.Error(), h.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(handler.Filename, ".md") {
		h.Error(w, "Only .md files are allowed", h.StatusBadRequest)
		return
	}

	// 读取文件内容
	buffer := make([]byte, handler.Size)
	_, err = file.Read(buffer)
	if err != nil {
		h.Error(w, "Failed to read file: "+err.Error(), h.StatusInternalServerError)
		return
	}

	content := string(buffer)

	// 提取JSON数据
	startMarker := "```json"
	endMarker := "```"
	startIndex := strings.Index(content, startMarker)
	if startIndex == -1 {
		h.Error(w, "Invalid file format: JSON block not found", h.StatusBadRequest)
		return
	}

	startIndex += len(startMarker)
	endIndex := strings.Index(content[startIndex:], endMarker)
	if endIndex == -1 {
		h.Error(w, "Invalid file format: JSON block not closed", h.StatusBadRequest)
		return
	}

	jsonContent := strings.TrimSpace(content[startIndex : startIndex+endIndex])

	// 解析JSON数据
	var importData []map[string]interface{}
	err = json.Unmarshal([]byte(jsonContent), &importData)
	if err != nil {
		h.Error(w, "Failed to parse JSON: "+err.Error(), h.StatusBadRequest)
		return
	}

	account := getAccountFromRequest(r)
	updatedCount := 0
	skippedCount := 0

	// 更新博客数据
	for _, data := range importData {
		title, ok := data["title"].(string)
		if !ok || title == "" {
			continue
		}

		// 检查博客是否存在
		existingBlog := control.GetBlog(account, title)
		if existingBlog == nil {
			skippedCount++
			continue
		}

		// 更新博客元数据（不包括content）
		if createTime, ok := data["create_time"].(string); ok && createTime != "" {
			existingBlog.CreateTime = createTime
		}
		if modifyTime, ok := data["modify_time"].(string); ok && modifyTime != "" {
			existingBlog.ModifyTime = modifyTime
		}
		if accessTime, ok := data["access_time"].(string); ok && accessTime != "" {
			existingBlog.AccessTime = accessTime
		}
		if modifyNum, ok := data["modify_num"].(float64); ok {
			existingBlog.ModifyNum = int(modifyNum)
		}
		if accessNum, ok := data["access_num"].(float64); ok {
			existingBlog.AccessNum = int(accessNum)
		}
		if authType, ok := data["auth_type"].(float64); ok {
			existingBlog.AuthType = int(authType)
		}
		if tags, ok := data["tags"].(string); ok {
			existingBlog.Tags = tags
		}
		if encrypt, ok := data["encrypt"].(float64); ok {
			existingBlog.Encrypt = int(encrypt)
		}

		// 保存更新的博客
		ubd := module.UploadedBlogData{
			Title:    existingBlog.Title,
			Content:  existingBlog.Content,
			AuthType: existingBlog.AuthType,
			Tags:     existingBlog.Tags,
			Encrypt:  existingBlog.Encrypt,
			Account:  account,
		}

		ret := control.ModifyBlog(account, &ubd)
		if ret == 0 {
			updatedCount++
		}
	}

	// 返回结果
	result := fmt.Sprintf("导入完成! 更新了 %d 个博客，跳过 %d 个不存在的博客", updatedCount, skippedCount)
	w.WriteHeader(h.StatusOK)
	w.Write([]byte(result))
}

// findRootTaskID 查找任务的根任务ID
func findRootTaskID(account, taskID, blogContent string) string {
	// 解析当前任务的parent_id
	parentID := extractParentIDFromContent(blogContent)
	if parentID == "" {
		// 没有parent，当前任务就是根任务
		return taskID
	}

	// 递归查找父任务的根任务
	// 防止循环引用，设置最大递归深度
	return findRootTaskIDRecursive(account, parentID, 0)
}

// extractParentIDFromContent 从博客内容中提取parent_id
func extractParentIDFromContent(content string) string {
	// 尝试解析JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		// 可能内容不是直接的JSON，尝试从markdown代码块中提取
		// 查找 ```json 代码块
		startMarker := "```json"
		endMarker := "```"
		startIndex := strings.Index(content, startMarker)
		if startIndex != -1 {
			startIndex += len(startMarker)
			endIndex := strings.Index(content[startIndex:], endMarker)
			if endIndex != -1 {
				jsonContent := strings.TrimSpace(content[startIndex : startIndex+endIndex])
				if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
					return ""
				}
			}
		} else {
			// 不是JSON格式，返回空
			return ""
		}
	}

	// 提取parent_id字段
	if parentID, ok := data["parent_id"].(string); ok && parentID != "" {
		return parentID
	}
	return ""
}

// findRootTaskIDRecursive 递归查找根任务ID
func findRootTaskIDRecursive(account, taskID string, depth int) string {
	// 防止循环引用和无限递归，设置最大深度
	const maxDepth = 100
	if depth > maxDepth {
		return taskID
	}

	// 获取任务博客
	blogName := fmt.Sprintf("taskbreakdown-%s", taskID)
	blog := control.GetBlog(account, blogName)
	if blog == nil {
		// 找不到博客，返回当前任务ID作为根
		return taskID
	}

	// 提取parent_id
	parentID := extractParentIDFromContent(blog.Content)
	if parentID == "" {
		// 没有parent，当前任务就是根任务
		return taskID
	}

	// 继续向上查找
	return findRootTaskIDRecursive(account, parentID, depth+1)
}
