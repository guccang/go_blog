package comment

import (
	"module"
	db "persistence"
	log "mylog"
	"time"
	"fmt"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"regexp"
	"errors"
)

// 评论用户管理器
type CommentUserManager struct {
	Users       map[string]*module.CommentUser      // UserID -> User
	Sessions    map[string]*module.CommentSession   // SessionID -> Session
	Usernames   map[string]*module.UsernameReservation // Username -> Reservation
}

var UserManager *CommentUserManager

func InitUserManager() {
	UserManager = &CommentUserManager{
		Users:     make(map[string]*module.CommentUser),
		Sessions:  make(map[string]*module.CommentSession),
		Usernames: make(map[string]*module.UsernameReservation),
	}
	
	// 加载已存在的用户数据
	loadCommentUsers()
	loadUsernameLocks()
	loadCommentSessions()
	
	log.Debug("CommentUserManager initialized")
}

// 生成唯一用户ID
func generateUserID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("user_%s", hex.EncodeToString(bytes)[:16])
}

// 生成会话ID
func generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("session_%s", hex.EncodeToString(bytes)[:24])
}

// 验证用户名格式
func validateUsername(username string) error {
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

// 检查用户名是否可用
func (cum *CommentUserManager) IsUsernameAvailable(username string) bool {
	users := cum.GetUsersByUsername(username)
	return len(users) == 0
}

// 通过用户名获取所有相关用户（可能有多个相同用户名的用户）
func (cum *CommentUserManager) GetUsersByUsername(username string) []*module.CommentUser {
	var users []*module.CommentUser
	for _, user := range cum.Users {
		if user.Username == username {
			users = append(users, user)
		}
	}
	return users
}

// 临时占用用户名（未注册用户）
func (cum *CommentUserManager) ReserveUsernameTemporary(username, userID string) error {
	if err := validateUsername(username); err != nil {
		return err
	}
	
	if !cum.IsUsernameAvailable(username) {
		return errors.New("用户名已被占用")
	}
	
	reservation := &module.UsernameReservation{
		Username:    username,
		UserID:      userID,
		ReserveTime: time.Now().Format("2006-01-02 15:04:05"),
		IsTemporary: true,
	}
	
	cum.Usernames[username] = reservation
	saveUsernameReservation(reservation)
	
	log.DebugF("临时占用用户名: %s by %s", username, userID)
	return nil
}

// 永久注册用户名
func (cum *CommentUserManager) RegisterUsername(username, userID string) error {
	if err := validateUsername(username); err != nil {
		return err
	}
	
	// 检查是否已被他人占用
	if reservation, exists := cum.Usernames[username]; exists {
		if reservation.UserID != userID {
			return errors.New("用户名已被其他用户占用")
		}
	}
	
	reservation := &module.UsernameReservation{
		Username:    username,
		UserID:      userID,
		ReserveTime: time.Now().Format("2006-01-02 15:04:05"),
		IsTemporary: false,
	}
	
	cum.Usernames[username] = reservation
	saveUsernameReservation(reservation)
	
	log.DebugF("永久注册用户名: %s by %s", username, userID)
	return nil
}

// 创建匿名用户会话
func (cum *CommentUserManager) CreateAnonymousSession(username, email, ip, userAgent string) (*module.CommentSession, error) {
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	
	// 检查用户名是否已存在
	existingUsers := cum.GetUsersByUsername(username)
	if len(existingUsers) > 0 {
		return nil, errors.New("该用户名已被注册，请输入密码进行身份验证")
	}
	
	userID := generateUserID()
	sessionID := generateSessionID()
	
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
	cum.Users[userID] = user
	
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
	cum.Sessions[sessionID] = session
	
	// 保存到数据库
	saveCommentUser(user)
	saveCommentSession(session)
	
	log.DebugF("创建匿名用户会话: %s (%s)", username, userID)
	return session, nil
}

// 验证会话
func (cum *CommentUserManager) ValidateSession(sessionID string) (*module.CommentUser, error) {
	session, exists := cum.Sessions[sessionID]
	if !exists {
		return nil, errors.New("会话不存在")
	}
	
	// 检查会话是否过期
	expireTime, err := time.Parse("2006-01-02 15:04:05", session.ExpireTime)
	if err != nil || time.Now().After(expireTime) {
		// 会话过期，删除
		delete(cum.Sessions, sessionID)
		return nil, errors.New("会话已过期")
	}
	
	// 获取用户信息
	user, exists := cum.Users[session.UserID]
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
	saveCommentUser(user)
	
	return user, nil
}

// 获取用户ID通过用户名
func (cum *CommentUserManager) GetUserIDByUsername(username string) string {
	reservation, exists := cum.Usernames[username]
	if exists {
		return reservation.UserID
	}
	return ""
}

// 检查用户是否可以评论
func (cum *CommentUserManager) CanUserComment(userID string) (bool, string) {
	user, exists := cum.Users[userID]
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

// 更新用户评论计数
func (cum *CommentUserManager) IncrementUserCommentCount(userID string) {
	user, exists := cum.Users[userID]
	if exists {
		user.CommentCount++
		user.LastActive = time.Now().Format("2006-01-02 15:04:05")
		
		// 根据评论数量增加信誉
		if user.CommentCount%10 == 0 {
			user.Reputation += 5
		} else {
			user.Reputation += 1
		}
		
		saveCommentUser(user)
		log.DebugF("用户 %s 评论计数更新: %d", user.Username, user.CommentCount)
	}
}

// 数据持久化函数（需要在persistence包中实现）
func saveCommentUser(user *module.CommentUser) {
	db.SaveCommentUser(user)
}

func saveCommentSession(session *module.CommentSession) {
	db.SaveCommentSession(session)
}

func saveUsernameReservation(reservation *module.UsernameReservation) {
	db.SaveUsernameReservation(reservation)
}

func loadCommentUsers() {
	users := db.GetAllCommentUsers()
	if users != nil {
		for userID, user := range users {
			UserManager.Users[userID] = user
		}
	}
	log.DebugF("加载评论用户数量: %d", len(UserManager.Users))
}

func loadUsernameLocks() {
	reservations := db.GetAllUsernameReservations()
	if reservations != nil {
		for username, reservation := range reservations {
			UserManager.Usernames[username] = reservation
		}
	}
	log.DebugF("加载用户名占用记录数量: %d", len(UserManager.Usernames))
}

func loadCommentSessions() {
	sessions := db.GetAllCommentSessions()
	if sessions != nil {
		for sessionID, session := range sessions {
			UserManager.Sessions[sessionID] = session
		}
	}
	log.DebugF("加载评论会话数量: %d", len(UserManager.Sessions))
}

// 清理过期会话和临时用户名占用
func (cum *CommentUserManager) CleanupExpiredData() {
	now := time.Now()
	
	// 清理过期会话
	for sessionID, session := range cum.Sessions {
		expireTime, err := time.Parse("2006-01-02 15:04:05", session.ExpireTime)
		if err != nil || now.After(expireTime) {
			delete(cum.Sessions, sessionID)
			db.DeleteCommentSession(sessionID)
		}
	}
	
	// 清理过期的临时用户名占用
	for username, reservation := range cum.Usernames {
		if reservation.IsTemporary {
			reserveTime, err := time.Parse("2006-01-02 15:04:05", reservation.ReserveTime)
			if err == nil && now.Sub(reserveTime) > 24*time.Hour {
				delete(cum.Usernames, username)
				db.DeleteUsernameReservation(username)
			}
		}
	}
	
	log.Debug("清理过期数据完成")
}

// 验证用户身份（通过密码）
func (cum *CommentUserManager) AuthenticateUser(username, password string) (*module.CommentUser, error) {
	users := cum.GetUsersByUsername(username)
	
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

// 创建或验证用户会话
func (cum *CommentUserManager) CreateOrAuthenticateSession(username, email, password, ip, userAgent string) (*module.CommentSession, *module.CommentUser, error) {
	if err := validateUsername(username); err != nil {
		return nil, nil, err
	}
	
	// 检查用户名是否已被使用
	existingUsers := cum.GetUsersByUsername(username)
	
	if len(existingUsers) > 0 {
		// 用户名已存在，必须提供正确的密码
		if password == "" {
			return nil, nil, errors.New("该用户名已被注册，请输入密码进行身份验证")
		}
		
		// 验证密码
		user, err := cum.AuthenticateUser(username, password)
		if err != nil {
			return nil, nil, errors.New("密码错误，无法使用该用户名")
		}
		
		// 验证成功，创建新会话
		sessionID := generateSessionID()
		session := &module.CommentSession{
			SessionID:  sessionID,
			UserID:     user.UserID,
			IP:         ip,
			UserAgent:  userAgent,
			CreateTime: time.Now().Format("2006-01-02 15:04:05"),
			ExpireTime: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
			IsActive:   true,
		}
		cum.Sessions[sessionID] = session
		saveCommentSession(session)
		
		log.DebugF("用户身份验证成功，创建新会话: %s (%s)", username, user.UserID)
		return session, user, nil
	}
	
	// 新用户名，创建新用户
	userID := generateUserID()
	sessionID := generateSessionID()
	
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
	
	cum.Users[userID] = user
	
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
	cum.Sessions[sessionID] = session
	
	// 保存到数据库
	saveCommentUser(user)
	saveCommentSession(session)
	
	log.DebugF("创建新用户会话: %s (%s)", username, userID)
	return session, user, nil
} 