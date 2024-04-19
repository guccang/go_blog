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

func Info(){
	fmt.Println("info config v1.0")
}

var datas = make(map[string]string)
var autodatesuffix = make([]string,0)
var publictags = make([]string,0)
var config_path = ""

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


func GetVersion() string{
	return "Version6.0"
}


func IsPublicTag(tag string) int {
	for _,v := range publictags{
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

func GetRecyclePath() string{
	path := GetConfig("recycle_path")
	if path == "" {
		path ="~/.go_blog_recycle"
	}
	return path
}

