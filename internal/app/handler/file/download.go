/*
 * @Description: 处理文件下载请求的HTTP Handler，适配本地与云端存储。
 * @Author: 安知鱼
 * @Date: 2025-06-29 21:34:22
 * @LastEditTime: 2025-07-19 10:39:55
 * @LastEditors: 安知鱼
 */
package file

import (
	"anheyu-app/internal/constant"
	"anheyu-app/internal/pkg/idgen"
	"anheyu-app/internal/pkg/response"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// HandleUniversalSignedDownload 处理所有带签名的下载请求 (e.g., GET /api/download/:public_id?sign=...)
func (h *FileHandler) HandleUniversalSignedDownload(c *gin.Context) {
	publicFileID := c.Param("public_id")

	err := h.fileSvc.ProcessSignedDownload(c.Request.Context(), c.Writer, c.Request, publicFileID)

	if err != nil {
		if !c.Writer.Written() {
			if errors.Is(err, constant.ErrLinkExpired) || errors.Is(err, constant.ErrSignatureInvalid) {
				response.Fail(c, http.StatusForbidden, err.Error())
			} else if errors.Is(err, constant.ErrNotFound) {
				response.Fail(c, http.StatusNotFound, "文件不存在")
			} else {
				response.Fail(c, http.StatusInternalServerError, "下载文件时发生错误: "+err.Error())
			}
		}
		return
	}
}

// DownloadFile 处理需要JWT认证的文件下载请求 (e.g., GET /api/file/download/:id)
func (h *FileHandler) DownloadFile(c *gin.Context) {
	publicFileID := c.Param("id")
	if publicFileID == "" {
		response.Fail(c, http.StatusBadRequest, "文件ID不能为空")
		return
	}

	claims, err := getClaims(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	// 从 claims 中解码出数据库 viewerID
	viewerID, userEntityType, err := idgen.DecodePublicID(claims.UserID)
	if err != nil || userEntityType != idgen.EntityTypeUser {
		response.Fail(c, http.StatusUnauthorized, "无效的用户凭证")
		return
	}

	// 调用核心下载服务，传入正确的 uint 类型的 viewerID
	fileMeta, err := h.fileSvc.Download(c.Request.Context(), uint(viewerID), publicFileID, c.Writer)

	if err != nil {
		if !c.Writer.Written() {
			if errors.Is(err, constant.ErrNotFound) {
				response.Fail(c, http.StatusNotFound, "文件不存在")
			} else if errors.Is(err, constant.ErrForbidden) {
				response.Fail(c, http.StatusForbidden, "无权下载此文件")
			} else {
				response.Fail(c, http.StatusInternalServerError, "下载文件失败: "+err.Error())
			}
		}
		return
	}

	if !c.Writer.Written() {
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(fileMeta.Name)))
		c.Header("Content-Length", fmt.Sprintf("%d", fileMeta.Size))
	}
}
