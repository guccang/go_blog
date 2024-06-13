package share
import (
	log "mylog"
	"github.com/google/uuid"
	"fmt"
	"time"
	"strconv"
	"config"
)

func Info(){
	log.MessageF("info share v8.0")
}


type SharedBlog struct {
	Pwd		string
	Title	string
	Count	int
	URL		string
	Timeout int64
}

type SharedTag struct {
    Pwd		string
	Tag		string
	Count	int
	URL		string
	Timeout int64
}

var SharedBlogs = make(map[string]*SharedBlog)
var SharedTags = make(map[string]*SharedTag)

func GetSharedBlog(title string)*SharedBlog{
	b, ok := SharedBlogs[title]
	if !ok {
		return nil
	}
	return b
}

func GetSharedTag(title string)*SharedTag{
	b, ok := SharedTags[title]
	if !ok {
		return nil
	}
	return b
}

// 7天超时时间戳
func Get7DaysTimeOutStamp()int64{
	utcTimestamp := time.Now().UTC().Unix()
	share_days,err := strconv.Atoi(config.GetConfig("share_days"))
	if err != nil {
		share_days = 7
	}
	return utcTimestamp + (int64(share_days) * 24 * 3600)
}

func AddSharedBlog(title string) (string,string){
	b, ok := SharedBlogs[title]
	if !ok {
		pwd := uuid.New().String()
		url := fmt.Sprintf("/getshare?t=0&name=%s&pwd=%s",title,pwd)
		SharedBlogs[title] = &SharedBlog{
			Title : title,
			Count : 9999,
			Pwd : pwd,
			URL : url,
			Timeout : Get7DaysTimeOutStamp(),
		}
		return url,pwd
	}

	b.Count = b.Count + 1
	return b.URL,b.Pwd
}

func AddSharedTag(tag string) (string,string){
	t, ok := SharedTags[tag]
	if !ok {
		pwd := uuid.New().String()
		url := fmt.Sprintf("/getshare?t=1&name=%s&pwd=%s",tag,pwd)
		SharedTags[tag] = &SharedTag{
			Tag : tag,
			Count : 9999,
			Pwd : pwd,
			URL : url,
			Timeout : Get7DaysTimeOutStamp(),
		}
		return url,pwd
	}
	t.Count = t.Count + 1
	return t.URL,t.Pwd
}

func ModifyCntSharedBlog(title string,c int) int{
	b, ok := SharedBlogs[title]	
	if !ok {
		return -1
	}

	b.Count = b.Count + c
	if b.Count < 0 {
		delete(SharedBlogs,title)
		return -2
	}	

	utcTimestamp := time.Now().UTC().Unix()
	if b.Timeout < utcTimestamp {
		delete(SharedBlogs,title)
		return -3
	}

	return b.Count
}

func ModifyCntSharedTag(tag string,c int) int{
	b, ok := SharedBlogs[tag]	
	if !ok {
		return -1
	}

	b.Count = b.Count + c
	if b.Count < 0 {
		delete(SharedTags,tag)
		return -2
	}	

	utcTimestamp := time.Now().UTC().Unix()
	if b.Timeout < utcTimestamp {
		delete(SharedTags,tag)
		return -3
	}

	return b.Count
}
