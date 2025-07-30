package http

import (
	"comment"
	"config"
	"control"
	"cooperation"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	h "net/http"
	"regexp"
	"share"
	"strconv"
	"strings"
	"view"
)

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
	pattern := `^[\p{Han}a-zA-Z0-9\._-]+$`
	reg := regexp.MustCompile(pattern)
	match := reg.MatchString(title)
	if !match {
		h.Error(w, "save failed! title is invalied!", h.StatusBadRequest)
		return
	}

	log.DebugF("title:%s", title)

	content := r.FormValue("content")
	// 在这里，您可以处理或保存content到数据库等
	log.DebugF("Received content:%s", content)

	// 解析权限设置
	auth_type_string := r.FormValue("authtype")
	log.DebugF("Received authtype:%s", auth_type_string)

	// 解析权限组合
	auth_type := parseAuthTypeString(auth_type_string)

	// 如果是协作用户，自动添加协作权限
	if IsCooperation(r) {
		auth_type |= module.EAuthType_cooperation
	}

	// tags
	tags := r.FormValue("tags")
	log.DebugF("Received tags:%s", tags)

	// encrypt
	encryptionKey := r.FormValue("encrypt")
	encrypt := 0
	log.DebugF("Received title=%s encrypt:%s", title, encryptionKey)

	//
	if encryptionKey != "" {
		encrypt = 1
		/*
			// aes加密
			log.DebugF("encryption key=%s",encryptionKey)
			content_encrypt  := encryption.AesSimpleEncrypt(content, encryptionKey);

			content_decrypt := encryption.AesSimpleDecrypt(content_encrypt, encryptionKey);
			log.DebugF("encryption content_decrypt=%s",content_encrypt)
			if content_decrypt != content {
				h.Error(w, "save failed! aes not match error!", h.StatusBadRequest)
				return
			}
			fmt.Printf("content encrypt=%s\n",content)
			// 邮件备份密码,todo
			content = content_encrypt
		*/
	}

	ubd := module.UploadedBlogData{
		Title:    title,
		Content:  content,
		AuthType: auth_type,
		Tags:     tags,
		Encrypt:  encrypt,
	}

	if IsCooperation(r) {
		if config.IsTitleAddDateSuffix(title) == 1 {
			h.Error(w, "save failed! cooperation auth error,timed blog not support", h.StatusBadRequest)
			return
		}
	}

	ret := control.AddBlog(&ubd)

	// 响应客户端
	if ret == 0 {
		w.Write([]byte(fmt.Sprintf("save successfully! ret=%d", ret)))
		if IsCooperation(r) {
			session := getsession(r)
			cooperation.AddCanEditBlogBySession(session, title)
		}
	} else {
		h.Error(w, "save failed! has same title blog", h.StatusBadRequest)
	}
}

// HandleD3 handles D3 visualization page
func HandleD3(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleHelp", r)
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	view.PageD3(w)

}

// HandleHelp handles help page requests
func HandleHelp(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleHelp", r)
	blogname := config.GetHelpBlogName()
	if blogname == "" {
		blogname = "help"
	}

	log.DebugF("help blogname=", blogname)

	usepublic := 0
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) != 0 {
		// 判定blog访问权限
		auth_type := control.GetBlogAuthType(blogname)
		if auth_type == module.EAuthType_private {
			h.Redirect(w, r, "/index", 302)
			return
		} else {
			usepublic = 1
		}
	}

	view.PageGetBlog(blogname, w, usepublic)
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
		view.PageGetBlog(name, w, usepublic)
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
		view.PageTags(w, name)
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

	// 首先获取博客信息以检查权限
	blog := control.GetBlog(blogname)
	if blog == nil {
		h.Error(w, fmt.Sprintf("blogname=%s not find", blogname), h.StatusBadRequest)
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
		expectedPassword := config.GetConfig("diary_password")
		if expectedPassword == "" {
			expectedPassword = "diary123" // 默认密码
		}

		if diaryPassword != expectedPassword {
			// 密码错误，返回错误页面
			view.PageDiaryPasswordError(w, blogname)
			return
		}

		// 密码正确，继续处理
		log.DebugF("日记博客密码验证成功: %s (AuthType: %d)", blogname, blog.AuthType)
	}

	// 兼容性：同时检查基于名称的日记博客（向后兼容）
	if config.IsDiaryBlog(blogname) && (blog.AuthType&module.EAuthType_diary) == 0 {
		// 检查是否提供了密码
		diaryPassword := r.URL.Query().Get("diary_pwd")
		if diaryPassword == "" {
			// 没有提供密码，显示密码输入页面
			view.PageDiaryPasswordInput(w, blogname)
			return
		}

		// 验证密码
		expectedPassword := config.GetConfig("diary_password")
		if expectedPassword == "" {
			expectedPassword = "diary123" // 默认密码
		}

		if diaryPassword != expectedPassword {
			// 密码错误，返回错误页面
			view.PageDiaryPasswordError(w, blogname)
			return
		}

		// 密码正确，继续处理
		log.DebugF("传统日记博客密码验证成功: %s", blogname)
	}

	// 检查是否是 todolist 博客，如果是则重定向到 todolist 页面
	if strings.HasPrefix(blogname, "todolist-") {
		// 从blogname中解析出日期，格式为todolist-YYYY-MM-DD
		date := strings.TrimPrefix(blogname, "todolist-")
		// 验证日期格式是否正确
		if len(date) == 10 && date[4] == '-' && date[7] == '-' {
			// 重定向到todolist页面，并传递date参数
			h.Redirect(w, r, fmt.Sprintf("/todolist?date=%s", date), 302)
			return
		}
		// 如果日期格式不正确，则使用默认重定向
		h.Redirect(w, r, "/todolist", 302)
		return
	}

	// 检查是否是 yearplan 博客，如果是则重定向到 yearplan 页面
	if strings.HasPrefix(blogname, "年计划_") {
		// 重定向到yearplan页面，并传递date参数
		date := strings.TrimPrefix(blogname, "年计划_")
		h.Redirect(w, r, fmt.Sprintf("/yearplan?year=%s", date), 302)
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

	// 检查是否是 月度目标 博客，如果是则重定向到 monthgoal 页面
	if strings.HasPrefix(blogname, "月度目标_") {
		// 从blogname中解析出年月，格式为月度目标_YYYY-MM
		yearMonth := strings.TrimPrefix(blogname, "月度目标_")
		// 验证年月格式是否正确
		if len(yearMonth) == 7 && yearMonth[4] == '-' {
			// 解析年份和月份
			year := yearMonth[:4]
			month := yearMonth[5:]
			// 重定向到monthgoal页面，并传递year和month参数
			h.Redirect(w, r, fmt.Sprintf("/monthgoal?year=%s&month=%s", year, month), 302)
			return
		}
		// 如果格式不正确，则使用默认重定向
		h.Redirect(w, r, "/monthgoal", 302)
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
	if strings.HasPrefix(blogname, "horoscope-") {
		// 重定向到constellation页面
		h.Redirect(w, r, "/constellation", 302)
		return
	}

	usepublic := 0
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) != 0 {
		// 判定blog访问权限 - 直接使用已获取的blog对象
		session := getsession(r)
		auth_type := blog.AuthType
		if cooperation.IsCooperation(session) {
			// 判定blog访问权限
			if (auth_type & module.EAuthType_cooperation) != 0 {
				if cooperation.CanEditBlog(session, blogname) != 0 {
					if (auth_type & module.EAuthType_public) == 0 {
						h.Redirect(w, r, "/index", 302)
						return
					}
				}
			}
		} else {
			if (auth_type & module.EAuthType_private) != 0 {
				h.Redirect(w, r, "/index", 302)
				return
			}
		}

		if (auth_type & module.EAuthType_public) != 0 {
			usepublic = 1
		} else {
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
		control.RecordBlogAccess(blogname, remoteAddr, userAgent)
	}

	view.PageGetBlog(blogname, w, usepublic)
}

// HandleComment handles blog comment functionality
func HandleComment(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleComment", r)
	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	// 设置请求体大小限制
	r.ParseMultipartForm(1 << 20) // 1MB

	// 获取单个字段值
	title := r.FormValue("title")
	pattern := `^[\p{Han}a-zA-Z0-9\._-]+$`
	reg := regexp.MustCompile(pattern)
	match := reg.MatchString(title)
	if !match {
		h.Error(w, "save failed! title is invalied!", h.StatusBadRequest)
		return
	}

	log.DebugF("comment title:%s", title)

	owner := r.FormValue("owner")
	mail := r.FormValue("mail")
	comment := r.FormValue("comment")
	sessionID := r.FormValue("session_id") // 新增会话ID参数

	if comment == "" {
		h.Error(w, "save failed! comment is invalied!", h.StatusBadRequest)
		return
	}

	// 获取用户IP和UserAgent
	ip := r.RemoteAddr
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		ip = xForwardedFor
	}
	userAgent := r.Header.Get("User-Agent")

	// 优先使用身份验证的评论系统
	if sessionID != "" {
		// 使用已有会话发表评论
		ret, msg := control.AddCommentWithAuth(title, comment, sessionID, ip, userAgent)
		if ret == 0 {
			w.WriteHeader(h.StatusOK)
			w.Write([]byte(msg))
		} else {
			h.Error(w, msg, h.StatusBadRequest)
		}
		return
	}

	// 如果没有会话ID且提供了用户名，使用密码验证机制
	if owner != "" {
		password := r.FormValue("pwd") // 获取密码

		if password != "" {
			// 使用密码验证创建会话
			ret, msg, newSessionID := control.AddCommentWithPassword(title, comment, owner, mail, password, ip, userAgent)
			if ret == 0 {
				// 构造包含会话ID的响应
				response := map[string]interface{}{
					"success":    true,
					"message":    msg,
					"session_id": newSessionID,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(h.StatusOK)
				json.NewEncoder(w).Encode(response)
			} else {
				h.Error(w, msg, h.StatusBadRequest)
			}
		} else {
			// 没有密码，创建匿名用户会话
			ret, msg := control.AddAnonymousComment(title, comment, owner, mail, ip, userAgent)
			if ret == 0 {
				w.WriteHeader(h.StatusOK)
				w.Write([]byte(msg))
			} else {
				h.Error(w, msg, h.StatusBadRequest)
			}
		}
		return
	}

	// 兜底：使用原有的简单评论系统（保持向后兼容）
	if owner == "" {
		owner = ip // 使用IP作为默认用户名
	}

	pwd := r.FormValue("pwd")
	if pwd == "" {
		pwd = ip // 使用IP作为默认密码
	}

	control.AddComment(title, comment, owner, pwd, mail)
	w.WriteHeader(h.StatusOK)
	w.Write([]byte("评论提交成功" + title + " " + owner + " " + pwd + " " + mail))
}

// HandleCheckUsername checks username availability for comments
// 检查用户名信息的API（返回使用该用户名的用户数量）
func HandleCheckUsername(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleCheckUsername", r)

	if r.Method != h.MethodGet {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(h.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "用户名参数缺失",
		})
		return
	}

	// 获取使用该用户名的用户列表
	users := comment.UserManager.GetUsersByUsername(username)
	userCount := len(users)

	response := map[string]interface{}{
		"success":    true,
		"available":  userCount == 0,
		"username":   username,
		"user_count": userCount,
	}

	if userCount == 0 {
		response["message"] = "新用户名，可直接使用"
	} else {
		response["message"] = "该用户名已被注册，请输入密码进行身份验证"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.StatusOK)
	json.NewEncoder(w).Encode(response)
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
	log.DebugF("delete title:%s", title)

	ret := control.DeleteBlog(title)
	if ret == 0 {
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

	// 设置请求体大小限制
	r.ParseMultipartForm(32 << 20) // 32MB

	// 获取单个字段值
	title := r.FormValue("title")
	log.DebugF("title:%s", title)

	// 解析权限设置
	auth_type_string := r.FormValue("auth_type")
	log.DebugF("Received auth_type:%s", auth_type_string)

	// 解析权限组合
	auth_type := parseAuthTypeString(auth_type_string)

	// tags
	tags := r.FormValue("tags")
	log.DebugF("Received tags:%s", tags)

	// 内容
	content := r.FormValue("content")
	// 在这里，您可以处理或保存content到数据库等
	//log.DebugF("Received content:%s", content)

	// 加密
	encryptionKey := r.FormValue("encrypt")
	encrypt := 0
	log.DebugF("Received title=%s encrypt:%s session:%s", title, encryptionKey, getsession(r))

	if encryptionKey != "" {
		encrypt = 1
		/*
			// aes加密
			log.DebugF("encryption key=%s",encryptionKey)
			content_encrypt  := encryption.AesSimpleEncrypt(content, encryptionKey);

			content_decrypt := encryption.AesSimpleDecrypt(content_encrypt, encryptionKey);
			log.DebugF("encryption content_decrypt=%s",content_encrypt)
			if content_decrypt != content {
				h.Error(w, "save failed! aes not match error!", h.StatusBadRequest)
				return
			}
			fmt.Printf("content encrypt=%s\n",content)

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
	}

	ret := control.ModifyBlog(&ubd)

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
	ret := view.PageSearchNormal(match, w, r)
	if ret != 0 {
		// 通用搜索逻辑
		session := getsession(r)
		view.PageSearch(match, w, session)
	}
}

// HandleTag handles tag-based blog listing
func HandleTag(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleTag", r)

	r.ParseMultipartForm(32 << 20) // 32MB

	tag := r.FormValue("tag")

	isTagPublic := config.IsPublicTag(tag)
	log.DebugF("HandleTag %s %d", tag, isTagPublic)
	if isTagPublic != 1 {
		if checkLogin(r) != 0 {
			h.Redirect(w, r, "/index", 302)
			return
		}
	}

	// 展示所有public tag
	view.PageTags(w, tag)
}

// HandlePublic renders the public blogs page
func HandlePublic(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandlePublic", r)

	view.PagePublic(w)
}
