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

func DeleteBlog(title string) int {
	_, ok := Blogs[title]
	if !ok {
		return 1
	}

	ret := db.DeleteBlog(title)
	if ret == 1 {
		return 2
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

func GetAll(num int) []*module.Blog {
	s := make([]*module.Blog,0)
	for _,b := range Blogs{
	s = append(s,b)
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
