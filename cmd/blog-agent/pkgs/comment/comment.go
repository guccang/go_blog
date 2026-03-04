package comment

import (
	"config"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"module"
	log "mylog"
	db "persistence"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ========== Simple Comment 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

// 数据结构
type AccountCommentData struct {
	comments map[string]*module.BlogComments
}

type CommentAccountData struct {
	Users     map[string]*module.CommentUser
	Sessions  map[string]*module.CommentSession
	Usernames map[string]*module.UsernameReservation
}

type CommentUserManager struct {
	AccountData map[string]*CommentAccountData
}

var (
	comments    map[string]*AccountCommentData
	userManager *CommentUserManager
	commentMu   sync.RWMutex
)

func Info() {
	log.InfoF(log.ModuleComment, "info comment v4.0 (simple)")
}

// Init 初始化 Comment 模块
func Init() {
	commentMu.Lock()
	defer commentMu.Unlock()

	comments = make(map[string]*AccountCommentData)
	userManager = &CommentUserManager{
		AccountData: make(map[string]*CommentAccountData),
	}
}

func strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// LoadComments 加载评论
func LoadComments(account string) {
	commentMu.Lock()
	defer commentMu.Unlock()
	initUserManager(account)
}

func initUserManager(account string) {
	userManager.AccountData[account] = &CommentAccountData{
		Users:     make(map[string]*module.CommentUser),
		Sessions:  make(map[string]*module.CommentSession),
		Usernames: make(map[string]*module.UsernameReservation),
	}
	comments[account] = &AccountCommentData{
		comments: make(map[string]*module.BlogComments),
	}

	// 加载数据
	loadCommentUsers(account)
	loadUsernameLocks(account)
	loadCommentSessions(account)

	all_datas := db.GetAllBlogCommentsWithAccount(account)
	if all_datas != nil {
		for _, c := range all_datas {
			comments[account].comments[c.Title] = c
		}
	}
	log.DebugF(log.ModuleComment, "getComments number=%d", len(comments[account].comments))
}

// ========== 对外接口 ==========

func AddComment(account, title, msg, owner, pwd, mail string) int {
	commentMu.Lock()
	defer commentMu.Unlock()

	bc, ok := comments[account].comments[title]
	if !ok {
		bc = &module.BlogComments{Title: title}
		comments[account].comments[title] = bc
	}

	if len(bc.Comments) > config.GetMaxBlogComments() {
		return 1
	}

	comment := module.Comment{
		Owner: owner, Msg: msg, CreateTime: strTime(), ModifyTime: strTime(),
		Idx: len(bc.Comments), Pwd: pwd, Mail: mail,
	}
	bc.Comments = append(bc.Comments, &comment)
	db.SaveBlogCommentsWithAccount(account, bc)
	return 0
}

func AddCommentWithAuth(account, title, msg, sessionID, ip, userAgent string) (int, string) {
	commentMu.Lock()
	defer commentMu.Unlock()

	user, err := validateSession(account, sessionID)
	if err != nil {
		return 1, err.Error()
	}

	canComment, reason := canUserComment(account, user.UserID)
	if !canComment {
		return 2, reason
	}

	bc, ok := comments[account].comments[title]
	if !ok {
		bc = &module.BlogComments{Title: title}
		comments[account].comments[title] = bc
	}

	if len(bc.Comments) > config.GetMaxBlogComments() {
		return 3, "评论数量已达上限"
	}

	comment := module.Comment{
		Owner: user.Username, Msg: msg, CreateTime: strTime(), ModifyTime: strTime(),
		Idx: len(bc.Comments), Mail: user.Email, UserID: user.UserID,
		SessionID: sessionID, IP: ip, UserAgent: userAgent, IsAnonymous: false, IsVerified: user.IsVerified,
	}
	bc.Comments = append(bc.Comments, &comment)
	db.SaveBlogCommentsWithAccount(account, bc)
	incrementUserCommentCount(account, user.UserID)
	return 0, "评论发表成功"
}

func AddAnonymousComment(account, title, msg, username, email, ip, userAgent string) (int, string) {
	commentMu.Lock()
	defer commentMu.Unlock()

	session, err := createAnonymousSession(account, username, email, ip, userAgent)
	if err != nil {
		return 1, err.Error()
	}
	commentMu.Unlock()
	ret, message := AddCommentWithAuth(account, title, msg, session.SessionID, ip, userAgent)
	commentMu.Lock()
	return ret, message
}

func AddCommentWithPassword(account, title, msg, username, email, password, ip, userAgent string) (int, string, string) {
	commentMu.Lock()
	defer commentMu.Unlock()

	session, _, err := createOrAuthenticateSession(account, username, email, password, ip, userAgent)
	if err != nil {
		return 1, err.Error(), ""
	}
	commentMu.Unlock()
	ret, message := AddCommentWithAuth(account, title, msg, session.SessionID, ip, userAgent)
	commentMu.Lock()
	return ret, message, session.SessionID
}

func ModifyComment(account, title, msg string, idx int) int {
	commentMu.Lock()
	defer commentMu.Unlock()

	bc, ok := comments[account].comments[title]
	if !ok || idx >= len(bc.Comments) {
		return 1
	}
	bc.Comments[idx].Msg = msg
	db.SaveBlogCommentsWithAccount(account, bc)
	return 0
}

func RemoveComment(account, title string, idx int) int {
	commentMu.Lock()
	defer commentMu.Unlock()

	bc, ok := comments[account].comments[title]
	if !ok || idx >= len(bc.Comments) {
		return 1
	}

	sub := bc.Comments[:0]
	cnt := 0
	for i, v := range bc.Comments {
		if i != idx {
			v.Idx = cnt
			sub = append(sub, v)
			cnt++
		}
	}
	bc.Comments = sub
	return 0
}

func GetComments(account, title string) *module.BlogComments {
	commentMu.RLock()
	defer commentMu.RUnlock()

	if _, exist := comments[account]; !exist {
		return nil
	}
	return comments[account].comments[title]
}

func GetAllComments(account string) map[string]*module.BlogComments {
	commentMu.RLock()
	defer commentMu.RUnlock()

	if _, exist := comments[account]; !exist {
		return nil
	}
	return comments[account].comments
}

func IsUsernameAvailable(account, username string) bool {
	commentMu.RLock()
	defer commentMu.RUnlock()
	users := getUsersByUsername(account, username)
	return len(users) == 0
}

func GetUsersByUsername(account, username string) []*module.CommentUser {
	commentMu.RLock()
	defer commentMu.RUnlock()
	return getUsersByUsername(account, username)
}

func ValidateSession(account, sessionID string) (*module.CommentUser, error) {
	commentMu.Lock()
	defer commentMu.Unlock()
	return validateSession(account, sessionID)
}

// ========== 内部函数 ==========

func generateUserID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("user_%s", hex.EncodeToString(bytes)[:16])
}

func generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("session_%s", hex.EncodeToString(bytes)[:24])
}

func validateUsername(username string) error {
	if len(username) < 2 || len(username) > 20 {
		return errors.New("用户名长度必须在2-20个字符之间")
	}
	pattern := `^[\p{Han}a-zA-Z0-9_]+$`
	if !regexp.MustCompile(pattern).MatchString(username) {
		return errors.New("用户名只能包含中文、英文、数字和下划线")
	}
	forbidden := []string{"admin", "管理员", "系统", "匿名", "游客", "root", "test"}
	for _, word := range forbidden {
		if strings.Contains(strings.ToLower(username), strings.ToLower(word)) {
			return errors.New("用户名包含禁用词汇")
		}
	}
	return nil
}

func getUsersByUsername(account, username string) []*module.CommentUser {
	var users []*module.CommentUser
	if _, exist := userManager.AccountData[account]; !exist {
		return users
	}
	for _, user := range userManager.AccountData[account].Users {
		if user.Username == username {
			users = append(users, user)
		}
	}
	return users
}

func validateSession(account, sessionID string) (*module.CommentUser, error) {
	if _, exist := userManager.AccountData[account]; !exist {
		return nil, errors.New("账户不存在")
	}
	session, exists := userManager.AccountData[account].Sessions[sessionID]
	if !exists {
		return nil, errors.New("会话不存在")
	}

	expireTime, err := time.Parse("2006-01-02 15:04:05", session.ExpireTime)
	if err != nil || time.Now().After(expireTime) {
		delete(userManager.AccountData[account].Sessions, sessionID)
		return nil, errors.New("会话已过期")
	}

	user, exists := userManager.AccountData[account].Users[session.UserID]
	if !exists {
		return nil, errors.New("用户不存在")
	}
	if user.Status == 2 {
		return nil, errors.New("用户被禁言")
	}
	if user.Status == 3 {
		return nil, errors.New("用户被封禁")
	}

	user.LastActive = strTime()
	db.SaveCommentUserWithAccount(account, user)
	return user, nil
}

func canUserComment(account, userID string) (bool, string) {
	user, exists := userManager.AccountData[account].Users[userID]
	if !exists {
		return false, "用户不存在"
	}
	switch user.Status {
	case 1:
		return true, ""
	case 2:
		return false, "用户被禁言"
	case 3:
		return false, "用户被封禁"
	default:
		return false, "用户状态异常"
	}
}

func incrementUserCommentCount(account, userID string) {
	user, exists := userManager.AccountData[account].Users[userID]
	if exists {
		user.CommentCount++
		user.LastActive = strTime()
		if user.CommentCount%10 == 0 {
			user.Reputation += 5
		} else {
			user.Reputation++
		}
		db.SaveCommentUserWithAccount(account, user)
	}
}

func createAnonymousSession(account, username, email, ip, userAgent string) (*module.CommentSession, error) {
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if len(getUsersByUsername(account, username)) > 0 {
		return nil, errors.New("该用户名已被注册，请输入密码进行身份验证")
	}

	userID := generateUserID()
	sessionID := generateSessionID()

	user := &module.CommentUser{
		UserID: userID, Username: username, Email: email,
		RegisterTime: strTime(), LastActive: strTime(),
		CommentCount: 0, Reputation: 0, Status: 1, IsVerified: false,
	}
	userManager.AccountData[account].Users[userID] = user

	session := &module.CommentSession{
		SessionID: sessionID, UserID: userID, IP: ip, UserAgent: userAgent,
		CreateTime: strTime(), ExpireTime: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
		IsActive: true,
	}
	userManager.AccountData[account].Sessions[sessionID] = session

	db.SaveCommentUserWithAccount(account, user)
	db.SaveCommentSessionWithAccount(account, session)
	return session, nil
}

func createOrAuthenticateSession(account, username, email, password, ip, userAgent string) (*module.CommentSession, *module.CommentUser, error) {
	if err := validateUsername(username); err != nil {
		return nil, nil, err
	}

	existingUsers := getUsersByUsername(account, username)
	if len(existingUsers) > 0 {
		if password == "" {
			return nil, nil, errors.New("该用户名已被注册，请输入密码进行身份验证")
		}
		// 验证密码
		for _, user := range existingUsers {
			hashedPassword := fmt.Sprintf("%x", []byte(password+user.UserID[:8]))
			if user.Email == hashedPassword {
				sessionID := generateSessionID()
				session := &module.CommentSession{
					SessionID: sessionID, UserID: user.UserID, IP: ip, UserAgent: userAgent,
					CreateTime: strTime(), ExpireTime: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
					IsActive: true,
				}
				userManager.AccountData[account].Sessions[sessionID] = session
				db.SaveCommentSessionWithAccount(account, session)
				return session, user, nil
			}
		}
		return nil, nil, errors.New("密码错误，无法使用该用户名")
	}

	// 新用户
	userID := generateUserID()
	sessionID := generateSessionID()

	user := &module.CommentUser{
		UserID: userID, Username: username, Email: email,
		RegisterTime: strTime(), LastActive: strTime(),
		CommentCount: 0, Reputation: 0, Status: 1, IsVerified: false,
	}
	if password != "" {
		user.Email = fmt.Sprintf("%x", []byte(password+userID[:8]))
	}
	userManager.AccountData[account].Users[userID] = user

	session := &module.CommentSession{
		SessionID: sessionID, UserID: userID, IP: ip, UserAgent: userAgent,
		CreateTime: strTime(), ExpireTime: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
		IsActive: true,
	}
	userManager.AccountData[account].Sessions[sessionID] = session

	db.SaveCommentUserWithAccount(account, user)
	db.SaveCommentSessionWithAccount(account, session)
	return session, user, nil
}

func loadCommentUsers(account string) {
	users := db.GetAllCommentUsersWithAccount(account)
	if users != nil {
		for userID, user := range users {
			userManager.AccountData[account].Users[userID] = user
		}
	}
}

func loadUsernameLocks(account string) {
	reservations := db.GetAllUsernameReservationsWithAccount(account)
	if reservations != nil {
		for username, r := range reservations {
			userManager.AccountData[account].Usernames[username] = r
		}
	}
}

func loadCommentSessions(account string) {
	sessions := db.GetAllCommentSessionsWithAccount(account)
	if sessions != nil {
		for sessionID, s := range sessions {
			userManager.AccountData[account].Sessions[sessionID] = s
		}
	}
}
