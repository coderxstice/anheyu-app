package direct_link

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/anzhiyu-c/anheyu-app/internal/infra/storage"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/auth"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/utils"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/direct_link"
)

// DirectLinkHandler 负责处理直链相关的HTTP请求。
type DirectLinkHandler struct {
	// ✅ 修正 #1: 将这里的字段类型从 *direct_link.Service (结构体指针)
	// 修改为 direct_link.Service (接口)。
	svc              direct_link.Service
	storageProviders map[constant.StoragePolicyType]storage.IStorageProvider
}

// NewDirectLinkHandler 是 DirectLinkHandler 的构造函数。
func NewDirectLinkHandler(
	svc direct_link.Service,
	providers map[constant.StoragePolicyType]storage.IStorageProvider,
) *DirectLinkHandler {
	return &DirectLinkHandler{
		svc:              svc,
		storageProviders: providers,
	}
}

// CreateDirectLinksRequest 定义了创建多个直链的请求体。
type CreateDirectLinksRequest struct {
	FileIDs []string `json:"file_ids" binding:"required,min=1"`
}

// DirectLinkResponseItem 定义了响应体中数组元素的结构
type DirectLinkResponseItem struct {
	Link    string `json:"link"`
	FileURL string `json:"file_url"`
}

// GetOrCreateDirectLinks 获取或创建文件直链
// @Summary      获取或创建文件直链
// @Description  为一个或多个文件生成公开的直接下载链接
// @Tags         直链管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  CreateDirectLinksRequest  true  "文件ID列表"
// @Success      200  {object}  response.Response{data=[]DirectLinkResponseItem}  "获取成功"
// @Failure      400  {object}  response.Response  "请求参数无效"
// @Failure      500  {object}  response.Response  "获取失败"
// @Router       /direct-links [post]
func (h *DirectLinkHandler) GetOrCreateDirectLinks(c *gin.Context) {
	var req CreateDirectLinksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}

	claimsValue, _ := c.Get(auth.ClaimsKey)
	claims := claimsValue.(*auth.CustomClaims)

	userGroupID, _, _ := idgen.DecodePublicID(claims.UserGroupID)

	dbFileIDs := make([]uint, 0, len(req.FileIDs))
	publicToDBIDMap := make(map[string]uint)
	for _, pid := range req.FileIDs {
		dbID, entityType, err := idgen.DecodePublicID(pid)
		if err == nil && entityType == idgen.EntityTypeFile {
			dbFileIDs = append(dbFileIDs, dbID)
			publicToDBIDMap[pid] = dbID
		}
	}

	if len(dbFileIDs) == 0 {
		response.Fail(c, http.StatusBadRequest, "未提供任何有效的文件ID")
		return
	}

	resultsMap, err := h.svc.GetOrCreateDirectLinks(c.Request.Context(), userGroupID, dbFileIDs)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	finalResult := make([]DirectLinkResponseItem, 0, len(resultsMap))
	for _, publicID := range req.FileIDs {
		dbID := publicToDBIDMap[publicID]
		if result, ok := resultsMap[dbID]; ok {
			finalResult = append(finalResult, DirectLinkResponseItem{
				Link:    result.URL,
				FileURL: result.VirtualURI,
			})
		}
	}

	response.Success(c, finalResult, "直链获取成功")
}

// HandleDirectDownload 处理公开的直链下载请求。
// @Summary      直链下载
// @Description  通过直链ID下载文件（无需认证）
// @Tags         直链管理
// @Produce      octet-stream
// @Param        publicID  path  string  true   "直链公共ID"
// @Param        filename  path  string  false  "文件名（可选）"
// @Success      200  {file}    file  "文件内容"
// @Success      302  {string}  string  "重定向到云存储下载链接"
// @Failure      404  {object}  response.Response  "直链未找到"
// @Failure      500  {object}  response.Response  "下载失败"
// @Router       /f/{publicID}/{filename} [get]
func (h *DirectLinkHandler) HandleDirectDownload(c *gin.Context) {
	publicID := c.Param("publicID")

	file, filename, policy, speedLimit, err := h.svc.PrepareDownload(c.Request.Context(), publicID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	provider, ok := h.storageProviders[policy.Type]
	if !ok {
		log.Printf("错误：找不到类型为 '%s' 的存储提供者", policy.Type)
		response.Fail(c, http.StatusInternalServerError, "存储提供者不可用")
		return
	}

	if policy.Type == constant.PolicyTypeLocal {
		// 本地存储：直接流式传输
		encodedFileName := url.QueryEscape(filename)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedFileName))
		c.Header("Content-Type", file.PrimaryEntity.MimeType.String)
		c.Header("Content-Length", fmt.Sprintf("%d", file.Size))

		throttledWriter := utils.NewThrottledWriter(c.Writer, speedLimit, c.Request.Context())

		err = provider.Stream(c.Request.Context(), policy, file.PrimaryEntity.Source.String, throttledWriter)
		if err != nil {
			log.Printf("下载文件 [FileID: %d] 时流式传输失败: %v", file.ID, err)
		}
	} else {
		// 云存储：重定向到直接下载链接
		options := storage.DownloadURLOptions{ExpiresIn: 3600}
		downloadURL, err := provider.GetDownloadURL(c.Request.Context(), policy, file.PrimaryEntity.Source.String, options)
		if err != nil {
			log.Printf("获取云存储下载链接失败 [FileID: %d]: %v", file.ID, err)
			response.Fail(c, http.StatusInternalServerError, "获取下载链接失败")
			return
		}

		// 302重定向到云存储的直接下载链接
		c.Redirect(http.StatusFound, downloadURL)
	}
}
