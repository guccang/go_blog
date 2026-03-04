package blog

import (
	"auth"
	"config"
	"encoding/json"
	"fmt"
	"ioutils"
	"module"
	log "mylog"
	"path/filepath"
	db "persistence"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== Simple Blog 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

// BlogStore 博客存储
type BlogStore struct {
	blogs map[string]*module.Blog
	mu    sync.RWMutex
}

// BlogManager 多账户管理
type BlogManager struct {
	stores map[string]*BlogStore
	mu     sync.Mutex
}

var blogManager *BlogManager

func Info() {
	log.InfoF(log.ModuleBlog, "info blog v4.0 (simple)")
}

// Init 初始化 Blog 模块
func Init() {
	log.Debug(log.ModuleBlog, "blog module Init (simple)")
	blogManager = &BlogManager{
		stores: make(map[string]*BlogStore),
	}
}

// getBlogStore 获取或创建指定账户的博客存储
func getBlogStore(account string) *BlogStore {
	blogManager.mu.Lock()
	defer blogManager.mu.Unlock()

	if store, ok := blogManager.stores[account]; ok {
		return store
	}

	// 创建新存储
	store := &BlogStore{
		blogs: make(map[string]*module.Blog),
	}

	// 从数据库加载
	blogs := db.GetBlogsByAccount(account)
	if blogs != nil {
		for _, b := range blogs {
			if b.Encrypt == 1 {
				b.AuthType = module.EAuthType_encrypt
			}
			store.blogs[b.Title] = b
		}
	}
	log.DebugF(log.ModuleBlog, "BlogStore loaded account=%s, count=%d", account, len(store.blogs))

	blogManager.stores[account] = store
	return store
}

func strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ========== 对外接口 ==========

// GetBlogsNumWithAccount 获取博客数量
func GetBlogsNumWithAccount(account string) int {
	store := getBlogStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()
	return len(store.blogs)
}

// GetBlogsWithAccount 获取所有博客
func GetBlogsWithAccount(account string) map[string]*module.Blog {
	store := getBlogStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.blogs
}

// GetBlogWithAccount 获取单个博客
func GetBlogWithAccount(account, title string) *module.Blog {
	store := getBlogStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	if b, ok := store.blogs[title]; ok {
		return b
	}
	return db.GetBlogWithAccount(account, title)
}

// ImportBlogsFromPathWithAccount 从路径导入博客（支持子目录递归导入）
func ImportBlogsFromPathWithAccount(account, dir string) {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	// 加载数据库博客
	blogs := db.GetBlogsByAccount(account)
	if blogs != nil {
		for _, b := range blogs {
			if b.Encrypt == 1 {
				b.AuthType = module.EAuthType_encrypt
			}
			store.blogs[b.Title] = b
		}
	}

	// 从目录递归导入（支持子文件夹）
	files := ioutils.GetFilesRecursive(dir)
	for _, file := range files {
		// 计算相对路径作为 title（支持子文件夹如 agent_tasks/xxx/output）
		relPath, err := filepath.Rel(dir, file)
		if err != nil {
			name, _ := ioutils.GetBaseAndExt(file)
			relPath = name + filepath.Ext(file)
		}
		// 去掉扩展名，统一使用 / 分隔符
		name := strings.TrimSuffix(relPath, filepath.Ext(relPath))
		name = filepath.ToSlash(name)

		datas, size := ioutils.GetFileDatas(file)
		if size > 0 {
			if b, ok := store.blogs[name]; ok {
				b.Account = account
				b.Content = datas
				log.DebugF(log.ModuleBlog, "import update blog %s", name)
			} else {
				now := strTime()
				store.blogs[name] = &module.Blog{
					Title:      name,
					Content:    datas,
					CreateTime: now,
					ModifyTime: now,
					AccessTime: now,
					AuthType:   module.EAuthType_private,
					Account:    account,
				}
				log.DebugF(log.ModuleBlog, "import add blog %s", name)
			}
		}
	}
	db.SaveBlogs(account, store.blogs)
}

// AddBlogWithAccount 添加博客
func AddBlogWithAccount(account string, udb *module.UploadedBlogData) int {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	title := udb.Title
	authType := udb.AuthType

	// 日期后缀
	if config.IsTitleAddDateSuffix(title) == 1 {
		title = fmt.Sprintf("%s_%s", title, time.Now().Format("2006-01-02"))
	}

	if _, ok := store.blogs[title]; ok {
		return 1 // 已存在
	}

	// 日记标志
	if config.IsDiaryBlogWithAccount(account, title) {
		authType |= module.EAuthType_diary
		log.DebugF(log.ModuleBlog, "检测到日记博客，设置日记权限: %s", title)
	}

	now := strTime()
	b := &module.Blog{
		Title:      title,
		Content:    udb.Content,
		CreateTime: now,
		ModifyTime: now,
		AccessTime: now,
		AuthType:   authType,
		Tags:       udb.Tags,
		Encrypt:    udb.Encrypt,
		Account:    udb.Account,
	}
	if b.Encrypt == 1 {
		b.AuthType = module.EAuthType_encrypt
	}

	log.DebugF(log.ModuleBlog, "add blog %s", title)
	store.blogs[title] = b
	db.SaveBlog(account, b)
	return 0
}

// ModifyBlogWithAccount 修改博客
func ModifyBlogWithAccount(account string, udb *module.UploadedBlogData) int {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	b, ok := store.blogs[udb.Title]
	if !ok {
		return 1
	}

	authType := udb.AuthType
	if config.IsDiaryBlogWithAccount(account, udb.Title) {
		authType |= module.EAuthType_diary
	}

	log.DebugF(log.ModuleBlog, "modify blog %s", udb.Title)
	b.Content = udb.Content
	b.ModifyTime = strTime()
	b.ModifyNum++
	b.AuthType = authType
	b.Tags = udb.Tags
	if udb.Account != "" {
		b.Account = udb.Account
	}

	db.SaveBlog(account, b)
	return 0
}

// DeleteBlogWithAccount 删除博客
func DeleteBlogWithAccount(account, title string) int {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.blogs[title]; !ok {
		return 1
	}
	if config.IsSysFile(title) == 1 {
		return 2
	}
	if db.DeleteBlogWithAccount(account, title) == 1 {
		return 3
	}
	delete(store.blogs, title)
	return 0
}

// GetRecentlyTimedBlogWithAccount 获取最近的定时博客
func GetRecentlyTimedBlogWithAccount(account, title string) *module.Blog {
	store := getBlogStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	for i := 1; i < 9999; i++ {
		str := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		newTitle := fmt.Sprintf("%s_%s", title, str)
		log.DebugF(log.ModuleBlog, "GetRecentlyTimedBlog title=%s", newTitle)
		if b, ok := store.blogs[newTitle]; ok {
			return b
		}
	}
	return nil
}

// GetAllWithAccount 获取指定权限的博客列表
func GetAllWithAccount(account string, num int, flag int) []*module.Blog {
	store := getBlogStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	s := make([]*module.Blog, 0)
	for _, b := range store.blogs {
		if (flag & b.AuthType) != 0 {
			s = append(s, b)
		}
	}
	sort.Slice(s, func(i, j int) bool {
		ti, _ := time.Parse("2006-01-02 15:04:05", s[i].ModifyTime)
		tj, _ := time.Parse("2006-01-02 15:04:05", s[j].ModifyTime)
		return ti.Unix() > tj.Unix()
	})
	if num > 0 {
		num = num - 1
	}
	if num > 0 && len(s) > num {
		return s[:num]
	}
	return s
}

// UpdateAccessTimeWithAccount 更新访问时间
func UpdateAccessTimeWithAccount(account string, b *module.Blog) {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	b.AccessTime = strTime()
	b.AccessNum++
	db.SaveBlog(account, b)
}

// GetBlogAuthTypeWithAccount 获取博客权限类型
func GetBlogAuthTypeWithAccount(account, blogname string) int {
	store := getBlogStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	if b, ok := store.blogs[blogname]; ok {
		return b.AuthType
	}
	return 0
}

// IsPublicTag 检查是否公开标签
func IsPublicTag(tag string) int {
	return config.IsPublicTag(tag)
}

// TagReplaceWithAccount 替换标签
func TagReplaceWithAccount(account, from, to string) []*module.Blog {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	blogs := []*module.Blog{}
	lowerFrom := strings.ToLower(from)
	lowerTo := strings.ToLower(to)

	for _, b := range store.blogs {
		if !strings.Contains(strings.ToLower(b.Tags), lowerFrom) {
			continue
		}

		if strings.ToLower(b.Tags) == lowerFrom {
			b.Tags = lowerTo
		} else {
			newTags := ""
			tags := strings.Split(b.Tags, "|")
			for _, tag := range tags {
				if strings.ToLower(tag) == lowerFrom {
					if to != "" {
						newTags = newTags + lowerTo + "|"
					}
				} else {
					newTags = newTags + tag + "|"
				}
			}
			if len(newTags) > 0 {
				newTags = newTags[:len(newTags)-1]
			}
			log.InfoF(log.ModuleBlog, "blog change tag from %s to %s", b.Tags, newTags)
			b.Tags = newTags
		}

		// 去重
		b.Tags = deduplicateTags(b.Tags)
		db.SaveBlog(account, b)
		blogs = append(blogs, b)
	}
	return blogs
}

// TagAddWithAccount 添加标签
func TagAddWithAccount(account, title, newtag string) []*module.Blog {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	blogs := []*module.Blog{}
	lowerNewtag := strings.ToLower(newtag)

	for _, b := range store.blogs {
		if !strings.Contains(strings.ToLower(b.Title), strings.ToLower(title)) {
			continue
		}
		if strings.Contains(strings.ToLower(b.Tags), lowerNewtag) {
			continue
		}

		if b.Tags == "" {
			b.Tags = lowerNewtag
		} else {
			b.Tags = fmt.Sprintf("%s|%s", b.Tags, lowerNewtag)
		}
		log.InfoF(log.ModuleBlog, "blog add new tag %s", b.Tags)

		b.Tags = deduplicateTags(b.Tags)
		db.SaveBlog(account, b)
		blogs = append(blogs, b)
	}
	return blogs
}

// deduplicateTags 标签去重
func deduplicateTags(tags string) string {
	if tags == "" {
		return ""
	}
	parts := strings.Split(tags, "|")
	used := make(map[string]bool)
	result := ""
	for _, tag := range parts {
		lt := strings.ToLower(tag)
		if !used[lt] {
			used[lt] = true
			result += tag + "|"
		}
	}
	if len(result) > 0 {
		result = result[:len(result)-1]
	}
	return result
}

// SetSameAuthWithAccount 设置相同权限
func SetSameAuthWithAccount(account, blogname string) {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	blog, ok := store.blogs[blogname]
	if !ok {
		return
	}

	names := getURLBlogNames(blog)
	for _, name := range names {
		if b, ok := store.blogs[name]; ok {
			b.AuthType = blog.AuthType
			db.SaveBlog(account, b)
		}
	}
}

// getURLBlogNames 获取博客内链接的博客名
func getURLBlogNames(blog *module.Blog) []string {
	names := make([]string, 0)
	if blog == nil {
		return names
	}
	linkPattern := regexp.MustCompile(`\[(.*?)\]\(/get\?blogname=(.*?)\)`)
	for _, t := range strings.Split(blog.Content, "\n") {
		if m := linkPattern.FindStringSubmatch(t); m != nil {
			names = append(names, m[2])
		}
	}
	return names
}

// AddAuthTypeWithAccount 添加权限类型
func AddAuthTypeWithAccount(account, blogname string, flag int) {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	if b, ok := store.blogs[blogname]; ok {
		b.AuthType |= flag
		db.SaveBlog(account, b)
	}
}

// DelAuthTypeWithAccount 删除权限类型
func DelAuthTypeWithAccount(account, blogname string, flag int) {
	store := getBlogStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	if b, ok := store.blogs[blogname]; ok {
		b.AuthType &= ^flag
		if b.AuthType == 0 {
			b.AuthType = module.EAuthType_private
		}
		db.SaveBlog(account, b)
	}
}

// GetURLBlogNamesWithAccount 获取博客内链接的博客名
func GetURLBlogNamesWithAccount(account, blogname string) []string {
	store := getBlogStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	blog, ok := store.blogs[blogname]
	if !ok {
		return []string{}
	}
	return getURLBlogNames(blog)
}

// ========== 年度计划 API ==========

// YearPlanData 年度计划数据结构
type YearPlanData struct {
	YearOverview string                 `json:"yearOverview"`
	MonthPlans   []string               `json:"monthPlans"`
	Year         int                    `json:"year"`
	Tasks        map[string]interface{} `json:"tasks"`
}

// GetYearPlanWithAccount 获取年度计划
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

	log.DebugF(log.ModuleBlog, "获取年计划 - 年份: %d, 任务数据大小: %d", year, len(planData.Tasks))
	if planData.Tasks == nil {
		planData.Tasks = make(map[string]interface{})
	}

	// 从原始JSON恢复任务数据
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(blog.Content), &rawData); err == nil {
		if tasks, ok := rawData["tasks"].(map[string]interface{}); ok && len(tasks) > 0 {
			if len(planData.Tasks) == 0 {
				planData.Tasks = tasks
			}
		}
	}
	return &planData, nil
}

// SaveYearPlanWithAccount 保存年度计划
func SaveYearPlanWithAccount(account string, planData *YearPlanData) error {
	if planData.Year < 2020 || planData.Year > 2100 {
		return fmt.Errorf("无效的年份: %d", planData.Year)
	}
	if len(planData.MonthPlans) != 12 {
		return fmt.Errorf("月度计划数量不正确，应为12个月")
	}

	log.DebugF(log.ModuleBlog, "保存计划 - 年份: %d, 任务数据大小: %d", planData.Year, len(planData.Tasks))

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
		log.DebugF(log.ModuleBlog, "新建年计划博客: %s", planTitle)
	} else {
		ret = ModifyBlogWithAccount(account, &udb)
		log.DebugF(log.ModuleBlog, "更新年计划博客: %s", planTitle)
	}

	if ret != 0 {
		return fmt.Errorf("保存计划失败，错误码: %d", ret)
	}
	return nil
}

// ========== 向后兼容 ==========

// GetAccountFromSession 从 session 获取账户
func GetAccountFromSession(session string) string {
	if session == "" {
		return ""
	}
	return auth.GetAccountBySession(session)
}
