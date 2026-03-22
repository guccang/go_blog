package main

import (
	"encoding/json"
	"time"
)

// ScheduleType 任务调度类型
type ScheduleType string

const (
	ScheduleTypeCron    ScheduleType = "cron"    // cron表达式
	ScheduleTypeOnce    ScheduleType = "once"    // 一次性任务
	ScheduleTypeInterval ScheduleType = "interval" // 间隔任务
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 待执行
	TaskStatusRunning   TaskStatus = "running"   // 执行中
	TaskStatusCompleted TaskStatus = "completed" // 已完成
	TaskStatusFailed    TaskStatus = "failed"    // 失败
	TaskStatusDisabled  TaskStatus = "disabled"  // 禁用
)

// ExecutionStatus 执行状态
type ExecutionStatus string

const (
	ExecutionStatusSuccess   ExecutionStatus = "success"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// CronTask 定时任务定义
type CronTask struct {
	ID           string          `json:"id"`            // 唯一标识
	Name         string          `json:"name"`          // 任务名称
	ScheduleType ScheduleType    `json:"schedule_type"` // "cron" | "once" | "interval"
	CronExpr     string          `json:"cron_expr"`     // cron表达式（schedule_type=cron时有效）
	IntervalSec  int64           `json:"interval_sec"`  // 间隔秒数（schedule_type=interval时有效）
	TargetAgent  string          `json:"target_agent"`  // 目标agent（默认"llm-agent"）
	TaskType     string          `json:"task_type"`     // llm-agent任务类型
	Payload      json.RawMessage `json:"payload"`       // 任务内容
	Enabled      bool            `json:"enabled"`       // 是否启用
	Status       TaskStatus      `json:"status"`        // 状态：pending/running/completed/failed/disabled
	NextRunAt    time.Time       `json:"next_run_at"`   // 下次执行时间
	CreatedAt    time.Time       `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time       `json:"updated_at"`    // 更新时间
}

// TaskExecution 执行记录
type TaskExecution struct {
	ID        string          `json:"id"`         // 执行记录ID
	TaskID    string          `json:"task_id"`    // 关联任务ID
	StartedAt time.Time       `json:"started_at"` // 开始时间
	EndedAt   time.Time       `json:"ended_at"`   // 结束时间
	Status    ExecutionStatus `json:"status"`     // 状态：success/failed/cancelled
	Result    string          `json:"result"`     // 执行结果摘要
	Error     string          `json:"error"`      // 错误信息
	Duration  int64           `json:"duration"`   // 执行时长(ms)
}

// TaskCreateRequest 创建任务请求
type TaskCreateRequest struct {
	Name         string          `json:"name"`
	ScheduleType ScheduleType    `json:"schedule_type"`
	CronExpr     string          `json:"cron_expr,omitempty"`
	IntervalSec  int64           `json:"interval_sec,omitempty"`
	DelaySec     int64           `json:"delay_sec,omitempty"` // 一次性任务延迟秒数
	TargetAgent  string          `json:"target_agent,omitempty"`
	TaskType     string          `json:"task_type"`
	Payload      json.RawMessage `json:"payload"`
	Enabled      bool            `json:"enabled,omitempty"`
}

// TaskUpdateRequest 更新任务请求
type TaskUpdateRequest struct {
	Name         *string          `json:"name,omitempty"`
	ScheduleType *ScheduleType    `json:"schedule_type,omitempty"`
	CronExpr     *string          `json:"cron_expr,omitempty"`
	IntervalSec  *int64           `json:"interval_sec,omitempty"`
	TargetAgent  *string          `json:"target_agent,omitempty"`
	TaskType     *string          `json:"task_type,omitempty"`
	Payload      *json.RawMessage `json:"payload,omitempty"`
	Enabled      *bool            `json:"enabled,omitempty"`
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	Tasks []*CronTask `json:"tasks"`
	Total int         `json:"total"`
}

// TaskTriggerRequest 手动触发任务请求
type TaskTriggerRequest struct {
	TaskID string `json:"task_id"`
}

// TaskStatusResponse 任务状态响应
type TaskStatusResponse struct {
	Task      *CronTask        `json:"task"`
	Executions []*TaskExecution `json:"executions,omitempty"`
}