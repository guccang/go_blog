package config

import (
	"bufio"
	"fmt"
	log "mylog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// ========== Simple Config 模块 ==========
// 无 Actor、无 Channel，使用 sync.RWMutex

// ConfigStore 配置存储
type ConfigStore struct {
	Account        string
	datas          map[string]string
	autodatesuffix []string
	publictags     []string
	diary_keywords []string
	config_path    string
	sys_files      []string
	blog_version   string
	mu             sync.RWMutex
}

// ConfigManager 多账户管理
type ConfigManager struct {
	stores map[string]*ConfigStore
	mu     sync.Mutex
}

var (
	configManager *ConfigManager
	adminAccount  string
)

func Info() {
	log.Debug(log.ModuleConfig, "info config v14.0 (simple)")
}

// Init 初始化 Config 模块
func Init(filePath string) {
	InitManager(filePath)
}

// InitManager 初始化配置管理器
func InitManager(defaultConfigPath string) {
	configManager = &ConfigManager{
		stores: make(map[string]*ConfigStore),
	}

	// 创建默认配置存储
	defaultStore := &ConfigStore{
		Account:        "",
		datas:          make(map[string]string),
		autodatesuffix: make([]string, 0),
		publictags:     make([]string, 0),
		diary_keywords: make([]string, 0),
		config_path:    defaultConfigPath,
		sys_files:      make([]string, 0),
		blog_version:   "Version14.0",
	}

	log.DebugF(log.ModuleConfig, "InitManager defaultConfigPath=%s", defaultConfigPath)

	if err := defaultStore.loadConfigInternal("", defaultConfigPath); err != nil {
		log.ErrorF(log.ModuleConfig, "Init default config store err=%s", err.Error())
	}

	configManager.stores[defaultStore.Account] = defaultStore
	adminAccount = defaultStore.Account
	log.InfoF(log.ModuleConfig, "Config manager initialized with default account: %s", defaultStore.Account)
}

// getConfigStore 获取或创建指定账户的配置存储
func getConfigStore(account string) *ConfigStore {
	configManager.mu.Lock()
	defer configManager.mu.Unlock()

	if store, exists := configManager.stores[account]; exists {
		return store
	}

	// 创建新存储
	newStore := &ConfigStore{
		Account:        account,
		datas:          make(map[string]string),
		autodatesuffix: make([]string, 0),
		publictags:     make([]string, 0),
		diary_keywords: make([]string, 0),
		config_path:    "",
		sys_files:      make([]string, 0),
		blog_version:   "Version14.0",
	}

	// 加载默认配置
	isAdmin := true
	defaultConfigs := getDefaultConfigForAccountSimple(account, isAdmin)
	for key, value := range defaultConfigs {
		newStore.datas[key] = value
	}
	newStore.parseConfigArrays()

	configManager.stores[account] = newStore
	log.InfoF(log.ModuleConfig, "Created new config store for account: %s", account)
	return newStore
}

// loadConfigInternal 加载配置文件
func (store *ConfigStore) loadConfigInternal(account string, filePath string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	log.DebugF(log.ModuleConfig, "loadConfigInternal account=%s filePath=%s", account, filePath)
	datas, err := readConfigFile(filePath)
	if err != nil {
		return err
	}
	store.datas = datas
	store.config_path = filePath

	for k, v := range store.datas {
		log.DebugF(log.ModuleConfig, "CONFIG %s=%s", k, v)
	}

	store.parseConfigArraysInternal()

	if account == "" {
		account = store.datas["admin"]
		store.Account = account
	}
	store.loadDiaryKeywordsFromSysConf(account)
	return nil
}

// parseConfigArraysInternal 解析配置数组（内部版本，不加锁）
func (store *ConfigStore) parseConfigArraysInternal() {
	if datetitles, ok := store.datas["title_auto_add_date_suffix"]; ok {
		store.autodatesuffix = strings.Split(datetitles, "|")
	}
	if tags, ok := store.datas["publictags"]; ok {
		store.publictags = strings.Split(tags, "|")
	}
	if sysfiles, ok := store.datas["sysfiles"]; ok {
		store.sys_files = strings.Split(sysfiles, "|")
	}
	if keywords, ok := store.datas["diary_keywords"]; ok {
		arr := strings.Split(keywords, "|")
		store.diary_keywords = make([]string, 0, len(arr))
		for _, keyword := range arr {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" {
				store.diary_keywords = append(store.diary_keywords, keyword)
			}
		}
	}
	if len(store.diary_keywords) == 0 {
		store.diary_keywords = []string{"日记_"}
	}
}

// parseConfigArrays 解析配置数组（公开版本，加锁）
func (store *ConfigStore) parseConfigArrays() {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.parseConfigArraysInternal()
}

// loadDiaryKeywordsFromSysConf 从 sys_conf.md 加载日记关键字
func (store *ConfigStore) loadDiaryKeywordsFromSysConf(account string) {
	sysConfPath := GetSysConfigPath(account)
	if _, err := os.Stat(sysConfPath); os.IsNotExist(err) {
		store.diary_keywords = []string{"日记_"}
		return
	}

	file, err := os.Open(sysConfPath)
	if err != nil {
		store.diary_keywords = []string{"日记_"}
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "diary_keywords=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				keywordsStr := strings.TrimSpace(parts[1])
				if keywordsStr != "" {
					keywords := strings.Split(keywordsStr, "|")
					store.diary_keywords = make([]string, 0, len(keywords))
					for _, keyword := range keywords {
						keyword = strings.TrimSpace(keyword)
						if keyword != "" {
							store.diary_keywords = append(store.diary_keywords, keyword)
						}
					}
					return
				}
			}
		}
	}

	if len(store.diary_keywords) == 0 {
		store.diary_keywords = []string{"日记_"}
	}
}

// ========== 对外接口 ==========

// GetVersionWithAccount 获取版本
func GetVersionWithAccount(account string) string {
	store := getConfigStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.blog_version
}

// GetConfigPathWithAccount 获取配置路径
func GetConfigPathWithAccount(account string) string {
	store := getConfigStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.config_path
}

// ReloadConfig 重新加载配置
func ReloadConfig(account, filePath string) {
	ReloadConfigWithAccount(account, filePath)
}

// ReloadConfigWithAccount 重新加载配置
func ReloadConfigWithAccount(account, filePath string) {
	store := getConfigStore(account)
	store.loadConfigInternal(account, filePath)
}

// GetDiaryKeywordsWithAccount 获取日记关键字
func GetDiaryKeywordsWithAccount(account string) []string {
	store := getConfigStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.diary_keywords
}

// IsDiaryBlogWithAccount 检查是否为日记博客
func IsDiaryBlogWithAccount(account, title string) bool {
	store := getConfigStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	for _, keyword := range store.diary_keywords {
		if len(title) >= len(keyword) && title[:len(keyword)] == keyword {
			return true
		}
	}
	return false
}

// GetConfigWithAccount 获取配置值
func GetConfigWithAccount(account, name string) string {
	store := getConfigStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	if v, ok := store.datas[name]; ok {
		return v
	}
	return ""
}

// IsSysFile 检查是否为系统文件
func IsSysFile(name string) int {
	store := getConfigStore(adminAccount)
	store.mu.RLock()
	defer store.mu.RUnlock()

	for _, v := range store.sys_files {
		if v == name {
			return 1
		}
	}
	return 0
}

// IsPublicTag 检查是否为公开标签
func IsPublicTag(tag string) int {
	return IsPublicTagWithAccount("", tag)
}

// IsPublicTagWithAccount 检查是否为公开标签
func IsPublicTagWithAccount(account, tag string) int {
	store := getConfigStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	for _, v := range store.publictags {
		if v == tag {
			return 1
		}
	}
	return 0
}

// IsTitleAddDateSuffix 检查是否需要添加日期后缀
func IsTitleAddDateSuffix(title string) int {
	return IsTitleAddDateSuffixWithAccount("", title)
}

// IsTitleAddDateSuffixWithAccount 检查是否需要添加日期后缀
func IsTitleAddDateSuffixWithAccount(account, title string) int {
	store := getConfigStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	for _, v := range store.autodatesuffix {
		if v == title {
			return 1
		}
	}
	return 0
}

// IsTitleContainsDateSuffix 检查标题是否包含日期后缀
func IsTitleContainsDateSuffix(title string) int {
	return IsTitleContainsDateSuffixWithAccount("", title)
}

// IsTitleContainsDateSuffixWithAccount 检查标题是否包含日期后缀
func IsTitleContainsDateSuffixWithAccount(account, title string) int {
	store := getConfigStore(account)
	store.mu.RLock()
	defer store.mu.RUnlock()

	for _, v := range store.autodatesuffix {
		if strings.Contains(strings.ToLower(title), strings.ToLower(v)) {
			return 1
		}
	}
	return 0
}

// UpdateConfigFromBlog 从博客内容更新配置
func UpdateConfigFromBlog(account, blogContent string) {
	store := getConfigStore(account)
	store.mu.Lock()
	defer store.mu.Unlock()

	configs := parseConfigFromBlogContent(blogContent)
	for key, value := range configs {
		store.datas[key] = value
	}
	store.parseConfigArraysInternal()
}

// ========== 辅助函数 ==========

func readConfigFile(filePath string) (map[string]string, error) {
	config := make(map[string]string)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			config[key] = value
		}
	}
	return config, scanner.Err()
}

func parseConfigFromBlogContent(content string) map[string]string {
	configs := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			configs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return configs
}

func getDefaultConfigForAccountSimple(account string, isAdmin bool) map[string]string {
	if isAdmin {
		return map[string]string{
			"port":                       "8888",
			"redis_ip":                   "127.0.0.1",
			"redis_port":                 "6666",
			"redis_pwd":                  "",
			"publictags":                 "public|share|demo",
			"sysfiles":                   GetSysConfigs(),
			"title_auto_add_date_suffix": "日记",
			"diary_keywords":             "日记_",
			"diary_password":             "",
			"main_show_blogs":            "10",
			"admin":                      account,
		}
	}
	return map[string]string{
		"publictags":                 "public|share|demo",
		"title_auto_add_date_suffix": "日记",
		"diary_keywords":             "日记_",
		"diary_password":             "",
		"main_show_blogs":            "10",
	}
}

// ========== 路径函数 ==========

func GetHttpTemplatePath() string {
	templates_path := GetConfigWithAccount(adminAccount, "templates_path")
	if templates_path == "" {
		exePath, _ := os.Executable()
		return filepath.Join(filepath.Dir(exePath), "templates")
	}
	return templates_path
}

func GetHttpStaticPath() string {
	statics_path := GetConfigWithAccount(adminAccount, "statics_path")
	if statics_path == "" {
		exePath, _ := os.Executable()
		return filepath.Join(filepath.Dir(exePath), "statics")
	}
	return statics_path
}

func GetExePath() string {
	exePath, _ := os.Executable()
	return filepath.Dir(exePath)
}

func GetBlogsPath(account string) string {
	return filepath.Join(GetExePath(), "blogs_txt", account)
}

func GetDownLoadPath() string {
	return GetConfigWithAccount(adminAccount, "download_path")
}

func GetHelpBlogName() string {
	return GetConfigWithAccount(adminAccount, "help_blog_name")
}

func GetMaxBlogComments() int {
	str_cnt := GetConfigWithAccount(adminAccount, "max_blog_comments")
	cnt, _ := strconv.Atoi(str_cnt)
	if cnt <= 0 {
		cnt = 100
	}
	return cnt
}

func GetMainBlogNum() int {
	str_cnt := GetConfigWithAccount(adminAccount, "main_show_blogs")
	cnt, _ := strconv.Atoi(str_cnt)
	if cnt <= 0 {
		cnt = 100
	}
	return cnt
}

func GetRecyclePath() string {
	path := GetConfigWithAccount(adminAccount, "recycle_path")
	if path == "" {
		path = ".go_blog_recycle"
	}
	return path
}

func GetAdminAccount() string {
	return adminAccount
}

func GetSysConfigPath(account string) string {
	return filepath.Join(GetBlogsPath(account), GetSysConfigFullName())
}

func GetSysConfigTitle() string {
	return "sys_conf"
}

func GetSysConfigFullName() string {
	return GetSysConfigTitle() + ".md"
}

func GetSysConfigTitleMCP() string {
	return "mcp_config"
}

func GetSysConfigs() string {
	return fmt.Sprintf("%s | %s | %s", GetSysConfigTitle(), GetSysConfigTitleMCP(), "sys_accounts")
}
