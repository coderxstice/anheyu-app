// internal/app/service/utility/email_service.go
package utility

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/anzhiyu-c/anheyu-app/internal/app/service/setting"
	"github.com/anzhiyu-c/anheyu-app/internal/constant"
	"github.com/anzhiyu-c/anheyu-app/internal/domain/model"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/parser"
)

// EmailService 定义了发送业务邮件的接口
type EmailService interface {
	SendActivationEmail(ctx context.Context, toEmail, nickname, userID, sign string) error
	SendForgotPasswordEmail(ctx context.Context, toEmail, nickname, userID, sign string) error
	// --- 修改点 1: 移除接口签名中的 targetMeta 参数 ---
	SendCommentNotification(newComment *model.Comment, parentComment *model.Comment)
	SendTestEmail(ctx context.Context, toEmail string) error
}

// emailService 是 EmailService 接口的实现
type emailService struct {
	settingSvc setting.SettingService
}

// NewEmailService 是 emailService 的构造函数
func NewEmailService(settingSvc setting.SettingService) EmailService {
	return &emailService{
		settingSvc: settingSvc,
	}
}

// SendTestEmail 负责发送一封测试邮件
func (s *emailService) SendTestEmail(ctx context.Context, toEmail string) error {
	appName := s.settingSvc.Get(constant.KeyAppName.String())
	siteURL := s.settingSvc.Get(constant.KeySiteURL.String())

	subject := fmt.Sprintf("这是一封来自「%s」的测试邮件", appName)
	body := fmt.Sprintf(`<p>你好！</p>
	<p>这是一封来自 <a href="%s">%s</a> 的测试邮件。</p>
	<p>如果您收到了这封邮件，那么证明您的网站邮件服务配置正确。</p>`, siteURL, appName)

	return s.send(toEmail, subject, body)
}

// SendCommentNotification 实现了发送评论通知的逻辑
func (s *emailService) SendCommentNotification(newComment *model.Comment, parentComment *model.Comment) {
	siteName := s.settingSvc.Get(constant.KeyAppName.String())
	siteURL := s.settingSvc.Get(constant.KeySiteURL.String())
	pageURL := siteURL + newComment.TargetPath
	var targetTitle string
	if newComment.TargetTitle != nil {
		targetTitle = *newComment.TargetTitle
	} else {
		targetTitle = "一个页面"
	}

	gravatarURL := s.settingSvc.Get(constant.KeyGravatarURL.String())
	defaultGravatar := s.settingSvc.Get(constant.KeyDefaultGravatarType.String())

	newCommentHTML, _ := parser.MarkdownToHTML(newComment.Content)
	var newCommenterEmail string
	var newCommentEmailMD5 string
	if newComment.Author.Email != nil {
		newCommenterEmail = *newComment.Author.Email
		newCommentEmailMD5 = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(newCommenterEmail))))
	}

	// --- 场景一：通知博主有新评论 ---
	adminEmail := s.settingSvc.Get(constant.KeyFrontDeskSiteOwnerEmail.String())
	if adminEmail != "" {
		adminSubjectTpl := s.settingSvc.Get(constant.KeyCommentMailSubjectAdmin.String())
		adminBodyTpl := s.settingSvc.Get(constant.KeyCommentMailTemplateAdmin.String())

		data := map[string]interface{}{
			"SITE_NAME":    siteName,
			"SITE_URL":     siteURL,
			"PAGE_URL":     pageURL,
			"TARGET_TITLE": targetTitle,
			"NICK":         newComment.Author.Nickname,
			"COMMENT":      template.HTML(newCommentHTML),
			"MAIL":         newCommenterEmail,
			"IP":           newComment.Author.IP,
			"IMG":          fmt.Sprintf("%s%s?d=%s", gravatarURL, newCommentEmailMD5, defaultGravatar),
		}

		subject, _ := renderTemplate(adminSubjectTpl, data)
		body, _ := renderTemplate(adminBodyTpl, data)
		go func() { _ = s.send(adminEmail, subject, body) }()
	}

	// --- 场景二：通知被回复者 ---
	if parentComment != nil && parentComment.AllowNotification && parentComment.Author.Email != nil && *parentComment.Author.Email != "" {
		if newCommenterEmail != "" && newCommenterEmail == *parentComment.Author.Email {
			return
		}

		parentCommentHTML, _ := parser.MarkdownToHTML(parentComment.Content)
		parentCommentEmailMD5 := fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(*parentComment.Author.Email))))

		replySubjectTpl := s.settingSvc.Get(constant.KeyCommentMailSubject.String())
		replyBodyTpl := s.settingSvc.Get(constant.KeyCommentMailTemplate.String())

		data := map[string]interface{}{
			"SITE_NAME":      siteName,
			"SITE_URL":       siteURL,
			"PAGE_URL":       pageURL,
			"PARENT_NICK":    parentComment.Author.Nickname,
			"PARENT_COMMENT": template.HTML(parentCommentHTML),
			"PARENT_IMG":     fmt.Sprintf("%s%s?d=%s", gravatarURL, parentCommentEmailMD5, defaultGravatar),
			"NICK":           newComment.Author.Nickname,
			"COMMENT":        template.HTML(newCommentHTML),
			"IMG":            fmt.Sprintf("%s%s?d=%s", gravatarURL, newCommentEmailMD5, defaultGravatar),
		}

		subject, _ := renderTemplate(replySubjectTpl, data)
		body, _ := renderTemplate(replyBodyTpl, data)
		go func() { _ = s.send(*parentComment.Author.Email, subject, body) }()
	}
}

// SendActivationEmail 负责发送激活邮件
func (s *emailService) SendActivationEmail(ctx context.Context, toEmail, nickname, userID, sign string) error {
	subjectTplStr := s.settingSvc.Get(constant.KeyActivateAccountSubject.String())
	bodyTplStr := s.settingSvc.Get(constant.KeyActivateAccountTemplate.String())
	appName := s.settingSvc.Get(constant.KeyAppName.String())
	siteURL := s.settingSvc.Get(constant.KeySiteURL.String())

	activateLink := fmt.Sprintf("%s/activate?id=%s&sign=%s", siteURL, userID, sign)
	data := map[string]string{
		"Nickname":     nickname,
		"AppName":      appName,
		"ActivateLink": activateLink,
	}

	subject, err := renderTemplate(subjectTplStr, data)
	if err != nil {
		return fmt.Errorf("渲染激活邮件主题失败: %w", err)
	}
	body, err := renderTemplate(bodyTplStr, data)
	if err != nil {
		return fmt.Errorf("渲染激活邮件正文失败: %w", err)
	}

	go func() { _ = s.send(toEmail, subject, body) }()
	return nil
}

// SendForgotPasswordEmail 负责发送重置密码邮件
func (s *emailService) SendForgotPasswordEmail(ctx context.Context, toEmail, nickname, userID, sign string) error {
	subjectTplStr := s.settingSvc.Get(constant.KeyResetPasswordSubject.String())
	bodyTplStr := s.settingSvc.Get(constant.KeyResetPasswordTemplate.String())
	appName := s.settingSvc.Get(constant.KeyAppName.String())
	siteURL := s.settingSvc.Get(constant.KeySiteURL.String())

	resetLink := fmt.Sprintf("%s/reset-password?id=%s&sign=%s", siteURL, userID, sign)
	data := map[string]string{
		"Nickname":  nickname,
		"AppName":   appName,
		"ResetLink": resetLink,
	}

	subject, err := renderTemplate(subjectTplStr, data)
	if err != nil {
		return fmt.Errorf("渲染重置密码邮件主题失败: %w", err)
	}
	body, err := renderTemplate(bodyTplStr, data)
	if err != nil {
		return fmt.Errorf("渲染重置密码邮件正文失败: %w", err)
	}

	go func() { _ = s.send(toEmail, subject, body) }()
	return nil
}

// send 是一个底层的、私有的邮件发送函数
func (s *emailService) send(to, subject, body string) error {
	host := s.settingSvc.Get(constant.KeySmtpHost.String())
	portStr := s.settingSvc.Get(constant.KeySmtpPort.String())
	username := s.settingSvc.Get(constant.KeySmtpUsername.String())
	password := s.settingSvc.Get(constant.KeySmtpPassword.String())
	senderName := s.settingSvc.Get(constant.KeySmtpSenderName.String())
	senderEmail := s.settingSvc.Get(constant.KeySmtpSenderEmail.String())
	replyToEmail := s.settingSvc.Get(constant.KeySmtpReplyToEmail.String())
	forceSSL := s.settingSvc.GetBool(constant.KeySmtpForceSSL.String())

	port, err := strconv.Atoi(portStr)
	if err != nil {
		msg := fmt.Sprintf("SMTP端口配置无效 '%s'", portStr)
		log.Printf("错误: %s: %v", msg, err)
		return fmt.Errorf("%s: %w", msg, err)
	}

	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", senderName, senderEmail)
	headers["To"] = to
	headers["Subject"] = subject
	headers["Content-Type"] = "text/html; charset=UTF-8"
	if replyToEmail != "" {
		headers["Reply-To"] = replyToEmail
	}

	var messageBuilder strings.Builder
	for k, v := range headers {
		messageBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	messageBuilder.WriteString("\r\n")
	messageBuilder.WriteString(body)
	message := []byte(messageBuilder.String())

	auth := smtp.PlainAuth("", username, password, host)
	addr := fmt.Sprintf("%s:%d", host, port)

	if forceSSL {
		if err := sendMailSSL(addr, auth, senderEmail, []string{to}, message); err != nil {
			log.Printf("错误: [SSL] 发送邮件到 %s 失败: %v", to, err)
			return err
		}
	} else {
		c, err := smtp.Dial(addr)
		if err != nil {
			log.Printf("错误: [STARTTLS] Dialing failed: %v", err)
			return err
		}
		defer c.Close()

		if ok, _ := c.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: true,
			}
			if err = c.StartTLS(tlsConfig); err != nil {
				log.Printf("错误: [STARTTLS] c.StartTLS failed: %v", err)
				return err
			}
		}

		if auth != nil {
			if err = c.Auth(auth); err != nil {
				log.Printf("错误: [STARTTLS] c.Auth failed: %v", err)
				return err
			}
		}

		if err = c.Mail(senderEmail); err != nil {
			return err
		}
		if err = c.Rcpt(to); err != nil {
			return err
		}

		w, err := c.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(message)
		if err != nil {
			return err
		}
		err = w.Close()
		if err != nil {
			return err
		}

		if err := c.Quit(); err != nil {
			log.Printf("警告: [STARTTLS] SMTP c.Quit() 执行失败: %v。这通常不影响邮件发送。", err)
		}

		return nil
	}
	return nil
}

// renderTemplate 是一个渲染 Go 模板的辅助函数
func renderTemplate(tplStr string, data interface{}) (string, error) {
	tpl, err := template.New("email").Parse(tplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// sendMailSSL 是用于处理直接SSL连接的辅助函数
func sendMailSSL(addr string, auth smtp.Auth, from string, to []string, message []byte) error {
	host := strings.Split(addr, ":")[0]
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS拨号失败: %w", err)
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP认证失败: %w", err)
		}
	}
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人 %s 失败: %w", recipient, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %w", err)
	}
	_, err = w.Write(message)
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("关闭写入器失败: %w", err)
	}
	if err := client.Quit(); err != nil {
		log.Printf("警告: SMTP client.Quit() 执行失败: %v。这通常不影响邮件发送。", err)
	}
	return nil
}
