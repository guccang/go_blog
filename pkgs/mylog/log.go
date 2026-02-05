package mylog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogModule int

const (
	ModuleAccount LogModule = iota
	ModuleBlog
	ModuleComment
	ModuleConfig
	ModuleLogin
	ModulePersistence
	ModuleUser
	ModuleExercise
	ModuleCommon
	ModuleAuth
	ModuleHandler
	ModuleLifeCountDown
	ModuleLLM
	ModuleMCP
	ModuleReading
	ModuleSMS
	ModuleYearPlan
	ModuleConstellation
	ModuleAssistant
	ModuleSearch
	ModuleSkill
	ModuleStatistics
	ModuleFinance
	ModuleShare
	ModuleTodolist
	ModuleTaskBreakdown
	ModuleView
	ModuleControl
	ModuleAgent
)

// LogLevel definition
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger configuration
type LogConfig struct {
	LogToFile    bool
	LogToConsole bool
	LogDir       string
	MaxFileSize  int64 // Maximum file size in MB
	MaxDays      int   // Maximum days to keep log files
	MinLevel     LogLevel
}

var (
	logConfig = LogConfig{
		LogToFile:    true,
		LogToConsole: true,
		LogDir:       "logs",
		MaxFileSize:  100,        // 100MB
		MaxDays:      30,         // 30 days
		MinLevel:     LevelDebug, // Default to Info to hide Debug logs
	}
	logFile     *os.File
	logMutex    sync.Mutex
	logWriter   io.Writer
	file_prefix string = "blog"
	logModules  map[LogModule]string
)

func Info() {
	logModules = map[LogModule]string{
		ModuleAccount:       "account",
		ModuleBlog:          "blog",
		ModuleComment:       "comment",
		ModuleConfig:        "config",
		ModuleLogin:         "login",
		ModulePersistence:   "persistence",
		ModuleUser:          "user",
		ModuleExercise:      "exercise",
		ModuleCommon:        "common",
		ModuleAuth:          "auth",
		ModuleHandler:       "handler",
		ModuleLifeCountDown: "lifecountdown",
		ModuleLLM:           "llm",
		ModuleMCP:           "mcp",
		ModuleReading:       "reading",
		ModuleSMS:           "sms",
		ModuleYearPlan:      "yearplan",
		ModuleConstellation: "constellation",
		ModuleAssistant:     "assistant",
		ModuleSearch:        "search",
		ModuleSkill:         "skill",
		ModuleStatistics:    "statistics",
		ModuleFinance:       "finance",
		ModuleShare:         "share",
		ModuleTodolist:      "todolist",
		ModuleTaskBreakdown: "taskbreakdown",
		ModuleView:          "view",
		ModuleControl:       "control",
		ModuleAgent:         "agent",
	}
}

// Init initializes the logging system
func Init(logsDir string) error {
	if logsDir == "" {
		logsDir = "logs" // Default logs directory
	}

	logMutex.Lock()
	defer logMutex.Unlock()

	if logsDir != "" {
		logConfig.LogDir = logsDir
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logConfig.LogDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Initialize log file
	if logConfig.LogToFile {
		if err := initLogFile(); err != nil {
			return fmt.Errorf("failed to initialize log file: %v", err)
		}
	}

	// Set up writers
	setupLogWriter()

	return nil
}

// SetConfig updates the logging configuration
func SetConfig(config LogConfig) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	logConfig = config

	// Reinitialize if needed
	if logConfig.LogToFile {
		if err := initLogFile(); err != nil {
			return err
		}
	}

	setupLogWriter()
	return nil
}

// initLogFile creates and opens the log file
func initLogFile() error {
	// Close existing file if open
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	// Generate log file name with current date
	now := time.Now()
	fileName := fmt.Sprintf("%s_%s.log", file_prefix, now.Format("2006-01-02"))
	filePath := filepath.Join(logConfig.LogDir, fileName)

	// Open log file
	var err error
	logFile, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// Check file size and rotate if necessary
	if err := checkAndRotateLog(); err != nil {
		return err
	}

	return nil
}

// setupLogWriter configures the multi-writer for console and file output
func setupLogWriter() {
	var writers []io.Writer

	if logConfig.LogToConsole {
		writers = append(writers, os.Stdout)
	}

	if logConfig.LogToFile && logFile != nil {
		writers = append(writers, logFile)
	}

	if len(writers) == 0 {
		logWriter = os.Stdout // Fallback to stdout
	} else if len(writers) == 1 {
		logWriter = writers[0]
	} else {
		logWriter = io.MultiWriter(writers...)
	}
}

// checkAndRotateLog checks if log file needs rotation
func checkAndRotateLog() error {
	if logFile == nil {
		return nil
	}

	fileInfo, err := logFile.Stat()
	if err != nil {
		return err
	}

	// Check if file size exceeds limit (convert MB to bytes)
	maxSize := logConfig.MaxFileSize * 1024 * 1024
	if fileInfo.Size() > maxSize {
		return rotateLogFile()
	}

	return nil
}

// rotateLogFile creates a new log file and archives the old one
func rotateLogFile() error {
	if logFile == nil {
		return nil
	}

	// Close current file
	logFile.Close()

	// Rename current file with timestamp
	now := time.Now()
	oldFileName := fmt.Sprintf("%s_%s.log", file_prefix, now.Format("2006-01-02"))
	newFileName := fmt.Sprintf("%s_%s_%s.log", file_prefix, now.Format("2006-01-02"), now.Format("15-04-05"))

	oldPath := filepath.Join(logConfig.LogDir, oldFileName)
	newPath := filepath.Join(logConfig.LogDir, newFileName)

	if err := os.Rename(oldPath, newPath); err != nil {
		// If rename fails, just continue with new file
		fmt.Printf("Warning: failed to rotate log file: %v\n", err)
	}

	// Create new log file
	return initLogFile()
}

// cleanOldLogs removes log files older than MaxDays
func cleanOldLogs() {
	if logConfig.LogDir == "" {
		return
	}

	cutoffTime := time.Now().AddDate(0, 0, -logConfig.MaxDays)

	filepath.Walk(logConfig.LogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Check if it's a log file and older than cutoff
		if filepath.Ext(path) == ".log" && info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				fmt.Printf("Warning: failed to remove old log file %s: %v\n", path, err)
			}
		}

		return nil
	})
}

// Cleanup closes log file and performs cleanup
func Cleanup() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// writeLog writes log message to configured outputs
func writeLog(module LogModule, levelName string, level LogLevel, message string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Filter by level
	if level < logConfig.MinLevel {
		return
	}

	// Check if we need to rotate log daily
	if logConfig.LogToFile {
		now := time.Now()
		expectedFileName := fmt.Sprintf("%s_%s.log", file_prefix, now.Format("2006-01-02"))
		expectedPath := filepath.Join(logConfig.LogDir, expectedFileName)

		// Check if current log file matches today's date
		if logFile != nil {
			currentPath, _ := filepath.Abs(logFile.Name())
			expectedAbsPath, _ := filepath.Abs(expectedPath)

			if currentPath != expectedAbsPath {
				// Need to switch to today's log file
				initLogFile()
				setupLogWriter()
			}
		}

		// Clean old logs periodically (every time we write, but with time check)
		if now.Hour() == 0 && now.Minute() < 5 { // Clean during early morning hours
			go cleanOldLogs()
		}
	}

	// Format and write log message
	timestamp := strTime()
	module_name := logModules[module]
	logMessage := fmt.Sprintf("[%s] [%s] %s %s\n", module_name, levelName, timestamp, message)

	if logWriter != nil {
		logWriter.Write([]byte(logMessage))
	} else {
		fmt.Print(logMessage)
	}
}

func strTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func Debug(module LogModule, str string) {
	writeLog(module, "DEBUG", LevelDebug, str)
}

func DebugF(module LogModule, f string, a ...any) {
	str := fmt.Sprintf(f, a...)
	writeLog(module, "DEBUG", LevelDebug, str)
}

func InfoF(module LogModule, f string, a ...any) {
	str := fmt.Sprintf(f, a...)
	writeLog(module, "MESSAGE", LevelInfo, str)
}

func ErrorF(module LogModule, f string, a ...any) {
	str := fmt.Sprintf(f, a...)
	writeLog(module, "ERROR", LevelError, str)
}

// Additional logging functions
func Error(module LogModule, str string) {
	writeLog(module, "ERROR", LevelError, str)
}

func Warn(module LogModule, str string) {
	writeLog(module, "WARN", LevelWarn, str)
}

func WarnF(module LogModule, f string, a ...any) {
	str := fmt.Sprintf(f, a...)
	writeLog(module, "WARN", LevelWarn, str)
}

func Message(module LogModule, str string) {
	writeLog(module, "MESSAGE", LevelInfo, str)
}
func MessageF(module LogModule, f string, a ...any) {
	str := fmt.Sprintf(f, a...)
	writeLog(module, "MESSAGE", LevelInfo, str)
}

// GetLogConfig returns current log configuration
func GetLogConfig() LogConfig {
	logMutex.Lock()
	defer logMutex.Unlock()
	return logConfig
}

// SetLogLevel sets minimum log level (for future enhancement)
func SetLogLevel(level string) {
	// This can be implemented later for log level filtering
}

// FlushLogs ensures all log data is written to disk
func FlushLogs() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile != nil {
		logFile.Sync()
	}
}
