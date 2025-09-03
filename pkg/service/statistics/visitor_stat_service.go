/*
 * @Description: 访问统计服务
 * @Author: 安知鱼
 * @Date: 2025-01-20 15:30:00
 * @LastEditTime: 2025-08-21 11:07:35
 * @LastEditors: 安知鱼
 */
package statistics

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/utility"

	"github.com/gin-gonic/gin"
)

// VisitorStatService 访问统计服务接口
type VisitorStatService interface {
	// 记录访问日志
	RecordVisit(ctx context.Context, c *gin.Context, req *model.VisitorLogRequest) error

	// 获取基础统计数据
	GetBasicStatistics(ctx context.Context) (*model.VisitorStatistics, error)

	// 获取访客分析数据
	GetVisitorAnalytics(ctx context.Context, startDate, endDate time.Time) (*model.VisitorAnalytics, error)

	// 获取热门页面
	GetTopPages(ctx context.Context, limit int) ([]*model.URLStatistics, error)

	// 获取访客趋势数据
	GetVisitorTrend(ctx context.Context, period string, days int) (*model.VisitorTrendData, error)

	// 聚合日统计数据
	AggregateDaily(ctx context.Context, date time.Time) error

	// 获取实时统计数据
	GetRealTimeStats(ctx context.Context) (*model.VisitorStatistics, error)

	// 获取最后一次成功聚合的日期
	GetLastAggregatedDate(ctx context.Context) (*time.Time, error)

	// 获取第一条访问日志的日期
	GetFirstLogDate(ctx context.Context) (*time.Time, error)

	// 获取访客访问日志（时间范围）
	GetVisitorLogs(ctx context.Context, startDate, endDate time.Time) ([]*ent.VisitorLog, error)
}

type visitorStatService struct {
	visitorStatRepo repository.VisitorStatRepository
	visitorLogRepo  repository.VisitorLogRepository
	urlStatRepo     repository.URLStatRepository
	geoipService    utility.GeoIPService
	cacheService    utility.CacheService
}

// NewVisitorStatService 创建访问统计服务实例
func NewVisitorStatService(
	visitorStatRepo repository.VisitorStatRepository,
	visitorLogRepo repository.VisitorLogRepository,
	urlStatRepo repository.URLStatRepository,
	cacheService utility.CacheService,
	geoipService utility.GeoIPService,
) (VisitorStatService, error) {
	return &visitorStatService{
		visitorStatRepo: visitorStatRepo,
		visitorLogRepo:  visitorLogRepo,
		urlStatRepo:     urlStatRepo,
		cacheService:    cacheService,
		geoipService:    geoipService,
	}, nil
}

// 获取最后一次成功聚合的日期
func (s *visitorStatService) GetLastAggregatedDate(ctx context.Context) (*time.Time, error) {
	return s.visitorStatRepo.GetLatestDate(ctx)
}

// 获取第一条访问日志的日期
func (s *visitorStatService) GetFirstLogDate(ctx context.Context) (*time.Time, error) {
	return s.visitorLogRepo.GetFirstDate(ctx)
}

// 获取访客访问日志（时间范围）
func (s *visitorStatService) GetVisitorLogs(ctx context.Context, startDate, endDate time.Time) ([]*ent.VisitorLog, error) {
	return s.visitorLogRepo.GetByTimeRange(ctx, startDate, endDate)
}

func (s *visitorStatService) RecordVisit(ctx context.Context, c *gin.Context, req *model.VisitorLogRequest) error {
	// 获取客户端IP
	clientIP := s.getClientIP(c)

	// 生成访客ID（基于IP和UserAgent的hash）
	visitorID := s.generateVisitorID(clientIP, c.GetHeader("User-Agent"))

	// 获取地理位置信息
	country, region, city := s.getGeoLocation(clientIP)

	// 解析User-Agent
	browser, os, device := s.parseUserAgent(c.GetHeader("User-Agent"))

	// 检查是否为新访客（今日内未访问过）
	recentLogs, _ := s.visitorLogRepo.GetByVisitorID(ctx, visitorID, 1)

	// 获取今日开始时间（UTC时区）
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// 判断是否为新访客：没有访问记录，或者最近访问时间在今天之前
	// 这样确保每天都会重新计算访客数，而不是基于24小时窗口
	isUnique := len(recentLogs) == 0 || recentLogs[0].CreatedAt.Before(todayStart)

	// 创建访问日志
	userAgent := c.GetHeader("User-Agent")
	log := &ent.VisitorLog{
		VisitorID: visitorID,
		IPAddress: clientIP,
		UserAgent: &userAgent,
		Referer:   &req.Referer,
		URLPath:   req.URLPath,
		Country:   &country,
		Region:    &region,
		City:      &city,
		Browser:   &browser,
		Os:        &os,
		Device:    &device,
		Duration:  req.Duration,
		IsBounce:  req.Duration < 10, // 停留时间少于10秒认为是跳出
		CreatedAt: time.Now(),
	}

	// 保存访问日志
	if err := s.visitorLogRepo.Create(ctx, log); err != nil {
		return fmt.Errorf("保存访问日志失败: %w", err)
	}

	// 更新URL统计
	if err := s.urlStatRepo.IncrementViews(ctx, req.URLPath, isUnique, req.Duration); err != nil {
		return fmt.Errorf("更新URL统计失败: %w", err)
	}

	// 更新Redis实时计数缓存
	if s.cacheService != nil {
		// 增加今日访问量
		todayViewsKey := CacheKeyTodayViews + time.Now().Format("2006-01-02")
		s.cacheService.Increment(ctx, todayViewsKey)
		s.cacheService.Expire(ctx, todayViewsKey, CacheExpireToday)

		// 如果是新访客，增加访客计数
		if isUnique {
			todayVisitorsKey := "stats:today:visitors:" + time.Now().Format("2006-01-02")
			s.cacheService.Increment(ctx, todayVisitorsKey)
			s.cacheService.Expire(ctx, todayVisitorsKey, CacheExpireToday)
		}

		// 清除基础统计缓存，确保数据一致性
		s.cacheService.Delete(ctx, CacheKeyBasicStats)
	}

	return nil
}

func (s *visitorStatService) GetBasicStatistics(ctx context.Context) (*model.VisitorStatistics, error) {
	// 尝试从缓存获取
	if s.cacheService != nil {
		cachedData, err := s.cacheService.Get(ctx, CacheKeyBasicStats)
		if err == nil && cachedData != "" {
			var stats model.VisitorStatistics
			if json.Unmarshal([]byte(cachedData), &stats) == nil {
				return &stats, nil
			}
		}
	}

	// 缓存未命中，尝试从Redis实时计数获取
	if s.cacheService != nil {
		stats := &model.VisitorStatistics{}
		now := time.Now()
		today := now.Format("2006-01-02")

		// 从Redis获取今日实时数据
		if todayViews, err := s.cacheService.Get(ctx, CacheKeyTodayViews+today); err == nil && todayViews != "" {
			if views, err := strconv.ParseInt(todayViews, 10, 64); err == nil {
				stats.TodayViews = views
			}
		}

		if todayVisitors, err := s.cacheService.Get(ctx, "stats:today:visitors:"+today); err == nil && todayVisitors != "" {
			if visitors, err := strconv.ParseInt(todayVisitors, 10, 64); err == nil {
				stats.TodayVisitors = visitors
			}
		}

		// 如果Redis中有今日数据，从数据库获取其他数据
		if stats.TodayViews > 0 || stats.TodayVisitors > 0 {
			// 从数据库获取昨日、月、年数据
			dbStats, err := s.visitorStatRepo.GetBasicStatistics(ctx)
			if err == nil {
				stats.YesterdayVisitors = dbStats.YesterdayVisitors
				stats.YesterdayViews = dbStats.YesterdayViews
				stats.MonthViews = dbStats.MonthViews
				stats.YearViews = dbStats.YearViews
			}

			// 写入缓存
			if data, err := json.Marshal(stats); err == nil {
				s.cacheService.Set(ctx, CacheKeyBasicStats, string(data), CacheExpireBasicStats)
			}

			return stats, nil
		}
	}

	// 如果Redis中没有实时数据，从数据库获取
	stats, err := s.visitorStatRepo.GetBasicStatistics(ctx)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	if s.cacheService != nil {
		if data, err := json.Marshal(stats); err == nil {
			s.cacheService.Set(ctx, CacheKeyBasicStats, string(data), CacheExpireBasicStats)
		}
	}

	return stats, nil
}

func (s *visitorStatService) GetVisitorAnalytics(ctx context.Context, startDate, endDate time.Time) (*model.VisitorAnalytics, error) {
	return s.visitorLogRepo.GetVisitorAnalytics(ctx, startDate, endDate)
}

func (s *visitorStatService) GetTopPages(ctx context.Context, limit int) ([]*model.URLStatistics, error) {
	// 尝试从缓存获取
	if s.cacheService != nil {
		cacheKey := fmt.Sprintf("%s%d", CacheKeyTopPages, limit)
		cachedData, err := s.cacheService.Get(ctx, cacheKey)
		if err == nil && cachedData != "" {
			var pages []*model.URLStatistics
			if json.Unmarshal([]byte(cachedData), &pages) == nil {
				return pages, nil
			}
		}
	}

	// 缓存未命中，从数据库获取
	pages, err := s.urlStatRepo.GetTopPages(ctx, limit)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	if s.cacheService != nil {
		if data, err := json.Marshal(pages); err == nil {
			cacheKey := fmt.Sprintf("%s%d", CacheKeyTopPages, limit)
			s.cacheService.Set(ctx, cacheKey, string(data), CacheExpireTopPages)
		}
	}

	return pages, nil
}

func (s *visitorStatService) GetVisitorTrend(ctx context.Context, period string, days int) (*model.VisitorTrendData, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	stats, err := s.visitorStatRepo.GetByDateRange(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	trendData := &model.VisitorTrendData{
		Daily: make([]model.DateRangeStats, 0),
	}

	// 转换为趋势数据格式
	for _, stat := range stats {
		trendData.Daily = append(trendData.Daily, model.DateRangeStats{
			Date:     stat.Date,
			Visitors: stat.UniqueVisitors,
			Views:    stat.TotalViews,
		})
	}

	return trendData, nil
}

func (s *visitorStatService) AggregateDaily(ctx context.Context, date time.Time) error {
	// 统计指定日期的数据
	uniqueVisitors, err := s.visitorLogRepo.CountUniqueVisitors(ctx, date)
	if err != nil {
		return fmt.Errorf("统计独立访客失败: %w", err)
	}

	totalViews, err := s.visitorLogRepo.CountTotalViews(ctx, date)
	if err != nil {
		return fmt.Errorf("统计总访问量失败: %w", err)
	}

	// 创建或更新统计记录
	stat := &ent.VisitorStat{
		Date:           date,
		UniqueVisitors: uniqueVisitors,
		TotalViews:     totalViews,
		PageViews:      totalViews, // 简化处理，实际可能需要区分页面浏览和其他请求
		BounceCount:    0,          // 需要单独统计跳出次数
	}

	return s.visitorStatRepo.CreateOrUpdate(ctx, stat)
}

// GetRealTimeStats 获取实时统计数据（优先从缓存获取）
func (s *visitorStatService) GetRealTimeStats(ctx context.Context) (*model.VisitorStatistics, error) {
	// 尝试从缓存获取
	if s.cacheService != nil {
		cacheKey := "stats:realtime:" + time.Now().Format("2006-01-02")
		cachedData, err := s.cacheService.Get(ctx, cacheKey)
		if err == nil && cachedData != "" {
			var stats model.VisitorStatistics
			if json.Unmarshal([]byte(cachedData), &stats) == nil {
				return &stats, nil
			}
		}
	}

	// 缓存未命中，从数据库获取
	return s.GetBasicStatistics(ctx)
}

// 高并发优化配置
const (
	// 缓存键常量
	CacheKeyBasicStats = "stats:basic"
	CacheKeyTopPages   = "stats:top_pages:"
	CacheKeyAnalytics  = "stats:analytics:"
	CacheKeyTodayViews = "stats:today:views:"

	// 实时计数缓存键
	CacheKeyRealTimeViews    = "stats:realtime:views:"
	CacheKeyRealTimeVisitors = "stats:realtime:visitors:"
	CacheKeyBatchQueue       = "stats:batch:queue:"

	// 缓存过期时间
	CacheExpireBasicStats = 5 * time.Minute
	CacheExpireTopPages   = 15 * time.Minute
	CacheExpireAnalytics  = 30 * time.Minute
	CacheExpireToday      = 24 * time.Hour
	CacheExpireRealTime   = 1 * time.Hour
	CacheExpireBatchQueue = 10 * time.Minute

	// 批量处理配置
	BatchSizeThreshold = 100 // 批量写入阈值
	BatchTimeThreshold = 30  // 批量写入时间阈值(秒)
	MaxRetryAttempts   = 3   // 最大重试次数
)

// 访问记录批次结构
type VisitBatch struct {
	Visits    []*ent.VisitorLog `json:"visits"`
	Count     int               `json:"count"`
	CreatedAt time.Time         `json:"created_at"`
}

// 高并发优化的访问记录方法
func (s *visitorStatService) RecordVisitOptimized(ctx context.Context, c *gin.Context, req *model.VisitorLogRequest) error {
	// 1. 立即更新Redis实时计数（毫秒级响应）
	if err := s.updateRealTimeCounts(ctx, c, req); err != nil {
		// 实时计数失败不影响用户体验，只记录日志
		fmt.Printf("实时计数更新失败: %v\n", err)
	}

	// 2. 异步批量写入数据库（不阻塞用户请求）
	go func() {
		if err := s.batchWriteVisit(ctx, c, req); err != nil {
			fmt.Printf("批量写入访问记录失败: %v\n", err)
		}
	}()

	return nil
}

// 更新实时计数
func (s *visitorStatService) updateRealTimeCounts(ctx context.Context, c *gin.Context, req *model.VisitorLogRequest) error {
	if s.cacheService == nil {
		return nil
	}

	now := time.Now()
	today := now.Format("2006-01-02")

	// 使用Redis原子操作增加计数
	viewsKey := CacheKeyRealTimeViews + today
	visitorsKey := CacheKeyRealTimeVisitors + today

	// 增加访问量
	if _, err := s.cacheService.Increment(ctx, viewsKey); err != nil {
		return fmt.Errorf("增加访问量失败: %w", err)
	}
	s.cacheService.Expire(ctx, viewsKey, CacheExpireRealTime)

	// 检查是否为新访客（基于IP和UserAgent的简单判断）
	// 注意：这里需要从gin.Context获取IP和UserAgent，因为req中没有这些字段
	clientIP := s.getClientIP(c)
	userAgent := c.GetHeader("User-Agent")
	visitorKey := fmt.Sprintf("stats:visitor:%s:%s", clientIP, userAgent)
	if exists, _ := s.cacheService.Get(ctx, visitorKey); exists == "" {
		// 新访客，增加访客数
		if _, err := s.cacheService.Increment(ctx, visitorsKey); err != nil {
			return fmt.Errorf("增加访客数失败: %w", err)
		}
		s.cacheService.Expire(ctx, visitorsKey, CacheExpireRealTime)

		// 标记访客已存在（24小时过期）
		s.cacheService.Set(ctx, visitorKey, "1", 24*time.Hour)
	}

	// 清除基础统计缓存，确保数据一致性
	s.cacheService.Delete(ctx, CacheKeyBasicStats)

	return nil
}

// 批量写入访问记录
func (s *visitorStatService) batchWriteVisit(ctx context.Context, c *gin.Context, req *model.VisitorLogRequest) error {
	// 1. 将访问记录添加到批量队列
	batchKey := CacheKeyBatchQueue + time.Now().Format("2006-01-02")

	// 创建访问日志
	userAgent := c.GetHeader("User-Agent")
	clientIP := s.getClientIP(c)
	visitorID := s.generateVisitorID(clientIP, userAgent)
	country, region, city := s.getGeoLocation(clientIP)
	browser, os, device := s.parseUserAgent(userAgent)

	log := &ent.VisitorLog{
		VisitorID: visitorID,
		IPAddress: clientIP,
		UserAgent: &userAgent,
		Referer:   &req.Referer,
		URLPath:   req.URLPath,
		Country:   &country,
		Region:    &region,
		City:      &city,
		Browser:   &browser,
		Os:        &os,
		Device:    &device,
		Duration:  req.Duration,
		IsBounce:  req.Duration < 10,
		CreatedAt: time.Now(),
	}

	// 2. 添加到批量队列
	if err := s.addToBatchQueue(ctx, batchKey, log); err != nil {
		return fmt.Errorf("添加到批量队列失败: %w", err)
	}

	// 3. 检查是否需要立即处理批次
	if shouldProcessBatch, err := s.shouldProcessBatch(ctx, batchKey); err == nil && shouldProcessBatch {
		return s.processBatchQueue(ctx, batchKey)
	}

	return nil
}

// 添加到批量队列
func (s *visitorStatService) addToBatchQueue(ctx context.Context, batchKey string, log *ent.VisitorLog) error {
	if s.cacheService == nil {
		return nil
	}

	// 使用Redis List结构存储批量数据
	logData, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("序列化访问日志失败: %w", err)
	}

	// 添加到队列尾部
	if err := s.cacheService.RPush(ctx, batchKey, string(logData)); err != nil {
		return fmt.Errorf("添加到批量队列失败: %w", err)
	}

	// 设置过期时间
	s.cacheService.Expire(ctx, batchKey, CacheExpireBatchQueue)

	return nil
}

// 检查是否应该处理批次
func (s *visitorStatService) shouldProcessBatch(ctx context.Context, batchKey string) (bool, error) {
	if s.cacheService == nil {
		return false, nil
	}

	// 检查队列长度
	length, err := s.cacheService.LLen(ctx, batchKey)
	if err != nil {
		return false, err
	}

	// 如果队列长度超过阈值，立即处理
	if length >= BatchSizeThreshold {
		return true, nil
	}

	// 检查队列中最早的数据时间
	firstItem, err := s.cacheService.LIndex(ctx, batchKey, 0)
	if err != nil || firstItem == "" {
		return false, nil
	}

	// 解析时间戳
	var log ent.VisitorLog
	if err := json.Unmarshal([]byte(firstItem), &log); err != nil {
		return false, nil
	}

	// 如果最早的数据超过时间阈值，立即处理
	if time.Since(log.CreatedAt) > time.Duration(BatchTimeThreshold)*time.Second {
		return true, nil
	}

	return false, nil
}

// 处理批量队列
func (s *visitorStatService) processBatchQueue(ctx context.Context, batchKey string) error {
	if s.cacheService == nil {
		return nil
	}

	// 1. 获取批次中的所有数据
	items, err := s.cacheService.LRange(ctx, batchKey, 0, -1)
	if err != nil {
		return fmt.Errorf("获取批量数据失败: %w", err)
	}

	if len(items) == 0 {
		return nil
	}

	// 2. 批量写入数据库
	var logs []*ent.VisitorLog
	for _, item := range items {
		var log ent.VisitorLog
		if err := json.Unmarshal([]byte(item), &log); err != nil {
			continue // 跳过无效数据
		}
		logs = append(logs, &log)
	}

	// 3. 批量创建访问日志（简化版本，逐个创建）
	if err := s.batchCreateVisitorLogs(ctx, logs); err != nil {
		return fmt.Errorf("批量创建访问日志失败: %w", err)
	}

	// 4. 更新URL统计（简化版本，逐个更新）
	if err := s.batchUpdateURLStats(ctx, logs); err != nil {
		return fmt.Errorf("批量更新URL统计失败: %w", err)
	}

	// 5. 清空队列
	s.cacheService.Del(ctx, batchKey)

	return nil
}

// 批量创建访问日志
func (s *visitorStatService) batchCreateVisitorLogs(ctx context.Context, logs []*ent.VisitorLog) error {
	// 分批处理，避免单次事务过大
	batchSize := 100
	for i := 0; i < len(logs); i += batchSize {
		end := i + batchSize
		if end > len(logs) {
			end = len(logs)
		}

		batch := logs[i:end]
		// 使用现有的Create方法逐个创建
		for _, log := range batch {
			if err := s.visitorLogRepo.Create(ctx, log); err != nil {
				return fmt.Errorf("创建访问日志失败: %w", err)
			}
		}
	}

	return nil
}

// 批量更新URL统计
func (s *visitorStatService) batchUpdateURLStats(ctx context.Context, logs []*ent.VisitorLog) error {
	// 统计每个URL的访问量和访客数
	urlStats := make(map[string]*struct {
		Views    int64
		Visitors map[string]bool
		Duration int64
		Count    int
	})

	for _, log := range logs {
		if urlStats[log.URLPath] == nil {
			urlStats[log.URLPath] = &struct {
				Views    int64
				Visitors map[string]bool
				Duration int64
				Count    int
			}{
				Visitors: make(map[string]bool),
			}
		}

		stats := urlStats[log.URLPath]
		stats.Views++
		stats.Visitors[log.VisitorID] = true
		stats.Duration += int64(log.Duration)
		stats.Count++
	}

	// 批量更新URL统计
	for urlPath, stats := range urlStats {
		uniqueVisitors := int64(len(stats.Visitors))
		avgDuration := stats.Duration / int64(stats.Count)

		// 使用现有的IncrementViews方法逐个更新
		if err := s.urlStatRepo.IncrementViews(ctx, urlPath, uniqueVisitors > 0, int(avgDuration)); err != nil {
			return fmt.Errorf("更新URL统计失败: %w", err)
		}
	}

	return nil
}

// 智能获取基础统计数据（支持高并发）
func (s *visitorStatService) GetBasicStatisticsOptimized(ctx context.Context) (*model.VisitorStatistics, error) {
	// 1. 尝试从缓存获取
	if s.cacheService != nil {
		cachedData, err := s.cacheService.Get(ctx, CacheKeyBasicStats)
		if err == nil && cachedData != "" {
			var stats model.VisitorStatistics
			if json.Unmarshal([]byte(cachedData), &stats) == nil {
				return &stats, nil
			}
		}
	}

	// 2. 从Redis实时计数获取今日数据
	stats := &model.VisitorStatistics{}
	if s.cacheService != nil {
		now := time.Now()
		today := now.Format("2006-01-02")

		// 获取实时访问量
		if todayViews, err := s.cacheService.Get(ctx, CacheKeyRealTimeViews+today); err == nil && todayViews != "" {
			if views, err := strconv.ParseInt(todayViews, 10, 64); err == nil {
				stats.TodayViews = views
			}
		}

		// 获取实时访客数
		if todayVisitors, err := s.cacheService.Get(ctx, CacheKeyRealTimeVisitors+today); err == nil && todayVisitors != "" {
			if visitors, err := strconv.ParseInt(todayVisitors, 10, 64); err == nil {
				stats.TodayVisitors = visitors
			}
		}

		// 如果Redis中有今日数据，从数据库获取其他数据
		if stats.TodayViews > 0 || stats.TodayVisitors > 0 {
			// 异步更新数据库统计（不阻塞读取）
			go func() {
				if err := s.updateDatabaseStats(ctx, stats); err != nil {
					fmt.Printf("异步更新数据库统计失败: %v\n", err)
				}
			}()

			// 写入缓存
			if data, err := json.Marshal(stats); err == nil {
				s.cacheService.Set(ctx, CacheKeyBasicStats, string(data), CacheExpireBasicStats)
			}

			return stats, nil
		}
	}

	// 3. 从数据库获取完整数据
	dbStats, err := s.visitorStatRepo.GetBasicStatistics(ctx)
	if err != nil {
		return nil, err
	}

	// 4. 合并Redis实时数据和数据库历史数据
	stats.TodayVisitors = dbStats.TodayVisitors
	stats.TodayViews = dbStats.TodayViews
	stats.YesterdayVisitors = dbStats.YesterdayVisitors
	stats.YesterdayViews = dbStats.YesterdayViews
	stats.MonthViews = dbStats.MonthViews
	stats.YearViews = dbStats.YearViews

	// 5. 写入缓存
	if s.cacheService != nil {
		if data, err := json.Marshal(stats); err == nil {
			s.cacheService.Set(ctx, CacheKeyBasicStats, string(data), CacheExpireBasicStats)
		}
	}

	return stats, nil
}

// 异步更新数据库统计
func (s *visitorStatService) updateDatabaseStats(ctx context.Context, stats *model.VisitorStatistics) error {
	// 这里可以实现异步更新逻辑，比如将统计数据写入消息队列
	// 然后由后台worker处理，避免阻塞主流程
	return nil
}

// 获取客户端真实IP
func (s *visitorStatService) getClientIP(c *gin.Context) string {
	// 检查代理头
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		// X-Forwarded-For 可能包含多个IP，取第一个
		if ips := strings.Split(ip, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}

	if ip := c.GetHeader("X-Original-Forwarded-For"); ip != "" {
		return ip
	}

	// 返回默认IP
	return c.ClientIP()
}

// 生成访客ID
func (s *visitorStatService) generateVisitorID(ip, userAgent string) string {
	hash := md5.Sum([]byte(ip + userAgent))
	return fmt.Sprintf("%x", hash)
}

// 获取地理位置信息
func (s *visitorStatService) getGeoLocation(ip string) (country, region, city string) {
	if s.geoipService == nil {
		return "未知", "未知", "未知"
	}

	location, err := s.geoipService.Lookup(ip)
	if err != nil {
		return "未知", "未知", "未知"
	}

	// 解析位置字符串，格式可能是 "省份 城市" 或 "城市" 或 "省份" 或 "国家"
	parts := strings.Split(strings.TrimSpace(location), " ")

	if len(parts) == 2 {
		// 格式: "省份 城市"
		return "未知", parts[0], parts[1]
	} else if len(parts) == 1 {
		// 格式: "城市" 或 "省份" 或 "国家"
		// 这里我们假设是城市，因为大多数情况下返回的是城市名
		return "未知", "未知", parts[0]
	}

	return "未知", "未知", "未知"
}

// 解析User-Agent
func (s *visitorStatService) parseUserAgent(userAgent string) (browser, os, device string) {
	// 这里可以使用第三方库来解析User-Agent，简化处理
	ua := strings.ToLower(userAgent)

	// 检测浏览器
	if strings.Contains(ua, "chrome") {
		browser = "Chrome"
	} else if strings.Contains(ua, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(ua, "safari") {
		browser = "Safari"
	} else if strings.Contains(ua, "edge") {
		browser = "Edge"
	} else {
		browser = "其他"
	}

	// 检测操作系统
	if strings.Contains(ua, "windows") {
		os = "Windows"
	} else if strings.Contains(ua, "mac") {
		os = "macOS"
	} else if strings.Contains(ua, "linux") {
		os = "Linux"
	} else if strings.Contains(ua, "android") {
		os = "Android"
	} else if strings.Contains(ua, "ios") {
		os = "iOS"
	} else {
		os = "其他"
	}

	// 检测设备类型
	if strings.Contains(ua, "mobile") {
		device = "手机"
	} else if strings.Contains(ua, "tablet") {
		device = "平板"
	} else {
		device = "桌面"
	}

	return browser, os, device
}
