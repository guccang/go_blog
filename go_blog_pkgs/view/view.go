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
)

func Info(){
	fmt.Println("info view v1.0")
}


type LinkData struct{
	URL string
	DESC string
}

type LinkDatas struct{
	LINKS []LinkData
	VERSION string
	BLOGS_NUMBER int
	TAGS []string
}

type EditorData struct{
	TITLE string
	CONTENT string
	CTIME string
	AUTHTYPE string
	TAGS string
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


func getLinks(blogs []*module.Blog) *LinkDatas{

	datas := LinkDatas{}
	datas.VERSION = config.GetVersion()
	datas.BLOGS_NUMBER = len(blogs)


	all_tags := make(map[string]int)
	all_tags["private"] = 0
	all_tags["public"] = 0

	for _,b := range blogs{
		ld := LinkData {
			URL:fmt.Sprintf("/get?blogname=%s",b.Title),
			DESC:b.Title,
		}
		datas.LINKS = append(datas.LINKS,ld)

		if b.AuthType == module.EAuthType_private {
			all_tags["private"] = all_tags["private"] + 1
		}else{
			all_tags["public"] = all_tags["public"] + 1
		}
		
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

func PageSearch(match string,w h.ResponseWriter ){

	blogs := control.GetMatch(match)

	datas := getLinks(blogs)

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

func PageLink(w h.ResponseWriter){
	
	blogs := control.GetAll()

	datas := getLinks(blogs)
	
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

func PageEditor(w h.ResponseWriter){
	exeDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(exeDir,"markdown_editor.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse markdown_editor",h.StatusInternalServerError)
		return
	}
	
	data := EditorData{
		TITLE:"input title",
		CONTENT:"# input content",
		AUTHTYPE:"private",
		TAGS:"",
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
	
	
	data := EditorData{
		TITLE:blog.Title,
		CONTENT:blog.Content,
		CTIME : blog.CreateTime,
		AUTHTYPE:auth_type_string,
		TAGS : blog.Tags,
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