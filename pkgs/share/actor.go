package share

import (
	"config"
	"core"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

/*
goroutine 线程安全
 goroutine 会被调度到任意一个线程上，因此会被任意一个线程执行接口
 线程安全原因
 原因1: 	actor使用chan通信，chan是线程安全的
 原因2: 	actor的mailbox是线程安全的

 添加一个功能需要的四个步骤:
  第一步: 实现功能逻辑
  第二步: 实现对应的cmd
  第三步: 在share.go中添加对应的接口
  第四步: 在http中添加对应的接口
*/

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

// actor
type ShareActor struct {
	*core.Actor
	sharedBlogs map[string]*SharedBlog
	sharedTags  map[string]*SharedTag
}

func (s *ShareActor) getSharedBlog(title string) *SharedBlog {
	b, ok := s.sharedBlogs[title]
	if !ok {
		return nil
	}
	return b
}

func (s *ShareActor) getSharedTag(tag string) *SharedTag {
	t, ok := s.sharedTags[tag]
	if !ok {
		return nil
	}
	return t
}

func (s *ShareActor) get7DaysTimeOutStamp() int64 {
	utcTimestamp := time.Now().UTC().Unix()
	share_days, err := strconv.Atoi(config.GetConfig("share_days"))
	if err != nil {
		share_days = 7
	}
	return utcTimestamp + (int64(share_days) * 24 * 3600)
}

func (s *ShareActor) addSharedBlog(title string) (string, string) {
	b, ok := s.sharedBlogs[title]
	if !ok {
		pwd := uuid.New().String()
		url := fmt.Sprintf("/getshare?t=0&name=%s&pwd=%s", title, pwd)
		s.sharedBlogs[title] = &SharedBlog{
			Title:   title,
			Count:   9999,
			Pwd:     pwd,
			URL:     url,
			Timeout: s.get7DaysTimeOutStamp(),
		}
		return url, pwd
	}

	b.Count = b.Count + 1
	return b.URL, b.Pwd
}

func (s *ShareActor) addSharedTag(tag string) (string, string) {
	t, ok := s.sharedTags[tag]
	if !ok {
		pwd := uuid.New().String()
		url := fmt.Sprintf("/getshare?t=1&name=%s&pwd=%s", tag, pwd)
		s.sharedTags[tag] = &SharedTag{
			Tag:     tag,
			Count:   9999,
			Pwd:     pwd,
			URL:     url,
			Timeout: s.get7DaysTimeOutStamp(),
		}
		return url, pwd
	}
	t.Count = t.Count + 1
	return t.URL, t.Pwd
}

func (s *ShareActor) modifyCntSharedBlog(title string, c int) int {
	b, ok := s.sharedBlogs[title]
	if !ok {
		return -1
	}

	b.Count = b.Count + c
	if b.Count < 0 {
		delete(s.sharedBlogs, title)
		return -2
	}

	utcTimestamp := time.Now().UTC().Unix()
	if b.Timeout < utcTimestamp {
		delete(s.sharedBlogs, title)
		return -3
	}

	return b.Count
}

func (s *ShareActor) modifyCntSharedTag(tag string, c int) int {
	t, ok := s.sharedTags[tag]
	if !ok {
		return -1
	}

	t.Count = t.Count + c
	if t.Count < 0 {
		delete(s.sharedTags, tag)
		return -2
	}

	utcTimestamp := time.Now().UTC().Unix()
	if t.Timeout < utcTimestamp {
		delete(s.sharedTags, tag)
		return -3
	}

	return t.Count
}

type GetSharedBlogsCmd struct {
	core.ActorCommand
}

func (cmd *GetSharedBlogsCmd) Do(actor core.ActorInterface) {
	shareActor := actor.(*ShareActor)
	cmd.Response() <- shareActor.sharedBlogs
}

type GetSharedTagsCmd struct {
	core.ActorCommand
}

func (cmd *GetSharedTagsCmd) Do(actor core.ActorInterface) {
	shareActor := actor.(*ShareActor)
	cmd.Response() <- shareActor.sharedTags
}
