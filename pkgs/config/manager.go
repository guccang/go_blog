package config

import (
	"core"
	log "mylog"
	"sync"
)

// ConfigManager manages multiple config actors for different accounts
type ConfigManager struct {
	actors map[string]*ConfigActor // account -> ConfigActor
	mu     sync.RWMutex
}

var configManager *ConfigManager

// InitManager initializes the config manager
func InitManager(defaultConfigPath string) {
	configManager = &ConfigManager{
		actors: make(map[string]*ConfigActor),
	}

	// Create a simple default actor that will be used as fallback
	defaultActor := &ConfigActor{
		Actor:          core.NewActor(),
		Account:        "",
		datas:          make(map[string]string),
		autodatesuffix: make([]string, 0),
		publictags:     make([]string, 0),
		diary_keywords: make([]string, 0),
		config_path:    defaultConfigPath,
		sys_files:      make([]string, 0),
		blog_version:   "Version13.0",
	}
	log.DebugF(log.ModuleConfig, "InitManager defaultConfigPath=%s", defaultConfigPath)

	err := defaultActor.loadConfigInternal("", defaultConfigPath)
	if err != nil {
		log.ErrorF(log.ModuleConfig, "Init default config actor err=%s", err.Error())
	}
	defaultActor.Start(defaultActor)

	configManager.actors[defaultActor.Account] = defaultActor
	adminAccount = defaultActor.Account
	log.InfoF(log.ModuleConfig, "Config manager initialized with default account: %s", defaultActor.Account)
}

// getConfigActor returns the config actor for the given account
func getConfigActor(account string) *ConfigActor {

	configManager.mu.RLock()
	if act, exists := configManager.actors[account]; exists {
		configManager.mu.RUnlock()
		return act
	}
	configManager.mu.RUnlock()

	// Create new actor for this account
	configManager.mu.Lock()
	defer configManager.mu.Unlock()

	// Double check after acquiring write lock
	if act, exists := configManager.actors[account]; exists {
		return act
	}

	newActor := &ConfigActor{
		Actor:          core.NewActor(),
		Account:        account,
		datas:          make(map[string]string),
		autodatesuffix: make([]string, 0),
		publictags:     make([]string, 0),
		diary_keywords: make([]string, 0),
		config_path:    "",
		sys_files:      make([]string, 0),
		blog_version:   "Version13.0",
	}
	newActor.Start(newActor)

	// Initialize with default config - assume admin for now if needed
	// We'll determine admin status from config files later
	isAdmin := true // Default to admin privileges during initial setup
	defaultConfigs := getDefaultConfigForAccountSimple(account, isAdmin)
	for key, value := range defaultConfigs {
		newActor.datas[key] = value
	}
	newActor.parseConfigArrays()

	configManager.actors[account] = newActor
	log.InfoF(log.ModuleConfig, "Created new config actor for account: %s", account)
	return newActor
}

// getDefaultConfigForAccountSimple returns default config without calling other manager functions
func getDefaultConfigForAccountSimple(account string, isAdmin bool) map[string]string {
	if isAdmin {
		// Admin gets full system configuration
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
	} else {
		// Regular users get limited personal configuration
		return map[string]string{
			"publictags":                 "public|share|demo",
			"title_auto_add_date_suffix": "日记",
			"diary_keywords":             "日记_",
			"diary_password":             "",
			"main_show_blogs":            "10",
		}
	}
}

// RemoveAccount removes an account's config actor
type RemoveAccountConfigCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *RemoveAccountConfigCmd) Do(actor core.ActorInterface) {
	configManager.mu.Lock()
	defer configManager.mu.Unlock()

	if act, exists := configManager.actors[cmd.Account]; exists {
		act.Stop()
		delete(configManager.actors, cmd.Account)
		log.InfoF(log.ModuleConfig, "Removed config actor for account: %s", cmd.Account)
	}
	cmd.Response() <- 0
}

// loadAccountConfigCmd loads config for a specific account from sys_conf_<account> blog
type loadAccountConfigCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *loadAccountConfigCmd) Do(actor core.ActorInterface) {
	configActor := actor.(*ConfigActor)

	// Load default configurations first
	defaultConfigs := getDefaultConfigForAccount(cmd.Account)
	for key, value := range defaultConfigs {
		configActor.datas[key] = value
	}

	// Try to load account-specific config from sys_conf_<account> blog
	// This will be handled by the blog system integration
	configActor.loadAccountSpecificConfig(cmd.Account)

	// Parse arrays from string configs
	configActor.parseConfigArrays()

	log.DebugF(log.ModuleConfig, "Loaded config for account %s, config count=%d", cmd.Account, len(configActor.datas))
	cmd.Response() <- 0
}

// getDefaultConfigForAccount returns default configuration for an account
func getDefaultConfigForAccount(account string) map[string]string {
	// Get admin account to determine if this is admin or regular user
	isAdmin := (account == GetAdminAccount())

	if isAdmin {
		// Admin gets full system configuration
		return map[string]string{
			"port":                       "8888",
			"redis_ip":                   "127.0.0.1",
			"redis_port":                 "6666",
			"redis_pwd":                  "",
			"publictags":                 "public|share|demo",
			"sysfiles":                   GetSysConfigs(),
			"title_auto_add_date_suffix": "日记",
			"diary_keywords":             "日记_",
			"diary_password":             "yuanbao2022",
			"main_show_blogs":            "89",
			"admin":                      account,
		}
	} else {
		// Regular users get limited personal configuration
		return map[string]string{
			"publictags":                 "public|share|demo",
			"title_auto_add_date_suffix": "日记",
			"diary_keywords":             "日记_",
			"diary_password":             "yuanbao2022",
			"main_show_blogs":            "89",
		}
	}
}
