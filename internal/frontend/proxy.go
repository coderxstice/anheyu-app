package frontend

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProxyMiddleware creates a reverse proxy middleware that forwards
// non-API requests to the Next.js frontend service.
// When a valid static directory is detected (custom frontend mode), public-facing
// pages are served from it; admin pages still proxy to Next.js.
func ProxyMiddleware(launcher *Launcher) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if shouldSkipProxy(path, launcher.SkipStaticProxy()) {
			c.Next()
			return
		}

		// 自定义前端模式：从 static 目录提供前台页面，
		// 管理后台路径和未找到的静态资源将穿透到 Next.js 代理。
		if IsStaticModeActive() {
			if handleStaticRequest(c, path) {
				return
			}
		}

		if !launcher.IsRunning() {
			c.Next()
			return
		}

		target, err := url.Parse(launcher.GetFrontendURL())
		if err != nil {
			log.Printf("[Frontend Proxy] URL 解析失败: %v", err)
			c.Next()
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = req.URL.Host
			req.Header.Set("X-Forwarded-Host", c.Request.Host)
			req.Header.Set("X-Forwarded-Proto", scheme(c))
			req.Header.Set("X-Real-IP", c.ClientIP())
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
			log.Printf("[Frontend Proxy] 代理错误: %v (target: %s)", proxyErr, launcher.GetFrontendURL())
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>前端服务暂不可用</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;padding:60px}h1{color:#333}p{color:#666}</style>
</head><body>
<h1>前端服务暂时不可用</h1>
<p>服务正在启动中或遇到问题，请稍后刷新页面重试。</p>
</body></html>`))
		}

		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

// shouldSkipProxy 决定请求是否不代理、交给 Go 处理。
// skipStaticProxy 为 true 时 /static/ 也跳过，由 Go 提供主题目录（自定义前端在 /static 下）；默认 false，/static 代理到 Next.js。
func shouldSkipProxy(path string, skipStaticProxy bool) bool {
	exactPaths := []string{
		"/robots.txt",
		"/sitemap.xml",
		"/rss.xml",
		"/feed.xml",
		"/atom.xml",
	}
	for _, exact := range exactPaths {
		if path == exact {
			return true
		}
	}

	skipPrefixes := []string{
		"/api/",
		"/f/",
		"/needcache/",
	}
	if skipStaticProxy {
		// 自定义主题模式：/static 由 Go 提供，不代理到 Next.js
		skipPrefixes = append(skipPrefixes, "/static/")
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// scheme 返回请求使用的协议。仅信任 TLS 状态和合法的 X-Forwarded-Proto 值，
// 需配合 Gin 的 TrustedProxies 配置确保该头来自受信任代理。
func scheme(c *gin.Context) string {
	if c.Request.TLS != nil {
		return "https"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto == "https" || proto == "http" {
		return proto
	}
	return "http"
}
