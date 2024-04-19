package comment
import (
	"module"
	db "persistence"
	log "mylog"
	"time"
	"fmt"
	"config"
)

func Info() {
	fmt.Println("info comment v3.0")
}

var Comments = make(map[string]*module.BlogComments)

func strTime() string{
	return  time.Now().Format("2006-01-02 15:04:05")
}

// 加载评论数据
func Init() {
	all_datas := db.GetAllBlogComments()
	if all_datas != nil {
		for _,c := range all_datas {
			Comments[c.Title] = c
		}
	}
	log.DebugF("getComments number=%d",len(Comments))
}


// 添加
func AddComment(title string,msg string,owner string,pwd string,mail string) int {
	bc,ok := Comments[title]
	if !ok {
		bc = &module.BlogComments {
			Title : title,
		}
		Comments[title] = bc
	}

	cur_cnt := len(bc.Comments)
	if cur_cnt > config.GetMaxBlogComments() {
		log.ErrorF("AddComment error comments max limits  max=%d",config.GetMaxBlogComments())
		return 0
	}

	c := module.Comment{
		Owner: owner,
		Msg : msg,
		CreateTime : strTime(),
		ModifyTime : strTime(),
		Idx : len(bc.Comments),
		Pwd : pwd,
		Mail : mail,
	}
	bc.Comments = append(bc.Comments,&c)
	db.SaveBlogComments(bc)
	return 0
}

// 修改
func ModifyComment(title string,msg string, idx int) int {
	bc,ok := Comments[title]
	if !ok {
		log.ErrorF("ModifyComment %s not find",title)
		return 1
	}
	if idx >= len(bc.Comments) {
		log.ErrorF("ModifyComment %s id=%d > len of comments %d",title,idx,len(bc.Comments))
		return 2
	}
	c := bc.Comments[idx]
	c.Msg = msg
	db.SaveBlogComments(bc)
	return 0
}

// 移除
func RemoveComment(title string,idx int) int {
	bc,ok := Comments[title]
	if !ok {
		log.ErrorF("RemoveComment %s not find",title)
		return 1
	}
	if idx >= len(bc.Comments) {
		log.ErrorF("RemoveComment %s id=%d > len of comments %d",title,idx,len(bc.Comments))
		return 2
	}

	sub_comments := bc.Comments[:0]
	cnt := 0
	for i ,v := range bc.Comments {
		if i != idx {
			sub_comments = append(sub_comments,v)
			v.Idx = cnt
			cnt = cnt + 1
		} 
	}

	bc.Comments = sub_comments

	return 0
}

// 获取评论数据
func GetComments(title string) *module.BlogComments{
	c,ok := Comments[title]
	if !ok {
		return nil
	}
	return c
}
