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
	"sort"
	"strings"
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

func DeleteBlog(title string) int{
	key := fmt.Sprintf("blog@%s",title)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("delete error key=%s err=%s",key,err.Error())
		return  1
	}

	log.DebugF("delete title=%s",key)

	deleteFile(title)

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
	values["encrypt"] = blog.Encrypt
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
	encrypt_s,ok := m["encrypt"]
	if !ok { encrypt_s = "0" }

	mn,_ := strconv.Atoi(mn_s)
	an,_ := strconv.Atoi(an_s)
	auth,_ := strconv.Atoi(auth_s) 	
	encrypt,_ := strconv.Atoi(encrypt_s)
	
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
		Encrypt : encrypt,
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
	if len(m) == 0 {
		return nil
	}
	log.DebugF("getblog success key=%s title=%s",key,m["title"])
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

func deleteFile(title string) int {
	filename := title
	path := config.GetBlogsPath()
	full := filepath.Join(path,filename)
	full = fmt.Sprintf("%s.md",full)

	recycle_path:= config.GetRecyclePath()
	ioutils.Mkdir(recycle_path)
	new_filename := fmt.Sprintf("%s-%s.md",filename,time.Now().Format("2006-01-02"))
	ioutils.Mvfile(full,filepath.Join(recycle_path,new_filename))
	return 0
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

func SaveBlogComments(bc *module.BlogComments){
	log.DebugF("SaveBlogComments title=%s comments_len=%d",bc.Title,len(bc.Comments))	

	key := fmt.Sprintf("comments@%s",bc.Title)
	values := make(map[string]interface{})
	s := "\x01"
	// save new keys
	for _,c := range bc.Comments {
		value := fmt.Sprintf("Idx=%d%sowner=%s%sct=%s%smt=%s%smsg=%s%smail=%s",
				c.Idx,s,
				c.Owner,s,
				c.CreateTime,s,
				c.ModifyTime,s,
				c.Msg,s,
				c.Mail)
		idx_str := fmt.Sprintf("%d",c.Idx)
		values[idx_str] = value
	}
	err := db.client.HMSet(key,values).Err()
	if err != nil {
		log.ErrorF("saveblogcomments error key=%s err=%s",key,err.Error())
	}else{
		log.DebugF("redis saveblogcomments success key=%s title=%s",key,bc.Title)
	}
}

func GetAllBlogComments() map[string]*module.BlogComments {
	// todo 
	keys,err := db.client.Keys("comments@*").Result()
	if err !=nil {
		log.ErrorF("getcomments error keys=comments@* err=%s",err.Error())
		return nil
	}

	bcs := make(map[string]*module.BlogComments)

	for _,key := range keys {
		m ,err := db.client.HGetAll(key).Result()
		if err!=nil {
				log.ErrorF("getComments error key=%s err=%s",key,err.Error())
				continue
		}
		log.DebugF("getComments success key=%s",key)
		title := key[strings.Index(key,"@")+1:]
		toBlogComments(title,m,bcs)
	}

	return bcs
}

func toBlogComments(title string,m map[string]string,bcs map[string]*module.BlogComments){

	bc,ok := bcs[title]
	if !ok {
		bc = &module.BlogComments {
			Title : title,
		}
		bcs[title] = bc
	}

	for _,v := range m {
		owner := ""
		msg  := ""
		ct   := ""
		mt   := ""
		mail := ""
		idx  := -1

		// analy the hash value, split by ASCII 0x01 which is can not print
		tokens := strings.Split(v,"\x01")
		log.DebugF("toBlogComments v=%s tokens_len=%d",v,len(tokens))
		for _,t := range tokens {
			kv := strings.Split(t,"=")
			if len(kv) >= 2 {
				k := kv[0]
				v := t[strings.Index(t,"=")+1:]
				log.DebugF("k=%s v=%s",k,v)

				if strings.ToLower(k) == "owner" {
					owner = v
				}else if strings.ToLower(k) == "msg" {
				msg  = v
				}else if strings.ToLower(k) == "ct"  {
					ct   = v
				}else if strings.ToLower(k) == "mt"  {
					mt   = v
				}else if strings.ToLower(k) == "mail"  {
					mail   = v
				}else if strings.ToLower(k) == "idx" {
					the_idx,err := strconv.Atoi(v)
					if err != nil {
						log.ErrorF("split idx conv to int error %s the_idx=%d",err.Error(),the_idx)
					}else{
						idx = the_idx
					}
				} 

			}else{
				log.ErrorF("split tokens %s error kv <= 2",t)
			}

		}

		if idx < 0 {
			log.ErrorF("toBlogComments idx<0 idx=%d",idx)
			continue
		}

		c := module.Comment {
			Owner: owner,
			Msg : msg,
			CreateTime : ct,
			ModifyTime : mt,
			Mail : mail,
			Idx : idx,
		}
		bc.Comments = append(bc.Comments,&c)
	}


	// sort by c.Idx
	sort.SliceStable(bc.Comments,func(i,j int) bool {
		return bc.Comments[i].Idx < bc.Comments[j].Idx
	})

	showBlogComments(bc)
}



func showBlogComments(cs *module.BlogComments){
	log.DebugF("title=%s",cs.Title)	
	for _,c := range cs.Comments {
		log.DebugF("Idx=%d",c.Idx)	
		log.DebugF("owner=%s",c.Owner)	
		log.DebugF("msg=%s",c.Msg)	
		log.DebugF("ct=%s",c.CreateTime)	
		log.DebugF("mt=%s",c.ModifyTime)	
	}
}
