package file

import (
	"github.com/anzhiyu-c/anheyu-app/internal/constant"
	"github.com/anzhiyu-c/anheyu-app/internal/domain/model"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/response"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CreateUploadSession 处理创建上传会话的请求 (PUT /api/file/upload)
func (h *FileHandler) CreateUploadSession(c *gin.Context) {
	var req model.CreateUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	claims, err := getClaims(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	ownerID, _, err := idgen.DecodePublicID(claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "无效的用户凭证")
		return
	}

	sessionData, err := h.uploadSvc.CreateUploadSession(c.Request.Context(), ownerID, &req)
	if err != nil {
		if errors.Is(err, constant.ErrConflict) {
			response.Fail(c, http.StatusConflict, "创建失败: "+err.Error())
		} else if errors.Is(err, constant.ErrNotFound) {
			response.Fail(c, http.StatusNotFound, "创建失败: "+err.Error())
		} else {
			response.Fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		}
		return
	}
	response.Success(c, sessionData, "上传会话创建成功")
}

// GetUploadSessionStatus 处理获取上传会话状态的请求 (GET /api/file/upload/session/:sessionId)
func (h *FileHandler) GetUploadSessionStatus(c *gin.Context) {
	sessionId := c.Param("sessionId")
	if sessionId == "" {
		response.Fail(c, http.StatusBadRequest, "缺少 sessionId")
		return
	}

	claims, err := getClaims(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	ownerID, _, err := idgen.DecodePublicID(claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "无效的用户凭证")
		return
	}

	sessionStatus, err := h.uploadSvc.GetUploadSessionStatus(c.Request.Context(), ownerID, sessionId)
	if err != nil {
		if errors.Is(err, constant.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"data":    model.UploadSessionInvalidResponse{IsValid: false},
				"message": "上传会话不存在或已过期",
			})
			return
		}
		if errors.Is(err, constant.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    http.StatusForbidden,
				"data":    model.UploadSessionInvalidResponse{IsValid: false},
				"message": "无权访问此上传会话",
			})
			return
		}
		response.Fail(c, http.StatusInternalServerError, "服务器内部错误: "+err.Error())
		return
	}

	response.Success(c, sessionStatus, "会话有效")
}

// UploadChunk 处理上传文件分片的请求 (POST /api/file/upload/:sessionId/:index)
func (h *FileHandler) UploadChunk(c *gin.Context) {
	sessionID := c.Param("sessionId")
	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 {
		response.Fail(c, http.StatusBadRequest, "无效的分块索引")
		return
	}
	err = h.uploadSvc.UploadChunk(c.Request.Context(), sessionID, index, c.Request.Body)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "文件块上传失败: "+err.Error())
		return
	}
	response.Success(c, nil, "文件块上传成功")
}

// DeleteUploadSession 处理删除/取消上传会话的请求 (DELETE /api/file/upload)
func (h *FileHandler) DeleteUploadSession(c *gin.Context) {
	var req model.DeleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	claims, err := getClaims(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	ownerID, _, err := idgen.DecodePublicID(claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "无效的用户凭证")
		return
	}

	err = h.uploadSvc.DeleteUploadSession(c.Request.Context(), ownerID, &req)
	if err != nil {
		if errors.Is(err, constant.ErrForbidden) {
			response.Fail(c, http.StatusForbidden, "删除失败: "+err.Error())
		} else {
			response.Fail(c, http.StatusInternalServerError, "删除上传会话失败: "+err.Error())
		}
		return
	}
	response.Success(c, nil, "上传会话已删除")
}
