package blog

import (
	"config"
	"core"
	"fmt"
	"ioutils"
	"module"
	log "mylog"
	db "persistence"
	"regexp"
	"sort"
	"strings"
	"time"
)

// BlogActor owns blog state and performs operations via the actor model
type BlogActor struct {
	*core.Actor
	blogs map[string]*module.Blog
}

func strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// loadBlogs loads blogs from persistence into memory
func (a *BlogActor) loadBlogs() int {
	blogs := db.GetBlogs()
	if blogs != nil {
		for _, b := range blogs {
			if b.Encrypt == 1 {
				b.AuthType = module.EAuthType_encrypt
			}
			a.blogs[b.Title] = b
		}
	}
	log.DebugF("getblogs number=%d", len(blogs))
	return 0
}

// importBlogsFromPath reads files from a dir and adds them as private blogs
func (a *BlogActor) importBlogsFromPath(dir string) int {
	files := ioutils.GetFiles(dir)
	for _, file := range files {
		name, _ := ioutils.GetBaseAndExt(file)
		datas, size := ioutils.GetFileDatas(file)
		if size > 0 {
			udb := module.UploadedBlogData{
				Title:    name,
				Content:  datas,
				AuthType: module.EAuthType_private,
			}
			ret := a.addBlog(&udb)
			if ret == 0 {
				log.DebugF("name=%s size=%d", name, size)
			}
		}
	}
	return 0
}

func (a *BlogActor) getBlog(title string) *module.Blog {
	if b, ok := a.blogs[title]; ok {
		return b
	}
	b := db.GetBlog(title)
	return b
}

func (a *BlogActor) addBlog(udb *module.UploadedBlogData) int {
	title := udb.Title
	content := udb.Content
	authType := udb.AuthType
	tags := udb.Tags

	if config.IsTitleAddDateSuffix(title) == 1 {
		str := time.Now().Format("2006-01-02")
		title = fmt.Sprintf("%s_%s", title, str)
	}

	if _, ok := a.blogs[title]; ok {
		return 1
	}

	// diary auto-flag
	if config.IsDiaryBlog(title) {
		authType |= module.EAuthType_diary
		log.DebugF("检测到日记博客，设置日记权限: %s", title)
	}

	log.DebugF("add blog %s", title)
	now := strTime()
	b := module.Blog{
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
	}
	if b.Encrypt == 1 {
		b.AuthType = module.EAuthType_encrypt
	}

	if (authType & module.EAuthType_diary) != 0 {
		log.InfoF("博客 '%s' 设置了日记权限，AuthType=%d", title, authType)
	}
	if (authType & module.EAuthType_encrypt) != 0 {
		log.InfoF("博客 '%s' 设置了加密权限，AuthType=%d", title, authType)
	}

	a.blogs[title] = &b
	db.SaveBlog(&b)
	return 0
}

func (a *BlogActor) modifyBlog(udb *module.UploadedBlogData) int {
	title := udb.Title
	content := udb.Content
	authType := udb.AuthType
	tags := udb.Tags

	b, ok := a.blogs[title]
	if !ok {
		return 1
	}

	log.DebugF("modify blog %s", title)

	if config.IsDiaryBlog(title) {
		authType |= module.EAuthType_diary
		log.DebugF("保持日记博客权限: %s", title)
	}

	b.Content = content
	b.ModifyTime = strTime()
	b.ModifyNum += 1

	finalAuthType := authType

	b.AuthType = finalAuthType
	b.Tags = tags

	if (authType & module.EAuthType_diary) != 0 {
		log.InfoF("博客 '%s' 更新了日记权限，AuthType=%d", title, authType)
	}
	if (authType & module.EAuthType_encrypt) != 0 {
		log.InfoF("博客 '%s' 更新了加密权限，AuthType=%d", title, authType)
	}

	db.SaveBlog(b)
	return 0
}

func (a *BlogActor) deleteBlog(title string) int {
	if _, ok := a.blogs[title]; !ok {
		return 1
	}
	if config.IsSysFile(title) == 1 {
		return 2
	}
	ret := db.DeleteBlog(title)
	if ret == 1 {
		return 3
	}
	delete(a.blogs, title)
	return 0
}

func (a *BlogActor) getAll(num int, flag int) []*module.Blog {
	s := make([]*module.Blog, 0)
	for _, b := range a.blogs {
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

func (a *BlogActor) updateAccessTime(blog *module.Blog) {
	blog.AccessTime = strTime()
	blog.AccessNum += 1
	db.SaveBlog(blog)
}

func (a *BlogActor) getBlogAuthType(blogname string) int {
	blog := a.getBlog(blogname)
	if blog == nil {
		return 0
	}
	return blog.AuthType
}

func (a *BlogActor) addAuthType(blogname string, flag int) {
	blog := a.getBlog(blogname)
	if blog == nil {
		return
	}
	blog.AuthType |= flag
	db.SaveBlog(blog)
}

func (a *BlogActor) delAuthType(blogname string, flag int) {
	blog := a.getBlog(blogname)
	if blog == nil {
		return
	}
	blog.AuthType &= ^flag
	if blog.AuthType == 0 {
		blog.AuthType = module.EAuthType_private
	}
	db.SaveBlog(blog)
}

func (a *BlogActor) getRecentlyTimedBlog(title string) *module.Blog {
	for i := 1; i < 9999; i++ {
		str := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		newTitle := fmt.Sprintf("%s_%s", title, str)
		log.DebugF("GetRecentlyTimedBlog title=%s", newTitle)
		b := a.getBlog(newTitle)
		if b != nil {
			return b
		}
	}
	return nil
}

// getURLBlogNames extracts linked blog names from a blog content
func (a *BlogActor) getURLBlogNames(blogname string) []string {
	names := make([]string, 0)
	blog := a.getBlog(blogname)
	if blog == nil {
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
}

// Utilities that operate across all blogs
func (a *BlogActor) tagReplace(from, to string) {
	for _, b := range a.blogs {
		if !strings.Contains(strings.ToLower(b.Tags), strings.ToLower(from)) {
			continue
		}
		if from == b.Tags {
			b.Tags = to
		} else {
			newTags := ""
			tags := strings.Split(b.Tags, "|")
			for _, tag := range tags {
				if from == tag {
					if to != "" {
						newTags = newTags + to + "|"
					}
				} else {
					newTags = newTags + tag + "|"
				}
			}
			newTags = newTags[:len(newTags)-1]
			log.InfoF("blog change tag from %s to %s", b.Tags, newTags)
			b.Tags = newTags
		}

		// remove duplicates
		tags := strings.Split(b.Tags, "|")
		used := make(map[string]bool)
		newTags := ""
		for _, tag := range tags {
			if !used[tag] {
				used[tag] = true
			} else {
				continue
			}
			newTags = newTags + tag + "|"
		}
		newTags = newTags[:len(newTags)-1]
		b.Tags = newTags
		db.SaveBlog(b)
	}
}

func (a *BlogActor) setSameAuth(blogname string) {
	blog := a.getBlog(blogname)
	if blog == nil {
		return
	}
	names := a.getURLBlogNames(blogname)
	for _, name := range names {
		b := a.getBlog(name)
		if b != nil {
			b.AuthType = blog.AuthType
			db.SaveBlog(b)
		}
	}
}
