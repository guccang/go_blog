package persistence

import (
	"config"
	"fmt"
	"ioutils"
	"module"
	log "mylog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
)

// ========== Simple Persistence 模块 ==========
// 无 Actor、无 Channel，使用 sync.Mutex

var (
	client      *redis.Client
	persistence sync.Mutex
)

func Info() {
	log.Debug(log.ModulePersistence, "info persistence v2.0 (simple)")
}

// Init 初始化 Persistence 模块
func Init() {
	persistence.Lock()
	defer persistence.Unlock()

	ip := config.GetConfigWithAccount(config.GetAdminAccount(), "redis_ip")
	port, _ := strconv.Atoi(config.GetConfigWithAccount(config.GetAdminAccount(), "redis_port"))
	pwd := config.GetConfigWithAccount(config.GetAdminAccount(), "redis_pwd")
	connect(ip, port, pwd)
}

func strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func connect(ip string, port int, password string) int {
	c := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", ip, port),
		Password: password,
		DB:       0,
	})

	pong, err := c.Ping().Result()
	if err == nil {
		client = c
		log.DebugF(log.ModulePersistence, "connect redis success ip=%s port=%d", ip, port)
		return 1
	}
	log.DebugF(log.ModulePersistence, pong, err)
	return 0
}

// ========== Blog 操作 ==========

func SaveBlog(account string, blog *module.Blog) {
	persistence.Lock()
	defer persistence.Unlock()
	saveBlogInternal(account, blog)
}

func saveBlogInternal(account string, blog *module.Blog) {
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

	if err := client.HMSet(key, values).Err(); err != nil {
		log.ErrorF(log.ModulePersistence, "saveblog error key=%s err=%s", key, err.Error())
	}
	log.DebugF(log.ModulePersistence, "redis saveblog success key=%s", key)

	// 删除旧格式key
	old_key := fmt.Sprintf("blog@%s", blog.Title)
	client.Del(old_key)

	saveToFile(account, blog)
}

func SaveBlogs(account string, blogs map[string]*module.Blog) {
	persistence.Lock()
	defer persistence.Unlock()
	for _, b := range blogs {
		saveBlogInternal(account, b)
	}
}

func GetBlogsByAccount(account string) map[string]*module.Blog {
	persistence.Lock()
	defer persistence.Unlock()

	pattern := fmt.Sprintf("%s:blog@*", account)
	keys, err := client.Keys(pattern).Result()
	if err != nil {
		log.ErrorF(log.ModulePersistence, "getblogsbyaccount error pattern=%s err=%s", pattern, err.Error())
		return nil
	}

	if account == config.GetAdminAccount() {
		legacy, _ := client.Keys("blog@*").Result()
		keys = append(keys, legacy...)
	}

	blogs := make(map[string]*module.Blog)
	for _, key := range keys {
		m, err := client.HGetAll(key).Result()
		if err != nil {
			continue
		}
		b := toBlog(m)
		blogs[b.Title] = b
	}
	return blogs
}

func GetBlogWithAccount(account, name string) *module.Blog {
	persistence.Lock()
	defer persistence.Unlock()

	key := fmt.Sprintf("%s:blog@%s", account, name)
	m, err := client.HGetAll(key).Result()
	if err != nil || len(m) == 0 {
		return nil
	}
	return toBlog(m)
}

func DeleteBlogWithAccount(account, title string) int {
	persistence.Lock()
	defer persistence.Unlock()

	key := fmt.Sprintf("%s:blog@%s", account, title)
	client.Del(key)
	deleteFile(account, title)
	return 0
}

func toBlog(m map[string]string) *module.Blog {
	now := strTime()
	ct := m["ct"]
	if ct == "" {
		ct = now
	}
	mt := m["mt"]
	if mt == "" {
		mt = now
	}
	at := m["at"]
	if at == "" {
		at = now
	}
	mn, _ := strconv.Atoi(m["modifynum"])
	an, _ := strconv.Atoi(m["accessnum"])
	auth, _ := strconv.Atoi(m["authtype"])
	encrypt, _ := strconv.Atoi(m["encrypt"])

	return &module.Blog{
		Title:      m["title"],
		Content:    m["content"],
		CreateTime: ct,
		ModifyTime: mt,
		AccessTime: at,
		ModifyNum:  mn,
		AccessNum:  an,
		AuthType:   auth,
		Tags:       m["tags"],
		Encrypt:    encrypt,
		Account:    m["account"],
	}
}

func saveToFile(account string, blog *module.Blog) {
	if blog.Account == "" {
		blog.Account = account
	}
	path := config.GetBlogsPath(account)
	full := fmt.Sprintf("%s.md", filepath.Join(path, blog.Title))
	// 确保父目录存在（支持子文件夹 Title，如 agent_tasks/xxx/output）
	ioutils.Mkdir(filepath.Dir(full))

	fcontent, _ := ioutils.GetFileDatas(full)
	if blog.Content == fcontent {
		return
	}
	ioutils.RmAndSaveFile(full, blog.Content)
}

func deleteFile(account, title string) int {
	path := config.GetBlogsPath(account)
	ioutils.Mkdir(path)
	full := fmt.Sprintf("%s.md", filepath.Join(path, title))

	recycle_path := config.GetRecyclePath()
	ioutils.Mkdir(recycle_path)
	new_filename := fmt.Sprintf("%s-%s.md", title, time.Now().Format("2006-01-02"))
	ioutils.Mvfile(full, filepath.Join(recycle_path, new_filename))
	return 0
}

// ========== Comments 操作 ==========

func SaveBlogComments(account string, bc *module.BlogComments) {
	persistence.Lock()
	defer persistence.Unlock()

	key := fmt.Sprintf("comments@%s", bc.Title)
	values := make(map[string]interface{})
	s := "\x01"
	for _, c := range bc.Comments {
		value := fmt.Sprintf("Idx=%d%sowner=%s%sct=%s%smt=%s%smsg=%s%smail=%s%sPwd=%s",
			c.Idx, s, c.Owner, s, c.CreateTime, s, c.ModifyTime, s, c.Msg, s, c.Mail, s, s, c.Pwd)
		values[fmt.Sprintf("%d", c.Idx)] = value
	}
	client.HMSet(key, values)
}

func GetAllBlogComments(account string) map[string]*module.BlogComments {
	persistence.Lock()
	defer persistence.Unlock()

	keys, err := client.Keys("comments@*").Result()
	if err != nil {
		return nil
	}

	bcs := make(map[string]*module.BlogComments)
	for _, key := range keys {
		m, err := client.HGetAll(key).Result()
		if err != nil {
			continue
		}
		title := key[strings.Index(key, "@")+1:]
		toBlogComments(title, m, bcs)
	}
	return bcs
}

func toBlogComments(title string, m map[string]string, bcs map[string]*module.BlogComments) {
	bc, ok := bcs[title]
	if !ok {
		bc = &module.BlogComments{Title: title}
		bcs[title] = bc
	}

	for _, v := range m {
		owner, msg, ct, mt, mail, pwd := "", "", "", "", "", ""
		idx := -1

		tokens := strings.Split(v, "\x01")
		for _, t := range tokens {
			kv := strings.Split(t, "=")
			if len(kv) >= 2 {
				k := strings.ToLower(kv[0])
				val := t[strings.Index(t, "=")+1:]
				switch k {
				case "owner":
					owner = val
				case "msg":
					msg = val
				case "ct":
					ct = val
				case "mt":
					mt = val
				case "mail":
					mail = val
				case "idx":
					idx, _ = strconv.Atoi(val)
				case "pwd":
					pwd = val
				}
			}
		}

		if idx >= 0 {
			bc.Comments = append(bc.Comments, &module.Comment{
				Owner: owner, Msg: msg, CreateTime: ct, ModifyTime: mt, Mail: mail, Idx: idx, Pwd: pwd,
			})
		}
	}

	sort.SliceStable(bc.Comments, func(i, j int) bool {
		return bc.Comments[i].Idx < bc.Comments[j].Idx
	})
}

// ========== CommentUser 操作 ==========

func SaveCommentUser(account string, user *module.CommentUser) {
	persistence.Lock()
	defer persistence.Unlock()

	key := fmt.Sprintf("comment_user@%s", user.UserID)
	values := map[string]interface{}{
		"user_id": user.UserID, "username": user.Username, "email": user.Email,
		"avatar": user.Avatar, "register_time": user.RegisterTime, "last_active": user.LastActive,
		"comment_count": user.CommentCount, "reputation": user.Reputation,
		"status": user.Status, "is_verified": user.IsVerified,
	}
	client.HMSet(key, values)
}

func GetAllCommentUsers(account string) map[string]*module.CommentUser {
	persistence.Lock()
	defer persistence.Unlock()

	keys, _ := client.Keys("comment_user@*").Result()
	users := make(map[string]*module.CommentUser)
	for _, key := range keys {
		m, err := client.HGetAll(key).Result()
		if err != nil {
			continue
		}
		user := toCommentUser(m)
		if user != nil {
			users[user.UserID] = user
		}
	}
	return users
}

func toCommentUser(m map[string]string) *module.CommentUser {
	userID, ok := m["user_id"]
	if !ok {
		return nil
	}
	commentCount, _ := strconv.Atoi(m["comment_count"])
	reputation, _ := strconv.Atoi(m["reputation"])
	status, _ := strconv.Atoi(m["status"])
	isVerified, _ := strconv.ParseBool(m["is_verified"])
	return &module.CommentUser{
		UserID: userID, Username: m["username"], Email: m["email"], Avatar: m["avatar"],
		RegisterTime: m["register_time"], LastActive: m["last_active"],
		CommentCount: commentCount, Reputation: reputation, Status: status, IsVerified: isVerified,
	}
}

// ========== CommentSession 操作 ==========

func SaveCommentSession(account string, session *module.CommentSession) {
	persistence.Lock()
	defer persistence.Unlock()

	key := fmt.Sprintf("comment_session@%s", session.SessionID)
	values := map[string]interface{}{
		"session_id": session.SessionID, "user_id": session.UserID, "ip": session.IP,
		"user_agent": session.UserAgent, "create_time": session.CreateTime,
		"expire_time": session.ExpireTime, "is_active": session.IsActive,
	}
	client.HMSet(key, values)
}

func GetAllCommentSessions(account string) map[string]*module.CommentSession {
	persistence.Lock()
	defer persistence.Unlock()

	keys, _ := client.Keys("comment_session@*").Result()
	sessions := make(map[string]*module.CommentSession)
	for _, key := range keys {
		m, err := client.HGetAll(key).Result()
		if err != nil {
			continue
		}
		session := toCommentSession(m)
		if session != nil {
			sessions[session.SessionID] = session
		}
	}
	return sessions
}

func toCommentSession(m map[string]string) *module.CommentSession {
	sessionID, ok := m["session_id"]
	if !ok {
		return nil
	}
	isActive, _ := strconv.ParseBool(m["is_active"])
	return &module.CommentSession{
		SessionID: sessionID, UserID: m["user_id"], IP: m["ip"], UserAgent: m["user_agent"],
		CreateTime: m["create_time"], ExpireTime: m["expire_time"], IsActive: isActive,
	}
}

func DeleteCommentSession(account, sessionID string) {
	persistence.Lock()
	defer persistence.Unlock()
	client.Del(fmt.Sprintf("comment_session@%s", sessionID))
}

// ========== UsernameReservation 操作 ==========

func SaveUsernameReservation(account string, reservation *module.UsernameReservation) {
	persistence.Lock()
	defer persistence.Unlock()

	key := fmt.Sprintf("username_reservation@%s", reservation.Username)
	values := map[string]interface{}{
		"username": reservation.Username, "user_id": reservation.UserID,
		"reserve_time": reservation.ReserveTime, "is_temporary": reservation.IsTemporary,
	}
	client.HMSet(key, values)
}

func GetAllUsernameReservations(account string) map[string]*module.UsernameReservation {
	persistence.Lock()
	defer persistence.Unlock()

	keys, _ := client.Keys("username_reservation@*").Result()
	reservations := make(map[string]*module.UsernameReservation)
	for _, key := range keys {
		m, err := client.HGetAll(key).Result()
		if err != nil {
			continue
		}
		r := toUsernameReservation(m)
		if r != nil {
			reservations[r.Username] = r
		}
	}
	return reservations
}

func toUsernameReservation(m map[string]string) *module.UsernameReservation {
	username, ok := m["username"]
	if !ok {
		return nil
	}
	isTemporary, _ := strconv.ParseBool(m["is_temporary"])
	return &module.UsernameReservation{
		Username: username, UserID: m["user_id"], ReserveTime: m["reserve_time"], IsTemporary: isTemporary,
	}
}

func DeleteUsernameReservation(account, username string) {
	persistence.Lock()
	defer persistence.Unlock()
	client.Del(fmt.Sprintf("username_reservation@%s", username))
}

// ========== 兼容性函数 ==========

func SaveBlogWithAccount(account string, blog *module.Blog)              { SaveBlog(account, blog) }
func SaveBlogsWithAccount(account string, blogs map[string]*module.Blog) { SaveBlogs(account, blogs) }
func SaveBlogCommentsWithAccount(account string, bc *module.BlogComments) {
	SaveBlogComments(account, bc)
}
func GetAllBlogCommentsWithAccount(account string) map[string]*module.BlogComments {
	return GetAllBlogComments(account)
}
func SaveCommentUserWithAccount(account string, user *module.CommentUser) {
	SaveCommentUser(account, user)
}
func SaveCommentSessionWithAccount(account string, session *module.CommentSession) {
	SaveCommentSession(account, session)
}
func SaveUsernameReservationWithAccount(account string, reservation *module.UsernameReservation) {
	SaveUsernameReservation(account, reservation)
}
func GetAllCommentUsersWithAccount(account string) map[string]*module.CommentUser {
	return GetAllCommentUsers(account)
}
func GetAllUsernameReservationsWithAccount(account string) map[string]*module.UsernameReservation {
	return GetAllUsernameReservations(account)
}
func GetAllCommentSessionsWithAccount(account string) map[string]*module.CommentSession {
	return GetAllCommentSessions(account)
}
func DeleteCommentSessionWithAccount(account, sessionID string) {
	DeleteCommentSession(account, sessionID)
}
func DeleteUsernameReservationWithAccount(account, username string) {
	DeleteUsernameReservation(account, username)
}
