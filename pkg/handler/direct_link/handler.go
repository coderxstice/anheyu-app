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

	encodedFileName := url.QueryEscape(filename)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedFileName))
	c.Header("Content-Type", file.PrimaryEntity.MimeType.String)
	c.Header("Content-Length", fmt.Sprintf("%d", file.Size))

	throttledWriter := utils.NewThrottledWriter(c.Writer, speedLimit, c.Request.Context())

	err = provider.Stream(c.Request.Context(), policy, file.PrimaryEntity.Source.String, throttledWriter)
	if err != nil {
		log.Printf("下载文件 [FileID: %d] 时流式传输失败: %v", file.ID, err)
	}
}
