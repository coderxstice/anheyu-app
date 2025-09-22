/*
 * @Description: 音乐处理器 - 提供音乐相关的RESTful API端点
 * @Author: 安知鱼
 * @Date: 2025-09-22 15:30:00
 */
package music_handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/music"
)

// MusicHandler 音乐处理器
type MusicHandler struct {
	musicSvc music.MusicService
}

// NewMusicHandler 创建新的音乐处理器
func NewMusicHandler(musicSvc music.MusicService) *MusicHandler {
	return &MusicHandler{
		musicSvc: musicSvc,
	}
}

// GetPlaylist 获取播放列表
// @Summary 获取音乐播放列表
// @Description 获取配置的音乐播放列表，支持缓存参数防止缓存
// @Tags 音乐
// @Accept json
// @Produce json
// @Param r query string false "随机参数，用于防止缓存"
// @Success 200 {object} response.Response{data=[]music.Song} "成功"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/public/music/playlist [get]
func (h *MusicHandler) GetPlaylist(c *gin.Context) {
	// 获取播放列表
	songs, err := h.musicSvc.FetchPlaylist(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取播放列表失败: "+err.Error())
		return
	}

	// 返回成功响应
	response.Success(c, gin.H{
		"songs": songs,
		"total": len(songs),
	}, "获取播放列表成功")
}

// GetSongResources 获取歌曲资源（音频和歌词）
// @Summary 获取歌曲资源
// @Description 根据歌曲信息获取音频URL和歌词内容，自动尝试高质量资源
// @Tags 音乐
// @Accept json
// @Produce json
// @Param body body GetSongResourcesRequest true "歌曲信息"
// @Success 200 {object} response.Response{data=music.SongResourceResponse} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/public/music/song-resources [post]
func (h *MusicHandler) GetSongResources(c *gin.Context) {
	var req GetSongResourcesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	// 验证必要参数（binding已经处理了必需字段的验证）
	// 如果ID为空，使用默认值
	if req.ID == "" {
		req.ID = "unknown"
	}

	// 构建歌曲对象
	song := music.Song{
		ID:        req.ID,
		NeteaseID: req.NeteaseID,
		Name:      req.Name,
		Artist:    req.Artist,
		URL:       req.URL,
		Pic:       req.Pic,
		Lrc:       req.Lrc,
	}

	// 获取歌曲资源
	resources, err := h.musicSvc.FetchSongResources(c.Request.Context(), song)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取歌曲资源失败: "+err.Error())
		return
	}

	// 返回成功响应
	response.Success(c, resources, "获取歌曲资源成功")
}

// GetHighQualityMusicUrl 获取高质量音频URL
// @Summary 获取高质量音频URL
// @Description 根据网易云音乐ID获取高质量音频URL
// @Tags 音乐
// @Accept json
// @Produce json
// @Param neteaseId query string true "网易云音乐ID"
// @Success 200 {object} response.Response{data=gin.H} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/public/music/high-quality-url [get]
func (h *MusicHandler) GetHighQualityMusicUrl(c *gin.Context) {
	neteaseID := c.Query("neteaseId")
	if neteaseID == "" {
		response.Fail(c, http.StatusBadRequest, "网易云音乐ID不能为空")
		return
	}

	// 获取高质量音频URL
	url, err := h.musicSvc.FetchHighQualityMusicUrl(c.Request.Context(), neteaseID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取高质量音频URL失败: "+err.Error())
		return
	}

	// 返回响应
	if url == "" {
		response.Success(c, gin.H{
			"url":       "",
			"available": false,
		}, "高质量音频不可用")
	} else {
		response.Success(c, gin.H{
			"url":       url,
			"available": true,
		}, "获取高质量音频URL成功")
	}
}

// GetHighQualityLyrics 获取高质量歌词
// @Summary 获取高质量歌词
// @Description 根据网易云音乐ID获取高质量歌词
// @Tags 音乐
// @Accept json
// @Produce json
// @Param neteaseId query string true "网易云音乐ID"
// @Success 200 {object} response.Response{data=gin.H} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/public/music/high-quality-lyrics [get]
func (h *MusicHandler) GetHighQualityLyrics(c *gin.Context) {
	neteaseID := c.Query("neteaseId")
	if neteaseID == "" {
		response.Fail(c, http.StatusBadRequest, "网易云音乐ID不能为空")
		return
	}

	// 获取高质量歌词
	lyrics, err := h.musicSvc.FetchHighQualityLyrics(c.Request.Context(), neteaseID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取高质量歌词失败: "+err.Error())
		return
	}

	// 返回响应
	if lyrics == "" {
		response.Success(c, gin.H{
			"lyrics":    "",
			"available": false,
		}, "高质量歌词不可用")
	} else {
		response.Success(c, gin.H{
			"lyrics":    lyrics,
			"available": true,
		}, "获取高质量歌词成功")
	}
}

// GetLyrics 获取歌词
// @Summary 获取歌词
// @Description 根据歌词URL获取歌词内容
// @Tags 音乐
// @Accept json
// @Produce json
// @Param url query string true "歌词URL"
// @Success 200 {object} response.Response{data=gin.H} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/public/music/lyrics [get]
func (h *MusicHandler) GetLyrics(c *gin.Context) {
	lrcURL := c.Query("url")
	if lrcURL == "" {
		response.Fail(c, http.StatusBadRequest, "歌词URL不能为空")
		return
	}

	// 获取歌词
	lyrics, err := h.musicSvc.FetchLyrics(c.Request.Context(), lrcURL)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取歌词失败: "+err.Error())
		return
	}

	// 返回响应
	if lyrics == "" {
		response.Success(c, gin.H{
			"lyrics":    "",
			"available": false,
		}, "歌词不可用")
	} else {
		response.Success(c, gin.H{
			"lyrics":    lyrics,
			"available": true,
		}, "获取歌词成功")
	}
}

// GetMusicConfig 获取音乐配置
// @Summary 获取音乐配置
// @Description 获取当前音乐播放器的配置信息
// @Tags 音乐
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=gin.H} "成功"
// @Router /api/public/music/config [get]
func (h *MusicHandler) GetMusicConfig(c *gin.Context) {
	// 这里可以返回一些音乐播放器的配置信息
	// 比如是否启用了高质量API、默认播放列表ID等
	response.Success(c, gin.H{
		"highQualityApiEnabled": true, // 这个可以从service获取实际状态
		"supportedFormats":      []string{"mp3", "flac", "wav"},
		"maxConcurrentRequests": 3,
	}, "获取音乐配置成功")
}

// GetSongResourcesRequest 获取歌曲资源的请求结构
type GetSongResourcesRequest struct {
	ID        string `json:"id"`
	NeteaseID string `json:"neteaseId"`
	Name      string `json:"name" binding:"required"`
	Artist    string `json:"artist" binding:"required"`
	URL       string `json:"url" binding:"required"`
	Pic       string `json:"pic"`
	Lrc       string `json:"lrc"`
}

// HealthCheck 健康检查
// @Summary 音乐服务健康检查
// @Description 检查音乐服务是否正常运行
// @Tags 音乐
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=gin.H} "服务正常"
// @Router /api/public/music/health [get]
func (h *MusicHandler) HealthCheck(c *gin.Context) {
	response.Success(c, gin.H{
		"status":    "healthy",
		"service":   "music",
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}, "音乐服务运行正常")
}
