package link

import (
	"net/http"
	"strconv"

	"anheyu-app/internal/app/service/link"
	"anheyu-app/internal/domain/model"
	"anheyu-app/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// Handler 负责处理友链相关的 API 请求。
type Handler struct {
	linkSvc link.Service
}

// NewHandler 是 Handler 的构造函数。
func NewHandler(linkSvc link.Service) *Handler {
	return &Handler{linkSvc: linkSvc}
}

// --- 前台公开接口 ---

// GetRandomLinks 处理随机获取友链的请求。
// @Router /api/public/links/random [get]
func (h *Handler) GetRandomLinks(c *gin.Context) {
	// 从查询参数中获取 num，如果不存在或无效，则默认为 0
	num, _ := strconv.Atoi(c.DefaultQuery("num", "0"))

	links, err := h.linkSvc.GetRandomLinks(c.Request.Context(), num)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取随机友链失败: "+err.Error())
		return
	}
	response.Success(c, links, "获取成功")
}

// ApplyLink 处理前台用户申请友链的请求。
// @Router /api/public/links/apply [post]
func (h *Handler) ApplyLink(c *gin.Context) {
	var req model.ApplyLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	_, err := h.linkSvc.ApplyLink(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "申请失败: "+err.Error())
		return
	}
	response.Success(c, nil, "申请已提交，等待审核")
}

// ListPublicLinks 处理前台获取已批准友链列表的请求。
// @Router /api/public/links [get]
func (h *Handler) ListPublicLinks(c *gin.Context) {
	var req model.ListPublicLinksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	result, err := h.linkSvc.ListPublicLinks(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取列表失败: "+err.Error())
		return
	}
	response.Success(c, result, "获取成功")
}

// ListCategories 获取友链分类列表。
// @Router /api/public/link-categories [get]
// @Router /api/links/categories [get]
func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.linkSvc.ListCategories(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取分类列表失败: "+err.Error())
		return
	}
	response.Success(c, categories, "获取成功")
}

// --- 后台管理接口 ---

// ListAllTags 获取所有友链标签。
// @Router /api/links/tags [get]
func (h *Handler) ListAllTags(c *gin.Context) {
	tags, err := h.linkSvc.AdminListAllTags(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取标签列表失败: "+err.Error())
		return
	}
	response.Success(c, tags, "获取成功")
}

// AdminCreateLink 处理后台管理员直接创建友链的请求。
// @Router /api/links [post]
func (h *Handler) AdminCreateLink(c *gin.Context) {
	var req model.AdminCreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}
	link, err := h.linkSvc.AdminCreateLink(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, link, "创建成功")
}

// ListLinks 处理后台管理员获取友链列表的请求。
// @Router /api/links [get]
func (h *Handler) ListLinks(c *gin.Context) {
	var req model.ListLinksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	result, err := h.linkSvc.ListLinks(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取列表失败: "+err.Error())
		return
	}
	response.Success(c, result, "获取成功")
}

// AdminUpdateLink 处理后台管理员更新友链的请求。
// @Router /api/links/:id [put]
func (h *Handler) AdminUpdateLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID 格式无效")
		return
	}
	var req model.AdminUpdateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}
	link, err := h.linkSvc.AdminUpdateLink(c.Request.Context(), id, &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	response.Success(c, link, "更新成功")
}

// AdminDeleteLink 处理后台管理员删除友链的请求。
// @Router /api/links/:id [delete]
func (h *Handler) AdminDeleteLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID 格式无效")
		return
	}
	err = h.linkSvc.AdminDeleteLink(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	response.Success(c, nil, "删除成功")
}

// ReviewLink 处理后台管理员审核友链的请求。
// @Router /api/links/:id/review [put]
func (h *Handler) ReviewLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID 格式无效")
		return
	}

	var req model.ReviewLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	if err := h.linkSvc.ReviewLink(c.Request.Context(), id, &req); err != nil {
		response.Fail(c, http.StatusInternalServerError, "审核操作失败: "+err.Error())
		return
	}
	response.Success(c, nil, "审核状态更新成功")
}

// CreateCategory 处理后台管理员创建友链分类的请求。
// @Router /api/links/categories [post]
func (h *Handler) CreateCategory(c *gin.Context) {
	var req model.CreateLinkCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	cat, err := h.linkSvc.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "创建分类失败: "+err.Error())
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, cat, "创建成功")
}

// CreateTag 处理后台管理员创建友链标签的请求。
// @Router /api/links/tags [post]
func (h *Handler) CreateTag(c *gin.Context) {
	var req model.CreateLinkTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	tag, err := h.linkSvc.CreateTag(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "创建标签失败: "+err.Error())
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, tag, "创建成功")
}

// UpdateCategory 处理后台管理员更新友链分类的请求。
// @Router /api/links/categories/:id [put]
func (h *Handler) UpdateCategory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID 格式无效")
		return
	}

	var req model.UpdateLinkCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	cat, err := h.linkSvc.UpdateCategory(c.Request.Context(), id, &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "更新分类失败: "+err.Error())
		return
	}
	response.Success(c, cat, "更新成功")
}

// UpdateTag 处理后台管理员更新友链标签的请求。
// @Router /api/links/tags/:id [put]
func (h *Handler) UpdateTag(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID 格式无效")
		return
	}

	var req model.UpdateLinkTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数无效: "+err.Error())
		return
	}

	tag, err := h.linkSvc.UpdateTag(c.Request.Context(), id, &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "更新标签失败: "+err.Error())
		return
	}
	response.Success(c, tag, "更新成功")
}
