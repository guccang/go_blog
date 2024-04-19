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
)


func Info(){
	log.Debug("info http v1.0")
}

type handle_content struct{
	content string
}

func checkLogin(r *h.Request) int{
	//session := r.URL.Query().Get("session")
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
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
		return
	}
	view.PageEditor(w)
}



func  HandleLink(w h.ResponseWriter, r *h.Request){
	if checkLogin(r) != 0{
		h.Redirect(w,r,"/index",302)
		return
	}
	view.PageLink(w)
}

func HandleStatics(w h.ResponseWriter, r *h.Request){
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

	// tags
	tags := r.FormValue("tags")
	log.DebugF("Received content:%s",tags)
	
	ubd := module.UploadedBlogData {
		Title : title,
		Content : content,
		AuthType : auth_type,
		Tags : tags,
	}

	ret := control.AddBlog(&ubd)

	// 响应客户端
	if ret==0 {
		w.Write([]byte(fmt.Sprintf("save successfully! ret=%d",ret)))
	}else{
		w.Write([]byte(fmt.Sprintf("save failed! has same title blog ret=%d",ret)))
	}
}

func HandleHelp(w h.ResponseWriter, r *h.Request){
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

func HandleGet(w h.ResponseWriter, r *h.Request){
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

	view.PageGetBlog(blogname,w,usepublic)
}

func HandleComment(w h.ResponseWriter, r *h.Request){
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

func HandleModify(w h.ResponseWriter, r *h.Request){
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
	

	content := r.FormValue("content")
	// 在这里，您可以处理或保存content到数据库等
	log.DebugF("Received content:%s", content)

	ubd := module.UploadedBlogData {
		Title : title,
		Content : content,
		AuthType : auth_type,
		Tags : tags,
	}

	ret := control.ModifyBlog(&ubd)

	// 响应客户端
	w.Write([]byte(fmt.Sprintf("Content received successfully! ret=%d",ret)))

}


func HandleSearch(w h.ResponseWriter,r *h.Request){
	if checkLogin(r) !=0 {
		h.Redirect(w,r,"/index",302)
		return
	}
	match := r.URL.Query().Get("match")

	if match == "$help" {
		h.Redirect(w,r,"/help",302)
		return
	}

	view.PageSearch(match,w)
}

func HandleLogin(w h.ResponseWriter,r *h.Request){

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
		h.Error(w,"Error account or pwd",h.StatusBadRequest)
		return
	}

	// set cookie
	cookie := &h.Cookie{
		Name:    "session",
		Value:   session,
		Expires: time.Now().Add(48 * time.Hour), // 过期时间为两天
	}
	h.SetCookie(w, cookie)
	
	log.DebugF("login success account=%s pwd=%s session=%s",account,pwd,session)
	h.Redirect(w, r,"/link", 302)
}

func HandleIndex(w h.ResponseWriter,r *h.Request){
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


func Init() int{
	h.HandleFunc("/link",HandleLink)
	h.HandleFunc("/editor",HandleEditor)
	h.HandleFunc("/statics",HandleStatics)
	h.HandleFunc("/save",HandleSave)
	h.HandleFunc("/get",HandleGet)
	h.HandleFunc("/modify",HandleModify)
	h.HandleFunc("/search",HandleSearch)
	h.HandleFunc("/login",HandleLogin)
	h.HandleFunc("/index",HandleIndex)
	h.HandleFunc("/help",HandleHelp)
	h.HandleFunc("/comment",HandleComment)

	root := config.GetHttpStaticPath()
	fs := h.FileServer(h.Dir(root))
	h.Handle("/", h.StripPrefix("/", fs))
	//h.Handle("/", h.StripPrefix("/",basicAuth(fs)))
	return 0
}

func Run() int{
	Init()
	port := config.GetConfig("port")
	h.ListenAndServe(fmt.Sprintf(":%s",port),nil)
	return 0;
}

func Stop() int {
	return 0;
}
