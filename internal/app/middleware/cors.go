package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 只对 API 路由应用 CORS 头部
		if strings.HasPrefix(path, "/api/") {
			origin := c.Request.Header.Get("Origin")

			// 可以设置为 * 允许所有，或限制域名 origin
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			// 添加更多允许的头部，包括文件下载相关的头部
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token, X-Requested-With, Range, Accept-Ranges, Content-Range, Content-Length, Content-Disposition")
			c.Header("Access-Control-Expose-Headers", "Authorization, Content-Range, Content-Length, Content-Disposition")
			c.Header("Access-Control-Allow-Credentials", "true")

			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}

		c.Next()
	}
}
