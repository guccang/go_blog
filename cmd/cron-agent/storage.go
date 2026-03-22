package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Storage 任务存储
type Storage struct {
	cfg        *Config
	tasksPath  string
	backupDir  string
	mu         sync.RWMutex
}

// NewStorage 创建存储实例
func NewStorage(cfg *Config) *Storage {
	// 确保目录存在
	os.MkdirAll(filepath.Dir(cfg.Storage.Path), 0755)
	if cfg.Storage.BackupDir != "" {
		os.MkdirAll(cfg.Storage.BackupDir, 0755)
	}

	return &Storage{
		cfg:       cfg,
		tasksPath: cfg.Storage.Path,
		backupDir: cfg.Storage.BackupDir,
	}
}

// LoadTasks 从文件加载所有任务
func (s *Storage) LoadTasks() ([]*CronTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.tasksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*CronTask{}, nil
		}
		return nil, fmt.Errorf("读取任务文件失败: %v", err)
	}

	var tasks []*CronTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("解析任务文件失败: %v", err)
	}

	return tasks, nil
}

// SaveTasks 保存所有任务到文件
func (s *Storage) SaveTasks(tasks []*CronTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 备份旧文件
	if s.backupDir != "" {
		if err := s.backup(); err != nil {
			log.Printf("[Storage] 备份失败: %v", err)
		}
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化任务失败: %v", err)
	}

	tmpPath := s.tasksPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 原子性替换
	if err := os.Rename(tmpPath, s.tasksPath); err != nil {
		return fmt.Errorf("重命名文件失败: %v", err)
	}

	return nil
}

// backup 备份当前任务文件
func (s *Storage) backup() error {
	if _, err := os.Stat(s.tasksPath); os.IsNotExist(err) {
		return nil // 文件不存在，无需备份
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(s.backupDir, fmt.Sprintf("tasks_%s.json", timestamp))

	src, err := os.Open(s.tasksPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// CreateTask 创建新任务
func (s *Storage) CreateTask(req *TaskCreateRequest) (*CronTask, error) {
	tasks, err := s.LoadTasks()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	task := &CronTask{
		ID:           uuid.New().String(),
		Name:         req.Name,
		ScheduleType: req.ScheduleType,
		CronExpr:     req.CronExpr,
		IntervalSec:  req.IntervalSec,
		TargetAgent:  req.TargetAgent,
		TaskType:     req.TaskType,
		Payload:      req.Payload,
		Enabled:      req.Enabled,
		Status:       TaskStatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 一次性任务：根据 delay_sec 计算 NextRunAt
	if req.ScheduleType == ScheduleTypeOnce && req.DelaySec > 0 {
		task.NextRunAt = now.Add(time.Duration(req.DelaySec) * time.Second)
	}

	// 设置默认值
	if task.TargetAgent == "" {
		task.TargetAgent = "llm-agent"
	}
	if !task.Enabled {
		task.Enabled = true
	}

	tasks = append(tasks, task)

	if err := s.SaveTasks(tasks); err != nil {
		return nil, err
	}

	return task, nil
}

// GetTask 获取任务
func (s *Storage) GetTask(taskID string) (*CronTask, error) {
	tasks, err := s.LoadTasks()
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		if task.ID == taskID {
			return task, nil
		}
	}

	return nil, fmt.Errorf("任务不存在: %s", taskID)
}

// UpdateTask 更新任务
func (s *Storage) UpdateTask(taskID string, req *TaskUpdateRequest) (*CronTask, error) {
	tasks, err := s.LoadTasks()
	if err != nil {
		return nil, err
	}

	for i, task := range tasks {
		if task.ID == taskID {
			// 更新字段
			if req.Name != nil {
				task.Name = *req.Name
			}
			if req.ScheduleType != nil {
				task.ScheduleType = *req.ScheduleType
			}
			if req.CronExpr != nil {
				task.CronExpr = *req.CronExpr
			}
			if req.IntervalSec != nil {
				task.IntervalSec = *req.IntervalSec
			}
			if req.TargetAgent != nil {
				task.TargetAgent = *req.TargetAgent
			}
			if req.TaskType != nil {
				task.TaskType = *req.TaskType
			}
			if req.Payload != nil {
				task.Payload = *req.Payload
			}
			if req.Enabled != nil {
				task.Enabled = *req.Enabled
				if !task.Enabled {
					task.Status = TaskStatusDisabled
				} else if task.Status == TaskStatusDisabled {
					task.Status = TaskStatusPending
				}
			}
			task.UpdatedAt = time.Now()

			tasks[i] = task

			if err := s.SaveTasks(tasks); err != nil {
				return nil, err
			}

			return task, nil
		}
	}

	return nil, fmt.Errorf("任务不存在: %s", taskID)
}

// DeleteTask 删除任务
func (s *Storage) DeleteTask(taskID string) error {
	tasks, err := s.LoadTasks()
	if err != nil {
		return err
	}

	newTasks := []*CronTask{}
	found := false
	for _, task := range tasks {
		if task.ID == taskID {
			found = true
			continue
		}
		newTasks = append(newTasks, task)
	}

	if !found {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	return s.SaveTasks(newTasks)
}

// ListTasks 列出所有任务
func (s *Storage) ListTasks() ([]*CronTask, error) {
	return s.LoadTasks()
}

// UpdateTaskStatus 更新任务状态
func (s *Storage) UpdateTaskStatus(taskID string, status TaskStatus) error {
	tasks, err := s.LoadTasks()
	if err != nil {
		return err
	}

	for i, task := range tasks {
		if task.ID == taskID {
			task.Status = status
			task.UpdatedAt = time.Now()
			tasks[i] = task
			return s.SaveTasks(tasks)
		}
	}

	return fmt.Errorf("任务不存在: %s", taskID)
}

// UpdateTaskNextRun 更新任务下次执行时间
func (s *Storage) UpdateTaskNextRun(taskID string, nextRunAt time.Time) error {
	tasks, err := s.LoadTasks()
	if err != nil {
		return err
	}

	for i, task := range tasks {
		if task.ID == taskID {
			task.NextRunAt = nextRunAt
			task.UpdatedAt = time.Now()
			tasks[i] = task
			return s.SaveTasks(tasks)
		}
	}

	return fmt.Errorf("任务不存在: %s", taskID)
}

// SaveExecution 保存执行记录
func (s *Storage) SaveExecution(execution *TaskExecution) error {
	// 执行记录存储在单独的文件中
	executionsPath := filepath.Join(filepath.Dir(s.tasksPath), "executions.json")

	var executions []*TaskExecution
	data, err := os.ReadFile(executionsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("读取执行记录失败: %v", err)
		}
		// 文件不存在，创建新列表
		executions = []*TaskExecution{}
	} else {
		if err := json.Unmarshal(data, &executions); err != nil {
			return fmt.Errorf("解析执行记录失败: %v", err)
		}
	}

	// 只保留最近1000条记录
	executions = append(executions, execution)
	if len(executions) > 1000 {
		executions = executions[len(executions)-1000:]
	}

	data, err = json.MarshalIndent(executions, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化执行记录失败: %v", err)
	}

	return os.WriteFile(executionsPath, data, 0644)
}

// GetTaskExecutions 获取任务执行记录
func (s *Storage) GetTaskExecutions(taskID string, limit int) ([]*TaskExecution, error) {
	executionsPath := filepath.Join(filepath.Dir(s.tasksPath), "executions.json")

	data, err := os.ReadFile(executionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*TaskExecution{}, nil
		}
		return nil, fmt.Errorf("读取执行记录失败: %v", err)
	}

	var allExecutions []*TaskExecution
	if err := json.Unmarshal(data, &allExecutions); err != nil {
		return nil, fmt.Errorf("解析执行记录失败: %v", err)
	}

	// 过滤并限制数量
	var filtered []*TaskExecution
	count := 0
	for i := len(allExecutions) - 1; i >= 0; i-- {
		if allExecutions[i].TaskID == taskID {
			filtered = append(filtered, allExecutions[i])
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
	}

	// 反转顺序，使最新的在前
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered, nil
}