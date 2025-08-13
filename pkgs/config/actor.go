package config

import (
	"core"
	log "mylog"
	"strings"
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
  第三步: 在config.go中添加对应的接口
  第四步: 在http中添加对应的接口

  上述精炼步骤产生过程:
  1. claudecode 实现版本
  2. 手写实现版本
  3. cursor+gpt5实现版本
  4. 最终综合上述不同实现版本的优点，有了上述的实现步骤.
  5. 最终实现版本 基于cmd的可撤回的actor并发模型,依赖于go的interface特性,简化了实现方式，非常特别的体验
*/

// actor
type ConfigActor struct {
	*core.Actor
	datas           map[string]string
	autodatesuffix  []string
	publictags      []string
	diary_keywords  []string
	config_path     string
	sys_files       []string
	blog_version    string
}

// 获取配置值
func (aconfig *ConfigActor) getConfig(name string) string {
	v, ok := aconfig.datas[name]
	if !ok {
		return ""
	}
	return v
}

// 重新加载配置
func (aconfig *ConfigActor) reloadConfig(filePath string) int {
	err := aconfig.loadConfigInternal(filePath)
	if err != nil {
		log.ErrorF("ReloadConfig err=%s", err.Error())
		return 1
	}
	return 0
}

// 获取版本信息
func (aconfig *ConfigActor) getVersion() string {
	return aconfig.blog_version
}

// 检查是否为系统文件
func (aconfig *ConfigActor) isSysFile(name string) int {
	for _, v := range aconfig.sys_files {
		if v == name {
			return 1
		}
	}
	return 0
}

// 检查是否为公开标签
func (aconfig *ConfigActor) isPublicTag(tag string) int {
	for _, v := range aconfig.publictags {
		log.DebugF("IsPublicTag %s %s", v, tag)
		if v == tag {
			return 1
		}
	}
	return 0
}

// 检查标题是否需要添加日期后缀
func (aconfig *ConfigActor) isTitleAddDateSuffix(title string) int {
	for _, v := range aconfig.autodatesuffix {
		if v == title {
			return 1
		}
	}
	return 0
}

// 检查是否为日记博客
func (aconfig *ConfigActor) isDiaryBlog(title string) bool {
	for _, keyword := range aconfig.diary_keywords {
		if len(title) >= len(keyword) && title[:len(keyword)] == keyword {
			return true
		}
	}
	return false
}

// 获取日记关键字列表
func (aconfig *ConfigActor) getDiaryKeywords() []string {
	return aconfig.diary_keywords
}

// 获取配置文件路径
func (aconfig *ConfigActor) getConfigPath() string {
	return aconfig.config_path
}

// 检查标题是否包含日期后缀
func (aconfig *ConfigActor) isTitleContainsDateSuffix(title string) int {
	for _, v := range aconfig.autodatesuffix {
		if strings.Contains(strings.ToLower(title), strings.ToLower(v)) {
			return 1
		}
	}
	return 0
}