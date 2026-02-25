package email

import (
	"config"
	"fmt"
	log "mylog"
	"net/smtp"
	"strings"
)

// EmailConfig 邮件配置
type EmailConfig struct {
	From     string
	Password string
	SmtpHost string
	SmtpPort string
	To       string // 默认收件人
	Enabled  bool
}

var globalEmailConfig *EmailConfig

// InitEmailConfig 从 sys_conf.md 初始化邮件配置
func InitEmailConfig() {
	adminAccount := config.GetAdminAccount()
	from := config.GetConfigWithAccount(adminAccount, "email_from")
	password := config.GetConfigWithAccount(adminAccount, "email_password")
	smtpHost := config.GetConfigWithAccount(adminAccount, "smtp_host")
	smtpPort := config.GetConfigWithAccount(adminAccount, "smtp_port")
	to := config.GetConfigWithAccount(adminAccount, "email_to")

	if smtpPort == "" {
		smtpPort = "587"
	}

	enabled := from != "" && password != "" && smtpHost != ""

	globalEmailConfig = &EmailConfig{
		From:     from,
		Password: password,
		SmtpHost: smtpHost,
		SmtpPort: smtpPort,
		To:       to,
		Enabled:  enabled,
	}

	if enabled {
		log.Message(log.ModuleEmail, "Email config initialized successfully")
	} else {
		log.Warn(log.ModuleEmail, "Email not configured - set email_from/email_password/smtp_host in sys_conf.md")
	}
}

// GetEmailConfig 获取邮件配置
func GetEmailConfig() *EmailConfig {
	return globalEmailConfig
}

// IsEnabled 邮件是否已配置
func IsEnabled() bool {
	return globalEmailConfig != nil && globalEmailConfig.Enabled
}

// SendEmail 发送电子邮件（使用全局配置）
func SendEmail(to string, subject string, body string) error {
	if !IsEnabled() {
		return fmt.Errorf("email not configured")
	}

	if to == "" {
		to = globalEmailConfig.To
	}
	if to == "" {
		return fmt.Errorf("no recipient specified")
	}

	return sendMail(globalEmailConfig.From, globalEmailConfig.Password,
		globalEmailConfig.SmtpHost, globalEmailConfig.SmtpPort, to, subject, body)
}

// SendHTMLEmail 发送 HTML 格式邮件
func SendHTMLEmail(to string, subject string, htmlBody string) error {
	if !IsEnabled() {
		return fmt.Errorf("email not configured")
	}

	if to == "" {
		to = globalEmailConfig.To
	}
	if to == "" {
		return fmt.Errorf("no recipient specified")
	}

	return sendHTMLMail(globalEmailConfig.From, globalEmailConfig.Password,
		globalEmailConfig.SmtpHost, globalEmailConfig.SmtpPort, to, subject, htmlBody)
}

// sendMail 发送纯文本邮件
func sendMail(from, password, smtpHost, smtpPort, to, subject, body string) error {
	message := []byte("Subject: " + subject + "\r\n" +
		"To: " + to + "\r\n" +
		"From: " + from + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
		"\r\n" + body)

	auth := smtp.PlainAuth("", from, password, smtpHost)

	recipients := strings.Split(to, ",")
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, recipients, message)
	if err != nil {
		log.WarnF(log.ModuleEmail, "Failed to send email to %s: %v", to, err)
		return err
	}
	log.MessageF(log.ModuleEmail, "Email sent to %s: %s", to, subject)
	return nil
}

// sendHTMLMail 发送 HTML 邮件
func sendHTMLMail(from, password, smtpHost, smtpPort, to, subject, htmlBody string) error {
	message := []byte("Subject: " + subject + "\r\n" +
		"To: " + to + "\r\n" +
		"From: " + from + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
		"\r\n" + htmlBody)

	auth := smtp.PlainAuth("", from, password, smtpHost)

	recipients := strings.Split(to, ",")
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, recipients, message)
	if err != nil {
		log.WarnF(log.ModuleEmail, "Failed to send HTML email to %s: %v", to, err)
		return err
	}
	log.MessageF(log.ModuleEmail, "HTML email sent to %s: %s", to, subject)
	return nil
}

// Email 邮件发送类（保留向后兼容）
type Email struct {
	From     string
	Password string
	SmtpHost string
	SmtpPort string
}

// NewEmail 创建新的邮件发送实例
func NewEmail(from, password, smtpHost, smtpPort string) *Email {
	return &Email{
		From:     from,
		Password: password,
		SmtpHost: smtpHost,
		SmtpPort: smtpPort,
	}
}

// Send 发送电子邮件
func (e *Email) Send(to string, subject string, body string) error {
	return sendMail(e.From, e.Password, e.SmtpHost, e.SmtpPort, to, subject, body)
}
