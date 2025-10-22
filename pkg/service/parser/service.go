// internal/app/service/parser/service.go
package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"github.com/anzhiyu-c/anheyu-app/internal/pkg/event"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// EmojiDef 用于解析JSON中每个表情的定义
type EmojiDef struct {
	Icon string `json:"icon"`
	Text string `json:"text"`
}

// EmojiPack 用于解析整个表情包的JSON结构
type EmojiPack struct {
	Container []EmojiDef `json:"container"`
}

// Service 是一个支持动态加载表情包和HTML安全过滤的解析服务
type Service struct {
	settingSvc      setting.SettingService
	mdParser        goldmark.Markdown
	policy          *bluemonday.Policy
	httpClient      *http.Client
	mu              sync.RWMutex
	emojiReplacer   *strings.Replacer
	currentEmojiURL string
	mermaidRegex    *regexp.Regexp
}

// NewService 创建一个新的解析服务实例
func NewService(settingSvc setting.SettingService, bus *event.EventBus) *Service {
	mdParser := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, extension.Footnote, extension.Typographer,
			extension.Linkify, extension.Strikethrough, extension.Table, extension.TaskList,
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(gmhtml.WithHardWraps(), gmhtml.WithXHTML(), gmhtml.WithUnsafe()),
	)

	policy := bluemonday.UGCPolicy()

	policy.AllowURLSchemes("anzhiyu")

	policy.AllowElements("div", "ul", "i", "table", "thead", "tbody", "tr", "th", "td", "button", "a", "img", "span", "code", "pre", "h1", "h2", "h3", "h4", "h5", "h6", "font", "p", "details", "summary", "svg", "path", "circle", "input", "math", "semantics", "mrow", "mi", "mo", "msup", "mn", "annotation", "style", "g", "marker", "rect", "foreignObject", "li", "ol", "strong", "u", "em", "s", "sup", "sub", "blockquote", "figure", "video")

	policy.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements("ul", "i", "code", "span", "img", "a", "button", "pre", "div", "table", "thead", "tbody", "tr", "th", "td", "h1", "h2", "h3", "h4", "h5", "h6", "font", "p", "details", "summary", "svg", "path", "circle", "input", "g", "rect", "li", "line", "text", "tspan", "blockquote", "video")
	policy.AllowAttrs("style").OnElements(
		"div", "span", "p", "font", "th", "td", "rect", "blockquote", "img", "h1", "h2", "h3", "h4", "h5", "h6", "a", "strong", "b", "em", "i", "u", "s", "strike", "del", "pre", "code", "sub", "sup", "mark", "ul", "ol", "li", "table", "thead", "tbody", "tfoot", "tr", "section", "article", "header", "footer", "nav", "aside", "main", "hr", "figure", "figcaption", "svg", "path", "circle", "line", "g", "text", "summary", "details", "button", "video",
	)
	policy.AllowAttrs("ontoggle").OnElements("details")
	policy.AllowAttrs("onmouseover", "onmouseout").OnElements("summary")
	policy.AllowAttrs("onclick").OnElements("button", "div", "i", "span")
	policy.AllowAttrs("onmouseenter", "onmouseleave").OnElements("span")
	policy.AllowAttrs("color").OnElements("font")
	policy.AllowAttrs("align").OnElements("div")
	policy.AllowAttrs("xmlns").OnElements("annotation", "div")
	policy.AllowAttrs("encoding").OnElements("input")
	policy.AllowAttrs("type").OnElements("input")
	policy.AllowAttrs("checked").OnElements("input")
	policy.AllowAttrs("size").OnElements("font")
	policy.AllowAttrs("target").OnElements("a")
	policy.AllowAttrs("rel").OnElements("a")
	policy.AllowAttrs("rn-wrapper").OnElements("span")
	policy.AllowAttrs("aria-hidden").OnElements("span")
	policy.AllowAttrs("transform").OnElements("g", "rect")
	policy.AllowAttrs("x1", "y1", "x2", "y2").OnElements("line")
	policy.AllowAttrs("rx", "ry").OnElements("rect")
	policy.AllowAttrs("x", "y", "text-anchor").OnElements("text")
	policy.AllowAttrs("x", "dy", "xml:space").OnElements("tspan")

	policy.AllowAttrs("orient", "markerHeight", "markerWidth", "markerUnits", "refY", "refX", "viewBox", "class", "id").OnElements("marker")
	policy.AllowAttrs("language").OnElements("code")
	policy.AllowAttrs("open").OnElements("details")
	policy.AllowAttrs("data-line").OnElements("details", "p", "h2", "h3", "blockquote", "ol", "li", "figure", "table", "div")
	policy.AllowAttrs("data-mermaid-theme", "data-closed", "data-processed").OnElements("p")
	policy.AllowAttrs("data-tips").OnElements("span")
	policy.AllowAttrs("data-href").OnElements("button")
	policy.AllowAttrs("type").OnElements("button")
	policy.AllowAttrs("aria-label").OnElements("button")

	policy.AllowAttrs("data-tip-id").OnElements("span")
	policy.AllowAttrs("data-content", "data-position", "data-theme", "data-trigger", "data-delay").OnElements("div")
	policy.AllowAttrs("role").OnElements("div")
	policy.AllowAttrs("aria-hidden").OnElements("div")

	// 密码保护内容相关属性
	policy.AllowAttrs("data-content-id", "data-title", "data-hint", "data-placeholder").OnElements("div", "input", "button")
	policy.AllowAttrs("placeholder").OnElements("input")
	policy.AllowAttrs("xmlns", "width", "height", "viewBox", "fill", "stroke", "stroke-width", "stroke-linecap", "stroke-linejoin", "preserveAspectRatio", "aria-roledescription", "role", "style", "xmlns:xlink", "id", "t").OnElements("svg")
	policy.AllowAttrs("cx", "cy", "r").OnElements("circle")
	policy.AllowAttrs("d", "style", "class", "marker-end", "fill", "p-id", "t").OnElements("path")
	policy.AllowAttrs("height", "width", "x", "y", "style", "class").OnElements("rect")
	policy.AllowAttrs("height", "width", "x", "y", "style").OnElements("foreignObject")
	policy.AllowAttrs("data-processed").OnElements("span")

	// 视频画廊相关属性
	policy.AllowAttrs("src", "poster", "controls", "preload", "playsinline", "type").OnElements("video")

	policy.AllowAttrs("id").OnElements("div", "h1", "h2", "h3", "h4", "h5", "h6", "button", "a", "img", "span", "code", "pre", "table", "thead", "tbody", "tr", "th", "td", "font", "details", "summary", "svg", "blockquote", "video")

	svc := &Service{
		settingSvc:   settingSvc,
		mdParser:     mdParser,
		policy:       policy,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		mermaidRegex: regexp.MustCompile(`(?s)<p data-line=".*?" class="md-editor-mermaid".*?</p>`),
	}

	bus.Subscribe(event.Topic(setting.TopicSettingUpdated), svc.handleSettingUpdate)
	initialEmojiURL := settingSvc.Get(constant.KeyCommentEmojiCDN.String())
	if initialEmojiURL != "" {
		log.Printf("解析服务初始化，正在加载初始表情包: %s", initialEmojiURL)
		svc.updateEmojiData(context.Background(), initialEmojiURL)
	}

	return svc
}

// handleSettingUpdate 是配置更新事件的处理函数
func (s *Service) handleSettingUpdate(eventData interface{}) {
	evt, ok := eventData.(setting.SettingUpdatedEvent)
	if !ok {
		return
	}

	if evt.Key == constant.KeyCommentEmojiCDN.String() {
		s.mu.RLock()
		currentURL := s.currentEmojiURL
		s.mu.RUnlock()
		if evt.Value != currentURL {
			log.Printf("检测到表情包CDN链接变更。旧: '%s', 新: '%s'。正在更新...", currentURL, evt.Value)
			s.updateEmojiData(context.Background(), evt.Value)
		} else {
			log.Printf("接收到表情包配置更新事件，但URL '%s' 未发生变化，无需重新加载。", evt.Value)
		}
	}
}

// updateEmojiData 负责从指定的URL获取、解析并更新表情包替换器
func (s *Service) updateEmojiData(ctx context.Context, emojiURL string) {
	if emojiURL == "" {
		s.mu.Lock()
		s.emojiReplacer = nil
		s.currentEmojiURL = ""
		s.mu.Unlock()
		log.Println("表情包CDN链接已清空，已卸载表情包解析器。")
		return
	}
	req, err := http.NewRequestWithContext(ctx, "GET", emojiURL, nil)
	if err != nil {
		log.Printf("错误：创建表情包HTTP请求失败: %v", err)
		return
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("错误：从URL '%s' 获取表情包JSON失败: %v", emojiURL, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("错误：从URL '%s' 获取表情包JSON状态码异常: %d", emojiURL, resp.StatusCode)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("错误：读取表情包响应体失败: %v", err)
		return
	}
	var emojiMap map[string]EmojiPack
	if err := json.Unmarshal(body, &emojiMap); err != nil {
		log.Printf("错误：解析表情包JSON数据失败: %v", err)
		return
	}
	var replacements []string
	for _, pack := range emojiMap {
		for _, emoji := range pack.Container {
			key := ":" + emoji.Text + ":"
			modifiedIcon, err := modifyEmojiImgTag(emoji.Icon, "anzhiyu-owo-emotion", emoji.Text)
			if err != nil {
				log.Printf("警告：为表情 '%s' 修改img标签失败，将使用原始图标: %v", emoji.Text, err)
				modifiedIcon = emoji.Icon
			}
			replacements = append(replacements, key, modifiedIcon)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(replacements) > 0 {
		s.emojiReplacer = strings.NewReplacer(replacements...)
		s.currentEmojiURL = emojiURL
		log.Printf("表情包数据已从 '%s' 成功更新并加载！", emojiURL)
	} else {
		s.emojiReplacer = nil
		s.currentEmojiURL = emojiURL
		log.Printf("警告：从 '%s' 加载的表情包数据为空。", emojiURL)
	}
}

// ToHTML 将包含表情包和Markdown的文本转换为安全的HTML。
func (s *Service) ToHTML(ctx context.Context, content string) (string, error) {
	placeholders := make(map[string]string)
	replacedContent := s.mermaidRegex.ReplaceAllStringFunc(content, func(match string) string {
		placeholder := "MERMAID_PLACEHOLDER_" + uuid.New().String()
		placeholders[placeholder] = match
		return placeholder
	})

	s.mu.RLock()
	replacer := s.emojiReplacer
	s.mu.RUnlock()
	if replacer != nil {
		replacedContent = replacer.Replace(replacedContent)
	}

	var buf strings.Builder
	if err := s.mdParser.Convert([]byte(replacedContent), &buf); err != nil {
		return "", err
	}

	safeHTML := s.policy.Sanitize(buf.String())

	finalHTML := safeHTML
	for placeholder, originalMermaid := range placeholders {
		finalHTML = strings.Replace(finalHTML, placeholder, originalMermaid, 1)
	}

	return finalHTML, nil
}

// SanitizeHTML 仅对传入的HTML字符串进行XSS安全过滤。
func (s *Service) SanitizeHTML(htmlContent string) string {
	placeholders := make(map[string]string)
	replacedContent := s.mermaidRegex.ReplaceAllStringFunc(htmlContent, func(match string) string {
		placeholder := "MERMAID_PLACEHOLDER_" + uuid.New().String()
		placeholders[placeholder] = match
		return placeholder
	})

	safeHTML := s.policy.Sanitize(replacedContent)

	finalHTML := safeHTML
	for placeholder, originalMermaid := range placeholders {
		finalHTML = strings.Replace(finalHTML, placeholder, originalMermaid, 1)
	}

	return finalHTML
}

// modifyEmojiImgTag 解析一个HTML片段，为找到的第一个<img>标签添加CSS类并设置alt属性。
func modifyEmojiImgTag(htmlSnippet string, classToAdd string, altText string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlSnippet))
	if err != nil {
		return "", err
	}
	var modified bool
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if modified {
			return
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			classExists := false
			altExists := false
			for i, attr := range n.Attr {
				switch attr.Key {
				case "class":
					classExists = true
					if !strings.Contains(" "+attr.Val+" ", " "+classToAdd+" ") {
						n.Attr[i].Val = strings.TrimSpace(attr.Val + " " + classToAdd)
					}
				case "alt":
					altExists = true
					n.Attr[i].Val = altText
				}
			}
			if !classExists {
				n.Attr = append(n.Attr, html.Attribute{Key: "class", Val: classToAdd})
			}
			if !altExists {
				n.Attr = append(n.Attr, html.Attribute{Key: "alt", Val: altText})
			}
			modified = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)
	var buf bytes.Buffer
	body := doc.FirstChild.LastChild
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}
