/*
 * @Description: 音乐服务 - 整合外部音乐API并提供统一接口
 * @Author: 安知鱼
 * @Date: 2025-09-22 15:00:00
 */
package music

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

// Song 歌曲结构体
type Song struct {
	ID        string `json:"id"`
	NeteaseID string `json:"neteaseId"`
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	URL       string `json:"url"`
	Pic       string `json:"pic"`
	Lrc       string `json:"lrc"`
}

// MusicApiResponse 高质量音乐API响应结构
type MusicApiResponse struct {
	Code      int                    `json:"code"`
	Data      []HighQualityMusicData `json:"data"`
	Msg       string                 `json:"msg"`       // 错误信息
	Timestamp string                 `json:"timestamp"` // 时间戳
}

// HighQualityMusicData 高质量音乐数据结构
type HighQualityMusicData struct {
	ID       int    `json:"id"` // 修改为int类型，因为API返回的是数字
	URL      string `json:"url"`
	Br       int    `json:"br"`
	Size     int    `json:"size"`
	MD5      string `json:"md5"`
	Level    string `json:"level"`    // 音质等级
	Duration string `json:"duration"` // 时长
	Time     string `json:"time"`     // 时间戳
}

// LyricApiResponse 歌词API响应结构
type LyricApiResponse struct {
	Code int `json:"code"`
	Data struct {
		Lrc string `json:"lrc"`
	} `json:"data"`
	Msg       string `json:"msg"`       // 错误信息
	Timestamp string `json:"timestamp"` // 时间戳
}

// SongResourceResponse 歌曲资源响应结构
type SongResourceResponse struct {
	AudioURL         string `json:"audioUrl"`
	LyricsText       string `json:"lyricsText"`
	UsingHighQuality bool   `json:"usingHighQuality"`
}

// MusicService 定义音乐服务接口
type MusicService interface {
	// 获取播放列表
	FetchPlaylist(ctx context.Context) ([]Song, error)
	// 获取歌曲资源（音频和歌词）
	FetchSongResources(ctx context.Context, song Song) (SongResourceResponse, error)
	// 获取高质量音频URL
	FetchHighQualityMusicUrl(ctx context.Context, neteaseID string) (string, error)
	// 获取高质量歌词
	FetchHighQualityLyrics(ctx context.Context, neteaseID string) (string, error)
	// 获取歌词
	FetchLyrics(ctx context.Context, lrcUrl string) (string, error)
}

// musicService 音乐服务实现
type musicService struct {
	settingSvc setting.SettingService
	httpClient *http.Client
	// API URLs
	highQualityMusicAPI string
	highQualityLyricAPI string
	// 状态控制
	isHighQualityApiEnabled bool
}

// NewMusicService 创建新的音乐服务
func NewMusicService(settingSvc setting.SettingService) MusicService {
	return &musicService{
		settingSvc:              settingSvc,
		httpClient:              &http.Client{Timeout: 15 * time.Second},
		highQualityMusicAPI:     "https://wyapi.toubiec.cn/api/music/url",
		highQualityLyricAPI:     "https://wyapi.toubiec.cn/api/music/lyric",
		isHighQualityApiEnabled: true,
	}
}

// logRequest 记录请求日志
func (s *musicService) logRequest(method, url string, requestBody []byte) {
	log.Printf("[MUSIC_API] ==================== API 请求开始 ====================")
	log.Printf("[MUSIC_API] 请求方法: %s", method)
	log.Printf("[MUSIC_API] 请求URL: %s", url)
	log.Printf("[MUSIC_API] 请求时间: %s", time.Now().Format("2006-01-02 15:04:05.000"))

	// 解析URL获取更多信息
	s.logURLInfo(url)

	if len(requestBody) > 0 {
		log.Printf("[MUSIC_API] 请求体长度: %d bytes", len(requestBody))
		log.Printf("[MUSIC_API] 请求体内容: %s", string(requestBody))

		// 尝试解析请求体JSON
		s.logRequestBodyInfo(requestBody)
	} else {
		log.Printf("[MUSIC_API] 请求体: 无")
	}
}

// logResponse 记录响应日志
func (s *musicService) logResponse(url string, statusCode int, responseBody []byte, duration time.Duration) {
	log.Printf("[MUSIC_API] ==================== API 响应完成 ====================")
	log.Printf("[MUSIC_API] 响应URL: %s", url)
	log.Printf("[MUSIC_API] 响应状态码: %d", statusCode)
	log.Printf("[MUSIC_API] 响应耗时: %v", duration)
	log.Printf("[MUSIC_API] 响应时间: %s", time.Now().Format("2006-01-02 15:04:05.000"))

	// 评估响应性能
	s.logPerformanceMetrics(duration, len(responseBody))

	if len(responseBody) == 0 {
		log.Printf("[MUSIC_API] 响应体: 空")
		return
	}

	responseStr := string(responseBody)
	responseSize := len(responseBody)

	// 记录响应体大小
	log.Printf("[MUSIC_API] 响应体长度: %d bytes", responseSize)

	// 根据响应大小决定记录策略
	if responseSize <= 2048 {
		// 小响应直接完整记录
		log.Printf("[MUSIC_API] 完整响应体: %s", responseStr)
	} else {
		// 大响应记录前500字符和后200字符
		prefix := ""
		suffix := ""

		if responseSize > 500 {
			prefix = responseStr[:500]
		} else {
			prefix = responseStr
		}

		if responseSize > 700 {
			suffix = responseStr[responseSize-200:]
		}

		log.Printf("[MUSIC_API] 响应体摘要(前500字符): %s", prefix)
		if suffix != "" {
			log.Printf("[MUSIC_API] 响应体摘要(后200字符): %s", suffix)
		}

		// 尝试解析JSON结构并记录关键信息
		s.logJSONStructure(responseStr)
	}

	log.Printf("[MUSIC_API] ==================== API 调用结束 ====================")
}

// logError 记录错误日志
func (s *musicService) logError(operation, url string, err error) {
	log.Printf("[MUSIC_API] ==================== API 错误 ====================")
	log.Printf("[MUSIC_API] 错误时间: %s", time.Now().Format("2006-01-02 15:04:05.000"))
	log.Printf("[MUSIC_API] 失败操作: %s", operation)
	log.Printf("[MUSIC_API] 请求URL: %s", url)
	log.Printf("[MUSIC_API] 错误类型: %T", err)
	log.Printf("[MUSIC_API] 错误详情: %v", err)

	// 识别错误类型
	errorType := "unknown"
	if strings.Contains(err.Error(), "timeout") {
		errorType = "timeout"
	} else if strings.Contains(err.Error(), "connection") {
		errorType = "connection"
	} else if strings.Contains(err.Error(), "json") {
		errorType = "json-parse"
	} else if strings.Contains(err.Error(), "unmarshal") {
		errorType = "data-parse"
	} else if strings.Contains(err.Error(), "context deadline exceeded") {
		errorType = "context-timeout"
	}

	log.Printf("[MUSIC_API] 错误分类: %s", errorType)
	log.Printf("[MUSIC_API] ==================== 错误记录结束 ====================")
}

// logJSONStructure 解析并记录JSON结构的关键信息
func (s *musicService) logJSONStructure(jsonStr string) {
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonData); err != nil {
		// 尝试解析为数组
		var jsonArray []interface{}
		if err2 := json.Unmarshal([]byte(jsonStr), &jsonArray); err2 == nil {
			log.Printf("[MUSIC_API] JSON结构: 数组, 元素数量: %d", len(jsonArray))
			if len(jsonArray) > 0 {
				// 记录第一个元素的结构（如果是对象）
				if firstElement, ok := jsonArray[0].(map[string]interface{}); ok {
					keys := make([]string, 0, len(firstElement))
					for key := range firstElement {
						keys = append(keys, key)
					}
					log.Printf("[MUSIC_API] 数组元素字段: %v", keys)
				}
			}
		} else {
			log.Printf("[MUSIC_API] JSON解析失败，可能不是有效的JSON格式")
		}
		return
	}

	// 记录JSON对象的关键字段
	summary := make(map[string]interface{})

	// 记录常见的API响应字段
	if code, exists := jsonData["code"]; exists {
		summary["code"] = code
	}
	if msg, exists := jsonData["msg"]; exists {
		summary["msg"] = msg
	}
	if message, exists := jsonData["message"]; exists {
		summary["message"] = message
	}
	if timestamp, exists := jsonData["timestamp"]; exists {
		summary["timestamp"] = timestamp
	}

	// 分析data字段
	if data, exists := jsonData["data"]; exists {
		if dataArray, ok := data.([]interface{}); ok {
			summary["data"] = fmt.Sprintf("数组(%d个元素)", len(dataArray))

			// 记录第一个元素的字段（如果是对象）
			if len(dataArray) > 0 {
				if firstItem, ok := dataArray[0].(map[string]interface{}); ok {
					keys := make([]string, 0, len(firstItem))
					for key := range firstItem {
						keys = append(keys, key)
					}
					summary["dataFields"] = keys
				}
			}
		} else if dataObj, ok := data.(map[string]interface{}); ok {
			keys := make([]string, 0, len(dataObj))
			for key := range dataObj {
				keys = append(keys, key)
			}
			summary["data"] = fmt.Sprintf("对象(字段: %v)", keys)
		} else if data == nil {
			summary["data"] = "null"
		} else {
			summary["data"] = fmt.Sprintf("%T", data)
		}
	}

	// 记录顶级字段
	allFields := make([]string, 0, len(jsonData))
	for key := range jsonData {
		allFields = append(allFields, key)
	}
	summary["allFields"] = allFields

	summaryJson, _ := json.Marshal(summary)
	log.Printf("[MUSIC_API] JSON结构摘要: %s", string(summaryJson))
}

// logURLInfo 解析并记录URL信息
func (s *musicService) logURLInfo(url string) {
	// 识别API类型
	apiType := "unknown"
	if strings.Contains(url, "meting.qjqq.cn") {
		apiType = "meting-api"
		if strings.Contains(url, "type=playlist") {
			apiType = "meting-playlist"
		} else if strings.Contains(url, "type=url") {
			apiType = "meting-audio"
		} else if strings.Contains(url, "type=lrc") {
			apiType = "meting-lyrics"
		} else if strings.Contains(url, "type=pic") {
			apiType = "meting-picture"
		}
	} else if strings.Contains(url, "wyapi.toubiec.cn") {
		apiType = "high-quality-api"
		if strings.Contains(url, "/music/url") {
			apiType = "high-quality-audio"
		} else if strings.Contains(url, "/music/lyric") {
			apiType = "high-quality-lyrics"
		}
	}

	log.Printf("[MUSIC_API] API类型: %s", apiType)

	// 提取URL参数
	if strings.Contains(url, "?") {
		parts := strings.Split(url, "?")
		if len(parts) > 1 {
			log.Printf("[MUSIC_API] URL参数: %s", parts[1])
		}
	}
}

// logRequestBodyInfo 解析并记录请求体信息
func (s *musicService) logRequestBodyInfo(requestBody []byte) {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(requestBody, &jsonData); err != nil {
		log.Printf("[MUSIC_API] 请求体不是有效的JSON格式")
		return
	}

	// 记录请求参数摘要
	summary := make(map[string]interface{})
	for key, value := range jsonData {
		// 对于敏感信息，只记录类型不记录值
		if key == "id" || key == "neteaseId" {
			summary[key] = value
		} else {
			summary[key] = fmt.Sprintf("%T", value)
		}
	}

	summaryJson, _ := json.Marshal(summary)
	log.Printf("[MUSIC_API] 请求参数摘要: %s", string(summaryJson))
}

// logPerformanceMetrics 记录性能指标
func (s *musicService) logPerformanceMetrics(duration time.Duration, responseSize int) {
	// 评估性能等级
	performanceLevel := "excellent"
	if duration > 2*time.Second {
		performanceLevel = "slow"
	} else if duration > 1*time.Second {
		performanceLevel = "normal"
	} else if duration > 500*time.Millisecond {
		performanceLevel = "good"
	}

	log.Printf("[MUSIC_API] 性能评级: %s", performanceLevel)

	// 计算平均速度
	if responseSize > 0 && duration > 0 {
		speed := float64(responseSize) / duration.Seconds() / 1024 // KB/s
		log.Printf("[MUSIC_API] 传输速度: %.2f KB/s", speed)
	}

	// 记录响应大小分类
	sizeCategory := "small"
	if responseSize > 100*1024 {
		sizeCategory = "large"
	} else if responseSize > 10*1024 {
		sizeCategory = "medium"
	}

	log.Printf("[MUSIC_API] 响应大小分类: %s (%d bytes)", sizeCategory, responseSize)
}

// getPlaylistID 获取播放列表ID
func (s *musicService) getPlaylistID() string {
	// 从多个配置键尝试获取播放列表ID
	playlistID := s.settingSvc.Get("music.player.playlist_id")
	if playlistID == "" {
		playlistID = s.settingSvc.Get("MUSIC_PLAYER_PLAYLIST_ID")
	}
	if playlistID == "" {
		playlistID = "8152976493" // 默认值
	}
	return playlistID
}

// buildPlaylistAPI 构建播放列表API URL
func (s *musicService) buildPlaylistAPI() string {
	playlistID := s.getPlaylistID()
	return fmt.Sprintf("https://meting.qjqq.cn/?server=netease&type=playlist&id=%s", playlistID)
}

// isValidSong 验证歌曲数据是否有效
func (s *musicService) isValidSong(song map[string]interface{}) bool {
	name, nameOk := song["name"].(string)
	artist, artistOk := song["artist"].(string)
	url, urlOk := song["url"].(string)

	return nameOk && artistOk && urlOk && name != "" && artist != "" && url != ""
}

// FetchPlaylist 获取播放列表
func (s *musicService) FetchPlaylist(ctx context.Context) ([]Song, error) {
	playlistURL := s.buildPlaylistAPI()

	// 记录开始日志
	log.Printf("[MUSIC_API] 开始获取播放列表 - 播放列表ID: %s", s.getPlaylistID())

	// 记录请求日志
	s.logRequest("GET", playlistURL, nil)

	startTime := time.Now()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", playlistURL, nil)
	if err != nil {
		s.logError("创建播放列表请求", playlistURL, err)
		return nil, fmt.Errorf("创建播放列表请求失败: %w", err)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logError("获取播放列表", playlistURL, err)
		return nil, fmt.Errorf("获取播放列表失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logError("读取播放列表响应", playlistURL, err)
		return nil, fmt.Errorf("读取播放列表响应失败: %w", err)
	}

	duration := time.Since(startTime)
	s.logResponse(playlistURL, resp.StatusCode, responseBody, duration)

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		log.Printf("[MUSIC_API] 播放列表API返回错误状态码: %d", resp.StatusCode)
		return nil, fmt.Errorf("播放列表API返回错误状态码: %d", resp.StatusCode)
	}

	// 解析JSON
	var data []map[string]interface{}
	if err := json.Unmarshal(responseBody, &data); err != nil {
		s.logError("解析播放列表JSON", playlistURL, err)
		return nil, fmt.Errorf("解析播放列表JSON失败: %w", err)
	}

	// 验证和转换数据
	var songs []Song
	validCount := 0

	for i, item := range data {
		if !s.isValidSong(item) {
			log.Printf("[MUSIC_API] 跳过无效歌曲数据，索引: %d", i)
			continue
		}

		// 提取网易云音乐ID
		neteaseID := s.extractNeteaseID(item)

		song := Song{
			ID:        fmt.Sprintf("%d", i),
			NeteaseID: neteaseID,
			Name:      getString(item["name"]),
			Artist:    getString(item["artist"]),
			URL:       getString(item["url"]),
			Pic:       getString(item["pic"]),
			Lrc:       getString(item["lrc"]),
		}

		songs = append(songs, song)
		validCount++
	}

	log.Printf("[MUSIC_API] 播放列表解析完成 - 总数: %d, 有效: %d", len(data), validCount)
	return songs, nil
}

// extractNeteaseID 从歌曲数据中提取网易云音乐ID
func (s *musicService) extractNeteaseID(song map[string]interface{}) string {
	// 尝试从URL中提取ID
	if url, ok := song["url"].(string); ok && url != "" {
		if id := s.extractIDFromURL(url); id != "" && s.isValidNeteaseID(id) {
			return id
		}
	}

	// 尝试从歌词URL中提取ID
	if lrc, ok := song["lrc"].(string); ok && lrc != "" {
		if id := s.extractIDFromURL(lrc); id != "" && s.isValidNeteaseID(id) {
			return id
		}
	}

	// 尝试从图片URL中提取ID
	if pic, ok := song["pic"].(string); ok && pic != "" {
		if id := s.extractIDFromURL(pic); id != "" && s.isValidNeteaseID(id) {
			return id
		}
	}

	log.Printf("[MUSIC_API] 未能从歌曲数据中提取有效的网易云音乐ID - 歌曲: %v", song["name"])
	return ""
}

// extractIDFromURL 从URL中提取ID
func (s *musicService) extractIDFromURL(url string) string {
	re := regexp.MustCompile(`[?&]id=(\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// FetchHighQualityMusicUrl 获取高质量音频URL
func (s *musicService) FetchHighQualityMusicUrl(ctx context.Context, neteaseID string) (string, error) {
	if !s.isHighQualityApiEnabled {
		log.Printf("[MUSIC_API] 高质量API已禁用，跳过获取高质量音频")
		return "", nil
	}

	if neteaseID == "" {
		log.Printf("[MUSIC_API] 网易云音乐ID为空，无法获取高质量音频")
		return "", nil
	}

	// 验证ID格式 - 必须是纯数字
	if !s.isValidNeteaseID(neteaseID) {
		log.Printf("[MUSIC_API] 网易云音乐ID格式无效，跳过高质量音频获取 - ID: %s", neteaseID)
		return "", nil // 返回空字符串而不是错误，允许降级到原始音频
	}

	// 构建请求体
	requestData := map[string]interface{}{
		"id":    neteaseID,
		"level": "lossless",
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		s.logError("构建高质量音频请求体", s.highQualityMusicAPI, err)
		return "", fmt.Errorf("构建高质量音频请求体失败: %w", err)
	}

	// 记录请求日志
	s.logRequest("POST", s.highQualityMusicAPI, requestBody)

	startTime := time.Now()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", s.highQualityMusicAPI, bytes.NewBuffer(requestBody))
	if err != nil {
		s.logError("创建高质量音频请求", s.highQualityMusicAPI, err)
		return "", fmt.Errorf("创建高质量音频请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Pragma", "no-cache")

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logError("获取高质量音频", s.highQualityMusicAPI, err)
		// 禁用高质量API
		s.disableHighQualityApi("高质量音频请求失败")
		return "", nil // 返回nil而不是错误，允许降级到原始API
	}
	defer resp.Body.Close()

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logError("读取高质量音频响应", s.highQualityMusicAPI, err)
		return "", fmt.Errorf("读取高质量音频响应失败: %w", err)
	}

	duration := time.Since(startTime)
	s.logResponse(s.highQualityMusicAPI, resp.StatusCode, responseBody, duration)

	// 解析响应
	var apiResponse MusicApiResponse
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		s.logError("解析高质量音频JSON", s.highQualityMusicAPI, err)
		return "", fmt.Errorf("解析高质量音频JSON失败: %w", err)
	}

	// 检查响应
	if apiResponse.Code != 200 {
		// API返回了错误响应，记录详细信息但不抛出错误（允许降级）
		if apiResponse.Msg != "" {
			log.Printf("[MUSIC_API] 高质量音频获取失败 - 网易云ID: %s, 响应码: %d, 错误信息: %s",
				neteaseID, apiResponse.Code, apiResponse.Msg)
		} else {
			log.Printf("[MUSIC_API] 高质量音频获取失败 - 网易云ID: %s, 响应码: %d", neteaseID, apiResponse.Code)
		}
		return "", nil // 返回空而不是错误，允许降级到原始API
	}

	// 检查数据是否有效
	if len(apiResponse.Data) > 0 {
		musicData := apiResponse.Data[0]
		if musicData.URL != "" {
			log.Printf("[MUSIC_API] 成功获取高质量音频URL - 网易云ID: %s, 码率: %d", neteaseID, musicData.Br)
			return musicData.URL, nil
		}
	}

	log.Printf("[MUSIC_API] 高质量音频数据为空或无效 - 网易云ID: %s", neteaseID)
	return "", nil
}

// FetchHighQualityLyrics 获取高质量歌词
func (s *musicService) FetchHighQualityLyrics(ctx context.Context, neteaseID string) (string, error) {
	if !s.isHighQualityApiEnabled {
		log.Printf("[MUSIC_API] 高质量API已禁用，跳过获取高质量歌词")
		return "", nil
	}

	if neteaseID == "" {
		log.Printf("[MUSIC_API] 网易云音乐ID为空，无法获取高质量歌词")
		return "", nil
	}

	// 验证ID格式 - 必须是纯数字
	if !s.isValidNeteaseID(neteaseID) {
		log.Printf("[MUSIC_API] 网易云音乐ID格式无效，跳过高质量歌词获取 - ID: %s", neteaseID)
		return "", nil // 返回空字符串而不是错误，允许降级到原始歌词
	}

	// 构建请求体
	requestData := map[string]interface{}{
		"id": neteaseID,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		s.logError("构建高质量歌词请求体", s.highQualityLyricAPI, err)
		return "", fmt.Errorf("构建高质量歌词请求体失败: %w", err)
	}

	// 记录请求日志
	s.logRequest("POST", s.highQualityLyricAPI, requestBody)

	startTime := time.Now()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", s.highQualityLyricAPI, bytes.NewBuffer(requestBody))
	if err != nil {
		s.logError("创建高质量歌词请求", s.highQualityLyricAPI, err)
		return "", fmt.Errorf("创建高质量歌词请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Pragma", "no-cache")

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logError("获取高质量歌词", s.highQualityLyricAPI, err)
		// 禁用高质量API
		s.disableHighQualityApi("高质量歌词请求失败")
		return "", nil // 返回nil而不是错误，允许降级到原始API
	}
	defer resp.Body.Close()

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logError("读取高质量歌词响应", s.highQualityLyricAPI, err)
		return "", fmt.Errorf("读取高质量歌词响应失败: %w", err)
	}

	duration := time.Since(startTime)
	s.logResponse(s.highQualityLyricAPI, resp.StatusCode, responseBody, duration)

	// 解析响应
	var apiResponse LyricApiResponse
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		s.logError("解析高质量歌词JSON", s.highQualityLyricAPI, err)
		return "", fmt.Errorf("解析高质量歌词JSON失败: %w", err)
	}

	// 检查响应
	if apiResponse.Code != 200 {
		// API返回了错误响应，记录详细信息但不抛出错误（允许降级）
		if apiResponse.Msg != "" {
			log.Printf("[MUSIC_API] 高质量歌词获取失败 - 网易云ID: %s, 响应码: %d, 错误信息: %s",
				neteaseID, apiResponse.Code, apiResponse.Msg)
		} else {
			log.Printf("[MUSIC_API] 高质量歌词获取失败 - 网易云ID: %s, 响应码: %d", neteaseID, apiResponse.Code)
		}
		return "", nil // 返回空而不是错误，允许降级到原始API
	}

	// 检查歌词数据是否有效
	if apiResponse.Data.Lrc != "" {
		lrcText := apiResponse.Data.Lrc
		if s.isValidLRCFormat(lrcText) {
			log.Printf("[MUSIC_API] 成功获取高质量歌词 - 网易云ID: %s, 长度: %d", neteaseID, len(lrcText))
			return lrcText, nil
		}
		log.Printf("[MUSIC_API] 高质量歌词格式无效 - 网易云ID: %s", neteaseID)
	}

	log.Printf("[MUSIC_API] 高质量歌词数据为空或无效 - 网易云ID: %s", neteaseID)
	return "", nil
}

// FetchLyrics 获取原始歌词
func (s *musicService) FetchLyrics(ctx context.Context, lrcUrl string) (string, error) {
	if lrcUrl == "" {
		return "", nil
	}

	// 记录请求日志
	s.logRequest("GET", lrcUrl, nil)

	startTime := time.Now()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", lrcUrl, nil)
	if err != nil {
		s.logError("创建歌词请求", lrcUrl, err)
		return "", fmt.Errorf("创建歌词请求失败: %w", err)
	}

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logError("获取歌词", lrcUrl, err)
		return "", fmt.Errorf("获取歌词失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logError("读取歌词响应", lrcUrl, err)
		return "", fmt.Errorf("读取歌词响应失败: %w", err)
	}

	duration := time.Since(startTime)
	s.logResponse(lrcUrl, resp.StatusCode, responseBody, duration)

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		log.Printf("[MUSIC_API] 歌词API返回错误状态码: %d", resp.StatusCode)
		return "", nil
	}

	lrcText := string(responseBody)

	// 验证歌词格式
	if lrcText == "" || !s.isValidLRCFormat(lrcText) {
		log.Printf("[MUSIC_API] 获取到的歌词格式无效或为空")
		return "", nil
	}

	log.Printf("[MUSIC_API] 成功获取原始歌词 - 长度: %d", len(lrcText))
	return lrcText, nil
}

// FetchSongResources 获取歌曲的完整资源
func (s *musicService) FetchSongResources(ctx context.Context, song Song) (SongResourceResponse, error) {
	log.Printf("[MUSIC_API] 开始获取歌曲资源 - 歌曲: %s, 艺术家: %s", song.Name, song.Artist)
	log.Printf("[MUSIC_API] 歌曲详细信息 - ID: %s, NeteaseID: %s, URL: %s, Pic: %s, Lrc: %s",
		song.ID, song.NeteaseID, song.URL, song.Pic, song.Lrc)

	result := SongResourceResponse{
		AudioURL:         song.URL, // 默认使用原始URL
		LyricsText:       "",
		UsingHighQuality: false,
	}

	// 如果有网易云ID，尝试获取高质量资源
	if song.NeteaseID != "" && s.isHighQualityApiEnabled {
		// 先验证ID格式
		if !s.isValidNeteaseID(song.NeteaseID) {
			log.Printf("[MUSIC_API] 歌曲包含无效的网易云音乐ID，跳过高质量资源获取 - ID: %s, 歌曲: %s", song.NeteaseID, song.Name)
		} else {
			log.Printf("[MUSIC_API] 尝试获取高质量资源 - 网易云ID: %s", song.NeteaseID)

			// 尝试获取高质量音频
			if highQualityURL, err := s.FetchHighQualityMusicUrl(ctx, song.NeteaseID); err == nil && highQualityURL != "" {
				result.AudioURL = highQualityURL
				result.UsingHighQuality = true
				log.Printf("[MUSIC_API] 使用高质量音频URL")

				// 音频成功后尝试获取高质量歌词
				if highQualityLyrics, err := s.FetchHighQualityLyrics(ctx, song.NeteaseID); err == nil && highQualityLyrics != "" {
					result.LyricsText = highQualityLyrics
					log.Printf("[MUSIC_API] 使用高质量歌词")
				}
			}
		}
	}

	// 如果还没有歌词，尝试获取原始歌词
	if result.LyricsText == "" && song.Lrc != "" {
		log.Printf("[MUSIC_API] 尝试获取原始歌词")
		if originalLyrics, err := s.FetchLyrics(ctx, song.Lrc); err == nil && originalLyrics != "" {
			result.LyricsText = originalLyrics
			log.Printf("[MUSIC_API] 使用原始歌词")
		}
	}

	log.Printf("[MUSIC_API] 歌曲资源获取完成 - 高质量: %v, 有歌词: %v",
		result.UsingHighQuality, result.LyricsText != "")

	return result, nil
}

// disableHighQualityApi 禁用高质量API
func (s *musicService) disableHighQualityApi(reason string) {
	if s.isHighQualityApiEnabled {
		s.isHighQualityApiEnabled = false
		log.Printf("[MUSIC_API] 禁用高质量API - 原因: %s", reason)
	}
}

// isValidLRCFormat 验证LRC格式
func (s *musicService) isValidLRCFormat(lrcText string) bool {
	if lrcText == "" {
		return false
	}

	// 检查是否包含LRC时间标签
	lrcPattern := regexp.MustCompile(`\[\d{1,2}:\d{2}[\.:]?\d{0,3}\]`)
	return lrcPattern.MatchString(lrcText)
}

// isValidNeteaseID 验证网易云音乐ID格式是否有效
func (s *musicService) isValidNeteaseID(neteaseID string) bool {
	if neteaseID == "" {
		return false
	}

	// 网易云音乐ID应该是纯数字格式，长度通常在6-12位
	neteaseIDPattern := regexp.MustCompile(`^\d{6,12}$`)
	return neteaseIDPattern.MatchString(neteaseID)
}

// getString 安全地获取字符串值
func getString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
