package statistics

import (
	"blog"
	"comment"
	"fmt"
	"module"
	log "mylog"
	"sort"
	"strings"
	"time"
)

func Info() {
	log.InfoF(log.ModuleStatistics, "info statistics v1.0")
}

// 统计数据结构
type Statistics struct {
	BlogStats    BlogStatistics    `json:"blog_stats"`
	AccessStats  AccessStatistics  `json:"access_stats"`
	EditStats    EditStatistics    `json:"edit_stats"`
	UserStats    UserStatistics    `json:"user_stats"`
	IPStats      IPStatistics      `json:"ip_stats"`
	CommentStats CommentStatistics `json:"comment_stats"`
	TagStats     TagStatistics     `json:"tag_stats"`
	SystemStats  SystemStatistics  `json:"system_stats"`
	TimeAnalysis TimeAnalysisData  `json:"time_analysis"`
	ContentStats ContentStatistics `json:"content_stats"`
}

type BlogStatistics struct {
	TotalBlogs    int `json:"total_blogs"`
	PublicBlogs   int `json:"public_blogs"`
	PrivateBlogs  int `json:"private_blogs"`
	EncryptBlogs  int `json:"encrypt_blogs"`
	TodayNewBlogs int `json:"today_new_blogs"`
	WeekNewBlogs  int `json:"week_new_blogs"`
	MonthNewBlogs int `json:"month_new_blogs"`
}

type AccessStatistics struct {
	TotalAccess       int64            `json:"total_access"`
	TodayAccess       int64            `json:"today_access"`
	WeekAccess        int64            `json:"week_access"`
	MonthAccess       int64            `json:"month_access"`
	AverageAccess     float64          `json:"average_access"`
	ZeroAccessBlogs   int              `json:"zero_access_blogs"`
	TopAccessedBlogs  []BlogAccessInfo `json:"top_accessed_blogs"`
	RecentAccessBlogs []BlogAccessInfo `json:"recent_access_blogs"`
}

type EditStatistics struct {
	TotalEdits        int64          `json:"total_edits"`
	TodayEdits        int64          `json:"today_edits"`
	WeekEdits         int64          `json:"week_edits"`
	MonthEdits        int64          `json:"month_edits"`
	AverageEdits      float64        `json:"average_edits"`
	NeverEditedBlogs  int            `json:"never_edited_blogs"`
	TopEditedBlogs    []BlogEditInfo `json:"top_edited_blogs"`
	RecentEditedBlogs []BlogEditInfo `json:"recent_edited_blogs"`
}

type UserStatistics struct {
	TotalLogins        int64   `json:"total_logins"`
	TodayLogins        int64   `json:"today_logins"`
	WeekLogins         int64   `json:"week_logins"`
	MonthLogins        int64   `json:"month_logins"`
	LastLoginTime      string  `json:"last_login_time"`
	AverageDailyLogins float64 `json:"average_daily_logins"`
}

type IPStatistics struct {
	UniqueVisitors      int      `json:"unique_visitors"`
	TodayUniqueVisitors int      `json:"today_unique_visitors"`
	TopActiveIPs        []IPInfo `json:"top_active_ips"`
	RecentAccessIPs     []IPInfo `json:"recent_access_ips"`
}

type CommentStatistics struct {
	TotalComments      int               `json:"total_comments"`
	BlogsWithComments  int               `json:"blogs_with_comments"`
	TodayNewComments   int               `json:"today_new_comments"`
	WeekNewComments    int               `json:"week_new_comments"`
	MonthNewComments   int               `json:"month_new_comments"`
	AverageComments    float64           `json:"average_comments"`
	TopCommentedBlogs  []BlogCommentInfo `json:"top_commented_blogs"`
	RecentComments     []CommentInfo     `json:"recent_comments"`
	ActiveCommentUsers []CommentUserInfo `json:"active_comment_users"`
}

type TagStatistics struct {
	TotalTags       int            `json:"total_tags"`
	PublicTags      int            `json:"public_tags"`
	HotTags         []TagInfo      `json:"hot_tags"`
	RecentUsedTags  []TagInfo      `json:"recent_used_tags"`
	TagDistribution map[string]int `json:"tag_distribution"`
}

type SystemStatistics struct {
	SystemUptime    string `json:"system_uptime"`
	DataSize        string `json:"data_size"`
	StaticFiles     int    `json:"static_files"`
	TemplateFiles   int    `json:"template_files"`
	TodayOperations int    `json:"today_operations"`
}

type TimeAnalysisData struct {
	CreationTimeDistribution map[string]int `json:"creation_time_distribution"`
	AccessHourDistribution   map[int]int    `json:"access_hour_distribution"`
	EditTimeDistribution     map[string]int `json:"edit_time_distribution"`
	ActiveTimeSlots          []TimeSlotInfo `json:"active_time_slots"`
}

type ContentStatistics struct {
	TotalCharacters      int64           `json:"total_characters"`
	AverageArticleLength float64         `json:"average_article_length"`
	EmptyContentBlogs    int             `json:"empty_content_blogs"`
	LongestArticle       BlogContentInfo `json:"longest_article"`
	ShortestArticle      BlogContentInfo `json:"shortest_article"`
}

// 辅助数据结构
type BlogAccessInfo struct {
	Title      string `json:"title"`
	AccessNum  int    `json:"access_num"`
	AccessTime string `json:"access_time"`
}

type BlogEditInfo struct {
	Title      string `json:"title"`
	ModifyNum  int    `json:"modify_num"`
	ModifyTime string `json:"modify_time"`
}

type IPInfo struct {
	IP          string `json:"ip"`
	AccessCount int    `json:"access_count"`
	LastAccess  string `json:"last_access"`
}

type BlogCommentInfo struct {
	Title        string `json:"title"`
	CommentCount int    `json:"comment_count"`
}

type CommentInfo struct {
	BlogTitle  string `json:"blog_title"`
	Owner      string `json:"owner"`
	Msg        string `json:"msg"`
	CreateTime string `json:"create_time"`
}

type CommentUserInfo struct {
	Owner        string `json:"owner"`
	CommentCount int    `json:"comment_count"`
}

type TagInfo struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type TimeSlotInfo struct {
	TimeSlot string `json:"time_slot"`
	Activity int    `json:"activity"`
}

type BlogContentInfo struct {
	Title  string `json:"title"`
	Length int    `json:"length"`
}

// 访问记录存储
var accessRecords = make(map[string][]AccessRecord)
var loginRecords = make([]LoginRecord, 0)
var ipRecords = make(map[string]*IPRecord)

type AccessRecord struct {
	BlogTitle string `json:"blog_title"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Timestamp string `json:"timestamp"`
}

type LoginRecord struct {
	Account   string `json:"account"`
	IP        string `json:"ip"`
	Timestamp string `json:"timestamp"`
	Success   bool   `json:"success"`
}

type IPRecord struct {
	IP          string `json:"ip"`
	AccessCount int    `json:"access_count"`
	FirstAccess string `json:"first_access"`
	LastAccess  string `json:"last_access"`
	UserAgent   string `json:"user_agent"`
}

// 统计数据缓存
var cachedStats *Statistics
var lastCacheTime time.Time
var cacheExpiry = 5 * time.Minute

// 初始化统计模块
func Init() {
	log.Debug(log.ModuleStatistics, "statistics module Init")
}

// 记录博客访问
func RecordBlogAccess(blogTitle, ip, userAgent string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	record := AccessRecord{
		BlogTitle: blogTitle,
		IP:        ip,
		UserAgent: userAgent,
		Timestamp: timestamp,
	}

	if accessRecords[blogTitle] == nil {
		accessRecords[blogTitle] = make([]AccessRecord, 0)
	}
	accessRecords[blogTitle] = append(accessRecords[blogTitle], record)

	if ipRecords[ip] == nil {
		ipRecords[ip] = &IPRecord{
			IP:          ip,
			AccessCount: 0,
			FirstAccess: timestamp,
			UserAgent:   userAgent,
		}
	}
	ipRecords[ip].AccessCount++
	ipRecords[ip].LastAccess = timestamp

	log.DebugF(log.ModuleStatistics, "记录博客访问: %s from %s", blogTitle, ip)
}

// 记录用户登录
func RecordUserLogin(account, ip string, success bool) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	record := LoginRecord{
		Account:   account,
		IP:        ip,
		Timestamp: timestamp,
		Success:   success,
	}

	loginRecords = append(loginRecords, record)

	log.DebugF(log.ModuleStatistics, "记录用户登录: %s from %s, success: %v", account, ip, success)
}

// 获取统计数据
func GetStatistics(account string) *Statistics {
	if cachedStats != nil && time.Since(lastCacheTime) < cacheExpiry {
		return cachedStats
	}

	log.Debug(log.ModuleStatistics, "生成新的统计数据")

	stats := &Statistics{}

	stats.BlogStats = calculateBlogStatistics(account)
	stats.AccessStats = calculateAccessStatistics(account)
	stats.EditStats = calculateEditStatistics(account)
	stats.UserStats = calculateUserStatistics(account)
	stats.IPStats = calculateIPStatistics(account)
	stats.CommentStats = calculateCommentStatistics(account)
	stats.TagStats = calculateTagStatistics(account)
	stats.SystemStats = calculateSystemStatistics(account)
	stats.TimeAnalysis = calculateTimeAnalysis(account)
	stats.ContentStats = calculateContentStatistics(account)

	cachedStats = stats
	lastCacheTime = time.Now()

	return stats
}

// GetOverallStatistics is an alias for GetStatistics for API compatibility
func GetOverallStatistics(account string) *Statistics {
	return GetStatistics(account)
}

// 计算博客统计
func calculateBlogStatistics(account string) BlogStatistics {
	stats := BlogStatistics{}

	blogs := blog.GetBlogsWithAccount(account)
	stats.TotalBlogs = len(blogs)

	now := time.Now()
	today := now.Format("2006-01-02")
	weekStart := now.AddDate(0, 0, -int(now.Weekday())).Format("2006-01-02")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")

	for _, b := range blogs {
		if (b.AuthType & module.EAuthType_public) != 0 {
			stats.PublicBlogs++
		}
		if (b.AuthType & module.EAuthType_private) != 0 {
			stats.PrivateBlogs++
		}
		if (b.AuthType & module.EAuthType_encrypt) != 0 {
			stats.EncryptBlogs++
		}

		createDate := strings.Split(b.CreateTime, " ")[0]
		if createDate == today {
			stats.TodayNewBlogs++
		}
		if createDate >= weekStart {
			stats.WeekNewBlogs++
		}
		if createDate >= monthStart {
			stats.MonthNewBlogs++
		}
	}

	return stats
}

// 计算访问统计
func calculateAccessStatistics(account string) AccessStatistics {
	stats := AccessStatistics{}

	blogs := blog.GetBlogsWithAccount(account)
	var totalAccess int64

	topBlogs := make([]BlogAccessInfo, 0)
	recentBlogs := make([]BlogAccessInfo, 0)

	for _, b := range blogs {
		totalAccess += int64(b.AccessNum)

		if b.AccessNum == 0 {
			stats.ZeroAccessBlogs++
		}

		topBlogs = append(topBlogs, BlogAccessInfo{
			Title:      b.Title,
			AccessNum:  b.AccessNum,
			AccessTime: b.AccessTime,
		})

		recentBlogs = append(recentBlogs, BlogAccessInfo{
			Title:      b.Title,
			AccessNum:  b.AccessNum,
			AccessTime: b.AccessTime,
		})
	}

	stats.TotalAccess = totalAccess
	if len(blogs) > 0 {
		stats.AverageAccess = float64(totalAccess) / float64(len(blogs))
	}

	// 排序热门博客
	sort.Slice(topBlogs, func(i, j int) bool {
		return topBlogs[i].AccessNum > topBlogs[j].AccessNum
	})
	if len(topBlogs) > 10 {
		stats.TopAccessedBlogs = topBlogs[:10]
	} else {
		stats.TopAccessedBlogs = topBlogs
	}

	// 排序最近访问博客
	sort.Slice(recentBlogs, func(i, j int) bool {
		ti, _ := time.Parse("2006-01-02 15:04:05", recentBlogs[i].AccessTime)
		tj, _ := time.Parse("2006-01-02 15:04:05", recentBlogs[j].AccessTime)
		return ti.Unix() > tj.Unix()
	})
	if len(recentBlogs) > 10 {
		stats.RecentAccessBlogs = recentBlogs[:10]
	} else {
		stats.RecentAccessBlogs = recentBlogs
	}

	return stats
}

// 计算编辑统计
func calculateEditStatistics(account string) EditStatistics {
	stats := EditStatistics{}

	blogs := blog.GetBlogsWithAccount(account)
	var totalEdits int64

	topBlogs := make([]BlogEditInfo, 0)
	recentBlogs := make([]BlogEditInfo, 0)

	for _, b := range blogs {
		totalEdits += int64(b.ModifyNum)

		if b.ModifyNum == 0 {
			stats.NeverEditedBlogs++
		}

		topBlogs = append(topBlogs, BlogEditInfo{
			Title:      b.Title,
			ModifyNum:  b.ModifyNum,
			ModifyTime: b.ModifyTime,
		})

		recentBlogs = append(recentBlogs, BlogEditInfo{
			Title:      b.Title,
			ModifyNum:  b.ModifyNum,
			ModifyTime: b.ModifyTime,
		})
	}

	stats.TotalEdits = totalEdits
	if len(blogs) > 0 {
		stats.AverageEdits = float64(totalEdits) / float64(len(blogs))
	}

	// 排序
	sort.Slice(topBlogs, func(i, j int) bool {
		return topBlogs[i].ModifyNum > topBlogs[j].ModifyNum
	})
	if len(topBlogs) > 10 {
		stats.TopEditedBlogs = topBlogs[:10]
	} else {
		stats.TopEditedBlogs = topBlogs
	}

	sort.Slice(recentBlogs, func(i, j int) bool {
		ti, _ := time.Parse("2006-01-02 15:04:05", recentBlogs[i].ModifyTime)
		tj, _ := time.Parse("2006-01-02 15:04:05", recentBlogs[j].ModifyTime)
		return ti.Unix() > tj.Unix()
	})
	if len(recentBlogs) > 10 {
		stats.RecentEditedBlogs = recentBlogs[:10]
	} else {
		stats.RecentEditedBlogs = recentBlogs
	}

	return stats
}

// 计算用户统计
func calculateUserStatistics(account string) UserStatistics {
	stats := UserStatistics{}

	var totalLogins int64
	var lastLoginTime string

	for _, record := range loginRecords {
		if record.Success {
			totalLogins++
			lastLoginTime = record.Timestamp
		}
	}

	stats.TotalLogins = totalLogins
	stats.LastLoginTime = lastLoginTime
	stats.AverageDailyLogins = float64(totalLogins) / 30.0

	return stats
}

// 计算IP统计
func calculateIPStatistics(account string) IPStatistics {
	stats := IPStatistics{}

	stats.UniqueVisitors = len(ipRecords)

	topIPs := make([]IPInfo, 0)
	recentIPs := make([]IPInfo, 0)

	for ip, record := range ipRecords {
		ipInfo := IPInfo{
			IP:          ip,
			AccessCount: record.AccessCount,
			LastAccess:  record.LastAccess,
		}

		topIPs = append(topIPs, ipInfo)
		recentIPs = append(recentIPs, ipInfo)
	}

	// 排序
	sort.Slice(topIPs, func(i, j int) bool {
		return topIPs[i].AccessCount > topIPs[j].AccessCount
	})
	if len(topIPs) > 10 {
		stats.TopActiveIPs = topIPs[:10]
	} else {
		stats.TopActiveIPs = topIPs
	}

	sort.Slice(recentIPs, func(i, j int) bool {
		ti, _ := time.Parse("2006-01-02 15:04:05", recentIPs[i].LastAccess)
		tj, _ := time.Parse("2006-01-02 15:04:05", recentIPs[j].LastAccess)
		return ti.Unix() > tj.Unix()
	})
	if len(recentIPs) > 50 {
		stats.RecentAccessIPs = recentIPs[:50]
	} else {
		stats.RecentAccessIPs = recentIPs
	}

	return stats
}

// 计算评论统计
func calculateCommentStatistics(account string) CommentStatistics {
	stats := CommentStatistics{}

	comments := comment.GetAllComments(account)
	var totalComments int
	blogsWithComments := 0

	topCommentedBlogs := make([]BlogCommentInfo, 0)
	recentComments := make([]CommentInfo, 0)
	userCommentCount := make(map[string]int)

	for title, blogComments := range comments {
		commentCount := len(blogComments.Comments)
		totalComments += commentCount

		if commentCount > 0 {
			blogsWithComments++

			topCommentedBlogs = append(topCommentedBlogs, BlogCommentInfo{
				Title:        title,
				CommentCount: commentCount,
			})
		}

		for _, c := range blogComments.Comments {
			recentComments = append(recentComments, CommentInfo{
				BlogTitle:  title,
				Owner:      c.Owner,
				Msg:        c.Msg,
				CreateTime: c.CreateTime,
			})

			userCommentCount[c.Owner]++
		}
	}

	stats.TotalComments = totalComments
	stats.BlogsWithComments = blogsWithComments

	if len(blog.GetBlogsWithAccount(account)) > 0 {
		stats.AverageComments = float64(totalComments) / float64(len(blog.GetBlogsWithAccount(account)))
	}

	// 排序热门评论博客
	sort.Slice(topCommentedBlogs, func(i, j int) bool {
		return topCommentedBlogs[i].CommentCount > topCommentedBlogs[j].CommentCount
	})
	if len(topCommentedBlogs) > 10 {
		stats.TopCommentedBlogs = topCommentedBlogs[:10]
	} else {
		stats.TopCommentedBlogs = topCommentedBlogs
	}

	// 排序最近评论
	sort.Slice(recentComments, func(i, j int) bool {
		ti, _ := time.Parse("2006-01-02 15:04:05", recentComments[i].CreateTime)
		tj, _ := time.Parse("2006-01-02 15:04:05", recentComments[j].CreateTime)
		return ti.Unix() > tj.Unix()
	})
	if len(recentComments) > 20 {
		stats.RecentComments = recentComments[:20]
	} else {
		stats.RecentComments = recentComments
	}

	// 活跃评论用户
	activeUsers := make([]CommentUserInfo, 0)
	for user, count := range userCommentCount {
		activeUsers = append(activeUsers, CommentUserInfo{
			Owner:        user,
			CommentCount: count,
		})
	}
	sort.Slice(activeUsers, func(i, j int) bool {
		return activeUsers[i].CommentCount > activeUsers[j].CommentCount
	})
	if len(activeUsers) > 10 {
		stats.ActiveCommentUsers = activeUsers[:10]
	} else {
		stats.ActiveCommentUsers = activeUsers
	}

	return stats
}

// 计算标签统计
func calculateTagStatistics(account string) TagStatistics {
	stats := TagStatistics{}

	tagCount := make(map[string]int)

	blogs := blog.GetBlogsWithAccount(account)
	for _, b := range blogs {
		if b.Tags != "" {
			tags := strings.Split(b.Tags, "|")
			for _, tag := range tags {
				if tag != "" {
					tagCount[tag]++
				}
			}
		}
	}

	stats.TotalTags = len(tagCount)
	stats.TagDistribution = tagCount

	// 热门标签
	hotTags := make([]TagInfo, 0)
	for tag, count := range tagCount {
		hotTags = append(hotTags, TagInfo{
			Tag:   tag,
			Count: count,
		})
	}
	sort.Slice(hotTags, func(i, j int) bool {
		return hotTags[i].Count > hotTags[j].Count
	})
	if len(hotTags) > 20 {
		stats.HotTags = hotTags[:20]
	} else {
		stats.HotTags = hotTags
	}

	stats.RecentUsedTags = hotTags[:min(len(hotTags), 10)]

	return stats
}

// 计算系统统计
func calculateSystemStatistics(account string) SystemStatistics {
	stats := SystemStatistics{}

	stats.SystemUptime = "系统运行中"
	stats.DataSize = fmt.Sprintf("博客: %d, 评论: %d", len(blog.GetBlogsWithAccount(account)), len(comment.GetAllComments(account)))
	stats.StaticFiles = 100
	stats.TemplateFiles = 20
	stats.TodayOperations = len(accessRecords)

	return stats
}

// 计算时间分析
func calculateTimeAnalysis(account string) TimeAnalysisData {
	analysis := TimeAnalysisData{}

	creationDist := make(map[string]int)
	accessHourDist := make(map[int]int)
	editDist := make(map[string]int)

	blogs := blog.GetBlogsWithAccount(account)
	for _, b := range blogs {
		if createTime, err := time.Parse("2006-01-02 15:04:05", b.CreateTime); err == nil {
			monthKey := createTime.Format("2006-01")
			creationDist[monthKey]++
		}

		if accessTime, err := time.Parse("2006-01-02 15:04:05", b.AccessTime); err == nil {
			accessHourDist[accessTime.Hour()]++
		}

		if modifyTime, err := time.Parse("2006-01-02 15:04:05", b.ModifyTime); err == nil {
			monthKey := modifyTime.Format("2006-01")
			editDist[monthKey]++
		}
	}

	analysis.CreationTimeDistribution = creationDist
	analysis.AccessHourDistribution = accessHourDist
	analysis.EditTimeDistribution = editDist

	activeSlots := make([]TimeSlotInfo, 0)
	for hour, count := range accessHourDist {
		slot := fmt.Sprintf("%02d:00-%02d:59", hour, hour)
		activeSlots = append(activeSlots, TimeSlotInfo{
			TimeSlot: slot,
			Activity: count,
		})
	}
	sort.Slice(activeSlots, func(i, j int) bool {
		return activeSlots[i].Activity > activeSlots[j].Activity
	})
	analysis.ActiveTimeSlots = activeSlots

	return analysis
}

// 计算内容统计
func calculateContentStatistics(account string) ContentStatistics {
	stats := ContentStatistics{}

	var totalCharacters int64
	var longestBlog, shortestBlog BlogContentInfo
	longestLength := 0
	shortestLength := int(^uint(0) >> 1)
	emptyBlogs := 0

	blogs := blog.GetBlogsWithAccount(account)
	for _, b := range blogs {
		length := len([]rune(b.Content))
		totalCharacters += int64(length)

		if length == 0 {
			emptyBlogs++
		}

		if length > longestLength {
			longestLength = length
			longestBlog = BlogContentInfo{
				Title:  b.Title,
				Length: length,
			}
		}

		if length < shortestLength {
			shortestLength = length
			shortestBlog = BlogContentInfo{
				Title:  b.Title,
				Length: length,
			}
		}
	}

	stats.TotalCharacters = totalCharacters
	if len(blogs) > 0 {
		stats.AverageArticleLength = float64(totalCharacters) / float64(len(blogs))
	}
	stats.LongestArticle = longestBlog
	stats.ShortestArticle = shortestBlog
	stats.EmptyContentBlogs = emptyBlogs

	return stats
}

// 清除缓存
func ClearCache() {
	cachedStats = nil
	log.Debug(log.ModuleStatistics, "统计缓存已清除")
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
