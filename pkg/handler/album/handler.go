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
// @Summary      获取相册图片列表
// @Description  获取相册图片列表，支持分页、分类筛选、标签筛选、时间筛选和排序
// @Tags         相册管理
// @Security     BearerAuth
// @Produce      json
// @Param        page          query  int     false  "页码"  default(1)
// @Param        pageSize      query  int     false  "每页数量"  default(10)
// @Param        categoryId    query  int     false  "分类ID筛选"
// @Param        tag           query  string  false  "标签筛选"
// @Param        createdAt[0]  query  string  false  "开始时间 (2006/01/02 15:04:05)"
// @Param        createdAt[1]  query  string  false  "结束时间 (2006/01/02 15:04:05)"
// @Param        sort          query  string  false  "排序方式"  default(display_order_asc)
// @Success      200  {object}  response.Response  "获取成功"
// @Failure      500  {object}  response.Response  "获取失败"
// @Router       /albums [get]
func (h *AlbumHandler) GetAlbums(c *gin.Context) {
	// 1. 解析参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	categoryIdStr := c.Query("categoryId")
	tag := c.Query("tag")
	startStr := c.Query("createdAt[0]")
	endStr := c.Query("createdAt[1]")
	sort := c.DefaultQuery("sort", "display_order_asc")

	// 解析 categoryId
	var categoryID *uint
	if categoryIdStr != "" {
		if id, err := strconv.ParseUint(categoryIdStr, 10, 32); err == nil {
			categoryIDVal := uint(id)
			categoryID = &categoryIDVal
		}
	}

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
		Page:       page,
		PageSize:   pageSize,
		CategoryID: categoryID,
		Tag:        tag,
		Start:      startTime,
		End:        endTime,
		Sort:       sort,
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取图片列表失败: "+err.Error())
		return
	}

	// 3. 准备响应 DTO (Data Transfer Object)
	type AlbumResponse struct {
		ID             uint      `json:"id"`
		CategoryID     *uint     `json:"categoryId"`
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
			CategoryID:     album.CategoryID,
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
// @Summary      新增相册图片
// @Description  新增图片到相册
// @Tags         相册管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  object{imageUrl=string,bigImageUrl=string,downloadUrl=string,thumbParam=string,bigParam=string,tags=[]string,width=int,height=int,fileSize=int,format=string,fileHash=string,displayOrder=int}  true  "图片信息"
// @Success      200  {object}  response.Response  "添加成功"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      500  {object}  response.Response  "添加失败"
// @Router       /albums [post]
func (h *AlbumHandler) AddAlbum(c *gin.Context) {
	var req struct {
		CategoryID   *uint    `json:"categoryId"`
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
		CategoryID:   req.CategoryID,
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
// @Summary      删除相册图片
// @Description  根据ID删除相册中的图片
// @Tags         相册管理
// @Security     BearerAuth
// @Param        id  path  int  true  "图片ID"
// @Success      200  {object}  response.Response  "删除成功"
// @Failure      400  {object}  response.Response  "ID非法"
// @Failure      500  {object}  response.Response  "删除失败"
// @Router       /albums/{id} [delete]
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
// @Summary      更新相册图片
// @Description  更新相册中图片的信息
// @Tags         相册管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path  int     true  "图片ID"
// @Param        body  body  object{imageUrl=string,bigImageUrl=string,downloadUrl=string,thumbParam=string,bigParam=string,tags=[]string,displayOrder=int}  true  "图片信息"
// @Success      200  {object}  response.Response  "更新成功"
// @Failure      400  {object}  response.Response  "参数错误或ID非法"
// @Failure      500  {object}  response.Response  "更新失败"
// @Router       /albums/{id} [put]
func (h *AlbumHandler) UpdateAlbum(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID非法")
		return
	}

	var req struct {
		CategoryID   *uint    `json:"categoryId"`
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
		CategoryID:   req.CategoryID,
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

// BatchImportAlbums 处理批量导入图片的请求
// @Summary      批量导入相册图片
// @Description  批量导入图片到相册，后端自动获取图片元数据
// @Tags         相册管理
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  object{urls=[]string,thumbParam=string,bigParam=string,tags=[]string,displayOrder=int}  true  "批量导入信息"
// @Success      200  {object}  response.Response  "导入完成"
// @Failure      400  {object}  response.Response  "参数错误"
// @Failure      500  {object}  response.Response  "导入失败"
// @Router       /albums/batch-import [post]
func (h *AlbumHandler) BatchImportAlbums(c *gin.Context) {
	var req struct {
		CategoryID   *uint    `json:"categoryId"`
		URLs         []string `json:"urls" binding:"required,min=1,max=100"`
		ThumbParam   string   `json:"thumbParam"`
		BigParam     string   `json:"bigParam"`
		Tags         []string `json:"tags"`
		DisplayOrder int      `json:"displayOrder"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 调用Service层进行批量导入
	result, err := h.albumSvc.BatchImportAlbums(c.Request.Context(), album.BatchImportParams{
		CategoryID:   req.CategoryID,
		URLs:         req.URLs,
		ThumbParam:   req.ThumbParam,
		BigParam:     req.BigParam,
		Tags:         req.Tags,
		DisplayOrder: req.DisplayOrder,
	})

	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "批量导入失败: "+err.Error())
		return
	}

	// 构造详细的响应数据
	responseData := gin.H{
		"successCount": result.SuccessCount,
		"failCount":    result.FailCount,
		"skipCount":    result.SkipCount,
		"total":        len(req.URLs),
	}

	// 如果有错误，添加错误详情
	if len(result.Errors) > 0 {
		errors := make([]gin.H, 0, len(result.Errors))
		for _, e := range result.Errors {
			errors = append(errors, gin.H{
				"url":    e.URL,
				"reason": e.Reason,
			})
		}
		responseData["errors"] = errors
	}

	// 如果有重复，添加重复列表
	if len(result.Duplicates) > 0 {
		responseData["duplicates"] = result.Duplicates
	}

	message := fmt.Sprintf("批量导入完成！成功 %d 张，失败 %d 张，跳过 %d 张",
		result.SuccessCount, result.FailCount, result.SkipCount)

	response.Success(c, responseData, message)
}
