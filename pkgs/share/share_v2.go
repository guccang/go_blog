package share

import (
	"config"
	"core"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// ========== 新版 Share Actor (基于泛型框架) ==========

// ShareActorV2 使用新版 ActorV2 框架
type ShareActorV2 struct {
	*core.ActorV2
	sharedBlogs map[string]*SharedBlog
	sharedTags  map[string]*SharedTag
}

var share_module_v2 *ShareActorV2

// InitV2 初始化新版 Share 模块
func InitV2() {
	share_module_v2 = &ShareActorV2{
		ActorV2:     core.NewActorV2(),
		sharedBlogs: make(map[string]*SharedBlog),
		sharedTags:  make(map[string]*SharedTag),
	}
}

// ========== 辅助函数 ==========

func get7DaysTimeOutStampV2() int64 {
	utcTimestamp := time.Now().UTC().Unix()
	shareDays, err := strconv.Atoi(config.GetConfigWithAccount(config.GetAdminAccount(), "share_days"))
	if err != nil {
		shareDays = 7
	}
	return utcTimestamp + (int64(shareDays) * 24 * 3600)
}

// ========== 对外接口 ==========

// GetSharedBlogV2 获取共享博客
func GetSharedBlogV2(title string) *SharedBlog {
	return core.Execute(share_module_v2.ActorV2, func() *SharedBlog {
		return share_module_v2.sharedBlogs[title]
	})
}

// GetSharedTagV2 获取共享标签
func GetSharedTagV2(tag string) *SharedTag {
	return core.Execute(share_module_v2.ActorV2, func() *SharedTag {
		return share_module_v2.sharedTags[tag]
	})
}

// GetSharedBlogsV2 获取所有共享博客
func GetSharedBlogsV2() map[string]*SharedBlog {
	return core.Execute(share_module_v2.ActorV2, func() map[string]*SharedBlog {
		return share_module_v2.sharedBlogs
	})
}

// GetSharedTagsV2 获取所有共享标签
func GetSharedTagsV2() map[string]*SharedTag {
	return core.Execute(share_module_v2.ActorV2, func() map[string]*SharedTag {
		return share_module_v2.sharedTags
	})
}

// AddSharedBlogV2 添加共享博客
func AddSharedBlogV2(title string) (url, pwd string) {
	return core.Execute2(share_module_v2.ActorV2, func() (string, string) {
		if b, ok := share_module_v2.sharedBlogs[title]; ok {
			b.Count++
			return b.URL, b.Pwd
		}

		pwd := uuid.New().String()
		url := fmt.Sprintf("/getshare?t=0&name=%s&pwd=%s", title, pwd)
		share_module_v2.sharedBlogs[title] = &SharedBlog{
			Title:   title,
			Count:   9999,
			Pwd:     pwd,
			URL:     url,
			Timeout: get7DaysTimeOutStampV2(),
		}
		return url, pwd
	})
}

// AddSharedTagV2 添加共享标签
func AddSharedTagV2(tag string) (url, pwd string) {
	return core.Execute2(share_module_v2.ActorV2, func() (string, string) {
		if t, ok := share_module_v2.sharedTags[tag]; ok {
			t.Count++
			return t.URL, t.Pwd
		}

		pwd := uuid.New().String()
		url := fmt.Sprintf("/getshare?t=1&name=%s&pwd=%s", tag, pwd)
		share_module_v2.sharedTags[tag] = &SharedTag{
			Tag:     tag,
			Count:   9999,
			Pwd:     pwd,
			URL:     url,
			Timeout: get7DaysTimeOutStampV2(),
		}
		return url, pwd
	})
}

// ModifyCntSharedBlogV2 修改共享博客计数
func ModifyCntSharedBlogV2(title string, c int) int {
	return core.Execute(share_module_v2.ActorV2, func() int {
		b, ok := share_module_v2.sharedBlogs[title]
		if !ok {
			return -1
		}

		b.Count += c
		if b.Count < 0 {
			delete(share_module_v2.sharedBlogs, title)
			return -2
		}

		utcTimestamp := time.Now().UTC().Unix()
		if b.Timeout < utcTimestamp {
			delete(share_module_v2.sharedBlogs, title)
			return -3
		}

		return b.Count
	})
}

// ModifyCntSharedTagV2 修改共享标签计数
func ModifyCntSharedTagV2(tag string, c int) int {
	return core.Execute(share_module_v2.ActorV2, func() int {
		t, ok := share_module_v2.sharedTags[tag]
		if !ok {
			return -1
		}

		t.Count += c
		if t.Count < 0 {
			delete(share_module_v2.sharedTags, tag)
			return -2
		}

		utcTimestamp := time.Now().UTC().Unix()
		if t.Timeout < utcTimestamp {
			delete(share_module_v2.sharedTags, tag)
			return -3
		}

		return t.Count
	})
}
