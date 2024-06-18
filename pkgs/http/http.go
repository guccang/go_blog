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
	"strings"
	"strconv"
	"share"
	"cooperation"
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
		w.Write([]byte(fmt.Sprintf("save failed! title is invalied")))
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
			w.Write([]byte(fmt.Sprintf("save error  aes not match error!")))
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
			w.Write([]byte(fmt.Sprintf("save failed! cooperation auth error, timed blog not support")))
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
		w.Write([]byte(fmt.Sprintf("save failed! has same title blog ret=%d",ret)))
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

	session := getsession(r)
	if cooperation.IsCooperation(session) {
		// 判定blog访问权限
		auth_type := control.GetBlogAuthType(blogname)
		if auth_type == module.EAuthType_private {
			h.Redirect(w,r,"/index",302)
			return
		}
		if cooperation.CanEditBlog(session,blogname) != 0 {
			usepublic = 1
		}	
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
		w.Write([]byte(fmt.Sprintf("save failed! title is invalied")))
		return
	}

	log.DebugF("comment title:%s",title)
	
	owner := r.FormValue("owner")
	pwd := r.FormValue("pwd")
	mail :=  r.FormValue("mail")
	comment := r.FormValue("comment")

	if owner == "" {
		// ipPort
		owner = r.RemoteAddr
	}

	if pwd == "" {
		// 
		pwd = r.RemoteAddr
	}

	if mail == "" {
	}

	if comment == "" {
		w.Write([]byte(fmt.Sprintf("save failed! comment is invalied")))
		return 
	}

	control.AddComment(title,comment,owner,pwd,mail)

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
			w.Write([]byte(fmt.Sprintf("save error  aes not match error! ret=%s")))
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
	session := getsession(r)
	is_cooperation := cooperation.IsCooperation(session)


	// 直接显示help
	tokens := strings.Split(match," ")
	if match == "@help" {
		h.Redirect(w,r,"/help",302)
		return
	}
    // 直接显示主页
	if match == "@main" {
		h.Redirect(w,r,"/link",302)
		return
	}
	// 创建timed blog
	if tokens[0] == "@c" {
		if is_cooperation {
			h.Error(w, "@c auth not support", h.StatusBadRequest)
			return
		}
		if len(tokens) != 2 {
			h.Error(w, "@c titlename need", h.StatusBadRequest)
			return
		}
		title := tokens[1]
		content := ""
		b := control.GetRecentlyTimedBlog(title)
		if b != nil {
			content = b.Content
		}
		view.PageEditor(w,title,content)
		return
	}
	// 分享private连接
	if tokens[0] == "@share" && len(tokens)>=2 {
		if is_cooperation {
			h.Error(w, "@c auth not support", h.StatusBadRequest)
			return
		}
	
		// 创建分享
		if tokens[1] == "c" && len(tokens)>=3 {
			blogname := tokens[2]
			view.PageShareBlog(w,blogname)
		}
		if tokens[1] == "t" && len(tokens)>=3{
			tag := tokens[2]
			view.PageShareTag(w,tag)
		}
		// 显示所有创建的分享
		if tokens[1] == "all" {
			if false == is_cooperation {
				view.PageShowAllShare(w)
			}else{
				w.Write([]byte("not support operation (showAllShare)!!!"))		
			}
		}
		return
	}
	// 创建协作账号
	if tokens[0] == "@cooperation" && len(tokens) >= 2{
		log.DebugF("cooperation opt=%s",tokens[1])
		if is_cooperation {
			h.Error(w, "@c auth not support", h.StatusBadRequest)
			return
		}
	
		// 创建
		if tokens[1] == "c" && len(tokens) == 3{
			account := tokens[2]
			view.PageAddCooperation(w,account)
		}
		// 删除
		if tokens[1] == "d" && len(tokens) == 3{
			account := tokens[2]
			view.PageDelCooperation(w,account)
		}
		// 显示
		if tokens[1] == "all" && len(tokens) == 2{
			if false == is_cooperation {
				view.PageShowCooperation(w)
			}else{
				w.Write([]byte("not support operation (showCooperation)!!!"))		
			}
		}
		// add edit blog
		if tokens[1] == "addblog" && len(tokens) == 4{
			account := tokens[2]
			blog := tokens[3]
			view.PageAddCooperationBlog(w,account,blog)
		}
		if tokens[1] == "delblog" && len(tokens) == 4{
			account := tokens[2]
			blog := tokens[3]
			view.PageDelCooperationBlog(w,account,blog)
		}
		// add edit tag
		if tokens[1] == "addtag" && len(tokens) == 4{
			account := tokens[2]
			tag := tokens[3]
			view.PageAddCooperationTag(w,account,tag)
		}
		if tokens[1] == "deltag" && len(tokens) == 4{
			account := tokens[2]
			tag := tokens[3]
			view.PageDelCooperationTag(w,account,tag)
		}
		return
	}

	// 通用搜索逻辑
	view.PageSearch(match,w,session)
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
	session , ret:= login.Login(account,pwd)
	if ret != 0 {
		session,ret = cooperation.CooperationLogin(account,pwd)
		if ret != 0 {
			h.Error(w,"Error account or pwd",h.StatusBadRequest)
			return
		}
		log.DebugF("cooperation login ok account=%s pwd=%s",account,pwd)
	}

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

func HandleToDoList(w h.ResponseWriter,r *h.Request){
	LogRemoteAddr("HandleToDoList",r)
	view.PageToDoList(w)
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


func Init() int{
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
	h.HandleFunc("/d3",HandleD3)
	h.HandleFunc("/tag",HandleTag)
	h.HandleFunc("/getshare",HandleGetShare)
	h.HandleFunc("/todolist",HandleToDoList)

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
