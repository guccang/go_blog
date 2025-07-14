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
	"time"
)

func Info(){
	fmt.Println("info view v1.0")
}


type LinkData struct{
	URL string
	DESC string
	COOPERATION int
	ACCESS_TIME string
	TAGS []string
}

type LinkDatas struct{
	LINKS []LinkData
	RECENT_LINKS []LinkData
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

type TodolistData struct {
	DATE string
}

// YearPlanData contains data for rendering the year plan template
type YearPlanData struct {
	YEAR         int
	YEAR_OVERVIEW string
	MONTH_PLANS  []string
}

// MonthGoalData contains data for rendering the month goal template
type MonthGoalData struct {
	CURRENT_YEAR  int
	CURRENT_MONTH int
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
			TAGS:[]string{},
		}
		datas.LINKS = append(datas.LINKS,ld)
	}

	for _,t := range sharedtags {
		ld := LinkData {
			URL:t.URL,
			DESC:fmt.Sprintf("Tag-%s",t.Tag),
			COOPERATION:0,
			TAGS:[]string{},
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


		// 处理博客标签
		var blogTags []string
		if b.Tags != "" {
			tags := strings.Split(b.Tags, "|")
			for _, tag := range tags {
				if tag != "" {
					blogTags = append(blogTags, tag)
				}
			}
		}
		
		ld := LinkData {
			URL:fmt.Sprintf("/get?blogname=%s",b.Title),
			DESC:b.Title,
			COOPERATION:(b.AuthType & module.EAuthType_cooperation),
			ACCESS_TIME:b.AccessTime,
			TAGS:blogTags,
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
		datas.TAGS = append(datas.TAGS,tag)
	}
	sort.Strings(datas.TAGS)

	// 处理最近访问的博客
	recent := make([]LinkData, len(datas.LINKS))
	copy(recent, datas.LINKS)
	
	// 根据访问时间排序，最新访问的在前
	sort.Slice(recent, func(i, j int) bool {
		// 如果访问时间为空，则放在最后
		if recent[i].ACCESS_TIME == "" {
			return false
		}
		if recent[j].ACCESS_TIME == "" {
			return true
		}
		
		// 使用time.Parse解析时间字符串为时间对象，然后比较Unix时间戳
		ti, errI := time.Parse("2006-01-02 15:04:05", recent[i].ACCESS_TIME)
		tj, errJ := time.Parse("2006-01-02 15:04:05", recent[j].ACCESS_TIME)
		
		// 如果解析出错，则按原字符串比较
		if errI != nil || errJ != nil {
			return recent[i].ACCESS_TIME > recent[j].ACCESS_TIME
		}
		
		// 使用Unix时间戳比较，更晚的时间排在前面
		if ti.Unix() != tj.Unix() {
			return ti.Unix() > tj.Unix()
		}
		
		// 如果访问时间相同，则按标题字母顺序排序，确保排序稳定性
		return recent[i].DESC < recent[j].DESC
	})
	
	// 最多取6个最近访问的博客
	var MAX_RECENT_LINKS = 9
	if len(recent) > MAX_RECENT_LINKS {
		datas.RECENT_LINKS = recent[:MAX_RECENT_LINKS]
	} else {
		datas.RECENT_LINKS = recent
	}

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
	
	blog_num := config.GetMainBlogNum()
	blogs := control.GetAll(blog_num,flag)
	log.DebugF("blogs cnt=%d",len(blogs))

	datas := getLinks(blogs,flag,session)
	
	exeDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(exeDir,"link.template"))
	if err != nil{
		log.ErrorF(err.Error())
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
	if (blog.AuthType & module.EAuthType_public) != 0 {
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
		log.Debug(err.Error())
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
		log.Debug(err.Error())
		h.Error(w,"Failed to render template get.template",h.StatusInternalServerError)
		return
	}

}


func PageDemo(w h.ResponseWriter,template_name string){
	tempDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(tempDir,template_name))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse demo template",h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w,nil)
	if err != nil{
		log.Debug(err.Error())
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
		log.Debug(err.Error())
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

func PageAddCooperation(w h.ResponseWriter,account string){
	ret,c := cooperation.CreateCooperation(account)
	if ret != 0 {
		h.Error(w, fmt.Sprintf("cooperation %s exit ret=%d c.pwd=%s",account,ret,c.Password),h.StatusBadRequest)
		return
	}
	w.Write([]byte(fmt.Sprintf("create cooperation \n account=%s \n pwd=%s ",c.Account,c.Password)))
	log.DebugF("PageAddCooperation account=%s ret=%d",account,ret)
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


func getsession(r *h.Request) string{
	session,err:= r.Cookie("session")
	if err != nil {
		return ""
	}
	return session.Value
}

func PageSearchNormal(match string,w h.ResponseWriter,r *h.Request) int{
	session := getsession(r)
	is_cooperation := cooperation.IsCooperation(session)

	// 直接显示help
	tokens := strings.Split(match," ")
	if match == "@help" {
		h.Redirect(w,r,"/help",302)
		return 0
	}
    // 直接显示主页
	if match == "@main" {
		h.Redirect(w,r,"/link",302)
		return 0
	}
	// 创建timed blog
	if tokens[0] == "@c" {
		if is_cooperation {
			h.Error(w, "@c auth not support", h.StatusBadRequest)
			return 0
		}
		if len(tokens) != 2 {
			h.Error(w, "@c titlename need", h.StatusBadRequest)
			return 0
		}
		title := tokens[1]
		content := ""
		b := control.GetRecentlyTimedBlog(title)
		if b != nil {
			content = b.Content
		}
		PageEditor(w,title,content)
		return 0
	}
	// 分享private连接
	if tokens[0] == "@share" && len(tokens)>=2 {
		if is_cooperation {
			h.Error(w, "@c auth not support", h.StatusBadRequest)
			return 0
		}
	
		// 创建分享
		if tokens[1] == "c" && len(tokens)>=3 {
			blogname := tokens[2]
			PageShareBlog(w,blogname)
		}
		if tokens[1] == "t" && len(tokens)>=3{
			tag := tokens[2]
			PageShareTag(w,tag)
		}
		// 显示所有创建的分享
		if tokens[1] == "all" {
			if false == is_cooperation {
				PageShowAllShare(w)
			}else{
				w.Write([]byte("not support operation (showAllShare)!!!"))		
			}
		}
		return 0
	}
	// 创建协作账号
	if tokens[0] == "@cooperation" && len(tokens) >= 2{
		log.DebugF("cooperation opt=%s",tokens[1])
		if is_cooperation {
			h.Error(w, "@c auth not support", h.StatusBadRequest)
			return 0 
		}
	
		// 创建
		if tokens[1] == "c" && len(tokens) == 3{
			account := tokens[2]
			PageAddCooperation(w,account)
		}
		// 删除
		if tokens[1] == "d" && len(tokens) == 3{
			account := tokens[2]
			PageDelCooperation(w,account)
		}
		// 显示
		if tokens[1] == "all" && len(tokens) == 2{
			if false == is_cooperation {
				PageShowCooperation(w)
			}else{
				w.Write([]byte("not support operation (showCooperation)!!!"))		
			}
		}
		// add edit blog
		if tokens[1] == "addblog" && len(tokens) == 4{
			account := tokens[2]
			blog := tokens[3]
			PageAddCooperationBlog(w,account,blog)
		}
		if tokens[1] == "delblog" && len(tokens) == 4{
			account := tokens[2]
			blog := tokens[3]
			PageDelCooperationBlog(w,account,blog)
		}
		// add edit tag
		if tokens[1] == "addtag" && len(tokens) == 4{
			account := tokens[2]
			tag := tokens[3]
			PageAddCooperationTag(w,account,tag)
		}
		if tokens[1] == "deltag" && len(tokens) == 4{
			account := tokens[2]
			tag := tokens[3]
			PageDelCooperationTag(w,account,tag)
		}
		return 0
	}

	// 继续其他search
	return 1
}


// timestamp
func PageTimeStamp(w h.ResponseWriter){
	tempDir := config.GetHttpTemplatePath()
	tmpl,err:=t.ParseFiles(filepath.Join(tempDir,"timestamp.template"))
	if err != nil{
		log.Debug(err.Error())
		h.Error(w,"Failed to parse timestamp.template",h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w,nil)
	if err != nil{
		h.Error(w,"Failed to render template timestamp.template",h.StatusInternalServerError)
		return
	}
}

func PageTodolist(w h.ResponseWriter, date string) {
	data := TodolistData{
		DATE: date,
	}

	tmpDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tmpDir, "todolist.template"))
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to parse todolist.template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to render template todolist.template", h.StatusInternalServerError)
		return
	}
}

// PageYearPlan renders the year plan page
func PageYearPlan(w h.ResponseWriter, year int) {
	tmpDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tmpDir, "yearplan.template"))
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to parse yearplan template", h.StatusInternalServerError)
		return
	}
	
	// Initialize data with just the year
	data := YearPlanData{
		YEAR:        year,
		MONTH_PLANS: make([]string, 12), // Initialize with 12 empty strings for months
	}
	
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to render yearplan template", h.StatusInternalServerError)
		return
	}
}

// PageMonthGoal renders the month goal page
func PageMonthGoal(w h.ResponseWriter, year int, month int) {
	tmpDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tmpDir, "monthgoal.template"))
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to parse monthgoal template", h.StatusInternalServerError)
		return
	}
	
	// Initialize data with current year and month
	data := MonthGoalData{
		CURRENT_YEAR:  year,
		CURRENT_MONTH: month,
	}
	
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to render monthgoal template", h.StatusInternalServerError)
		return
	}
}

// PageStatistics renders the statistics page
func PageStatistics(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "statistics.template"))
	if err != nil {
		log.ErrorF("Failed to parse statistics.template: %s", err.Error())
		h.Error(w, "Failed to parse statistics template", h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w, nil)
	if err != nil {
		log.ErrorF("Failed to render statistics.template: %s", err.Error())
		h.Error(w, "Failed to render statistics template", h.StatusInternalServerError)
		return
	}
}

// PageReading renders the reading page
func PageReading(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "reading.template"))
	if err != nil {
		log.ErrorF("Failed to parse reading.template: %s", err.Error())
		h.Error(w, "Failed to parse reading template", h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w, nil)
	if err != nil {
		log.ErrorF("Failed to render reading.template: %s", err.Error())
		h.Error(w, "Failed to render reading template", h.StatusInternalServerError)
		return
	}
}

// PageBookDetail renders the book detail page
func PageBookDetail(w h.ResponseWriter, book *module.Book) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "book_detail.template"))
	if err != nil {
		log.ErrorF("Failed to parse book_detail.template: %s", err.Error())
		h.Error(w, "Failed to parse book detail template", h.StatusInternalServerError)
		return
	}
	
	data := struct {
		Book *module.Book
	}{
		Book: book,
	}
	
	err = tmpl.Execute(w, data)
	if err != nil {
		log.ErrorF("Failed to render book_detail.template: %s", err.Error())
		h.Error(w, "Failed to render book detail template", h.StatusInternalServerError)
		return
	}
}

// PageReadingDashboard renders the reading dashboard page
func PageReadingDashboard(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "reading_dashboard.template"))
	if err != nil {
		log.ErrorF("Failed to parse reading_dashboard.template: %s", err.Error())
		h.Error(w, "Failed to parse reading dashboard template", h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w, nil)
	if err != nil {
		log.ErrorF("Failed to render reading_dashboard.template: %s", err.Error())
		h.Error(w, "Failed to render reading dashboard template", h.StatusInternalServerError)
		return
	}
}

// PagePublic renders the public blogs page
func PagePublic(w h.ResponseWriter) {
	// 获取所有public标签的博客
	blogs := control.GetMatch("@public")
	
	// 只展示public权限的博客
	flag := module.EAuthType_public
	
	// 获取链接数据
	datas := getLinks(blogs, flag, "")
	
	// 渲染模板
	exeDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(exeDir, "public.template"))
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to parse public.template", h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w, datas)
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to render template public.template", h.StatusInternalServerError)
		return
	}
}

// PageExercise renders the exercise page
func PageExercise(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "exercise.template"))
	if err != nil {
		log.ErrorF("Failed to parse exercise.template: %s", err.Error())
		h.Error(w, "Failed to parse exercise template", h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w, nil)
	if err != nil {
		log.ErrorF("Failed to render exercise.template: %s", err.Error())
		h.Error(w, "Failed to render exercise template", h.StatusInternalServerError)
		return
	}
}

// PageLifeCountdown renders the life countdown page
func PageLifeCountdown(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "lifecountdown.template"))
	if err != nil {
		log.ErrorF("Failed to parse lifecountdown.template: %s", err.Error())
		h.Error(w, "Failed to parse lifecountdown template", h.StatusInternalServerError)
		return
	}
	
	err = tmpl.Execute(w, nil)
	if err != nil {
		log.ErrorF("Failed to render lifecountdown.template: %s", err.Error())
		h.Error(w, "Failed to render lifecountdown template", h.StatusInternalServerError)
		return
	}
}

func PageDiaryPasswordInput(w h.ResponseWriter, blogname string) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "diary_password.template"))
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to parse diary_password.template", h.StatusInternalServerError)
		return
	}

	data := struct {
		BLOGNAME string
	}{
		BLOGNAME: blogname,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to render template diary_password.template", h.StatusInternalServerError)
		return
	}
}

func PageDiaryPasswordError(w h.ResponseWriter, blogname string) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "diary_password_error.template"))
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to parse diary_password_error.template", h.StatusInternalServerError)
		return
	}

	data := struct {
		BLOGNAME string
	}{
		BLOGNAME: blogname,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(err.Error())
		h.Error(w, "Failed to render template diary_password_error.template", h.StatusInternalServerError)
		return
	}
}