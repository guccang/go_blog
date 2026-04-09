package llm

import (
	"codegen"
	"fmt"
	log "mylog"
	"net/http"
	"net/url"
	"time"
)

// ProcessRequestViaAgent 通过 llm-agent 处理 assistant 请求。
// 发送 MsgTaskAssign，监听 MsgTaskEvent 流式返回，写入 SSE。
func ProcessRequestViaAgent(account, query string, w http.ResponseWriter, flusher http.Flusher) error {
	// 检查 gateway 连接
	if !codegen.IsLLMAgentOnline() {
		return fmt.Errorf("llm-agent 不在线，请检查 gateway 和 agent 状态")
	}

	// 生成 taskID
	taskID := fmt.Sprintf("ast_%d", time.Now().UnixNano())

	// 注册事件监听
	eventCh := codegen.RegisterTaskListener(taskID)
	if eventCh == nil {
		return fmt.Errorf("gateway bridge 未初始化")
	}
	defer codegen.UnregisterTaskListener(taskID)

	// 构建 payload 并发送 MsgTaskAssign
	taskPayload := map[string]interface{}{
		"task_type": "assistant_chat",
		"query":     query,
		"account":   account,
	}

	if err := codegen.SendTaskToLLMAgent(taskID, taskPayload); err != nil {
		return fmt.Errorf("发送任务失败: %v", err)
	}

	log.InfoF(log.ModuleLLM, "Assistant task sent to llm-agent: task=%s account=%s", taskID, account)

	// 监听事件，流式写入 SSE
	timeout := time.After(10 * time.Minute)

	for {
		select {
		case evt, ok := <-eventCh:
			if !ok {
				// channel 被关闭
				return nil
			}

			if evt.Complete {
				// 任务完成
				if evt.Error != "" {
					log.WarnF(log.ModuleLLM, "Assistant task failed: task=%s error=%s", taskID, evt.Error)
				}
				return nil
			}

			// 流式事件
			switch evt.Event {
			case "chunk":
				// LLM 文本 chunk → SSE
				fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(evt.Text))
				flusher.Flush()
			case "tool_info":
				// 工具调用信息 → SSE
				fmt.Fprintf(w, "data: %s\n\n", url.QueryEscape(evt.Text))
				flusher.Flush()
			}

		case <-timeout:
			log.WarnF(log.ModuleLLM, "Assistant task timeout: task=%s", taskID)
			return fmt.Errorf("任务超时（10分钟）")
		}
	}
}
