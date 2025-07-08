package http

import(
	"fmt"
	"os"
	"path/filepath"
	h "net/http"
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
)

func Info(){
	log.Debug("info http v1.0")
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

	// 是否公开
	auth_type_string := r.FormValue("authtype")
	log.DebugF("Received content:%s",auth_type_string)
	auth_type := module.EAuthType_private
	if auth_type_string == "public" {
		auth_type = module.EAuthType_public
	}
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

	usepublic := 0
	// 权限检测成功使用private模板,可修改数据
	// 权限检测失败,并且为公开blog，使用public模板，只能查看数据
	if checkLogin(r) !=0 {
		// 判定blog访问权限
		session := getsession(r)
		auth_type := control.GetBlogAuthType(blogname)
		if cooperation.IsCooperation(session) {
			// 判定blog访问权限
			auth_type := control.GetBlogAuthType(blogname)
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

	// auth_type
	auth_type_string := r.FormValue("auth_type")
	log.DebugF("auth:%d",auth_type_string)
	auth_type := module.EAuthType_private
	if auth_type_string == "public" {
		auth_type = module.EAuthType_public
	}

	// tags
	tags := r.FormValue("tags")
	log.DebugF("Received content:%s",tags)

	// 内容
	content := r.FormValue("content")
	// 在这里，您可以处理或保存content到数据库等
	//log.DebugF("Received content:%s", content)


	// 加密
	encryptionKey := r.FormValue("encrypt")
	encrypt := 0
	log.DebugF("Received title=%s encrypt:%s session:%s",title,encrypt,getsession(r))

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
