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
	// 初始化用户管理器
	InitUserManager()
	
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

// 添加带用户身份验证的评论
func AddCommentWithAuth(title, msg, sessionID, ip, userAgent string) (int, string) {
	// 验证会话
	user, err := UserManager.ValidateSession(sessionID)
	if err != nil {
		return 1, err.Error()
	}
	
	// 检查用户是否可以评论
	canComment, reason := UserManager.CanUserComment(user.UserID)
	if !canComment {
		return 2, reason
	}
	
	// 获取或创建博客评论集合
	bc, ok := Comments[title]
	if !ok {
		bc = &module.BlogComments{Title: title}
		Comments[title] = bc
	}
	
	// 检查评论数量限制
	cur_cnt := len(bc.Comments)
	if cur_cnt > config.GetMaxBlogComments() {
		log.ErrorF("AddCommentWithAuth error comments max limits max=%d", config.GetMaxBlogComments())
		return 3, "评论数量已达上限"
	}
	
	// 创建评论
	comment := module.Comment{
		Owner:       user.Username,
		Msg:         msg,
		CreateTime:  strTime(),
		ModifyTime:  strTime(),
		Idx:         len(bc.Comments),
		Pwd:         "", // 使用用户身份，不需要密码
		Mail:        user.Email,
		UserID:      user.UserID,
		SessionID:   sessionID,
		IP:          ip,
		UserAgent:   userAgent,
		IsAnonymous: false,
		IsVerified:  user.IsVerified,
	}
	
	bc.Comments = append(bc.Comments, &comment)
	db.SaveBlogComments(bc)
	
	// 更新用户评论计数
	UserManager.IncrementUserCommentCount(user.UserID)
	
	log.DebugF("AddCommentWithAuth success: user=%s title=%s", user.Username, title)
	return 0, "评论发表成功"
}

// 创建匿名用户会话并发表评论
func AddAnonymousComment(title, msg, username, email, ip, userAgent string) (int, string) {
	// 创建匿名用户会话
	session, err := UserManager.CreateAnonymousSession(username, email, ip, userAgent)
	if err != nil {
		return 1, err.Error()
	}
	
	// 使用新会话发表评论
	return AddCommentWithAuth(title, msg, session.SessionID, ip, userAgent)
}

// 创建带密码验证的用户会话并发表评论
func AddCommentWithPassword(title, msg, username, email, password, ip, userAgent string) (int, string, string) {
	// 创建或验证用户会话
	session, _, err := UserManager.CreateOrAuthenticateSession(username, email, password, ip, userAgent)
	if err != nil {
		return 1, err.Error(), ""
	}
	
	// 使用会话发表评论
	ret, message := AddCommentWithAuth(title, msg, session.SessionID, ip, userAgent)
	return ret, message, session.SessionID
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
