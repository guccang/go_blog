package email

import (
	"net/smtp"
)

// SendEmail 发送电子邮件
func SendEmail(to string, subject string, body string) error {
	from := "your_email@example.com" // 替换为您的电子邮件地址
	password := "your_password"       // 替换为您的电子邮件密码

	// 设置 SMTP 服务器
	smtpHost := "smtp.example.com" // 替换为您的 SMTP 服务器
	smtpPort := "587"               // SMTP 端口

	// 创建邮件内容
	message := []byte("Subject: " + subject + "\r\n" +
		"To: " + to + "\r\n" +
		"From: " + from + "\r\n" +
		"\r\n" + body)

	// 认证
	auth := smtp.PlainAuth("", from, password, smtpHost)

	// 发送邮件
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, message)
	if err != nil {
		return err
	}
	return nil
}

)

// Email 邮件发送类
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
    // 创建邮件内容
    message := []byte("Subject: " + subject + "\r\n" +
        "To: " + to + "\r\n" +
        "From: " + e.From + "\r\n" +
        "\r\n" + body)

    // 认证
    auth := smtp.PlainAuth("", e.From, e.Password, e.SmtpHost)

    // 发送邮件
    err := smtp.SendMail(e.SmtpHost+":"+e.SmtpPort, auth, e.From, []string{to}, message)
    if err != nil {
        return err
    }
    return nil
}