package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	selfStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	remoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	systemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

type phase int

const (
	phaseLogin phase = iota
	phaseConnecting
	phaseChat
)

type chatEntry struct {
	At      time.Time
	Kind    string
	Content string
	Meta    map[string]any
}

type connectedMsg struct {
	client *appClient
}

type connectFailedMsg struct{ err error }
type receivedMsg struct {
	client   *appClient
	envelope pushEnvelope
}
type receiveFailedMsg struct {
	client *appClient
	err    error
}
type sentMsg struct{ content string }
type sendFailedMsg struct {
	content string
	err     error
}

type model struct {
	phase       phase
	width       int
	height      int
	focus       int
	loginInputs []textinput.Model
	input       textinput.Model
	viewport    viewport.Model
	entries     []chatEntry
	client      *appClient
	logger      *messageLogger
	status      string
	errText     string
}

func newModel(cfg clientConfig, logger *messageLogger) model {
	fields := []struct {
		placeholder string
		value       string
		password    bool
	}{
		{"App-agent URL", cfg.BaseURL, false},
		{"User ID", cfg.UserID, false},
		{"Password", cfg.Password, true},
		{"Receive token (optional)", cfg.ReceiveToken, true},
	}
	inputs := make([]textinput.Model, 0, len(fields))
	for index, field := range fields {
		input := textinput.New()
		input.Placeholder = field.placeholder
		input.SetValue(field.value)
		input.CharLimit = 512
		input.Width = 56
		if field.password {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '•'
		}
		if index == 0 {
			input.Focus()
		}
		inputs = append(inputs, input)
	}

	messageInput := textinput.New()
	messageInput.Placeholder = "输入消息，Enter 发送，/help 查看命令"
	messageInput.CharLimit = 16 * 1024

	return model{
		phase:       phaseLogin,
		loginInputs: inputs,
		input:       messageInput,
		viewport:    viewport.New(80, 20),
		logger:      logger,
		status:      "填写连接信息后按 Enter，消息日志: " + logger.path,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.closeClient()
			return m, tea.Quit
		}
		switch m.phase {
		case phaseLogin:
			return m.updateLogin(msg)
		case phaseChat:
			return m.updateChat(msg)
		}
	case connectedMsg:
		m.client = msg.client
		m.phase = phaseChat
		m.errText = ""
		m.status = "已连接 " + msg.client.cfg.BaseURL + "，用户 " + msg.client.cfg.UserID
		m.input.Focus()
		m.appendEntry(chatEntry{At: time.Now(), Kind: "system", Content: "登录和 WebSocket 连接成功"})
		return m, readMessageCmd(m.client)
	case connectFailedMsg:
		m.phase = phaseLogin
		m.errText = msg.err.Error()
		m.status = "连接失败"
		m.loginInputs[m.focus].Focus()
	case receivedMsg:
		if msg.client != m.client {
			return m, nil
		}
		m.appendEnvelope(msg.envelope)
		return m, readMessageCmd(m.client)
	case receiveFailedMsg:
		if msg.client != m.client {
			return m, nil
		}
		m.appendEntry(chatEntry{At: time.Now(), Kind: "error", Content: "WebSocket 已断开: " + msg.err.Error()})
		m.status = "WebSocket 已断开，使用 /reconnect 重连"
		m.closeClient()
	case sentMsg:
		m.appendEntry(chatEntry{At: time.Now(), Kind: "self", Content: msg.content})
		m.status = "消息已提交"
	case sendFailedMsg:
		m.appendEntry(chatEntry{At: time.Now(), Kind: "error", Content: "发送失败: " + msg.err.Error()})
		m.status = "发送失败"
	}

	var cmd tea.Cmd
	if m.phase == phaseConnecting {
		return m, nil
	}
	if m.phase == phaseChat {
		m.viewport, cmd = m.viewport.Update(message)
		return m, cmd
	}
	return m, nil
}

func (m model) updateLogin(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "tab", "down":
		m.focus = (m.focus + 1) % len(m.loginInputs)
	case "shift+tab", "up":
		m.focus = (m.focus - 1 + len(m.loginInputs)) % len(m.loginInputs)
	case "enter":
		if m.focus < len(m.loginInputs)-1 {
			m.focus++
		} else {
			return m.startConnect()
		}
	}

	cmds := make([]tea.Cmd, 0, len(m.loginInputs))
	for index := range m.loginInputs {
		if index == m.focus {
			m.loginInputs[index].Focus()
		} else {
			m.loginInputs[index].Blur()
		}
		var cmd tea.Cmd
		m.loginInputs[index], cmd = m.loginInputs[index].Update(key)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) startConnect() (tea.Model, tea.Cmd) {
	cfg := clientConfig{
		BaseURL:      m.loginInputs[0].Value(),
		UserID:       m.loginInputs[1].Value(),
		Password:     m.loginInputs[2].Value(),
		ReceiveToken: m.loginInputs[3].Value(),
	}
	for index := range m.loginInputs {
		m.loginInputs[index].Blur()
	}
	m.phase = phaseConnecting
	m.errText = ""
	m.status = "正在登录并连接 WebSocket..."
	return m, connectCmd(cfg, m.logger)
}

func (m model) updateChat(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.closeClient()
		m.phase = phaseLogin
		m.status = "已断开，修改连接信息后按 Enter"
		m.input.Blur()
		m.loginInputs[m.focus].Focus()
		return m, nil
	case "enter":
		content := strings.TrimSpace(m.input.Value())
		if content == "" {
			return m, nil
		}
		m.input.SetValue("")
		switch content {
		case "/quit", "/exit":
			m.closeClient()
			return m, tea.Quit
		case "/clear":
			m.entries = nil
			m.refreshViewport()
			m.status = "消息已清空"
			return m, nil
		case "/help":
			m.appendEntry(chatEntry{At: time.Now(), Kind: "system", Content: "命令: /clear 清屏，/meta 显示最近消息元数据，/reconnect 重连，/quit 退出；Esc 返回登录页"})
			return m, nil
		case "/meta":
			m.showLatestMeta()
			return m, nil
		case "/reconnect":
			cfg := m.client.cfg
			m.closeClient()
			m.phase = phaseConnecting
			m.status = "正在重新登录并连接..."
			return m, connectCmd(cfg, m.logger)
		default:
			m.status = "正在发送..."
			return m, sendMessageCmd(m.client, content)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	return m, cmd
}

func (m *model) resize() {
	width := max(30, m.width-4)
	m.input.Width = max(20, width-2)
	m.viewport.Width = width
	m.viewport.Height = max(5, m.height-8)
	for index := range m.loginInputs {
		m.loginInputs[index].Width = min(72, max(24, m.width-8))
	}
	m.refreshViewport()
}

func (m *model) appendEnvelope(envelope pushEnvelope) {
	kind := "remote"
	if envelope.MessageType == "system" {
		kind = "system"
	}
	content := envelope.Content
	if envelope.MessageType != "" && envelope.MessageType != "text" && envelope.MessageType != "system" {
		content = "[" + envelope.MessageType + "] " + content
	}
	at := time.Now()
	if envelope.Timestamp > 0 {
		at = time.UnixMilli(envelope.Timestamp)
	}
	m.appendEntry(chatEntry{At: at, Kind: kind, Content: content, Meta: envelope.Meta})
	m.status = fmt.Sprintf("收到消息 seq=%d channel=%s", envelope.Sequence, envelope.Channel)
}

func (m *model) appendEntry(entry chatEntry) {
	m.entries = append(m.entries, entry)
	if len(m.entries) > 1000 {
		m.entries = append([]chatEntry(nil), m.entries[len(m.entries)-1000:]...)
	}
	m.refreshViewport()
	m.viewport.GotoBottom()
}

func (m *model) showLatestMeta() {
	for index := len(m.entries) - 1; index >= 0; index-- {
		if len(m.entries[index].Meta) == 0 {
			continue
		}
		keys := make([]string, 0, len(m.entries[index].Meta))
		for key := range m.entries[index].Meta {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", key, m.entries[index].Meta[key]))
		}
		m.appendEntry(chatEntry{At: time.Now(), Kind: "system", Content: "最近消息 meta: " + strings.Join(parts, ", ")})
		return
	}
	m.appendEntry(chatEntry{At: time.Now(), Kind: "system", Content: "暂无消息元数据"})
}

func (m *model) refreshViewport() {
	lines := make([]string, 0, len(m.entries))
	for _, entry := range m.entries {
		prefix := entry.At.Format("15:04:05") + " "
		switch entry.Kind {
		case "self":
			lines = append(lines, selfStyle.Render(prefix+"你: ")+entry.Content)
		case "remote":
			lines = append(lines, remoteStyle.Render(prefix+"app-agent: ")+entry.Content)
		case "error":
			lines = append(lines, errorStyle.Render(prefix+"错误: "+entry.Content))
		default:
			lines = append(lines, systemStyle.Render(prefix+"系统: ")+entry.Content)
		}
	}
	m.viewport.SetContent(strings.Join(lines, "\n\n"))
}

func (m *model) closeClient() {
	if m.client != nil {
		m.client.close()
		m.client = nil
	}
}

func (m model) View() string {
	switch m.phase {
	case phaseLogin:
		return m.loginView()
	case phaseConnecting:
		return "\n  " + titleStyle.Render("App-Agent TUI") + "\n\n  " + m.status + "\n"
	default:
		return m.chatView()
	}
}

func (m model) loginView() string {
	var builder strings.Builder
	builder.WriteString("\n  " + titleStyle.Render("App-Agent TUI") + "\n")
	builder.WriteString("  " + statusStyle.Render("用于快速测试登录、消息发送、WebSocket 推送和 ACK") + "\n\n")
	labels := []string{"地址", "用户", "密码", "接收令牌"}
	for index, input := range m.loginInputs {
		builder.WriteString("  " + labelStyle.Render(labels[index]) + "\n")
		builder.WriteString("  " + input.View() + "\n\n")
	}
	if m.errText != "" {
		builder.WriteString("  " + errorStyle.Render(m.errText) + "\n\n")
	}
	builder.WriteString("  " + statusStyle.Render("Tab/↑/↓ 切换字段，最后一项按 Enter 连接，Ctrl+C 退出"))
	return builder.String()
}

func (m model) chatView() string {
	header := titleStyle.Render("App-Agent TUI") + "  " + statusStyle.Render(m.status)
	footer := m.input.View() + "\n" + statusStyle.Render("Enter 发送  PgUp/PgDn 滚动  Esc 返回登录页  Ctrl+C 退出")
	return header + "\n\n" + m.viewport.View() + "\n\n" + footer
}

func connectCmd(cfg clientConfig, logger *messageLogger) tea.Cmd {
	return func() tea.Msg {
		client := newAppClient(cfg, logger)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := client.loginAndConnect(ctx); err != nil {
			client.close()
			return connectFailedMsg{err: err}
		}
		return connectedMsg{client: client}
	}
}

func readMessageCmd(client *appClient) tea.Cmd {
	return func() tea.Msg {
		envelope, err := client.readMessage()
		if err != nil {
			return receiveFailedMsg{client: client, err: err}
		}
		return receivedMsg{client: client, envelope: envelope}
	}
}

func sendMessageCmd(client *appClient, content string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := client.sendMessage(ctx, content); err != nil {
			return sendFailedMsg{content: content, err: err}
		}
		return sentMsg{content: content}
	}
}

func main() {
	baseURL := flag.String("url", "http://blog.guccang.cn:8883", "app-agent base URL")
	userID := flag.String("user", "ztt", "user ID")
	password := flag.String("password", "", "password")
	token := flag.String("token", "123456", "app-agent receive token")
	logFile := flag.String("log", "logs/app-agent-tui.jsonl", "UTF-8 JSONL message log file")
	flag.Parse()

	logger, err := newMessageLogger(*logFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer logger.close()

	app := newModel(clientConfig{
		BaseURL:      *baseURL,
		UserID:       *userID,
		Password:     *password,
		ReceiveToken: *token,
	}, logger)
	program := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
