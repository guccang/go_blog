package http

import (
	"auth"
	"config"
	"constellation"
	"cooperation"
	"exercise"
	"fmt"
	h "net/http"
	"mcp"
	"module"
	log "mylog"
	"os"
	"path/filepath"
	"strings"
	"todolist"
	"view"
	"yearplan"
)

// Info displays package version information
func Info() {
	log.Debug("info http v1.0")
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
		case "cooperation":
			authType |= module.EAuthType_cooperation
		case "encrypt":
			authType |= module.EAuthType_encrypt
		}
	}

	// 如果没有设置任何基础权限，默认为私有
	if (authType & (module.EAuthType_private | module.EAuthType_public)) == 0 {
		authType |= module.EAuthType_private
	}

	log.DebugF("Parsed auth type: %s -> %d", authTypeStr, authType)
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
	log.DebugF("RemoteAddr %s %s", remoteAddr, msg)
}

// getsession extracts session from request cookie
func getsession(r *h.Request) string {
	session, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return session.Value
}

// IsCooperation checks if the request is from a cooperation user
func IsCooperation(r *h.Request) bool {
	session := getsession(r)
	return cooperation.IsCooperation(session)
}

// checkLogin validates user login session
func checkLogin(r *h.Request) int {
	session, err := r.Cookie("session")
	if err != nil {
		log.ErrorF("not find cookie session err=%s", err.Error())
		return 1
	}

	log.DebugF("checkLogin session=%s", session.Value)
	if auth.CheckLoginSession(session.Value) != 0 {
		log.InfoF("checkLogin session=%s not find", session.Value)
		return 1
	}
	return 0
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

// HandleDemo handles the demo page
func HandleDemo(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleDemo", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}
	tmp_name := r.URL.Query().Get("tmp_name")
	view.PageDemo(w, tmp_name)
}

// HandleLink handles the main link/dashboard page
func HandleLink(w h.ResponseWriter, r *h.Request) {
	LogRemoteAddr("HandleLink", r)
	if checkLogin(r) != 0 {
		h.Redirect(w, r, "/index", 302)
		return
	}

	session := getsession(r)
	is_cooperation := cooperation.IsCooperation(session)
	flag := module.EAuthType_all
	if is_cooperation {
		flag = module.EAuthType_cooperation | module.EAuthType_public
	}
	view.PageLink(w, flag, session)
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
	log.Debug(exeDir)
	log.Debug(filePath)
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
	// Initialize todolist before registering handlers
	if err := todolist.InitTodoList(); err != nil {
		log.ErrorF("Failed to initialize todolist: %v", err)
	}

	// Core routes
	h.HandleFunc("/main", HandleLink)
	h.HandleFunc("/link", HandleLink)
	h.HandleFunc("/editor", HandleEditor)
	h.HandleFunc("/statics", HandleStatics)
	h.HandleFunc("/demo", HandleDemo)
	h.HandleFunc("/timestamp", HandleTimeStamp)
	h.HandleFunc("/index", HandleIndex)
	h.HandleFunc("/help", HandleHelp)
	h.HandleFunc("/d3", HandleD3)

	// Authentication routes
	h.HandleFunc("/login", HandleLogin)
	h.HandleFunc("/loginsms", HandleLoginSMS)
	h.HandleFunc("/api/logingensms", HandleLoginSMSAPI)

	// Blog routes
	h.HandleFunc("/save", HandleSave)
	h.HandleFunc("/get", HandleGet)
	h.HandleFunc("/modify", HandleModify)
	h.HandleFunc("/delete", HandleDelete)
	h.HandleFunc("/search", HandleSearch)
	h.HandleFunc("/comment", HandleComment)
	h.HandleFunc("/api/check-username", HandleCheckUsername)
	h.HandleFunc("/tag", HandleTag)
	h.HandleFunc("/getshare", HandleGetShare)
	h.HandleFunc("/public", HandlePublic)

	// Todolist routes
	h.HandleFunc("/todolist", HandleTodolist)
	h.HandleFunc("/api/todos", todolist.HandleTodos)
	h.HandleFunc("/api/todos/toggle", todolist.HandleToggleTodo)
	h.HandleFunc("/api/todos/time", todolist.HandleUpdateTodoTime)
	h.HandleFunc("/api/todos/history", todolist.HandleHistoricalTodos)
	h.HandleFunc("/api/todos/order", todolist.HandleUpdateTodoOrder)

	// Year plan and goal routes
	h.HandleFunc("/yearplan", HandleYearPlan)
	h.HandleFunc("/monthgoal", HandleMonthGoal)
	h.HandleFunc("/api/getplan", yearplan.HandleGetPlan)
	h.HandleFunc("/api/saveplan", yearplan.HandleSavePlan)

	// Month goal routes
	h.HandleFunc("/api/monthgoal", yearplan.HandleGetMonthGoal)
	h.HandleFunc("/api/savemonthgoal", yearplan.HandleSaveMonthGoal)
	h.HandleFunc("/api/weekgoal", yearplan.HandleGetWeekGoal)
	h.HandleFunc("/api/saveweekgoal", yearplan.HandleSaveWeekGoal)
	h.HandleFunc("/api/addtask", yearplan.HandleAddTask)
	h.HandleFunc("/api/updatetask", yearplan.HandleUpdateTask)
	h.HandleFunc("/api/deletetask", yearplan.HandleDeleteTask)
	h.HandleFunc("/api/monthgoals", yearplan.HandleGetMonthGoals)

	// Statistics routes
	h.HandleFunc("/statistics", HandleStatistics)
	h.HandleFunc("/api/statistics", HandleStatisticsAPI)

	// Exercise routes
	h.HandleFunc("/exercise", HandleExercise)
	h.HandleFunc("/api/exercises", exercise.HandleExercises)
	h.HandleFunc("/api/exercises/toggle", exercise.HandleToggleExercise)
	h.HandleFunc("/api/exercise-templates", exercise.HandleTemplates)
	h.HandleFunc("/api/exercise-stats", exercise.HandleExerciseStats)
	h.HandleFunc("/api/exercise-collections", exercise.HandleCollections)
	h.HandleFunc("/api/exercise-collections/add", exercise.HandleAddFromCollection)
	h.HandleFunc("/api/exercise-collections/details", exercise.HandleGetCollectionDetails)
	h.HandleFunc("/api/exercise-profile", exercise.HandleUserProfile)
	h.HandleFunc("/api/exercise-calculate-calories", exercise.HandleCalculateCalories)
	h.HandleFunc("/api/exercise-met-values", exercise.HandleMETValues)
	h.HandleFunc("/api/exercise-get-met-value", exercise.HandleGetMETValue)
	h.HandleFunc("/api/exercise-update-template-calories", exercise.HandleUpdateTemplateCalories)
	h.HandleFunc("/api/exercise-update-exercise-calories", exercise.HandleUpdateExerciseCalories)

	// Reading routes
	h.HandleFunc("/reading", HandleReading)
	h.HandleFunc("/reading-dashboard", HandleReadingDashboard)
	h.HandleFunc("/reading/book/", HandleBookDetail)
	h.HandleFunc("/api/books", HandleBooksAPI)
	h.HandleFunc("/api/reading-statistics", HandleReadingStatisticsAPI)
	h.HandleFunc("/api/parse-book-url", HandleParseBookURL)
	h.HandleFunc("/api/books/progress", HandleBookProgressAPI)
	h.HandleFunc("/api/books/finish", HandleBookFinishAPI)
	h.HandleFunc("/api/books/notes", HandleBookNotesAPI)
	h.HandleFunc("/api/books/insights", HandleBookInsightsAPI)

	// Advanced reading feature routes
	h.HandleFunc("/api/reading-plans", HandleReadingPlansAPI)
	h.HandleFunc("/api/reading-goals", HandleReadingGoalsAPI)
	h.HandleFunc("/api/book-recommendations", HandleBookRecommendationsAPI)
	h.HandleFunc("/api/reading-session", HandleReadingSessionAPI)
	h.HandleFunc("/api/book-collections", HandleBookCollectionsAPI)
	h.HandleFunc("/api/advanced-reading-statistics", HandleAdvancedReadingStatisticsAPI)
	h.HandleFunc("/api/export-reading-data", HandleExportReadingDataAPI)

	// Life countdown routes
	h.HandleFunc("/lifecountdown", HandleLifeCountdown)
	h.HandleFunc("/api/lifecountdown", HandleLifeCountdownAPI)
	h.HandleFunc("/api/lifecountdown/config", HandleLifeCountdownConfigAPI)

	// Assistant routes
	h.HandleFunc("/assistant", HandleAssistant)
	h.HandleFunc("/api/assistant/chat", HandleAssistantChat)
	h.HandleFunc("/api/assistant/chat/history", HandleAssistantChatHistory)
	h.HandleFunc("/api/assistant/stats", HandleAssistantStats)
	h.HandleFunc("/api/assistant/suggestions", HandleAssistantSuggestions)
	h.HandleFunc("/api/assistant/trends", HandleAssistantTrends)
	h.HandleFunc("/api/assistant/health-comprehensive", HandleAssistantHealthComprehensive)

	// System configuration routes
	h.HandleFunc("/config", HandleConfig)
	h.HandleFunc("/api/config", HandleConfigAPI)

	// MCP configuration routes
	h.HandleFunc("/mcp", mcp.HandleMCPPage)
	h.HandleFunc("/api/mcp", mcp.HandleMCPAPI)
	h.HandleFunc("/api/mcp/tools", HandleMCPToolsAPI)

	// Constellation divination routes
	h.HandleFunc("/constellation", constellation.HandleConstellation)
	h.HandleFunc("/api/constellation/horoscope", constellation.HandleDailyHoroscope)
	h.HandleFunc("/api/constellation/birthchart", constellation.HandleBirthChart)
	h.HandleFunc("/api/constellation/divination", constellation.HandleDivination)
	h.HandleFunc("/api/constellation/compatibility", constellation.HandleCompatibility)
	h.HandleFunc("/api/constellation/history", constellation.HandleDivinationHistory)
	h.HandleFunc("/api/constellation/statistics", constellation.HandleDivinationStats)
	h.HandleFunc("/api/constellation/info", constellation.HandleConstellationInfo)
	h.HandleFunc("/api/constellation/date", constellation.HandleGetConstellationByDate)
	h.HandleFunc("/api/constellation/accuracy", constellation.HandleUpdateAccuracy)
	h.HandleFunc("/api/constellation/batch-horoscope", constellation.HandleBatchHoroscope)

	// Static file server
	root := config.GetHttpStaticPath()
	fs := h.FileServer(h.Dir(root))
	h.Handle("/", h.StripPrefix("/", fs))
	//h.Handle("/", h.StripPrefix("/",basicAuth(fs)))
	return 0
}

// Run starts the HTTP server
func Run(certFile string, keyFile string) int {
	Init()
	port := config.GetConfig("port")
	//h.ListenAndServe(fmt.Sprintf(":%s",port),nil)
	if len(certFile) <= 0 || len(keyFile) <= 0 {
		h.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	} else {
		h.ListenAndServeTLS(fmt.Sprintf(":%s", port), certFile, keyFile, nil)
	}
	return 0
}

// Stop stops the HTTP server
func Stop() int {
	return 0
}