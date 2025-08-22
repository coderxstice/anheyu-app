package article

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/auth"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"

	articleSvc "github.com/anzhiyu-c/anheyu-app/pkg/service/article"

	"github.com/gin-gonic/gin"
)

// Handler 封装了所有与文章相关的 HTTP 处理器。
type Handler struct {
	svc articleSvc.Service
}

// NewHandler 是 Handler 的构造函数。
func NewHandler(svc articleSvc.Service) *Handler {
	return &Handler{svc: svc}
}

// UploadImage 处理文章图片的上传请求。
func (h *Handler) UploadImage(c *gin.Context) {
	// 1. 从请求中获取文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Printf("[Handler.UploadImage] 获取上传文件失败: %v", err)
		response.Fail(c, http.StatusBadRequest, "无效的文件上传请求")
		return
	}

	// 2. 打开文件流
	fileReader, err := fileHeader.Open()
	if err != nil {
		log.Printf("[Handler.UploadImage] 打开文件流失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "无法处理上传的文件")
		return
	}
	defer fileReader.Close()

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

	// 4. 调用Service层处理业务逻辑
	directLinkURL, err := h.svc.UploadArticleImage(c.Request.Context(), ownerID, fileReader, fileHeader.Filename)
	if err != nil {
		log.Printf("[Handler.UploadImage] Service处理失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "图片上传失败")
		return
	}

	// 5. 成功响应，返回直链URL
	response.Success(c, gin.H{
		"url": directLinkURL,
	}, "图片上传成功")
}

// ListPublic
// @Summary      获取前台文章列表
// @Description  获取公开的、分页的文章列表。结果按置顶优先级和创建时间排序。
// @Tags         Article Public
// @Produce      json
// @Param        page query int false "页码" default(1)
// @Param        pageSize query int false "每页数量" default(10)
// @Param        category query string false "分类名称"
// @Param        tag query string false "标签名称"
// @Param        year query int false "年份"
// @Param        month query int false "月份"
// @Success      200 {object} response.Response{data=model.ArticleListResponse} "成功响应"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /public/articles [get]
func (h *Handler) ListPublic(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))

	options := &model.ListPublicArticlesOptions{
		Page:         page,
		PageSize:     pageSize,
		CategoryName: c.Query("category"),
		TagName:      c.Query("tag"),
		Year:         year,
		Month:        month,
	}

	result, err := h.svc.ListPublic(c.Request.Context(), options)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取文章列表失败: "+err.Error())
		return
	}

	response.Success(c, result, "获取列表成功")
}

// ListArchives
// @Summary      获取文章归档摘要
// @Description  获取按年月分组的文章统计信息，用于侧边栏展示。
// @Tags         Article Public
// @Produce      json
// @Success      200 {object} response.Response{data=model.ArchiveSummaryResponse} "成功响应"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /public/articles/archives [get]
func (h *Handler) ListArchives(c *gin.Context) {
	archives, err := h.svc.ListArchives(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取归档列表失败: "+err.Error())
		return
	}
	response.Success(c, archives, "获取归档列表成功")
}

// GetRandom
// @Summary      随机获取一篇文章
// @Description  随机获取一篇已发布的文章的详细信息，用于“随便看看”等功能。
// @Tags         Article Public
// @Produce      json
// @Success      200 {object} response.Response{data=model.ArticleResponse} "成功响应"
// @Failure      404 {object} response.Response "没有找到已发布的文章"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /public/articles/random [get]
func (h *Handler) GetRandom(c *gin.Context) {
	article, err := h.svc.GetRandom(c.Request.Context())
	if err != nil {
		// 专门处理 "未找到" 的情况
		if ent.IsNotFound(err) {
			response.Fail(c, http.StatusNotFound, "没有找到已发布的文章")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "获取随机文章失败: "+err.Error())
		return
	}

	response.Success(c, article, "获取成功")
}

// Create
// @Summary      创建新文章
// @Description  根据提供的请求体创建一个新文章。总字数、阅读时长和IP属地由后端自动计算。
// @Tags         Article
// @Accept       json
// @Produce      json
// @Param        article body model.CreateArticleRequest true "创建文章的请求体"
// @Success      200 {object} response.Response{data=model.ArticleResponse} "成功响应"
// @Failure      400 {object} response.Response "请求参数错误"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /articles [post]
func (h *Handler) Create(c *gin.Context) {
	var req model.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	// 从 Gin 的 Context 中获取客户端IP地址
	clientIP := c.ClientIP()

	// 调用 Service 时传递 IP 地址
	article, err := h.svc.Create(c.Request.Context(), &req, clientIP)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "创建文章失败: "+err.Error())
		return
	}

	response.Success(c, article, "创建成功")
}

// ListHome
// @Summary      获取首页推荐文章
// @Description  获取配置为在首页卡片中展示的文章列表 (按 home_sort 排序, 最多6篇)
// @Tags         Article Public
// @Produce      json
// @Success      200 {object} response.Response{data=[]model.ArticleResponse} "成功响应"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /public/articles/home [get]
func (h *Handler) ListHome(c *gin.Context) {
	articles, err := h.svc.ListHome(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取首页文章列表失败: "+err.Error())
		return
	}
	response.Success(c, articles, "获取列表成功")
}

// GetPublic
// @Summary      获取单篇公开文章及其上下文
// @Description  根据文章的公共ID或Abbrlink获取详细信息，同时返回上一篇、下一篇和相关文章。
// @Tags         Article Public
// @Produce      json
// @Param        id path string true "文章的公共ID或Abbrlink"
// @Success      200 {object} response.Response{data=model.ArticleDetailResponse} "成功响应"
// @Failure      404 {object} response.Response "文章未找到"
// @Router       /public/articles/{id} [get]
func (h *Handler) GetPublic(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Fail(c, http.StatusBadRequest, "文章ID或Abbrlink不能为空")
		return
	}

	articleResponse, err := h.svc.GetPublicBySlugOrID(c.Request.Context(), id)
	if err != nil {
		if ent.IsNotFound(err) {
			response.Fail(c, http.StatusNotFound, "文章未找到")
		} else {
			response.Fail(c, http.StatusInternalServerError, "获取文章失败: "+err.Error())
		}
		return
	}

	response.Success(c, articleResponse, "获取成功")
}

// Get
// @Summary      获取单篇文章
// @Description  根据文章的公共ID获取详细信息
// @Tags         Article
// @Produce      json
// @Param        id path string true "文章的公共ID"
// @Success      200 {object} response.Response{data=model.ArticleResponse} "成功响应"
// @Failure      400 {object} response.Response "文章ID不能为空"
// @Failure      404 {object} response.Response "文章未找到"
// @Router       /articles/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Fail(c, http.StatusBadRequest, "文章ID不能为空")
		return
	}

	article, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "文章未找到")
		return
	}

	response.Success(c, article, "获取成功")
}

// Update
// @Summary      更新文章
// @Description  根据文章ID和请求体更新文章信息。如果内容更新，总字数和阅读时长会自动重新计算。如果IP属地留空，则由后端自动获取。
// @Tags         Article
// @Accept       json
// @Produce      json
// @Param        id path string true "文章的公共ID"
// @Param        article body model.UpdateArticleRequest true "更新文章的请求体"
// @Success      200 {object} response.Response{data=model.ArticleResponse} "成功响应"
// @Failure      400 {object} response.Response "请求参数错误"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /articles/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Fail(c, http.StatusBadRequest, "文章ID不能为空")
		return
	}

	var req model.UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	// 获取客户端 IP 地址
	clientIP := c.ClientIP()
	log.Printf("[DEBUG] 准备更新文章，从 Gin Context 获取到的原始 IP 是: %s", clientIP)

	// 将 clientIP 传递给 Service 层
	article, err := h.svc.Update(c.Request.Context(), id, &req, clientIP)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "更新文章失败: "+err.Error())
		return
	}

	response.Success(c, article, "更新成功")
}

// Delete
// @Summary      删除文章
// @Description  根据文章的公共ID删除文章 (软删除)
// @Tags         Article
// @Produce      json
// @Param        id path string true "文章的公共ID"
// @Success      200 {object} response.Response "成功响应"
// @Failure      400 {object} response.Response "文章ID不能为空"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /articles/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Fail(c, http.StatusBadRequest, "文章ID不能为空")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, http.StatusInternalServerError, "删除文章失败: "+err.Error())
		return
	}

	response.Success(c, nil, "删除成功")
}

// List
// @Summary      获取文章列表
// @Description  根据查询参数获取分页的文章列表
// @Tags         Article
// @Produce      json
// @Param        page query int false "页码" default(1)
// @Param        pageSize query int false "每页数量" default(10)
// @Param        query query string false "搜索关键词 (标题或摘要)"
// @Param        status query string false "文章状态 (DRAFT, PUBLISHED, ARCHIVED)" Enums(DRAFT, PUBLISHED, ARCHIVED)
// @Success      200 {object} response.Response{data=model.ArticleListResponse} "成功响应"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /articles [get]
func (h *Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	options := &model.ListArticlesOptions{
		Page:     page,
		PageSize: pageSize,
		Query:    c.Query("query"),
		Status:   c.Query("status"),
	}

	result, err := h.svc.List(c.Request.Context(), options)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取文章列表失败: "+err.Error())
		return
	}

	response.Success(c, result, "获取列表成功")
}

// getClaims 从 gin.Context 中安全地提取 JWT Claims
func getClaims(c *gin.Context) (*auth.CustomClaims, error) {
	claimsValue, exists := c.Get(auth.ClaimsKey)
	if !exists {
		return nil, errors.New("无法获取用户信息，请确认是否已登录")
	}
	claims, ok := claimsValue.(*auth.CustomClaims)
	if !ok {
		return nil, errors.New("用户信息格式不正确")
	}
	return claims, nil
}
