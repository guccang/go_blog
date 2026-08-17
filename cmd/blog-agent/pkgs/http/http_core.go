package http

import (
	"auth"
	"blog"
	"config"
	"encoding/json"
	"exercise"
	"fmt"
	"module"
	log "mylog"
	h "net/http"
	"net/url"
	"os"
	"path/filepath"
	control "service"
	"strconv"
	"strings"
	"time"
	"tools"
	// [Phase 1] wechat 模块已迁移至独立 wechat-agent
	// [Phase 3] 以下模块已屏蔽，统一使用 goal + todolist
	// "projectmgmt"
	// "taskbreakdown"
	// "yearplan"

	goalpkg "goal"
)

// Info displays package version information
func Info() {
	log.Debug(log.ModuleHandler, "info http v1.0")
}

// parseAuthTypeString parses permission type string, supports combined permissions
// parseAuthTypeString 解析权限类型字符串，支持组合权限
func parseAuthTypeString(authTypeStr string) int {
	if authTypeStr == "" {
		return module.EAuthType_private
	}

	authType := 0
	permissions := strings.Split(authTypeStr, ",")

	for _, perm := range permissions {
		perm = strings.TrimSpace(perm)
		switch perm {
		case "private":
			authType |= module.EAuthType_private
		case "public":
			authType |= module.EAuthType_public
		case "diary":
			authType |= module.EAuthType_diary
		case "encrypt":
			authType |= module.EAuthType_encrypt
		}
	}

	// 如果没有设置任何基础权限，默认为私有
	if (authType & (module.EAuthType_private | module.EAuthType_public)) == 0 {
		authType |= module.EAuthType_private
	}

	log.DebugF(log.ModuleHandler, "Parsed auth type: %s -> %d", authTypeStr, authType)
	return authType
}

// handle_content is a helper struct for content handling
type handle_content struct {
	content string
}

// LogRemoteAddr logs remote address with forwarded IP consideration
func LogRemoteAddr(msg string, r *h.Request) {
	remoteAddr := r.RemoteAddr
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		remoteAddr = xForwardedFor
	}
	log.DebugF(log.ModuleHandler, "RemoteAddr %s %s", remoteAddr, msg)
}

// getAccountFromRequest uses the canonical SQLite-backed account resolver.
func getAccountFromRequest(r *h.Request) string {
	return auth.GetAccountFromRequest(r)
}

// checkLogin validates user login session
func checkLogin(r *h.Request) int {
	if getAccountFromRequest(r) == "" {
		return 1
	}
	return 0
}

// requireAPIAuth protects JSON APIs and prevents any handler from operating
// with an empty account when a session is absent or expired.
func requireAPIAuth(next h.HandlerFunc) h.HandlerFunc {
	return func(w h.ResponseWriter, r *h.Request) {
		if getAccountFromRequest(r) == "" {
			h.Error(w, "unauthorized", h.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// HandleEditor handles the editor page
func HandleEditor(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleEditor", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	view.PageEditor(w, "", "")
}

// HandleLink handles the main link/dashboard page
func HandleLink(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLink", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	flag := module.EAuthType_all
	account := getAccountFromRequest(r)
	emitUsageHook(r, account, blog.HookPageOpened, "content_library", "page", "link", "BLOG", "", nil, map[string]any{"status": "success"})
	view.PageLink(w, flag, account)
}

// HandleImagePasteDemo provides a browser-only usability test for screenshot pasting.
// It deliberately does not persist images or modify blog content.
func HandleImagePasteDemo(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", h.StatusFound)
		return
	}
	PageImagePasteDemo(w)
}

// HandleBlogSummaries 为首页“加载更多”提供分页摘要，正文仅在打开文章时读取。
func HandleBlogSummaries(w h.ResponseWriter, r *h.Request) {
	if checkLogin(r) != 0 {
		h.Error(w, "unauthorized", h.StatusUnauthorized)
		return
	}
	limit, offset := 20, 0
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 100 {
		limit = n
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && n >= 0 {
		offset = n
	}
	account := getAccountFromRequest(r)
	blogs := control.ListBlogSummaries(account, limit+1, offset, module.EAuthType_all)
	hasMore := len(blogs) > limit
	if hasMore {
		blogs = blogs[:limit]
	}
	items := make([]map[string]interface{}, 0, len(blogs))
	for _, b := range blogs {
		isDiary := (b.AuthType & module.EAuthType_diary) != 0
		isEncrypted := b.Encrypt == 1 || (b.AuthType&module.EAuthType_encrypt) != 0
		preview, imageURL := "", ""
		switch {
		case isEncrypted:
			preview = "加密内容，打开后验证访问权限"
		case isDiary:
			preview = "日记内容，打开后验证访问权限"
		default:
			preview, imageURL = buildMainBlogPreview(b.Title, b.Content)
		}
		items = append(items, map[string]interface{}{
			"title": b.Title, "url": "/get?blogname=" + url.QueryEscape(b.Title),
			"access_time": recentTimeLabel(b.AccessTime, b.ModifyTime, time.Now()),
			"preview":     preview, "image_url": imageURL,
			"diary": isDiary, "encrypted": isEncrypted,
			"tech_doc": strings.Contains(b.Tags, "blog实现技术文档"),
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "has_more": hasMore})
}

// HandleStatics handles static file serving
func HandleStatics(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleStatics", r)
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		h.Error(w, "Filepath parameter is missing", h.StatusBadRequest)
		return
	}

	spath := config.GetHttpStaticPath()
	filePath := filepath.Join(spath, filename)

	// 打开文件
	exeDir := config.GetExePath()
	log.Debug(log.ModuleHandler, exeDir)
	log.Debug(log.ModuleHandler, filePath)
	file, err := h.Dir(spath).Open(filename)
	if err != nil {
		h.Error(w, "File not found", h.StatusNotFound)
		return
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		h.Error(w, "File not found", h.StatusNotFound)
		return
	}

	// 设置HTTP响应头
	w.Header().Set("Content-Disposition", "attachment; filename="+filePath)
	w.Header().Set("Content-Type", "application/octet-stream")

	// 将文件内容发送到响应体
	h.ServeContent(w, r, filename, fileInfo.ModTime(), file)
}

// Init initializes all HTTP routes and handlers
func Init() int {
	// [Phase 3] taskbreakdown 已屏蔽，统一使用 goal
	// if err := taskbreakdown.InitTaskBreakdown(); err != nil {
	// 	log.ErrorF(log.ModuleHandler, "Failed to initialize task breakdown: %v", err)
	// }

	// Core routes
	h.HandleFunc("/main", HandleMain)
	h.HandleFunc("/api/blogs/page", HandleBlogSummaries)
	h.HandleFunc("/api/blogs/fts", HandleBlogFTSSearch)
	h.HandleFunc("/api/pi/ask", HandlePIAsk)
	h.HandleFunc("/api/pi/article", HandlePIArticle)
	h.HandleFunc("/api/pi/usage", HandlePIUsage)
	h.HandleFunc("/ask", HandleAskPage)
	h.HandleFunc("/insights", HandleHookInsights)
	h.HandleFunc("/api/hooks/insights", HandleHookInsightsAPI)
	h.HandleFunc("/api/media/upload", HandleMediaUpload)
	h.HandleFunc("/api/blog/content", HandleBlogContentChunk)
	h.HandleFunc("/media/render/", HandleMediaRender)
	h.HandleFunc("/media/view/", HandleMediaView)
	h.HandleFunc("/media/", HandleMediaGet)
	h.HandleFunc("/link", HandleLink)
	h.HandleFunc("/editor", HandleEditor)
	h.HandleFunc("/image-paste-demo", HandleImagePasteDemo)
	h.HandleFunc("/statics", HandleStatics)
	h.HandleFunc("/index", HandleIndex)

	// Authentication routes
	h.HandleFunc("/login", HandleLogin)
	h.HandleFunc("/register", HandleRegister)

	// Blog routes
	h.HandleFunc("/save", HandleSave)
	h.HandleFunc("/get", HandleGet)
	h.HandleFunc("/modify", HandleModify)
	h.HandleFunc("/delete", HandleDelete)
	h.HandleFunc("/search", HandleSearch)
	h.HandleFunc("/tag", HandleTag)
	h.HandleFunc("/getshare", HandleGetShare)
	h.HandleFunc("/public", HandlePublic)

	// Share routes
	h.HandleFunc("/api/createshare", HandleCreateShare)

	// [Phase 3] Task breakdown routes 已屏蔽，统一使用 goal
	// h.HandleFunc("/taskbreakdown", taskbreakdown.HandleTaskBreakdown)
	// h.HandleFunc("/taskbreakdown/completed", taskbreakdown.HandleCompletedTasks)
	// h.HandleFunc("/taskbreakdown/deleted", taskbreakdown.HandleDeletedTasks)
	// h.HandleFunc("/api/tasks", taskbreakdown.HandleTasks)
	// h.HandleFunc("/api/tasks/", taskbreakdown.HandleTasks) // 处理带ID的路径
	// h.HandleFunc("/api/tasks/progress", taskbreakdown.HandleTaskProgress)
	// h.HandleFunc("/api/tasks/order", taskbreakdown.HandleTaskOrder)
	// h.HandleFunc("/api/tasks/subtasks", taskbreakdown.HandleSubtasks)
	// h.HandleFunc("/api/tasks/timeline", taskbreakdown.HandleTimeline)
	// h.HandleFunc("/api/tasks/graph", taskbreakdown.HandleTaskGraph)
	// h.HandleFunc("/api/tasks/trends", taskbreakdown.HandleTimeTrends)
	// h.HandleFunc("/api/tasks/statistics", taskbreakdown.HandleStatistics)
	// h.HandleFunc("/api/tasks/search", taskbreakdown.HandleSearchTasks)
	// h.HandleFunc("/api/tasks/time-analysis", taskbreakdown.HandleTaskTimeAnalysis)
	// h.HandleFunc("/api/tasks/daily-overlap", taskbreakdown.HandleDailyTimeOverlap)
	// h.HandleFunc("/api/tasks/sync-to-todo", taskbreakdown.HandleSyncToTodo)

	// Goal management routes (unified daily/weekly/monthly/yearly)
	h.HandleFunc("/goal", HandleGoal)
	h.HandleFunc("/goal/manage", HandleGoalManage)
	h.HandleFunc("/api/goal", requireAPIAuth(goalpkg.HandleGetGoal))
	h.HandleFunc("/api/goal/save", requireAPIAuth(goalpkg.HandleSaveGoal))
	h.HandleFunc("/api/goal/task", requireAPIAuth(goalpkg.HandleAddGoalTask))
	h.HandleFunc("/api/goal/task/update", requireAPIAuth(goalpkg.HandleUpdateGoalTask))
	h.HandleFunc("/api/goal/task/delete", requireAPIAuth(goalpkg.HandleDeleteGoalTask))
	h.HandleFunc("/api/goal/delete", requireAPIAuth(goalpkg.HandleDeleteGoal))
	h.HandleFunc("/api/goals/current", requireAPIAuth(goalpkg.HandleGetCurrentGoals))
	h.HandleFunc("/api/goals", requireAPIAuth(goalpkg.HandleListGoals))
	h.HandleFunc("/api/goal/parent", requireAPIAuth(goalpkg.HandleGetParentGoals))
	h.HandleFunc("/api/goal/task/note", requireAPIAuth(goalpkg.HandleAddTaskNote))
	h.HandleFunc("/api/goal/review", requireAPIAuth(goalpkg.HandleGetReview))
	h.HandleFunc("/api/goal/review/save", requireAPIAuth(goalpkg.HandleSaveReview))
	h.HandleFunc("/api/goal/review/generate", requireAPIAuth(goalpkg.HandleGenerateReview))

	// Exercise routes
	h.HandleFunc("/exercise", HandleExercise)
	h.HandleFunc("/exercise/manage", HandleExerciseManage)
	h.HandleFunc("/api/exercises", requireAPIAuth(exercise.HandleExercises))
	h.HandleFunc("/api/exercises/toggle", requireAPIAuth(exercise.HandleToggleExercise))
	h.HandleFunc("/api/exercise-templates", requireAPIAuth(exercise.HandleTemplates))
	h.HandleFunc("/api/exercise-stats", requireAPIAuth(exercise.HandleExerciseStats))
	h.HandleFunc("/api/exercise-collections", requireAPIAuth(exercise.HandleCollections))
	h.HandleFunc("/api/exercise-collections/add", requireAPIAuth(exercise.HandleAddFromCollection))
	h.HandleFunc("/api/exercise-collections/details", requireAPIAuth(exercise.HandleGetCollectionDetails))
	h.HandleFunc("/api/exercise-profile", requireAPIAuth(exercise.HandleUserProfile))
	h.HandleFunc("/api/exercise-calculate-calories", requireAPIAuth(exercise.HandleCalculateCalories))
	h.HandleFunc("/api/exercise-met-values", requireAPIAuth(exercise.HandleMETValues))
	h.HandleFunc("/api/exercise-get-met-value", requireAPIAuth(exercise.HandleGetMETValue))
	h.HandleFunc("/api/exercise-update-template-calories", requireAPIAuth(exercise.HandleUpdateTemplateCalories))
	h.HandleFunc("/api/exercise-update-exercise-calories", requireAPIAuth(exercise.HandleUpdateExerciseCalories))

	// Reading routes
	h.HandleFunc("/reading", HandleReading)
	h.HandleFunc("/reading/manage", HandleReadingManage)
	h.HandleFunc("/reading-dashboard", HandleReadingDashboard)
	h.HandleFunc("/reading/book/", HandleBookDetail)
	h.HandleFunc("/api/books", HandleBooksAPI)
	h.HandleFunc("/api/reading-statistics", HandleReadingStatisticsAPI)
	h.HandleFunc("/api/parse-book-url", HandleParseBookURL)
	h.HandleFunc("/api/books/progress", HandleBookProgressAPI)
	h.HandleFunc("/api/books/finish", HandleBookFinishAPI)
	h.HandleFunc("/api/books/notes", HandleBookNotesAPI)
	h.HandleFunc("/api/books/insights", HandleBookInsightsAPI)

	// [Phase 3] Advanced reading routes 已屏蔽 (goals/plans 由统一 goal 模块管理)
	// h.HandleFunc("/api/reading-plans", HandleReadingPlansAPI)
	// h.HandleFunc("/api/reading-goals", HandleReadingGoalsAPI)
	h.HandleFunc("/api/book-recommendations", HandleBookRecommendationsAPI)
	h.HandleFunc("/api/reading-session", HandleReadingSessionAPI)
	h.HandleFunc("/api/book-collections", HandleBookCollectionsAPI)
	h.HandleFunc("/api/advanced-reading-statistics", HandleAdvancedReadingStatisticsAPI)
	h.HandleFunc("/api/export-reading-data", HandleExportReadingDataAPI)

	// System configuration routes
	h.HandleFunc("/config", HandleConfig)
	h.HandleFunc("/api/config", HandleConfigAPI)

	// Tools routes
	h.HandleFunc("/tools", HandleTools)
	h.HandleFunc("/api/tools/time", tools.TimeToolHandler)
	h.HandleFunc("/api/tools/data", tools.DataProcessHandler)
	h.HandleFunc("/api/tools/calculator", tools.CalculatorHandler)
	h.HandleFunc("/api/tools/bmi", tools.BMIHandler)
	h.HandleFunc("/api/tools/text", tools.TextToolHandler)
	h.HandleFunc("/api/tools/unit-convert", tools.UnitConvertHandler)

	// account
	h.HandleFunc("/account", HandleAccount)
	h.HandleFunc("/api/account", HandleAccountAPI)

	// Static file server
	root := config.GetHttpStaticPath()
	fs := h.FileServer(h.Dir(root))
	h.Handle("/", h.StripPrefix("/", fs))
	//h.Handle("/", h.StripPrefix("/",basicAuth(fs)))
	return 0
}

// Run starts the HTTP server
func Run(certFile string, keyFile string, portOverride string) error {
	Init()
	port := config.GetConfigWithAccount(config.GetAdminAccount(), "port")
	if portOverride != "" {
		port = portOverride
	}
	var err error
	//h.ListenAndServe(fmt.Sprintf(":%s",port),nil)
	if len(certFile) <= 0 || len(keyFile) <= 0 {
		err = h.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	} else {
		err = h.ListenAndServeTLS(fmt.Sprintf(":%s", port), certFile, keyFile, nil)
	}
	return err
}

// Stop stops the HTTP server
func Stop() int {
	return 0
}
