/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-06-15 11:30:55
 * @LastEditTime: 2025-08-17 04:07:04
 * @LastEditors: 安知鱼
 */
package storage_policy_handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/anzhiyu-c/anheyu-app/internal/app/service/volume"
	"github.com/anzhiyu-c/anheyu-app/internal/constant"
	"github.com/anzhiyu-c/anheyu-app/internal/domain/model"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/auth"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// CreatePolicyRequest 定义了创建策略时请求体的结构
type CreatePolicyRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Type        string                 `json:"type" binding:"required"`
	Server      string                 `json:"server"`
	Flag        string                 `json:"flag"`
	BucketName  string                 `json:"bucket_name"`
	IsPrivate   bool                   `json:"is_private"`
	AccessKey   string                 `json:"access_key"`
	SecretKey   string                 `json:"secret_key"`
	MaxSize     int64                  `json:"max_size"`
	BasePath    string                 `json:"base_path"`
	VirtualPath string                 `json:"virtual_path"`
	Settings    map[string]interface{} `json:"settings"`
}

// UpdatePolicyRequest 定义了更新策略时请求体的结构
type UpdatePolicyRequest CreatePolicyRequest

// PaginationRequest 定义了分页查询的请求参数
type PaginationRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

// StoragePolicyResponseItem 定义了单个存储策略在响应体中的结构
type StoragePolicyResponseItem struct {
	ID          string                 `json:"id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Flag        string                 `json:"flag,omitempty"`
	Server      string                 `json:"server,omitempty"`
	BucketName  string                 `json:"bucket_name,omitempty"`
	IsPrivate   bool                   `json:"is_private"`
	AccessKey   string                 `json:"access_key,omitempty"`
	SecretKey   string                 `json:"secret_key,omitempty"`
	MaxSize     int64                  `json:"max_size"`
	BasePath    string                 `json:"base_path,omitempty"`
	VirtualPath string                 `json:"virtual_path,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
}

// PolicyListResponse 定义了存储策略列表的响应结构
type PolicyListResponse struct {
	List  []*StoragePolicyResponseItem `json:"list"`
	Total int64                        `json:"total"`
}

// StoragePolicyHandler 负责处理所有与存储策略相关的HTTP请求
type StoragePolicyHandler struct {
	svc volume.IStoragePolicyService
}

// NewStoragePolicyHandler 是 StoragePolicyHandler 的构造函数
func NewStoragePolicyHandler(svc volume.IStoragePolicyService) *StoragePolicyHandler {
	return &StoragePolicyHandler{svc: svc}
}

// Create 处理创建存储策略的请求
func (h *StoragePolicyHandler) Create(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	claims, exists := c.Get(auth.ClaimsKey)
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "无法获取用户信息")
		return
	}

	authClaims, ok := claims.(*auth.CustomClaims)
	if !ok {
		response.Fail(c, http.StatusInternalServerError, "用户信息格式不正确")
		return
	}

	ownerID, _, err := idgen.DecodePublicID(authClaims.UserID)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "无效的用户凭证")
		return
	}
	policy := &model.StoragePolicy{
		Name:        req.Name,
		Type:        constant.StoragePolicyType(req.Type),
		Flag:        req.Flag,
		Server:      req.Server,
		BucketName:  req.BucketName,
		IsPrivate:   req.IsPrivate,
		AccessKey:   req.AccessKey,
		SecretKey:   req.SecretKey,
		MaxSize:     req.MaxSize,
		BasePath:    req.BasePath,
		VirtualPath: req.VirtualPath,
		Settings:    req.Settings,
	}

	if err := h.svc.CreatePolicy(c.Request.Context(), ownerID, policy); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	responseItem, err := h.buildStoragePolicyResponseItem(policy)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "构建响应失败: "+err.Error())
		return
	}
	response.Success(c, responseItem, "创建成功")
}

// Get 处理获取存储策略的请求
func (h *StoragePolicyHandler) Get(c *gin.Context) {
	publicID := c.Param("id")
	if publicID == "" {
		response.Fail(c, http.StatusBadRequest, "ID 不能为空")
		return
	}

	policy, err := h.svc.GetPolicyByID(c.Request.Context(), publicID)
	if err != nil {
		if errors.Is(err, constant.ErrPolicyNotFound) {
			response.Fail(c, http.StatusNotFound, "策略未找到")
			return
		}
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	responseItem, err := h.buildStoragePolicyResponseItem(policy)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "构建响应失败: "+err.Error())
		return
	}
	response.Success(c, responseItem, "获取成功")
}

// List 处理获取存储策略列表的请求
func (h *StoragePolicyHandler) List(c *gin.Context) {
	const (
		defaultPageSize = 10
		maxPageSize     = 100
	)

	var req PaginationRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "分页参数错误")
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	if req.PageSize > maxPageSize {
		req.PageSize = maxPageSize
	}

	policies, total, err := h.svc.ListPolicies(c.Request.Context(), req.Page, req.PageSize)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	responseList := make([]*StoragePolicyResponseItem, len(policies))
	for i, policy := range policies {
		item, buildErr := h.buildStoragePolicyResponseItem(policy)
		if buildErr != nil {
			response.Fail(c, http.StatusInternalServerError, "构建策略列表项失败: "+buildErr.Error())
			return
		}
		responseList[i] = item
	}

	response.Success(c, PolicyListResponse{
		List:  responseList,
		Total: total,
	}, "获取列表成功")
}

// Delete 处理删除存储策略的请求
func (h *StoragePolicyHandler) Delete(c *gin.Context) {
	publicID := c.Param("id")
	if publicID == "" {
		response.Fail(c, http.StatusBadRequest, "ID 不能为空")
		return
	}

	if err := h.svc.DeletePolicy(c.Request.Context(), publicID); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, nil, "删除成功")
}

// Update 处理更新存储策略的请求
func (h *StoragePolicyHandler) Update(c *gin.Context) {
	publicID := c.Param("id")
	if publicID == "" {
		response.Fail(c, http.StatusBadRequest, "ID 不能为空")
		return
	}

	internalID, entityType, err := idgen.DecodePublicID(publicID)
	if err != nil || entityType != idgen.EntityTypeStoragePolicy {
		response.Fail(c, http.StatusBadRequest, "无效的ID格式")
		return
	}

	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	policy := &model.StoragePolicy{
		ID:          internalID,
		Name:        req.Name,
		Flag:        req.Flag,
		Type:        constant.StoragePolicyType(req.Type),
		Server:      req.Server,
		BucketName:  req.BucketName,
		IsPrivate:   req.IsPrivate,
		AccessKey:   req.AccessKey,
		SecretKey:   req.SecretKey,
		MaxSize:     req.MaxSize,
		BasePath:    req.BasePath,
		VirtualPath: req.VirtualPath,
		Settings:    req.Settings,
	}

	if err := h.svc.UpdatePolicy(c.Request.Context(), policy); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	updatedPolicy, err := h.svc.GetPolicyByID(c.Request.Context(), publicID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取更新后的策略信息失败: "+err.Error())
		return
	}

	responseItem, err := h.buildStoragePolicyResponseItem(updatedPolicy)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "构建响应失败: "+err.Error())
		return
	}

	response.Success(c, responseItem, "更新成功")
}

// ConnectOneDrive 获取 OneDrive 授权链接
func (h *StoragePolicyHandler) ConnectOneDrive(c *gin.Context) {
	publicID := c.Param("id")
	authURL, err := h.svc.GenerateAuthURL(c.Request.Context(), publicID)
	if err != nil {
		if errors.Is(err, constant.ErrPolicyNotFound) {
			response.Fail(c, http.StatusNotFound, "策略未找到")
			return
		}
		if errors.Is(err, constant.ErrPolicyNotSupportAuth) {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Fail(c, http.StatusInternalServerError, "生成授权链接失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"url": authURL}, "获取成功")
}

// AuthorizeRequest 是接收前端 code 和 state 的请求体
type AuthorizeRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

// AuthorizeOneDrive 完成 OneDrive 授权
func (h *StoragePolicyHandler) AuthorizeOneDrive(c *gin.Context) {
	var req AuthorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	err := h.svc.FinalizeAuth(c.Request.Context(), req.Code, req.State)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "授权处理失败: "+err.Error())
		return
	}
	response.Success(c, nil, "授权成功")
}

// buildStoragePolicyResponseItem 辅助函数，将 model.StoragePolicy 转换为 StoragePolicyResponseItem
func (h *StoragePolicyHandler) buildStoragePolicyResponseItem(policy *model.StoragePolicy) (*StoragePolicyResponseItem, error) {
	if policy == nil {
		return nil, nil
	}

	publicID, err := idgen.GeneratePublicID(policy.ID, idgen.EntityTypeStoragePolicy)
	if err != nil {
		return nil, fmt.Errorf("生成存储策略公共ID失败: %w", err)
	}

	return &StoragePolicyResponseItem{
		ID:          publicID,
		CreatedAt:   policy.CreatedAt,
		UpdatedAt:   policy.UpdatedAt,
		Name:        policy.Name,
		Flag:        policy.Flag,
		Type:        string(policy.Type),
		Server:      policy.Server,
		BucketName:  policy.BucketName,
		IsPrivate:   policy.IsPrivate,
		AccessKey:   policy.AccessKey,
		SecretKey:   policy.SecretKey,
		MaxSize:     policy.MaxSize,
		BasePath:    policy.BasePath,
		VirtualPath: policy.VirtualPath,
		Settings:    policy.Settings,
	}, nil
}
