/*
 * @Description: 配置导入导出 Handler
 * @Author: 安知鱼
 * @Date: 2025-10-19
 */
package config_handler

import (
	"log"
	"net/http"

	"github.com/anzhiyu-c/anheyu-app/pkg/service/config"
	"github.com/gin-gonic/gin"
)

// ConfigImportExportHandler 处理配置导入导出相关的HTTP请求
type ConfigImportExportHandler struct {
	importExportSvc config.ImportExportService
}

// NewConfigImportExportHandler 创建一个新的配置导入导出Handler实例
func NewConfigImportExportHandler(importExportSvc config.ImportExportService) *ConfigImportExportHandler {
	return &ConfigImportExportHandler{
		importExportSvc: importExportSvc,
	}
}

// ExportConfig 导出配置数据
// @Summary      导出配置数据
// @Description  导出数据库中的所有配置项（JSON 格式）
// @Tags         配置管理
// @Produce      application/json
// @Success      200 {file} file "配置文件"
// @Failure      500 {object} response.Response "导出失败"
// @Security     BearerAuth
// @Router       /config/export [get]
func (h *ConfigImportExportHandler) ExportConfig(c *gin.Context) {
	content, err := h.importExportSvc.ExportConfig(c.Request.Context())
	if err != nil {
		log.Printf("导出配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "导出配置失败: " + err.Error(),
		})
		return
	}

	// 设置文件下载响应头
	filename := "anheyu-settings.json"
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/json")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Data(http.StatusOK, "application/json", content)
}

// ImportConfig 导入配置数据
// @Summary      导入配置数据
// @Description  导入配置数据到数据库（JSON 格式）
// @Tags         配置管理
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "配置文件（JSON格式）"
// @Success      200 {object} response.Response "导入成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      500 {object} response.Response "导入失败"
// @Security     BearerAuth
// @Router       /config/import [post]
func (h *ConfigImportExportHandler) ImportConfig(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "请上传配置文件",
		})
		return
	}

	// 检查文件扩展名
	if len(file.Filename) < 5 || file.Filename[len(file.Filename)-5:] != ".json" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "配置文件必须是 .json 格式",
		})
		return
	}

	// 读取文件内容
	fileContent, err := file.Open()
	if err != nil {
		log.Printf("读取上传文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "读取文件失败: " + err.Error(),
		})
		return
	}
	defer fileContent.Close()

	// 导入配置
	if err := h.importExportSvc.ImportConfig(c.Request.Context(), fileContent); err != nil {
		log.Printf("导入配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    http.StatusInternalServerError,
			"message": "导入配置失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "配置导入成功，已更新到数据库",
	})
}
