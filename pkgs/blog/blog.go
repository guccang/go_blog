package blog

import (
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

// blog actor instance
var blog_actor *BlogActor

// Init initializes the blog module and loads blogs via the actor
func Init() {
	log.Debug("module Init")
	blog_actor = &BlogActor{
		Actor: core.NewActor(),
		blogs: make(map[string]*module.Blog),
	}
	blog_actor.Start(blog_actor)
	// load existing blogs
	cmd := &loadBlogsCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}}
	blog_actor.Send(cmd)
	<-cmd.Response()
}

func GetBlogsNum() int {
	cmd := &getBlogsNumCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}}
	blog_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

// 多个goroutine 并发访问，会存在问题
// 但是在当前的场景下使用不会出问题，原因单用户访问操作。不存在并发访问
func GetBlogs() map[string]*module.Blog {
	return blog_actor.blogs
}

func ImportBlogsFromPath(dir string) {
	cmd := &importBlogsCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Dir:          dir,
	}
	blog_actor.Send(cmd)
	<-cmd.Response()
}

func GetBlog(title string) *module.Blog {
	cmd := &getBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Title:        title,
	}
	blog_actor.Send(cmd)
	ret := <-cmd.Response()
	if ret == nil {
		return nil
	}
	return ret.(*module.Blog)
}

func AddBlog(udb *module.UploadedBlogData) int {
	cmd := &addBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		UDB:          udb,
	}
	blog_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func ModifyBlog(udb *module.UploadedBlogData) int {
	cmd := &modifyBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		UDB:          udb,
	}
	blog_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func DeleteBlog(title string) int {
	cmd := &deleteBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Title:        title,
	}
	blog_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func GetRecentlyTimedBlog(title string) *module.Blog {
	cmd := &getRecentlyTimedBlogCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Title:        title,
	}
	blog_actor.Send(cmd)
	ret := <-cmd.Response()
	if ret == nil {
		return nil
	}
	return ret.(*module.Blog)
}

func GetAll(num int, flag int) []*module.Blog {
	cmd := &getAllCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Num:          num,
		Flag:         flag,
	}
	blog_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.([]*module.Blog)
}

func UpdateAccessTime(b *module.Blog) {
	cmd := &updateAccessTimeCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blog:         b,
	}
	blog_actor.Send(cmd)
	<-cmd.Response()
}

func GetBlogAuthType(blogname string) int {
	cmd := &getBlogAuthTypeCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blogname:     blogname,
	}
	blog_actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func IsPublicTag(tag string) int {
	return config.IsPublicTag(tag)
}

func TagReplace(from, to string) {
	cmd := &tagReplaceCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		From:         from,
		To:           to,
	}
	blog_actor.Send(cmd)
	<-cmd.Response()
}

func SetSameAuth(blogname string) {
	cmd := &setSameAuthCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blogname:     blogname,
	}
	blog_actor.Send(cmd)
	<-cmd.Response()
}

func AddAuthType(blogname string, flag int) {
	cmd := &addAuthTypeCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blogname:     blogname,
		Flag:         flag,
	}
	blog_actor.Send(cmd)
	<-cmd.Response()
}

func DelAuthType(blogname string, flag int) {
	cmd := &delAuthTypeCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Blogname:     blogname,
		Flag:         flag,
	}
	blog_actor.Send(cmd)
	<-cmd.Response()
}

func GetURLBlogNames(blogname string) []string {
	cmd := &getURLNamesCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}, Blogname: blogname}
	blog_actor.Send(cmd)
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
func GetYearPlan(year int) (*YearPlanData, error) {
	planTitle := fmt.Sprintf("年计划_%d", year)
	blog := GetBlog(planTitle)
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
func SaveYearPlan(planData *YearPlanData) error {
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
	blog := GetBlog(planTitle)
	udb := module.UploadedBlogData{
		Title:    planTitle,
		Content:  string(content),
		AuthType: module.EAuthType_private,
		Tags:     "年计划",
	}
	var ret int
	if blog == nil {
		ret = AddBlog(&udb)
		log.DebugF("新建年计划博客: %s", planTitle)
	} else {
		ret = ModifyBlog(&udb)
		log.DebugF("更新年计划博客: %s", planTitle)
	}
	if ret != 0 {
		return fmt.Errorf("保存计划失败，错误码: %d", ret)
	}
	savedPlan, err := GetYearPlan(planData.Year)
	if err != nil {
		log.ErrorF("无法验证保存的计划: %v", err)
	} else {
		log.DebugF("验证 - 保存后的任务数据大小: %d", len(savedPlan.Tasks))
	}
	return nil
}
