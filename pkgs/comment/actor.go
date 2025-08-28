package comment

import (
	"config"
	"core"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"module"
	log "mylog"
	db "persistence"
	"regexp"
	"strings"
	"time"
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
  第三步: 在comment.go中添加对应的接口
  第四步: 在http中添加对应的接口
*/

// actor

type AccountCommentData struct {
	comments map[string]*module.BlogComments
}

type CommentActor struct {
	*core.Actor
	comments    map[string]*AccountCommentData
	userManager *CommentUserManager
}

type CommentAccountData struct {
	Users     map[string]*module.CommentUser
	Sessions  map[string]*module.CommentSession
	Usernames map[string]*module.UsernameReservation
}

// CommentUserManager - 评论用户管理器
type CommentUserManager struct {
	AccountData map[string]*CommentAccountData
}

func (c *CommentActor) strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func (c *CommentActor) LoadComments(account string) {
	// 初始化用户管理器
	c.initUserManager(account)

}

func (c *CommentActor) initUserManager(account string) {
	c.userManager.AccountData[account] = &CommentAccountData{
		Users:     make(map[string]*module.CommentUser),
		Sessions:  make(map[string]*module.CommentSession),
		Usernames: make(map[string]*module.UsernameReservation),
	}
	c.comments[account] = &AccountCommentData{
		comments: make(map[string]*module.BlogComments),
	}

	// 加载已存在的数据
	c.loadCommentUsers(account)
	c.loadUsernameLocks(account)
	c.loadCommentSessions(account)

	// 加载评论数据
	all_datas := db.GetAllBlogCommentsWithAccount(account)
	if all_datas != nil {
		for _, comment := range all_datas {
			c.comments[account].comments[comment.Title] = comment
		}
	}
	log.DebugF("getComments number=%d", len(c.comments[account].comments))

	log.Debug("CommentUserManager initialized")
}

func (c *CommentActor) addComment(account, title string, msg string, owner string, pwd string, mail string) int {
	bc, ok := c.comments[account].comments[title]
	if !ok {
		bc = &module.BlogComments{
			Title: title,
		}
		c.comments[account].comments[title] = bc
	}

	cur_cnt := len(bc.Comments)
	if cur_cnt > config.GetMaxBlogComments() {
		log.ErrorF("AddComment error comments max limits max=%d", config.GetMaxBlogComments())
		return 0
	}

	comment := module.Comment{
		Owner:      owner,
		Msg:        msg,
		CreateTime: c.strTime(),
		ModifyTime: c.strTime(),
		Idx:        len(bc.Comments),
		Pwd:        pwd,
		Mail:       mail,
	}
	bc.Comments = append(bc.Comments, &comment)
	db.SaveBlogCommentsWithAccount(account, bc)
	return 0
}

func (c *CommentActor) addCommentWithAuth(account, title, msg, sessionID, ip, userAgent string) (int, string) {
	// 验证会话
	user, err := c.validateSession(account, sessionID)
	if err != nil {
		return 1, err.Error()
	}

	// 检查用户是否可以评论
	canComment, reason := c.canUserComment(account, user.UserID)
	if !canComment {
		return 2, reason
	}

	// 获取或创建博客评论集合
	bc, ok := c.comments[account].comments[title]
	if !ok {
		bc = &module.BlogComments{Title: title}
		c.comments[account].comments[title] = bc
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
		CreateTime:  c.strTime(),
		ModifyTime:  c.strTime(),
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
	db.SaveBlogCommentsWithAccount(account, bc)

	// 更新用户评论计数
	c.incrementUserCommentCount(account, user.UserID)

	log.DebugF("AddCommentWithAuth success: user=%s title=%s", user.Username, title)
	return 0, "评论发表成功"
}

func (c *CommentActor) addAnonymousComment(account, title, msg, username, email, ip, userAgent string) (int, string) {
	// 创建匿名用户会话
	session, err := c.createAnonymousSession(account, username, email, ip, userAgent)
	if err != nil {
		return 1, err.Error()
	}

	// 使用新会话发表评论
	return c.addCommentWithAuth(account, title, msg, session.SessionID, ip, userAgent)
}

func (c *CommentActor) addCommentWithPassword(account, title, msg, username, email, password, ip, userAgent string) (int, string, string) {
	// 创建或验证用户会话
	session, _, err := c.createOrAuthenticateSession(account, username, email, password, ip, userAgent)
	if err != nil {
		return 1, err.Error(), ""
	}

	// 使用会话发表评论
	ret, message := c.addCommentWithAuth(account, title, msg, session.SessionID, ip, userAgent)
	return ret, message, session.SessionID
}

func (c *CommentActor) modifyComment(account, title string, msg string, idx int) int {
	bc, ok := c.comments[account].comments[title]
	if !ok {
		log.ErrorF("ModifyComment %s not find", title)
		return 1
	}
	if idx >= len(bc.Comments) {
		log.ErrorF("ModifyComment %s id=%d > len of comments %d", title, idx, len(bc.Comments))
		return 2
	}
	comment := bc.Comments[idx]
	comment.Msg = msg
	db.SaveBlogCommentsWithAccount(account, bc)
	return 0
}

func (c *CommentActor) removeComment(account, title string, idx int) int {
	bc, ok := c.comments[account].comments[title]
	if !ok {
		log.ErrorF("RemoveComment %s not find", title)
		return 1
	}
	if idx >= len(bc.Comments) {
		log.ErrorF("RemoveComment %s id=%d > len of comments %d", title, idx, len(bc.Comments))
		return 2
	}

	sub_comments := bc.Comments[:0]
	cnt := 0
	for i, v := range bc.Comments {
		if i != idx {
			sub_comments = append(sub_comments, v)
			v.Idx = cnt
			cnt = cnt + 1
		}
	}

	bc.Comments = sub_comments
	return 0
}

func (c *CommentActor) getComments(account, title string) *module.BlogComments {
	if _, exit := c.comments[account]; !exit {
		return nil
	}

	comment, ok := c.comments[account].comments[title]
	if !ok {
		return nil
	}
	return comment
}

func (c *CommentActor) getAllComments(account string) map[string]*module.BlogComments {
	return c.comments[account].comments
}

// User management methods

func (c *CommentActor) generateUserID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("user_%s", hex.EncodeToString(bytes)[:16])
}

func (c *CommentActor) generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("session_%s", hex.EncodeToString(bytes)[:24])
}

func (c *CommentActor) validateUsername(username string) error {
	if len(username) < 2 || len(username) > 20 {
		return errors.New("用户名长度必须在2-20个字符之间")
	}

	// 只允许中文、英文、数字、下划线
	pattern := `^[\p{Han}a-zA-Z0-9_]+$`
	reg := regexp.MustCompile(pattern)
	if !reg.MatchString(username) {
		return errors.New("用户名只能包含中文、英文、数字和下划线")
	}

	// 禁用敏感词
	forbidden := []string{"admin", "管理员", "系统", "匿名", "游客", "root", "test"}
	for _, word := range forbidden {
		if strings.Contains(strings.ToLower(username), strings.ToLower(word)) {
			return errors.New("用户名包含禁用词汇")
		}
	}

	return nil
}

func (c *CommentActor) isUsernameAvailable(account, username string) bool {
	users := c.getUsersByUsername(account, username)
	return len(users) == 0
}

func (c *CommentActor) getUsersByUsername(account, username string) []*module.CommentUser {
	var users []*module.CommentUser
	for _, user := range c.userManager.AccountData[account].Users {
		if user.Username == username {
			users = append(users, user)
		}
	}
	return users
}

func (c *CommentActor) createAnonymousSession(account, username, email, ip, userAgent string) (*module.CommentSession, error) {
	if err := c.validateUsername(username); err != nil {
		return nil, err
	}

	// 检查用户名是否已存在
	existingUsers := c.getUsersByUsername(account, username)
	if len(existingUsers) > 0 {
		return nil, errors.New("该用户名已被注册，请输入密码进行身份验证")
	}

	userID := c.generateUserID()
	sessionID := c.generateSessionID()

	// 创建匿名用户
	user := &module.CommentUser{
		UserID:       userID,
		Username:     username,
		Email:        email,
		RegisterTime: time.Now().Format("2006-01-02 15:04:05"),
		LastActive:   time.Now().Format("2006-01-02 15:04:05"),
		CommentCount: 0,
		Reputation:   0,
		Status:       1, // 正常状态
		IsVerified:   false,
	}
	c.userManager.AccountData[account].Users[userID] = user

	// 创建会话
	session := &module.CommentSession{
		SessionID:  sessionID,
		UserID:     userID,
		IP:         ip,
		UserAgent:  userAgent,
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		ExpireTime: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05"), // 7天过期
		IsActive:   true,
	}
	c.userManager.AccountData[account].Sessions[sessionID] = session

	// 保存到数据库
	c.saveCommentUser(account, user)
	c.saveCommentSession(account, session)

	log.DebugF("创建匿名用户会话: %s (%s)", username, userID)
	return session, nil
}

func (c *CommentActor) validateSession(account, sessionID string) (*module.CommentUser, error) {
	session, exists := c.userManager.AccountData[account].Sessions[sessionID]
	if !exists {
		return nil, errors.New("会话不存在")
	}

	// 检查会话是否过期
	expireTime, err := time.Parse("2006-01-02 15:04:05", session.ExpireTime)
	if err != nil || time.Now().After(expireTime) {
		// 会话过期，删除
		delete(c.userManager.AccountData[account].Sessions, sessionID)
		return nil, errors.New("会话已过期")
	}

	// 获取用户信息
	user, exists := c.userManager.AccountData[account].Users[session.UserID]
	if !exists {
		return nil, errors.New("用户不存在")
	}

	// 检查用户状态
	if user.Status == 2 {
		return nil, errors.New("用户被禁言")
	}
	if user.Status == 3 {
		return nil, errors.New("用户被封禁")
	}

	// 更新最后活跃时间
	user.LastActive = time.Now().Format("2006-01-02 15:04:05")
	c.saveCommentUser(account, user)

	return user, nil
}

func (c *CommentActor) canUserComment(account, userID string) (bool, string) {
	user, exists := c.userManager.AccountData[account].Users[userID]
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

func (c *CommentActor) incrementUserCommentCount(account, userID string) {
	user, exists := c.userManager.AccountData[account].Users[userID]
	if exists {
		user.CommentCount++
		user.LastActive = time.Now().Format("2006-01-02 15:04:05")

		// 根据评论数量增加信誉
		if user.CommentCount%10 == 0 {
			user.Reputation += 5
		} else {
			user.Reputation += 1
		}

		c.saveCommentUser(account, user)
		log.DebugF("用户 %s 评论计数更新: %d", user.Username, user.CommentCount)
	}
}

func (c *CommentActor) authenticateUser(account, username, password string) (*module.CommentUser, error) {
	users := c.getUsersByUsername(account, username)

	// 遍历所有同名用户，检查密码是否匹配
	for _, user := range users {
		// 简单的密码验证逻辑（实际应用中应该使用加密密码）
		// 这里使用email字段存储密码哈希，或者使用UserID的一部分作为简单验证
		if password != "" {
			hashedPassword := fmt.Sprintf("%x", []byte(password+user.UserID[:8])) // 简单的盐值
			if user.Email == hashedPassword {
				return user, nil
			}
		}
	}

	return nil, errors.New("用户名或密码不正确")
}

func (c *CommentActor) createOrAuthenticateSession(account, username, email, password, ip, userAgent string) (*module.CommentSession, *module.CommentUser, error) {
	if err := c.validateUsername(username); err != nil {
		return nil, nil, err
	}

	// 检查用户名是否已被使用
	existingUsers := c.getUsersByUsername(account, username)

	if len(existingUsers) > 0 {
		// 用户名已存在，必须提供正确的密码
		if password == "" {
			return nil, nil, errors.New("该用户名已被注册，请输入密码进行身份验证")
		}

		// 验证密码
		user, err := c.authenticateUser(account, username, password)
		if err != nil {
			return nil, nil, errors.New("密码错误，无法使用该用户名")
		}

		// 验证成功，创建新会话
		sessionID := c.generateSessionID()
		session := &module.CommentSession{
			SessionID:  sessionID,
			UserID:     user.UserID,
			IP:         ip,
			UserAgent:  userAgent,
			CreateTime: time.Now().Format("2006-01-02 15:04:05"),
			ExpireTime: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
			IsActive:   true,
		}
		c.userManager.AccountData[account].Sessions[sessionID] = session
		c.saveCommentSession(account, session)

		log.DebugF("用户身份验证成功，创建新会话: %s (%s)", username, user.UserID)
		return session, user, nil
	}

	// 新用户名，创建新用户
	userID := c.generateUserID()
	sessionID := c.generateSessionID()

	user := &module.CommentUser{
		UserID:       userID,
		Username:     username,
		Email:        email,
		RegisterTime: time.Now().Format("2006-01-02 15:04:05"),
		LastActive:   time.Now().Format("2006-01-02 15:04:05"),
		CommentCount: 0,
		Reputation:   0,
		Status:       1,
		IsVerified:   false,
	}

	// 如果提供了密码，保存密码信息
	if password != "" {
		hashedPassword := fmt.Sprintf("%x", []byte(password+userID[:8]))
		user.Email = hashedPassword
	}

	c.userManager.AccountData[account].Users[userID] = user

	// 创建会话
	session := &module.CommentSession{
		SessionID:  sessionID,
		UserID:     userID,
		IP:         ip,
		UserAgent:  userAgent,
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		ExpireTime: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
		IsActive:   true,
	}
	c.userManager.AccountData[account].Sessions[sessionID] = session

	// 保存到数据库
	c.saveCommentUser(account, user)
	c.saveCommentSession(account, session)

	log.DebugF("创建新用户会话: %s (%s)", username, userID)
	return session, user, nil
}

// Data persistence methods
func (c *CommentActor) saveCommentUser(account string, user *module.CommentUser) {
	db.SaveCommentUserWithAccount(account, user)
}

func (c *CommentActor) saveCommentSession(account string, session *module.CommentSession) {
	db.SaveCommentSessionWithAccount(account, session)
}

func (c *CommentActor) saveUsernameReservation(account string, reservation *module.UsernameReservation) {
	db.SaveUsernameReservationWithAccount(account, reservation)
}

func (c *CommentActor) loadCommentUsers(account string) {
	users := db.GetAllCommentUsersWithAccount(account)
	if users != nil {
		for userID, user := range users {
			c.userManager.AccountData[account].Users[userID] = user
		}
	}
	log.DebugF("加载评论用户数量: %d", len(c.userManager.AccountData[account].Users))
}

func (c *CommentActor) loadUsernameLocks(account string) {
	reservations := db.GetAllUsernameReservationsWithAccount(account)
	if reservations != nil {
		for username, reservation := range reservations {
			c.userManager.AccountData[account].Usernames[username] = reservation
		}
	}
	log.DebugF("加载用户名占用记录数量: %d", len(c.userManager.AccountData[account].Usernames))
}

func (c *CommentActor) loadCommentSessions(account string) {
	sessions := db.GetAllCommentSessionsWithAccount(account)
	if sessions != nil {
		for sessionID, session := range sessions {
			c.userManager.AccountData[account].Sessions[sessionID] = session
		}
	}
	log.DebugF("加载评论会话数量: %d", len(c.userManager.AccountData[account].Sessions))
}
