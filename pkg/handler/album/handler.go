package album_handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/album"

	"github.com/gin-gonic/gin"
)

// AlbumHandler 封装了相册相关的控制器方法
type AlbumHandler struct {
	albumSvc album.AlbumService
}

// NewAlbumHandler 是 AlbumHandler 的构造函数
func NewAlbumHandler(albumSvc album.AlbumService) *AlbumHandler {
	return &AlbumHandler{
		albumSvc: albumSvc,
	}
}

// GetAlbums 处理获取图片列表的请求
func (h *AlbumHandler) GetAlbums(c *gin.Context) {
	// 1. 解析参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	tag := c.Query("tag")
	startStr := c.Query("createdAt[0]")
	endStr := c.Query("createdAt[1]")
	sort := c.DefaultQuery("sort", "display_order_asc")

	var startTime, endTime *time.Time
	const layout = "2006/01/02 15:04:05"
	if t, err := time.ParseInLocation(layout, startStr, time.Local); err == nil {
		startTime = &t
	}
	if t, err := time.ParseInLocation(layout, endStr, time.Local); err == nil {
		endTime = &t
	}

	// 2. 调用更新后的 Service 方法
	pageResult, err := h.albumSvc.FindAlbums(c.Request.Context(), album.FindAlbumsParams{
		Page:     page,
		PageSize: pageSize,
		Tag:      tag,
		Start:    startTime,
		End:      endTime,
		Sort:     sort,
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取图片列表失败: "+err.Error())
		return
	}

	// 3. 准备响应 DTO (Data Transfer Object)
	type AlbumResponse struct {
		ID             uint      `json:"id"`
		ImageUrl       string    `json:"imageUrl"`
		BigImageUrl    string    `json:"bigImageUrl"`
		DownloadUrl    string    `json:"downloadUrl"`
		ThumbParam     string    `json:"thumbParam"`
		BigParam       string    `json:"bigParam"`
		Tags           string    `json:"tags"`
		ViewCount      int       `json:"viewCount"`
		DownloadCount  int       `json:"downloadCount"`
		FileSize       int64     `json:"fileSize"`
		Format         string    `json:"format"`
		AspectRatio    string    `json:"aspectRatio"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
		Width          int       `json:"width"`
		Height         int       `json:"height"`
		WidthAndHeight string    `json:"widthAndHeight"`
		DisplayOrder   int       `json:"displayOrder"`
	}

	// 从 PageResult 中获取 Items
	responseList := make([]AlbumResponse, 0, len(pageResult.Items))
	for _, album := range pageResult.Items {
		responseList = append(responseList, AlbumResponse{
			ID:             album.ID,
			ImageUrl:       album.ImageUrl,
			BigImageUrl:    album.BigImageUrl,
			DownloadUrl:    album.DownloadUrl,
			ThumbParam:     album.ThumbParam,
			BigParam:       album.BigParam,
			Tags:           album.Tags,
			ViewCount:      album.ViewCount,
			DownloadCount:  album.DownloadCount,
			CreatedAt:      album.CreatedAt,
			UpdatedAt:      album.UpdatedAt,
			FileSize:       album.FileSize,
			Format:         album.Format,
			AspectRatio:    album.AspectRatio,
			Width:          album.Width,
			Height:         album.Height,
			WidthAndHeight: fmt.Sprintf("%dx%d", album.Width, album.Height),
			DisplayOrder:   album.DisplayOrder,
		})
	}

	response.Success(c, gin.H{
		"list":     responseList,
		"total":    pageResult.Total,
		"pageNum":  page,
		"pageSize": pageSize,
	}, "获取图片列表成功")
}

// AddAlbum 处理新增图片的请求
func (h *AlbumHandler) AddAlbum(c *gin.Context) {
	var req struct {
		ImageUrl     string   `json:"imageUrl" binding:"required"`
		BigImageUrl  string   `json:"bigImageUrl"`
		DownloadUrl  string   `json:"downloadUrl"`
		ThumbParam   string   `json:"thumbParam"`
		BigParam     string   `json:"bigParam"`
		Tags         []string `json:"tags"`
		Width        int      `json:"width"`
		Height       int      `json:"height"`
		FileSize     int64    `json:"fileSize"`
		Format       string   `json:"format"`
		FileHash     string   `json:"fileHash" binding:"required"`
		DisplayOrder int      `json:"displayOrder"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	_, err := h.albumSvc.CreateAlbum(c.Request.Context(), album.CreateAlbumParams{
		ImageUrl:     req.ImageUrl,
		BigImageUrl:  req.BigImageUrl,
		DownloadUrl:  req.DownloadUrl,
		ThumbParam:   req.ThumbParam,
		BigParam:     req.BigParam,
		Tags:         req.Tags,
		Width:        req.Width,
		Height:       req.Height,
		FileSize:     req.FileSize,
		Format:       req.Format,
		FileHash:     req.FileHash,
		DisplayOrder: req.DisplayOrder,
	})

	if err != nil {
		// Service 层返回的错误可以直接展示给前端
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, nil, "添加成功")
}

// DeleteAlbum 处理删除图片的请求
func (h *AlbumHandler) DeleteAlbum(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID非法")
		return
	}

	if err := h.albumSvc.DeleteAlbum(c.Request.Context(), uint(id)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}

	response.Success(c, nil, "删除成功")
}

// UpdateAlbum 处理更新图片的请求
func (h *AlbumHandler) UpdateAlbum(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID非法")
		return
	}

	var req struct {
		ImageUrl     string   `json:"imageUrl" binding:"required"`
		BigImageUrl  string   `json:"bigImageUrl"`
		DownloadUrl  string   `json:"downloadUrl"`
		ThumbParam   string   `json:"thumbParam"`
		BigParam     string   `json:"bigParam"`
		Tags         []string `json:"tags"`
		DisplayOrder *int     `json:"displayOrder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	_, err = h.albumSvc.UpdateAlbum(c.Request.Context(), uint(id), album.UpdateAlbumParams{
		ImageUrl:     req.ImageUrl,
		BigImageUrl:  req.BigImageUrl,
		DownloadUrl:  req.DownloadUrl,
		ThumbParam:   req.ThumbParam,
		BigParam:     req.BigParam,
		Tags:         req.Tags,
		DisplayOrder: req.DisplayOrder,
	})

	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	response.Success(c, nil, "更新成功")
}
