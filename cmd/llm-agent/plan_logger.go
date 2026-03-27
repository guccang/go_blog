package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PlanLogger 专用于 plan_and_execute 任务的日志记录器
type PlanLogger struct {
	file      *os.File
	mu        sync.Mutex
	startTime time.Time
}

// CreatePlanLogger 创建日志记录器
func CreatePlanLogger(taskID string) (*PlanLogger, error) {
	// 创建日志目录
	logDir := filepath.Join("logs", "plan_and_execute")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 生成日志文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", taskID, timestamp))

	// 创建日志文件
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}

	return &PlanLogger{
		file:      file,
		startTime: time.Now(),
	}, nil
}

// LogStart 记录任务开始
func (l *PlanLogger) LogStart(title, account string, plan *TaskPlan) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(l.file, "========================================\n")
	fmt.Fprintf(l.file, "Plan Execution Started\n")
	fmt.Fprintf(l.file, "========================================\n")
	fmt.Fprintf(l.file, "Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(l.file, "Title: %s\n", title)
	fmt.Fprintf(l.file, "Account: %s\n", account)
	fmt.Fprintf(l.file, "\nPlan Overview:\n")
	fmt.Fprintf(l.file, "- Mode: %s\n", plan.ExecutionMode)
	fmt.Fprintf(l.file, "- Total SubTasks: %d\n", len(plan.SubTasks))
	fmt.Fprintf(l.file, "\nSubTasks:\n")
	for i, st := range plan.SubTasks {
		fmt.Fprintf(l.file, "  [%d] %s: %s\n", i+1, st.ID, st.Title)
		fmt.Fprintf(l.file, "      Dependencies: %v\n", st.DependsOn)
	}
	fmt.Fprintf(l.file, "\n========================================\n\n")
	l.file.Sync()
}

// LogSubTaskStart 记录子任务开始
func (l *PlanLogger) LogSubTaskStart(subTaskID, title string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(l.file, "[%s] SubTask Started: [%s] %s\n\n", timestamp, subTaskID, title)
	l.file.Sync()
}

// LogSubTaskEnd 记录子任务结束
func (l *PlanLogger) LogSubTaskEnd(subTaskID, status, result string, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(l.file, "[%s] SubTask Completed: [%s]\n", timestamp, subTaskID)
	fmt.Fprintf(l.file, "  Status: %s\n", status)
	fmt.Fprintf(l.file, "  Duration: %s\n", formatDuration(duration))
	fmt.Fprintf(l.file, "  Result: %s\n\n", truncateResult(result))
	l.file.Sync()
}

// LogEnd 记录任务结束
func (l *PlanLogger) LogEnd(results []SubTaskResult, totalDuration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(l.file, "========================================\n")
	fmt.Fprintf(l.file, "Plan Execution Completed\n")
	fmt.Fprintf(l.file, "========================================\n")
	fmt.Fprintf(l.file, "Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(l.file, "Total Duration: %s\n", formatDuration(totalDuration))
	fmt.Fprintf(l.file, "\nResults Summary:\n")

	successCount := 0
	for _, r := range results {
		symbol := "✓"
		if r.Status != "done" {
			symbol = "✗"
		} else {
			successCount++
		}
		fmt.Fprintf(l.file, "  %s %s: %s\n", symbol, r.SubTaskID, r.Status)
	}

	fmt.Fprintf(l.file, "\nSuccess Rate: %d/%d (%.0f%%)\n", successCount, len(results), float64(successCount)/float64(len(results))*100)
	fmt.Fprintf(l.file, "========================================\n")
	l.file.Sync()
}

// Close 关闭日志文件
func (l *PlanLogger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// truncateResult 截断过长的结果文本
func truncateResult(result string) string {
	maxLen := 200
	if len(result) <= maxLen {
		return result
	}
	return result[:maxLen] + "..."
}
