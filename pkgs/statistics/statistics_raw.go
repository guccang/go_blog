package statistics

import (
	"blog"
	"comment"
	"cooperation"
	"exercise"
	"fmt"
	log "mylog"
	"sort"
	"strings"
	"time"
)

// =================================== MCP Interface - Raw接口 =========================================

// 获取当前日期
func RawCurrentDate() string {
	return time.Now().Format("2006-01-02")
}

// 获取当前时间
func RawCurrentTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// 获取日记数量
func RawAllDiaryCount() int {
	count := 0
	for _, b := range blog.Blogs {
		if strings.Contains(b.Title, "日记_") {
			count++
		}
	}
	return count
}

// 获取所有日志内容
func RawAllDiaryContent() string {
	content := ""
	for _, b := range blog.Blogs {
		if strings.Contains(b.Title, "日记_") && b.Content != "" {
			content += b.Title + ":\n" + b.Content + "\n"
		}
	}
	return content
}

// 获取指定博客内容，通过博客名称匹配
func RawGetBlogByTitleMatch(match string) string {
	content := ""
	for _, b := range blog.Blogs {
		if strings.Contains(b.Title, match) && b.Content != "" {
			content += b.Title + ":\n" + b.Content + "\n"
		}
	}
	return content
}

// 获取锻炼总次数
func RawAllExerciseCount() int {
	count := 0
	for _, b := range blog.Blogs {
		if strings.Contains(b.Title, "锻炼_") {
			count++
		}
	}
	return count
}

// 获取锻炼总时间分钟
func RawAllExerciseTotalMinutes() int {
	time := 0
	all, _ := exercise.GetAllExercises()
	for _, e := range all {
		for _, item := range e.Items {
			time += item.Duration
		}
	}
	return time
}

// 获取锻炼总距离
func RawAllExerciseDistance() int {
	distance := 0
	all, _ := exercise.GetAllExercises()
	for _, e := range all {
		for _, item := range e.Items {
			distance += item.Calories
		}
	}
	return distance
}

// 获取锻炼总卡路里
func RawAllExerciseCalories() int {
	calories := 0
	all, _ := exercise.GetAllExercises()
	for _, e := range all {
		for _, item := range e.Items {
			calories += item.Calories
		}
	}
	return calories
}

// 获取博客总数量
func RawAllBlogCount() int {
	log.DebugF("RawAllBlogCount: %d", len(blog.Blogs))
	return len(blog.Blogs)
}

// 获取所有blog名称,以空格分割
func RawAllBlogData() string {
	blogs := blog.Blogs
	blogNames := make([]string, 0)
	for _, b := range blogs {
		blogNames = append(blogNames, b.Title)
	}
	return strings.Join(blogNames, " ")
}

// 通过名称获取blog内容
func RawGetBlogData(title string) string {
	blog := blog.GetBlog(title)
	log.DebugF("RawBlogData: %s, blog: %v", title, blog)
	if blog != nil {
		return blog.Content
	}
	return ""
}

// 获取所有comment
func RawAllCommentData() string {
	comments := comment.Comments
	commentData := make([]string, 0)
	for _, c := range comments {
		commentData = append(commentData, c.Title)
	}
	return strings.Join(commentData, " ")
}

// 通过名称获取comment
func RawCommentData(title string) string {
	comments := comment.GetComments(title)
	if comments != nil {
		msg := ""
		for _, c := range comments.Comments {
			msg += c.Msg + "\n"
		}
		return msg
	}
	return ""
}

// 获取所有cooperation
func RawAllCooperationData() string {
	cooperations := cooperation.Cooperations
	cooperationData := make([]string, 0)
	for _, c := range cooperations {
		cooperationData = append(cooperationData, c.Account)
	}
	return strings.Join(cooperationData, " ")
}

// 根据日期获取所有Blog
func RawAllBlogDataByDate(date string) string {
	blogs := blog.Blogs
	blogData := make([]string, 0)
	for _, b := range blogs {
		// CreateTime 2006-01-02 15:04:05
		// date 2006-01-02
		createTime, err := time.Parse("2006-01-02 15:04:05", b.CreateTime)
		if err != nil {
			continue
		}
		if createTime.Format("2006-01-02") == date {
			blogData = append(blogData, b.Title)
		}
	}
	if len(blogData) == 0 {
		return "Error NOT find blog: " + date
	}
	return strings.Join(blogData, " ")

}

// 根据日期范围获取所有Blog
func RawAllBlogDataByDateRange(startDate, endDate string) string {
	blogs := blog.Blogs
	blogData := make([]string, 0)
	for _, b := range blogs {
		// 使用时间对比
		createTime, err := time.Parse("2006-01-02 15:04:05", b.CreateTime)
		if err != nil {
			continue
		}
		start, err := time.Parse("2006-01-02 15:04:05", startDate+" 00:00:00")
		if err != nil {
			continue
		}
		end, err := time.Parse("2006-01-02 15:04:05", endDate+" 23:59:59")
		if err != nil {
			continue
		}
		if createTime.After(start) && createTime.Before(end) {
			blogData = append(blogData, b.Title)
		}
	}
	if len(blogData) == 0 {
		return "Error NOT find blog: " + startDate + " to " + endDate
	}
	return strings.Join(blogData, " ")
}

// 根据日期范围获取所有Blog数量
func RawAllBlogDataByDateRangeCount(startDate, endDate string) int {
	blogs := blog.Blogs
	count := 0
	for _, b := range blogs {
		createTime, err := time.Parse("2006-01-02 15:04:05", b.CreateTime)
		if err != nil {
			continue
		}
		start, err := time.Parse("2006-01-02 15:04:05", startDate+" 00:00:00")
		if err != nil {
			continue
		}
		end, err := time.Parse("2006-01-02 15:04:05", endDate+" 23:59:59")
		if err != nil {
			continue
		}
		if createTime.After(start) && createTime.Before(end) {
			count++
		}
	}

	return count
}

// 获取指定日期Blog数量
func RawGetBlogDataByDate(date string) string {
	blogs := blog.Blogs
	blogData := make([]string, 0)
	for _, b := range blogs {
		// 使用时间对比
		createTime, err := time.Parse("2006-01-02 15:04:05", b.CreateTime)
		if err != nil {
			continue
		}
		if createTime.Format("2006-01-02") == date {
			blogData = append(blogData, b.Title)
		}
	}
	if len(blogData) == 0 {
		return "Error NOT find blog: " + date
	}
	return strings.Join(blogData, " ")
}

// =================================== 扩展Raw接口 =========================================

// 获取博客详细统计信息
func RawBlogStatistics() string {
	stats := calculateBlogStatistics()
	result := fmt.Sprintf("总博客数:%d,公开:%d,私有:%d,加密:%d,协作:%d,今日新增:%d,本周新增:%d,本月新增:%d",
		stats.TotalBlogs, stats.PublicBlogs, stats.PrivateBlogs, stats.EncryptBlogs,
		stats.CooperationBlogs, stats.TodayNewBlogs, stats.WeekNewBlogs, stats.MonthNewBlogs)
	return result
}

// 获取访问统计信息
func RawAccessStatistics() string {
	stats := calculateAccessStatistics()
	result := fmt.Sprintf("总访问量:%d,今日访问:%d,本周访问:%d,本月访问:%d,平均访问:%.2f,零访问博客数:%d",
		stats.TotalAccess, stats.TodayAccess, stats.WeekAccess, stats.MonthAccess,
		stats.AverageAccess, stats.ZeroAccessBlogs)
	return result
}

// 获取热门博客列表(前10)
func RawTopAccessedBlogs() string {
	stats := calculateAccessStatistics()
	result := "热门博客TOP10:\n"
	for i, blog := range stats.TopAccessedBlogs {
		if i >= 10 {
			break
		}
		result += fmt.Sprintf("%d. %s (访问:%d次, 最后访问:%s)\n",
			i+1, blog.Title, blog.AccessNum, blog.AccessTime)
	}
	return result
}

// 获取最近访问的博客列表
func RawRecentAccessedBlogs() string {
	stats := calculateAccessStatistics()
	result := "最近访问博客:\n"
	for i, blog := range stats.RecentAccessBlogs {
		if i >= 10 {
			break
		}
		result += fmt.Sprintf("- %s (访问:%d次, 最后访问:%s)\n",
			blog.Title, blog.AccessNum, blog.AccessTime)
	}
	return result
}

// 获取编辑统计信息
func RawEditStatistics() string {
	stats := calculateEditStatistics()
	result := fmt.Sprintf("总编辑次数:%d,今日编辑:%d,本周编辑:%d,本月编辑:%d,平均编辑:%.2f,从未编辑博客数:%d",
		stats.TotalEdits, stats.TodayEdits, stats.WeekEdits, stats.MonthEdits,
		stats.AverageEdits, stats.NeverEditedBlogs)
	return result
}

// 获取标签统计信息
func RawTagStatistics() string {
	stats := calculateTagStatistics()
	result := fmt.Sprintf("标签总数:%d,公开标签:%d\n热门标签:\n", stats.TotalTags, stats.PublicTags)
	for i, tag := range stats.HotTags {
		if i >= 10 {
			break
		}
		result += fmt.Sprintf("- %s (%d次)\n", tag.Tag, tag.Count)
	}
	return result
}

// 获取评论统计信息
func RawCommentStatistics() string {
	stats := calculateCommentStatistics()
	result := fmt.Sprintf("评论总数:%d,有评论博客数:%d,今日新评论:%d,本周新评论:%d,本月新评论:%d,平均评论:%.2f",
		stats.TotalComments, stats.BlogsWithComments, stats.TodayNewComments,
		stats.WeekNewComments, stats.MonthNewComments, stats.AverageComments)
	return result
}

// 获取内容统计信息
func RawContentStatistics() string {
	stats := calculateContentStatistics()
	result := fmt.Sprintf("总字符数:%d,平均文章长度:%.2f,空内容博客数:%d\n最长文章:%s(%d字)\n最短文章:%s(%d字)",
		stats.TotalCharacters, stats.AverageArticleLength, stats.EmptyContentBlogs,
		stats.LongestArticle.Title, stats.LongestArticle.Length,
		stats.ShortestArticle.Title, stats.ShortestArticle.Length)
	return result
}

// 按权限类型获取博客列表
func RawBlogsByAuthType(authType int) string {
	blogs := blog.Blogs
	blogNames := make([]string, 0)
	for _, b := range blogs {
		if (b.AuthType & authType) != 0 {
			blogNames = append(blogNames, b.Title)
		}
	}
	if len(blogNames) == 0 {
		return fmt.Sprintf("未找到权限类型为%d的博客", authType)
	}
	return strings.Join(blogNames, " ")
}

// 按标签获取博客列表
func RawBlogsByTag(tag string) string {
	blogs := blog.Blogs
	blogNames := make([]string, 0)
	for _, b := range blogs {
		if strings.Contains(b.Tags, tag) {
			blogNames = append(blogNames, b.Title)
		}
	}
	if len(blogNames) == 0 {
		return fmt.Sprintf("未找到标签为%s的博客", tag)
	}
	return strings.Join(blogNames, " ")
}

// 获取博客元数据(不包含内容)
func RawBlogMetadata(title string) string {
	b := blog.GetBlog(title)
	if b == nil {
		return "博客不存在: " + title
	}
	result := fmt.Sprintf("标题:%s\n创建时间:%s\n修改时间:%s\n访问时间:%s\n修改次数:%d\n访问次数:%d\n权限类型:%d\n标签:%s\n是否加密:%d",
		b.Title, b.CreateTime, b.ModifyTime, b.AccessTime, b.ModifyNum, b.AccessNum, b.AuthType, b.Tags, b.Encrypt)
	return result
}

// 获取近期活跃博客(近7天有访问或修改)
func RawRecentActiveBlog() string {
	blogs := blog.Blogs
	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -7)
	
	activeBlogs := make([]string, 0)
	for _, b := range blogs {
		// 检查访问时间
		if accessTime, err := time.Parse("2006-01-02 15:04:05", b.AccessTime); err == nil {
			if accessTime.After(sevenDaysAgo) {
				activeBlogs = append(activeBlogs, fmt.Sprintf("%s(最后访问:%s)", b.Title, b.AccessTime))
				continue
			}
		}
		// 检查修改时间
		if modifyTime, err := time.Parse("2006-01-02 15:04:05", b.ModifyTime); err == nil {
			if modifyTime.After(sevenDaysAgo) {
				activeBlogs = append(activeBlogs, fmt.Sprintf("%s(最后修改:%s)", b.Title, b.ModifyTime))
			}
		}
	}
	
	if len(activeBlogs) == 0 {
		return "近7天无活跃博客"
	}
	return strings.Join(activeBlogs, "\n")
}

// 获取月度创建趋势
func RawMonthlyCreationTrend() string {
	blogs := blog.Blogs
	monthCount := make(map[string]int)
	
	for _, b := range blogs {
		if createTime, err := time.Parse("2006-01-02 15:04:05", b.CreateTime); err == nil {
			monthKey := createTime.Format("2006-01")
			monthCount[monthKey]++
		}
	}
	
	result := "月度创建趋势:\n"
	// 按月份排序
	months := make([]string, 0, len(monthCount))
	for month := range monthCount {
		months = append(months, month)
	}
	sort.Strings(months)
	
	for _, month := range months {
		result += fmt.Sprintf("%s: %d篇\n", month, monthCount[month])
	}
	
	return result
}

// 搜索博客内容
func RawSearchBlogContent(keyword string) string {
	blogs := blog.Blogs
	matchedBlogs := make([]string, 0)
	
	for _, b := range blogs {
		if strings.Contains(strings.ToLower(b.Content), strings.ToLower(keyword)) ||
		   strings.Contains(strings.ToLower(b.Title), strings.ToLower(keyword)) {
			matchedBlogs = append(matchedBlogs, b.Title)
		}
	}
	
	if len(matchedBlogs) == 0 {
		return fmt.Sprintf("未找到包含关键词'%s'的博客", keyword)
	}
	return strings.Join(matchedBlogs, " ")
}

// 获取锻炼详细统计
func RawExerciseDetailedStats() string {
	all, err := exercise.GetAllExercises()
	if err != nil {
		return "获取锻炼数据失败: " + err.Error()
	}
	
	totalSessions := len(all)
	totalMinutes := 0
	totalCalories := 0
	typeCount := make(map[string]int)
	
	for _, e := range all {
		for _, item := range e.Items {
			totalMinutes += item.Duration
			totalCalories += item.Calories
			typeCount[item.Type]++
		}
	}
	
	result := fmt.Sprintf("锻炼详细统计:\n总锻炼次数:%d\n总时长:%d分钟(%.1f小时)\n总卡路里:%d千卡\n\n锻炼类型分布:\n",
		totalSessions, totalMinutes, float64(totalMinutes)/60.0, totalCalories)
	
	for exerciseType, count := range typeCount {
		result += fmt.Sprintf("- %s: %d次\n", exerciseType, count)
	}
	
	return result
}

// 获取近期锻炼记录
func RawRecentExerciseRecords(days int) string {
	all, err := exercise.GetAllExercises()
	if err != nil {
		return "获取锻炼数据失败: " + err.Error()
	}
	
	now := time.Now()
	cutoffDate := now.AddDate(0, 0, -days)
	
	result := fmt.Sprintf("最近%d天锻炼记录:\n", days)
	recentCount := 0
	
	for _, e := range all {
		// ExerciseList.Date字段包含日期信息
		exerciseDate := e.Date
		if date, err := time.Parse("2006-01-02", exerciseDate); err == nil {
			if date.After(cutoffDate) {
				result += fmt.Sprintf("日期:%s, 项目数:%d\n", exerciseDate, len(e.Items))
				for _, item := range e.Items {
					result += fmt.Sprintf("  - %s(%s): %d分钟, %d千卡\n", 
						item.Name, item.Type, item.Duration, item.Calories)
				}
				recentCount++
			}
		}
	}
	
	if recentCount == 0 {
		result += fmt.Sprintf("最近%d天无锻炼记录\n", days)
	}
	
	return result
}