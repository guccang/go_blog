package blog
import (
	"fmt"
	"module"
	log "mylog"
	db "persistence"
	"ioutils"
	"time"
	"config"
	"sort"
	"strings"
	"regexp"
	"encoding/json"
)

func Info(){
	fmt.Println("info blog v3.0");
}

var Blogs = make(map[string]*module.Blog)

func strTime() string{
	return  time.Now().Format("2006-01-02 15:04:05")
}

func Init(){
	log.Debug("module Init")
	blogs := db.GetBlogs()

	if blogs!=nil{
		for _,b := range blogs{
			if b.Encrypt == 1 {
				b.AuthType = module.EAuthType_encrypt
			}
			Blogs[b.Title] = b
			//log.DebugF("blog title=%s auth=%d",b.Title,b.AuthType)
		}
	}
	log.DebugF("getblogs number=%d",len(blogs))
}

func GetBlogsNum() int {
	return len(Blogs)
}

func ImportBlogsFromPath(dir string){
	files :=  ioutils.GetFiles(dir)
	for _,file := range files{
		name,_:= ioutils.GetBaseAndExt(file)
		datas,size:= ioutils.GetFileDatas(file)
		if size > 0 {
			udb := module.UploadedBlogData{
				Title : name,
				Content : datas,
				AuthType : module.EAuthType_private,
			}
			ret:=AddBlog(&udb)
			if ret==0{
				log.DebugF("name=%s size=%d",name,size)
			}
		}
	}
}

func GetBlog(title string)*module.Blog{
	b,ok := Blogs[title]
	if !ok {
		b = db.GetBlog(title)
		if b == nil {
			return nil
		}
	}
	return b
}

func AddBlog(udb *module.UploadedBlogData) int{
	title := udb.Title
	content := udb.Content
	auth_type := udb.AuthType
	tags := udb.Tags

	add_date_suffix := config.IsTitleAddDateSuffix(title)
	if add_date_suffix == 1 {
		str:=time.Now().Format("2006-01-02")
		title = fmt.Sprintf("%s_%s",title,str)
	}

	_,ok := Blogs[title]
	if ok {
		//log.DebugF("has same name blog=%s",title)
		return 1
	}

	// 检查是否是日记博客，如果是则自动设置为日记权限
	if config.IsDiaryBlog(title) {
		auth_type |= module.EAuthType_diary
		log.DebugF("检测到日记博客，设置日记权限: %s", title)
	}

	log.DebugF("add blog %s",title)
	// add
	now := strTime()
	b := module.Blog{
		Title:title,
		Content:content,
		CreateTime : now,
		ModifyTime : now,
		AccessTime : now,
		ModifyNum  : 0,
		AccessNum  : 0,
		AuthType   : auth_type,
		Tags	   : tags,
		Encrypt	   : udb.Encrypt,
	}
	if b.Encrypt == 1 {
		b.AuthType = module.EAuthType_encrypt
	}
	
	// 日志记录权限设置
	if (auth_type & module.EAuthType_diary) != 0 {
		log.InfoF("博客 '%s' 设置了日记权限，AuthType=%d", title, auth_type)
	}
	if (auth_type & module.EAuthType_cooperation) != 0 {
		log.InfoF("博客 '%s' 设置了协作权限，AuthType=%d", title, auth_type)
	}
	if (auth_type & module.EAuthType_encrypt) != 0 {
		log.InfoF("博客 '%s' 设置了加密权限，AuthType=%d", title, auth_type)
	}
	
	Blogs[title] = &b
	db.SaveBlog(&b)
	return 0

}

func ModifyBlog(udb *module.UploadedBlogData) int {
	title := udb.Title
	content := udb.Content
	auth_type := udb.AuthType
	tags := udb.Tags

	b, ok := Blogs[title]
	if !ok {
		return 1
	}

	log.DebugF("modify blog %s",title)

	// 检查是否是日记博客，如果是则保持日记权限
	if config.IsDiaryBlog(title) {
		auth_type |= module.EAuthType_diary
		log.DebugF("保持日记博客权限: %s", title)
	}

	// modify
	b.Content = content
	b.ModifyTime = strTime()
	b.ModifyNum += 1
	
	// 协作权限的智能处理逻辑
	// 如果新权限中明确包含或排除协作权限，则尊重用户选择
	// 否则保留原有的协作权限设置
	finalAuthType := auth_type
	
	// 检查是否从协作用户发起的请求（这种情况下需要保留协作权限）
	// 注意：这里的逻辑需要配合请求上下文，暂时简化处理
	
	// 如果原博客有协作权限，但新权限中没有明确设置协作权限
	// 我们需要判断是用户主动移除还是意外遗漏
	if (b.AuthType & module.EAuthType_cooperation) != 0 {
		// 如果新权限中明确包含协作权限，保留
		if (auth_type & module.EAuthType_cooperation) != 0 {
			log.DebugF("博客 '%s' 保持协作权限", title)
		} else {
			// 用户明确移除了协作权限
			log.InfoF("博客 '%s' 移除协作权限，原AuthType=%d，新AuthType=%d", title, b.AuthType, auth_type)
		}
	}
	
	b.AuthType = finalAuthType
	b.Tags = tags
	
	// 日志记录权限更新
	if (auth_type & module.EAuthType_diary) != 0 {
		log.InfoF("博客 '%s' 更新了日记权限，AuthType=%d", title, auth_type)
	}
	if (auth_type & module.EAuthType_cooperation) != 0 {
		log.InfoF("博客 '%s' 更新了协作权限，AuthType=%d", title, auth_type)
	}
	if (auth_type & module.EAuthType_encrypt) != 0 {
		log.InfoF("博客 '%s' 更新了加密权限，AuthType=%d", title, auth_type)
	}
	
	db.SaveBlog(b)
	return 0
}

func DeleteBlog(title string) int {
	_, ok := Blogs[title]
	if !ok {
		return 1
	}

	ret := config.IsSysFile(title)
	if ret == 1 {
		return 2
	}

	ret = db.DeleteBlog(title)
	if ret == 1 {
		return 3 
	}

	delete(Blogs,title)

	return 0
}

// 获取最近的timedblog
func GetRecentlyTimedBlog(title string) *module.Blog {
	for i:=1 ; i<9999; i++ {
		// 每次往后推一天
		str:=time.Now().AddDate(0,0,-i).Format("2006-01-02")
		new_title := fmt.Sprintf("%s_%s",title,str)
		log.DebugF("GetRecentlyTimedBlog title=%s",new_title)
		b := GetBlog(new_title)
		if b!= nil{
			return b
		}
	}
	return nil
}

func GetAll(num int,flag int) []*module.Blog {
	s := make([]*module.Blog,0)
	for _,b := range Blogs{
		//log.DebugF("flag=%d b.AuthType=%d",flag,b.AuthType)
		if (flag & b.AuthType) != 0 {
			s = append(s,b)
		}
	}
	sort.Slice(s,func(i,j int) bool {
		ti,_ := time.Parse("2006-01-02 15:04:05",s[i].ModifyTime)
		tj,_ := time.Parse("2006-01-02 15:04:05",s[j].ModifyTime)
		return ti.Unix() > tj.Unix()
	})

	if num > 0 {
		num = num - 1
	}

	if(len(s) > num){
		return s[:num]
	}else {
		return s
	}
}

func UpdateAccessTime(blog *module.Blog){
	blog.AccessTime =  strTime()
	blog.AccessNum += 1
	db.SaveBlog(blog)
}

func GetBlogAuthType(blogname string) int {
	blog := GetBlog(blogname)
	return blog.AuthType
}

func IsPublicTag(tag string) int {
	return config.IsPublicTag(tag)
}

func TagReplace(from string,to string) {
	for _,b := range Blogs {

		if !strings.Contains(strings.ToLower(b.Tags),strings.ToLower(from)) {
			continue
		}

		if from == b.Tags {
			b.Tags = to
		}else{
			newTags := ""
			tags := strings.Split(b.Tags,"|")
			for _,tag := range tags {
				if from == tag {
					// if to == "" delete tag
					if to != "" {
						newTags =  newTags + to + "|"
					}
				}else{
					newTags = newTags + tag + "|"
				}
			}
			// remove last "|"
			newTags = newTags[:len(newTags)-1]
			log.InfoF("blog change tag from %s to %s",b.Tags,newTags)
			b.Tags = newTags
		}

		// remove same tags
		tags := strings.Split(b.Tags,"|")	
		usedTags := make(map[string]bool)
		newTags := ""
		for _,tag := range tags {
			_,ok := usedTags[tag]
			if !ok {
				usedTags[tag] = true
			}else{
				continue
			}
			newTags = newTags + tag + "|"
		}
		newTags = newTags[:len(newTags)-1]
		b.Tags = newTags
		db.SaveBlog(b)
	}	
}

func GetURLBlogNames(blogname string) []string {
	names := make([]string,0)
	
	blog := GetBlog(blogname)
	if blog == nil {
		return names
	}

	linkPattern := regexp.MustCompile(`\[(.*?)\]\(/get\?blogname=(.*?)\)`)
	tokens := strings.Split(blog.Content,"\n")
	for line_no,t := range tokens {
		log.DebugF("line_no=%d %s",line_no,t)
		
		// 匹配并提取博客名称
	    if linkMatches := linkPattern.FindStringSubmatch(t); linkMatches != nil {
			names = append(names,linkMatches[2])
		}
	}

	return names
}

func SetSameAuth(blogname string){
	blog := GetBlog(blogname)
	if blog == nil {
		return
	}

	names := GetURLBlogNames(blogname)

	for _,name := range names {
		b := GetBlog(name)
		if b != nil {
			b.AuthType = blog.AuthType
			db.SaveBlog(b)
		}
	}
}

func AddAuthType(blogname string,flag int){
	blog := GetBlog(blogname)
	if blog == nil {
		return
	}

	blog.AuthType |= flag
	db.SaveBlog(blog)
}

func DelAuthType(blogname string, flag int){
	blog := GetBlog(blogname)
	if blog == nil {
		return
	}

	blog.AuthType &= ^flag
	if blog.AuthType == 0 {
		blog.AuthType = module.EAuthType_private
	}
	db.SaveBlog(blog)
}

// 年度计划相关数据结构
type YearPlanData struct {
	YearOverview string                 `json:"yearOverview"`
	MonthPlans   []string               `json:"monthPlans"`
	Year         int                    `json:"year"`
	Tasks        map[string]interface{} `json:"tasks"` // 存储每月任务列表
}

// 获取年度计划
func GetYearPlan(year int) (*YearPlanData, error) {
	// 构建标题
	planTitle := fmt.Sprintf("年计划_%d", year)
	
	// 尝试获取博客
	blog := GetBlog(planTitle)
	if blog == nil {
		return nil, fmt.Errorf("未找到年份 %d 的计划", year)
	}
	
	// 解析内容
	var planData YearPlanData
	err := json.Unmarshal([]byte(blog.Content), &planData)
	if err != nil {
		return nil, fmt.Errorf("解析计划数据失败: %v", err)
	}
	
	// 检查是否包含任务数据
	log.DebugF("获取年计划 - 年份: %d, 任务数据大小: %d", year, len(planData.Tasks))
	
	// 初始化任务映射（如果为空）
	if planData.Tasks == nil {
		planData.Tasks = make(map[string]interface{})
		log.DebugF("初始化空任务映射")
	}
	
	// 检查原始JSON中是否存在tasks字段
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(blog.Content), &rawData); err == nil {
		if tasks, ok := rawData["tasks"].(map[string]interface{}); ok && len(tasks) > 0 {
			if len(planData.Tasks) == 0 {
				planData.Tasks = tasks
				log.DebugF("从原始JSON中恢复任务数据, 大小: %d", len(tasks))
			}
		}
	}
	
	return &planData, nil
}

// 保存年度计划
func SaveYearPlan(planData *YearPlanData) error {
	// 验证数据
	if planData.Year < 2020 || planData.Year > 2100 {
		return fmt.Errorf("无效的年份: %d", planData.Year)
	}
	
	if len(planData.MonthPlans) != 12 {
		return fmt.Errorf("月度计划数量不正确，应为12个月")
	}
	
	// 记录传入的任务数据
	log.DebugF("保存计划 - 年份: %d, 任务数据大小: %d", planData.Year, len(planData.Tasks))
	for month, tasks := range planData.Tasks {
		if tasksArray, ok := tasks.([]interface{}); ok {
			log.DebugF("月份 %s 的任务数量: %d", month, len(tasksArray))
		}
	}
	
	// 构建标题
	planTitle := fmt.Sprintf("年计划_%d", planData.Year)
	
	// 序列化数据
	content, err := json.Marshal(planData)
	if err != nil {
		return fmt.Errorf("序列化计划数据失败: %v", err)
	}
	
	// 检查是否已存在
	blog := GetBlog(planTitle)
	
	// 上传数据
	udb := module.UploadedBlogData{
		Title:    planTitle,
		Content:  string(content),
		AuthType: module.EAuthType_private, // 默认为私有
		Tags:     "年计划",                 // 添加标签方便查找
	}
	
	var ret int
	if blog == nil {
		// 新建博客
		ret = AddBlog(&udb)
		log.DebugF("新建年计划博客: %s", planTitle)
	} else {
		// 更新博客
		ret = ModifyBlog(&udb)
		log.DebugF("更新年计划博客: %s", planTitle)
	}
	
	if ret != 0 {
		return fmt.Errorf("保存计划失败，错误码: %d", ret)
	}
	
	// 验证保存是否成功
	savedPlan, err := GetYearPlan(planData.Year)
	if err != nil {
		log.ErrorF("无法验证保存的计划: %v", err)
	} else {
		log.DebugF("验证 - 保存后的任务数据大小: %d", len(savedPlan.Tasks))
	}
	
	return nil
}
