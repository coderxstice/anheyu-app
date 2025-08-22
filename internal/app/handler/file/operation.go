package file

import (
	"anheyu-app/internal/constant"
	"anheyu-app/internal/domain/model"
	"anheyu-app/internal/pkg/idgen"
	"anheyu-app/internal/pkg/response"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CopyItems 处理复制文件或文件夹的请求
func (h *FileHandler) CopyItems(c *gin.Context) {
	var req model.CopyItemsRequest
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

	err = h.fileSvc.CopyItems(c.Request.Context(), ownerID, req.SourceIDs, req.DestinationID)
	if err != nil {
		switch {
		case errors.Is(err, constant.ErrConflict):
			response.Fail(c, http.StatusConflict, "复制失败: "+err.Error())
		case errors.Is(err, constant.ErrForbidden):
			response.Fail(c, http.StatusForbidden, "复制失败: "+err.Error())
		case errors.Is(err, constant.ErrNotFound):
			response.Fail(c, http.StatusNotFound, "复制失败: "+err.Error())
		case errors.Is(err, constant.ErrInvalidOperation):
			response.Fail(c, http.StatusBadRequest, "复制失败: "+err.Error())
		default:
			response.Fail(c, http.StatusInternalServerError, "复制失败: "+err.Error())
		}
		return
	}

	response.Success(c, nil, "复制成功")
}

// MoveItems 处理移动文件或文件夹的请求
func (h *FileHandler) MoveItems(c *gin.Context) {
	var req model.MoveItemsRequest
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

	err = h.fileSvc.MoveItems(c.Request.Context(), ownerID, req.SourceIDs, req.DestinationID)
	if err != nil {
		switch {
		case errors.Is(err, constant.ErrConflict):
			response.Fail(c, http.StatusConflict, "移动失败: "+err.Error())
		case errors.Is(err, constant.ErrForbidden):
			response.Fail(c, http.StatusForbidden, "移动失败: "+err.Error())
		case errors.Is(err, constant.ErrNotFound):
			response.Fail(c, http.StatusNotFound, "移动失败: "+err.Error())
		case errors.Is(err, constant.ErrInvalidOperation):
			response.Fail(c, http.StatusBadRequest, "移动失败: "+err.Error())
		default:
			response.Fail(c, http.StatusInternalServerError, "移动失败: "+err.Error())
		}
		return
	}

	response.Success(c, nil, "移动成功")
}

// CreateEmptyFile 处理创建空文件或目录的请求
func (h *FileHandler) CreateEmptyFile(c *gin.Context) {
	var req model.CreateFileRequest
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
	fileItem, err := h.fileSvc.CreateEmptyFile(c.Request.Context(), ownerID, &req)
	if err != nil {
		if errors.Is(err, constant.ErrConflict) {
			response.Fail(c, http.StatusConflict, "创建失败: "+err.Error())
		} else {
			response.Fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		}
		return
	}
	response.Success(c, fileItem, "创建成功")
}

// DeleteItems 处理删除文件或文件夹的请求 (DELETE /api/files)
func (h *FileHandler) DeleteItems(c *gin.Context) {
	var req model.DeleteItemsRequest
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

	err = h.fileSvc.DeleteItems(c.Request.Context(), ownerID, req.IDs)
	if err != nil {
		if errors.Is(err, constant.ErrForbidden) {
			response.Fail(c, http.StatusForbidden, "删除失败: "+err.Error())
		} else {
			response.Fail(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		}
		return
	}

	response.Success(c, nil, "项目已删除")
}

// RenameItem 处理重命名文件或文件夹的请求 (PUT /api/file/rename)
func (h *FileHandler) RenameItem(c *gin.Context) {
	var req model.RenameItemRequest
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

	updatedFileItem, err := h.fileSvc.RenameItem(c.Request.Context(), ownerID, &req)
	if err != nil {
		if errors.Is(err, constant.ErrConflict) {
			response.Fail(c, http.StatusConflict, "重命名失败：目标位置已存在同名文件或文件夹")
		} else if errors.Is(err, constant.ErrForbidden) {
			response.Fail(c, http.StatusForbidden, "重命名失败：您没有权限执行此操作")
		} else if errors.Is(err, constant.ErrNotFound) {
			response.Fail(c, http.StatusNotFound, "重命名失败：要操作的项目不存在")
		} else {
			response.Fail(c, http.StatusInternalServerError, "重命名失败，发生未知错误")
		}
		return
	}
	response.Success(c, updatedFileItem, "重命名成功")
}

// UpdateFolderView 更新文件夹视图配置
func (h *FileHandler) UpdateFolderView(c *gin.Context) {
	var req model.UpdateViewConfigRequest
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
	view, err := h.fileSvc.UpdateFolderViewConfig(c.Request.Context(), ownerID, &req)
	if err != nil {
		if errors.Is(err, constant.ErrNotFound) {
			response.Fail(c, http.StatusNotFound, "操作失败: "+err.Error())
		} else if errors.Is(err, constant.ErrForbidden) {
			response.Fail(c, http.StatusForbidden, "操作失败: "+err.Error())
		} else {
			response.Fail(c, http.StatusInternalServerError, "操作失败: "+err.Error())
		}
		return
	}
	response.Success(c, view, "视图配置更新成功")
}

// UpdateFileContentByID 处理通过ID和URI更新文件内容的请求
func (h *FileHandler) UpdateFileContentByID(c *gin.Context) {
	// 1. 从路径和查询参数获取ID和URI
	publicID := c.Param("publicID")
	uriStr := c.Query("uri")

	if publicID == "" {
		response.Fail(c, http.StatusBadRequest, "Missing file public ID in path")
		return
	}
	if uriStr == "" {
		response.Fail(c, http.StatusBadRequest, "Missing required 'uri' parameter")
		return
	}

	// 2. 获取用户身份
	claims, err := getClaims(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	// 3. 将所有参数传递给 Service 层
	updatedResult, err := h.fileSvc.UpdateFileContentByIDAndURI(
		c.Request.Context(),
		claims.UserID,
		publicID,
		uriStr,
		c.Request.Body,
	)

	// 4. 处理错误
	if err != nil {
		switch {
		case errors.Is(err, constant.ErrNotFound):
			response.Fail(c, http.StatusNotFound, "File not found")
		case errors.Is(err, constant.ErrForbidden):
			response.Fail(c, http.StatusForbidden, "Access denied")
		case errors.Is(err, constant.ErrConflict):
			// 这个冲突现在有了更明确的含义：文件被移动或重命名了
			response.Fail(c, http.StatusConflict, "File location or name has changed. Please refresh.")
		default:
			log.Printf("[Handler-ERROR] UpdateFileContentByIDAndURI failed for ID '%s': %v", publicID, err)
			response.Fail(c, http.StatusInternalServerError, "Failed to update file content")
		}
		return
	}

	// 5. 发送成功响应
	response.Success(c, updatedResult, "File content updated successfully.")
}
