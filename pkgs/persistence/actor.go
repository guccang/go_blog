package persistence

import (
	"core"
	"fmt"
	"module"
	log "mylog"
	"strconv"
	"time"

	"config"
	"ioutils"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-redis/redis"
)

/*
 goroutine 线程安全
  goroutine 会被调度到任意一个线程上，因此会被任意一个线程执行接口
  线程安全原因
  原因1: 	actor使用chan通信，chan是线程安全的
  原因2: 	actor的mailbox是线程安全的

  添加一个功能需要的四个步骤:
   第一步: 实现功能逻辑
   第二步: 实现对应的cmd
   第三步: 在persistence.go中添加对应的接口
   第四步: 在http中添加对应的接口
*/

// actor
type PersistenceActor struct {
	*core.Actor
	client *redis.Client
}

func (p *PersistenceActor) strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func (p *PersistenceActor) connect(ip string, port int, password string) int {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", ip, port),
		Password: password,
		DB:       0,
	})

	pong, err := client.Ping().Result()
	if err == nil {
		p.client = client
		log.DebugF("connect redis success ip=%s port=%d password=%s", ip, port, password)
		return 1
	}

	log.DebugF(pong, err)
	return 0
}

func (p *PersistenceActor) deleteBlog(account, title string) int {
	// delete redis keys for all accounts and legacy key
	pattern := fmt.Sprintf("*:blog@%s", title)
	keys, _ := p.client.Keys(pattern).Result()
	legacyKey := fmt.Sprintf("blog@%s", title)
	keys = append(keys, legacyKey)
	for _, k := range keys {
		if err := p.client.Del(k).Err(); err != nil {
			log.ErrorF("delete error key=%s err=%s", k, err.Error())
		} else {
			log.DebugF("delete key=%s", k)
		}
	}
	p.deleteFile(account, title)
	return 0
}

func (p *PersistenceActor) saveBlog(account string, blog *module.Blog) {

	key := fmt.Sprintf("%s:blog@%s", account, blog.Title)
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
	values["account"] = account
	err := p.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("saveblog error key=%s err=%s", key, err.Error())
	}
	log.DebugF("redis saveblog success key=%s mt=%s", key, blog.ModifyTime)

	p.saveToFile(account, blog)
}

func (p *PersistenceActor) saveBlogs(account string, blogs map[string]*module.Blog) {
	for _, b := range blogs {
		p.saveBlog(account, b)
	}
}

func (p *PersistenceActor) toBlog(m map[string]string) *module.Blog {
	now := p.strTime()
	ct, ok := m["ct"]
	if !ok {
		ct = now
	}
	mt, ok := m["mt"]
	if !ok {
		mt = now
	}
	at, ok := m["at"]
	if !ok {
		at = now
	}
	mn_s, ok := m["modifynum"]
	if !ok {
		mn_s = "0"
	}
	an_s, ok := m["accessnum"]
	if !ok {
		an_s = "0"
	}
	auth_s, ok := m["authtype"]
	if !ok {
		auth_s = "0"
	}
	tags, ok := m["tags"]
	if !ok {
		tags = ""
	}
	encrypt_s, ok := m["encrypt"]
	if !ok {
		encrypt_s = "0"
	}
	account, _ := m["account"]

	mn, _ := strconv.Atoi(mn_s)
	an, _ := strconv.Atoi(an_s)
	auth, _ := strconv.Atoi(auth_s)
	encrypt, _ := strconv.Atoi(encrypt_s)

	b := module.Blog{
		Title:      m["title"],
		Content:    m["content"],
		CreateTime: ct,
		ModifyTime: mt,
		AccessTime: at,
		ModifyNum:  mn,
		AccessNum:  an,
		AuthType:   auth,
		Tags:       tags,
		Encrypt:    encrypt,
		Account:    account,
	}
	return &b
}

func (p *PersistenceActor) showBlog(b *module.Blog) {
	log.DebugF("title=%s", b.Title)
	log.DebugF("ct=%s", b.CreateTime)
	log.DebugF("mt=%s", b.ModifyTime)
	log.DebugF("at=%s", b.AccessTime)
	log.DebugF("mn=%d", b.ModifyNum)
	log.DebugF("an=%d", b.AccessNum)
}

func (p *PersistenceActor) getBlogsByAccount(account string) map[string]*module.Blog {
	log.DebugF("getBlogsByAccount account=%s", account)
	pattern := fmt.Sprintf("%s:blog@*", account)
	keys, err := p.client.Keys(pattern).Result()
	if err != nil {
		log.ErrorF("getblogsbyaccount error keys=%s err=%s", pattern, err.Error())
		return nil
	}

	if account == config.GetAdminAccount() {
		legacy, _ := p.client.Keys("blog@*").Result()
		keys = append(keys, legacy...)
		log.DebugF("getBlogsByAccount admin account=%s keys_len=%d", account, len(keys))
	} else {
		log.DebugF("getBlogsByAccount account=%s keys_len=%d", account, len(keys))
	}

	blogs := make(map[string]*module.Blog)

	for _, key := range keys {
		m, err := p.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("getblogbyaccount error key=%s err=%s", key, err.Error())
			continue
		}
		b := p.toBlog(m)
		blogs[b.Title] = b
	}

	log.DebugF("getBlogsByAccount blogs_len=%d", len(blogs))
	return blogs
}

func (p *PersistenceActor) deleteFile(account string, title string) int {
	filename := title
	path := config.GetBlogsPath(account)

	ioutils.Mkdir(path)
	full := filepath.Join(path, filename)
	full = fmt.Sprintf("%s.md", full)

	recycle_path := config.GetRecyclePath()
	ioutils.Mkdir(recycle_path)
	new_filename := fmt.Sprintf("%s-%s.md", filename, time.Now().Format("2006-01-02"))
	ioutils.Mvfile(full, filepath.Join(recycle_path, new_filename))
	return 0
}

func (p *PersistenceActor) saveToFile(account string, blog *module.Blog) {
	filename := blog.Title
	content := blog.Content

	if blog.Account == "" {
		blog.Account = account
	}

	path := config.GetBlogsPath(account)

	ioutils.Mkdir(path)
	full := filepath.Join(path, filename)
	full = fmt.Sprintf("%s.md", full)
	log.DebugF("saveToFile full=%s", full)

	fcontent, _ := ioutils.GetFileDatas(full)
	if content == fcontent {
		log.DebugF("saveToFile Cancle content is same %s", full)
		return
	}
	ioutils.RmAndSaveFile(full, content)
}

func (p *PersistenceActor) saveBlogComments(account string, bc *module.BlogComments) {
	log.DebugF("SaveBlogComments title=%s comments_len=%d", bc.Title, len(bc.Comments))

	key := fmt.Sprintf("comments@%s", bc.Title)
	values := make(map[string]interface{})
	s := "\x01"
	// save new keys
	for _, c := range bc.Comments {
		value := fmt.Sprintf("Idx=%d%sowner=%s%sct=%s%smt=%s%smsg=%s%smail=%s%sPwd=%s",
			c.Idx, s,
			c.Owner, s,
			c.CreateTime, s,
			c.ModifyTime, s,
			c.Msg, s,
			c.Mail, s,
			s,
			c.Pwd)
		idx_str := fmt.Sprintf("%d", c.Idx)
		values[idx_str] = value
	}
	err := p.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("saveblogcomments error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("redis saveblogcomments success key=%s title=%s", key, bc.Title)
	}
}

func (p *PersistenceActor) getAllBlogComments(account string) map[string]*module.BlogComments {
	keys, err := p.client.Keys("comments@*").Result()
	if err != nil {
		log.ErrorF("getcomments error keys=comments@* err=%s", err.Error())
		return nil
	}

	bcs := make(map[string]*module.BlogComments)

	for _, key := range keys {
		m, err := p.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("getComments error key=%s err=%s", key, err.Error())
			continue
		}
		log.DebugF("getComments success key=%s", key)
		title := key[strings.Index(key, "@")+1:]
		p.toBlogComments(title, m, bcs)
	}

	return bcs
}

func (p *PersistenceActor) toBlogComments(title string, m map[string]string, bcs map[string]*module.BlogComments) {
	bc, ok := bcs[title]
	if !ok {
		bc = &module.BlogComments{
			Title: title,
		}
		bcs[title] = bc
	}

	for _, v := range m {
		owner := ""
		msg := ""
		ct := ""
		mt := ""
		mail := ""
		idx := -1
		pwd := ""

		// analy the hash value, split by ASCII 0x01 which is can not print
		tokens := strings.Split(v, "\x01")
		log.DebugF("toBlogComments v=%s tokens_len=%d", v, len(tokens))
		for _, t := range tokens {
			kv := strings.Split(t, "=")
			if len(kv) >= 2 {
				k := kv[0]
				v := t[strings.Index(t, "=")+1:]
				log.DebugF("k=%s v=%s", k, v)

				if strings.ToLower(k) == "owner" {
					owner = v
				} else if strings.ToLower(k) == "msg" {
					msg = v
				} else if strings.ToLower(k) == "ct" {
					ct = v
				} else if strings.ToLower(k) == "mt" {
					mt = v
				} else if strings.ToLower(k) == "mail" {
					mail = v
				} else if strings.ToLower(k) == "idx" {
					the_idx, err := strconv.Atoi(v)
					if err != nil {
						log.ErrorF("split idx conv to int error %s the_idx=%d", err.Error(), the_idx)
					} else {
						idx = the_idx
					}
				} else if strings.ToLower(k) == "pwd" {
					pwd = v
				}

			} else {
				log.ErrorF("split tokens %s error kv <= 2", t)
			}

		}

		if idx < 0 {
			log.ErrorF("toBlogComments idx<0 idx=%d", idx)
			continue
		}

		c := module.Comment{
			Owner:      owner,
			Msg:        msg,
			CreateTime: ct,
			ModifyTime: mt,
			Mail:       mail,
			Idx:        idx,
			Pwd:        pwd,
		}
		bc.Comments = append(bc.Comments, &c)
	}

	// sort by c.Idx
	sort.SliceStable(bc.Comments, func(i, j int) bool {
		return bc.Comments[i].Idx < bc.Comments[j].Idx
	})

	p.showBlogComments(bc)
}

func (p *PersistenceActor) showBlogComments(cs *module.BlogComments) {
	log.DebugF("title=%s", cs.Title)
	for _, c := range cs.Comments {
		log.DebugF("Idx=%d", c.Idx)
		log.DebugF("owner=%s", c.Owner)
		log.DebugF("msg=%s", c.Msg)
		log.DebugF("ct=%s", c.CreateTime)
		log.DebugF("mt=%s", c.ModifyTime)
	}
}

func (p *PersistenceActor) saveCommentUser(account string, user *module.CommentUser) {
	key := fmt.Sprintf("comment_user@%s", user.UserID)
	values := make(map[string]interface{})
	values["user_id"] = user.UserID
	values["username"] = user.Username
	values["email"] = user.Email
	values["avatar"] = user.Avatar
	values["register_time"] = user.RegisterTime
	values["last_active"] = user.LastActive
	values["comment_count"] = user.CommentCount
	values["reputation"] = user.Reputation
	values["status"] = user.Status
	values["is_verified"] = user.IsVerified

	err := p.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveCommentUser error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveCommentUser success key=%s", key)
	}
}

func (p *PersistenceActor) saveCommentSession(account string, session *module.CommentSession) {
	key := fmt.Sprintf("comment_session@%s", session.SessionID)
	values := make(map[string]interface{})
	values["session_id"] = session.SessionID
	values["user_id"] = session.UserID
	values["ip"] = session.IP
	values["user_agent"] = session.UserAgent
	values["create_time"] = session.CreateTime
	values["expire_time"] = session.ExpireTime
	values["is_active"] = session.IsActive

	err := p.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveCommentSession error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveCommentSession success key=%s", key)
	}
}

func (p *PersistenceActor) saveUsernameReservation(account string, reservation *module.UsernameReservation) {
	key := fmt.Sprintf("username_reservation@%s", reservation.Username)
	values := make(map[string]interface{})
	values["username"] = reservation.Username
	values["user_id"] = reservation.UserID
	values["reserve_time"] = reservation.ReserveTime
	values["is_temporary"] = reservation.IsTemporary

	err := p.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveUsernameReservation error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveUsernameReservation success key=%s", key)
	}
}

func (p *PersistenceActor) getAllCommentUsers(account string) map[string]*module.CommentUser {
	keys, err := p.client.Keys("comment_user@*").Result()
	if err != nil {
		log.ErrorF("GetAllCommentUsers error keys=comment_user@* err=%s", err.Error())
		return nil
	}

	users := make(map[string]*module.CommentUser)

	for _, key := range keys {
		m, err := p.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllCommentUsers error key=%s err=%s", key, err.Error())
			continue
		}

		user := p.toCommentUser(m)
		if user != nil {
			users[user.UserID] = user
			log.DebugF("GetAllCommentUsers success key=%s", key)
		}
	}

	return users
}

func (p *PersistenceActor) getAllUsernameReservations(account string) map[string]*module.UsernameReservation {
	keys, err := p.client.Keys("username_reservation@*").Result()
	if err != nil {
		log.ErrorF("GetAllUsernameReservations error keys=username_reservation@* err=%s", err.Error())
		return nil
	}

	reservations := make(map[string]*module.UsernameReservation)

	for _, key := range keys {
		m, err := p.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllUsernameReservations error key=%s err=%s", key, err.Error())
			continue
		}

		reservation := p.toUsernameReservation(m)
		if reservation != nil {
			reservations[reservation.Username] = reservation
		}
	}

	return reservations
}

func (p *PersistenceActor) getAllCommentSessions(account string) map[string]*module.CommentSession {
	keys, err := p.client.Keys("comment_session@*").Result()
	if err != nil {
		log.ErrorF("GetAllCommentSessions error keys=comment_session@* err=%s", err.Error())
		return nil
	}

	sessions := make(map[string]*module.CommentSession)

	for _, key := range keys {
		m, err := p.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllCommentSessions error key=%s err=%s", key, err.Error())
			continue
		}

		session := p.toCommentSession(m)
		if session != nil {
			sessions[session.SessionID] = session
		}
	}

	return sessions
}

func (p *PersistenceActor) deleteCommentSession(account, sessionID string) {
	key := fmt.Sprintf("comment_session@%s", sessionID)
	err := p.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteCommentSession error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteCommentSession success key=%s", key)
	}
}

func (p *PersistenceActor) deleteUsernameReservation(account, username string) {
	key := fmt.Sprintf("username_reservation@%s", username)
	err := p.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteUsernameReservation error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteUsernameReservation success key=%s", key)
	}
}

func (p *PersistenceActor) toCommentUser(m map[string]string) *module.CommentUser {
	userID, ok := m["user_id"]
	if !ok {
		return nil
	}

	commentCount, _ := strconv.Atoi(m["comment_count"])
	reputation, _ := strconv.Atoi(m["reputation"])
	status, _ := strconv.Atoi(m["status"])
	isVerified, _ := strconv.ParseBool(m["is_verified"])

	return &module.CommentUser{
		UserID:       userID,
		Username:     m["username"],
		Email:        m["email"],
		Avatar:       m["avatar"],
		RegisterTime: m["register_time"],
		LastActive:   m["last_active"],
		CommentCount: commentCount,
		Reputation:   reputation,
		Status:       status,
		IsVerified:   isVerified,
	}
}

func (p *PersistenceActor) toUsernameReservation(m map[string]string) *module.UsernameReservation {
	username, ok := m["username"]
	if !ok {
		return nil
	}

	isTemporary, _ := strconv.ParseBool(m["is_temporary"])

	return &module.UsernameReservation{
		Username:    username,
		UserID:      m["user_id"],
		ReserveTime: m["reserve_time"],
		IsTemporary: isTemporary,
	}
}

func (p *PersistenceActor) toCommentSession(m map[string]string) *module.CommentSession {
	sessionID, ok := m["session_id"]
	if !ok {
		return nil
	}

	isActive, _ := strconv.ParseBool(m["is_active"])

	return &module.CommentSession{
		SessionID:  sessionID,
		UserID:     m["user_id"],
		IP:         m["ip"],
		UserAgent:  m["user_agent"],
		CreateTime: m["create_time"],
		ExpireTime: m["expire_time"],
		IsActive:   isActive,
	}
}

func (p *PersistenceActor) getBlogWithAccount(account, name string) *module.Blog {

	key := fmt.Sprintf("%s:blog@%s", account, name)
	m, err := p.client.HGetAll(key).Result()
	if err != nil {
		log.ErrorF("getBlogWithAccount error key=%s err=%s", key, err.Error())
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	log.DebugF("getBlogWithAccount success key=%s title=%s", key, m["title"])
	b := p.toBlog(m)
	return b
}

func (p *PersistenceActor) deleteBlogWithAccount(account, title string) int {

	// delete account-specific key
	key := fmt.Sprintf("%s:blog@%s", account, title)
	err := p.client.Del(key).Err()
	if err != nil {
		log.ErrorF("deleteBlogWithAccount error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("deleteBlogWithAccount success key=%s", key)
	}

	// also try to delete the file
	p.deleteFile(account, title)
	return 0
}
