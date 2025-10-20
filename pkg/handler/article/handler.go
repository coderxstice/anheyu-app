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
	"github.com/anzhiyu-c/anheyu-app/pkg/util"

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
// @Summary      上传文章图片
// @Description  上传文章中使用的图片文件
// @Tags         文章管理
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "图片文件"
// @Success      200   {object}  response.Response{data=object{url=string,file_id=string}}  "上传成功"
// @Failure      400   {object}  response.Response  "无效的文件上传请求"
// @Failure      401   {object}  response.Response  "未授权"
// @Failure      500   {object}  response.Response  "图片上传失败"
// @Router       /articles/upload [post]
func (h *Handler) UploadImage(c *gin.Context) {
	log.Printf("[Handler.UploadImage] 开始处理图片上传请求")
	log.Printf("[Handler.UploadImage] 请求方法: %s, 路径: %s", c.Request.Method, c.Request.URL.Path)

	// 1. 从请求中获取文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Printf("[Handler.UploadImage] 获取上传文件失败: %v", err)
		response.Fail(c, http.StatusBadRequest, "无效的文件上传请求")
		return
	}
	log.Printf("[Handler.UploadImage] 接收到文件: %s, 大小: %d bytes", fileHeader.Filename, fileHeader.Size)

	// 2. 打开文件流
	fileReader, err := fileHeader.Open()
	if err != nil {
		log.Printf("[Handler.UploadImage] 打开文件流失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "无法处理上传的文件")
		return
	}
	defer fileReader.Close()

	// 3. 获取用户认证信息
	log.Printf("[Handler.UploadImage] 开始获取用户认证信息")
	claims, err := getClaims(c)
	if err != nil {
		log.Printf("[Handler.UploadImage] 认证失败: %v", err)
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	log.Printf("[Handler.UploadImage] 用户认证成功, UserID: %s", claims.UserID)

	ownerID, _, err := idgen.DecodePublicID(claims.UserID)
	if err != nil {
		log.Printf("[Handler.UploadImage] 解析用户ID失败: %v", err)
		response.Fail(c, http.StatusUnauthorized, "无效的用户凭证")
		return
	}
	log.Printf("[Handler.UploadImage] 解析用户ID成功, ownerID: %d", ownerID)

	// 4. 调用Service层处理业务逻辑
	log.Printf("[Handler.UploadImage] 开始调用Service层处理图片上传")
	directLinkURL, publicFileID, err := h.svc.UploadArticleImage(c.Request.Context(), ownerID, fileReader, fileHeader.Filename)
	if err != nil {
		log.Printf("[Handler.UploadImage] Service处理失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "图片上传失败")
		return
	}
	log.Printf("[Handler.UploadImage] 图片上传成功, URL: %s, 文件ID: %s", directLinkURL, publicFileID)

	// 5. 成功响应，返回直链URL和文件公共ID
	response.Success(c, gin.H{
		"url":     directLinkURL,
		"file_id": publicFileID,
	}, "图片上传成功")
}

// ListPublic
// @Summary      获取前台文章列表
// @Description  获取公开的、分页的文章列表。结果按置顶优先级和创建时间排序。
// @Tags         公开文章
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
// @Tags         公开文章
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
// @Tags         公开文章
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
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        article body model.CreateArticleRequest true "创建文章的请求体"
// @Success      200 {object} response.Response{data=model.ArticleResponse} "成功响应"
// @Failure      400 {object} response.Response "请求参数错误"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /articles [post]
func (h *Handler) Create(c *gin.Context) {
	log.Printf("[Handler.Create] ========== 收到创建文章请求 ==========")
	var req model.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler.Create] ❌ 请求参数绑定失败: %v", err)
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	log.Printf("[Handler.Create] 文章标题: %s", req.Title)
	log.Printf("[Handler.Create] CustomPublishedAt: %v", req.CustomPublishedAt)
	if req.CustomPublishedAt != nil {
		log.Printf("[Handler.Create] CustomPublishedAt 值: %s", *req.CustomPublishedAt)
	}
	log.Printf("[Handler.Create] CustomUpdatedAt: %v", req.CustomUpdatedAt)
	if req.CustomUpdatedAt != nil {
		log.Printf("[Handler.Create] CustomUpdatedAt 值: %s", *req.CustomUpdatedAt)
	}

	// 使用改进的IP获取方法，优先检查代理头部
	clientIP := util.GetRealClientIP(c)

	// 调用 Service 时传递 IP 地址
	log.Printf("[Handler.Create] 调用 Service.Create...")
	article, err := h.svc.Create(c.Request.Context(), &req, clientIP)
	if err != nil {
		log.Printf("[Handler.Create] ❌ Service.Create 失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "创建文章失败: "+err.Error())
		return
	}

	log.Printf("[Handler.Create] ✅ 文章创建成功")
	response.Success(c, article, "创建成功")
}

// ListHome
// @Summary      获取首页推荐文章
// @Description  获取配置为在首页卡片中展示的文章列表 (按 home_sort 排序, 最多6篇)
// @Tags         公开文章
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
// @Tags         公开文章
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
// @Tags         文章管理
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
// @Tags         文章管理
// @Accept       json
// @Produce      json
// @Param        id path string true "文章的公共ID"
// @Param        article body model.UpdateArticleRequest true "更新文章的请求体"
// @Success      200 {object} response.Response{data=model.ArticleResponse} "成功响应"
// @Failure      400 {object} response.Response "请求参数错误"
// @Failure      500 {object} response.Response "服务器内部错误"
// @Router       /articles/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	log.Printf("[Handler.Update] ========== 收到更新文章请求 ==========")
	id := c.Param("id")
	if id == "" {
		response.Fail(c, http.StatusBadRequest, "文章ID不能为空")
		return
	}
	log.Printf("[Handler.Update] 文章ID: %s", id)

	var req model.UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler.Update] ❌ 请求参数绑定失败: %v", err)
		response.Fail(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	log.Printf("[Handler.Update] CustomUpdatedAt: %v", req.CustomUpdatedAt)
	if req.CustomUpdatedAt != nil {
		log.Printf("[Handler.Update] CustomUpdatedAt 值: %s", *req.CustomUpdatedAt)
	}

	// 使用改进的IP获取方法，优先检查代理头部
	clientIP := util.GetRealClientIP(c)
	log.Printf("[Handler.Update] 准备更新文章，获取到的真实 IP 是: %s", clientIP)

	// 将 clientIP 传递给 Service 层
	log.Printf("[Handler.Update] 调用 Service.Update...")
	article, err := h.svc.Update(c.Request.Context(), id, &req, clientIP)
	if err != nil {
		log.Printf("[Handler.Update] ❌ Service.Update 失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "更新文章失败: "+err.Error())
		return
	}

	log.Printf("[Handler.Update] ✅ 文章更新成功")
	response.Success(c, article, "更新成功")
}

// Delete
// @Summary      删除文章
// @Description  根据文章的公共ID删除文章 (软删除)
// @Tags         文章管理
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
// @Tags         文章管理
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

// GetPrimaryColor 处理获取图片主色调的请求。
// @Summary      获取图片主色调
// @Description  根据图片URL获取主色调
// @Tags         文章管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  object{image_url=string}  true  "图片URL"
// @Success      200   {object}  response.Response{data=object{primary_color=string}}  "获取成功"
// @Failure      400   {object}  response.Response  "无效的请求参数"
// @Failure      401   {object}  response.Response  "未授权"
// @Failure      500   {object}  response.Response  "获取主色调失败"
// @Router       /articles/primary-color [post]
func (h *Handler) GetPrimaryColor(c *gin.Context) {
	log.Printf("[Handler.GetPrimaryColor] 开始处理获取主色调请求")

	// 1. 解析请求参数
	var req struct {
		ImageURL string `json:"image_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler.GetPrimaryColor] 参数解析失败: %v", err)
		response.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	log.Printf("[Handler.GetPrimaryColor] 图片URL: %s", req.ImageURL)

	// 2. 验证用户登录状态
	_, err := getClaims(c)
	if err != nil {
		log.Printf("[Handler.GetPrimaryColor] 认证失败: %v", err)
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	// 3. 调用Service层获取主色调
	log.Printf("[Handler.GetPrimaryColor] 开始调用Service层获取主色调...")
	primaryColor, err := h.svc.GetPrimaryColorFromURL(c.Request.Context(), req.ImageURL)
	if err != nil {
		log.Printf("[Handler.GetPrimaryColor] 获取主色调失败: %v", err)
		response.Fail(c, http.StatusInternalServerError, "获取主色调失败: "+err.Error())
		return
	}

	log.Printf("[Handler.GetPrimaryColor] 成功获取主色调: %s", primaryColor)

	// 4. 成功响应
	response.Success(c, gin.H{
		"primary_color": primaryColor,
	}, "获取主色调成功")
}
