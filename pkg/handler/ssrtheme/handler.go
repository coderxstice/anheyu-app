/*
 * SSR 主题 API 处理器
 * 提供 SSR 主题的安装、启动、停止、卸载等功能
 */
package ssrtheme

import (
	"net/http"

	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/ssr"
	"github.com/gin-gonic/gin"
)

// Handler SSR 主题处理器
type Handler struct {
	manager *ssr.Manager
}

// NewHandler 创建 SSR 主题处理器
func NewHandler(manager *ssr.Manager) *Handler {
	return &Handler{manager: manager}
}

// GetManager 获取 SSR 管理器（供中间件使用）
func (h *Handler) GetManager() *ssr.Manager {
	return h.manager
}

// InstallThemeRequest 安装主题请求
type InstallThemeRequest struct {
	ThemeName   string `json:"themeName" binding:"required"`
	DownloadURL string `json:"downloadUrl" binding:"required"`
}

// StartThemeRequest 启动主题请求
type StartThemeRequest struct {
	Port int `json:"port"`
}

// InstallTheme 安装 SSR 主题
// @Summary 安装 SSR 主题
// @Description 从指定 URL 下载并安装 SSR 主题
// @Tags SSR主题管理
// @Accept json
// @Produce json
// @Param request body InstallThemeRequest true "安装请求"
// @Success 200 {object} response.Response
// @Router /api/admin/ssr-theme/install [post]
func (h *Handler) InstallTheme(c *gin.Context) {
	var req InstallThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if err := h.manager.Install(c.Request.Context(), req.ThemeName, req.DownloadURL); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil, "主题安装成功")
}

// UninstallTheme 卸载 SSR 主题
// @Summary 卸载 SSR 主题
// @Description 卸载指定的 SSR 主题
// @Tags SSR主题管理
// @Produce json
// @Param name path string true "主题名称"
// @Success 200 {object} response.Response
// @Router /api/admin/ssr-theme/{name} [delete]
func (h *Handler) UninstallTheme(c *gin.Context) {
	themeName := c.Param("name")
	if themeName == "" {
		response.Fail(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}

	if err := h.manager.Uninstall(themeName); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil, "主题卸载成功")
}

// StartTheme 启动 SSR 主题
// @Summary 启动 SSR 主题
// @Description 启动指定的 SSR 主题
// @Tags SSR主题管理
// @Accept json
// @Produce json
// @Param name path string true "主题名称"
// @Param request body StartThemeRequest false "启动参数"
// @Success 200 {object} response.Response
// @Router /api/admin/ssr-theme/{name}/start [post]
func (h *Handler) StartTheme(c *gin.Context) {
	themeName := c.Param("name")
	if themeName == "" {
		response.Fail(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}

	var req StartThemeRequest
	c.ShouldBindJSON(&req) // 忽略绑定错误，使用默认值
	if req.Port == 0 {
		req.Port = 3000
	}

	if err := h.manager.Start(themeName, req.Port); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, gin.H{"port": req.Port}, "主题启动成功")
}

// StopTheme 停止 SSR 主题
// @Summary 停止 SSR 主题
// @Description 停止指定的 SSR 主题
// @Tags SSR主题管理
// @Produce json
// @Param name path string true "主题名称"
// @Success 200 {object} response.Response
// @Router /api/admin/ssr-theme/{name}/stop [post]
func (h *Handler) StopTheme(c *gin.Context) {
	themeName := c.Param("name")
	if themeName == "" {
		response.Fail(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}

	if err := h.manager.Stop(themeName); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil, "主题停止成功")
}

// GetThemeStatus 获取主题状态
// @Summary 获取 SSR 主题状态
// @Description 获取指定 SSR 主题的状态信息
// @Tags SSR主题管理
// @Produce json
// @Param name path string true "主题名称"
// @Success 200 {object} response.Response
// @Router /api/admin/ssr-theme/{name}/status [get]
func (h *Handler) GetThemeStatus(c *gin.Context) {
	themeName := c.Param("name")
	if themeName == "" {
		response.Fail(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}

	status := h.manager.GetStatus(themeName)
	response.Success(c, status, "获取成功")
}

// ListInstalledThemes 列出已安装的 SSR 主题
// @Summary 列出已安装的 SSR 主题
// @Description 获取所有已安装的 SSR 主题列表
// @Tags SSR主题管理
// @Produce json
// @Success 200 {object} response.Response
// @Router /api/admin/ssr-theme/list [get]
func (h *Handler) ListInstalledThemes(c *gin.Context) {
	themes, err := h.manager.ListInstalled()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, themes, "获取成功")
}
