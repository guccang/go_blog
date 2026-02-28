package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"
)

// Pipeline codegen → deploy → verify 流水线
type Pipeline struct {
	deployPath    string // deploy-agent.exe 路径
	deployConfig  string // deploy.conf 路径
	verifyURL     string // 验证 URL（HTTP GET）
	verifyTimeout int    // 验证超时秒数
}

// sendEventFunc 发送 stream event 的回调
type sendEventFunc func(sessionID, eventType, text string)

// Run 执行 deploy + verify 流水线
func (p *Pipeline) Run(sessionID string, sendEvent sendEventFunc) error {
	sendEvent(sessionID, "system", "🚀 开始自动部署...")

	if err := p.deploy(sessionID, sendEvent); err != nil {
		sendEvent(sessionID, "error", fmt.Sprintf("❌ 部署失败: %v", err))
		return err
	}

	sendEvent(sessionID, "system", "✅ 部署完成")

	if p.verifyURL != "" {
		sendEvent(sessionID, "system", "⏳ 等待服务启动 (5s)...")
		time.Sleep(5 * time.Second)

		if err := p.verify(sessionID); err != nil {
			sendEvent(sessionID, "error", fmt.Sprintf("❌ 验证失败: %v", err))
			return err
		}
		sendEvent(sessionID, "system", "✅ 部署验证通过（HTTP 200）")
	}

	return nil
}

// deploy 执行 deploy-agent
func (p *Pipeline) deploy(sessionID string, sendEvent sendEventFunc) error {
	args := []string{"--config", p.deployConfig}
	log.Printf("[PIPELINE] exec: %s %v", p.deployPath, args)

	cmd := exec.Command(p.deployPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start deploy-agent: %v", err)
	}

	// 逐行转发 stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[DEPLOY] %s", line)
			sendEvent(sessionID, "system", "📦 "+line)
		}
	}()

	// 逐行转发 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[DEPLOY-ERR] %s", line)
			sendEvent(sessionID, "system", "⚠️ "+line)
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("deploy-agent exit: %v", err)
	}

	return nil
}

// verify HTTP GET 验证部署结果
func (p *Pipeline) verify(sessionID string) error {
	timeout := p.verifyTimeout
	if timeout <= 0 {
		timeout = 10
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Get(p.verifyURL)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}
