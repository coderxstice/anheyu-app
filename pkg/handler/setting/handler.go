/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-06-15 12:26:45
 * @LastEditTime: 2025-08-13 10:16:47
 * @LastEditors: 安知鱼
 */
package setting_handler

import (
	"log"
	"net/http"

	"github.com/anzhiyu-c/anheyu-app/pkg/handler/setting/dto"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/utility"

	"github.com/gin-gonic/gin"
)

// SettingHandler 封装了站点配置相关的控制器方法
// 它现在也依赖 EmailService 来处理邮件发送请求。
type SettingHandler struct {
	settingSvc setting.SettingService
	emailSvc   utility.EmailService
}

// NewSettingHandler 是 SettingHandler 的构造函数
// 注意：构造函数已更新，需要注入 EmailService。
// 您需要更新您的依赖注入配置（例如 wire.go）。
func NewSettingHandler(
	settingSvc setting.SettingService,
	emailSvc utility.EmailService,
) *SettingHandler {
	return &SettingHandler{
		settingSvc: settingSvc,
		emailSvc:   emailSvc,
	}
}

// TestEmail
// @Summary      发送测试邮件
// @Description  根据当前配置发送一封测试邮件到指定地址，用于验证邮件服务是否可用。
// @Tags         Setting Admin
// @Accept       json
// @Produce      json
// @Param        body body dto.TestEmailRequest true "测试邮件请求"
// @Success      200 {object} response.Response "成功发送"
// @Failure      400 {object} response.Response "请求参数错误"
// @Failure      500 {object} response.Response "邮件发送失败"
// @Security     ApiKeyAuth
// @Router       /settings/test-email [post]
func (h *SettingHandler) TestEmail(c *gin.Context) {
	var req dto.TestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	err := h.emailSvc.SendTestEmail(c.Request.Context(), req.ToEmail)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "发送测试邮件失败: "+err.Error())
		return
	}

	response.Success(c, nil, "测试邮件已发送，请检查收件箱")
}

// GetSiteConfig 处理获取公开的站点配置的请求
func (h *SettingHandler) GetSiteConfig(c *gin.Context) {
	siteConfig := h.settingSvc.GetSiteConfig()
	response.Success(c, siteConfig, "获取站点配置成功")
}

// GetSettingsByKeysReq 定义了按键获取配置的请求体结构
type GetSettingsByKeysReq struct {
	Keys []string `json:"keys" binding:"required,min=1"`
}

// GetSettingsByKeys 处理根据一组键名批量获取配置的请求
func (h *SettingHandler) GetSettingsByKeys(c *gin.Context) {
	var req GetSettingsByKeysReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: 'keys' 不能为空")
		return
	}

	settings := h.settingSvc.GetByKeys(req.Keys)
	response.Success(c, settings, "获取配置成功")
}

// UpdateSettings 处理批量更新配置项的请求
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var settingsToUpdate map[string]string
	if err := c.ShouldBindJSON(&settingsToUpdate); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	if len(settingsToUpdate) == 0 {
		response.Fail(c, http.StatusBadRequest, "没有需要更新的配置项")
		return
	}

	// 调用 Service 层执行更新
	err := h.settingSvc.UpdateSettings(c.Request.Context(), settingsToUpdate)
	if err != nil {
		log.Printf("更新站点配置时发生错误: %v", err)
		response.Fail(c, http.StatusInternalServerError, "更新配置失败，请查看服务器日志")
		return
	}

	response.Success(c, nil, "更新配置成功")
}
