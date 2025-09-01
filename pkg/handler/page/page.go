package page

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/page"
)

// Handler 页面处理器
type Handler struct {
	pageService page.Service
}

// NewHandler 创建页面处理器
func NewHandler(pageService page.Service) *Handler {
	return &Handler{
		pageService: pageService,
	}
}

// Create 创建页面
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Title       string `json:"title" binding:"required"`
		Path        string `json:"path" binding:"required"`
		Content     string `json:"content" binding:"required"`
		Description string `json:"description"`
		IsPublished bool   `json:"is_published"`
		Sort        int    `json:"sort"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	options := &model.CreatePageOptions{
		Title:       req.Title,
		Path:        req.Path,
		Content:     req.Content,
		Description: req.Description,
		IsPublished: req.IsPublished,
		Sort:        req.Sort,
	}

	page, err := h.pageService.Create(c.Request.Context(), options)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "创建页面失败")
		return
	}

	response.Success(c, page, "创建页面成功")
}

// GetByID 根据ID获取页面
func (h *Handler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Fail(c, http.StatusBadRequest, "页面ID不能为空")
		return
	}

	page, err := h.pageService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取页面失败")
		return
	}

	response.Success(c, page, "获取页面成功")
}

// GetByPath 根据路径获取页面
func (h *Handler) GetByPath(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		response.Fail(c, http.StatusBadRequest, "页面路径不能为空")
		return
	}

	page, err := h.pageService.GetByPath(c.Request.Context(), path)
	if err != nil {
		// 检查是否是"页面不存在"错误
		if strings.Contains(err.Error(), "页面不存在") {
			response.Fail(c, http.StatusNotFound, "页面不存在")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "获取页面失败")
		return
	}

	response.Success(c, page, "获取页面成功")
}

// List 列出页面
func (h *Handler) List(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")
	search := c.Query("search")
	isPublishedStr := c.Query("is_published")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	var isPublished *bool
	if isPublishedStr != "" {
		val, err := strconv.ParseBool(isPublishedStr)
		if err == nil {
			isPublished = &val
		}
	}

	options := &model.ListPagesOptions{
		Page:        page,
		PageSize:    pageSize,
		Search:      search,
		IsPublished: isPublished,
	}

	pages, total, err := h.pageService.List(c.Request.Context(), options)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取页面列表失败")
		return
	}

	response.Success(c, gin.H{
		"pages": pages,
		"total": total,
		"page":  page,
		"size":  pageSize,
	}, "获取页面列表成功")
}

// Update 更新页面
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Fail(c, http.StatusBadRequest, "页面ID不能为空")
		return
	}

	var req struct {
		Title       *string `json:"title"`
		Path        *string `json:"path"`
		Content     *string `json:"content"`
		Description *string `json:"description"`
		IsPublished *bool   `json:"is_published"`
		Sort        *int    `json:"sort"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	options := &model.UpdatePageOptions{
		Title:       req.Title,
		Path:        req.Path,
		Content:     req.Content,
		Description: req.Description,
		IsPublished: req.IsPublished,
		Sort:        req.Sort,
	}

	page, err := h.pageService.Update(c.Request.Context(), id, options)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "更新页面失败")
		return
	}

	response.Success(c, page, "更新页面成功")
}

// Delete 删除页面
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Fail(c, http.StatusBadRequest, "页面ID不能为空")
		return
	}

	err := h.pageService.Delete(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "删除页面失败")
		return
	}

	response.Success(c, nil, "删除页面成功")
}

// InitializeDefaultPages 初始化默认页面
func (h *Handler) InitializeDefaultPages(c *gin.Context) {
	err := h.pageService.InitializeDefaultPages(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "初始化默认页面失败")
		return
	}

	response.Success(c, nil, "初始化默认页面成功")
}
