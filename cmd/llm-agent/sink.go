package main

import "strings"

// ========================= EventSink 接口与实现 =========================

// EventSink 抽象不同来源的输出差异
type EventSink interface {
	OnChunk(text string)        // LLM 文本片段
	OnEvent(event, text string) // 结构化事件 (tool_info, thinking, tool_call, tool_result, subtask_*, skill_*, etc.)
	Streaming() bool            // 是否使用流式 LLM 调用
}

// StreamingSink Web 前端流式输出
type StreamingSink struct {
	bridge *Bridge
	taskID string
	target string
}

func (s *StreamingSink) OnChunk(text string) {
	s.bridge.sendTaskEventTo(s.taskID, s.target, "chunk", text)
}
func (s *StreamingSink) OnEvent(event, text string) {
	s.bridge.sendTaskEventTo(s.taskID, s.target, event, text)
}
func (s *StreamingSink) Streaming() bool { return true }

// BufferSink 缓冲输出（llm_request）
type BufferSink struct {
	buf strings.Builder
}

func (s *BufferSink) OnChunk(text string)        { s.buf.WriteString(text) }
func (s *BufferSink) OnEvent(event, text string) {}
func (s *BufferSink) Streaming() bool            { return false }
func (s *BufferSink) Result() string             { return s.buf.String() }

// LLMRequestSink 缓冲文本 + 转发事件（用于 llm_request 任务，支持 Path 2 进度推送）
type LLMRequestSink struct {
	buf    strings.Builder
	bridge *Bridge
	taskID string
	target string
}

func (s *LLMRequestSink) OnChunk(text string) { s.buf.WriteString(text) }
func (s *LLMRequestSink) OnEvent(event, text string) {
	s.bridge.sendTaskEventTo(s.taskID, s.target, event, text)
}
func (s *LLMRequestSink) Streaming() bool { return false }
func (s *LLMRequestSink) Result() string  { return s.buf.String() }
