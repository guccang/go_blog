package persistence

import(
	"fmt"
	log "mylog"
	"module"
	"strconv"
	"github.com/go-redis/redis"
	"time"
	"config"
	"ioutils"
	"path/filepath"
)

type DbRedis struct{
	client *redis.Client
}

var db =  DbRedis{}

func GetDb() *DbRedis{
	return &db
}

func strTime() string{
	return  time.Now().Format("2006-01-02 15:04:05")
}

func Init(){
	ip := config.GetConfig("redis_ip")
	port,_:= strconv.Atoi(config.GetConfig("redis_port"))
	pwd := config.GetConfig("redis_pwd")
	connect(ip,port,pwd)
}

func connect(ip string, port int,password string) int{
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d",ip,port),
		Password:password,
		DB:0,
	})

	pong,err:=client.Ping().Result()
	if err == nil {
		db.client = client
		log.DebugF("connect redis success ip=%s port=%d password=%s",ip,port,password)
		return 1
	}

	log.DebugF(pong,err)
	return 0
}

func SaveBlog(blog *module.Blog){
	key := fmt.Sprintf("blog@%s",blog.Title)
	values := make(map[string]interface{})
	values["title"] = blog.Title
	values["content"] = blog.Content
	values["ct"] = blog.CreateTime
	values["mt"] = blog.ModifyTime
	values["at"] = blog.AccessTime
	values["modifynum"] = blog.ModifyNum
	values["accessnum"] = blog.AccessNum
	values["authtype"] = blog.AuthType
	values["tags"] = blog.Tags
	err := db.client.HMSet(key,values).Err()
	if err != nil {
		log.ErrorF("saveblog error key=%s err=%s",key,err.Error())
	}
	log.DebugF("redis saveblog success key=%s mt=%s",key,blog.ModifyTime)

	saveToFile(blog)
}

func SaveBlogs(blogs map[string]*module.Blog){
	for _,b := range blogs {
		SaveBlog(b)
	}
}

func toBlog(m map[string]string)*module.Blog{
	now :=  strTime()
	ct,ok := m["ct"]
	if !ok { ct = now }
	mt,ok := m["mt"]
	if !ok { mt = now }
	at,ok := m["at"]
	if !ok { at = now }
	mn_s,ok := m["modifynum"]
	if !ok { mn_s = "0"}
	an_s,ok := m["accessnum"]
	if !ok { an_s = "0"}
	auth_s,ok := m["authtype"]
	if !ok { auth_s = "0" }
	tags,ok := m["tags"]
	if !ok { tags = "" }

	mn,_ := strconv.Atoi(mn_s)
	an,_ := strconv.Atoi(an_s)
	auth,_ := strconv.Atoi(auth_s) 	
	
	b := module.Blog{
		Title:m["title"],
		Content:m["content"],
		CreateTime:ct,
		ModifyTime:mt,
		AccessTime:at,
		ModifyNum:mn,
		AccessNum:an,
		AuthType:auth,
		Tags : tags,
	}
	return &b
}

func GetBlog(name string)*module.Blog{
	key := fmt.Sprintf("blog@%s",name)
	m ,err := db.client.HGetAll(key).Result()
	if err !=nil {
		log.ErrorF("getblog error key=%s err=%s",key,err.Error())
		return nil
	}
	log.DebugF("getblog success key=%s",key)
	b := toBlog(m)
	return b
}

func GetBlogs()map[string]*module.Blog{
    keys,err := db.client.Keys("blog@*").Result()
	if err !=nil {
		log.ErrorF("getblogs error keys=blog@* err=%s",err.Error())
		return nil
	}

	blogs := make(map[string]*module.Blog)

	for _,key := range keys {
		m ,err := db.client.HGetAll(key).Result()
		if err!=nil {
				log.ErrorF("getblog error key=%s err=%s",key,err.Error())
				continue
		}
		log.DebugF("getblog success key=%s",key)
		b := toBlog(m)
		blogs[b.Title] = b
		showBlog(b)
	}

	return blogs
}

func showBlog(b *module.Blog){
	log.DebugF("title=%s",b.Title)	
	log.DebugF("ct=%s",b.CreateTime)	
	log.DebugF("mt=%s",b.ModifyTime)	
	log.DebugF("at=%s",b.AccessTime)	
	log.DebugF("mn=%d",b.ModifyNum)	
	log.DebugF("an=%d",b.AccessNum)	
}

func saveToFile(blog *module.Blog){
	filename := blog.Title
	content := blog.Content

	path := config.GetBlogsPath()
	full := filepath.Join(path,filename)
	full = fmt.Sprintf("%s.md",full)

	fcontent,_ := ioutils.GetFileDatas(full)
	if(content == fcontent){
		log.DebugF("saveToFile Cancle content is same %s",full);
		return
	}
	ioutils.RmAndSaveFile(full,content)

}
