/*
 * @Description: 搜索服务 - 搜索架构实现
 * @Author: 安知鱼
 * @Date: 2025-01-27 10:00:00
 * @LastEditTime: 2025-08-30 15:22:34
 * @LastEditors: 安知鱼
 */
package search

import (
	"context"
	"fmt"
	"log"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

// AppSearcher 全局搜索器实例
var AppSearcher model.Searcher

// SearchService 搜索服务
type SearchService struct {
	searcher model.Searcher
}

// NewSearchService 创建搜索服务实例
func NewSearchService() *SearchService {
	return &SearchService{
		searcher: AppSearcher,
	}
}

// Search 执行搜索
func (s *SearchService) Search(ctx context.Context, query string, page int, size int) (*model.SearchResult, error) {
	if s.searcher == nil {
		return nil, fmt.Errorf("搜索引擎未初始化")
	}
	return s.searcher.Search(ctx, query, page, size)
}

// IndexArticle 索引文章
func (s *SearchService) IndexArticle(ctx context.Context, article *model.Article) error {
	if s.searcher == nil {
		log.Println("[警告] 搜索引擎未初始化，跳过索引操作")
		return nil // 返回 nil 而不是错误，避免影响主流程
	}
	return s.searcher.IndexArticle(ctx, article)
}

// DeleteArticle 删除文章索引
func (s *SearchService) DeleteArticle(ctx context.Context, articleID string) error {
	if s.searcher == nil {
		log.Println("[警告] 搜索引擎未初始化，跳过删除索引操作")
		return nil // 返回 nil 而不是错误，避免影响主流程
	}
	return s.searcher.DeleteArticle(ctx, articleID)
}

// RebuildAllIndexes 重建所有文章的搜索索引
func (s *SearchService) RebuildAllIndexes(ctx context.Context) error {
	// 这里需要调用文章服务来获取所有文章
	// 由于存在循环依赖，我们通过全局变量来访问
	if s.searcher == nil || AppSearcher == nil {
		log.Println("[警告] 搜索引擎未初始化，跳过重建索引操作")
		return fmt.Errorf("搜索引擎未初始化")
	}

	// 获取所有文章的逻辑将在应用启动时实现
	log.Println("开始重建搜索索引...")

	// 清理所有现有的搜索索引
	if err := s.clearAllIndexes(ctx); err != nil {
		return fmt.Errorf("清理现有索引失败: %w", err)
	}

	log.Println("搜索索引清理完成，等待文章数据重建...")
	return nil
}

// clearAllIndexes 清理所有现有的搜索索引
func (s *SearchService) clearAllIndexes(ctx context.Context) error {
	if s.searcher == nil {
		return fmt.Errorf("搜索引擎未初始化")
	}

	// 检查搜索器类型
	redisSearcher, ok := s.searcher.(*RedisSearcher)
	if !ok {
		// 如果不是 Redis 搜索器，跳过清理操作（Simple 搜索器不需要清理）
		log.Println("当前使用简单搜索模式，无需清理索引")
		return nil
	}

	// 获取所有以 "anheyu:search:" 开头的键
	pattern := KeyNamespace + "search:*"
	keys, err := redisSearcher.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("获取搜索索引键失败: %w", err)
	}

	if len(keys) > 0 {
		// 批量删除所有搜索相关的键
		pipe := redisSearcher.client.Pipeline()
		for _, key := range keys {
			pipe.Del(ctx, key)
		}

		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("删除搜索索引失败: %w", err)
		}

		log.Printf("已清理 %d 个搜索索引键", len(keys))
	}

	return nil
}

// InitializeSearchEngine 初始化搜索引擎（支持自动降级）
func InitializeSearchEngine(settingSvc setting.SettingService) error {
	// 尝试使用 Redis 搜索模式
	redisSearcher, err := NewRedisSearcher(settingSvc)
	if err != nil {
		return fmt.Errorf("Redis 搜索初始化失败: %w", err)
	}

	if redisSearcher != nil {
		// Redis 可用，使用 Redis 搜索
		AppSearcher = redisSearcher
		log.Println("✅ Redis 搜索模式已启用")
		return nil
	}

	// Redis 不可用，降级到简单搜索模式
	simpleSearcher := NewSimpleSearcher(settingSvc)
	AppSearcher = simpleSearcher
	log.Println("✅ 简单搜索模式已启用（降级方案）")
	return nil
}
