package router

import (
	"crypto/md5"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/internal/pkg/parser"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/strutil"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	article_service "github.com/anzhiyu-c/anheyu-app/pkg/service/article"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

type CustomHTMLRender struct{ Templates *template.Template }

func (r CustomHTMLRender) Instance(name string, data interface{}) render.Render {
	return render.HTML{Template: r.Templates, Name: name, Data: data}
}

// ：生成内容ETag
func generateContentETag(content interface{}) string {
	data, _ := json.Marshal(content)
	hash := md5.Sum(data)
	return fmt.Sprintf(`"ctx7-%x"`, hash)
}

// ：设置智能缓存策略
func setSmartCacheHeaders(c *gin.Context, pageType string, etag string, maxAge int) {
	switch pageType {
	case "article_detail":
		// 文章详情页：短期缓存，依赖ETag
		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", maxAge))
		c.Header("ETag", etag)
		c.Header("Vary", "Accept-Encoding")
		c.Header("X-Content-Type-Options", "nosniff")

	case "home_page":
		// 首页：中等缓存，频繁更新
		c.Header("Cache-Control", "public, max-age=300, must-revalidate") // 5分钟
		c.Header("ETag", etag)
		c.Header("Vary", "Accept-Encoding")

	case "static_page":
		// 静态页面：长期缓存
		c.Header("Cache-Control", "public, max-age=1800, must-revalidate") // 30分钟
		c.Header("ETag", etag)
		c.Header("Vary", "Accept-Encoding")

	default:
		// 默认：谨慎缓存
		c.Header("Cache-Control", "public, max-age=180, must-revalidate") // 3分钟
		c.Header("ETag", etag)
		c.Header("Vary", "Accept-Encoding")
	}
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("X-XSS-Protection", "1; mode=block")
}

// ：处理条件请求
func handleConditionalRequest(c *gin.Context, etag string) bool {
	// 检查 If-None-Match 头
	ifNoneMatch := c.GetHeader("If-None-Match")
	if ifNoneMatch != "" && ifNoneMatch == etag {
		// 内容未修改，返回304
		c.Header("ETag", etag)
		c.Status(http.StatusNotModified)
		return true
	}
	return false
}

// getRequestScheme 确定请求的协议 (http 或 https)
func getRequestScheme(c *gin.Context) string {
	// 优先检查 X-Forwarded-Proto Header，这在反向代理后很常见
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	// 检查请求的 TLS 字段
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

// tryServeStaticFile 尝试从对应的文件系统中提供静态文件
func tryServeStaticFile(c *gin.Context, filePath string, staticMode bool, distFS fs.FS) bool {
	if staticMode {
		// 外部主题模式：从 static 目录查找文件
		overrideDir := "static"
		fullPath := filepath.Join(overrideDir, filePath)
		if _, err := os.Stat(fullPath); err == nil {
			// log.Printf("提供外部静态文件: %s", fullPath)
			c.File(fullPath)
			return true
		} else {
			log.Printf("外部文件未找到: %s, 错误: %v", fullPath, err)
		}
	} else {
		// 内嵌主题模式：从内嵌文件系统查找文件
		if file, err := distFS.Open(filePath); err == nil {
			defer file.Close()
			if stat, err := file.Stat(); err == nil && !stat.IsDir() {
				// log.Printf("提供内嵌静态文件: %s", filePath)
				http.ServeFileFS(c.Writer, c.Request, distFS, filePath)
				return true
			}
		} else {
			log.Printf("内嵌文件未找到: %s, 错误: %v", filePath, err)
		}
	}
	return false
}

// isStaticFileRequest 判断是否是静态文件请求（基于文件扩展名）
func isStaticFileRequest(filePath string) bool {
	staticExtensions := []string{
		".ico", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".tiff",
		".js", ".css", ".map",
		".pdf", ".txt", ".xml", ".json",
		".woff", ".woff2", ".ttf", ".eot", ".otf",
		".mp4", ".mp3", ".wav", ".ogg", ".avi", ".mov",
		".zip", ".rar", ".tar", ".gz", ".br",
	}

	filePath = strings.ToLower(filePath)

	// 检查文件扩展名
	for _, ext := range staticExtensions {
		if strings.HasSuffix(filePath, ext) {
			return true
		}
	}

	return false
}

// shouldReturnIndexHTML 判断是否应该返回 index.html（让前端路由处理）
// 这个函数使用排除法：只有明确不是SPA路由的请求才不返回index.html
func shouldReturnIndexHTML(path string) bool {
	// 明确排除的路径（这些不应该由前端处理）
	excludedPrefixes := []string{
		"/api/",          // API 接口
		"/f/",            // 文件服务
		"/needcache/",    // 缓存服务
		"/static/",       // 静态资源
		"/manifest.json", // PWA manifest
		"/sw.js",         // Service Worker
		"/robots.txt",    // 搜索引擎爬虫文件
		"/sitemap.xml",   // 网站地图
		"/favicon.ico",   // 网站图标
	}

	// 检查是否是被排除的路径
	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(path, prefix) || path == strings.TrimSuffix(prefix, "/") {
			return false
		}
	}

	// 如果路径有文件扩展名，检查是否是静态文件
	if strings.Contains(path, ".") {
		return !isStaticFileRequest(path)
	}

	// 其他所有路径都应该返回 index.html 让前端处理
	// 这包括：/admin/dashboard, /login, /posts/xxx, 以及任何未来新增的前端路由
	return true
}

// isStaticModeActive 检查是否使用静态模式（与主题服务保持一致）
func isStaticModeActive() bool {
	staticDirName := "static"

	// 检查 static 目录是否存在
	if _, err := os.Stat(staticDirName); os.IsNotExist(err) {
		return false
	}

	// 检查 index.html 是否存在
	indexPath := filepath.Join(staticDirName, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return false
	}

	// 额外检查：确保 index.html 不是空文件
	if fileInfo, err := os.Stat(indexPath); err == nil {
		if fileInfo.Size() == 0 {
			log.Printf("警告：发现空的 index.html 文件，视为非静态模式")
			return false
		}
	}

	// 检查是否有其他必要的静态文件（可选）
	// 确保这是一个真正的主题目录，而不是意外创建的空目录
	entries, err := os.ReadDir(staticDirName)
	if err != nil {
		return false
	}

	// 如果目录只有 index.html 且没有其他文件，可能是意外创建的
	if len(entries) == 1 && entries[0].Name() == "index.html" {
		// 检查 index.html 内容是否像一个真正的 HTML 文件
		content, err := os.ReadFile(indexPath)
		if err != nil {
			return false
		}

		contentStr := string(content)
		// 简单检查是否包含基本的 HTML 结构
		if !strings.Contains(strings.ToLower(contentStr), "<html") &&
			!strings.Contains(strings.ToLower(contentStr), "<!doctype") {
			log.Printf("警告：index.html 似乎不是有效的 HTML 文件，视为非静态模式")
			return false
		}
	}

	return true
}

// SetupFrontend 封装了所有与前端静态资源和模板相关的配置（动态模式）
func SetupFrontend(engine *gin.Engine, settingSvc setting.SettingService, articleSvc article_service.Service, embeddedFS embed.FS) {
	log.Println("正在配置动态前端路由系统...")

	engine.GET("/manifest.json", func(c *gin.Context) {
		type ManifestIcon struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
			Type  string `json:"type"`
		}
		type WebAppManifest struct {
			Name            string         `json:"name"`
			ShortName       string         `json:"short_name"`
			Description     string         `json:"description"`
			ThemeColor      string         `json:"theme_color"`
			BackgroundColor string         `json:"background_color"`
			Display         string         `json:"display"`
			StartURL        string         `json:"start_url"`
			Icons           []ManifestIcon `json:"icons"`
		}

		manifest := WebAppManifest{
			Name:            settingSvc.Get(constant.KeyAppName.String()),
			ShortName:       settingSvc.Get(constant.KeyAppName.String()),
			Description:     settingSvc.Get(constant.KeySiteDescription.String()),
			ThemeColor:      settingSvc.Get(constant.KeyThemeColor.String()),
			BackgroundColor: "#ffffff",
			Display:         "standalone",
			StartURL:        "/",
			Icons: []ManifestIcon{
				{Src: settingSvc.Get(constant.KeyLogoURL192.String()), Sizes: "192x192", Type: "image/png"},
				{Src: settingSvc.Get(constant.KeyLogoURL512.String()), Sizes: "512x512", Type: "image/png"},
			},
		}
		if manifest.ThemeColor == "" {
			manifest.ThemeColor = "#ffffff"
		}
		c.JSON(http.StatusOK, manifest)
	})

	// 准备一个通用的模板函数映射
	funcMap := template.FuncMap{
		"json": func(v interface{}) template.JS {
			a, _ := json.Marshal(v)
			return template.JS(a)
		},
	}

	// 预加载嵌入式资源，避免每次请求都处理
	distFS, err := fs.Sub(embeddedFS, "assets/dist")
	if err != nil {
		log.Fatalf("致命错误: 无法从嵌入的资源中创建 'assets/dist' 子文件系统: %v", err)
	}

	embeddedStaticFS, _ := fs.Sub(distFS, "static")
	embeddedTemplates, err := template.New("index.html").Funcs(funcMap).ParseFS(distFS, "index.html")
	if err != nil {
		log.Fatalf("解析嵌入式HTML模板失败: %v", err)
	}

	// 动态静态文件路由 - 每次请求都检查静态模式
	engine.GET("/static/*filepath", func(c *gin.Context) {
		if isStaticModeActive() {
			// 使用外部 static 目录
			log.Printf("动态路由：使用外部主题静态文件 %s", c.Param("filepath"))
			overrideDir := "static"
			staticHandler := http.StripPrefix("/static", http.FileServer(http.Dir(filepath.Join(overrideDir, "static"))))
			c.Header("Cache-Control", "public, max-age=300") // 5分钟缓存
			c.Header("ETag", fmt.Sprintf(`"external-%d"`, time.Now().Unix()/300))
			staticHandler.ServeHTTP(c.Writer, c.Request)
		} else {
			// 使用内嵌资源
			log.Printf("动态路由：使用内嵌静态文件 %s", c.Param("filepath"))
			c.Header("Cache-Control", "public, max-age=3600") // 1小时缓存
			c.Header("ETag", fmt.Sprintf(`"embedded-%d"`, time.Now().Unix()/3600))
			// 需要去掉 /static 前缀，因为 embeddedStaticFS 已经是 static 目录了
			embeddedStaticHandler := http.StripPrefix("/static", http.FileServer(http.FS(embeddedStaticFS)))
			embeddedStaticHandler.ServeHTTP(c.Writer, c.Request)
		}
	})

	// 动态根目录文件路由
	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API路由直接返回404
		if strings.HasPrefix(path, "/api/") {
			response.Fail(c, http.StatusNotFound, "API 路由未找到")
			return
		}

		// 判断是否应该返回 index.html 让前端路由处理
		if shouldReturnIndexHTML(path) {
			log.Printf("SPA路由请求: %s，返回index.html让前端处理", path)

			// 渲染HTML页面
			staticMode := isStaticModeActive()
			var templateInstance *template.Template

			if staticMode {
				log.Printf("动态路由：当前使用外部主题模式，路径: %s", path)
				// 每次都重新解析外部模板，确保获取最新内容
				overrideDir := "static"
				parsedTemplates, err := template.New("index.html").Funcs(funcMap).ParseFiles(filepath.Join(overrideDir, "index.html"))
				if err != nil {
					log.Printf("解析外部HTML模板失败: %v，回退到内嵌模板", err)
					templateInstance = embeddedTemplates
				} else {
					templateInstance = parsedTemplates
				}
			} else {
				log.Printf("动态路由：当前使用内嵌主题模式，路径: %s", path)
				templateInstance = embeddedTemplates
			}

			// 渲染HTML页面
			renderHTMLPage(c, settingSvc, articleSvc, templateInstance)
			return
		}

		// 尝试提供静态文件（处理根目录下的静态文件，如 favicon.ico, robots.txt 等）
		filePath := strings.TrimPrefix(path, "/")
		if filePath != "" && tryServeStaticFile(c, filePath, isStaticModeActive(), distFS) {
			return
		}

		// 如果是静态文件请求但找不到文件，返回404
		if filePath != "" && isStaticFileRequest(filePath) {
			log.Printf("静态文件请求未找到: %s", filePath)
			response.Fail(c, http.StatusNotFound, "文件未找到")
			return
		}

		// 其他未知请求，返回404
		log.Printf("未知请求: %s", path)
		response.Fail(c, http.StatusNotFound, "页面未找到")
	})

	log.Println("动态前端路由系统配置完成")
}

// renderHTMLPage 渲染HTML页面的通用函数（版本）
func renderHTMLPage(c *gin.Context, settingSvc setting.SettingService, articleSvc article_service.Service, templates *template.Template) {
	// 获取完整的当前页面 URL
	fullURL := fmt.Sprintf("%s://%s%s", getRequestScheme(c), c.Request.Host, c.Request.URL.RequestURI())

	isPostDetail, _ := regexp.MatchString(`^/posts/([^/]+)$`, c.Request.URL.Path)
	if isPostDetail {
		slug := strings.TrimPrefix(c.Request.URL.Path, "/posts/")
		articleResponse, err := articleSvc.GetPublicBySlugOrID(c.Request.Context(), slug)
		if err != nil {
			// 文章不存在或已删除，返回404
			log.Printf("文章未找到或已删除: %s, 错误: %v", slug, err)
			response.Fail(c, http.StatusNotFound, "文章未找到")
			return
		}
		if articleResponse != nil {
			// 🎯 ：生成文章内容ETag（基于更新时间和内容）
			contentForETag := struct {
				UpdatedAt   time.Time `json:"updated_at"`
				Title       string    `json:"title"`
				ContentHash string    `json:"content_hash"`
			}{
				UpdatedAt:   articleResponse.UpdatedAt,
				Title:       articleResponse.Title,
				ContentHash: fmt.Sprintf("%x", md5.Sum([]byte(articleResponse.ContentHTML))),
			}
			etag := generateContentETag(contentForETag)

			if handleConditionalRequest(c, etag) {
				return
			}

			// 📊 ：设置文章页面缓存策略（基于更新时间动态调整）
			timeSinceUpdate := time.Since(articleResponse.UpdatedAt)
			var cacheMaxAge int
			if timeSinceUpdate < 24*time.Hour {
				cacheMaxAge = 300 // 新文章：5分钟缓存
			} else if timeSinceUpdate < 7*24*time.Hour {
				cacheMaxAge = 600 // 一周内：10分钟缓存
			} else {
				cacheMaxAge = 1800 // 老文章：30分钟缓存
			}

			setSmartCacheHeaders(c, "article_detail", etag, cacheMaxAge)

			pageTitle := fmt.Sprintf("%s - %s", articleResponse.Title, settingSvc.Get(constant.KeyAppName.String()))

			var pageDescription string
			if len(articleResponse.Summaries) > 0 && articleResponse.Summaries[0] != "" {
				pageDescription = articleResponse.Summaries[0]
			} else {
				plainText := parser.StripHTML(articleResponse.ContentHTML)
				plainText = strings.Join(strings.Fields(plainText), " ")
				pageDescription = strutil.Truncate(plainText, 150)
			}
			if pageDescription == "" {
				pageDescription = settingSvc.Get(constant.KeySiteDescription.String())
			}

			// 构建文章标签列表
			articleTags := make([]string, len(articleResponse.PostTags))
			for i, tag := range articleResponse.PostTags {
				articleTags[i] = tag.Name
			}

			// 使用传入的模板实例渲染
			render := CustomHTMLRender{Templates: templates}
			c.Render(http.StatusOK, render.Instance("index.html", gin.H{
				// --- 基础 SEO 和页面信息 ---
				"pageTitle":       pageTitle,
				"pageDescription": pageDescription,
				"keywords":        settingSvc.Get(constant.KeySiteKeywords.String()),
				"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
				"themeColor":      articleResponse.PrimaryColor,
				"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
				// --- 用于 Vue 水合的数据 ---
				"initialData":   articleResponse,
				"ogType":        "article",
				"ogUrl":         fullURL,
				"ogTitle":       pageTitle,
				"ogDescription": pageDescription,
				"ogImage":       articleResponse.CoverURL,
				"ogSiteName":    settingSvc.Get(constant.KeyAppName.String()),
				"ogLocale":      "zh_CN",
				// --- Article 元标签数据 ---
				"articlePublishedTime": articleResponse.CreatedAt.Format(time.RFC3339),
				"articleModifiedTime":  articleResponse.UpdatedAt.Format(time.RFC3339),
				"articleAuthor":        articleResponse.CopyrightAuthor,
				"articleTags":          articleTags,
				// --- 网站脚本和额外信息 ---
				"siteScript": template.HTML(settingSvc.Get(constant.KeyFooterCode.String())),
			}))
			return
		}
	}

	// --- 默认页面渲染 ---
	defaultTitle := fmt.Sprintf("%s - %s", settingSvc.Get(constant.KeyAppName.String()), settingSvc.Get(constant.KeySubTitle.String()))
	defaultDescription := settingSvc.Get(constant.KeySiteDescription.String())
	defaultImage := settingSvc.Get(constant.KeyLogoURL512.String())

	// 🎯 ：为默认页面生成ETag（基于站点配置）
	siteConfigForETag := struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Path        string `json:"path"`
		Timestamp   int64  `json:"timestamp"`
	}{
		Title:       defaultTitle,
		Description: defaultDescription,
		Path:        c.Request.URL.Path,
		Timestamp:   time.Now().Unix() / 300, // 5分钟粒度
	}
	defaultETag := generateContentETag(siteConfigForETag)

	// 🚀 ：处理条件请求
	if handleConditionalRequest(c, defaultETag) {
		return // 返回304 Not Modified
	}

	// 📊 ：根据页面类型设置缓存策略
	var pageType string
	if c.Request.URL.Path == "/" || c.Request.URL.Path == "/index" {
		pageType = "home_page"
	} else {
		pageType = "static_page"
	}
	setSmartCacheHeaders(c, pageType, defaultETag, 0) // maxAge由pageType决定

	// 使用传入的模板实例渲染
	render := CustomHTMLRender{Templates: templates}
	c.Render(http.StatusOK, render.Instance("index.html", gin.H{
		// --- 基础 SEO 和页面信息 ---
		"pageTitle":       defaultTitle,
		"pageDescription": defaultDescription,
		"keywords":        settingSvc.Get(constant.KeySiteKeywords.String()),
		"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
		"themeColor":      "#f7f9fe",
		"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
		// --- 用于 Vue 水合的数据 ---
		"initialData":   nil,
		"ogType":        "website",
		"ogUrl":         fullURL,
		"ogTitle":       defaultTitle,
		"ogDescription": defaultDescription,
		"ogImage":       defaultImage,
		"ogSiteName":    settingSvc.Get(constant.KeyAppName.String()),
		"ogLocale":      "zh_CN",
		// --- Article 元标签数据 (默认为空) ---
		"articlePublishedTime": nil,
		"articleModifiedTime":  nil,
		"articleAuthor":        nil,
		"articleTags":          nil,
		// --- 网站脚本和额外信息 ---
		"siteScript": template.HTML(settingSvc.Get(constant.KeyFooterCode.String())),
	}))
}
