package view

import(
	"fmt"
	"config"
	t "html/template"
	h "net/http"
	log "mylog"
	"path/filepath"
	"control"
	"module"
	"strings"
	"sort"
	"share"
	"cooperation"
)

func Info(){
	fmt.Println("info view v1.0")
}


type LinkData struct{
	URL string
	DESC string
	COOPERATION int
}

type LinkDatas struct{
	LINKS []LinkData
	VERSION string
	BLOGS_NUMBER int
	TAGS []string
}

type CommentDatas struct {
	IDX		int
	OWNER	string
	MSG		string
	CTIME	string
	MAIL	string
}

type EditorData struct{
	TITLE		string
	CONTENT		string
	CTIME		string
	AUTHTYPE	string
	TAGS		string
	COMMENTS	[]CommentDatas	
	ENCRYPT		string
}



func Notify(msg string,w h.ResponseWriter){
	tmpDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(tmpDir,"notify.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse markdown_editor",h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w,msg)
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to render template markdown_editor",h.StatusInternalServerError)
		return
	}
	fmt.Println("view Notify",msg)
}


func getShareLinks() *LinkDatas{
	datas := LinkDatas{}

	sharedblogs := share.SharedBlogs
	sharedtags  := share.SharedTags

	total_shared_data := len(sharedblogs) + len(sharedtags)
	datas.VERSION = fmt.Sprintf("%s|%d",config.GetVersion(),total_shared_data)
	datas.BLOGS_NUMBER = total_shared_data

	for _,b := range sharedblogs {
		ld := LinkData {
			URL:b.URL,
			DESC:b.Title,
			COOPERATION:0,
		}
		datas.LINKS = append(datas.LINKS,ld)
	}

	for _,t := range sharedtags {
		ld := LinkData {
			URL:t.URL,
			DESC:fmt.Sprintf("Tag-%s",t.Tag),
			COOPERATION:0,
		}
		datas.LINKS = append(datas.LINKS,ld)
	}

	return &datas
}


func getLinks(blogs []*module.Blog,flag int,session string) *LinkDatas{

	datas := LinkDatas{}
	datas.VERSION = fmt.Sprintf("%s|%d",config.GetVersion(),control.GetBlogsNum())
	datas.BLOGS_NUMBER = len(blogs)


	all_tags := make(map[string]int)

	for _,b := range blogs{

		// not show encrypt blog
		if (b.AuthType &  flag) == 0 {
			continue
		}

		if session != "" && cooperation.IsCooperation(session) {
			if cooperation.CanEditBlog(session,b.Title) != 0 {
				continue
			}
		}


		ld := LinkData {
			URL:fmt.Sprintf("/get?blogname=%s",b.Title),
			DESC:b.Title,
			COOPERATION:(b.AuthType & module.EAuthType_cooperation),
		}
		datas.LINKS = append(datas.LINKS,ld)

		tags := strings.Split(b.Tags,"|")
		for _,tag := range tags {
			if tag == "" {
				continue
			}
			cnt,ok := all_tags[tag]
			if !ok {
				all_tags[tag] = 1
			}else{
				all_tags[tag] = cnt + 1
			}
		}
	}

	for tag,_ := range all_tags {
		tags_str := fmt.Sprintf("$%s",tag)
		datas.TAGS = append(datas.TAGS,tags_str)
	}
	sort.Strings(datas.TAGS)

	return &datas
}

func PageSearch(match string,w h.ResponseWriter,session string){

	blogs := control.GetMatch(match)
	is_cooperation := cooperation.IsCooperation(session)
	flag := module.EAuthType_all
	if is_cooperation {
		flag = module.EAuthType_public | module.EAuthType_cooperation
	}
	
	datas := getLinks(blogs,flag,session)

	exeDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(exeDir,"link.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse link.template",h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w,datas)
	if err != nil{
		h.Error(w,"Failed to render template link.template",h.StatusInternalServerError)
		return
	}
}

func PageTags(w h.ResponseWriter,tag string){

	blogs := control.GetMatch("$"+tag)	

	flag := module.EAuthType_public
	// 只展示public

	datas := getLinks(blogs,flag,"")

	exeDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(exeDir,"tags.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse tags.template",h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w,datas)
	if err != nil{
		h.Error(w,"Failed to render template tags.template",h.StatusInternalServerError)
		return
	}

}

func PageLink(w h.ResponseWriter,flag int,session string){
	
	blogs := control.GetAll(100,flag)
	log.DebugF("blogs cnt=%d",len(blogs))

	datas := getLinks(blogs,flag,session)
	
	exeDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(exeDir,"link.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse link.template",h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w,datas)
	if err != nil{
		log.ErrorF("Failed to render template link.tempate err=%s",err.Error())
		h.Error(w,"Failed to render template link.template %s",h.StatusInternalServerError)
		return
	}
}

func PageEditor(w h.ResponseWriter,init_title string,init_content string){
	exeDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(exeDir,"markdown_editor.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse markdown_editor",h.StatusInternalServerError)
		return
	}

	title := "input title"
	content := "# input content"

	if len(init_title) > 0 {
		title = init_title
	}

	if len(init_content) > 0 {
		content = init_content
	}
	
	data := EditorData{
		TITLE:title,
		CONTENT:content,
		AUTHTYPE:"private",
		TAGS:"",
		ENCRYPT:"",
	}

	err = tmpl.Execute(w,data)
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to render template markdown_editor",h.StatusInternalServerError)
		return
	}
}

func PageGetBlog(blogname string,w h.ResponseWriter,usepublic int){
	blog := control.GetBlog(blogname)
	if blog == nil {
		h.Error(w, fmt.Sprintf("blogname=%s not find",blogname),h.StatusBadRequest)
		return
	}

	// modify accesstime
	control.UpdateAccessTime(blog)

	auth_type_string := "private"
	template_name := "get.template"
	if usepublic != 0 {
		template_name = "get_public.template"
	}
	if blog.AuthType == module.EAuthType_public {
		auth_type_string = "public"
	}

	tempDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(tempDir,template_name))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse get.template",h.StatusInternalServerError)
		return
	}
	

	encrypt_str := ""
	if blog.Encrypt == 1 {
		encrypt_str = "aes"
	}
	
	data := EditorData{
		TITLE:blog.Title,
		CONTENT:blog.Content,
		CTIME : blog.CreateTime,
		AUTHTYPE:auth_type_string,
		TAGS : blog.Tags,
		ENCRYPT:encrypt_str,
	}

	bc := control.GetBlogComments(blogname)
	if bc != nil {
		for _,c := range bc.Comments {
			cd := CommentDatas {
				IDX : c.Idx,
				OWNER: c.Owner,
				MSG : c.Msg,
				CTIME: c.CreateTime,
				MAIL: c.Mail,
			}
			data.COMMENTS = append(data.COMMENTS,cd)
		}
	}

	err = tmpl.Execute(w,data)
	if err != nil{
		h.Error(w,"Failed to render template get.template",h.StatusInternalServerError)
		return
	}

}

func PageIndex(w h.ResponseWriter){

	tempDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(tempDir,"login.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse get.template",h.StatusInternalServerError)
		return
	}
	
	
	err = tmpl.Execute(w,nil)
	if err != nil{
		h.Error(w,"Failed to render template get.template",h.StatusInternalServerError)
		return
	}

}

func PageD3(w h.ResponseWriter){

	tempDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(tempDir,"d3.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse get.template",h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w,nil)
	if err != nil{
		h.Error(w,"Failed to render template get.template",h.StatusInternalServerError)
		return
	}

}

// 将blogname设置为分享
func PageShareBlog(w h.ResponseWriter,blogname string){
	blog := control.GetBlog(blogname)
	if blog == nil {
		h.Error(w, fmt.Sprintf("blogname=%s not find",blogname),h.StatusBadRequest)
		return
	}
	url,pwd := share.AddSharedBlog(blogname)
	w.Write([]byte(fmt.Sprintf("PageShareBlog \n url=%s \n pwd=%s ",url,pwd)))
}

// 将tag设置为分享
func PageShareTag(w h.ResponseWriter, tag string){
	url,pwd := share.AddSharedTag(tag)
	w.Write([]byte(fmt.Sprintf("PageShareTag\n url=%s \n pwd=%s",url,pwd)))
}

// 返回所有分享
func PageShowAllShare(w h.ResponseWriter){
	tempDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(tempDir,"share.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse sharetemplate",h.StatusInternalServerError)
		return
	}

	shareddatas := getShareLinks()
	
	err = tmpl.Execute(w,shareddatas)
	if err != nil{
		h.Error(w,"Failed to render template share.template",h.StatusInternalServerError)
		return
	}
}

// todolist
func PageToDoList(w h.ResponseWriter){
	tempDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(tempDir,"todolist.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse todolist.template",h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w,nil)
	if err != nil{
		h.Error(w,"Failed to render template todolist.template",h.StatusInternalServerError)
		return
	}

}

func PageAddCooperation(w h.ResponseWriter,account string){
	ret,c := cooperation.CreateCooperation(account)
	if ret != 0 {
		h.Error(w, fmt.Sprintf("cooperation %s exit ret=%d c.pwd=%s",account,ret,c.Password),h.StatusBadRequest)
		return
	}
	w.Write([]byte(fmt.Sprintf("create cooperation \n account=%s \n pwd=%s ",c.Account,c.Password)))
}

func PageDelCooperation(w h.ResponseWriter,account string){
	ret := cooperation.DelCooperation(account)
	w.Write([]byte(fmt.Sprintf("delete cooperation \n account=%s ret=%d",account,ret)))
}

func PageShowCooperation(w h.ResponseWriter){
	cooperations := cooperation.Cooperations
	str := "All Cooperations:"
	for _,c := range cooperations {
		c_str := fmt.Sprintf("account=%s pwd=%s ct=%v",c.Account,c.Password,c.CreateTime)
		str = fmt.Sprintf("%s \n %s ",str,c_str)
	}
	w.Write([]byte(str))
}

func PageAddCooperationBlog(w h.ResponseWriter,account string, blogname string) {
	ret := cooperation.AddCanEditBlog(account,blogname)
	if ret != 0 {
		h.Error(w, fmt.Sprintf("cooperation addblog account=%s blog=%s ret=%d",account,blogname,ret),h.StatusBadRequest)
		return
	}
	w.Write([]byte(fmt.Sprintf("cooperation addblog \n account=%s \n blog=%s ",account,blogname)))
}

func PageDelCooperationBlog(w h.ResponseWriter,account string, blogname string) {
	ret := cooperation.DelCanEditBlog(account,blogname)
	if ret != 0 {
		h.Error(w, fmt.Sprintf("cooperation delblog %s ret=%d",account,ret),h.StatusBadRequest)
		return
	}
	w.Write([]byte(fmt.Sprintf("cooperation delblog \n account=%s \n blog=%s ",account,blogname)))
}

func PageAddCooperationTag(w h.ResponseWriter,account string, tag string) {
	ret := cooperation.AddCanEditTag(account,tag)
	if ret != 0 {
		h.Error(w, fmt.Sprintf("cooperation addtag account=%s tag=%s ret=%d",account,tag,ret),h.StatusBadRequest)
		return
	}
	w.Write([]byte(fmt.Sprintf("cooperation addtag \n account=%s \n blog=%s ",account,tag)))
}

func PageDelCooperationTag(w h.ResponseWriter,account string, tag string) {
	ret := cooperation.DelCanEditTag(account,tag)
	if ret != 0 {
		h.Error(w, fmt.Sprintf("cooperation deltag account=%s tag=%s ret=%d",account,tag,ret),h.StatusBadRequest)
		return
	}
	w.Write([]byte(fmt.Sprintf("cooperation deltag \n account=%s \n blog=%s ",account,tag)))
}

