/*
 * @Description: 访问统计API处理器
 * @Author: 安知鱼
 * @Date: 2025-01-20 15:30:00
 * @LastEditTime: 2025-08-26 20:02:33
 * @LastEditors: 安知鱼
 */
package statistics

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/statistics"

	"github.com/gin-gonic/gin"
)

// StatisticsHandler 统计API处理器
type StatisticsHandler struct {
	statService statistics.VisitorStatService
}

// NewStatisticsHandler 创建统计处理器实例
func NewStatisticsHandler(statService statistics.VisitorStatService) *StatisticsHandler {
	return &StatisticsHandler{
		statService: statService,
	}
}

// GetBasicStatistics 获取基础统计数据（前台接口）
// @Summary 获取基础统计数据
// @Description 获取今日、昨日、月、年访问统计数据
// @Tags 统计
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=model.VisitorStatistics}
// @Router /api/public/statistics/basic [get]
func (h *StatisticsHandler) GetBasicStatistics(c *gin.Context) {
	stats, err := h.statService.GetBasicStatistics(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取统计数据失败")
		return
	}

	response.Success(c, stats, "获取统计数据成功")
}

// RecordVisit 记录访问（前台接口）
// @Summary 记录访问
// @Description 记录用户访问行为
// @Tags 统计
// @Accept json
// @Produce json
// @Param request body model.VisitorLogRequest true "访问记录请求"
// @Success 200 {object} response.Response
// @Router /api/public/statistics/visit [post]
func (h *StatisticsHandler) RecordVisit(c *gin.Context) {
	var req model.VisitorLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 添加重试机制处理并发冲突
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if err := h.statService.RecordVisit(c.Request.Context(), c, &req); err != nil {
			lastErr = err

			// 检查是否是唯一约束冲突，如果是则重试
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				log.Printf("[statistics] RecordVisit retry %d due to constraint conflict: %v", i+1, err)
				// 短暂延迟后重试
				time.Sleep(time.Duration(i+1) * 10 * time.Millisecond)
				continue
			}

			// 其他错误直接返回
			break
		}

		// 成功则直接返回
		response.Success(c, nil, "记录访问成功")
		return
	}

	// 所有重试都失败了
	log.Printf("[statistics] RecordVisit service error after %d retries: %v", maxRetries, lastErr)
	response.Fail(c, http.StatusInternalServerError, "记录访问失败")
}

// GetVisitorAnalytics 获取访客分析数据（后台接口）
// @Summary 获取访客分析数据
// @Description 获取指定时间范围内的访客分析数据
// @Tags 统计管理
// @Accept json
// @Produce json
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} response.Response{data=model.VisitorAnalytics}
// @Security BearerAuth
// @Router /api/statistics/analytics [get]
func (h *StatisticsHandler) GetVisitorAnalytics(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	// 默认查询最近7天
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -7)

	var err error
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "开始日期格式错误")
			return
		}
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "结束日期格式错误")
			return
		}
	}

	analytics, err := h.statService.GetVisitorAnalytics(c.Request.Context(), startDate, endDate)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取访客分析数据失败")
		return
	}

	response.Success(c, analytics, "获取访客分析数据成功")
}

// GetTopPages 获取热门页面（后台接口）
// @Summary 获取热门页面
// @Description 获取访问量最高的页面列表
// @Tags 统计管理
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制" default(10)
// @Success 200 {object} response.Response{data=[]model.URLStatistics}
// @Security BearerAuth
// @Router /api/statistics/top-pages [get]
func (h *StatisticsHandler) GetTopPages(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	if limit > 100 {
		limit = 100 // 限制最大返回数量
	}

	pages, err := h.statService.GetTopPages(c.Request.Context(), limit)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取热门页面失败")
		return
	}

	response.Success(c, pages, "获取热门页面成功")
}

// GetVisitorTrend 获取访客趋势数据（后台接口）
// @Summary 获取访客趋势数据
// @Description 获取指定时间段的访客趋势数据
// @Tags 统计管理
// @Accept json
// @Produce json
// @Param period query string false "时间周期 (daily/weekly/monthly)" default(daily)
// @Param days query int false "查询天数" default(30)
// @Success 200 {object} response.Response{data=model.VisitorTrendData}
// @Security BearerAuth
// @Router /api/statistics/trend [get]
func (h *StatisticsHandler) GetVisitorTrend(c *gin.Context) {
	period := c.DefaultQuery("period", "daily")
	daysStr := c.DefaultQuery("days", "30")

	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 30
	}

	if days > 365 {
		days = 365 // 限制最大查询天数
	}

	trendData, err := h.statService.GetVisitorTrend(c.Request.Context(), period, days)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取访客趋势数据失败")
		return
	}

	response.Success(c, trendData, "获取访客趋势数据成功")
}

// GetStatisticsSummary 获取统计概览（后台接口）
// @Summary 获取统计概览
// @Description 获取完整的统计概览数据，包括基础统计、热门页面、访客分析等
// @Tags 统计管理
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=StatisticsSummary}
// @Security BearerAuth
// @Router /api/statistics/summary [get]
func (h *StatisticsHandler) GetStatisticsSummary(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取基础统计
	basicStats, err := h.statService.GetBasicStatistics(ctx)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取基础统计数据失败")
		return
	}

	// 获取热门页面
	topPages, err := h.statService.GetTopPages(ctx, 10)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取热门页面失败")
		return
	}

	// 获取最近7天的访客分析
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -7)
	analytics, err := h.statService.GetVisitorAnalytics(ctx, startDate, endDate)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取访客分析数据失败")
		return
	}

	// 获取最近30天的趋势数据
	trendData, err := h.statService.GetVisitorTrend(ctx, "daily", 30)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取趋势数据失败")
		return
	}

	summary := StatisticsSummary{
		BasicStats: basicStats,
		TopPages:   topPages,
		Analytics:  analytics,
		TrendData:  trendData,
	}

	response.Success(c, summary, "获取统计概览成功")
}

// StatisticsSummary 统计概览数据结构
type StatisticsSummary struct {
	BasicStats *model.VisitorStatistics `json:"basic_stats"`
	TopPages   []*model.URLStatistics   `json:"top_pages"`
	Analytics  *model.VisitorAnalytics  `json:"analytics"`
	TrendData  *model.VisitorTrendData  `json:"trend_data"`
}
