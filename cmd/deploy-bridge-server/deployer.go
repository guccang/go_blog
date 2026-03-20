package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogEntry 部署日志条目
type LogEntry struct {
	Time  string `json:"time"`
	Level string `json:"level"` // info / error / done
	Text  string `json:"text"`
}

// DeployTask 部署任务
type DeployTask struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	TargetDir  string    `json:"target_dir"`
	Script     string    `json:"script"`
	Status     string    `json:"status"` // pending / running / done / error
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Error      string    `json:"error,omitempty"`

	mu          sync.Mutex
	logs        []LogEntry
	subscribers map[chan LogEntry]bool
}

// addLog 添加日志并推送给所有 SSE 订阅者
func (t *DeployTask) addLog(level, text string) {
	entry := LogEntry{
		Time:  time.Now().Format("15:04:05"),
		Level: level,
		Text:  text,
	}
	t.mu.Lock()
	t.logs = append(t.logs, entry)
	// 复制 subscribers 避免持锁发送
	subs := make([]chan LogEntry, 0, len(t.subscribers))
	for ch := range t.subscribers {
		subs = append(subs, ch)
	}
	t.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- entry:
		default: // 订阅者慢，丢弃
		}
	}
}

// subscribe 订阅日志流，返回 channel 和已有日志快照
func (t *DeployTask) subscribe() (chan LogEntry, []LogEntry) {
	ch := make(chan LogEntry, 64)
	t.mu.Lock()
	snapshot := make([]LogEntry, len(t.logs))
	copy(snapshot, t.logs)
	if t.subscribers == nil {
		t.subscribers = make(map[chan LogEntry]bool)
	}
	t.subscribers[ch] = true
	t.mu.Unlock()
	return ch, snapshot
}

// unsubscribe 取消订阅
func (t *DeployTask) unsubscribe(ch chan LogEntry) {
	t.mu.Lock()
	delete(t.subscribers, ch)
	t.mu.Unlock()
	close(ch)
}

// getLogs 获取全量日志
func (t *DeployTask) getLogs() []LogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]LogEntry, len(t.logs))
	copy(out, t.logs)
	return out
}

// DeployManager 部署管理器
type DeployManager struct {
	cfg       *Config
	mu        sync.Mutex
	tasks     map[string]*DeployTask
	taskOrder []string // 按时间倒序
	md5Cache  map[string]string // filename → md5
}

// NewDeployManager 创建部署管理器
func NewDeployManager(cfg *Config) *DeployManager {
	m := &DeployManager{
		cfg:      cfg,
		tasks:    make(map[string]*DeployTask),
		md5Cache: make(map[string]string),
	}
	m.initMD5Cache()
	return m
}

// initMD5Cache 启动时扫描 upload 目录，缓存已有文件的 MD5
func (m *DeployManager) initMD5Cache() {
	entries, err := os.ReadDir(m.cfg.UploadDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		filePath := filepath.Join(m.cfg.UploadDir, e.Name())
		hash, err := fileMD5(filePath)
		if err != nil {
			continue
		}
		m.md5Cache[e.Name()] = hash
	}
}

// FindDuplicateByMD5 查找是否已有相同 MD5 的文件
func (m *DeployManager) FindDuplicateByMD5(md5sum string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, hash := range m.md5Cache {
		if hash == md5sum {
			return name, true
		}
	}
	return "", false
}

// CacheMD5 缓存文件 MD5
func (m *DeployManager) CacheMD5(filename, md5sum string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.md5Cache[filename] = md5sum
}

// GetMD5 获取缓存的文件 MD5
func (m *DeployManager) GetMD5(filename string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.md5Cache[filename]
}

// CreateTask 创建部署任务
func (m *DeployManager) CreateTask(filename, targetDir, script string) *DeployTask {
	id := fmt.Sprintf("d_%d_%s", time.Now().Unix(), randStr(6))
	task := &DeployTask{
		ID:          id,
		Filename:    filename,
		TargetDir:   targetDir,
		Script:      script,
		Status:      "pending",
		StartedAt:   time.Now(),
		subscribers: make(map[chan LogEntry]bool),
	}

	m.mu.Lock()
	m.tasks[id] = task
	m.taskOrder = append([]string{id}, m.taskOrder...)
	// 保留最近 N 条
	if len(m.taskOrder) > m.cfg.LogRetainCount {
		removeID := m.taskOrder[len(m.taskOrder)-1]
		m.taskOrder = m.taskOrder[:len(m.taskOrder)-1]
		delete(m.tasks, removeID)
	}
	m.mu.Unlock()

	return task
}

// GetTask 获取任务
func (m *DeployManager) GetTask(id string) *DeployTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tasks[id]
}

// ListTasks 获取任务列表（按时间倒序）
func (m *DeployManager) ListTasks() []*DeployTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*DeployTask
	for _, id := range m.taskOrder {
		if t, ok := m.tasks[id]; ok {
			out = append(out, t)
		}
	}
	return out
}

// RunDeploy 执行部署（异步调用）
func (m *DeployManager) RunDeploy(task *DeployTask) {
	task.Status = "running"
	task.addLog("info", fmt.Sprintf("开始部署: %s → %s", task.Filename, task.TargetDir))

	zipSrc := filepath.Join(m.cfg.UploadDir, task.Filename)
	targetDir := filepath.Clean(task.TargetDir)

	// 1. 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		m.failTask(task, fmt.Sprintf("创建目标目录失败: %v", err))
		return
	}

	// 2. 复制 zip 到目标目录
	zipDst := filepath.Join(targetDir, task.Filename)
	task.addLog("info", fmt.Sprintf("复制 %s → %s", task.Filename, targetDir))
	if err := copyFileSimple(zipSrc, zipDst); err != nil {
		m.failTask(task, fmt.Sprintf("复制文件失败: %v", err))
		return
	}

	// 3. 解压
	task.addLog("info", fmt.Sprintf("解压 %s...", task.Filename))
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(m.cfg.DeployTimeout)*time.Second)
	defer cancel()

	if err := m.runCmd(ctx, task, targetDir, "unzip", "-o", task.Filename); err != nil {
		m.failTask(task, fmt.Sprintf("解压失败: %v", err))
		return
	}

	// 4. 执行部署脚本
	if task.Script != "" {
		scriptPath := filepath.Join(targetDir, task.Script)
		if _, err := os.Stat(scriptPath); err != nil {
			m.failTask(task, fmt.Sprintf("脚本不存在: %s", task.Script))
			return
		}

		task.addLog("info", fmt.Sprintf("执行脚本: %s", task.Script))
		// chmod +x
		_ = os.Chmod(scriptPath, 0755)

		if err := m.runCmd(ctx, task, targetDir, "bash", task.Script); err != nil {
			m.failTask(task, fmt.Sprintf("脚本执行失败: %v", err))
			return
		}
	}

	// 完成
	task.Status = "done"
	task.FinishedAt = time.Now()
	elapsed := task.FinishedAt.Sub(task.StartedAt)
	task.addLog("done", fmt.Sprintf("部署完成，耗时 %.1fs", elapsed.Seconds()))
}

// runCmd 执行命令，实时采集 stdout/stderr 到日志
func (m *DeployManager) runCmd(ctx context.Context, task *DeployTask, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // 合并 stderr 到 stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		task.addLog("info", scanner.Text())
	}

	return cmd.Wait()
}

// failTask 标记任务失败
func (m *DeployManager) failTask(task *DeployTask, errMsg string) {
	task.Status = "error"
	task.Error = errMsg
	task.FinishedAt = time.Now()
	task.addLog("error", errMsg)
}

// copyFileSimple 简单文件复制
func copyFileSimple(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// randStr 生成随机字符串
func randStr(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// ListPackages 列出已上传的 zip 包（按时间倒序）
func (m *DeployManager) ListPackages() []PackageInfo {
	entries, err := os.ReadDir(m.cfg.UploadDir)
	if err != nil {
		return nil
	}

	var pkgs []PackageInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		pkgs = append(pkgs, PackageInfo{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			MD5:     m.GetMD5(e.Name()),
		})
	}

	// 按时间倒序
	for i, j := 0, len(pkgs)-1; i < j; i, j = i+1, j-1 {
		pkgs[i], pkgs[j] = pkgs[j], pkgs[i]
	}

	return pkgs
}

// PackageInfo 包信息
type PackageInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	MD5     string    `json:"md5"`
}

// fileMD5 计算文件的 MD5 哈希
func fileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
