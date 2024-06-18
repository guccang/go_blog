package cooperation
import (
	"github.com/google/uuid"
	log "mylog"
	"module"
	db "persistence"
	"auth"
	"time"
	"strings"
	"fmt"
	"blog"
)

func Info() {
	log.Debug("info coorperation v1.0")
}


var Cooperations = make(map[string]*module.Cooperation)
var sessions = make(map[string]string)

func genPwd() string{
	return uuid.New().String()
}

// load data from redis
// check from cfg
func Init(){
	log.Debug("coorperation Init")

	cs := db.GetCooperations()
	if cs != nil {
		for _,c := range cs {
			Cooperations[c.Account] = c
			log.DebugF("cooperation account=%s pwd=%s",c.Account,c.Password)
		}
	}
}


// Is cooperation login
func CooperationLogin(account string, pwd string) (string,int) {
	c,ok := Cooperations[account]
	if !ok {
		return "",1
	}

	if c.Password != pwd {
		return "",2
	}

	s := auth.AddSession(account)
	sessions[s] = account

	return s,0
}

func IsCooperation(session string) bool{
	_,ok := sessions[session]
	return ok
}

func CreateCooperation(account string) (int,*module.Cooperation){
	c,ok := Cooperations[account]
	if ok {
		log.DebugF("create cooperation ret=1 account=%s pwd=%s",c.Account,c.Password)
		return 1,c
	}

	pwd := genPwd()
	ret := db.SaveCooperation(account,pwd,"","")
	if ret == 1 {
		log.Debug("create cooperation ret=2")
		return 2,nil
	}

	c = &module.Cooperation {
		Account : account,
		Password :pwd,
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		Blogs : "help",
	}
	Cooperations[account]  = c

	log.DebugF("CreateCooperation account=%s pwd=%s ret=%d",account,pwd, ret)

	return 0,c
}

func DelCooperation(account string) int{
	delete(Cooperations,account)
	ret := db.DelCooperation(account)
	log.DebugF("DelCooperation account=%s ret=%d",account,ret)
	return ret
}


func AddCanEditBlogBySession(session string,blogname string) int {
	account,ok := sessions[session]	
	if !ok {
		return 1
	}

	return AddCanEditBlog(account,blogname)
}

func AddCanEditBlog(account string,blogname string) int {
	c,ok := Cooperations[account]
	if !ok {
		return 1
	}


	c.Blogs = fmt.Sprintf("%s %s",c.Blogs,blogname)

	links_blog := blog.GetURLBlogNames(blogname) 
	for _, name := range links_blog {
		log.DebugF("URL blogname=%s",name)
		c.Blogs = fmt.Sprintf("%s %s",c.Blogs,name)
	}
	blog.SetSameAuth(blogname)
	blog.AddAuthType(blogname,module.EAuthType_cooperation)

	ret := db.SaveCooperation(account,c.Password,c.Blogs,c.Tags)
	if ret == 1 {
		log.Debug("AddCanEditBlog cooperation ret=2")
		return 2
	}

	return 0
}

func AddCanEditTag(account string,tag string) int{
	c,ok := Cooperations[account]
	if !ok {
		return 1
	}

	c.Tags = fmt.Sprintf("%s %s",c.Tags,tag)

	ret := db.SaveCooperation(account,c.Password,c.Blogs,c.Tags)
	if ret == 1 {
		log.Debug("AddCanEditTag cooperation ret=2")
		return 2
	}

	return 0
}

func CanEditBlog(session string,blogname string) int {
	account := sessions[session]
	c,ok := Cooperations[account]
	if !ok {
		return 1
	}

	tokens := strings.Split(strings.TrimSpace(c.Blogs)," ")

	for _,b := range tokens {
		if b == blogname {
			return 0
		}
	}

	return 2
}

func CanEditTag(session string,tag string) int {
	account := sessions[session]
	c,ok := Cooperations[account]
	if !ok {
		return 1
	}

	tokens := strings.Split(strings.TrimSpace(c.Tags)," ")

	for _,t := range tokens {
		if t == tag {
			return 0
		}
	}

	return 2
}


func DelCanEditBlog(account string,blogname string) int {
	c,ok := Cooperations[account]
	if !ok {
		return 1
	}

	c.Blogs = strings.ReplaceAll(c.Blogs,blogname,"")

	ret := db.SaveCooperation(account,c.Password,c.Blogs,c.Tags)
	if ret == 1 {
		log.Debug("AddCanEditBlog cooperation ret=2")
		return 2
	}
	blog.DelAuthType(blogname,module.EAuthType_cooperation)

	return 0
}

func DelCanEditTag(account string,tag string) int{
	c,ok := Cooperations[account]
	if !ok {
		return 1
	}

	c.Tags = strings.ReplaceAll(c.Tags,tag,"")

	ret := db.SaveCooperation(account,c.Password,c.Blogs,c.Tags)
	if ret == 1 {
		log.Debug("AddCanEditTag cooperation ret=2")
		return 2
	}

	return 0
}


