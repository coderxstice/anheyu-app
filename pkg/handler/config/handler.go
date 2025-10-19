/*
 * @Description: 配置备份管理 Handler
 * @Author: 安知鱼
 * @Date: 2025-10-19
 */
package config_handler

import (
	"log"
	"net/http"

	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/config"
	"github.com/gin-gonic/gin"
)

// ConfigBackupHandler 封装了配置备份相关的控制器方法
type ConfigBackupHandler struct {
	backupSvc config.BackupService
}

// NewConfigBackupHandler 是 ConfigBackupHandler 的构造函数
func NewConfigBackupHandler(backupSvc config.BackupService) *ConfigBackupHandler {
	return &ConfigBackupHandler{
		backupSvc: backupSvc,
	}
}

// CreateBackupRequest 定义了创建备份的请求体
type CreateBackupRequest struct {
	Description string `json:"description"` // 备份描述（可选）
}

// RestoreBackupRequest 定义了恢复备份的请求体
type RestoreBackupRequest struct {
	Filename string `json:"filename" binding:"required"` // 要恢复的备份文件名
}

// DeleteBackupRequest 定义了删除备份的请求体
type DeleteBackupRequest struct {
	Filename string `json:"filename" binding:"required"` // 要删除的备份文件名
}

// CleanBackupsRequest 定义了清理备份的请求体
type CleanBackupsRequest struct {
	KeepCount int `json:"keep_count" binding:"required,min=1"` // 保留的备份数量
}

// CreateBackup 创建配置备份
// @Summary      创建配置备份
// @Description  手动创建配置文件的备份
// @Tags         配置备份管理
// @Accept       json
// @Produce      json
// @Param        body body CreateBackupRequest false "备份描述"
// @Success      200 {object} response.Response{data=config.BackupInfo} "创建成功"
// @Failure      500 {object} response.Response "创建失败"
// @Security     BearerAuth
// @Router       /config/backup/create [post]
func (h *ConfigBackupHandler) CreateBackup(c *gin.Context) {
	var req CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有提供描述，使用默认描述
		req.Description = "手动备份"
	}

	if req.Description == "" {
		req.Description = "手动备份"
	}

	backup, err := h.backupSvc.CreateBackup(c.Request.Context(), req.Description, false)
	if err != nil {
		log.Printf("创建配置备份失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "创建备份失败: "+err.Error())
		return
	}

	response.Success(c, backup, "备份创建成功")
}

// ListBackups 获取所有备份列表
// @Summary      获取备份列表
// @Description  获取所有配置文件的备份列表
// @Tags         配置备份管理
// @Produce      json
// @Success      200 {object} response.Response{data=[]config.BackupInfo} "获取成功"
// @Failure      500 {object} response.Response "获取失败"
// @Security     BearerAuth
// @Router       /config/backup/list [get]
func (h *ConfigBackupHandler) ListBackups(c *gin.Context) {
	backups, err := h.backupSvc.ListBackups(c.Request.Context())
	if err != nil {
		log.Printf("获取备份列表失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "获取备份列表失败: "+err.Error())
		return
	}

	response.Success(c, backups, "获取备份列表成功")
}

// RestoreBackup 从备份恢复配置
// @Summary      恢复配置备份
// @Description  从指定的备份文件恢复配置（恢复前会自动创建当前配置的备份）
// @Tags         配置备份管理
// @Accept       json
// @Produce      json
// @Param        body body RestoreBackupRequest true "备份文件名"
// @Success      200 {object} response.Response "恢复成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      500 {object} response.Response "恢复失败"
// @Security     BearerAuth
// @Router       /config/backup/restore [post]
func (h *ConfigBackupHandler) RestoreBackup(c *gin.Context) {
	var req RestoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if err := h.backupSvc.RestoreBackup(c.Request.Context(), req.Filename); err != nil {
		log.Printf("恢复配置备份失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "恢复备份失败: "+err.Error())
		return
	}

	response.Success(c, nil, "配置已恢复成功，建议重启应用以使配置生效")
}

// DeleteBackup 删除指定的备份
// @Summary      删除配置备份
// @Description  删除指定的配置备份文件
// @Tags         配置备份管理
// @Accept       json
// @Produce      json
// @Param        body body DeleteBackupRequest true "备份文件名"
// @Success      200 {object} response.Response "删除成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      500 {object} response.Response "删除失败"
// @Security     BearerAuth
// @Router       /config/backup/delete [post]
func (h *ConfigBackupHandler) DeleteBackup(c *gin.Context) {
	var req DeleteBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if err := h.backupSvc.DeleteBackup(c.Request.Context(), req.Filename); err != nil {
		log.Printf("删除配置备份失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "删除备份失败: "+err.Error())
		return
	}

	response.Success(c, nil, "备份已删除")
}

// CleanOldBackups 清理旧备份
// @Summary      清理旧备份
// @Description  清理旧的配置备份，只保留指定数量的最新备份
// @Tags         配置备份管理
// @Accept       json
// @Produce      json
// @Param        body body CleanBackupsRequest true "保留数量"
// @Success      200 {object} response.Response "清理成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      500 {object} response.Response "清理失败"
// @Security     BearerAuth
// @Router       /config/backup/clean [post]
func (h *ConfigBackupHandler) CleanOldBackups(c *gin.Context) {
	var req CleanBackupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if err := h.backupSvc.CleanOldBackups(c.Request.Context(), req.KeepCount); err != nil {
		log.Printf("清理旧备份失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "清理备份失败: "+err.Error())
		return
	}

	response.Success(c, nil, "旧备份清理成功")
}

// ExportConfig 导出当前配置
// @Summary      导出配置数据
// @Description  导出数据库中的所有配置项（JSON 格式）
// @Tags         配置备份管理
// @Produce      application/json
// @Success      200 {file} file "配置文件"
// @Failure      500 {object} response.Response "导出失败"
// @Security     BearerAuth
// @Router       /config/export [get]
func (h *ConfigBackupHandler) ExportConfig(c *gin.Context) {
	content, err := h.backupSvc.ExportConfig(c.Request.Context())
	if err != nil {
		log.Printf("导出配置失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "导出配置失败: "+err.Error())
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

// ImportConfig 导入配置文件
// @Summary      导入配置数据
// @Description  导入配置数据到数据库（JSON 格式）
// @Tags         配置备份管理
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "配置文件（JSON格式）"
// @Success      200 {object} response.Response "导入成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      500 {object} response.Response "导入失败"
// @Security     BearerAuth
// @Router       /config/import [post]
func (h *ConfigBackupHandler) ImportConfig(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "请上传配置文件")
		return
	}

	// 检查文件扩展名
	if len(file.Filename) < 5 || file.Filename[len(file.Filename)-5:] != ".json" {
		response.Fail(c, http.StatusBadRequest, "配置文件必须是 .json 格式")
		return
	}

	// 读取文件内容
	fileContent, err := file.Open()
	if err != nil {
		log.Printf("读取上传文件失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "读取文件失败: "+err.Error())
		return
	}
	defer fileContent.Close()

	// 导入配置
	if err := h.backupSvc.ImportConfig(c.Request.Context(), fileContent); err != nil {
		log.Printf("导入配置失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "导入配置失败: "+err.Error())
		return
	}

	response.Success(c, nil, "配置导入成功，已更新到数据库")
}
