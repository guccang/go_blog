package blog

import (
	"config"
	"core"
	"encoding/json"
	"fmt"
	"ioutils"
	"module"
	log "mylog"
	db "persistence"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 新版 Blog Actor (基于泛型框架) ==========

// BlogActorV2 使用新版 ActorV2 框架
type BlogActorV2 struct {
	*core.ActorV2
	Account string
	blogs   map[string]*module.Blog
}

// 多账户 Actor 管理
var (
	blogActorsV2   = make(map[string]*BlogActorV2)
	blogActorMutex sync.Mutex
)

// getBlogActorV2 获取或创建指定账户的 BlogActor
func getBlogActorV2(account string) *BlogActorV2 {
	blogActorMutex.Lock()
	defer blogActorMutex.Unlock()

	if actor, ok := blogActorsV2[account]; ok {
		return actor
	}

	// 创建新 Actor
	actor := &BlogActorV2{
		ActorV2: core.NewActorV2(),
		Account: account,
		blogs:   make(map[string]*module.Blog),
	}

	// 从数据库加载博客
	blogs := db.GetBlogsByAccount(account)
	if blogs != nil {
		for _, b := range blogs {
			if b.Encrypt == 1 {
				b.AuthType = module.EAuthType_encrypt
			}
			actor.blogs[b.Title] = b
		}
	}
	log.DebugF(log.ModuleBlog, "BlogActorV2 loaded blogs for account=%s, count=%d", account, len(actor.blogs))

	blogActorsV2[account] = actor
	return actor
}

// ========== 辅助函数 ==========

func strTimeV2() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ========== 对外接口 (全部使用泛型框架) ==========

// GetBlogsNumWithAccountV2 获取博客数量
func GetBlogsNumWithAccountV2(account string) int {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() int {
		return len(actor.blogs)
	})
}

// GetBlogsWithAccountV2 获取所有博客
func GetBlogsWithAccountV2(account string) map[string]*module.Blog {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() map[string]*module.Blog {
		return actor.blogs
	})
}

// GetBlogWithAccountV2 获取单个博客
func GetBlogWithAccountV2(account, title string) *module.Blog {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() *module.Blog {
		if b, ok := actor.blogs[title]; ok {
			return b
		}
		// 尝试从数据库加载
		return db.GetBlogWithAccount(account, title)
	})
}

// AddBlogWithAccountV2 添加博客
func AddBlogWithAccountV2(account string, udb *module.UploadedBlogData) int {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() int {
		title := udb.Title
		content := udb.Content
		authType := udb.AuthType
		tags := udb.Tags

		// 日期后缀
		if config.IsTitleAddDateSuffix(title) == 1 {
			str := time.Now().Format("2006-01-02")
			title = fmt.Sprintf("%s_%s", title, str)
		}

		if _, ok := actor.blogs[title]; ok {
			return 1 // 已存在
		}

		// 日记自动标志
		if config.IsDiaryBlogWithAccount(account, title) {
			authType |= module.EAuthType_diary
		}

		now := strTimeV2()
		b := &module.Blog{
			Title:      title,
			Content:    content,
			CreateTime: now,
			ModifyTime: now,
			AccessTime: now,
			ModifyNum:  0,
			AccessNum:  0,
			AuthType:   authType,
			Tags:       tags,
			Encrypt:    udb.Encrypt,
			Account:    udb.Account,
		}
		if b.Encrypt == 1 {
			b.AuthType = module.EAuthType_encrypt
		}

		actor.blogs[title] = b
		db.SaveBlog(account, b)
		return 0
	})
}

// ModifyBlogWithAccountV2 修改博客
func ModifyBlogWithAccountV2(account string, udb *module.UploadedBlogData) int {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() int {
		b, ok := actor.blogs[udb.Title]
		if !ok {
			return 1
		}

		authType := udb.AuthType
		if config.IsDiaryBlogWithAccount(account, udb.Title) {
			authType |= module.EAuthType_diary
		}

		b.Content = udb.Content
		b.ModifyTime = strTimeV2()
		b.ModifyNum++
		b.AuthType = authType
		b.Tags = udb.Tags
		if udb.Account != "" {
			b.Account = udb.Account
		}

		db.SaveBlog(account, b)
		return 0
	})
}

// DeleteBlogWithAccountV2 删除博客
func DeleteBlogWithAccountV2(account, title string) int {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() int {
		if _, ok := actor.blogs[title]; !ok {
			return 1
		}
		if config.IsSysFile(title) == 1 {
			return 2
		}
		ret := db.DeleteBlogWithAccount(account, title)
		if ret == 1 {
			return 3
		}
		delete(actor.blogs, title)
		return 0
	})
}

// GetAllWithAccountV2 获取指定权限的博客列表
func GetAllWithAccountV2(account string, num int, flag int) []*module.Blog {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() []*module.Blog {
		s := make([]*module.Blog, 0)
		for _, b := range actor.blogs {
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
	})
}

// UpdateAccessTimeWithAccountV2 更新访问时间
func UpdateAccessTimeWithAccountV2(account string, b *module.Blog) {
	actor := getBlogActorV2(account)
	core.Fire(actor.ActorV2, func() {
		b.AccessTime = strTimeV2()
		b.AccessNum++
		db.SaveBlog(account, b)
	})
}

// GetBlogAuthTypeWithAccountV2 获取博客权限类型
func GetBlogAuthTypeWithAccountV2(account, blogname string) int {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() int {
		if b, ok := actor.blogs[blogname]; ok {
			return b.AuthType
		}
		return 0
	})
}

// AddAuthTypeWithAccountV2 添加权限类型
func AddAuthTypeWithAccountV2(account, blogname string, flag int) {
	actor := getBlogActorV2(account)
	core.Fire(actor.ActorV2, func() {
		if b, ok := actor.blogs[blogname]; ok {
			b.AuthType |= flag
			db.SaveBlog(account, b)
		}
	})
}

// DelAuthTypeWithAccountV2 删除权限类型
func DelAuthTypeWithAccountV2(account, blogname string, flag int) {
	actor := getBlogActorV2(account)
	core.Fire(actor.ActorV2, func() {
		if b, ok := actor.blogs[blogname]; ok {
			b.AuthType &= ^flag
			if b.AuthType == 0 {
				b.AuthType = module.EAuthType_private
			}
			db.SaveBlog(account, b)
		}
	})
}

// GetRecentlyTimedBlogWithAccountV2 获取最近的定时博客
func GetRecentlyTimedBlogWithAccountV2(account, title string) *module.Blog {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() *module.Blog {
		for i := 1; i < 9999; i++ {
			str := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
			newTitle := fmt.Sprintf("%s_%s", title, str)
			if b, ok := actor.blogs[newTitle]; ok {
				return b
			}
		}
		return nil
	})
}

// GetURLBlogNamesWithAccountV2 获取博客内链接的博客名
func GetURLBlogNamesWithAccountV2(account, blogname string) []string {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() []string {
		names := make([]string, 0)
		blog, ok := actor.blogs[blogname]
		if !ok {
			return names
		}
		linkPattern := regexp.MustCompile(`\[(.*?)\]\(/get\?blogname=(.*?)\)`)
		tokens := strings.Split(blog.Content, "\n")
		for _, t := range tokens {
			if linkMatches := linkPattern.FindStringSubmatch(t); linkMatches != nil {
				names = append(names, linkMatches[2])
			}
		}
		return names
	})
}

// TagReplaceWithAccountV2 替换标签
func TagReplaceWithAccountV2(account, from, to string) []*module.Blog {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() []*module.Blog {
		blogs := []*module.Blog{}
		lowerFrom := strings.ToLower(from)
		lowerTo := strings.ToLower(to)
		for _, b := range actor.blogs {
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
				b.Tags = newTags
			}

			// 去重
			tags := strings.Split(b.Tags, "|")
			used := make(map[string]bool)
			newTags := ""
			for _, tag := range tags {
				lowerTag := strings.ToLower(tag)
				if !used[lowerTag] {
					used[lowerTag] = true
					newTags = newTags + tag + "|"
				}
			}
			if len(newTags) > 0 {
				newTags = newTags[:len(newTags)-1]
			}
			b.Tags = newTags
			db.SaveBlog(account, b)
			blogs = append(blogs, b)
		}
		return blogs
	})
}

// TagAddWithAccountV2 添加标签
func TagAddWithAccountV2(account, title, newtag string) []*module.Blog {
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() []*module.Blog {
		blogs := []*module.Blog{}
		lowerNewtag := strings.ToLower(newtag)
		for _, b := range actor.blogs {
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

			// 去重
			tags := strings.Split(b.Tags, "|")
			used := make(map[string]bool)
			newTags := ""
			for _, tag := range tags {
				lowerTag := strings.ToLower(tag)
				if !used[lowerTag] {
					used[lowerTag] = true
					newTags = newTags + tag + "|"
				}
			}
			if len(newTags) > 0 {
				newTags = newTags[:len(newTags)-1]
			}
			b.Tags = newTags
			db.SaveBlog(account, b)
			blogs = append(blogs, b)
		}
		return blogs
	})
}

// SetSameAuthWithAccountV2 设置相同权限
func SetSameAuthWithAccountV2(account, blogname string) {
	actor := getBlogActorV2(account)
	core.Fire(actor.ActorV2, func() {
		blog, ok := actor.blogs[blogname]
		if !ok {
			return
		}
		// 获取链接的博客名
		linkPattern := regexp.MustCompile(`\[(.*?)\]\(/get\?blogname=(.*?)\)`)
		tokens := strings.Split(blog.Content, "\n")
		for _, t := range tokens {
			if linkMatches := linkPattern.FindStringSubmatch(t); linkMatches != nil {
				name := linkMatches[2]
				if b, ok := actor.blogs[name]; ok {
					b.AuthType = blog.AuthType
					db.SaveBlog(account, b)
				}
			}
		}
	})
}

// ImportBlogsFromPathWithAccountV2 从路径导入博客
func ImportBlogsFromPathWithAccountV2(account, dir string) {
	actor := getBlogActorV2(account)
	core.Fire(actor.ActorV2, func() {
		files := ioutils.GetFiles(dir)
		for _, file := range files {
			name, _ := ioutils.GetBaseAndExt(file)
			datas, size := ioutils.GetFileDatas(file)
			if size > 0 {
				if b, ok := actor.blogs[name]; ok {
					// 更新
					b.Account = account
					b.Content = datas
					log.DebugF(log.ModuleBlog, "import update blog %s", name)
				} else {
					// 新增
					now := strTimeV2()
					newBlog := &module.Blog{
						Title:      name,
						Content:    datas,
						CreateTime: now,
						ModifyTime: now,
						AccessTime: now,
						AuthType:   module.EAuthType_private,
						Account:    account,
					}
					actor.blogs[name] = newBlog
					log.DebugF(log.ModuleBlog, "import add blog %s", name)
				}
			}
		}
		db.SaveBlogs(account, actor.blogs)
	})
}

// ========== 年度计划 API ==========

// GetYearPlanWithAccountV2 获取年度计划
func GetYearPlanWithAccountV2(account string, year int) (*YearPlanData, error) {
	planTitle := fmt.Sprintf("年度计划%d", year)
	actor := getBlogActorV2(account)
	return core.Execute2(actor.ActorV2, func() (*YearPlanData, error) {
		blog, ok := actor.blogs[planTitle]
		if !ok {
			// 创建新的年度计划
			planData := &YearPlanData{
				Year:         year,
				YearOverview: "",
				MonthPlans:   make([]string, 12),
				Tasks:        make(map[string]interface{}),
			}
			return planData, nil
		}

		var planData YearPlanData
		if err := json.Unmarshal([]byte(blog.Content), &planData); err != nil {
			return nil, fmt.Errorf("解析年度计划失败: %v", err)
		}
		return &planData, nil
	})
}

// SaveYearPlanWithAccountV2 保存年度计划
func SaveYearPlanWithAccountV2(account string, planData *YearPlanData) error {
	planTitle := fmt.Sprintf("年度计划%d", planData.Year)
	actor := getBlogActorV2(account)
	return core.Execute(actor.ActorV2, func() error {
		content, err := json.Marshal(planData)
		if err != nil {
			return fmt.Errorf("序列化年度计划失败: %v", err)
		}

		blog, ok := actor.blogs[planTitle]
		if !ok {
			// 创建新博客
			now := strTimeV2()
			blog = &module.Blog{
				Title:      planTitle,
				Content:    string(content),
				CreateTime: now,
				ModifyTime: now,
				AccessTime: now,
				AuthType:   module.EAuthType_private,
				Account:    account,
			}
			actor.blogs[planTitle] = blog
		} else {
			blog.Content = string(content)
			blog.ModifyTime = strTimeV2()
			blog.ModifyNum++
		}

		db.SaveBlog(account, blog)
		return nil
	})
}
