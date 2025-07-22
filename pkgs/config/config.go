package config
import (
	"fmt"
	"os"
	"path/filepath"
	"bufio"
	"strings"
	log "mylog"
	"strconv"
)

var blog_version = "Version13.0"

func Info(){
	fmt.Println("info config v13.0")
}

func GetVersion() string{
	return  blog_version
}

var datas = make(map[string]string)
var autodatesuffix = make([]string,0)
var publictags = make([]string,0)
var diary_keywords = make([]string,0)  // 新增：日记关键字列表
var config_path = ""
var sys_files = make([]string,0)

func Init(filePath string){
	config_path = filePath
	loadConfig(config_path)
}

func GetConfigPath() string{
	return config_path
}

func ReloadConfig(filePath string){
	loadConfig(filePath)
}

func loadConfig(filePath string){
	datas,_= readConfigFile(filePath)	
	for k,v := range datas{
		log.DebugF("CONFIG %s=%s",k,v)
	}

	datetitles,ok := datas["title_auto_add_date_suffix"]
	if ok {
		arr := strings.Split(datetitles,"|")
		autodatesuffix = arr
	}

	tags,ok := datas["publictags"] 
	if ok {
		arr := strings.Split(tags,"|")
		publictags = arr
	}

	sysfiles,ok := datas["sysfiles"]
	if ok {
		arr := strings.Split(sysfiles,"|")
		sys_files = arr
	}
	
	// 从 sys_conf.md 文件中读取日记关键字配置
	loadDiaryKeywordsFromSysConf()
}

// 从 sys_conf.md 文件中读取日记关键字配置
func loadDiaryKeywordsFromSysConf() {
	// 获取 blogs_txt 目录路径
	blogsPath := GetBlogsPath()
	sysConfPath := filepath.Join(blogsPath, "sys_conf.md")
	
	// 检查文件是否存在
	if _, err := os.Stat(sysConfPath); os.IsNotExist(err) {
		log.DebugF("sys_conf.md 文件不存在: %s", sysConfPath)
		// 设置默认的日记关键字
		diary_keywords = []string{"日记_"}
		return
	}
	
	// 读取文件内容
	file, err := os.Open(sysConfPath)
	if err != nil {
		log.ErrorF("无法打开 sys_conf.md 文件: %v", err)
		diary_keywords = []string{"日记_"}
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
					diary_keywords = make([]string, 0, len(keywords))
					for _, keyword := range keywords {
						keyword = strings.TrimSpace(keyword)
						if keyword != "" {
							diary_keywords = append(diary_keywords, keyword)
						}
					}
					log.DebugF("从 sys_conf.md 加载日记关键字: %v", diary_keywords)
					return
				}
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		log.ErrorF("读取 sys_conf.md 文件出错: %v", err)
	}
	
	// 如果没有找到配置，使用默认值
	if len(diary_keywords) == 0 {
		diary_keywords = []string{"日记_"}
		log.DebugF("未找到日记关键字配置，使用默认值: %v", diary_keywords)
	}
}

// 获取日记关键字列表
func GetDiaryKeywords() []string {
	return diary_keywords
}

// 检查标题是否匹配日记关键字
func IsDiaryBlog(title string) bool {
	for _, keyword := range diary_keywords {
		if strings.HasPrefix(title, keyword) {
			return true
		}
	}
	return false
}

func GetConfig(name string)(string){
	v,ok := datas[name]
	if !ok {
		return ""
	}else{
		return v
	}
}

func GetHttpTemplatePath() string{
	templates_path := GetConfig("templates_path")
    if templates_path == ""{
		exePath,_:= os.Executable()
		templates_path = filepath.Dir(exePath)
		return filepath.Join(templates_path,"templates")
	}else{
		return templates_path
	}
}

func GetHttpStaticPath() string{
	statics_path := GetConfig("statics_path")
    if statics_path == "" {
		exePath,_:= os.Executable()
		statics_path = filepath.Dir(exePath)
		return filepath.Join(statics_path,"statics")
	}else{
		return statics_path
	}
}

func GetExePath() string{
	exePath,_:=os.Executable()
	exeDir := filepath.Dir(exePath)
	return exeDir
}

func GetBlogsPath() string{
	exeDir := GetExePath()
	return filepath.Join(exeDir,"blogs_txt")
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
		log.DebugF("line =%s",line)
		if strings.HasPrefix(line,"#"){
			continue;
		}
		log.DebugF("parse line =%s",line)
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
	for _,v := range sys_files{
		if v == name {
			return 1
		}
	}
	return 0
}

func IsPublicTag(tag string) int {
	for _,v := range publictags{
		log.DebugF("IsPublicTag %s %s",v,tag)
		if v == tag {
			return 1
		}
	}
	return 0
}

func IsTitleAddDateSuffix(title string)int{
	for _,v := range autodatesuffix {
		if v == title {
			return 1
		}
	}
	return 0
}

func IsTitleContainsDateSuffix(title string)int{
	for _,v := range autodatesuffix {
		if strings.Contains(strings.ToLower(title),strings.ToLower(v)) {
			return 1
		}
	}
	return 0
}

func GetDownLoadPath()string{
	return GetConfig("download_path")
}
	
func GetHelpBlogName() string{
	return GetConfig("help_blog_name")
}

func GetMaxBlogComments() int {
	str_cnt := GetConfig("max_blog_comments")
	cnt,_:= strconv.Atoi(str_cnt)
	if cnt <= 0 {
		cnt = 100
	}
	return cnt
}

func GetMainBlogNum() int {
	str_cnt := GetConfig("main_show_blogs")
	cnt,_:=strconv.Atoi(str_cnt)
	if cnt <= 0{
		cnt = 100
	}
	return cnt
}

func GetRecyclePath() string{
	path := GetConfig("recycle_path")
	if path == "" {
		path =".go_blog_recycle"
	}
	return path
}

