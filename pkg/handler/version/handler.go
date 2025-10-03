/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-09-26 09:52:32
 * @LastEditTime: 2025-09-26 11:36:56
 * @LastEditors: 安知鱼
 */
package version

import (
	"net/http"

	"github.com/anzhiyu-c/anheyu-app/internal/pkg/version"
	"github.com/gin-gonic/gin"
)

// Handler 版本信息处理器
type Handler struct{}

// NewHandler 创建版本信息处理器实例
func NewHandler() *Handler {
	return &Handler{}
}

// GetVersion 获取版本信息
// @Summary      获取版本信息
// @Description  获取应用的详细版本信息
// @Tags         辅助工具
// @Produce      json
// @Success      200  {object}  object{code=int,message=string,data=object}  "版本信息"
// @Router       /public/version [get]
func (h *Handler) GetVersion(c *gin.Context) {
	buildInfo := version.GetBuildInfo()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取版本信息成功",
		"data":    buildInfo,
	})
}

// GetVersionString 获取版本字符串
// @Summary      获取版本字符串
// @Description  获取应用的版本号字符串
// @Tags         辅助工具
// @Produce      json
// @Success      200  {object}  object{version=string}  "版本字符串"
// @Router       /public/version/string [get]
func (h *Handler) GetVersionString(c *gin.Context) {
	versionString := version.GetVersionString()

	c.JSON(http.StatusOK, gin.H{
		"version": versionString,
	})
}
