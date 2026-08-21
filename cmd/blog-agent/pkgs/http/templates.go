package http

import (
	"auth"
	"config"
	"fmt"
	t "html/template"
	"module"
	log "mylog"
	h "net/http"
	"net/url"
	"path/filepath"
	"search"
	control "service"
	"share"
	"sort"
	"strings"
	"time"
)

// viewGroups template helpers with the former view package's call shape while
// keeping rendering in the HTTP package.
var view = struct {
	PageBookDetail         func(h.ResponseWriter, *module.Book)
	PageDiaryPasswordError func(h.ResponseWriter, string)
	PageDiaryPasswordInput func(h.ResponseWriter, string)
	PageEditor             func(h.ResponseWriter, string, string)
	PageExercise           func(h.ResponseWriter)
	PageExercisePro        func(h.ResponseWriter)
	PageGetBlog            func(string, h.ResponseWriter, int, string)
	PageGoal               func(h.ResponseWriter)
	PageGoalManage         func(h.ResponseWriter)
	PageIndex              func(h.ResponseWriter)
	PageLink               func(h.ResponseWriter, int, string)
	PageMigration          func(h.ResponseWriter)
	PagePublic             func(h.ResponseWriter, string)
	PageReading            func(h.ResponseWriter)
	PageReadingManage      func(h.ResponseWriter)
	PageReadingDashboard   func(h.ResponseWriter)
	PageSearch             func(string, h.ResponseWriter, string)
	PageSearchNormal       func(string, h.ResponseWriter, *h.Request) int
	PageTags               func(h.ResponseWriter, string, string)
	PageTools              func(h.ResponseWriter)
	PageVisualThemes       func(h.ResponseWriter)
	PageNaturalMotionLab   func(h.ResponseWriter)
}{
	PageBookDetail:         PageBookDetail,
	PageDiaryPasswordError: PageDiaryPasswordError,
	PageDiaryPasswordInput: PageDiaryPasswordInput,
	PageEditor:             PageEditor,
	PageExercise:           PageExercise,
	PageExercisePro:        PageExercisePro,
	PageGetBlog:            PageGetBlog,
	PageGoal:               PageGoal,
	PageGoalManage:         PageGoalManage,
	PageIndex:              PageIndex,
	PageLink:               PageLink,
	PageMigration:          PageMigration,
	PagePublic:             PagePublic,
	PageReading:            PageReading,
	PageReadingManage:      PageReadingManage,
	PageReadingDashboard:   PageReadingDashboard,
	PageSearch:             PageSearch,
	PageSearchNormal:       PageSearchNormal,
	PageTags:               PageTags,
	PageTools:              PageTools,
	PageVisualThemes:       PageVisualThemes,
	PageNaturalMotionLab:   PageNaturalMotionLab,
}

func templateInfo() {
	log.InfoF(log.ModuleView, "info view v1.0")
}

// helper: get template full path
func GetTemplatePath(name string) string {
	return filepath.Join(config.GetHttpTemplatePath(), name)
}

// helper: render a template with data
func RenderTemplate(w h.ResponseWriter, fullpath string, data interface{}) error {
	tmpl, err := t.ParseFiles(fullpath)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

// generateUserAvatar generates a simple avatar string for the user
func generateUserAvatar(account string) string {
	if account == "" {
		return "👤"
	}
	// Use the first character of the account name as avatar
	runes := []rune(strings.ToUpper(account))
	if len(runes) > 0 {
		return string(runes[0])
	}
	return "👤"
}

type LinkData struct {
	URL          string
	DESC         string
	ACCESS_TIME  string
	PREVIEW      string
	IMAGE_URL    string
	TAGS         []string
	IS_ENCRYPTED bool
	IS_DIARY     bool
	IS_TECH_DOC  bool
}

type LinkDatas struct {
	LINKS           []LinkData
	RECENT_LINKS    []LinkData
	VERSION         string
	BLOGS_NUMBER    int
	USER_ACCOUNT    string
	USER_AVATAR     string
	SEARCH_COMMANDS []SearchCommandInfo
	SHOW_LOAD_MORE  bool
}

// SearchCommandInfo 搜索命令信息
type SearchCommandInfo struct {
	Name        string // 命令名称，如 "@tag match"
	DisplayName string // 显示名称，如 "标签搜索"
	Description string // 命令描述
	Example     string // 使用示例
	HasParam    bool   // 是否需要额外参数
	ParamHint   string // 参数提示，如 "标签名"
}

type CommentDatas struct {
	IDX   int
	OWNER string
	MSG   string
	CTIME string
	MAIL  string
}

type EditorData struct {
	TITLE        string
	CONTENT      string
	CTIME        string
	AUTHTYPE     string
	TAGS         string
	COMMENTS     []CommentDatas
	ENCRYPT      string
	ACCOUNT      string
	IS_LARGE     bool
	CONTENT_SIZE int
	// 权限状态字段
	IS_PRIVATE   bool
	IS_PUBLIC    bool
	IS_DIARY     bool
	IS_ENCRYPTED bool
	PI_ENABLED   bool
}

type TodolistData struct {
	DATE string
}

func Notify(msg string, w h.ResponseWriter) {
	tmpDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tmpDir, "notify.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse markdown_editor", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, msg)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template markdown_editor", h.StatusInternalServerError)
		return
	}
	fmt.Println("view Notify", msg)
}

func getShareLinks() *LinkDatas {
	datas := LinkDatas{}

	sharedblogs := share.GetSharedBlogs()
	sharedtags := share.GetSharedTags()

	total_shared_data := len(sharedblogs) + len(sharedtags)
	datas.VERSION = fmt.Sprintf("%s|%d", config.GetVersionWithAccount(config.GetAdminAccount()), total_shared_data)
	datas.BLOGS_NUMBER = total_shared_data

	for _, b := range sharedblogs {
		ld := LinkData{
			URL:          b.URL,
			DESC:         b.Title,
			TAGS:         []string{},
			IS_ENCRYPTED: false,
			IS_DIARY:     false,
		}
		datas.LINKS = append(datas.LINKS, ld)
	}

	for _, t := range sharedtags {
		ld := LinkData{
			URL:          t.URL,
			DESC:         fmt.Sprintf("Tag-%s", t.Tag),
			TAGS:         []string{},
			IS_ENCRYPTED: false,
			IS_DIARY:     false,
		}
		datas.LINKS = append(datas.LINKS, ld)
	}

	return &datas
}

func getLinks(blogs []*module.Blog, flag int, account string) *LinkDatas {

	datas := LinkDatas{}
	datas.VERSION = fmt.Sprintf("%s|%d", config.GetVersionWithAccount(account), control.GetBlogsNum(account))
	datas.BLOGS_NUMBER = len(blogs)
	datas.USER_ACCOUNT = account
	datas.USER_AVATAR = generateUserAvatar(account)

	for _, b := range blogs {

		// not show encrypt blog
		if (b.AuthType & flag) == 0 {
			continue
		}

		// Include account parameter in URL for public blogs to ensure correct blog retrieval
		linkURL := fmt.Sprintf("/get?blogname=%s", url.QueryEscape(b.Title))
		if (flag&module.EAuthType_public) != 0 && account != "" {
			linkURL = fmt.Sprintf("/get?blogname=%s&account=%s", url.QueryEscape(b.Title), url.QueryEscape(account))
		}

		isEncrypted := b.Encrypt == 1 || (b.AuthType&module.EAuthType_encrypt) != 0
		isDiary := (b.AuthType & module.EAuthType_diary) != 0
		preview, imageURL := "", ""
		switch {
		case isEncrypted:
			preview = "加密内容，打开后验证访问权限"
		case isDiary:
			preview = "日记内容，打开后验证访问权限"
		default:
			preview, imageURL = buildMainBlogPreview(b.Title, b.Content)
		}
		ld := LinkData{
			URL:          linkURL,
			DESC:         b.Title,
			ACCESS_TIME:  recentTimeLabel(b.AccessTime, b.ModifyTime, time.Now()),
			PREVIEW:      preview,
			IMAGE_URL:    imageURL,
			IS_ENCRYPTED: isEncrypted,
			IS_DIARY:     isDiary,
			IS_TECH_DOC:  strings.Contains(b.Tags, "blog实现技术文档"),
		}
		datas.LINKS = append(datas.LINKS, ld)

	}

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

	// 获取搜索命令数据
	searchCommands := search.GetSearchCommands()
	for _, cmd := range searchCommands {
		datas.SEARCH_COMMANDS = append(datas.SEARCH_COMMANDS, SearchCommandInfo{
			Name:        cmd.Name,
			DisplayName: cmd.DisplayName,
			Description: cmd.Description,
			Example:     cmd.Example,
			HasParam:    cmd.HasParam,
			ParamHint:   cmd.ParamHint,
		})
	}

	return &datas
}

// parseAuthTypeToEditorData 解析权限类型到EditorData结构体
func parseAuthTypeToEditorData(authType int, encrypt int) (string, bool, bool, bool, bool) {
	authTypeString := "private"
	isPrivate := (authType & module.EAuthType_private) != 0
	isPublic := (authType & module.EAuthType_public) != 0
	isDiary := (authType & module.EAuthType_diary) != 0
	isEncrypted := encrypt == 1 || (authType&module.EAuthType_encrypt) != 0

	// 设置主要权限字符串（用于向后兼容）
	if isPublic {
		authTypeString = "public"
	} else if isDiary {
		authTypeString = "diary"
	} else {
		authTypeString = "private"
	}

	log.DebugF(log.ModuleView, "Parsed auth type %d: private=%v, public=%v, diary=%v, encrypted=%v",
		authType, isPrivate, isPublic, isDiary, isEncrypted)

	return authTypeString, isPrivate, isPublic, isDiary, isEncrypted
}

func PageSearch(match string, w h.ResponseWriter, account string) {
	blogs := control.GetMatch(account, match)
	flag := module.EAuthType_all
	datas := getLinks(blogs, flag, account)

	// 为搜索结果中的所有链接添加highlight参数
	for i := range datas.LINKS {
		if strings.Contains(datas.LINKS[i].URL, "/get?blogname=") {
			datas.LINKS[i].URL = fmt.Sprintf("%s&highlight=%s", datas.LINKS[i].URL, match)
		}
	}
	for i := range datas.RECENT_LINKS {
		if strings.Contains(datas.RECENT_LINKS[i].URL, "/get?blogname=") {
			datas.RECENT_LINKS[i].URL = fmt.Sprintf("%s&highlight=%s", datas.RECENT_LINKS[i].URL, match)
		}
	}

	exeDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(exeDir, "link.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse link.template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, datas)
	if err != nil {
		h.Error(w, "Failed to render template link.template", h.StatusInternalServerError)
		return
	}
}

func PageTags(w h.ResponseWriter, tag, account string) {
	blogs := control.GetMatch(account, "@tag match"+tag)

	flag := module.EAuthType_public
	// 只展示public

	datas := getLinks(blogs, flag, account)

	exeDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(exeDir, "tags.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse tags.template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, datas)
	if err != nil {
		h.Error(w, "Failed to render template tags.template", h.StatusInternalServerError)
		return
	}

}

func PageLink(w h.ResponseWriter, flag int, account string) {

	blog_num := config.GetMainBlogNum()
	if blog_num <= 0 || blog_num > 20 {
		blog_num = 20
	}
	blogs := control.ListBlogSummaries(account, blog_num, 0, flag)
	log.DebugF(log.ModuleView, "blogs cnt=%d", len(blogs))

	datas := getLinks(blogs, flag, account)
	datas.BLOGS_NUMBER = control.GetBlogsNum(account)
	datas.SHOW_LOAD_MORE = len(blogs) < datas.BLOGS_NUMBER

	exeDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(exeDir, "link.template"))
	if err != nil {
		log.ErrorF(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse link.template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, datas)
	if err != nil {
		log.ErrorF(log.ModuleView, "Failed to render template link.tempate err=%s", err.Error())
		h.Error(w, "Failed to render template link.template %s", h.StatusInternalServerError)
		return
	}
}

func PageEditor(w h.ResponseWriter, init_title string, init_content string) {
	exeDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(exeDir, "markdown_editor.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse markdown_editor", h.StatusInternalServerError)
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

	// 为新博客设置默认权限
	authTypeString, isPrivate, isPublic, isDiary, isEncrypted := parseAuthTypeToEditorData(module.EAuthType_private, 0)

	data := EditorData{
		TITLE:        title,
		CONTENT:      content,
		AUTHTYPE:     authTypeString,
		TAGS:         "",
		ENCRYPT:      "",
		IS_PRIVATE:   isPrivate,
		IS_PUBLIC:    isPublic,
		IS_DIARY:     isDiary,
		IS_ENCRYPTED: isEncrypted,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template markdown_editor", h.StatusInternalServerError)
		return
	}
}

// PageImagePasteDemo renders an isolated interaction prototype, without storage side effects.
func PageImagePasteDemo(w h.ResponseWriter) {
	exeDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(exeDir, "image_paste_demo.template"))
	if err != nil {
		h.Error(w, "Failed to parse image_paste_demo.template", h.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, nil); err != nil {
		h.Error(w, "Failed to render image_paste_demo.template", h.StatusInternalServerError)
	}
}

func PageGetBlog(blogname string, w h.ResponseWriter, usepublic int, account string) {
	blogObj := control.GetBlog(account, blogname)
	if blogObj == nil {
		h.Error(w, fmt.Sprintf("blogname=%s not find", blogname), h.StatusBadRequest)
		return
	}

	// modify accesstime
	control.UpdateAccessTime(account, blogObj)

	template_name := "get.template"
	if usepublic != 0 {
		template_name = "get_public.template"
	}

	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, template_name))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse get.template", h.StatusInternalServerError)
		return
	}

	encrypt_str := ""
	if blogObj.Encrypt == 1 {
		encrypt_str = "aes"
	}

	// 解析博客权限状态
	authTypeString, isPrivate, isPublic, isDiary, isEncrypted := parseAuthTypeToEditorData(blogObj.AuthType, blogObj.Encrypt)

	const largeBlogThreshold = 256 * 1024
	isLarge := len(blogObj.Content) > largeBlogThreshold && blogObj.Encrypt == 0 && (blogObj.AuthType&module.EAuthType_diary) == 0
	content := blogObj.Content
	if isLarge {
		content = ""
	}
	data := EditorData{
		TITLE:        blogObj.Title,
		CONTENT:      content,
		CTIME:        blogObj.CreateTime,
		AUTHTYPE:     authTypeString,
		TAGS:         blogObj.Tags,
		ENCRYPT:      encrypt_str,
		ACCOUNT:      account,
		IS_LARGE:     isLarge,
		CONTENT_SIZE: len(blogObj.Content),
		IS_PRIVATE:   isPrivate,
		IS_PUBLIC:    isPublic,
		IS_DIARY:     isDiary,
		IS_ENCRYPTED: isEncrypted,
		PI_ENABLED:   !isDiary && !isEncrypted,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template get.template", h.StatusInternalServerError)
		return
	}

}

func PageIndex(w h.ResponseWriter) {

	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "login.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse get.template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template get.template", h.StatusInternalServerError)
		return
	}

}

// 将blogname设置为分享
func PageShareBlog(w h.ResponseWriter, account, blogname string) {
	blog := control.GetBlog(account, blogname)
	if blog == nil {
		h.Error(w, fmt.Sprintf("blogname=%s not find", blogname), h.StatusBadRequest)
		return
	}
	url, pwd := share.AddSharedBlog(blogname)
	w.Write([]byte(fmt.Sprintf("PageShareBlog \n url=%s \n pwd=%s ", url, pwd)))
}

// 将tag设置为分享
func PageShareTag(w h.ResponseWriter, tag string) {
	url, pwd := share.AddSharedTag(tag)
	w.Write([]byte(fmt.Sprintf("PageShareTag\n url=%s \n pwd=%s", url, pwd)))
}

// 返回所有分享
func PageShowAllShare(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "share.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse sharetemplate", h.StatusInternalServerError)
		return
	}

	shareddatas := getShareLinks()

	err = tmpl.Execute(w, shareddatas)
	if err != nil {
		h.Error(w, "Failed to render template share.template", h.StatusInternalServerError)
		return
	}
}

func templateSession(r *h.Request) string {
	session, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return session.Value
}

func PageSearchNormal(match string, w h.ResponseWriter, r *h.Request) int {
	account := auth.GetAccountFromRequest(r)

	// 直接显示help
	tokens := strings.Split(match, " ")
	if match == "@help" {
		h.Redirect(w, r, "/help", 302)
		return 0
	}
	// 直接显示主页
	if match == "@main" {
		h.Redirect(w, r, "/main", 302)
		return 0
	}
	// 创建timed blog
	if tokens[0] == "@c" {
		if len(tokens) != 2 {
			h.Error(w, "@c titlename need", h.StatusBadRequest)
			return 0
		}
		title := tokens[1]
		content := ""
		account := auth.GetAccountFromRequest(r)
		b := control.GetRecentlyTimedBlog(account, title)
		if b != nil {
			content = b.Content
		}
		PageEditor(w, title, content)
		return 0
	}
	// 分享private连接
	if tokens[0] == "@share" && len(tokens) >= 2 {

		// 创建分享
		if tokens[1] == "c" && len(tokens) >= 3 {
			blogname := tokens[2]
			PageShareBlog(w, account, blogname)
		}
		if tokens[1] == "t" && len(tokens) >= 3 {
			tag := tokens[2]
			PageShareTag(w, tag)
		}
		// 显示所有创建的分享
		if tokens[1] == "all" {
			PageShowAllShare(w)
		}
		return 0
	}

	// 继续其他search
	return 1
}

func PageTodolist(w h.ResponseWriter, date string) {
	data := TodolistData{
		DATE: date,
	}

	tmpDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tmpDir, "todolist.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse todolist.template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template todolist.template", h.StatusInternalServerError)
		return
	}
}

// PageSkill renders the skill learning page
func PageSkill(w h.ResponseWriter) {
	tmpDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tmpDir, "skill.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse skill template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template skill.template", h.StatusInternalServerError)
		return
	}
}

// PageGoal renders the unified goal management page
func PageGoal(w h.ResponseWriter) {
	tmpDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tmpDir, "goal_map.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse goal template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render goal template", h.StatusInternalServerError)
		return
	}
}

func PageGoalManage(w h.ResponseWriter) {
	tmpDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tmpDir, "goal.template"))
	if err != nil {
		h.Error(w, "Failed to parse goal manage template", h.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, nil); err != nil {
		h.Error(w, "Failed to render goal manage template", h.StatusInternalServerError)
	}
}

func PageClipboard(w h.ResponseWriter) {
	if err := RenderTemplate(w, GetTemplatePath("clipboard.template"), nil); err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render clipboard template", h.StatusInternalServerError)
	}
}

func PageProducts(w h.ResponseWriter, data ProductsPageData) {
	if err := RenderTemplate(w, GetTemplatePath("products.template"), data); err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render products template", h.StatusInternalServerError)
	}
}

// PageStatistics renders the statistics page
func PageStatistics(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "statistics.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse statistics template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render statistics template", h.StatusInternalServerError)
		return
	}
}

// PageReading renders the reading page
func PageReading(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "reading_shelf.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse reading template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render reading template", h.StatusInternalServerError)
		return
	}
}

func PageReadingManage(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "reading.template"))
	if err != nil {
		h.Error(w, "Failed to parse reading manage template", h.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, nil); err != nil {
		h.Error(w, "Failed to render reading manage template", h.StatusInternalServerError)
	}
}

// PageBookDetail renders the book detail page
func PageBookDetail(w h.ResponseWriter, book *module.Book) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "book_detail_focus.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
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
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render book detail template", h.StatusInternalServerError)
		return
	}
}

// PageReadingDashboard renders the reading dashboard page
func PageReadingDashboard(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "reading_dashboard.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse reading dashboard template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render reading dashboard template", h.StatusInternalServerError)
		return
	}
}

// PagePublic renders the public blogs page
func PagePublic(w h.ResponseWriter, account string) {
	// 公开权限直接读取 SQLite auth_type，不再依赖系统博客或内存缓存。
	flag := module.EAuthType_public
	blogs := control.ListBlogSummaries(account, control.GetBlogsNum(account), 0, flag)

	// 获取链接数据
	datas := getLinks(blogs, flag, account)

	// 渲染模板
	exeDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(exeDir, "public.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse public.template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, datas)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template public.template", h.StatusInternalServerError)
		return
	}
}

// PageExercise renders the exercise page
func PageExercise(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "exercise_dashboard.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse exercise template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render exercise template", h.StatusInternalServerError)
		return
	}
}

// PageExercisePro renders the professional bodyweight training page.
func PageExercisePro(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "exercise_professional.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse professional exercise template", h.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, nil); err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render professional exercise template", h.StatusInternalServerError)
	}
}

func PageDiaryPasswordInput(w h.ResponseWriter, blogname string) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "diary_password.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
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
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template diary_password.template", h.StatusInternalServerError)
		return
	}
}

func PageDiaryPasswordError(w h.ResponseWriter, blogname string) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "diary_password_error.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
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
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render template diary_password_error.template", h.StatusInternalServerError)
		return
	}
}

// 智能助手页面
func PageAssistant(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "assistant.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse assistant template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render assistant template", h.StatusInternalServerError)
		return
	}
}

// PageMCP renders the MCP page
func PageMCP(w h.ResponseWriter, data interface{}) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "mcp.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse MCP template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render MCP template", h.StatusInternalServerError)
		return
	}
}

// PageConstellation renders the constellation divination page
func PageConstellation(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "constellation.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse constellation template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render constellation template", h.StatusInternalServerError)
		return
	}
}

// PageTools renders the tools page
func PageTools(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "tools.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse tools template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render tools template", h.StatusInternalServerError)
		return
	}
}

// PageVisualThemes renders the visual theme atlas.
func PageVisualThemes(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "visual_themes.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse visual themes template", h.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render visual themes template", h.StatusInternalServerError)
	}
}

// PageNaturalMotionLab renders the isolated 001 Celadon Rain quality sample.
func PageNaturalMotionLab(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "natural_motion_lab.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse natural motion lab template", h.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render natural motion lab template", h.StatusInternalServerError)
	}
}

// PageMigration renders the migration page
func PageMigration(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "migration.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse migration template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render migration template", h.StatusInternalServerError)
		return
	}
}

// PageFinance renders the family asset calculation page
func PageFinance(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "finance.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse finance template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render finance template", h.StatusInternalServerError)
		return
	}
}

// PageTaskBreakdown renders the task breakdown page
func PageTaskBreakdown(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "taskbreakdown.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse taskbreakdown template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render taskbreakdown template", h.StatusInternalServerError)
		return
	}
}

// PageCodeGen renders the codegen page
func PageCodeGen(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "codegen.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse codegen template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render codegen template", h.StatusInternalServerError)
		return
	}
}

// PageEnglishLearning renders the English learning tracker page
func PageEnglishLearning(w h.ResponseWriter) {
	tempDir := config.GetHttpTemplatePath()
	tmpl, err := t.ParseFiles(filepath.Join(tempDir, "english-learning-tracker.template"))
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to parse english-learning-tracker template", h.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Debug(log.ModuleView, err.Error())
		h.Error(w, "Failed to render english-learning-tracker template", h.StatusInternalServerError)
		return
	}
}
