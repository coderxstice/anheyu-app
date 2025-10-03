/*
 * @Description: 主题管理处理器（优化版）
 * @Author: 安知鱼
 * @Date: 2025-09-18 11:00:00
 * @LastEditTime: 2025-09-21 21:56:02
 * @LastEditors: 安知鱼
 *
 * 1. 结构化数据绑定和验证
 * 2. 统一错误处理
 * 3. 更好的Context管理
 * 4. 日志记录优化
 */
package theme

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/anzhiyu-c/anheyu-app/internal/pkg/auth"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/theme"
	"github.com/gin-gonic/gin"
)

// Handler 主题管理处理器
type Handler struct {
	themeService theme.ThemeService
}

// ThemeHandler 类型别名，简化引用
type ThemeHandler = Handler

// 请求结构体定义
type (
	// ThemeInstallRequest 主题安装请求
	ThemeInstallRequest struct {
		ThemeName     string `json:"theme_name" binding:"required,min=1,max=100"`
		DownloadURL   string `json:"download_url" binding:"required,url"`
		ThemeMarketID *int   `json:"theme_market_id,omitempty"`
	}

	// ThemeSwitchRequest 主题切换请求
	ThemeSwitchRequest struct {
		ThemeName string `json:"theme_name" binding:"required,min=1,max=100"`
	}

	// ThemeUninstallRequest 主题卸载请求
	ThemeUninstallRequest struct {
		ThemeName string `json:"theme_name" binding:"required,min=1,max=100"`
	}

	// ThemeUploadResponse 主题上传响应
	ThemeUploadResponse struct {
		ThemeName string      `json:"theme_name"`
		ThemeInfo interface{} `json:"theme_info"`
		Installed bool        `json:"installed"`
		Message   string      `json:"message"`
	}
)

// NewHandler 创建主题管理处理器实例
func NewHandler(themeService theme.ThemeService) *Handler {
	return &Handler{
		themeService: themeService,
	}
}

// 辅助函数：统一的用户ID提取和验证
func (h *Handler) extractUserID(c *gin.Context) (uint, error) {
	// 从JWT中间件设置的Claims中获取用户信息
	claimsValue, exists := c.Get("user_claims")
	if !exists {
		return 0, errors.New("用户未登录")
	}

	// 类型断言为CustomClaims
	claims, ok := claimsValue.(*auth.CustomClaims)
	if !ok {
		return 0, errors.New("用户认证信息格式错误")
	}

	// 解码公共用户ID为内部数据库ID
	userID, entityType, err := idgen.DecodePublicID(claims.UserID)
	if err != nil {
		return 0, errors.New("用户ID解码失败: " + err.Error())
	}
	if entityType != idgen.EntityTypeUser {
		return 0, errors.New("用户ID类型不匹配")
	}

	return userID, nil
}

// 辅助函数：统一的错误响应处理
func (h *Handler) handleError(c *gin.Context, err error, message string, statusCode int) {
	log.Printf("[Theme Handler Error] %s: %v", message, err)
	response.Fail(c, statusCode, message+": "+err.Error())
}

// GetCurrentTheme 获取当前使用的主题
// @Summary      获取当前主题
// @Description  获取用户当前使用的主题信息
// @Tags         主题管理
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.Response{data=theme.ThemeInfo}  "获取成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      401  {object}  response.Response  "未授权"
// @Failure      500  {object}  response.Response  "获取失败"
// @Router       /theme/current [get]
func (h *Handler) GetCurrentTheme(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	// 获取当前主题信息
	themeInfo, err := h.themeService.GetCurrentTheme(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err, "获取当前主题失败", http.StatusInternalServerError)
		return
	}

	log.Printf("[Theme Handler] 成功获取用户 %d 的当前主题: %s", userID, themeInfo.Name)
	response.Success(c, themeInfo, "获取当前主题成功")
}

// GetInstalledThemes 获取已安装的主题列表
// @Summary      获取已安装主题列表
// @Description  获取用户已安装的所有主题
// @Tags         主题管理
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.Response{data=[]theme.ThemeInfo}  "获取成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      500  {object}  response.Response  "获取失败"
// @Router       /theme/installed [get]
func (h *Handler) GetInstalledThemes(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	themes, err := h.themeService.GetInstalledThemes(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取已安装主题失败: "+err.Error())
		return
	}

	response.Success(c, themes, "获取已安装主题成功")
}

// InstallTheme 安装主题
// @Summary      安装主题
// @Description  从指定URL下载并安装主题（主题名必须以theme-开头）
// @Tags         主题管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request  body  theme.ThemeInstallRequest  true  "主题安装请求"
// @Success      200  {object}  response.Response  "安装成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      500  {object}  response.Response  "安装失败"
// @Router       /theme/install [post]
func (h *Handler) InstallTheme(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	var req theme.ThemeInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误: "+err.Error())
		return
	}

	// 基础参数验证
	if req.ThemeName == "" {
		response.Fail(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}

	if req.DownloadURL == "" {
		response.Fail(c, http.StatusBadRequest, "下载URL不能为空")
		return
	}

	// 主题名称必须以theme-开头
	if len(req.ThemeName) < 6 || req.ThemeName[:6] != "theme-" {
		response.Fail(c, http.StatusBadRequest, "主题名称必须以'theme-'开头")
		return
	}

	err = h.themeService.InstallTheme(c.Request.Context(), userID, &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "安装主题失败: "+err.Error())
		return
	}

	response.Success(c, nil, "主题安装成功")
}

// SwitchTheme 切换主题
// @Summary      切换主题
// @Description  切换到指定的已安装主题或官方主题
// @Tags         主题管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request  body  SwitchThemeRequest  true  "切换主题请求"
// @Success      200  {object}  response.Response  "切换成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      500  {object}  response.Response  "切换失败"
// @Router       /theme/switch [post]
func (h *Handler) SwitchTheme(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	var req SwitchThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误: "+err.Error())
		return
	}

	if req.ThemeName == "" {
		response.Fail(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}

	err = h.themeService.SwitchToTheme(c.Request.Context(), userID, req.ThemeName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "切换主题失败: "+err.Error())
		return
	}

	// 添加缓存清理头，告诉浏览器清理静态文件缓存
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	response.Success(c, nil, "主题切换成功")
}

// SwitchToOfficial 切换到官方主题
// @Summary      切换到官方主题
// @Description  切换到官方内嵌主题
// @Tags         主题管理
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.Response  "切换成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      500  {object}  response.Response  "切换失败"
// @Router       /theme/official [post]
func (h *Handler) SwitchToOfficial(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	err = h.themeService.SwitchToOfficial(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "切换到官方主题失败: "+err.Error())
		return
	}

	// 添加缓存清理头，告诉浏览器清理静态文件缓存
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	response.Success(c, nil, "成功切换到官方主题")
}

// UninstallTheme 卸载主题
// @Summary      卸载主题
// @Description  卸载指定的主题（不能卸载当前使用的主题）
// @Tags         主题管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        request  body  UninstallThemeRequest  true  "卸载主题请求"
// @Success      200  {object}  response.Response  "卸载成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      500  {object}  response.Response  "卸载失败"
// @Router       /theme/uninstall [post]
func (h *Handler) UninstallTheme(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	var req UninstallThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误: "+err.Error())
		return
	}

	if req.ThemeName == "" {
		response.Fail(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}

	err = h.themeService.UninstallTheme(c.Request.Context(), userID, req.ThemeName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "卸载主题失败: "+err.Error())
		return
	}

	response.Success(c, nil, "主题卸载成功")
}

// ThemeMarketListResponse 主题商城列表响应结构
type ThemeMarketListResponse struct {
	List  []*theme.MarketTheme `json:"list"`
	Total int                  `json:"total"`
}

// GetThemeMarket 获取主题商城列表
// @Summary      获取主题商城列表
// @Description  获取主题商城中的所有可用主题
// @Tags         主题商城
// @Produce      json
// @Success      200  {object}  response.Response{data=ThemeMarketListResponse}  "获取成功"
// @Failure      500  {object}  response.Response  "获取失败"
// @Router       /public/theme/market [get]
func (h *Handler) GetThemeMarket(c *gin.Context) {
	themes, err := h.themeService.GetThemeMarketList(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取主题商城列表失败: "+err.Error())
		return
	}

	// 构造符合前端期待的数据格式
	responseData := ThemeMarketListResponse{
		List:  themes,
		Total: len(themes),
	}

	response.Success(c, responseData, "获取主题商城列表成功")
}

// CheckStaticMode 检查是否处于静态模式
// @Summary      检查静态模式
// @Description  检查当前是否处于静态主题模式（是否存在static目录）
// @Tags         主题管理
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.Response{data=StaticModeResponse}  "获取成功"
// @Router       /theme/static-mode [get]
func (h *Handler) CheckStaticMode(c *gin.Context) {
	isActive := h.themeService.IsStaticModeActive()

	response.Success(c, StaticModeResponse{
		IsActive: isActive,
	}, "获取静态模式状态成功")
}

// SwitchThemeRequest 切换主题请求结构
type SwitchThemeRequest struct {
	ThemeName string `json:"theme_name" binding:"required"`
}

// UninstallThemeRequest 卸载主题请求结构
type UninstallThemeRequest struct {
	ThemeName string `json:"theme_name" binding:"required"`
}

// StaticModeResponse 静态模式响应结构
type StaticModeResponse struct {
	IsActive bool `json:"is_active"`
}

// UploadTheme 上传主题压缩包
// @Summary      上传主题压缩包
// @Description  上传主题压缩包文件（ZIP格式，最大50MB），系统会自动解析theme.json并安装主题
// @Tags         主题管理
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        file          formData  file    true   "主题压缩包文件"
// @Param        force_update  formData  string  false  "是否强制更新"
// @Success      200  {object}  response.Response{data=ThemeUploadResponse}  "上传成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      401  {object}  response.Response  "未授权"
// @Failure      500  {object}  response.Response  "上传失败"
// @Router       /theme/upload [post]
func (h *Handler) UploadTheme(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "获取上传文件失败: "+err.Error())
		return
	}

	// 验证文件类型
	if file.Header.Get("Content-Type") != "application/zip" &&
		!strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		response.Fail(c, http.StatusBadRequest, "仅支持ZIP格式的主题压缩包")
		return
	}

	// 验证文件大小（最大50MB）
	const maxFileSize = 50 * 1024 * 1024 // 50MB
	if file.Size > maxFileSize {
		response.Fail(c, http.StatusBadRequest, "文件大小不能超过50MB")
		return
	}

	// 检查是否有强制更新标志
	forceUpdate := c.PostForm("force_update") == "true"

	// 调用服务层处理上传
	themeInfo, err := h.themeService.UploadTheme(c.Request.Context(), userID, file, forceUpdate)
	if err != nil {
		h.handleError(c, err, "上传主题失败", http.StatusInternalServerError)
		return
	}

	// 构造响应
	uploadResponse := ThemeUploadResponse{
		ThemeName: themeInfo.Name,
		ThemeInfo: themeInfo,
		Installed: true,
		Message:   "主题上传并安装成功",
	}

	log.Printf("[Theme Handler] 用户 %d 成功上传主题: %s", userID, themeInfo.Name)
	response.Success(c, uploadResponse, "主题上传成功")
}

// ValidateTheme 验证主题压缩包
// @Summary      验证主题压缩包
// @Description  验证主题压缩包的格式和内容是否符合规范
// @Tags         主题管理
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "主题压缩包文件"
// @Success      200  {object}  response.Response  "验证成功"
// @Failure      400  {object}  response.Response  "验证失败"
// @Failure      401  {object}  response.Response  "未授权"
// @Router       /theme/validate [post]
func (h *Handler) ValidateTheme(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		h.handleError(c, err, "获取上传文件失败", http.StatusBadRequest)
		return
	}

	// 验证主题压缩包
	result, err := h.themeService.ValidateThemePackage(c.Request.Context(), userID, file)
	if err != nil {
		h.handleError(c, err, "主题验证失败", http.StatusBadRequest)
		return
	}

	log.Printf("[Theme Handler] 主题验证完成: %v", result.IsValid)
	response.Success(c, result, "主题验证完成")
}

// FixThemeStatus 修复主题状态数据一致性
// @Summary      修复主题状态
// @Description  修复用户主题的当前状态数据一致性，解决多个主题同时标记为当前使用的问题
// @Tags         主题管理
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  response.Response  "修复成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      401  {object}  response.Response  "未授权"
// @Failure      500  {object}  response.Response  "修复失败"
// @Router       /theme/fix-status [post]
func (h *Handler) FixThemeStatus(c *gin.Context) {
	// 提取用户ID
	userID, err := h.extractUserID(c)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "用户未登录" {
			status = http.StatusUnauthorized
		}
		response.Fail(c, status, err.Error())
		return
	}

	// 调用修复方法
	err = h.themeService.FixThemeCurrentStatus(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err, "修复主题状态失败", http.StatusInternalServerError)
		return
	}

	log.Printf("[Theme Handler] 用户 %d 的主题状态修复完成", userID)
	response.Success(c, nil, "主题状态修复完成")
}
