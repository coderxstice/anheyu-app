package frontend

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProxyMiddleware creates a reverse proxy middleware that forwards
// non-API requests to the Next.js frontend service.
func ProxyMiddleware(launcher *Launcher) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if shouldSkipProxy(path) {
			c.Next()
			return
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
			w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>前端服务暂不可用</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;padding:60px}h1{color:#333}p{color:#666}</style>
</head><body>
<h1>前端服务暂时不可用</h1>
<p>Next.js 服务正在启动中或遇到问题，请稍后刷新。</p>
<p>目标地址: %s</p>
</body></html>`, launcher.GetFrontendURL())))
		}

		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func shouldSkipProxy(path string) bool {
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
		"/static/",
		"/f/",
		"/needcache/",
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

func scheme(c *gin.Context) string {
	if c.Request.TLS != nil {
		return "https"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}
