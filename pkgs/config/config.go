package config

import (
	"bufio"
	"core"
	log "mylog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 配置模块actor
var config_module *ConfigActor

func Info() {
	log.Debug("info config v13.0")
}

// 初始化config模块
func Init(filePath string) {
	config_module = &ConfigActor{
		Actor:          core.NewActor(),
		datas:          make(map[string]string),
		autodatesuffix: make([]string, 0),
		publictags:     make([]string, 0),
		diary_keywords: make([]string, 0),
		config_path:    filePath,
		sys_files:      make([]string, 0),
		blog_version:   "Version13.0",
	}

	err := config_module.loadConfigInternal(filePath)
	if err != nil {
		log.ErrorF("Init config err=%s", err.Error())
	}
	config_module.Start(config_module)
}

// interface

func GetVersion() string {
	cmd := &GetVersionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	config_module.Send(cmd)
	version := <-cmd.Response()
	return version.(string)
}

func GetConfigPath() string {
	cmd := &GetConfigPathCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	config_module.Send(cmd)
	path := <-cmd.Response()
	return path.(string)
}

func ReloadConfig(filePath string) {
	cmd := &ReloadConfigCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		FilePath: filePath,
	}
	config_module.Send(cmd)
	<-cmd.Response()
}

// 内部方法：加载配置
func (aconfig *ConfigActor) loadConfigInternal(filePath string) error {
	datas, err := readConfigFile(filePath)
	if err != nil {
		return err
	}
	aconfig.datas = datas
	aconfig.config_path = filePath

	for k, v := range aconfig.datas {
		log.DebugF("CONFIG %s=%s", k, v)
	}

	datetitles, ok := aconfig.datas["title_auto_add_date_suffix"]
	if ok {
		arr := strings.Split(datetitles, "|")
		aconfig.autodatesuffix = arr
	}

	tags, ok := aconfig.datas["publictags"]
	if ok {
		arr := strings.Split(tags, "|")
		aconfig.publictags = arr
	}

	sysfiles, ok := aconfig.datas["sysfiles"]
	if ok {
		arr := strings.Split(sysfiles, "|")
		aconfig.sys_files = arr
	}

	// 从 sys_conf.md 文件中读取日记关键字配置
	aconfig.loadDiaryKeywordsFromSysConf()
	return nil
}

// 从 sys_conf.md 文件中读取日记关键字配置
func (aconfig *ConfigActor) loadDiaryKeywordsFromSysConf() {
	// 获取 blogs_txt 目录路径
	blogsPath := GetBlogsPath()
	sysConfPath := filepath.Join(blogsPath, "sys_conf.md")

	// 检查文件是否存在
	if _, err := os.Stat(sysConfPath); os.IsNotExist(err) {
		log.DebugF("sys_conf.md 文件不存在: %s", sysConfPath)
		// 设置默认的日记关键字
		aconfig.diary_keywords = []string{"日记_"}
		return
	}

	// 读取文件内容
	file, err := os.Open(sysConfPath)
	if err != nil {
		log.ErrorF("无法打开 sys_conf.md 文件: %v", err)
		aconfig.diary_keywords = []string{"日记_"}
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 查找日记关键字配置行
		if strings.HasPrefix(line, "diary_keywords=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				keywordsStr := strings.TrimSpace(parts[1])
				if keywordsStr != "" {
					keywords := strings.Split(keywordsStr, "|")
					aconfig.diary_keywords = make([]string, 0, len(keywords))
					for _, keyword := range keywords {
						keyword = strings.TrimSpace(keyword)
						if keyword != "" {
							aconfig.diary_keywords = append(aconfig.diary_keywords, keyword)
						}
					}
					log.DebugF("从 sys_conf.md 加载日记关键字: %v", aconfig.diary_keywords)
					return
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.ErrorF("读取 sys_conf.md 文件出错: %v", err)
	}

	// 如果没有找到配置，使用默认值
	if len(aconfig.diary_keywords) == 0 {
		aconfig.diary_keywords = []string{"日记_"}
		log.DebugF("未找到日记关键字配置，使用默认值: %v", aconfig.diary_keywords)
	}
}

// 获取日记关键字列表
func GetDiaryKeywords() []string {
	cmd := &GetDiaryKeywordsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	config_module.Send(cmd)
	keywords := <-cmd.Response()
	return keywords.([]string)
}

// 检查标题是否匹配日记关键字
func IsDiaryBlog(title string) bool {
	cmd := &IsDiaryBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	config_module.Send(cmd)
	ret := <-cmd.Response()
	return ret.(bool)
}

func GetConfig(name string) string {
	cmd := &GetConfigCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Name: name,
	}
	config_module.Send(cmd)
	value := <-cmd.Response()
	return value.(string)
}

func GetHttpTemplatePath() string {
	templates_path := GetConfig("templates_path")
	if templates_path == "" {
		exePath, _ := os.Executable()
		templates_path = filepath.Dir(exePath)
		return filepath.Join(templates_path, "templates")
	} else {
		return templates_path
	}
}

func GetHttpStaticPath() string {
	statics_path := GetConfig("statics_path")
	if statics_path == "" {
		exePath, _ := os.Executable()
		statics_path = filepath.Dir(exePath)
		return filepath.Join(statics_path, "statics")
	} else {
		return statics_path
	}
}

func GetExePath() string {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	return exeDir
}

func GetBlogsPath() string {
	exeDir := GetExePath()
	return filepath.Join(exeDir, "blogs_txt")
}

func readConfigFile(filePath string) (map[string]string, error) {
	config := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		log.DebugF("line =%s", line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		log.DebugF("parse line =%s", line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			config[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return config, nil
}

func IsSysFile(name string) int {
	cmd := &IsSysFileCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Name: name,
	}
	config_module.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func IsPublicTag(tag string) int {
	cmd := &IsPublicTagCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Tag: tag,
	}
	config_module.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func IsTitleAddDateSuffix(title string) int {
	cmd := &IsTitleAddDateSuffixCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	config_module.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func IsTitleContainsDateSuffix(title string) int {
	cmd := &IsTitleContainsDateSuffixCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	config_module.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func GetDownLoadPath() string {
	return GetConfig("download_path")
}

func GetHelpBlogName() string {
	return GetConfig("help_blog_name")
}

func GetMaxBlogComments() int {
	str_cnt := GetConfig("max_blog_comments")
	cnt, _ := strconv.Atoi(str_cnt)
	if cnt <= 0 {
		cnt = 100
	}
	return cnt
}

func GetMainBlogNum() int {
	str_cnt := GetConfig("main_show_blogs")
	cnt, _ := strconv.Atoi(str_cnt)
	if cnt <= 0 {
		cnt = 100
	}
	return cnt
}

func GetRecyclePath() string {
	path := GetConfig("recycle_path")
	if path == "" {
		path = ".go_blog_recycle"
	}
	return path
}
