package share

import (
	"config"
	"fmt"
	log "mylog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== Simple Share 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

// 数据结构
type SharedBlog struct {
	Pwd     string
	Title   string
	Count   int
	URL     string
	Timeout int64
}

type SharedTag struct {
	Pwd     string
	Tag     string
	Count   int
	URL     string
	Timeout int64
}

var (
	sharedBlogs map[string]*SharedBlog
	sharedTags  map[string]*SharedTag
	mu          sync.RWMutex
)

func Info() {
	log.InfoF(log.ModuleShare, "info share v9.0 (simple)")
}

// Init 初始化 Share 模块
func Init() {
	mu.Lock()
	defer mu.Unlock()
	sharedBlogs = make(map[string]*SharedBlog)
	sharedTags = make(map[string]*SharedTag)
}

// ========== 辅助函数 ==========

func get7DaysTimeout() int64 {
	utcTimestamp := time.Now().UTC().Unix()
	shareDays, err := strconv.Atoi(config.GetConfigWithAccount(config.GetAdminAccount(), "share_days"))
	if err != nil {
		shareDays = 7
	}
	return utcTimestamp + (int64(shareDays) * 24 * 3600)
}

// ========== 对外接口 ==========

// GetSharedBlog 获取共享博客
func GetSharedBlog(title string) *SharedBlog {
	mu.RLock()
	defer mu.RUnlock()
	return sharedBlogs[title]
}

// GetSharedTag 获取共享标签
func GetSharedTag(tag string) *SharedTag {
	mu.RLock()
	defer mu.RUnlock()
	return sharedTags[tag]
}

// GetSharedBlogs 获取所有共享博客
func GetSharedBlogs() map[string]*SharedBlog {
	mu.RLock()
	defer mu.RUnlock()
	return sharedBlogs
}

// GetSharedTags 获取所有共享标签
func GetSharedTags() map[string]*SharedTag {
	mu.RLock()
	defer mu.RUnlock()
	return sharedTags
}

// AddSharedBlog 添加共享博客
func AddSharedBlog(title string) (url, pwd string) {
	mu.Lock()
	defer mu.Unlock()

	if b, ok := sharedBlogs[title]; ok {
		b.Count++
		return b.URL, b.Pwd
	}

	pwd = uuid.New().String()
	url = fmt.Sprintf("/getshare?t=0&name=%s&pwd=%s", title, pwd)
	sharedBlogs[title] = &SharedBlog{
		Title:   title,
		Count:   9999,
		Pwd:     pwd,
		URL:     url,
		Timeout: get7DaysTimeout(),
	}
	return url, pwd
}

// AddSharedTag 添加共享标签
func AddSharedTag(tag string) (url, pwd string) {
	mu.Lock()
	defer mu.Unlock()

	if t, ok := sharedTags[tag]; ok {
		t.Count++
		return t.URL, t.Pwd
	}

	pwd = uuid.New().String()
	url = fmt.Sprintf("/getshare?t=1&name=%s&pwd=%s", tag, pwd)
	sharedTags[tag] = &SharedTag{
		Tag:     tag,
		Count:   9999,
		Pwd:     pwd,
		URL:     url,
		Timeout: get7DaysTimeout(),
	}
	return url, pwd
}

// ModifyCntSharedBlog 修改共享博客计数
func ModifyCntSharedBlog(title string, c int) int {
	mu.Lock()
	defer mu.Unlock()

	b, ok := sharedBlogs[title]
	if !ok {
		return -1
	}

	b.Count += c
	if b.Count < 0 {
		delete(sharedBlogs, title)
		return -2
	}

	if b.Timeout < time.Now().UTC().Unix() {
		delete(sharedBlogs, title)
		return -3
	}

	return b.Count
}

// ModifyCntSharedTag 修改共享标签计数
func ModifyCntSharedTag(tag string, c int) int {
	mu.Lock()
	defer mu.Unlock()

	t, ok := sharedTags[tag]
	if !ok {
		return -1
	}

	t.Count += c
	if t.Count < 0 {
		delete(sharedTags, tag)
		return -2
	}

	if t.Timeout < time.Now().UTC().Unix() {
		delete(sharedTags, tag)
		return -3
	}

	return t.Count
}
