package config

import (
	"bufio"
	"core"
	"fmt"
	log "mylog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var adminAccount string

func Info() {
	log.Debug(log.ModuleConfig, "info config v13.0")
}

// 初始化config模块
func Init(filePath string) {
	InitManager(filePath)
}

// interface

func GetVersionWithAccount(account string) string {
	actor := getConfigActor(account)
	cmd := &GetVersionCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	actor.Send(cmd)
	version := <-cmd.Response()
	return version.(string)
}

func GetConfigPathWithAccount(account string) string {
	actor := getConfigActor(account)
	cmd := &GetConfigPathCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	actor.Send(cmd)
	path := <-cmd.Response()
	return path.(string)
}

func ReloadConfig(account, filePath string) {
	ReloadConfigWithAccount(account, filePath)
}

func ReloadConfigWithAccount(account, filePath string) {
	actor := getConfigActor(account)
	cmd := &ReloadConfigCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Account:  account,
		FilePath: filePath,
	}
	actor.Send(cmd)
	<-cmd.Response()
}

// 内部方法：加载配置
func (aconfig *ConfigActor) loadConfigInternal(account string, filePath string) error {
	log.DebugF(log.ModuleConfig, "loadConfigInternal account=%s filePath=%s", account, filePath)
	datas, err := readConfigFile(filePath)
	if err != nil {
		return err
	}
	aconfig.datas = datas
	aconfig.config_path = filePath

	for k, v := range aconfig.datas {
		log.DebugF(log.ModuleConfig, "CONFIG %s=%s", k, v)
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
	if account == "" {
		account = aconfig.getConfig("admin")
		aconfig.Account = account
	}
	aconfig.loadDiaryKeywordsFromSysConf(account)
	return nil
}

// 从 sys_conf.md 文件中读取日记关键字配置
func (aconfig *ConfigActor) loadDiaryKeywordsFromSysConf(account string) {
	log.DebugF(log.ModuleConfig, "loadDiaryKeywordsFromSysConf account=%s", account)
	// 获取 blogs_txt 目录路径
	sysConfPath := GetSysConfigPath(account)

	// 检查文件是否存在
	if _, err := os.Stat(sysConfPath); os.IsNotExist(err) {
		log.DebugF(log.ModuleConfig, "sys_conf.md 文件不存在: %s", sysConfPath)
		// 设置默认的日记关键字
		aconfig.diary_keywords = []string{"日记_"}
		return
	}

	// 读取文件内容
	file, err := os.Open(sysConfPath)
	if err != nil {
		log.ErrorF(log.ModuleConfig, "无法打开 sys_conf.md 文件: %v", err)
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
					log.DebugF(log.ModuleConfig, "从 sys_conf.md 加载日记关键字: %v", aconfig.diary_keywords)
					return
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.ErrorF(log.ModuleConfig, "读取 sys_conf.md 文件出错: %v", err)
	}

	// 如果没有找到配置，使用默认值
	if len(aconfig.diary_keywords) == 0 {
		aconfig.diary_keywords = []string{"日记_"}
		log.DebugF(log.ModuleConfig, "未找到日记关键字配置，使用默认值: %v", aconfig.diary_keywords)
	}
}

func GetDiaryKeywordsWithAccount(account string) []string {
	actor := getConfigActor(account)
	cmd := &GetDiaryKeywordsCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
	}
	actor.Send(cmd)
	keywords := <-cmd.Response()
	return keywords.([]string)
}

func IsDiaryBlogWithAccount(account, title string) bool {
	actor := getConfigActor(account)
	cmd := &IsDiaryBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(bool)
}

func GetConfigWithAccount(account, name string) string {
	actor := getConfigActor(account)
	cmd := &GetConfigCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Name: name,
	}
	actor.Send(cmd)
	value := <-cmd.Response()
	return value.(string)
}

func GetHttpTemplatePath() string {
	templates_path := GetConfigWithAccount(adminAccount, "templates_path")
	if templates_path == "" {
		exePath, _ := os.Executable()
		templates_path = filepath.Dir(exePath)
		return filepath.Join(templates_path, "templates")
	} else {
		return templates_path
	}
}

func GetHttpStaticPath() string {
	statics_path := GetConfigWithAccount(adminAccount, "statics_path")
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

func GetBlogsPath(account string) string {
	exeDir := GetExePath()
	return filepath.Join(exeDir, "blogs_txt", account)
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
		if strings.HasPrefix(line, "#") {
			continue
		}
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
	actor := getConfigActor(adminAccount)
	cmd := &IsSysFileCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Name: name,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func IsPublicTag(tag string) int {
	return IsPublicTagWithAccount("", tag)
}

func IsPublicTagWithAccount(account, tag string) int {
	actor := getConfigActor(account)
	cmd := &IsPublicTagCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Tag: tag,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func IsTitleAddDateSuffix(title string) int {
	return IsTitleAddDateSuffixWithAccount("", title)
}

func IsTitleAddDateSuffixWithAccount(account, title string) int {
	actor := getConfigActor(account)
	cmd := &IsTitleAddDateSuffixCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func IsTitleContainsDateSuffix(title string) int {
	return IsTitleContainsDateSuffixWithAccount("", title)
}

func IsTitleContainsDateSuffixWithAccount(account, title string) int {
	actor := getConfigActor(account)
	cmd := &IsTitleContainsDateSuffixCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		Title: title,
	}
	actor.Send(cmd)
	ret := <-cmd.Response()
	return ret.(int)
}

func GetDownLoadPath() string {
	return GetConfigWithAccount(adminAccount, "download_path")
}

func GetHelpBlogName() string {
	return GetConfigWithAccount(adminAccount, "help_blog_name")
}

func GetMaxBlogComments() int {
	str_cnt := GetConfigWithAccount(adminAccount, "max_blog_comments")
	cnt, _ := strconv.Atoi(str_cnt)
	if cnt <= 0 {
		cnt = 100
	}
	return cnt
}

func GetMainBlogNum() int {
	str_cnt := GetConfigWithAccount(adminAccount, "main_show_blogs")
	cnt, _ := strconv.Atoi(str_cnt)
	if cnt <= 0 {
		cnt = 100
	}
	return cnt
}

func GetRecyclePath() string {
	path := GetConfigWithAccount(adminAccount, "recycle_path")
	if path == "" {
		path = ".go_blog_recycle"
	}
	return path
}

// UpdateConfigFromBlog updates account-specific configuration from blog content
func UpdateConfigFromBlog(account, blogContent string) {
	actor := getConfigActor(account)
	cmd := &UpdateConfigFromBlogCmd{
		ActorCommand: core.ActorCommand{
			Res: make(chan interface{}),
		},
		BlogContent: blogContent,
	}
	actor.Send(cmd)
	<-cmd.Response()
}

func GetAdminAccount() string {
	return adminAccount
}

func GetSysConfigPath(account string) string {
	return filepath.Join(GetBlogsPath(account), GetSysConfigFullName())
}

func GetSysConfigTitle() string {
	return "sys_conf"
}

func GetSysConfigFullName() string {
	return GetSysConfigTitle() + ".md"
}

func GetSysConfigTitleMCP() string {
	return "mcp_config"
}

func GetSysConfigs() string {
	return fmt.Sprintf("%s | %s | %s", GetSysConfigTitle(), GetSysConfigTitleMCP(), "sys_accounts")
}
