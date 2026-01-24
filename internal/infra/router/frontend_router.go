package router

import (
	"bytes"
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
	"github.com/anzhiyu-c/anheyu-app/pkg/config"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/handler/rss"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	article_service "github.com/anzhiyu-c/anheyu-app/pkg/service/article"
	rss_service "github.com/anzhiyu-c/anheyu-app/pkg/service/rss"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/utility"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

type CustomHTMLRender struct{ Templates *template.Template }

func (r CustomHTMLRender) Instance(name string, data interface{}) render.Render {
	return render.HTML{Template: r.Templates, Name: name, Data: data}
}

// 全局 Debug 标志
var isDebugMode bool

// debugLog 根据 Debug 配置条件性地打印日志
func debugLog(format string, v ...interface{}) {
	if isDebugMode {
		log.Printf(format, v...)
	}
}

// ：生成内容ETag
func generateContentETag(content interface{}) string {
	data, _ := json.Marshal(content)
	hash := md5.Sum(data)
	return fmt.Sprintf(`"ctx7-%x"`, hash)
}

// ：设置智能缓存策略（针对CDN优化）
func setSmartCacheHeaders(c *gin.Context, pageType string, etag string, maxAge int) {
	// 检测是否通过CDN访问
	isCDN := c.GetHeader("CF-Ray") != "" || // Cloudflare
		c.GetHeader("X-Amz-Cf-Id") != "" || // CloudFront
		c.GetHeader("X-Cache") != "" || // 通用CDN标识
		c.GetHeader("X-Served-By") != "" // Fastly等

	switch pageType {
	case "article_detail":
		if isCDN {
			// CDN环境：更短的缓存时间，强制验证
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, s-maxage=%d, must-revalidate, stale-while-revalidate=60",
				min(maxAge, 180), min(maxAge/2, 60))) // CDN缓存时间更短
		} else {
			// 直连环境：正常缓存策略
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, must-revalidate", maxAge))
		}
		c.Header("ETag", etag)
		c.Header("Vary", "Accept-Encoding")
		c.Header("X-Content-Type-Options", "nosniff")
		// 添加缓存标签，便于CDN批量清除
		c.Header("Cache-Tag", fmt.Sprintf("article-detail,article-%s", extractArticleIDFromPath(c.Request.URL.Path)))

	case "home_page":
		if isCDN {
			// 首页：CDN缓存2分钟，浏览器缓存5分钟
			c.Header("Cache-Control", "public, max-age=300, s-maxage=120, must-revalidate, stale-while-revalidate=30")
		} else {
			c.Header("Cache-Control", "public, max-age=300, must-revalidate") // 5分钟
		}
		c.Header("ETag", etag)
		c.Header("Vary", "Accept-Encoding")
		c.Header("Cache-Tag", "home-page,article-list")

	case "static_page":
		if isCDN {
			// 静态页面：CDN缓存10分钟，浏览器缓存30分钟
			c.Header("Cache-Control", "public, max-age=1800, s-maxage=600, must-revalidate, stale-while-revalidate=120")
		} else {
			c.Header("Cache-Control", "public, max-age=1800, must-revalidate") // 30分钟
		}
		c.Header("ETag", etag)
		c.Header("Vary", "Accept-Encoding")
		c.Header("Cache-Tag", "static-page")

	default:
		if isCDN {
			// 默认：CDN缓存1分钟，浏览器缓存3分钟
			c.Header("Cache-Control", "public, max-age=180, s-maxage=60, must-revalidate, stale-while-revalidate=30")
		} else {
			c.Header("Cache-Control", "public, max-age=180, must-revalidate") // 3分钟
		}
		c.Header("ETag", etag)
		c.Header("Vary", "Accept-Encoding")
		c.Header("Cache-Tag", "default")
	}

	// 安全头部
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("X-XSS-Protection", "1; mode=block")

	// 添加版本标识，便于缓存失效
	c.Header("X-App-Version", getAppVersion())
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractArticleIDFromPath 从URL路径中提取文章ID
func extractArticleIDFromPath(path string) string {
	// 匹配 /posts/{id} 格式
	re := regexp.MustCompile(`^/posts/([^/]+)$`)
	matches := re.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}
	return "unknown"
}

// getAppVersion 获取应用版本号（用于缓存失效）
func getAppVersion() string {
	// 可以从环境变量、构建时间或版本文件中获取
	// 这里使用简单的时间戳作为版本标识
	return fmt.Sprintf("v%d", time.Now().Unix()/3600) // 每小时变化一次
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

// generateFileETag 为文件生成基于内容的ETag
func generateFileETag(filePath string, modTime time.Time, size int64) string {
	// 使用文件路径、修改时间和大小生成ETag，避免读取大文件内容
	data := fmt.Sprintf("%s-%d-%d", filePath, modTime.Unix(), size)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf(`"static-%x"`, hash)
}

// getAcceptedEncoding 获取客户端支持的编码格式，按优先级排序
func getAcceptedEncoding(c *gin.Context) string {
	acceptEncoding := c.GetHeader("Accept-Encoding")
	if acceptEncoding == "" {
		return ""
	}

	// 优先级：brotli > gzip > identity
	if strings.Contains(acceptEncoding, "br") {
		return "br"
	}
	if strings.Contains(acceptEncoding, "gzip") {
		return "gzip"
	}
	return ""
}

// tryServeCompressedFile 尝试提供压缩文件
func tryServeCompressedFile(c *gin.Context, basePath string, staticMode bool, distFS fs.FS) (bool, string, time.Time, int64) {
	encoding := getAcceptedEncoding(c)
	if encoding == "" {
		return false, "", time.Time{}, 0
	}

	var compressedPath string
	var contentEncoding string

	switch encoding {
	case "br":
		compressedPath = basePath + ".br"
		contentEncoding = "br"
	case "gzip":
		compressedPath = basePath + ".gz"
		contentEncoding = "gzip"
	default:
		return false, "", time.Time{}, 0
	}

	if staticMode {
		// 外部主题模式
		overrideDir := "static"
		fullPath := filepath.Join(overrideDir, compressedPath)
		if fileInfo, err := os.Stat(fullPath); err == nil {
			c.Header("Content-Encoding", contentEncoding)
			c.Header("Content-Type", getContentType(basePath))
			return true, fullPath, fileInfo.ModTime(), fileInfo.Size()
		}
	} else {
		// 内嵌主题模式
		if file, err := distFS.Open(compressedPath); err == nil {
			defer file.Close()
			if stat, err := file.Stat(); err == nil && !stat.IsDir() {
				c.Header("Content-Encoding", contentEncoding)
				c.Header("Content-Type", getContentType(basePath))
				return true, compressedPath, stat.ModTime(), stat.Size()
			}
		}
	}

	return false, "", time.Time{}, 0
}

// getContentType 根据文件扩展名获取MIME类型
func getContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}

// handleStaticFileConditionalRequest 处理静态文件的条件请求
func handleStaticFileConditionalRequest(c *gin.Context, etag string, filePath string) bool {
	// 检查 If-None-Match 头
	ifNoneMatch := c.GetHeader("If-None-Match")
	if ifNoneMatch != "" && ifNoneMatch == etag {
		// 内容未修改，返回304
		c.Header("ETag", etag)
		// 根据文件类型设置缓存策略
		if isHTMLFile(filePath) {
			// HTML文件不缓存
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		} else {
			// 其他静态文件使用协商缓存（1年，但每次验证）
			c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
		}
		c.Status(http.StatusNotModified)
		return true
	}
	return false
}

// isHTMLFile 判断是否是HTML文件
func isHTMLFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".html" || ext == ".htm"
}

// tryServeStaticFile 尝试从对应的文件系统中提供静态文件（优先压缩版本）
func tryServeStaticFile(c *gin.Context, filePath string, staticMode bool, distFS fs.FS) bool {
	// 首先尝试提供压缩文件
	if compressed, compressedPath, modTime, size := tryServeCompressedFile(c, filePath, staticMode, distFS); compressed {
		// 生成基于压缩文件的ETag
		etag := generateFileETag(compressedPath, modTime, size)

		// 处理条件请求
		if handleStaticFileConditionalRequest(c, etag, filePath) {
			return true
		}

		// 设置缓存头 - 根据文件类型设置不同策略
		c.Header("ETag", etag)
		if isHTMLFile(filePath) {
			// HTML文件不缓存
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		} else {
			// 其他静态文件使用协商缓存（1年，但每次验证）
			c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
		}
		c.Header("Vary", "Accept-Encoding")

		if staticMode {
			// log.Printf("提供外部压缩静态文件: %s", compressedPath)
			c.File(compressedPath)
		} else {
			// log.Printf("提供内嵌压缩静态文件: %s", compressedPath)
			http.ServeFileFS(c.Writer, c.Request, distFS, compressedPath)
		}
		return true
	}

	// 如果没有压缩版本，提供原文件
	if staticMode {
		// 外部主题模式：从 static 目录查找文件
		overrideDir := "static"
		fullPath := filepath.Join(overrideDir, filePath)
		if fileInfo, err := os.Stat(fullPath); err == nil {
			// 生成基于文件内容的ETag
			etag := generateFileETag(filePath, fileInfo.ModTime(), fileInfo.Size())

			// 处理条件请求
			if handleStaticFileConditionalRequest(c, etag, filePath) {
				return true
			}

			// 设置缓存头 - 根据文件类型设置不同策略
			c.Header("ETag", etag)
			if isHTMLFile(filePath) {
				// HTML文件不缓存
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Header("Pragma", "no-cache")
				c.Header("Expires", "0")
			} else {
				// 其他静态文件使用协商缓存（1年，但每次验证）
				c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
			}
			c.Header("Vary", "Accept-Encoding")
			c.Header("Content-Type", getContentType(filePath))

			// debugLog("提供外部原始静态文件: %s", fullPath)
			c.File(fullPath)
			return true
		} else {
			debugLog("外部文件未找到: %s, 错误: %v", fullPath, err)
		}
	} else {
		// 内嵌主题模式：从内嵌文件系统查找文件
		if file, err := distFS.Open(filePath); err == nil {
			defer file.Close()
			if stat, err := file.Stat(); err == nil && !stat.IsDir() {
				// 生成基于文件内容的ETag
				etag := generateFileETag(filePath, stat.ModTime(), stat.Size())

				// 处理条件请求
				if handleStaticFileConditionalRequest(c, etag, filePath) {
					return true
				}

				// 设置缓存头 - 根据文件类型设置不同策略
				c.Header("ETag", etag)
				if isHTMLFile(filePath) {
					// HTML文件不缓存
					c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
					c.Header("Pragma", "no-cache")
					c.Header("Expires", "0")
				} else {
					// 其他静态文件使用协商缓存（1年，但每次验证）
					c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
				}
				c.Header("Vary", "Accept-Encoding")
				c.Header("Content-Type", getContentType(filePath))

				// debugLog("提供内嵌原始静态文件: %s", filePath)
				http.ServeFileFS(c.Writer, c.Request, distFS, filePath)
				return true
			}
		} else {
			debugLog("内嵌文件未找到: %s, 错误: %v", filePath, err)
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
		"/api/",        // API 接口
		"/f/",          // 文件服务
		"/needcache/",  // 缓存服务
		"/static/",     // 静态资源
		"/robots.txt",  // 搜索引擎爬虫文件
		"/sitemap.xml", // 网站地图
		"/favicon.ico", // 网站图标
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

// isAdminPath 判断是否是后台管理路径
// 后台路径始终使用官方内嵌资源，不受外部主题影响
func isAdminPath(path string) bool {
	adminPrefixes := []string{
		"/admin", // 后台管理页面
		"/login", // 登录页面（后台入口）
	}

	for _, prefix := range adminPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// shouldUseExternalTheme 判断当前路径是否应该使用外部主题
// 只有前台页面且 static 目录存在时才使用外部主题
func shouldUseExternalTheme(path string) bool {
	// 后台路径始终使用官方内嵌资源
	if isAdminPath(path) {
		return false
	}
	// 前台路径：检查是否有外部主题
	return isStaticModeActive()
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
			debugLog("警告：发现空的 index.html 文件，视为非静态模式")
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
			debugLog("警告：index.html 似乎不是有效的 HTML 文件，视为非静态模式")
			return false
		}
	}

	return true
}

// SetupFrontend 封装了所有与前端静态资源和模板相关的配置（动态模式）
func SetupFrontend(engine *gin.Engine, settingSvc setting.SettingService, articleSvc article_service.Service, cacheSvc utility.CacheService, embeddedFS embed.FS, cfg *config.Config) {
	// 从配置中读取 Debug 模式
	isDebugMode = cfg.GetBool(config.KeyServerDebug)

	// 启动时打印主题模式信息
	log.Println("========================================")
	log.Println("🎨 前后台分离主题系统已启用")
	log.Println("   后台管理 (/admin/*, /login): 始终使用官方内嵌资源")
	if isStaticModeActive() {
		log.Println("   前台展示 (其他路径): 外部主题模式 (static 目录)")
		log.Println("   说明: 检测到 static/index.html，前台将从 static 目录加载")
	} else {
		log.Println("   前台展示 (其他路径): 官方主题模式 (embed)")
		log.Println("   说明: 未检测到 static/index.html，前台将使用内嵌资源")
	}
	log.Println("========================================")

	debugLog("正在配置动态前端路由系统...")

	// 配置 RSS feed
	rssSvc := rss_service.NewService(articleSvc, settingSvc, cacheSvc)
	rssHandler := rss.NewHandler(rssSvc, settingSvc)
	engine.GET("/rss.xml", rssHandler.GetRSSFeed)
	engine.GET("/feed.xml", rssHandler.GetRSSFeed)
	engine.GET("/atom.xml", rssHandler.GetRSSFeed)
	debugLog("RSS feed 路由已配置: /rss.xml, /feed.xml 和 /atom.xml")

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

	embeddedTemplates, err := template.New("index.html").Funcs(funcMap).ParseFS(distFS, "index.html")
	if err != nil {
		log.Fatalf("解析嵌入式HTML模板失败: %v", err)
	}

	// 后台专用静态文件路由 - 始终从 embed 读取，不受外部主题影响
	// 这是前后台分离的关键：后台的 JS/CSS 使用 /admin-static/ 路径
	engine.GET("/admin-static/*filepath", func(c *gin.Context) {
		filePath := strings.TrimPrefix(c.Param("filepath"), "/")
		debugLog("后台静态资源请求: %s (始终使用内嵌资源)", filePath)

		// 首先尝试提供压缩文件
		if compressed, compressedPath, modTime, size := tryServeCompressedFile(c, "static/"+filePath, false, distFS); compressed {
			etag := generateFileETag(compressedPath, modTime, size)
			if handleStaticFileConditionalRequest(c, etag, "static/"+filePath) {
				return
			}
			c.Header("ETag", etag)
			if isHTMLFile(filePath) {
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Header("Pragma", "no-cache")
				c.Header("Expires", "0")
			} else {
				c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
			}
			c.Header("Vary", "Accept-Encoding")
			http.ServeFileFS(c.Writer, c.Request, distFS, compressedPath)
			return
		}

		// 提供原始文件
		staticFilePath := "static/" + filePath
		if file, err := distFS.Open(staticFilePath); err == nil {
			defer file.Close()
			if stat, err := file.Stat(); err == nil && !stat.IsDir() {
				etag := generateFileETag(filePath, stat.ModTime(), stat.Size())
				if handleStaticFileConditionalRequest(c, etag, filePath) {
					return
				}
				c.Header("ETag", etag)
				if isHTMLFile(filePath) {
					c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
					c.Header("Pragma", "no-cache")
					c.Header("Expires", "0")
				} else {
					c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
				}
				c.Header("Vary", "Accept-Encoding")
				c.Header("Content-Type", getContentType(filePath))
				http.ServeFileFS(c.Writer, c.Request, distFS, staticFilePath)
				return
			}
		}
		c.Status(http.StatusNotFound)
	})

	// 动态静态文件路由 - 前台静态资源，根据外部主题是否存在决定来源
	engine.GET("/static/*filepath", func(c *gin.Context) {
		filePath := strings.TrimPrefix(c.Param("filepath"), "/")
		staticMode := isStaticModeActive()

		// 首先尝试提供压缩文件
		if compressed, compressedPath, modTime, size := tryServeCompressedFile(c, "static/"+filePath, staticMode, distFS); compressed {
			// 生成基于压缩文件的ETag
			etag := generateFileETag(compressedPath, modTime, size)

			// 处理条件请求
			if handleStaticFileConditionalRequest(c, etag, "static/"+filePath) {
				return
			}

			// 设置缓存头 - 根据文件类型设置不同策略
			c.Header("ETag", etag)
			if isHTMLFile(filePath) {
				// HTML文件不缓存
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Header("Pragma", "no-cache")
				c.Header("Expires", "0")
			} else {
				// 其他静态文件使用协商缓存（1年，但每次验证）
				c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
			}
			c.Header("Vary", "Accept-Encoding")

			if staticMode {
				debugLog("动态路由：使用外部主题压缩文件 %s", compressedPath)
				c.File(compressedPath)
			} else {
				debugLog("动态路由：使用内嵌压缩文件 %s", compressedPath)
				http.ServeFileFS(c.Writer, c.Request, distFS, compressedPath)
			}
			return
		}

		// 如果没有压缩版本，提供原文件
		if staticMode {
			// 使用外部 static 目录
			overrideDir := "static"
			fullPath := filepath.Join(overrideDir, "static", filePath)

			if fileInfo, err := os.Stat(fullPath); err == nil {
				// 生成基于文件内容的ETag
				etag := generateFileETag(filePath, fileInfo.ModTime(), fileInfo.Size())

				// 处理条件请求
				if handleStaticFileConditionalRequest(c, etag, filePath) {
					return
				}

				// 设置缓存头 - 根据文件类型设置不同策略
				c.Header("ETag", etag)
				if isHTMLFile(filePath) {
					// HTML文件不缓存
					c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
					c.Header("Pragma", "no-cache")
					c.Header("Expires", "0")
				} else {
					// 其他静态文件使用协商缓存（1年，但每次验证）
					c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
				}
				c.Header("Vary", "Accept-Encoding")
				c.Header("Content-Type", getContentType(filePath))

				debugLog("动态路由：使用外部主题原始文件 %s", c.Param("filepath"))
				staticHandler := http.StripPrefix("/static", http.FileServer(http.Dir(filepath.Join(overrideDir, "static"))))
				staticHandler.ServeHTTP(c.Writer, c.Request)
			} else {
				c.Status(http.StatusNotFound)
			}
		} else {
			// 使用内嵌资源
			staticFilePath := "static/" + filePath
			if file, err := distFS.Open(staticFilePath); err == nil {
				defer file.Close()
				if stat, err := file.Stat(); err == nil && !stat.IsDir() {
					// 生成基于文件内容的ETag
					etag := generateFileETag(filePath, stat.ModTime(), stat.Size())

					// 处理条件请求
					if handleStaticFileConditionalRequest(c, etag, filePath) {
						return
					}

					// 设置缓存头 - 根据文件类型设置不同策略
					c.Header("ETag", etag)
					if isHTMLFile(filePath) {
						// HTML文件不缓存
						c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
						c.Header("Pragma", "no-cache")
						c.Header("Expires", "0")
					} else {
						// 其他静态文件使用协商缓存（1年，但每次验证）
						c.Header("Cache-Control", "public, max-age=31536000, must-revalidate")
					}
					c.Header("Vary", "Accept-Encoding")
					c.Header("Content-Type", getContentType(filePath))

					debugLog("动态路由：使用内嵌原始文件 %s", c.Param("filepath"))
					http.ServeFileFS(c.Writer, c.Request, distFS, staticFilePath)
				} else {
					c.Status(http.StatusNotFound)
				}
			} else {
				c.Status(http.StatusNotFound)
			}
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
			debugLog("SPA路由请求: %s，返回index.html让前端处理", path)

			// 核心改进：根据路径决定使用哪个模板
			// - 后台路径（/admin/*, /login）：始终使用官方内嵌模板，且静态资源路径重写
			// - 前台路径：根据 static 目录是否存在决定
			isAdmin := isAdminPath(path)
			useExternalTheme := shouldUseExternalTheme(path)
			var templateInstance *template.Template

			if useExternalTheme {
				debugLog("动态路由：前台页面使用外部主题模式，路径: %s", path)
				// 每次都重新解析外部模板，确保获取最新内容
				overrideDir := "static"
				parsedTemplates, err := template.New("index.html").Funcs(funcMap).ParseFiles(filepath.Join(overrideDir, "index.html"))
				if err != nil {
					debugLog("解析外部HTML模板失败: %v，回退到内嵌模板", err)
					templateInstance = embeddedTemplates
				} else {
					templateInstance = parsedTemplates
				}
			} else {
				if isAdmin {
					debugLog("动态路由：后台页面始终使用内嵌模板，路径: %s", path)
				} else {
					debugLog("动态路由：前台页面使用内嵌主题模式，路径: %s", path)
				}
				templateInstance = embeddedTemplates
			}

			// 渲染HTML页面
			// 如果是后台页面且存在外部主题，需要重写静态资源路径
			if isAdmin && isStaticModeActive() {
				renderHTMLPageWithAdminRewrite(c, settingSvc, articleSvc, templateInstance)
			} else {
				renderHTMLPage(c, settingSvc, articleSvc, templateInstance)
			}
			return
		}

		// 尝试提供静态文件（处理根目录下的静态文件，如 favicon.ico, robots.txt 等）
		filePath := strings.TrimPrefix(path, "/")
		// 静态文件也需要区分前后台：后台的静态文件始终从 embed 读取
		useExternalForStatic := !isAdminPath(path) && isStaticModeActive()
		if filePath != "" && tryServeStaticFile(c, filePath, useExternalForStatic, distFS) {
			return
		}

		// 如果是静态文件请求但找不到文件，返回404
		if filePath != "" && isStaticFileRequest(filePath) {
			debugLog("静态文件请求未找到: %s", filePath)
			response.Fail(c, http.StatusNotFound, "文件未找到")
			return
		}

		// 其他未知请求，返回404
		debugLog("未知请求: %s", path)
		response.Fail(c, http.StatusNotFound, "页面未找到")
	})

	debugLog("动态前端路由系统配置完成")
}

// ensureScriptTagsClosed 确保HTML中的script标签正确闭合
// 这个函数会检测未闭合的script标签并自动添加闭合标签
func ensureScriptTagsClosed(html string) string {
	if html == "" {
		return html
	}

	// 使用正则表达式匹配所有 script 开始标签和结束标签
	openTagRegex := regexp.MustCompile(`(?i)<script[^>]*>`)
	closeTagRegex := regexp.MustCompile(`(?i)</script>`)

	openTags := openTagRegex.FindAllString(html, -1)
	closeTags := closeTagRegex.FindAllString(html, -1)

	// 如果有开始标签但闭合标签数量不足，补全缺失的闭合标签
	if len(openTags) > len(closeTags) {
		missingCloseTags := len(openTags) - len(closeTags)
		for i := 0; i < missingCloseTags; i++ {
			html += "</script>"
		}
		debugLog("⚠️ 检测到 %d 个未闭合的 script 标签，已自动补全", missingCloseTags)
	}

	return html
}

// MenuItem 定义导航菜单项结构
type MenuItem struct {
	Title      string     `json:"title"`
	Path       string     `json:"path"`
	Icon       string     `json:"icon"`
	IsExternal bool       `json:"isExternal"`
	Items      []MenuItem `json:"items"`
}

// generateBreadcrumbList 根据当前路径生成面包屑导航的结构化数据
// 返回符合 Schema.org BreadcrumbList 规范的 JSON 数据
func generateBreadcrumbList(path string, baseURL string, settingSvc setting.SettingService) []map[string]interface{} {
	siteName := settingSvc.Get(constant.KeyAppName.String())

	breadcrumbs := []map[string]interface{}{
		{
			"@type":    "ListItem",
			"position": 1,
			"name":     siteName,
			"item":     baseURL,
		},
	}

	// 如果是首页，只返回首页项
	if path == "/" || path == "" {
		return breadcrumbs
	}

	// 从配置中读取导航菜单
	menuJSON := settingSvc.Get(constant.KeyHeaderMenu.String())
	var menuGroups []MenuItem
	if err := json.Unmarshal([]byte(menuJSON), &menuGroups); err != nil {
		debugLog("解析导航菜单配置失败: %v", err)
		// 解析失败时返回基础面包屑
		return breadcrumbs
	}

	// 构建路径到菜单项的映射
	navItems := make(map[string]string)
	for _, group := range menuGroups {
		for _, item := range group.Items {
			if item.Path != "" && !item.IsExternal {
				navItems[item.Path] = item.Title
			}
		}
	}

	// 处理文章详情页 /posts/{slug}
	if strings.HasPrefix(path, "/posts/") {
		// 添加"全部文章"面包屑（如果在菜单中存在）
		archivesTitle := "全部文章"
		if title, exists := navItems["/archives"]; exists {
			archivesTitle = title
		}
		breadcrumbs = append(breadcrumbs, map[string]interface{}{
			"@type":    "ListItem",
			"position": 2,
			"name":     archivesTitle,
			"item":     baseURL + "/archives",
		})
		// 当前文章页（不需要 item 属性）
		slug := strings.TrimPrefix(path, "/posts/")
		breadcrumbs = append(breadcrumbs, map[string]interface{}{
			"@type":    "ListItem",
			"position": 3,
			"name":     slug, // 实际渲染时会被文章标题替换
		})
		return breadcrumbs
	}

	// 处理导航菜单中的页面
	if title, exists := navItems[path]; exists {
		breadcrumbs = append(breadcrumbs, map[string]interface{}{
			"@type":    "ListItem",
			"position": 2,
			"name":     title,
		})
		return breadcrumbs
	}

	// 处理分类详情页 /categories/{slug}
	if strings.HasPrefix(path, "/categories/") {
		categoriesTitle := "分类列表"
		if title, exists := navItems["/categories"]; exists {
			categoriesTitle = title
		}
		breadcrumbs = append(breadcrumbs, map[string]interface{}{
			"@type":    "ListItem",
			"position": 2,
			"name":     categoriesTitle,
			"item":     baseURL + "/categories",
		})
		categorySlug := strings.TrimPrefix(path, "/categories/")
		breadcrumbs = append(breadcrumbs, map[string]interface{}{
			"@type":    "ListItem",
			"position": 3,
			"name":     categorySlug,
		})
		return breadcrumbs
	}

	// 处理标签详情页 /tags/{slug}
	if strings.HasPrefix(path, "/tags/") {
		tagsTitle := "标签列表"
		if title, exists := navItems["/tags"]; exists {
			tagsTitle = title
		}
		breadcrumbs = append(breadcrumbs, map[string]interface{}{
			"@type":    "ListItem",
			"position": 2,
			"name":     tagsTitle,
			"item":     baseURL + "/tags",
		})
		tagSlug := strings.TrimPrefix(path, "/tags/")
		breadcrumbs = append(breadcrumbs, map[string]interface{}{
			"@type":    "ListItem",
			"position": 3,
			"name":     tagSlug,
		})
		return breadcrumbs
	}

	// 默认情况，只返回首页
	return breadcrumbs
}

// convertImagesToLazyLoad 将HTML中的图片转换为懒加载格式
// 在服务端渲染时直接生成懒加载HTML，避免浏览器在解析时就开始加载图片
func convertImagesToLazyLoad(html string) string {
	if html == "" {
		return html
	}

	// 占位符图片 - 1x1 透明像素的 base64 编码
	const placeholderImage = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMSIgaGVpZ2h0PSIxIiB2aWV3Qm94PSIwIDAgMSAxIiBmaWxsPSJub25lIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPgo8cmVjdCB3aWR0aD0iMSIgaGVpZ2h0PSIxIiBmaWxsPSJ0cmFuc3BhcmVudCIvPgo8L3N2Zz4="

	// 匹配 <img> 标签，包括自闭合和非自闭合格式
	// 排除已经有 data-src 的图片（避免重复处理）
	imgRegex := regexp.MustCompile(`<img\s+([^>]*?)\s*\/?>`)

	result := imgRegex.ReplaceAllStringFunc(html, func(match string) string {
		// 如果已经包含 data-src 或 data-lazy-processed，跳过处理
		if strings.Contains(match, "data-src") || strings.Contains(match, "data-lazy-processed") {
			return match
		}

		// 如果已经是占位符图片，跳过处理
		if strings.Contains(match, placeholderImage) {
			return match
		}

		// 提取 src 属性
		srcRegex := regexp.MustCompile(`src=["']([^"']+)["']`)
		srcMatch := srcRegex.FindStringSubmatch(match)

		if len(srcMatch) < 2 {
			// 没有 src 属性，保持原样
			return match
		}

		originalSrc := srcMatch[1]

		// 跳过 data: URL（这些通常是占位符或内联图片）
		if strings.HasPrefix(originalSrc, "data:") {
			return match
		}

		// 构建新的 img 标签
		// 1. 将原始 src 替换为占位符
		newMatch := srcRegex.ReplaceAllString(match, fmt.Sprintf(`src="%s"`, placeholderImage))

		// 2. 添加 data-src 属性（在 src 之后插入）
		newMatch = strings.Replace(newMatch, fmt.Sprintf(`src="%s"`, placeholderImage),
			fmt.Sprintf(`src="%s" data-src="%s"`, placeholderImage, originalSrc), 1)

		// 3. 添加懒加载相关的 class
		classRegex := regexp.MustCompile(`class=["']([^"']+)["']`)
		if classMatch := classRegex.FindStringSubmatch(newMatch); len(classMatch) >= 2 {
			// 已有 class，追加新的类名
			existingClasses := classMatch[1]
			if !strings.Contains(existingClasses, "lazy-image") {
				newClasses := existingClasses + " lazy-image"
				newMatch = classRegex.ReplaceAllString(newMatch, fmt.Sprintf(`class="%s"`, newClasses))
			}
		} else {
			// 没有 class，添加新的 class 属性
			newMatch = strings.Replace(newMatch, "<img", `<img class="lazy-image"`, 1)
		}

		// 4. 添加 data-lazy-processed 标记
		newMatch = strings.Replace(newMatch, "<img", `<img data-lazy-processed="true"`, 1)

		return newMatch
	})

	return result
}

// SocialLink 定义社交链接结构
type SocialLink struct {
	Title string `json:"title"`
	Link  string `json:"link"`
	Icon  string `json:"icon"`
}

// generateSocialMediaLinks 从配置中提取社交媒体链接用于结构化数据
func generateSocialMediaLinks(settingSvc setting.SettingService) []string {
	var allLinks []string

	// 获取左侧社交链接
	leftLinksJSON := settingSvc.Get(constant.KeyFooterSocialBarLeft.String())
	var leftLinks []SocialLink
	if err := json.Unmarshal([]byte(leftLinksJSON), &leftLinks); err == nil {
		for _, link := range leftLinks {
			if link.Link != "" && !strings.HasSuffix(link.Link, ".xml") {
				// 过滤掉 RSS 链接和相对路径
				if strings.HasPrefix(link.Link, "http://") || strings.HasPrefix(link.Link, "https://") {
					allLinks = append(allLinks, link.Link)
				}
			}
		}
	}

	// 获取右侧社交链接
	rightLinksJSON := settingSvc.Get(constant.KeyFooterSocialBarRight.String())
	var rightLinks []SocialLink
	if err := json.Unmarshal([]byte(rightLinksJSON), &rightLinks); err == nil {
		for _, link := range rightLinks {
			if link.Link != "" {
				// 过滤掉相对路径
				if strings.HasPrefix(link.Link, "http://") || strings.HasPrefix(link.Link, "https://") {
					allLinks = append(allLinks, link.Link)
				}
			}
		}
	}

	// 如果没有社交链接，返回空数组
	if len(allLinks) == 0 {
		return []string{}
	}

	return allLinks
}

// rewriteStaticPathsForAdmin 为后台页面重写静态资源路径
// 将 /static/ 替换为 /admin-static/，确保后台资源始终从官方 embed 加载
func rewriteStaticPathsForAdmin(html string) string {
	// 替换 script src 中的 /static/
	html = strings.ReplaceAll(html, `src="/static/`, `src="/admin-static/`)
	// 替换 link href 中的 /static/
	html = strings.ReplaceAll(html, `href="/static/`, `href="/admin-static/`)
	// 替换可能的其他资源路径
	html = strings.ReplaceAll(html, `url("/static/`, `url("/admin-static/`)
	html = strings.ReplaceAll(html, `url('/static/`, `url('/admin-static/`)
	return html
}

// renderHTMLPageWithAdminRewrite 为后台页面渲染HTML，并重写静态资源路径
// 这确保后台页面的JS/CSS始终从官方embed加载，不受外部主题影响
func renderHTMLPageWithAdminRewrite(c *gin.Context, settingSvc setting.SettingService, articleSvc article_service.Service, templates *template.Template) {
	// 设置响应头
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate, private, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Content-Type", "text/html; charset=utf-8")

	// 获取完整的当前页面 URL
	fullURL := fmt.Sprintf("%s://%s%s", getRequestScheme(c), c.Request.Host, c.Request.URL.RequestURI())

	// 获取默认页面数据
	defaultTitle := fmt.Sprintf("%s - %s", settingSvc.Get(constant.KeyAppName.String()), settingSvc.Get(constant.KeySubTitle.String()))
	defaultDescription := settingSvc.Get(constant.KeySiteDescription.String())
	defaultImage := settingSvc.Get(constant.KeyLogoURL512.String())

	// 处理自定义HTML
	customHeaderHTML := ensureScriptTagsClosed(settingSvc.Get(constant.KeyCustomHeaderHTML.String()))
	customFooterHTML := ensureScriptTagsClosed(settingSvc.Get(constant.KeyCustomFooterHTML.String()))

	// 准备模板数据
	data := gin.H{
		"pageTitle":            defaultTitle,
		"pageDescription":      defaultDescription,
		"keywords":             settingSvc.Get(constant.KeySiteKeywords.String()),
		"author":               settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
		"themeColor":           "#f7f9fe",
		"favicon":              settingSvc.Get(constant.KeyIconURL.String()),
		"initialData":          nil,
		"ogType":               "website",
		"ogUrl":                fullURL,
		"ogTitle":              defaultTitle,
		"ogDescription":        defaultDescription,
		"ogImage":              defaultImage,
		"ogSiteName":           settingSvc.Get(constant.KeyAppName.String()),
		"ogLocale":             "zh_CN",
		"articlePublishedTime": nil,
		"articleModifiedTime":  nil,
		"articleAuthor":        nil,
		"articleTags":          nil,
		"breadcrumbList":       nil,
		"socialMediaLinks":     []string{},
		"customHeaderHTML":     template.HTML(customHeaderHTML),
		"customFooterHTML":     template.HTML(customFooterHTML),
	}

	// 渲染到 buffer
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "index.html", data); err != nil {
		log.Printf("[Admin Render] 渲染模板失败: %v", err)
		c.String(http.StatusInternalServerError, "渲染页面失败")
		return
	}

	// 重写静态资源路径
	html := rewriteStaticPathsForAdmin(buf.String())

	debugLog("后台页面静态资源路径已重写为 /admin-static/")

	// 写入响应
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write([]byte(html))
}

// renderHTMLPage 渲染HTML页面的通用函数（版本）
func renderHTMLPage(c *gin.Context, settingSvc setting.SettingService, articleSvc article_service.Service, templates *template.Template) {
	// 🚫 强制禁用HTML页面的所有缓存
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate, private, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// 获取完整的当前页面 URL
	fullURL := fmt.Sprintf("%s://%s%s", getRequestScheme(c), c.Request.Host, c.Request.URL.RequestURI())

	isPostDetail, _ := regexp.MatchString(`^/posts/([^/]+)$`, c.Request.URL.Path)
	if isPostDetail {
		slug := strings.TrimPrefix(c.Request.URL.Path, "/posts/")
		articleResponse, err := articleSvc.GetPublicBySlugOrID(c.Request.Context(), slug)
		if err != nil {
			// 文章不存在或已删除，返回 index.html 让前端处理404
			debugLog("文章未找到或已删除: %s, 错误: %v，交给前端处理", slug, err)
			// 不返回 JSON 错误，继续执行到默认页面渲染逻辑
		} else if articleResponse != nil {

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

			// 🖼️ 关键修复：在服务端渲染时将图片转换为懒加载格式，避免浏览器解析HTML时自动加载
			articleResponse.ContentHTML = convertImagesToLazyLoad(articleResponse.ContentHTML)

			// 处理自定义HTML，确保script标签正确闭合
			customHeaderHTML := ensureScriptTagsClosed(settingSvc.Get(constant.KeyCustomHeaderHTML.String()))
			customFooterHTML := ensureScriptTagsClosed(settingSvc.Get(constant.KeyCustomFooterHTML.String()))

			// 创建包含时间戳的初始数据
			initialDataWithTimestamp := map[string]interface{}{
				"data":          articleResponse,
				"__timestamp__": time.Now().UnixMilli(), // 添加时间戳用于客户端验证数据新鲜度
			}

			// 确定使用的 keywords：优先使用文章的 keywords，否则使用全站的 keywords
			keywords := settingSvc.Get(constant.KeySiteKeywords.String())
			if articleResponse.Keywords != "" {
				keywords = articleResponse.Keywords
			}

			// 生成面包屑导航数据
			baseURL := settingSvc.Get(constant.KeySiteURL.String())
			breadcrumbList := generateBreadcrumbList(c.Request.URL.Path, baseURL, settingSvc)
			// 将文章标题更新到面包屑的最后一项
			if len(breadcrumbList) > 0 {
				breadcrumbList[len(breadcrumbList)-1]["name"] = articleResponse.Title
			}

			// 生成社交媒体链接
			socialMediaLinks := generateSocialMediaLinks(settingSvc)

			// 使用传入的模板实例渲染
			render := CustomHTMLRender{Templates: templates}
			c.Render(http.StatusOK, render.Instance("index.html", gin.H{
				// --- 基础 SEO 和页面信息 ---
				"pageTitle":       pageTitle,
				"pageDescription": pageDescription,
				"keywords":        keywords,
				"author":          settingSvc.Get(constant.KeyFrontDeskSiteOwnerName.String()),
				"themeColor":      articleResponse.PrimaryColor,
				"favicon":         settingSvc.Get(constant.KeyIconURL.String()),
				// --- 用于 Vue 水合的数据（包含时间戳） ---
				"initialData":   initialDataWithTimestamp,
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
				// --- 面包屑导航数据 ---
				"breadcrumbList": breadcrumbList,
				// --- 社交媒体链接 ---
				"socialMediaLinks": socialMediaLinks,
				// --- 自定义HTML（包含CSS/JS） ---
				"customHeaderHTML": template.HTML(customHeaderHTML),
				"customFooterHTML": template.HTML(customFooterHTML),
			}))
			return
		}
	}

	// --- 默认页面渲染 ---
	defaultTitle := fmt.Sprintf("%s - %s", settingSvc.Get(constant.KeyAppName.String()), settingSvc.Get(constant.KeySubTitle.String()))
	defaultDescription := settingSvc.Get(constant.KeySiteDescription.String())
	defaultImage := settingSvc.Get(constant.KeyLogoURL512.String())

	// 处理自定义HTML，确保script标签正确闭合
	customHeaderHTML := ensureScriptTagsClosed(settingSvc.Get(constant.KeyCustomHeaderHTML.String()))
	customFooterHTML := ensureScriptTagsClosed(settingSvc.Get(constant.KeyCustomFooterHTML.String()))

	// 生成面包屑导航数据
	baseURL := settingSvc.Get(constant.KeySiteURL.String())
	breadcrumbList := generateBreadcrumbList(c.Request.URL.Path, baseURL, settingSvc)

	// 生成社交媒体链接
	socialMediaLinks := generateSocialMediaLinks(settingSvc)

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
		// --- 面包屑导航数据 ---
		"breadcrumbList": breadcrumbList,
		// --- 社交媒体链接 ---
		"socialMediaLinks": socialMediaLinks,
		// --- 自定义HTML（包含CSS/JS） ---
		"customHeaderHTML": template.HTML(customHeaderHTML),
		"customFooterHTML": template.HTML(customFooterHTML),
	}))
}
