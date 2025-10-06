// internal/app/middleware/auth.go
package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/anzhiyu-c/anheyu-app/internal/pkg/auth"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	service_auth "github.com/anzhiyu-c/anheyu-app/pkg/service/auth"

	"github.com/gin-gonic/gin"
)

// min 辅助函数，返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type Middleware struct {
	tokenSvc service_auth.TokenService
}

func NewMiddleware(tokenSvc service_auth.TokenService) *Middleware {
	return &Middleware{tokenSvc: tokenSvc}
}

// JWTAuth 是一个强制性的JWT认证中间件
func (m *Middleware) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			response.Fail(c, http.StatusUnauthorized, "请求未携带Token，无权限访问")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.Fail(c, http.StatusUnauthorized, "Token格式不正确")
			c.Abort()
			return
		}

		tokenString := parts[1]
		log.Printf("[JWTAuth] 开始解析JWT token: %s", tokenString[:min(20, len(tokenString))]+"...")
		claims, err := m.tokenSvc.ParseAccessToken(c.Request.Context(), tokenString)
		if err != nil {
			log.Printf("[JWTAuth] JWT token解析失败: %v", err)
			response.Fail(c, http.StatusUnauthorized, "无效或过期的Token")
			c.Abort()
			return
		}

		log.Printf("[JWTAuth] JWT token解析成功，设置ClaimsKey: %s", auth.ClaimsKey)
		c.Set(auth.ClaimsKey, claims)
		c.Next()
	}
}

// JWTAuthOptional 是一个可选的JWT认证中间件
// 如果没有Token，允许游客访问；如果有Token但过期，返回401触发自动刷新
func (m *Middleware) JWTAuthOptional() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.Next() // 没有Token，直接放行（游客）
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.Next() // Token格式不正确，直接放行（游客）
			return
		}

		tokenString := parts[1]
		claims, err := m.tokenSvc.ParseAccessToken(c.Request.Context(), tokenString)
		if err != nil {
			// Token无效或过期，返回401触发前端自动刷新token
			log.Printf("[JWTAuthOptional] Token解析失败: %v, 返回401触发自动刷新", err)
			response.Fail(c, http.StatusUnauthorized, "Token已过期")
			c.Abort()
			return
		}

		// Token有效，将用户信息存入context
		c.Set(auth.ClaimsKey, claims)
		c.Next()
	}
}

// AdminAuth 是一个管理员权限验证中间件
func (m *Middleware) AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[AdminAuth] 开始验证管理员权限: %s %s", c.Request.Method, c.Request.URL.Path)

		claimsValue, exists := c.Get(auth.ClaimsKey)
		if !exists {
			log.Printf("[AdminAuth] 错误: 上下文中没有找到认证信息 ClaimsKey")
			log.Printf("[AdminAuth] 可用的上下文键: %v", func() []string {
				keys := make([]string, 0, len(c.Keys))
				for k := range c.Keys {
					if key, ok := k.(string); ok {
						keys = append(keys, key)
					}
				}
				return keys
			}())
			response.Fail(c, http.StatusForbidden, "权限信息获取失败")
			c.Abort()
			return
		}

		claims, ok := claimsValue.(*auth.CustomClaims)
		if !ok {
			log.Printf("[AdminAuth] 错误: 权限信息格式不正确")
			response.Fail(c, http.StatusForbidden, "权限信息格式不正确")
			c.Abort()
			return
		}

		log.Printf("[AdminAuth] 用户信息: UserID=%s, UserGroupID=%s", claims.UserID, claims.UserGroupID)

		userGroupID, entityType, err := idgen.DecodePublicID(claims.UserGroupID)
		if err != nil || entityType != idgen.EntityTypeUserGroup {
			log.Printf("[AdminAuth] 错误: 解析用户组ID失败: %v, entityType: %v", err, entityType)
			response.Fail(c, http.StatusForbidden, "权限信息无效：用户组ID无法解析")
			c.Abort()
			return
		}

		log.Printf("[AdminAuth] 解析用户组ID: %d (需要管理员组ID: 1)", userGroupID)

		// 约定管理员的用户组ID为 1
		if userGroupID != 1 {
			log.Printf("[AdminAuth] 权限不足: 用户组ID %d 不是管理员组 (需要 1)", userGroupID)
			response.Fail(c, http.StatusForbidden, "权限不足：此操作需要管理员权限")
			c.Abort()
			return
		}

		log.Printf("[AdminAuth] 管理员权限验证通过")
		c.Next()
	}
}
