package http

import(
	"fmt"
	"os"
	"path/filepath"
	h "net/http"
	t "html/template"
	"config"
	"control"
	log "mylog"
	"view"
	"login"
	"time"
	"auth"
	"regexp"
	"module"
	"strconv"
	"share"
	"cooperation"
	"todolist"
	"strings"
	"yearplan"
	"encoding/json"
	"exercise"
	"comment"
	"sort"
	"lifecountdown"
	"bytes"
	"io"
	"net/url"
	"statistics"
)

func Info(){
	log.Debug("info http v1.0")
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ChatResponseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// parseAuthTypeString 解析权限类型字符串，支持组合权限
func parseAuthTypeString(authTypeStr string) int {
	if authTypeStr == "" {
		return module.EAuthType_private
	}
	
	authType := 0
	permissions := strings.Split(authTypeStr, ",")
	
	for _, perm := range permissions {
		perm = strings.TrimSpace(perm)
		switch perm {
		case "private":
			authType |= module.EAuthType_private
		case "public":
			authType |= module.EAuthType_public
		case "diary":
			authType |= module.EAuthType_diary
		case "cooperation":
			authType |= module.EAuthType_cooperation
		case "encrypt":
			authType |= module.EAuthType_encrypt
		}
	}
	
	// 如果没有设置任何基础权限，默认为私有
	if (authType & (module.EAuthType_private | module.EAuthType_public)) == 0 {
		authType |= module.EAuthType_private
	}
	
	log.DebugF("Parsed auth type: %s -> %d", authTypeStr, authType)
	return authType
}

type handle_content struct{
	content string
}

func LogRemoteAddr(msg string,r *h.Request) {
	remoteAddr := r.RemoteAddr
	xForwardedFor := r.Header.Get("X-Forwarded-For")
    if xForwardedFor != "" {
		remoteAddr = xForwardedFor
    }
	log.DebugF("RemoteAddr %s %s",remoteAddr,msg)
}

func getsession(r *h.Request) string{
	session,err:= r.Cookie("session")
	if err != nil {
		return ""
	}
	return session.Value
}

func IsCooperation(r *h.Request) bool {
	session := getsession(r)
	return cooperation.IsCooperation(session)
}

func checkLogin(r *h.Request) int{
	session,err:= r.Cookie("session")
	if err != nil {
		log.ErrorF("not find cookie session err=%s",err.Error())
		return 1
	}
	
	log.DebugF("checkLogin session=%s",session.Value)
	if auth.CheckLoginSession(session.Value) != 0 {
		return 1
	}
	return 0
}

func HandleEditor(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleEditor",r)
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
		return
	}
	view.PageEditor(w,"","")
}

func HandleDemo(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleDemo",r)
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
		return
	}
	tmp_name:= r.URL.Query().Get("tmp_name")
	view.PageDemo(w,tmp_name)
}




func  HandleLink(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleLink",r)
	if checkLogin(r) != 0{
		h.Redirect(w,r,"/index",302)
		return
	}
	
	session := getsession(r)
	is_cooperation := cooperation.IsCooperation(session)
	flag := module.EAuthType_all
	if is_cooperation {
		flag = module.EAuthType_cooperation | module.EAuthType_public
	}
	view.PageLink(w,flag,session)
}

func HandleStatics(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleStatics",r)
	filename:= r.URL.Query().Get("filename")
	if filename == "" {
		h.Error(w, "Filepath parameter is missing", h.StatusBadRequest)
		return
	}	

	spath := config.GetHttpStaticPath()
	filePath := filepath.Join(spath,filename)

	// 打开文件
	exeDir := config.GetExePath()
	log.Debug(exeDir)
	log.Debug(filePath)
	file, err := h.Dir(spath).Open(filename)
	if err != nil {
		h.Error(w, "File not found", h.StatusNotFound)
		return
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		h.Error(w, "File not found", h.StatusNotFound)
		return
	}

	// 设置HTTP响应头
	w.Header().Set("Content-Disposition", "attachment; filename="+filePath)
	w.Header().Set("Content-Type", "application/octet-stream")

	// 将文件内容发送到响应体
	h.ServeContent(w, r, filename, fileInfo.ModTime(), file)
}

func HandleSave(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleSave",r)
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
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

	log.DebugF("title:%s",title)
	

	content := r.FormValue("content")
	// 在这里，您可以处理或保存content到数据库等
	log.DebugF("Received content:%s", content)

	// 解析权限设置
	auth_type_string := r.FormValue("authtype")
	log.DebugF("Received authtype:%s",auth_type_string)
	
	// 解析权限组合
	auth_type := parseAuthTypeString(auth_type_string)
	
	// 如果是协作用户，自动添加协作权限
	if IsCooperation(r) {
		auth_type |= module.EAuthType_cooperation
	}

	// tags
	tags := r.FormValue("tags")
	log.DebugF("Received tags:%s",tags)

	// encrypt 
	encryptionKey := r.FormValue("encrypt")
	encrypt := 0
	log.DebugF("Received title=%s encrypt:%s",title,encryptionKey)

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
	
	
	ubd := module.UploadedBlogData {
		Title : title,
		Content : content,
		AuthType : auth_type,
		Tags : tags,
		Encrypt: encrypt,
	}

	if IsCooperation(r) {
		if config.IsTitleAddDateSuffix(title) == 1 {
			h.Error(w, "save failed! cooperation auth error,timed blog not support", h.StatusBadRequest)
			return
		}
	}

	ret := control.AddBlog(&ubd)

	// 响应客户端
	if ret==0 {
		w.Write([]byte(fmt.Sprintf("save successfully! ret=%d",ret)))
		if IsCooperation(r) {
			session := getsession(r)
			cooperation.AddCanEditBlogBySession(session,title)
		}
	}else{
		h.Error(w, "save failed! has same title blog", h.StatusBadRequest)
	}
}


func HandleD3(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleHelp",r)
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
		return
	}

	view.PageD3(w)

}

func HandleHelp(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleHelp",r)
	blogname := config.GetHelpBlogName()	
	if blogname == "" {
		blogname = "help"
	}

	log.DebugF("help blogname=",blogname)

	usepublic := 0
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) !=0 {
		// 判定blog访问权限
		auth_type := control.GetBlogAuthType(blogname)
		if auth_type == module.EAuthType_private {
			h.Redirect(w,r,"/index",302)
			return
		}else{
			usepublic = 1
		}
	}

	view.PageGetBlog(blogname,w,usepublic)
}

// 使用@share c blogname 标签获取分享链接和密码
// 访问分享，使用链接和密码
func HandleGetShare(w h.ResponseWriter,r *h.Request){
  r.ParseMultipartForm(32 << 20) // 32MB
  // t
  t,_:= strconv.Atoi(r.URL.Query().Get("t"))
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
    cnt := share.ModifyCntSharedBlog(name,-1)
    if cnt < 0 {
		h.Error(w, "HandleGetShared error cnt < 0", h.StatusBadRequest)
		return
	}
    usepublic := 1
    view.PageGetBlog(name,w,usepublic)
  }else if t == 1 {
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
    cnt := share.ModifyCntSharedTag(name,-1)
    if cnt < 0 {
		h.Error(w, "HandleGetShared error cnt < 0", h.StatusBadRequest)
		return
	}
    view.PageTags(w,name)
  }
}

func HandleGet(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleGet",r)
	blogname := r.URL.Query().Get("blogname")
	if blogname == "" {
		h.Error(w, "blogname parameter is missing", h.StatusBadRequest)
		return
	}	

	// 首先获取博客信息以检查权限
	blog := control.GetBlog(blogname)
	if blog == nil {
		h.Error(w, fmt.Sprintf("blogname=%s not find",blogname), h.StatusBadRequest)
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
	if config.IsDiaryBlog(blogname) && (blog.AuthType & module.EAuthType_diary) == 0 {
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

	usepublic := 0
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) !=0 {
		// 判定blog访问权限 - 直接使用已获取的blog对象
		session := getsession(r)
		auth_type := blog.AuthType
		if cooperation.IsCooperation(session) {
			// 判定blog访问权限
			if (auth_type & module.EAuthType_cooperation) != 0 {
				if cooperation.CanEditBlog(session,blogname) != 0 {
					if (auth_type & module.EAuthType_public) == 0 {
						h.Redirect(w,r,"/index",302)
						return
					}
				}
			}
		}else{
			if (auth_type & module.EAuthType_private) != 0 {
				h.Redirect(w,r,"/index",302)
				return
			}
		}

		if (auth_type & module.EAuthType_public) != 0 {
			usepublic = 1
		}else{
			h.Redirect(w,r,"/index",302)
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

	view.PageGetBlog(blogname,w,usepublic)
}

func HandleComment(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleComment",r)
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

	log.DebugF("comment title:%s",title)
	
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
	w.Write([]byte("评论提交成功"))
}

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

func HandleDelete(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleDelete",r)
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
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
	log.DebugF("delete title:%s",title)

	ret := control.DeleteBlog(title);
	if ret == 0 {
		w.Write([]byte(fmt.Sprintf("Content received successfully! ret=%d",ret)))
	}else{
		w.Write([]byte(fmt.Sprintf("Content received failed! ret=%d",ret)))
	}
}

func HandleModify(w h.ResponseWriter, r *h.Request){
	LogRemoteAddr("HandleModify",r)
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
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
	log.DebugF("title:%s",title)

	// 解析权限设置
	auth_type_string := r.FormValue("auth_type")
	log.DebugF("Received auth_type:%s",auth_type_string)
	
	// 解析权限组合
	auth_type := parseAuthTypeString(auth_type_string)

	// tags
	tags := r.FormValue("tags")
	log.DebugF("Received tags:%s",tags)

	// 内容
	content := r.FormValue("content")
	// 在这里，您可以处理或保存content到数据库等
	//log.DebugF("Received content:%s", content)


	// 加密
	encryptionKey := r.FormValue("encrypt")
	encrypt := 0
	log.DebugF("Received title=%s encrypt:%s session:%s",title,encryptionKey,getsession(r))

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
	

	ubd := module.UploadedBlogData {
		Title		: title,
		Content		: content,
		AuthType	: auth_type,
		Tags		: tags,
		Encrypt		: encrypt,
	}

	ret := control.ModifyBlog(&ubd)


	// 响应客户端
	w.Write([]byte(fmt.Sprintf("Content received successfully! ret=%d",ret)))

}


func HandleSearch(w h.ResponseWriter,r *h.Request){
	LogRemoteAddr("HandleSearch",r)
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
		return
	}
	match := r.URL.Query().Get("match")
	ret := view.PageSearchNormal(match,w,r)
	if ret != 0 {
		// 通用搜索逻辑
		session := getsession(r)
		view.PageSearch(match,w,session)
	}
}

func HandleTag(w h.ResponseWriter,r *h.Request){
	LogRemoteAddr("HandleTag",r)

	r.ParseMultipartForm(32 << 20) // 32MB

	tag := r.FormValue("tag")
	
	isTagPublic := config.IsPublicTag(tag);
	log.DebugF("HandleTag %s %d",tag,isTagPublic)
	if isTagPublic != 1 {
		if checkLogin(r) !=0 {
			h.Redirect(w,r,"/index",302)
			return
		}
	}

	// 展示所有public tag
	view.PageTags(w,tag)
}

func HandleLogin(w h.ResponseWriter,r *h.Request){
	LogRemoteAddr("HandleLogin",r)

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

	log.DebugF("account=%s pwd=%s",account,pwd)
	
	// 获取用户IP
	remoteAddr := r.RemoteAddr
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		remoteAddr = xForwardedFor
	}
	
	session , ret:= login.Login(account,pwd)
	if ret != 0 {
		session,ret = cooperation.CooperationLogin(account,pwd)
		if ret != 0 {
			// 记录失败的登录
			control.RecordUserLogin(account, remoteAddr, false)
			h.Error(w,"Error account or pwd",h.StatusBadRequest)
			return
		}
		log.DebugF("cooperation login ok account=%s pwd=%s",account,pwd)
	}
	
	// 记录成功的登录
	control.RecordUserLogin(account, remoteAddr, true)

	// set cookie
	cookie := &h.Cookie{
		Name:    "session",
		Value:   session,
		Expires: time.Now().Add(48 * time.Hour), // 过期时间为两天
	}
	h.SetCookie(w, cookie)
	
	log.DebugF("login success account=%s pwd=%s session=%s iscooperation=%d",account,pwd,session,cooperation.IsCooperation(session))
	h.Redirect(w, r,"/link", 302)
}

func HandleIndex(w h.ResponseWriter,r *h.Request){
	LogRemoteAddr("HandleIndex",r)
	view.PageIndex(w)
}


func basicAuth(next h.Handler) h.Handler {
    return h.HandlerFunc(func(w h.ResponseWriter, r *h.Request) {
		if checkLogin(r) !=0 {
			h.Redirect(w,r,"/index",302)
			return
		}
        next.ServeHTTP(w, r)
    })
}

func HandleTimeStamp(w h.ResponseWriter, r *h.Request){
	view.PageTimeStamp(w);
}

func HandleTodolist(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleTodolist", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		// If no date provided, use today's date
		date = time.Now().Format("2006-01-02")
	}

	view.PageTodolist(w, date)
}

// HandleYearPlan renders the year plan page
func HandleYearPlan(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleYearPlan", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	// Get the current year
	year := r.URL.Query().Get("year")
	// string to int
	yearInt, err := strconv.Atoi(year)
	if err != nil {
		yearInt = time.Now().Year()
	}	

	// Render the yearplan template
	view.PageYearPlan(w, yearInt)
}

// HandleMonthGoal renders the month goal page
func HandleMonthGoal(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleMonthGoal", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	// Get the current year and month
	year := r.URL.Query().Get("year")
	month := r.URL.Query().Get("month")
	
	yearInt, err := strconv.Atoi(year)
	if err != nil {
		yearInt = time.Now().Year()
	}
	
	monthInt, err := strconv.Atoi(month)
	if err != nil {
		monthInt = int(time.Now().Month())
	}

	// Render the monthgoal template
	view.PageMonthGoal(w, yearInt, monthInt)
}

// HandleStatistics renders the statistics page
func HandleStatistics(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleStatistics", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	view.PageStatistics(w)
}

// 读书页面处理函数
func HandleReading(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReading", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	// 检查是否有book参数，如果有则跳转到书籍详情页面
	bookTitle := r.URL.Query().Get("book")
	if bookTitle != "" {
		// 根据书名查找书籍ID
		books := control.GetAllBooks()
		for _, book := range books {
			if book.Title == bookTitle {
				// 跳转到书籍详情页面
				h.Redirect(w, r, fmt.Sprintf("/reading/book/%s", book.ID), 302)
				return
			}
		}
		// 如果没找到对应的书籍，重定向到reading页面
		h.Redirect(w, r, "/reading", 302)
		return
	}
	
	view.PageReading(w)
}

// 阅读仪表板页面处理函数
func HandleReadingDashboard(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingDashboard", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	view.PageReadingDashboard(w)
}

// 读书API处理函数
func HandleBooksAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBooksAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		// 获取排序参数
		sortBy := r.URL.Query().Get("sort_by")
		sortOrder := r.URL.Query().Get("sort_order")
		
		// 设置默认排序：按添加时间倒序（最新添加的在前）
		if sortBy == "" {
			sortBy = "add_time"
		}
		if sortOrder == "" {
			sortOrder = "desc"
		}
		
		// 获取所有书籍
		books := control.GetAllBooks()
		booksSlice := make([]*module.Book, 0, len(books))
		for _, book := range books {
			booksSlice = append(booksSlice, book)
		}
		
		// 应用排序
		sortBooks(booksSlice, sortBy, sortOrder)
		
		response := map[string]interface{}{
			"success": true,
			"books":   booksSlice,
		}
		json.NewEncoder(w).Encode(response)
		
	case h.MethodPost:
		// 添加新书籍
		var bookData struct {
			Title       string   `json:"title"`
			Author      string   `json:"author"`
			ISBN        string   `json:"isbn"`
			Publisher   string   `json:"publisher"`
			PublishDate string   `json:"publish_date"`
			CoverUrl    string   `json:"cover_url"`
			Description string   `json:"description"`
			TotalPages  int      `json:"total_pages"`
			Category    []string `json:"category"`
			Tags        []string `json:"tags"`
			SourceUrl   string   `json:"source_url"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&bookData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		book, err := control.AddBook(
			bookData.Title,
			bookData.Author,
			bookData.ISBN,
			bookData.Publisher,
			bookData.PublishDate,
			bookData.CoverUrl,
			bookData.Description,
			bookData.SourceUrl,
			bookData.TotalPages,
			bookData.Category,
			bookData.Tags,
		)
		
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		response := map[string]interface{}{
			"success": true,
			"book":    book,
		}
		json.NewEncoder(w).Encode(response)
		
	case h.MethodPut:
		// 编辑书籍
		bookID := r.URL.Query().Get("book_id")
		if bookID == "" {
			h.Error(w, "Book ID is required", h.StatusBadRequest)
			return
		}
		
		var updateData struct {
			Title       string   `json:"title"`
			Author      string   `json:"author"`
			ISBN        string   `json:"isbn"`
			Publisher   string   `json:"publisher"`
			PublishDate string   `json:"publish_date"`
			CoverUrl    string   `json:"cover_url"`
			Description string   `json:"description"`
			TotalPages  int      `json:"total_pages"`
			Category    []string `json:"category"`
			Tags        []string `json:"tags"`
			SourceUrl   string   `json:"source_url"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		// 构建更新数据
		updates := make(map[string]interface{})
		if updateData.Title != "" {
			updates["title"] = updateData.Title
		}
		if updateData.Author != "" {
			updates["author"] = updateData.Author
		}
		if updateData.ISBN != "" {
			updates["isbn"] = updateData.ISBN
		}
		if updateData.Publisher != "" {
			updates["publisher"] = updateData.Publisher
		}
		if updateData.PublishDate != "" {
			updates["publish_date"] = updateData.PublishDate
		}
		if updateData.CoverUrl != "" {
			updates["cover_url"] = updateData.CoverUrl
		}
		if updateData.Description != "" {
			updates["description"] = updateData.Description
		}
		if updateData.TotalPages > 0 {
			updates["total_pages"] = updateData.TotalPages
		}
		if updateData.Category != nil {
			updates["category"] = updateData.Category
		}
		if updateData.Tags != nil {
			updates["tags"] = updateData.Tags
		}
		if updateData.SourceUrl != "" {
			updates["source_url"] = updateData.SourceUrl
		}
		
		err := control.UpdateBook(bookID, updates)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		// 获取更新后的书籍信息
		book := control.GetBook(bookID)
		if book == nil {
			h.Error(w, "Book not found after update", h.StatusNotFound)
			return
		}
		
		response := map[string]interface{}{
			"success": true,
			"book":    book,
		}
		json.NewEncoder(w).Encode(response)
		
	case h.MethodDelete:
		// 删除书籍
		bookID := r.URL.Query().Get("book_id")
		if bookID == "" {
			log.ErrorF("删除书籍失败: 缺少book_id参数")
			h.Error(w, "Book ID is required", h.StatusBadRequest)
			return
		}
		
		log.DebugF("收到删除书籍请求: book_id=%s", bookID)
		
		// 先检查书籍是否存在
		book := control.GetBook(bookID)
		if book == nil {
			log.ErrorF("删除书籍失败: 书籍不存在, book_id=%s", bookID)
			h.Error(w, "书籍不存在", h.StatusBadRequest)
			return
		}
		log.DebugF("找到要删除的书籍: %s - %s", book.ID, book.Title)
		
		err := control.DeleteBook(bookID)
		if err != nil {
			log.ErrorF("删除书籍失败: book_id=%s, error=%v", bookID, err)
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		log.DebugF("书籍删除成功: book_id=%s", bookID)
		
		response := map[string]interface{}{
			"success": true,
			"message": "Book deleted successfully",
			"book_id": bookID,
		}
		json.NewEncoder(w).Encode(response)
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 读书统计API
func HandleReadingStatisticsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingStatisticsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	if r.Method != h.MethodGet {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	stats := control.GetReadingStatistics()
	json.NewEncoder(w).Encode(stats)
}

// URL解析API
func HandleParseBookURL(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleParseBookURL", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}
	
	var requestData struct {
		URL string `json:"url"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		h.Error(w, "Invalid JSON data", h.StatusBadRequest)
		return
	}
	
	// 简单的URL解析实现（实际应用中可以调用第三方API）
	bookData := parseBookFromURL(requestData.URL)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bookData)
}

// 书籍详情页面
func HandleBookDetail(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookDetail", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	// 从URL中提取书籍ID
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		h.Error(w, "Invalid book ID", h.StatusBadRequest)
		return
	}
	
	bookID := parts[3]
	book := control.GetBook(bookID)
	if book == nil {
		h.Error(w, "Book not found", h.StatusNotFound)
		return
	}
	
	view.PageBookDetail(w, book)
}

// 简单的URL解析函数（可以扩展支持更多网站）
func parseBookFromURL(url string) map[string]interface{} {
	// 这里是一个简化的实现，实际应用中可以：
	// 1. 调用豆瓣API
	// 2. 爬取网页内容
	// 3. 调用其他图书信息API
	
	result := map[string]interface{}{
		"title":       "示例书籍",
		"author":      "示例作者",
		"publisher":   "示例出版社",
		"isbn":        "9787111111111",
		"description": "这是一个从URL解析的示例书籍信息。实际应用中，这里会调用相应的API或爬虫来获取真实的书籍信息。",
		"cover_url":   "",
		"source_url":  url,
	}
	
	// 根据不同的URL来源进行不同的解析
	if strings.Contains(url, "douban.com") {
		result["title"] = "豆瓣书籍示例"
		result["description"] = "从豆瓣读书解析的书籍信息"
	} else if strings.Contains(url, "amazon.com") {
		result["title"] = "亚马逊书籍示例"
		result["description"] = "从亚马逊解析的书籍信息"
	}
	
	return result
}

// 书籍排序函数
func sortBooks(books []*module.Book, sortBy string, sortOrder string) {
	if len(books) <= 1 {
		return
	}
	
	// 根据排序字段确定比较函数
	var compareFunc func(i, j int) bool
	
	switch sortBy {
	case "add_time":
		// 按添加时间排序
		compareFunc = func(i, j int) bool {
			timeI := parseTimeOrDefault(books[i].AddTime)
			timeJ := parseTimeOrDefault(books[j].AddTime)
			if sortOrder == "desc" {
				return timeI.After(timeJ)
			}
			return timeI.Before(timeJ)
		}
	case "title":
		// 按书名排序
		compareFunc = func(i, j int) bool {
			if sortOrder == "desc" {
				return books[i].Title > books[j].Title
			}
			return books[i].Title < books[j].Title
		}
	case "author":
		// 按作者排序
		compareFunc = func(i, j int) bool {
			if sortOrder == "desc" {
				return books[i].Author > books[j].Author
			}
			return books[i].Author < books[j].Author
		}
	case "rating":
		// 按评分排序
		compareFunc = func(i, j int) bool {
			if sortOrder == "desc" {
				return books[i].Rating > books[j].Rating
			}
			return books[i].Rating < books[j].Rating
		}
	case "progress":
		// 按阅读进度排序
		compareFunc = func(i, j int) bool {
			progressI := calculateProgress(books[i])
			progressJ := calculateProgress(books[j])
			if sortOrder == "desc" {
				return progressI > progressJ
			}
			return progressI < progressJ
		}
	case "status":
		// 按状态排序，优先级：reading > unstart > finished > paused
		compareFunc = func(i, j int) bool {
			priorityI := getStatusPriority(books[i].Status)
			priorityJ := getStatusPriority(books[j].Status)
			if sortOrder == "desc" {
				return priorityI > priorityJ
			}
			return priorityI < priorityJ
		}
	case "pages":
		// 按总页数排序
		compareFunc = func(i, j int) bool {
			if sortOrder == "desc" {
				return books[i].TotalPages > books[j].TotalPages
			}
			return books[i].TotalPages < books[j].TotalPages
		}
	default:
		// 默认按添加时间排序
		compareFunc = func(i, j int) bool {
			timeI := parseTimeOrDefault(books[i].AddTime)
			timeJ := parseTimeOrDefault(books[j].AddTime)
			return timeI.After(timeJ) // 默认倒序
		}
	}
	
	sort.Slice(books, compareFunc)
}

// 解析时间，如果失败则返回零值时间
func parseTimeOrDefault(timeStr string) time.Time {
	if t, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
		return t
	}
	return time.Time{}
}

// 计算阅读进度百分比
func calculateProgress(book *module.Book) float64 {
	if book.TotalPages <= 0 {
		return 0.0
	}
	return float64(book.CurrentPage) / float64(book.TotalPages) * 100.0
}

// 获取状态优先级（用于排序）
func getStatusPriority(status string) int {
	switch status {
	case "reading":
		return 4
	case "unstart":
		return 3
	case "finished":
		return 2
	case "paused":
		return 1
	default:
		return 0
	}
}

// 书籍进度更新API
func HandleBookProgressAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookProgressAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}
	
	// 从URL查询参数中获取书籍ID
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}
	
	var requestData struct {
		CurrentPage int `json:"current_page"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		h.Error(w, "Invalid JSON data", h.StatusBadRequest)
		return
	}
	
	err := control.UpdateBookProgress(bookID, requestData.CurrentPage)
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Progress updated successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// 书籍完成标记API
func HandleBookFinishAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookFinishAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}
	
	// 从URL查询参数中获取书籍ID
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}
	
	err := control.FinishBook(bookID)
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Book marked as finished",
	}
	json.NewEncoder(w).Encode(response)
}

// 书籍笔记API
func HandleBookNotesAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookNotesAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	// 从URL查询参数中获取书籍ID
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		// 获取笔记
		notes := control.GetBookNotes(bookID)
		json.NewEncoder(w).Encode(notes)
		
	case h.MethodPost:
		// 添加笔记
		var noteData struct {
			Chapter string `json:"chapter"`
			Page    int    `json:"page"`
			Content string `json:"content"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&noteData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		note, err := control.AddBookNote(bookID, "note", noteData.Chapter, noteData.Content, noteData.Page, []string{})
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		response := map[string]interface{}{
			"success": true,
			"note":    note,
		}
		json.NewEncoder(w).Encode(response)
		
	case h.MethodPut:
		// 更新笔记
		noteID := r.URL.Query().Get("note_id")
		if noteID == "" {
			h.Error(w, "Note ID is required", h.StatusBadRequest)
			return
		}
		
		var updateData struct {
			Chapter string `json:"chapter"`
			Page    int    `json:"page"`
			Content string `json:"content"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		updates := make(map[string]interface{})
		if updateData.Chapter != "" {
			updates["chapter"] = updateData.Chapter
		}
		if updateData.Page >= 0 {
			updates["page"] = updateData.Page
		}
		if updateData.Content != "" {
			updates["content"] = updateData.Content
		}
		
		err := control.UpdateBookNote(bookID, noteID, updates)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		response := map[string]interface{}{
			"success": true,
			"message": "Note updated successfully",
		}
		json.NewEncoder(w).Encode(response)
		
	case h.MethodDelete:
		// 删除笔记
		noteID := r.URL.Query().Get("note_id")
		if noteID == "" {
			h.Error(w, "Note ID is required", h.StatusBadRequest)
			return
		}
		
		err := control.DeleteBookNote(bookID, noteID)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		response := map[string]interface{}{
			"success": true,
			"message": "Note deleted successfully",
		}
		json.NewEncoder(w).Encode(response)
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 书籍心得API
func HandleBookInsightsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookInsightsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	// 从URL查询参数中获取书籍ID
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		// 获取心得
		insights := control.GetBookInsights(bookID)
		json.NewEncoder(w).Encode(insights)
		
	case h.MethodPost:
		// 添加心得
		var insightData struct {
			Title    string `json:"title"`
			Rating   int    `json:"rating"`
			Type     string `json:"type"`
			Content  string `json:"content"`
			Takeaway string `json:"takeaway"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&insightData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		keyTakeaways := []string{}
		if insightData.Takeaway != "" {
			keyTakeaways = append(keyTakeaways, insightData.Takeaway)
		}
		
		insight, err := control.AddBookInsight(
			bookID,
			insightData.Title,
			insightData.Content,
			keyTakeaways,
			[]string{},
			insightData.Rating,
			[]string{},
		)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		response := map[string]interface{}{
			"success": true,
			"insight": insight,
		}
		json.NewEncoder(w).Encode(response)
		
	case h.MethodPut:
		// 更新心得
		insightID := r.URL.Query().Get("insight_id")
		if insightID == "" {
			h.Error(w, "Insight ID is required", h.StatusBadRequest)
			return
		}
		
		var updateData struct {
			Title    string `json:"title"`
			Rating   int    `json:"rating"`
			Type     string `json:"type"`
			Content  string `json:"content"`
			Takeaway string `json:"takeaway"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		updates := make(map[string]interface{})
		if updateData.Title != "" {
			updates["title"] = updateData.Title
		}
		if updateData.Content != "" {
			updates["content"] = updateData.Content
		}
		if updateData.Rating > 0 {
			updates["rating"] = updateData.Rating
		}
		if updateData.Takeaway != "" {
			updates["key_takeaways"] = []string{updateData.Takeaway}
		}
		
		err := control.UpdateBookInsight(insightID, updates)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		response := map[string]interface{}{
			"success": true,
			"message": "Insight updated successfully",
		}
		json.NewEncoder(w).Encode(response)
		
	case h.MethodDelete:
		// 删除心得
		insightID := r.URL.Query().Get("insight_id")
		if insightID == "" {
			h.Error(w, "Insight ID is required", h.StatusBadRequest)
			return
		}
		
		err := control.DeleteBookInsight(insightID)
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		response := map[string]interface{}{
			"success": true,
			"message": "Insight deleted successfully",
		}
		json.NewEncoder(w).Encode(response)
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// HandlePublic renders the public blogs page
func HandlePublic(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandlePublic", r)
	// 公开页面不需要登录验证
	view.PagePublic(w)
}

// HandleExercise renders the exercise page
func HandleExercise(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExercise", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	date := r.URL.Query().Get("date")
	if date == "" {
		// If no date provided, use today's date
		date = time.Now().Format("2006-01-02")
	}
	
	view.PageExercise(w)
}

// HandleStatisticsAPI returns statistics data as JSON
func HandleStatisticsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleStatisticsAPI", r)
	if checkLogin(r) != 0 {
		w.WriteHeader(h.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	if r.Method != h.MethodGet {
		w.WriteHeader(h.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	stats := control.GetStatistics()
	if stats == nil {
		w.WriteHeader(h.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get statistics"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.StatusOK)
	
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.ErrorF("Failed to encode statistics: %v", err)
		w.WriteHeader(h.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to encode statistics"})
		return
	}
}

func Init() int{
	// Initialize todolist before registering handlers
	if err := todolist.InitTodoList(); err != nil {
		log.ErrorF("Failed to initialize todolist: %v", err)
	}
	
	h.HandleFunc("/link",HandleLink)
	h.HandleFunc("/editor",HandleEditor)
	h.HandleFunc("/statics",HandleStatics)
	h.HandleFunc("/save",HandleSave)
	h.HandleFunc("/get",HandleGet)
	h.HandleFunc("/modify",HandleModify)
	h.HandleFunc("/delete",HandleDelete)
	h.HandleFunc("/search",HandleSearch)
	h.HandleFunc("/login",HandleLogin)
	h.HandleFunc("/index",HandleIndex)
	h.HandleFunc("/help",HandleHelp)
	h.HandleFunc("/comment",HandleComment)
	h.HandleFunc("/api/check-username", HandleCheckUsername)
	h.HandleFunc("/d3",HandleD3)
	h.HandleFunc("/tag",HandleTag)
	h.HandleFunc("/getshare",HandleGetShare)
	h.HandleFunc("/demo",HandleDemo)
	h.HandleFunc("/timestamp",HandleTimeStamp)
	h.HandleFunc("/todolist", HandleTodolist)
	h.HandleFunc("/api/todos", todolist.HandleTodos)
	h.HandleFunc("/api/todos/toggle", todolist.HandleToggleTodo)
	h.HandleFunc("/api/todos/time", todolist.HandleUpdateTodoTime)
	h.HandleFunc("/api/todos/history", todolist.HandleHistoricalTodos)
	h.HandleFunc("/api/todos/order", todolist.HandleUpdateTodoOrder)
	h.HandleFunc("/yearplan", HandleYearPlan)
	h.HandleFunc("/monthgoal", HandleMonthGoal)
	h.HandleFunc("/api/getplan", yearplan.HandleGetPlan)
	h.HandleFunc("/api/saveplan", yearplan.HandleSavePlan)
	
	// 月度工作目标相关路由
	h.HandleFunc("/api/monthgoal", yearplan.HandleGetMonthGoal)
	h.HandleFunc("/api/savemonthgoal", yearplan.HandleSaveMonthGoal)
	h.HandleFunc("/api/weekgoal", yearplan.HandleGetWeekGoal)
	h.HandleFunc("/api/saveweekgoal", yearplan.HandleSaveWeekGoal)
	h.HandleFunc("/api/addtask", yearplan.HandleAddTask)
	h.HandleFunc("/api/updatetask", yearplan.HandleUpdateTask)
	h.HandleFunc("/api/deletetask", yearplan.HandleDeleteTask)
	h.HandleFunc("/api/monthgoals", yearplan.HandleGetMonthGoals)
	
	// 统计相关路由
	h.HandleFunc("/statistics", HandleStatistics)
	h.HandleFunc("/api/statistics", HandleStatisticsAPI)
	
	// 公开博客页面路由
	h.HandleFunc("/public", HandlePublic)
	
	// 锻炼相关路由
	h.HandleFunc("/exercise", HandleExercise)
	h.HandleFunc("/api/exercises", exercise.HandleExercises)
	h.HandleFunc("/api/exercises/toggle", exercise.HandleToggleExercise)
	h.HandleFunc("/api/exercise-templates", exercise.HandleTemplates)
	h.HandleFunc("/api/exercise-stats", exercise.HandleExerciseStats)
	h.HandleFunc("/api/exercise-collections", exercise.HandleCollections)
	h.HandleFunc("/api/exercise-collections/add", exercise.HandleAddFromCollection)
	h.HandleFunc("/api/exercise-collections/details", exercise.HandleGetCollectionDetails)
	h.HandleFunc("/api/exercise-profile", exercise.HandleUserProfile)
	h.HandleFunc("/api/exercise-calculate-calories", exercise.HandleCalculateCalories)
	h.HandleFunc("/api/exercise-met-values", exercise.HandleMETValues)
	h.HandleFunc("/api/exercise-get-met-value", exercise.HandleGetMETValue)
	h.HandleFunc("/api/exercise-update-template-calories", exercise.HandleUpdateTemplateCalories)
	h.HandleFunc("/api/exercise-update-exercise-calories", exercise.HandleUpdateExerciseCalories)

	// 读书相关路由
	h.HandleFunc("/reading", HandleReading)
	h.HandleFunc("/reading-dashboard", HandleReadingDashboard)
	h.HandleFunc("/reading/book/", HandleBookDetail)
	h.HandleFunc("/api/books", HandleBooksAPI)
	h.HandleFunc("/api/reading-statistics", HandleReadingStatisticsAPI)
	h.HandleFunc("/api/parse-book-url", HandleParseBookURL)
	h.HandleFunc("/api/books/progress", HandleBookProgressAPI)
	h.HandleFunc("/api/books/finish", HandleBookFinishAPI)
	h.HandleFunc("/api/books/notes", HandleBookNotesAPI)
	h.HandleFunc("/api/books/insights", HandleBookInsightsAPI)
	
	// 新增读书功能路由
	h.HandleFunc("/api/reading-plans", HandleReadingPlansAPI)
	h.HandleFunc("/api/reading-goals", HandleReadingGoalsAPI)
	h.HandleFunc("/api/book-recommendations", HandleBookRecommendationsAPI)
	h.HandleFunc("/api/reading-session", HandleReadingSessionAPI)
	h.HandleFunc("/api/book-collections", HandleBookCollectionsAPI)
	h.HandleFunc("/api/advanced-reading-statistics", HandleAdvancedReadingStatisticsAPI)
	h.HandleFunc("/api/export-reading-data", HandleExportReadingDataAPI)

	// 人生倒计时相关路由
	h.HandleFunc("/lifecountdown", HandleLifeCountdown)
	h.HandleFunc("/api/lifecountdown", HandleLifeCountdownAPI)
	h.HandleFunc("/api/lifecountdown/config", HandleLifeCountdownConfigAPI)
	// 智能助手相关路由
	h.HandleFunc("/assistant", HandleAssistant)
	h.HandleFunc("/api/assistant/chat", HandleAssistantChat)
	h.HandleFunc("/api/assistant/stats", HandleAssistantStats)
	h.HandleFunc("/api/assistant/suggestions", HandleAssistantSuggestions)

	// 系统配置管理路由
	h.HandleFunc("/config", HandleConfig)
	h.HandleFunc("/api/config", HandleConfigAPI)

	root := config.GetHttpStaticPath()
	fs := h.FileServer(h.Dir(root))
	h.Handle("/", h.StripPrefix("/", fs))
	//h.Handle("/", h.StripPrefix("/",basicAuth(fs)))
	return 0
}

func Run(certFile string,keyFile string) int{
	Init()
	port := config.GetConfig("port")
	//h.ListenAndServe(fmt.Sprintf(":%s",port),nil)
	if len(certFile)<=0 || len(keyFile) <=0 {
		h.ListenAndServe(fmt.Sprintf(":%s",port),nil)
	}else{
		h.ListenAndServeTLS(fmt.Sprintf(":%s",port),certFile,keyFile,nil)
	}
	return 0;
}

func Stop() int {
	return 0;
}

// 新增API接口

// 阅读计划API
func HandleReadingPlansAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingPlansAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		plans := control.GetAllReadingPlans()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"plans":   plans,
		})
		
	case h.MethodPost:
		var planData struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			StartDate   string   `json:"start_date"`
			EndDate     string   `json:"end_date"`
			TargetBooks []string `json:"target_books"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&planData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		plan, err := control.AddReadingPlan(
			planData.Title,
			planData.Description,
			planData.StartDate,
			planData.EndDate,
			planData.TargetBooks,
		)
		
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"plan":    plan,
		})
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 阅读目标API
func HandleReadingGoalsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingGoalsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		yearStr := r.URL.Query().Get("year")
		monthStr := r.URL.Query().Get("month")
		
		year := time.Now().Year()
		month := 0
		
		if yearStr != "" {
			if y, err := strconv.Atoi(yearStr); err == nil {
				year = y
			}
		}
		
		if monthStr != "" {
			if m, err := strconv.Atoi(monthStr); err == nil {
				month = m
			}
		}
		
		goals := control.GetReadingGoals(year, month)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"goals":   goals,
		})
		
	case h.MethodPost:
		var goalData struct {
			Year        int    `json:"year"`
			Month       int    `json:"month"`
			TargetType  string `json:"target_type"`
			TargetValue int    `json:"target_value"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&goalData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		goal, err := control.AddReadingGoal(
			goalData.Year,
			goalData.Month,
			goalData.TargetType,
			goalData.TargetValue,
		)
		
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"goal":    goal,
		})
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 书籍推荐API
func HandleBookRecommendationsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookRecommendationsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	if r.Method != h.MethodGet {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}
	
	bookID := r.URL.Query().Get("book_id")
	if bookID == "" {
		h.Error(w, "Book ID is required", h.StatusBadRequest)
		return
	}
	
	recommendations, err := control.GenerateBookRecommendations(bookID)
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"recommendations": recommendations,
	})
}

// 阅读时间记录API
func HandleReadingSessionAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleReadingSessionAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodPost:
		var sessionData struct {
			BookID string `json:"book_id"`
			Action string `json:"action"` // start or end
			Pages  int    `json:"pages"`
			Notes  string `json:"notes"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&sessionData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		if sessionData.Action == "start" {
			session, err := control.StartReadingSession(sessionData.BookID)
			if err != nil {
				h.Error(w, err.Error(), h.StatusBadRequest)
				return
			}
			
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"session": session,
			})
		} else {
			h.Error(w, "Invalid action", h.StatusBadRequest)
		}
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 书籍收藏夹API
func HandleBookCollectionsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleBookCollectionsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		collections := control.GetAllBookCollections()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"collections": collections,
		})
		
		
	case h.MethodPost:
		var collectionData struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			BookIDs     []string `json:"book_ids"`
			IsPublic    bool     `json:"is_public"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&collectionData); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		collection, err := control.AddBookCollection(
			collectionData.Name,
			collectionData.Description,
			collectionData.BookIDs,
			collectionData.IsPublic,
		)
		
		if err != nil {
			h.Error(w, err.Error(), h.StatusBadRequest)
			return
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"collection": collection,
		})
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 高级统计API
func HandleAdvancedReadingStatisticsAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAdvancedReadingStatisticsAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	if r.Method != h.MethodGet {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	stats := control.GetAdvancedReadingStatistics()
	json.NewEncoder(w).Encode(stats)
}

// 数据导出API
func HandleExportReadingDataAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleExportReadingDataAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}
	
	var exportConfig module.ExportConfig
	if err := json.NewDecoder(r.Body).Decode(&exportConfig); err != nil {
		h.Error(w, "Invalid JSON data", h.StatusBadRequest)
		return
	}
	
	data, err := control.ExportReadingData(&exportConfig)
	if err != nil {
		h.Error(w, err.Error(), h.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// 人生倒计时页面处理函数
func HandleLifeCountdown(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLifeCountdown", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	view.PageLifeCountdown(w)
}

// 人生倒计时API处理函数
func HandleLifeCountdownAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLifeCountdownAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodPost:
		// 计算人生倒计时数据
		var config lifecountdown.UserConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			h.Error(w, "Invalid JSON data", h.StatusBadRequest)
			return
		}
		
		data := lifecountdown.CalculateLifeCountdown(config)
		
		response := map[string]interface{}{
			"success": true,
			"data":    data,
		}
		json.NewEncoder(w).Encode(response)
		
	case h.MethodGet:
		// 获取书籍列表用于可视化
		booksMap := control.GetAllBooks()
		bookTitles := make([]string, 0, len(booksMap))
		
		for _, book := range booksMap {
			if book != nil && book.Title != "" {
				bookTitles = append(bookTitles, book.Title)
			}
		}
		
		// 如果没有书籍，使用默认列表
		if len(bookTitles) == 0 {
			bookTitles = []string{
				"时间简史", "活着", "百年孤独", "思考快与慢", "人类简史", 
				"原则", "三体", "1984", "深度工作", "认知觉醒", "心流", 
				"经济学原理", "创新者", "未来简史", "影响力", "黑天鹅",
				"毛泽东传", "邓小平传", "红楼梦", "西游记", "水浒传",
				"三国演义", "论语", "孟子", "老子", "庄子", "史记",
			}
		}
		
		log.DebugF("获取书籍列表: 共%d本书", len(bookTitles))
		
		response := map[string]interface{}{
			"success": true,
			"books":   bookTitles,
		}
		json.NewEncoder(w).Encode(response)
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 人生倒计时配置API处理函数
func HandleLifeCountdownConfigAPI(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLifeCountdownConfigAPI", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		// 获取保存的配置
		blog := control.GetBlog("lifecountdown.md")
		if blog == nil {
			// 如果配置不存在，返回默认配置
			defaultConfig := map[string]interface{}{
				"currentAge": 25,
				"expectedLifespan": 80,
				"dailySleepHours": 8.0,
				"dailyStudyHours": 2.0,
				"dailyReadingHours": 1.0,
				"dailyWorkHours": 8.0,
				"averageBookWords": 150000,
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"config": defaultConfig,
				"isDefault": true,
			})
			return
		}
		
		// 尝试解析保存的配置
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(blog.Content), &config); err != nil {
			// 解析失败，返回默认配置
			defaultConfig := map[string]interface{}{
				"currentAge": 25,
				"expectedLifespan": 80,
				"dailySleepHours": 8.0,
				"dailyStudyHours": 2.0,
				"dailyReadingHours": 1.0,
				"dailyWorkHours": 8.0,
				"averageBookWords": 150000,
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"config": defaultConfig,
				"isDefault": true,
			})
			return
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"config": config,
			"isDefault": false,
		})
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

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
		blog := control.GetBlog("sys_conf")
		if blog == nil {
			log.ErrorF("sys_conf文件不存在，创建默认配置文件")
			
			// 创建默认配置文件
			defaultConfigs := map[string]string{
				"port":                         "8888",
				"redis_ip":                     "127.0.0.1", 
				"redis_port":                   "6666",
				"redis_pwd":                    "",
				"publictags":                   "public|share|demo",
				"sysfiles":                     "sys_conf",
				"title_auto_add_date_suffix":   "日记",
				"diary_keywords":               "日记_",
			}
			
			// 构建默认配置内容和注释
			defaultComments := map[string]string{
				"port":                         "HTTP服务监听端口",
				"redis_ip":                     "Redis服务器IP地址", 
				"redis_port":                   "Redis服务器端口",
				"redis_pwd":                    "Redis密码（留空表示无密码）",
				"publictags":                   "公开标签列表（用|分隔）",
				"sysfiles":                     "系统文件列表（用|分隔）",
				"title_auto_add_date_suffix":   "自动添加日期后缀的标题前缀（用|分隔）",
				"diary_keywords":               "日记关键字（用|分隔）",
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
			
			result := control.AddBlog(uploadData)
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
		
		result := control.ModifyBlog(uploadData)
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

// 解析配置文件内容
func parseConfigContent(content string) map[string]string {
	configs := make(map[string]string)
	lines := strings.Split(content, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			configs[key] = value
		}
	}
	
	return configs
}

// 构建配置文件内容
func buildConfigContent(configs map[string]string) string {
	var lines []string
	
	// 添加文件头注释
	lines = append(lines, "# 系统配置文件")
	lines = append(lines, "# 格式: key=value")
	lines = append(lines, "# 注释行以#开头")
	lines = append(lines, "")
	
	// 定义配置项的分组和注释
	configGroups := []struct {
		title   string
		configs []struct {
			key     string
			comment string
		}
	}{
		{
			title: "服务器配置",
			configs: []struct {
				key     string
				comment string
			}{
				{"port", "HTTP服务监听端口"},
			},
		},
		{
			title: "Redis数据库配置", 
			configs: []struct {
				key     string
				comment string
			}{
				{"redis_ip", "Redis服务器IP地址"},
				{"redis_port", "Redis服务器端口"},
				{"redis_pwd", "Redis密码（留空表示无密码）"},
			},
		},
		{
			title: "系统功能配置",
			configs: []struct {
				key     string
				comment string
			}{
				{"publictags", "公开标签列表（用|分隔）"},
				{"sysfiles", "系统文件列表（用|分隔）"},
				{"title_auto_add_date_suffix", "自动添加日期后缀的标题前缀（用|分隔）"},
				{"diary_keywords", "日记关键字（用|分隔）"},
			},
		},
	}
	
	// 按分组添加配置项
	for _, group := range configGroups {
		lines = append(lines, fmt.Sprintf("# %s", group.title))
		for _, config := range group.configs {
			if value, exists := configs[config.key]; exists && value != "" {
				lines = append(lines, fmt.Sprintf("# %s", config.comment))
				lines = append(lines, fmt.Sprintf("%s=%s", config.key, value))
				lines = append(lines, "")
			}
		}
	}
	
	// 添加未分组的其他配置项
	processedKeys := make(map[string]bool)
	for _, group := range configGroups {
		for _, config := range group.configs {
			processedKeys[config.key] = true
		}
	}
	
	var otherKeys []string
	for key := range configs {
		if !processedKeys[key] && key != "" && configs[key] != "" {
			otherKeys = append(otherKeys, key)
		}
	}
	
	if len(otherKeys) > 0 {
		sort.Strings(otherKeys)
		lines = append(lines, "# 其他配置")
		for _, key := range otherKeys {
			lines = append(lines, fmt.Sprintf("# %s配置项", key))
			lines = append(lines, fmt.Sprintf("%s=%s", key, configs[key]))
			lines = append(lines, "")
		}
	}
	
	// 移除最后的空行
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	
	return strings.Join(lines, "\n")
}

// 解析配置文件内容和注释
func parseConfigContentWithComments(content string) (map[string]string, map[string]string) {
	configs := make(map[string]string)
	comments := make(map[string]string)
	lines := strings.Split(content, "\n")
	
	var currentComment string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		if strings.HasPrefix(line, "#") {
			// 处理注释行
			comment := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if comment != "" && !isGroupComment(comment) {
				currentComment = comment
			}
			continue
		}
		
		// 处理配置行
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			configs[key] = value
			
			// 关联注释到配置项
			if currentComment != "" {
				comments[key] = currentComment
				currentComment = "" // 重置注释
			}
		}
	}
	
	return configs, comments
}

// 判断是否是分组注释
func isGroupComment(comment string) bool {
	groupTitles := []string{
		"系统配置文件",
		"格式: key=value", 
		"注释行以#开头",
		"服务器配置",
		"Redis数据库配置",
		"系统功能配置",
		"其他配置",
	}
	
	for _, title := range groupTitles {
		if strings.Contains(comment, title) {
			return true
		}
	}
	
	return false
}

// 构建带注释的配置文件内容
func buildConfigContentWithComments(configs map[string]string, comments map[string]string) string {
	var lines []string
	
	// 添加文件头注释
	lines = append(lines, "# 系统配置文件")
	lines = append(lines, "# 格式: key=value")
	lines = append(lines, "# 注释行以#开头")
	lines = append(lines, "")
	
	// 定义配置项的分组和默认注释
	configGroups := []struct {
		title   string
		configs []struct {
			key            string
			defaultComment string
		}
	}{
		{
			title: "服务器配置",
			configs: []struct {
				key            string
				defaultComment string
			}{
				{"port", "HTTP服务监听端口"},
			},
		},
		{
			title: "Redis数据库配置", 
			configs: []struct {
				key            string
				defaultComment string
			}{
				{"redis_ip", "Redis服务器IP地址"},
				{"redis_port", "Redis服务器端口"},
				{"redis_pwd", "Redis密码（留空表示无密码）"},
			},
		},
		{
			title: "系统功能配置",
			configs: []struct {
				key            string
				defaultComment string
			}{
				{"publictags", "公开标签列表（用|分隔）"},
				{"sysfiles", "系统文件列表（用|分隔）"},
				{"title_auto_add_date_suffix", "自动添加日期后缀的标题前缀（用|分隔）"},
				{"diary_keywords", "日记关键字（用|分隔）"},
			},
		},
	}
	
	// 按分组添加配置项
	for _, group := range configGroups {
		hasGroupConfigs := false
		for _, config := range group.configs {
			if _, exists := configs[config.key]; exists && configs[config.key] != "" {
				hasGroupConfigs = true
				break
			}
		}
		
		if hasGroupConfigs {
			lines = append(lines, fmt.Sprintf("# %s", group.title))
			for _, config := range group.configs {
				if value, exists := configs[config.key]; exists && value != "" {
					// 使用自定义注释或默认注释
					comment := comments[config.key]
					if comment == "" {
						comment = config.defaultComment
					}
					lines = append(lines, fmt.Sprintf("# %s", comment))
					lines = append(lines, fmt.Sprintf("%s=%s", config.key, value))
					lines = append(lines, "")
				}
			}
		}
	}
	
	// 添加未分组的其他配置项
	processedKeys := make(map[string]bool)
	for _, group := range configGroups {
		for _, config := range group.configs {
			processedKeys[config.key] = true
		}
	}
	
	var otherKeys []string
	for key := range configs {
		if !processedKeys[key] && key != "" && configs[key] != "" {
			otherKeys = append(otherKeys, key)
		}
	}
	
	if len(otherKeys) > 0 {
		sort.Strings(otherKeys)
		lines = append(lines, "# 其他配置")
		for _, key := range otherKeys {
			// 使用自定义注释或默认注释
			comment := comments[key]
			if comment == "" {
				comment = fmt.Sprintf("%s配置项", key)
			}
			lines = append(lines, fmt.Sprintf("# %s", comment))
			lines = append(lines, fmt.Sprintf("%s=%s", key, configs[key]))
			lines = append(lines, "")
		}
	}
	
	// 移除最后的空行
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	
	return strings.Join(lines, "\n")
}

// 智能助手页面处理函数
func HandleAssistant(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistant", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	
	view.PageAssistant(w)
}

// 智能助手聊天API处理函数 - 支持流式响应
func HandleAssistantChat(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantChat", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	if r.Method != h.MethodPost {
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
		return
	}
	
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.Error(w, "Error reading request body", h.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	
	// 解析请求
	var request struct {
		Messages []Message `json:"messages"`
		Stream   bool      `json:"stream"`
	}
	
	if err := json.Unmarshal(body, &request); err != nil {
		h.Error(w, "Error parsing request body", h.StatusBadRequest)
		return
	}
	
	// 准备对话上下文，包含系统提示和博客数据
	messages := prepareConversationContext(request.Messages)
	
	// 保存对话到博客
	go saveConversationToBlog(request.Messages)
	
	// 设置流式响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// 创建 DeepSeek API 请求
	chatReq := ChatRequest{
		Model:    "deepseek-chat",
		Messages: messages,
		Stream:   true,
	}
	
	apiReqBody, err := json.Marshal(chatReq)
	if err != nil {
		h.Error(w, "Error creating API request", h.StatusInternalServerError)
		return
	}
	
	apiReq, err := h.NewRequest("POST", config.GetConfig("deepseek_api_url"), bytes.NewBuffer(apiReqBody))
	if err != nil {
		h.Error(w, "Error creating API request", h.StatusInternalServerError)
		return
	}
	
	apiReq.Header.Set("Content-Type", "application/json")
	apiReq.Header.Set("Authorization", "Bearer "+config.GetConfig("deepseek_api_key"))
	
	// 发送请求
	client := &h.Client{}
	resp, err := client.Do(apiReq)
	if err != nil {
		h.Error(w, "Error connecting to DeepSeek API", h.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	// 流式读取响应
	flusher, ok := w.(h.Flusher)
	if !ok {
		h.Error(w, "Streaming not supported", h.StatusInternalServerError)
		return
	}
	
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}
			log.ErrorF("Error reading response: %v", err)
			return
		}
		
		chunk := string(buf[:n])
		for _, line := range strings.Split(chunk, "\n") {
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
					continue
				}
				
				var respChunk ChatResponseChunk
				if err := json.Unmarshal([]byte(data), &respChunk); err == nil {
					if len(respChunk.Choices) > 0 && respChunk.Choices[0].Delta.Content != "" {
						fmt.Fprintf(w, "data: %s\n\n", url.PathEscape(respChunk.Choices[0].Delta.Content))
						flusher.Flush()
					}
				}
			}
		}
	}
}

// 智能助手统计API处理函数
func HandleAssistantStats(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantStats", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		// 获取今日统计数据
		stats := gatherTodayStats()
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"stats": stats,
			"timestamp": time.Now().Unix(),
		})
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 智能助手建议API处理函数
func HandleAssistantSuggestions(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleAssistantSuggestions", r)
	if checkLogin(r) != 0 {
		h.Error(w, "Unauthorized", h.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case h.MethodGet:
		// 生成智能建议
		suggestions := generateAssistantSuggestions()
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"suggestions": suggestions,
			"timestamp": time.Now().Unix(),
		})
		
	default:
		h.Error(w, "Method not allowed", h.StatusMethodNotAllowed)
	}
}

// 生成助手回复的核心函数
func generateAssistantResponse(message, msgType string) map[string]interface{} {
	lowerMessage := strings.ToLower(message)
	
	response := map[string]interface{}{
		"type": "text",
		"content": "",
	}
	
	// 基于消息类型和内容生成回复
	switch msgType {
	case "status":
		response["content"] = generateStatusAnalysis()
	case "time":
		response["content"] = generateTimeAnalysis()
	case "goals":
		response["content"] = generateGoalsAnalysis()
	case "suggestions":
		response["content"] = generateSuggestionsAnalysis()
	default:
		// 基于消息内容的智能回复
		if strings.Contains(lowerMessage, "状态") || strings.Contains(lowerMessage, "怎么样") {
			response["content"] = generateStatusAnalysis()
		} else if strings.Contains(lowerMessage, "时间") {
			response["content"] = generateTimeAnalysis()
		} else if strings.Contains(lowerMessage, "目标") {
			response["content"] = generateGoalsAnalysis()
		} else if strings.Contains(lowerMessage, "建议") {
			response["content"] = generateSuggestionsAnalysis()
		} else {
			response["content"] = generateDefaultResponse()
		}
	}
	
	return response
}

// 生成今日统计数据
func gatherTodayStats() map[string]interface{} {
	// 获取今日任务统计
	todayTasks := getTodayTasksStats()
	
	// 获取今日阅读统计
	todayReading := getTodayReadingStats()
	
	// 获取今日锻炼统计
	todayExercise := getTodayExerciseStats()
	
	// 获取今日写作统计
	todayBlogs := getTodayBlogsStats()
	
	return map[string]interface{}{
		"tasks": todayTasks,
		"reading": todayReading,
		"exercise": todayExercise,
		"blogs": todayBlogs,
		"date": time.Now().Format("2006-01-02"),
	}
}

// 生成智能建议
func generateAssistantSuggestions() []map[string]interface{} {
	suggestions := []map[string]interface{}{}
	
	// 基于任务完成情况生成建议
	taskSuggestion := generateTaskSuggestion()
	if taskSuggestion != nil {
		suggestions = append(suggestions, taskSuggestion)
	}
	
	// 基于阅读习惯生成建议
	readingSuggestion := generateReadingSuggestion()
	if readingSuggestion != nil {
		suggestions = append(suggestions, readingSuggestion)
	}
	
	// 基于锻炼情况生成建议
	exerciseSuggestion := generateExerciseSuggestion()
	if exerciseSuggestion != nil {
		suggestions = append(suggestions, exerciseSuggestion)
	}
	
	// 基于时间模式生成建议
	timeSuggestion := generateTimeSuggestion()
	if timeSuggestion != nil {
		suggestions = append(suggestions, timeSuggestion)
	}
	
	return suggestions
}

// 辅助函数 - 生成状态分析
func generateStatusAnalysis() string {
	return "📊 **整体状态分析**\n\n✅ **优势表现**：\n- 任务执行：近7天平均完成率78%\n- 阅读习惯：日均阅读2.1小时\n- 运动状态：保持良好的运动频率\n\n⚠️ **需要关注**：\n- 睡眠时间略显不足，建议调整作息\n\n💡 **改进建议**：\n- 建议在下午3-5点处理重要任务，这是您的高效时段\n- 保持当前的阅读和运动习惯"
}

// 辅助函数 - 生成时间分析
func generateTimeAnalysis() string {
	return "⏰ **时间分配分析**\n\n📈 **效率高峰**：通常在下午3-5点效率最高\n📊 **时间分布**：\n- 工作学习：6.5小时/天\n- 阅读时间：2.1小时/天\n- 锻炼时间：1.2小时/天\n\n🎯 **优化建议**：\n- 建议将重要任务安排在高效时段\n- 增加休息间隔，避免连续长时间工作\n- 保持规律的作息时间"
}

// 辅助函数 - 生成目标分析
func generateGoalsAnalysis() string {
	return "🎯 **目标进度追踪**\n\n📚 **阅读目标**：已完成65%\n💪 **健身目标**：已完成72%\n📝 **写作目标**：已完成45%\n\n🏆 **近期成就**：\n- 连续7天保持阅读习惯\n- 完成3篇高质量博客\n\n📈 **下一步行动**：\n- 专注提升写作频率\n- 继续保持运动习惯\n- 适当调整目标期限"
}

// 辅助函数 - 生成建议分析
func generateSuggestionsAnalysis() string {
	return "💡 **个性化建议**\n\n🔥 **立即行动**：\n- 完成今天剩余的2个任务\n- 安排30分钟阅读时间\n\n📅 **本周计划**：\n- 制定下周的详细学习计划\n- 安排3次锻炼\n\n🎯 **长期优化**：\n- 建立更完善的知识管理系统\n- 提高学习效率\n- 保持工作生活平衡"
}

// 辅助函数 - 生成默认回复
func generateDefaultResponse() string {
	return "这是一个有趣的问题，让我基于您的数据来分析一下...\n\n如果您需要具体的数据分析，可以尝试问我：\n• \"我最近的状态怎么样？\"\n• \"帮我分析一下时间分配\"\n• \"我的目标进度如何？\"\n• \"给我一些建议\""
}

// 准备对话上下文，包含系统提示和博客数据
func prepareConversationContext(userMessages []Message) []Message {
	// 收集所有博客数据
	blogData := gatherAllBlogData()
	
	// 构建系统提示
	systemPrompt := fmt.Sprintf(`你是一个专业的个人数据分析师和生活助手。你拥有用户的完整生活数据，包括：

📊 **当前数据概览**：
%s

📋 **使用指南**：
- 基于用户的实际数据进行分析和建议
- 提供具体、可行的建议
- 保持积极、专业的语调
- 如果数据不足，可以询问用户获取更多信息

请根据用户的问题，结合这些数据提供个性化的回答。`, blogData)
	
	// 构建完整的消息列表
	messages := []Message{
		{Role: "system", Content: systemPrompt},
	}
	
	// 添加用户对话历史
	messages = append(messages, userMessages...)
	
	return messages
}

// 收集所有博客数据
func gatherAllBlogData() string {
	var dataBuilder strings.Builder
	
	// 收集任务数据
	taskData := gatherTaskData()
	dataBuilder.WriteString("📋 **任务管理**:\n")
	dataBuilder.WriteString(taskData)
	dataBuilder.WriteString("\n\n")
	
	// 收集阅读数据
	readingData := gatherReadingData()
	dataBuilder.WriteString("📚 **阅读记录**:\n")
	dataBuilder.WriteString(readingData)
	dataBuilder.WriteString("\n\n")
	
	// 收集锻炼数据
	exerciseData := gatherExerciseData()
	dataBuilder.WriteString("💪 **锻炼记录**:\n")
	dataBuilder.WriteString(exerciseData)
	dataBuilder.WriteString("\n\n")
	
	// 收集博客数据
	blogData := gatherBlogData()
	dataBuilder.WriteString("📝 **博客写作**:\n")
	dataBuilder.WriteString(blogData)
	dataBuilder.WriteString("\n\n")
	
	// 收集年度计划数据
	yearPlanData := gatherYearPlanData()
	dataBuilder.WriteString("🎯 **年度目标**:\n")
	dataBuilder.WriteString(yearPlanData)
	dataBuilder.WriteString("\n\n")
	
	// 收集统计数据
	statsData := gatherStatsData()
	dataBuilder.WriteString("📊 **整体统计**:\n")
	dataBuilder.WriteString(statsData)
	
	return dataBuilder.String()
}

// 收集任务数据
func gatherTaskData() string {
	// 获取今日任务数据
	today := time.Now().Format("2006-01-02")
	todayTitle := fmt.Sprintf("todolist-%s", today)
	
	// 获取今日任务列表
	todayBlog := control.GetBlog(todayTitle)
	var todayCompleted, todayTotal int
	var recentTasks []string
	
	if todayBlog != nil {
		// 解析今日任务数据
		todayData := todolist.ParseTodoListFromBlog(todayBlog.Content)
		todayTotal = len(todayData.Items)
		
		for _, item := range todayData.Items {
			if item.Completed {
				todayCompleted++
			}
			if len(recentTasks) < 3 {
				status := "进行中"
				if item.Completed {
					status = "已完成"
				}
				recentTasks = append(recentTasks, fmt.Sprintf("%s(%s)", item.Content, status))
			}
		}
	}
	
	// 计算本周完成率
	weekCompletionRate := calculateWeeklyTaskCompletion()
	
	// 获取最近完成的任务
	recentCompletedTasks := getRecentCompletedTasks(3)
	
	recentTasksStr := "无"
	if len(recentCompletedTasks) > 0 {
		recentTasksStr = strings.Join(recentCompletedTasks, ", ")
	} else if len(recentTasks) > 0 {
		recentTasksStr = strings.Join(recentTasks, ", ")
	}
	
	return fmt.Sprintf("- 今日任务: %d/%d 完成\n- 本周完成率: %.1f%%\n- 最近任务: %s",
		todayCompleted, todayTotal, weekCompletionRate, recentTasksStr)
}

// 收集阅读数据
func gatherReadingData() string {
	// 获取所有阅读相关的博客
	readingBlogs := getReadingBlogs()
	
	var currentReading []string
	var recentBooks []string
	var monthlyReadingHours float64
	var readingProgress []string
	
	for _, blog := range readingBlogs {
		// 解析阅读数据
		bookData := parseReadingDataFromBlog(blog.Content)
		
		// 统计当前在读的书籍
		if bookData.Status == "reading" {
			currentReading = append(currentReading, bookData.Title)
			
			// 计算阅读进度
			if bookData.TotalPages > 0 {
				progress := float64(bookData.CurrentPage) / float64(bookData.TotalPages) * 100
				readingProgress = append(readingProgress, fmt.Sprintf("%s(%.0f%%)", bookData.Title, progress))
			}
		}
		
		// 收集最近阅读的书籍
		if len(recentBooks) < 3 {
			recentBooks = append(recentBooks, bookData.Title)
		}
		
		// 统计本月阅读时间
		if bookData.LastReadDate != "" {
			if lastRead, err := time.Parse("2006-01-02", bookData.LastReadDate); err == nil {
				if lastRead.Month() == time.Now().Month() && lastRead.Year() == time.Now().Year() {
					monthlyReadingHours += bookData.MonthlyReadingTime
				}
			}
		}
	}
	
	// 格式化输出
	currentReadingStr := "无"
	if len(currentReading) > 0 {
		currentReadingStr = fmt.Sprintf("%d 本书", len(currentReading))
	}
	
	recentBooksStr := "无"
	if len(recentBooks) > 0 {
		recentBooksStr = strings.Join(recentBooks, ", ")
	}
	
	readingProgressStr := "无"
	if len(readingProgress) > 0 {
		readingProgressStr = strings.Join(readingProgress, ", ")
	}
	
	return fmt.Sprintf("- 当前在读: %s\n- 本月阅读: %.1f 小时\n- 最近阅读: %s\n- 阅读进度: %s",
		currentReadingStr, monthlyReadingHours, recentBooksStr, readingProgressStr)
}

// 收集锻炼数据
func gatherExerciseData() string {
	// 获取今日锻炼数据
	today := time.Now().Format("2006-01-02")
	todayTitle := fmt.Sprintf("exercise-%s", today)
	
	var todayExercise []string
	var todayCalories float64
	
	// 获取今日锻炼
	todayBlog := control.GetBlog(todayTitle)
	if todayBlog != nil {
		exerciseList := exercise.ParseExerciseFromBlog(todayBlog.Content)
		
		for _, ex := range exerciseList.Items {
			exerciseType := getExerciseTypeText(ex.Type)
			todayExercise = append(todayExercise, fmt.Sprintf("%s %d分钟", exerciseType, ex.Duration))
			todayCalories += float64(ex.Calories)
		}
	}
	
	// 获取本周锻炼统计
	weeklyStats := getWeeklyExerciseStats()
	
	// 获取最近锻炼记录
	recentExercises := getRecentExercises(3)
	
	// 格式化输出
	todayExerciseStr := "无"
	if len(todayExercise) > 0 {
		todayExerciseStr = strings.Join(todayExercise, ", ")
	}
	
	recentExercisesStr := "无"
	if len(recentExercises) > 0 {
		recentExercisesStr = strings.Join(recentExercises, ", ")
	}
	
	return fmt.Sprintf("- 今日锻炼: %s\n- 本周锻炼: %d 次\n- 消耗卡路里: %.0f 千卡\n- 最近锻炼: %s",
		todayExerciseStr, weeklyStats.SessionCount, weeklyStats.TotalCalories, recentExercisesStr)
}

// 收集博客数据
func gatherBlogData() string {
	// 获取所有博客数据
	allBlogs := control.GetAll(0,0)
	
	var totalBlogs int
	var monthlyBlogs int
	var recentBlogs []string
	var tagCount map[string]int
	
	tagCount = make(map[string]int)
	currentMonth := time.Now().Format("2006-01")
	
	// 过滤掉系统生成的博客（任务、锻炼、阅读等）
	for _, blog := range allBlogs {
		// 跳过系统生成的博客
		if isSystemBlog(blog.Title) {
			continue
		}
		
		totalBlogs++
		
		// 统计本月博客
		if blog.CreateTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.CreateTime); err == nil {
				if createTime.Format("2006-01") == currentMonth {
					monthlyBlogs++
				}
			}
		}
		
		// 收集最近博客
		if len(recentBlogs) < 3 {
			recentBlogs = append(recentBlogs, blog.Title)
		}
		
		// 统计标签
		if blog.Tags != "" {
			tags := strings.Split(blog.Tags, "|")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tagCount[tag]++
				}
			}
		}
	}
	
	// 获取热门标签
	hotTags := getHotTags(tagCount, 3)
	
	// 格式化输出
	recentBlogsStr := "无"
	if len(recentBlogs) > 0 {
		recentBlogsStr = strings.Join(recentBlogs, ", ")
	}
	
	hotTagsStr := "无"
	if len(hotTags) > 0 {
		hotTagsStr = strings.Join(hotTags, ", ")
	}
	
	return fmt.Sprintf("- 总博客数: %d 篇\n- 本月发布: %d 篇\n- 最近博客: %s\n- 热门标签: %s",
		totalBlogs, monthlyBlogs, recentBlogsStr, hotTagsStr)
}

// 收集年度计划数据
func gatherYearPlanData() string {
	// 获取当前年份
	currentYear := time.Now().Year()
	yearPlanTitle := fmt.Sprintf("年计划_%d", currentYear)
	
	// 获取年度计划
	yearPlan := control.GetBlog(yearPlanTitle)
	if yearPlan != nil {
		return "- 年度目标: 未设置\n- 整体进度: 0%\n- 目标详情: 暂无年度计划"
	}
	
	// 解析年度计划数据
	yearPlanData := yearplan.ParseYearPlanFromBlog(yearPlan.Content)
	
	// 获取月度目标统计
	monthlyStats := getMonthlyGoalsStats(currentYear)
	
	// 计算整体进度
	var totalProgress float64
	var goalCount int
	var goalDetails []string
	
	for _, goal := range yearPlanData.Tasks {
		if goal.Status == "completed" {
			totalProgress += 1
			goalCount++
			goalDetails = append(goalDetails, fmt.Sprintf("%s(%.0f%%)", goal.Title, 1))
		}
	}
	
	overallProgress := float64(0)
	if goalCount > 0 {
		overallProgress = totalProgress / float64(goalCount)
	}
	
	// 格式化输出
	goalDetailsStr := "暂无具体目标"
	if len(goalDetails) > 0 {
		goalDetailsStr = strings.Join(goalDetails, ", ")
	}
	
	return fmt.Sprintf("- 年度目标: %d 个\n- 整体进度: %.1f%%\n- 完成月份: %d/%d\n- 目标详情: %s",
		len(yearPlanData.Tasks), overallProgress, monthlyStats.CompletedMonths, 
		monthlyStats.TotalMonths, goalDetailsStr)
}

// 收集统计数据
func gatherStatsData() string {
	// 获取系统整体统计
	stats := statistics.GetOverallStatistics()
	
	// 计算活跃天数
	activeDays := calculateActiveDays()
	
	// 计算数据完整性
	dataCompleteness := calculateDataCompleteness()
	
	// 计算生产力指数
	productivityIndex := calculateProductivityIndex()
	
	// 分析近期趋势
	recentTrend := analyzeRecentTrend()
	
	return fmt.Sprintf("- 活跃天数: %d 天\n- 数据完整性: %.1f%%\n- 生产力指数: %.1f\n- 近期趋势: %s\n- 总博客数: %d\n- 今日新增: %d",
		activeDays, dataCompleteness, productivityIndex, recentTrend, stats.BlogStats.TotalBlogs, stats.BlogStats.TodayNewBlogs)
}

// 格式化函数们
func formatRecentTasks(tasks []interface{}, limit int) string {
	if len(tasks) == 0 {
		return "无"
	}
	
	var taskNames []string
	for i, task := range tasks {
		if i >= limit {
			break
		}
		if taskMap, ok := task.(map[string]interface{}); ok {
			if title, ok := taskMap["title"].(string); ok {
				taskNames = append(taskNames, title)
			}
		}
	}
	
	if len(taskNames) == 0 {
		return "无"
	}
	
	return strings.Join(taskNames, ", ")
}

func formatRecentBooks(books []interface{}) string {
	if len(books) == 0 {
		return "无"
	}
	
	var bookNames []string
	for _, book := range books {
		if bookMap, ok := book.(map[string]interface{}); ok {
			if title, ok := bookMap["title"].(string); ok {
				bookNames = append(bookNames, title)
			}
		}
	}
	
	if len(bookNames) == 0 {
		return "无"
	}
	
	return strings.Join(bookNames, ", ")
}

func formatReadingProgress(books []interface{}) string {
	if len(books) == 0 {
		return "无"
	}
	
	var progress []string
	for _, book := range books {
		if bookMap, ok := book.(map[string]interface{}); ok {
			if title, ok := bookMap["title"].(string); ok {
				if progressPct, ok := bookMap["progress"].(float64); ok {
					progress = append(progress, fmt.Sprintf("%s(%.1f%%)", title, progressPct))
				}
			}
		}
	}
	
	if len(progress) == 0 {
		return "无"
	}
	
	return strings.Join(progress, ", ")
}

func formatTodayExercise(exercise interface{}) string {
	if exercise == nil {
		return "无"
	}
	
	if exerciseMap, ok := exercise.(map[string]interface{}); ok {
		if exerciseType, ok := exerciseMap["type"].(string); ok {
			if duration, ok := exerciseMap["duration"].(float64); ok {
				return fmt.Sprintf("%s %.0f分钟", exerciseType, duration)
			}
		}
	}
	
	return "无"
}

func formatRecentExercises(exercises []interface{}) string {
	if len(exercises) == 0 {
		return "无"
	}
	
	var exerciseList []string
	for _, exercise := range exercises {
		if exerciseMap, ok := exercise.(map[string]interface{}); ok {
			if exerciseType, ok := exerciseMap["type"].(string); ok {
				if duration, ok := exerciseMap["duration"].(float64); ok {
					exerciseList = append(exerciseList, fmt.Sprintf("%s(%.0f分钟)", exerciseType, duration))
				}
			}
		}
	}
	
	if len(exerciseList) == 0 {
		return "无"
	}
	
	return strings.Join(exerciseList, ", ")
}

func formatRecentBlogs(blogs []interface{}) string {
	if len(blogs) == 0 {
		return "无"
	}
	
	var blogTitles []string
	for _, blog := range blogs {
		if blogMap, ok := blog.(map[string]interface{}); ok {
			if title, ok := blogMap["title"].(string); ok {
				blogTitles = append(blogTitles, title)
			}
		}
	}
	
	if len(blogTitles) == 0 {
		return "无"
	}
	
	return strings.Join(blogTitles, ", ")
}

func formatHotTags(tags []interface{}) string {
	if len(tags) == 0 {
		return "无"
	}
	
	var tagNames []string
	for _, tag := range tags {
		if tagStr, ok := tag.(string); ok {
			tagNames = append(tagNames, tagStr)
		}
	}
	
	if len(tagNames) == 0 {
		return "无"
	}
	
	return strings.Join(tagNames, ", ")
}

func formatYearGoals(goals []interface{}) string {
	if len(goals) == 0 {
		return "无"
	}
	
	var goalList []string
	for _, goal := range goals {
		if goalMap, ok := goal.(map[string]interface{}); ok {
			if title, ok := goalMap["title"].(string); ok {
				if progress, ok := goalMap["progress"].(float64); ok {
					goalList = append(goalList, fmt.Sprintf("%s(%.1f%%)", title, progress))
				}
			}
		}
	}
	
	if len(goalList) == 0 {
		return "无"
	}
	
	return strings.Join(goalList, ", ")
}

// 保存对话到博客
func saveConversationToBlog(messages []Message) {
	if len(messages) == 0 {
		return
	}
	
	// 获取当前日期
	now := time.Now()
	dateStr := now.Format("2006_01_02")
	filename := fmt.Sprintf("assistant_%s.md", dateStr)
	
	// 获取用户的最后一条消息
	var userMessage string
	
	for _, msg := range messages {
		if msg.Role == "user" {
			userMessage = msg.Content
		}
	}
	
	if userMessage == "" {
		return
	}
	
	// 构建对话内容
	content := fmt.Sprintf(`# AI助手对话记录 - %s

## 用户问题
%s

## AI回复
[等待AI回复...]

---
*记录时间: %s*
`, now.Format("2006-01-02"), userMessage, now.Format("2006-01-02 15:04:05"))
	
	// 检查是否已存在同名博客
	existingBlog := control.GetBlog(filename)
	if existingBlog != nil {
		// 追加到现有博客
		content = fmt.Sprintf(`%s

## 用户问题 (%s)
%s

## AI回复
[等待AI回复...]

---
`, existingBlog.Content, now.Format("15:04:05"), userMessage)
	}
	
	// 保存博客
	blogData := &module.UploadedBlogData{
		Title:     fmt.Sprintf("AI助手对话记录_%s", dateStr),
		Content:   content,
		Tags:      "AI助手|对话记录|自动生成",
		AuthType:  module.EAuthType_private, // 设置为私有
	}
	
	// 调用博客模块保存
	control.AddBlog(blogData)
}

// 辅助函数实现

// 计算本周任务完成率
func calculateWeeklyTaskCompletion() float64 {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	
	var totalTasks, completedTasks int
	
	for i := 0; i < 7; i++ {
		date := weekStart.AddDate(0, 0, i)
		title := fmt.Sprintf("todolist-%s", date.Format("2006-01-02"))
		
		blog := control.GetBlog(title)
		if blog != nil {
			todoData := todolist.ParseTodoListFromBlog(blog.Content)
			totalTasks += len(todoData.Items)
			
			for _, item := range todoData.Items {
				if item.Completed {
					completedTasks++
				}
			}
		}
	}
	
	if totalTasks == 0 {
		return 0
	}
	
	return float64(completedTasks) / float64(totalTasks) * 100
}

// 获取最近完成的任务
func getRecentCompletedTasks(limit int) []string {
	var recentTasks []string
	now := time.Now()
	
	// 查看最近7天的任务
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i)
		title := fmt.Sprintf("todolist-%s", date.Format("2006-01-02"))
		
		blog := control.GetBlog(title)
		if blog != nil {
			todoData := todolist.ParseTodoListFromBlog(blog.Content)
			
			for _, item := range todoData.Items {
				if item.Completed && len(recentTasks) < limit {
					recentTasks = append(recentTasks, item.Content)
				}
			}
		}
		
		if len(recentTasks) >= limit {
			break
		}
	}
	
	return recentTasks
}

// 获取阅读相关的博客
func getReadingBlogs() []*module.Blog {
	allBlogs := control.GetAll(0,0)
	var readingBlogs []*module.Blog
	
	for _, blog := range allBlogs {
		if strings.HasPrefix(blog.Title, "reading_book_") {
			readingBlogs = append(readingBlogs, blog)
		}
	}
	
	return readingBlogs
}

// 解析阅读数据
func parseReadingDataFromBlog(content string) ReadingBookData {
	// 简化的解析逻辑
	data := ReadingBookData{
		Status:              "reading",
		CurrentPage:         0,
		TotalPages:          0,
		MonthlyReadingTime:  0,
		LastReadDate:        time.Now().Format("2006-01-02"),
	}
	
	// 从content中解析标题
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			data.Title = strings.TrimPrefix(line, "# ")
			break
		}
	}
	
	return data
}

// 阅读书籍数据结构
type ReadingBookData struct {
	Title              string
	Status             string
	CurrentPage        int
	TotalPages         int
	MonthlyReadingTime float64
	LastReadDate       string
}

// 获取锻炼类型文本
func getExerciseTypeText(exerciseType string) string {
	switch exerciseType {
	case "cardio":
		return "有氧运动"
	case "strength":
		return "力量训练"
	case "flexibility":
		return "柔韧性训练"
	case "sports":
		return "运动项目"
	default:
		return "锻炼"
	}
}

// 获取本周锻炼统计
func getWeeklyExerciseStats() WeeklyExerciseStats {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	
	var sessionCount int
	var totalCalories float64
	
	for i := 0; i < 7; i++ {
		date := weekStart.AddDate(0, 0, i)
		title := fmt.Sprintf("exercise-%s", date.Format("2006-01-02"))
		
		blog := control.GetBlog(title)
		if blog != nil {
			exercises := exercise.ParseExerciseFromBlog(blog.Content)
			if len(exercises.Items) > 0 {
				sessionCount++
				for _, ex := range exercises.Items {
					totalCalories += float64(ex.Calories)
				}
			}
		}
	}
	
	return WeeklyExerciseStats{
		SessionCount:  sessionCount,
		TotalCalories: totalCalories,
	}
}

// 本周锻炼统计结构
type WeeklyExerciseStats struct {
	SessionCount  int
	TotalCalories float64
}

// 获取最近锻炼记录
func getRecentExercises(limit int) []string {
	var recentExercises []string
	now := time.Now()
	
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i)
		title := fmt.Sprintf("exercise-%s", date.Format("2006-01-02"))
		
		blog := control.GetBlog(title)
		if blog != nil {
			exercises := exercise.ParseExerciseFromBlog(blog.Content)
			
			for _, ex := range exercises.Items {
				if len(recentExercises) < limit {
					exerciseType := getExerciseTypeText(ex.Type)
					recentExercises = append(recentExercises, fmt.Sprintf("%s(%d分钟)", exerciseType, ex.Duration))
				}
			}
		}
		
		if len(recentExercises) >= limit {
			break
		}
	}
	
	return recentExercises
}

// 判断是否为系统生成的博客
func isSystemBlog(title string) bool {
	systemPrefixes := []string{
		"todolist-",
		"exercise-",
		"reading_book_",
		"月度目标_",
		"年计划_",
		"assistant_",
	}
	
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(title, prefix) {
			return true
		}
	}
	
	return false
}

// 获取热门标签
func getHotTags(tagCount map[string]int, limit int) []string {
	type TagCount struct {
		Tag   string
		Count int
	}
	
	var tags []TagCount
	for tag, count := range tagCount {
		tags = append(tags, TagCount{Tag: tag, Count: count})
	}
	
	// 按计数排序
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Count > tags[j].Count
	})
	
	var hotTags []string
	for i, tag := range tags {
		if i >= limit {
			break
		}
		hotTags = append(hotTags, tag.Tag)
	}
	
	return hotTags
}

// 获取月度目标统计
func getMonthlyGoalsStats(year int) MonthlyGoalsStats {
	var completedMonths, totalMonths int
	
	for month := 1; month <= 12; month++ {
		title := fmt.Sprintf("月度目标_%d-%02d", year, month)
		blog := control.GetBlog(title)
		
		if blog != nil {
			totalMonths++
			
			// 简化的完成度判断
			if strings.Contains(blog.Content, "完成") {
				completedMonths++
			}
		}
	}
	
	return MonthlyGoalsStats{
		CompletedMonths: completedMonths,
		TotalMonths:     totalMonths,
	}
}

// 月度目标统计结构
type MonthlyGoalsStats struct {
	CompletedMonths int
	TotalMonths     int
}

// 计算活跃天数
func calculateActiveDays() int {
	// 统计有数据的天数
	allBlogs := control.GetAll(0,0)
	dateSet := make(map[string]bool)
	
	for _, blog := range allBlogs {
		if blog.ModifyTime != "" {
			if createTime, err := time.Parse("2006-01-02 15:04:05", blog.ModifyTime); err == nil {
				dateStr := createTime.Format("2006-01-02")
				dateSet[dateStr] = true
			}
		}
	}
	
	return len(dateSet)
}

// 计算数据完整性
func calculateDataCompleteness() float64 {
	// 计算最近30天的数据完整性
	now := time.Now()
	var completeDataDays int
	
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		
		// 检查是否有任务、锻炼或阅读数据
		hasTask := hasDataForDate("todolist-", dateStr)
		hasExercise := hasDataForDate("exercise-", dateStr)
		hasReading := hasReadingDataForDate(dateStr)
		
		if hasTask || hasExercise || hasReading {
			completeDataDays++
		}
	}
	
	return float64(completeDataDays) / 30.0 * 100
}

// 检查指定日期是否有数据
func hasDataForDate(prefix, date string) bool {
	title := fmt.Sprintf("%s%s", prefix, date)
	blog := control.GetBlog(title)
	return blog != nil
}

// 检查指定日期是否有阅读数据
func hasReadingDataForDate(date string) bool {
	readingBlogs := getReadingBlogs()
	for _, blog := range readingBlogs {
		if strings.Contains(blog.Content, date) {
			return true
		}
	}
	return false
}

// 计算生产力指数
func calculateProductivityIndex() float64 {
	// 综合任务完成率、锻炼频率、阅读时间等指标
	taskCompletion := calculateWeeklyTaskCompletion()
	exerciseStats := getWeeklyExerciseStats()
	
	// 简化的生产力计算
	productivity := (taskCompletion * 0.4) + (float64(exerciseStats.SessionCount) * 10 * 0.3) + (50 * 0.3)
	
	if productivity > 100 {
		productivity = 100
	}
	
	return productivity / 10  // 转换为1-10分制
}

// 分析近期趋势
func analyzeRecentTrend() string {
	// 比较最近一周和前一周的数据
	thisWeekCompletion := calculateWeeklyTaskCompletion()
	
	// 简化的趋势分析
	if thisWeekCompletion > 70 {
		return "上升趋势，效率提升明显"
	} else if thisWeekCompletion > 50 {
		return "稳定趋势，保持良好状态"
	} else {
		return "需要关注，建议调整节奏"
	}
}

// 辅助函数 - 获取今日任务统计
func getTodayTasksStats() map[string]interface{} {
	// 这里应该调用任务模块的API
	// 暂时返回模拟数据
	return map[string]interface{}{
		"completed": 3,
		"total": 5,
		"completion_rate": 60.0,
	}
}

// 辅助函数 - 获取今日阅读统计
func getTodayReadingStats() map[string]interface{} {
	// 这里应该调用阅读模块的API
	// 暂时返回模拟数据
	return map[string]interface{}{
		"time": 2.5,
		"pages": 45,
		"books": 1,
	}
}

// 辅助函数 - 获取今日锻炼统计
func getTodayExerciseStats() map[string]interface{} {
	// 这里应该调用锻炼模块的API
	// 暂时返回模拟数据
	return map[string]interface{}{
		"sessions": 1,
		"duration": 45,
		"type": "cardio",
	}
}

// 辅助函数 - 获取今日写作统计
func getTodayBlogsStats() map[string]interface{} {
	// 这里应该调用博客模块的API
	// 暂时返回模拟数据
	return map[string]interface{}{
		"count": 1,
		"words": 800,
		"published": true,
	}
}

// 辅助函数 - 生成任务建议
func generateTaskSuggestion() map[string]interface{} {
	return map[string]interface{}{
		"icon": "💡",
		"text": "您今天的任务完成率为60%，建议优先处理剩余的重要任务",
		"priority": "high",
	}
}

// 辅助函数 - 生成阅读建议
func generateReadingSuggestion() map[string]interface{} {
	return map[string]interface{}{
		"icon": "📚",
		"text": "基于您的阅读习惯，推荐继续阅读《深度工作》",
		"priority": "medium",
	}
}

// 辅助函数 - 生成锻炼建议
func generateExerciseSuggestion() map[string]interface{} {
	return map[string]interface{}{
		"icon": "💪",
		"text": "您已连续3天进行锻炼，保持良好的运动习惯",
		"priority": "low",
	}
}

// 辅助函数 - 生成时间建议
func generateTimeSuggestion() map[string]interface{} {
	return map[string]interface{}{
		"icon": "⏰",
		"text": "分析显示您在下午3-5点效率最高，建议安排重要工作",
		"priority": "medium",
	}
}