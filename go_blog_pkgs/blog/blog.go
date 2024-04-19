package blog
import (
	"fmt"
	"module"
	log "mylog"
	db "persistence"
	"ioutils"
	"strings"
	"sort"
	"time"
	"config"
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
			Blogs[b.Title] = b
		}
	}
	log.DebugF("getblogs number=%d",len(blogs))
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

	// modify
	b.Content = content
	b.ModifyTime = strTime()
	b.ModifyNum += 1
	b.AuthType = auth_type
	b.Tags = tags
	db.SaveBlog(b)
	return 0
}

func GetAll() []*module.Blog {
	s := make([]*module.Blog,0)
	for _,b := range Blogs{
		s = append(s,b)
	}
	sort.Slice(s,func(i,j int) bool {
		ti,_ := time.Parse("2006-01-02 15:04:05",s[i].ModifyTime)
		tj,_ := time.Parse("2006-01-02 15:04:05",s[j].ModifyTime)
		return ti.Unix() > tj.Unix()
	})
	return s
}

func GetMatch(match string) []*module.Blog{
	// 是否$开头
    // 是的话解析match数据，并根据$private $public 来权限匹配
	auth_type := -1
	if strings.HasPrefix(match,"$public") {
		auth_type = module.EAuthType_public
	}else if strings.HasPrefix(match, "$private") {
		auth_type = module.EAuthType_private
	}

	if auth_type != -1 {
		space_index := strings.Index(match," ")
		if space_index != -1 {
			match = match[space_index+1:]
		}else{
			// 所有的$private/$public
			match = ""
		}
	}

	// tags
	match_tag := ""
	if strings.HasPrefix(match,"$") && len(match)>1  {
		space_index := strings.Index(match," ")
		if space_index != -1 {
			 match_tag = match[1:space_index]
			 match = match[space_index+1:]
		 }else{
			 match_tag = match[1:]
			 match = ""
		}
	}

	s := make([]*module.Blog,0)
	for _,b := range Blogs {

		// auth
		if auth_type != -1 && b.AuthType != auth_type {
			continue
		}

		// tag
		if match_tag!="" && false == strings.Contains(strings.ToLower(b.Tags),strings.ToLower(match_tag)) {
			continue
		}

		// match all
		if match == "" {
			s = append(s,b)
			continue
		}
		
		// title match
		if strings.Contains(strings.ToLower(b.Title),strings.ToLower(match)) {
			s = append(s,b)
			continue
		}	
		// content match
		if strings.Contains(strings.ToLower(b.Content),strings.ToLower(match)){
			s = append(s,b)
			continue
		}
	}
	sort.Slice(s,func(i,j int) bool {
		ti,_ := time.Parse("2006-01-02 15:04:05",s[i].ModifyTime)
		tj,_ := time.Parse("2006-01-02 15:04:05",s[j].ModifyTime)
		//log.DebugF("%d-%s %d-%s ui=%d uj=%d",i,s[i].ModifyTime,j,s[j].ModifyTime,ti.Unix(),tj.Unix())
		return ti.Unix() > tj.Unix()
	})
	return s
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
