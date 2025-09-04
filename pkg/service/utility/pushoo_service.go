// pkg/service/utility/pushoo_service.go
package utility

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

// PushooService 定义了即时消息推送的接口
type PushooService interface {
	SendCommentNotification(ctx context.Context, newComment *model.Comment, parentComment *model.Comment) error
}

// pushooService 是 PushooService 接口的实现
type pushooService struct {
	settingSvc setting.SettingService
	httpClient *http.Client
}

// NewPushooService 是 pushooService 的构造函数
func NewPushooService(settingSvc setting.SettingService) PushooService {
	return &pushooService{
		settingSvc: settingSvc,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // 增加超时时间到30秒
		},
	}
}

// SendCommentNotification 发送评论通知推送
func (s *pushooService) SendCommentNotification(ctx context.Context, newComment *model.Comment, parentComment *model.Comment) error {
	log.Printf("[DEBUG] PushooService.SendCommentNotification 开始执行")

	channel := strings.TrimSpace(s.settingSvc.Get(constant.KeyPushooChannel.String()))
	pushURL := strings.TrimSpace(s.settingSvc.Get(constant.KeyPushooURL.String()))

	log.Printf("[DEBUG] PushooService 配置获取:")
	log.Printf("[DEBUG]   - channel: '%s'", channel)
	log.Printf("[DEBUG]   - pushURL: '%s'", pushURL)

	if channel == "" || pushURL == "" {
		log.Printf("[DEBUG] channel 或 pushURL 为空，静默返回 (channel: '%s', pushURL: '%s')", channel, pushURL)
		return nil // 未配置，静默返回
	}

	log.Printf("[DEBUG] 配置检查通过，开始准备模板数据")

	// 1. 准备模板数据
	data, err := s.prepareTemplateData(newComment, parentComment)
	if err != nil {
		log.Printf("[ERROR] 准备推送模板数据失败: %v", err)
		return fmt.Errorf("准备推送模板数据失败: %w", err)
	}
	log.Printf("[DEBUG] 模板数据准备完成，数据项数量: %d", len(data))

	// 2. 根据不同通道发送推送
	log.Printf("[DEBUG] 开始根据渠道发送推送，渠道: %s", channel)
	switch strings.ToLower(channel) {
	case "bark":
		log.Printf("[DEBUG] 使用 Bark 渠道发送推送")
		return s.sendBarkPush(ctx, pushURL, data)
	case "webhook":
		log.Printf("[DEBUG] 使用 Webhook 渠道发送推送")
		return s.sendWebhookPush(ctx, pushURL, data)
	default:
		log.Printf("[ERROR] 不支持的推送渠道: %s", channel)
		return fmt.Errorf("不支持的推送通道: %s", channel)
	}
}

// prepareTemplateData 准备推送所需的模板数据
func (s *pushooService) prepareTemplateData(newComment *model.Comment, parentComment *model.Comment) (map[string]interface{}, error) {
	siteName := s.settingSvc.Get(constant.KeyAppName.String())
	siteURL := s.settingSvc.Get(constant.KeySiteURL.String())

	// 生成评论的公开ID用作hash
	commentPublicID, err := idgen.GeneratePublicID(newComment.ID, idgen.EntityTypeComment)
	if err != nil {
		log.Printf("[WARN] 生成评论公开ID失败: %v", err)
		commentPublicID = fmt.Sprintf("%d", newComment.ID)
	}

	// 构建带有评论hash的URL，格式为 #comment-{公开ID}
	pageURL := fmt.Sprintf("%s%s#comment-%s", siteURL, newComment.TargetPath, commentPublicID)
	log.Printf("[DEBUG] 生成带hash的评论链接: %s", pageURL)

	var title, body string
	var parentNick, parentContent string

	if parentComment != nil {
		title = fmt.Sprintf("您在「%s」收到了新回复", siteName)
		body = fmt.Sprintf("%s 回复了您的评论：「%s」", newComment.Author.Nickname, newComment.Content)
		parentNick = parentComment.Author.Nickname
		parentContent = parentComment.Content
	} else {
		title = fmt.Sprintf("「%s」收到了新评论", siteName)
		body = fmt.Sprintf("%s 发表了评论：「%s」", newComment.Author.Nickname, newComment.Content)
	}

	// 为Bark URL路径部分进行URL编码
	// 对于Bark，我们需要特殊处理，避免某些字符影响显示
	encodedTitle := strings.ReplaceAll(url.QueryEscape(title), "+", "%20")
	encodedBody := strings.ReplaceAll(url.QueryEscape(body), "+", "%20")

	// 移除换行符，避免显示问题
	encodedBody = strings.ReplaceAll(encodedBody, "%0A", " ")
	encodedBody = strings.ReplaceAll(encodedBody, "%0D", "")

	data := map[string]interface{}{
		"SITE_NAME":      siteName,
		"SITE_URL":       siteURL,
		"POST_URL":       pageURL,
		"TITLE":          encodedTitle,
		"BODY":           encodedBody,
		"NICK":           newComment.Author.Nickname,
		"COMMENT":        newComment.Content,
		"IP":             newComment.Author.IP,
		"MAIL":           *newComment.Author.Email,
		"PARENT_NICK":    parentNick,
		"PARENT_COMMENT": parentContent,
		"TIME":           newComment.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	return data, nil
}

// sendBarkPush 使用模板处理URL后发送Bark推送
func (s *pushooService) sendBarkPush(ctx context.Context, pushURLTpl string, data map[string]interface{}) error {
	log.Printf("[DEBUG] sendBarkPush 开始执行，URL模板: %s", pushURLTpl)

	finalURL, err := renderPushooTemplate(pushURLTpl, data)
	if err != nil {
		log.Printf("[ERROR] 渲染Bark URL模板失败: %v", err)
		return fmt.Errorf("渲染Bark URL模板失败: %w", err)
	}
	log.Printf("[DEBUG] Bark URL模板渲染完成: %s", finalURL)

	// 对于Bark API，我们不需要对整个路径进行编码，因为模板渲染时已经处理了特殊字符
	// 只需要确保URL格式正确
	_, err = url.Parse(finalURL)
	if err != nil {
		log.Printf("[ERROR] 解析Bark URL失败: %v", err)
		return fmt.Errorf("解析Bark URL失败: %w", err)
	}

	// 重新构建正确的URL，不对路径进行额外编码
	finalEncodedURL := finalURL
	log.Printf("[DEBUG] 最终Bark请求URL: %s", finalEncodedURL)

	// 创建一个独立的context，避免继承已经超时的context
	reqCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", finalEncodedURL, nil)
	if err != nil {
		log.Printf("[ERROR] 创建Bark请求失败: %v", err)
		return fmt.Errorf("创建Bark请求失败: %w", err)
	}

	log.Printf("[DEBUG] 开始发送Bark HTTP请求")

	// 添加网络诊断信息
	log.Printf("[DEBUG] 请求目标: %s", req.URL.Host)
	log.Printf("[DEBUG] 请求方法: %s", req.Method)
	log.Printf("[DEBUG] 超时设置: %v", s.httpClient.Timeout)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] 发送Bark推送失败: %v", err)
		log.Printf("[DEBUG] 错误类型: %T", err)
		// 尝试手动测试连接
		log.Printf("[DEBUG] 建议手动测试: curl -I https://api.day.app")
		return fmt.Errorf("发送Bark推送失败: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[DEBUG] Bark推送响应状态码: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Bark推送返回错误状态码: %d", resp.StatusCode)
		return fmt.Errorf("Bark推送返回错误状态码: %d", resp.StatusCode)
	}

	log.Printf("[INFO] Bark推送发送成功: %s", data["TITLE"])
	return nil
}

// sendWebhookPush 发送包含完整信息的Webhook推送
func (s *pushooService) sendWebhookPush(ctx context.Context, webhookURL string, data map[string]interface{}) error {
	log.Printf("[DEBUG] sendWebhookPush 开始执行，URL: %s", webhookURL)

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("[ERROR] 序列化Webhook数据失败: %v", err)
		return fmt.Errorf("序列化Webhook数据失败: %w", err)
	}
	log.Printf("[DEBUG] Webhook数据序列化完成，数据长度: %d bytes", len(jsonData))

	// 创建一个独立的context，避免继承已经超时的context
	reqCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[ERROR] 创建Webhook请求失败: %v", err)
		return fmt.Errorf("创建Webhook请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf("[DEBUG] 开始发送Webhook HTTP请求")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] 发送Webhook推送失败: %v", err)
		return fmt.Errorf("发送Webhook推送失败: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[DEBUG] Webhook推送响应状态码: %d", resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[ERROR] Webhook推送返回错误状态码: %d", resp.StatusCode)
		return fmt.Errorf("Webhook推送返回错误状态码: %d", resp.StatusCode)
	}

	log.Printf("[INFO] Webhook推送发送成功: %s", data["TITLE"])
	return nil
}

// renderPushooTemplate 渲染推送模板（用于URL或内容）
func renderPushooTemplate(tplStr string, data interface{}) (string, error) {
	tpl, err := template.New("pushoo").Parse(tplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
