package blog

import (
	"auth"
	"config"
	"core"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
)

func Info() {
	fmt.Println("info blog v3.0")
}

// Init initializes the blog module and loads blogs via the actor
func Init() {
	log.Debug("blog module Init")

	// Initialize blog manager
	managerCmd := &InitManagerCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}}

	// Use a temporary actor to initialize the manager
	tempActor := &BlogActor{
		Actor:   core.NewActor(),
		Account: getDefaultAccount(),
		blogs:   make(map[string]*module.Blog),
	}
	tempActor.Start(tempActor)
	tempActor.Send(managerCmd)
	<-managerCmd.Response()
	tempActor.Stop()
}

// getDefaultAccount returns the default admin account
func getDefaultAccount() string {
	return config.GetConfig("admin")
}

// getBlogActor returns the blog actor for the given account
func getBlogActor(account string) *BlogActor {
	// If account is empty, use default account
	if account == "" {
		account = getDefaultAccount()
	}

	// We need to use the blog manager's default actor to handle this
	// For now, we'll use a simple approach - create the actor directly if needed
	// This is a temporary solution until we properly integrate the manager

	if blogManager == nil {
		// Initialize manager if not already done
		blogManager = &BlogManager{
			actors: make(map[string]*BlogActor),
			defaultAct: &BlogActor{
				Actor:   core.NewActor(),
				Account: getDefaultAccount(),
				blogs:   make(map[string]*module.Blog),
			},
		}
		blogManager.defaultAct.Start(blogManager.defaultAct)

		// Load system blogs
		loadCmd := &loadBlogsCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}}
		blogManager.defaultAct.Send(loadCmd)
		<-loadCmd.Response()
	}

	if account == getDefaultAccount() {
		return blogManager.defaultAct
	}

	blogManager.mu.RLock()
	if act, exists := blogManager.actors[account]; exists {
		blogManager.mu.RUnlock()
		return act
	}
	blogManager.mu.RUnlock()

	// Create new actor for this account
	blogManager.mu.Lock()
	defer blogManager.mu.Unlock()

	newActor := &BlogActor{
		Actor:   core.NewActor(),
		Account: account,
		blogs:   make(map[string]*module.Blog),
	}
	newActor.Start(newActor)

	// Load account-specific blogs
	loadCmd := &loadAccountBlogsCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Account:      account,
	}
	newActor.Send(loadCmd)
	<-loadCmd.Response()

	blogManager.actors[account] = newActor
	return newActor
}

func GetBlogsNumWithAccount(account string) int {
	actor := getBlogActor(account)
	cmd := &getBlogsNumCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

// 多个goroutine 并发访问，会存在问题
// 但是在当前的场景下使用不会出问题，原因单用户访问操作。不存在并发访问
func GetBlogsWithAccount(account string) map[string]*module.Blog {
	actor := getBlogActor(account)
	return actor.blogs
}

func ImportBlogsFromPathWithAccount(account, dir string) {
	actor := getBlogActor(account)
	cmd := &importBlogsCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Dir:          dir,
	}
	actor.Send(cmd)
	<-cmd.Response()
}

func GetBlogWithAccount(account, title string) *module.Blog {
	actor := getBlogActor(account)
	cmd := &getBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Title:        title,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	if ret == nil {
		return nil
	}
	return ret.(*module.Blog)
}

func AddBlogWithAccount(account string, udb *module.UploadedBlogData) int {
	actor := getBlogActor(account)
	cmd := &addBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		UDB:          udb,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func ModifyBlogWithAccount(account string, udb *module.UploadedBlogData) int {
	actor := getBlogActor(account)
	cmd := &modifyBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		UDB:          udb,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func DeleteBlogWithAccount(account, title string) int {
	actor := getBlogActor(account)
	cmd := &deleteBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Title:        title,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func GetRecentlyTimedBlogWithAccount(account, title string) *module.Blog {
	actor := getBlogActor(account)
	cmd := &getRecentlyTimedBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Title:        title,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	if ret == nil {
		return nil
	}
	return ret.(*module.Blog)
}

func GetAllWithAccount(account string, num int, flag int) []*module.Blog {
	actor := getBlogActor(account)
	cmd := &getAllCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Num:          num,
		Flag:         flag,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.([]*module.Blog)
}

func UpdateAccessTimeWithAccount(account string, b *module.Blog) {
	actor := getBlogActor(account)
	cmd := &updateAccessTimeCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blog:         b,
	}
	actor.Send(cmd)
	<-cmd.Response()
}

func GetBlogAuthTypeWithAccount(account, blogname string) int {
	actor := getBlogActor(account)
	cmd := &getBlogAuthTypeCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blogname:     blogname,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func IsPublicTag(tag string) int {
	return config.IsPublicTag(tag)
}

func TagReplaceWithAccount(account, from, to string) {
	actor := getBlogActor(account)
	cmd := &tagReplaceCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		From:         from,
		To:           to,
	}
	actor.Send(cmd)
	<-cmd.Response()
}

func SetSameAuthWithAccount(account, blogname string) {
	actor := getBlogActor(account)
	cmd := &setSameAuthCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blogname:     blogname,
	}
	actor.Send(cmd)
	<-cmd.Response()
}

func AddAuthTypeWithAccount(account, blogname string, flag int) {
	actor := getBlogActor(account)
	cmd := &addAuthTypeCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blogname:     blogname,
		Flag:         flag,
	}
	actor.Send(cmd)
	<-cmd.Response()
}

func DelAuthTypeWithAccount(account, blogname string, flag int) {
	actor := getBlogActor(account)
	cmd := &delAuthTypeCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blogname:     blogname,
		Flag:         flag,
	}
	actor.Send(cmd)
	<-cmd.Response()
}

func GetURLBlogNamesWithAccount(account, blogname string) []string {
	actor := getBlogActor(account)
	cmd := &getURLNamesCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}, Blogname: blogname}
	actor.Send(cmd)
	ret := <-cmd.Response()
	if ret == nil {
		return []string{}
	}
	return ret.([]string)
}

// ===== Year plan API remains here, using the facade functions above =====

// 年度计划相关数据结构
type YearPlanData struct {
	YearOverview string                 `json:"yearOverview"`
	MonthPlans   []string               `json:"monthPlans"`
	Year         int                    `json:"year"`
	Tasks        map[string]interface{} `json:"tasks"` // 存储每月任务列表
}

// 获取年度计划
func GetYearPlanWithAccount(account string, year int) (*YearPlanData, error) {
	planTitle := fmt.Sprintf("年计划_%d", year)
	blog := GetBlogWithAccount(account, planTitle)
	if blog == nil {
		return nil, fmt.Errorf("未找到年份 %d 的计划", year)
	}
	var planData YearPlanData
	if err := json.Unmarshal([]byte(blog.Content), &planData); err != nil {
		return nil, fmt.Errorf("解析计划数据失败: %v", err)
	}
	log.DebugF("获取年计划 - 年份: %d, 任务数据大小: %d", year, len(planData.Tasks))
	if planData.Tasks == nil {
		planData.Tasks = make(map[string]interface{})
		log.DebugF("初始化空任务映射")
	}
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(blog.Content), &rawData); err == nil {
		if tasks, ok := rawData["tasks"].(map[string]interface{}); ok && len(tasks) > 0 {
			if len(planData.Tasks) == 0 {
				planData.Tasks = tasks
				log.DebugF("从原始JSON中恢复任务数据, 大小: %d", len(tasks))
			}
		}
	}
	return &planData, nil
}

// 保存年度计划
func SaveYearPlanWithAccount(account string, planData *YearPlanData) error {
	if planData.Year < 2020 || planData.Year > 2100 {
		return fmt.Errorf("无效的年份: %d", planData.Year)
	}
	if len(planData.MonthPlans) != 12 {
		return fmt.Errorf("月度计划数量不正确，应为12个月")
	}
	log.DebugF("保存计划 - 年份: %d, 任务数据大小: %d", planData.Year, len(planData.Tasks))
	for month, tasks := range planData.Tasks {
		if tasksArray, ok := tasks.([]interface{}); ok {
			log.DebugF("月份 %s 的任务数量: %d", month, len(tasksArray))
		}
	}
	planTitle := fmt.Sprintf("年计划_%d", planData.Year)
	content, err := json.Marshal(planData)
	if err != nil {
		return fmt.Errorf("序列化计划数据失败: %v", err)
	}
	blog := GetBlogWithAccount(account, planTitle)
	udb := module.UploadedBlogData{
		Title:    planTitle,
		Content:  string(content),
		AuthType: module.EAuthType_private,
		Tags:     "年计划",
		Account:  account,
	}
	var ret int
	if blog == nil {
		ret = AddBlogWithAccount(account, &udb)
		log.DebugF("新建年计划博客: %s", planTitle)
	} else {
		ret = ModifyBlogWithAccount(account, &udb)
		log.DebugF("更新年计划博客: %s", planTitle)
	}
	if ret != 0 {
		return fmt.Errorf("保存计划失败，错误码: %d", ret)
	}
	savedPlan, err := GetYearPlanWithAccount(account, planData.Year)
	if err != nil {
		log.ErrorF("无法验证保存的计划: %v", err)
	} else {
		log.DebugF("验证 - 保存后的任务数据大小: %d", len(savedPlan.Tasks))
	}
	return nil
}

// ===== Backward compatibility for system modules =====

// GetDefaultAccount returns the default admin account for system modules
func GetDefaultAccount() string {
	return getDefaultAccount()
}

// GetAccountFromSession returns the account from session if available, otherwise default account
func GetAccountFromSession(session string) string {
	if session == "" {
		return getDefaultAccount()
	}

	account := auth.GetAccountBySession(session)
	if account == "" {
		return getDefaultAccount()
	}
	return account
}

// Backward compatibility functions for system modules
// These functions use the default account internally

func GetBlogs() map[string]*module.Blog {
	return GetBlogsWithAccount("")
}

func GetBlog(title string) *module.Blog {
	return GetBlogWithAccount("", title)
}

func AddBlog(udb *module.UploadedBlogData) int {
	return AddBlogWithAccount("", udb)
}

func ModifyBlog(udb *module.UploadedBlogData) int {
	return ModifyBlogWithAccount("", udb)
}

func DeleteBlog(title string) int {
	return DeleteBlogWithAccount("", title)
}

func GetRecentlyTimedBlog(title string) *module.Blog {
	return GetRecentlyTimedBlogWithAccount("", title)
}

func GetAll(num int, flag int) []*module.Blog {
	return GetAllWithAccount("", num, flag)
}

func GetBlogAuthType(blogname string) int {
	return GetBlogAuthTypeWithAccount("", blogname)
}

func TagReplace(from, to string) {
	TagReplaceWithAccount("", from, to)
}

func SetSameAuth(blogname string) {
	SetSameAuthWithAccount("", blogname)
}

func AddAuthType(blogname string, flag int) {
	AddAuthTypeWithAccount("", blogname, flag)
}

func DelAuthType(blogname string, flag int) {
	DelAuthTypeWithAccount("", blogname, flag)
}

func GetURLBlogNames(blogname string) []string {
	return GetURLBlogNamesWithAccount("", blogname)
}

func GetBlogsNum() int {
	return GetBlogsNumWithAccount("")
}

func ImportBlogsFromPath(dir string) {
	ImportBlogsFromPathWithAccount("", dir)
}

func UpdateAccessTime(b *module.Blog) {
	UpdateAccessTimeWithAccount("", b)
}

// Year plan backward compatibility
func GetYearPlan(year int) (*YearPlanData, error) {
	return GetYearPlanWithAccount("", year)
}

func SaveYearPlan(planData *YearPlanData) error {
	return SaveYearPlanWithAccount("", planData)
}
