# Enhanced Logging System (mylog)

This package provides a comprehensive logging system with disk persistence, log rotation, and configurable output options.

## Features

### ✨ Core Features
- **Dual Output**: Simultaneous logging to console and file
- **Daily Log Rotation**: Automatic creation of daily log files
- **Size-based Rotation**: Automatic rotation when files exceed size limit
- **Old Log Cleanup**: Automatic removal of logs older than configured days
- **Thread-safe**: Concurrent logging support with mutex protection
- **Configurable**: Flexible configuration options

### 📁 Log File Structure
```
logs/
├── app_2024-01-15.log           # Today's log file
├── app_2024-01-14_16-30-45.log  # Rotated log (when size exceeded)
├── app_2024-01-14.log           # Yesterday's log
└── app_2024-01-13.log           # Older logs (cleaned after MaxDays)
```

## Configuration

### Default Configuration
```go
LogConfig{
    LogToFile:    true,     // Enable file logging
    LogToConsole: true,     // Enable console logging  
    LogDir:       "logs",   // Log directory
    MaxFileSize:  100,      // Maximum file size in MB
    MaxDays:      30,       // Keep logs for 30 days
}
```

### System Configuration (sys_conf.md)
Add the following line to configure log directory:
```
logs_dir=logs
```

## API Reference

### Initialization
```go
import log "mylog"

// Initialize with default logs directory
err := log.Init("logs")

// Initialize with custom directory from config
logsDir := config.GetConfig("logs_dir")
err := log.Init(logsDir)
```

### Basic Logging Functions
```go
// Simple logging
log.Debug("Debug message")
log.InfoMessage("Info message") 
log.Warn("Warning message")
log.Error("Error message")
log.Fatal("Fatal message") // Exits program

// Formatted logging
log.DebugF("Debug: %s = %d", "value", 42)
log.InfoF("Info: %s", "formatted message")
log.WarnF("Warning: %.2f%%", 85.5)
log.ErrorF("Error: %v", err)
log.FatalF("Fatal: %s", "critical error") // Exits program
```

### Advanced Functions
```go
// Configuration management
config := log.GetLogConfig()
err := log.SetConfig(log.LogConfig{
    LogToFile:    true,
    LogToConsole: false,  // Disable console output
    LogDir:       "/var/log/myapp",
    MaxFileSize:  50,     // 50MB limit
    MaxDays:      7,      // Keep for 1 week
})

// Manual operations
log.FlushLogs()  // Force write to disk
log.Cleanup()    // Close files and cleanup
```

## Log Levels

| Level   | Function     | Use Case                     |
|---------|--------------|------------------------------|
| DEBUG   | Debug()      | Development debugging        |
| MESSAGE | InfoMessage()| General information          |
| WARN    | Warn()       | Warning conditions           |
| ERROR   | Error()      | Error conditions             |
| FATAL   | Fatal()      | Critical errors (exits app)  |

## Integration Example

### In main.go
```go
func main() {
    // Initialize config first
    config.Init(args[1])
    
    // Initialize logging
    logsDir := config.GetConfig("logs_dir")
    if logsDir == "" {
        logsDir = "logs"
    }
    if err := log.Init(logsDir); err != nil {
        fmt.Printf("Warning: Failed to initialize file logging: %v\n", err)
    }
    log.Debug("Logging system initialized")
    
    // Your application code here...
    
    // Cleanup on exit
    log.Debug("Application exit")
    log.FlushLogs()
    log.Cleanup()
}
```

## Performance Considerations

- **Thread Safety**: All logging operations are mutex-protected
- **Buffered I/O**: Uses buffered file operations for performance
- **Async Cleanup**: Old log cleanup runs in background goroutine
- **Daily Rotation**: Minimal overhead daily file switching
- **Size Monitoring**: Efficient file size checking before rotation

## File Permissions

- Log directory: `0755` (drwxr-xr-x)
- Log files: `0644` (-rw-r--r--)

## Error Handling

The logging system gracefully handles errors:
- If file logging fails, falls back to console only
- If directory creation fails, returns initialization error
- If log rotation fails, continues with new file
- If old log cleanup fails, logs warning but continues

## Migration from Old Version

Old code using simple functions remains compatible:
```go
// These functions continue to work unchanged
log.Debug("message")
log.DebugF("format %s", "value")
log.InfoF("format %s", "value") 
log.ErrorF("format %s", "value")
```

New features are available through new functions:
```go
log.Warn("warning")      // New warning level
log.Error("error")       // Simple error (vs ErrorF)
log.InfoMessage("info")  // Clearer naming
log.Fatal("fatal")       // New fatal level
```