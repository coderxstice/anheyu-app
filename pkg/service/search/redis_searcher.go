/*
 * @Description: Redis 搜索器实现，包含优化后的分词和搜索逻辑
 * @Author: 安知鱼
 * @Date: 2025-08-30 14:01:22
 * @LastEditTime: 2025-08-30 15:37:40
 * @LastEditors: 安知鱼
 */

package search

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisSearcher 使用 Redis 实现的搜索器
type RedisSearcher struct {
	client *redis.Client
}

var (
	reNonAlphanumeric = regexp.MustCompile(`[^\p{L}\p{N}]+`)
)

// NewRedisSearcher 创建新的 Redis 搜索器
func NewRedisSearcher() (*RedisSearcher, error) {
	redisAddr := os.Getenv("ANHEYU_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // 默认值
	}

	redisPassword := os.Getenv("ANHEYU_REDIS_PASSWORD")
	redisDB := 0 // 默认使用 0 号数据库

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	return &RedisSearcher{
		client: rdb,
	}, nil
}

func tokenize(text string) []string {
	lowerText := strings.ToLower(text)
	cleanedText := reNonAlphanumeric.ReplaceAllString(lowerText, " ")
	parts := strings.Fields(cleanedText)
	if len(parts) == 0 {
		return []string{}
	}
	seen := make(map[string]bool)
	var finalResult []string
	for _, part := range parts {
		if isLatin(part) {
			if !seen[part] {
				finalResult = append(finalResult, part)
				seen[part] = true
			}
		} else {
			runes := []rune(part)
			for _, r := range runes {
				char := string(r)
				if !seen[char] {
					finalResult = append(finalResult, char)
					seen[char] = true
				}
			}
			if len(runes) > 1 {
				for i := 0; i < len(runes)-1; i++ {
					bigram := string(runes[i : i+2])
					if !seen[bigram] {
						finalResult = append(finalResult, bigram)
						seen[bigram] = true
					}
				}
			}
		}
	}
	return finalResult
}

func isLatin(s string) bool {
	for _, r := range s {
		if r > unicode.MaxLatin1 {
			return false
		}
	}
	return true
}

// Search 执行搜索
func (rs *RedisSearcher) Search(ctx context.Context, query string, page int, size int) (*model.SearchResult, error) {
	words := tokenize(query)
	if len(words) == 0 {
		return &model.SearchResult{
			Pagination: &model.SearchPagination{Total: 0, Page: page, Size: size, TotalPages: 0},
			Hits:       []*model.SearchHit{},
		}, nil
	}

	indexKeys := make([]string, len(words))
	for i, word := range words {
		indexKeys[i] = fmt.Sprintf("search:index:%s", word)
	}

	tempResultKey := fmt.Sprintf("search:result:%s", uuid.New().String())
	_, err := rs.client.SInterStore(ctx, tempResultKey, indexKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("计算搜索结果交集失败: %w", err)
	}
	rs.client.Expire(ctx, tempResultKey, 10*time.Minute)

	total, err := rs.client.SCard(ctx, tempResultKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取搜索结果总数失败: %w", err)
	}

	if total == 0 {
		return &model.SearchResult{
			Pagination: &model.SearchPagination{Total: 0, Page: page, Size: size, TotalPages: 0},
			Hits:       []*model.SearchHit{},
		}, nil
	}

	start := (page - 1) * size
	sortCmd := &redis.Sort{
		By:     "search:article:*->publish_timestamp",
		Offset: int64(start),
		Count:  int64(size),
		Order:  "DESC",
		Get: []string{
			"#",
			"search:article:*->title",
			"search:article:*->content",
			"search:article:*->author",
			"search:article:*->category",
			"search:article:*->tags",
			"search:article:*->publish_date", // 仍然获取字符串格式用于显示
			"search:article:*->cover_url",
			"search:article:*->abbrlink",
			"search:article:*->view_count",
			"search:article:*->word_count",
			"search:article:*->reading_time",
		},
	}

	data, err := rs.client.Sort(ctx, tempResultKey, sortCmd).Result()
	if err != nil {
		// 加上原始错误信息，方便调试
		return nil, fmt.Errorf("在Redis中排序分页失败: %w", err)
	}

	searchHits := make([]*model.SearchHit, 0, len(data)/12)
	for i := 0; i < len(data); i += 12 {
		hit := &model.SearchHit{ID: data[i]}
		hit.Title = data[i+1]
		content := data[i+2]
		if len(content) > 150 {
			hit.Snippet = string([]rune(content)[:150]) + "..."
		} else {
			hit.Snippet = content
		}
		hit.Author = data[i+3]
		hit.Category = data[i+4]
		hit.Tags = strings.Split(data[i+5], ",")
		if pTime, err := time.Parse(time.RFC3339, data[i+6]); err == nil {
			hit.PublishDate = pTime
		}
		hit.CoverURL = data[i+7]
		hit.Abbrlink = data[i+8]
		fmt.Sscanf(data[i+9], "%d", &hit.ViewCount)
		fmt.Sscanf(data[i+10], "%d", &hit.WordCount)
		fmt.Sscanf(data[i+11], "%d", &hit.ReadingTime)
		searchHits = append(searchHits, hit)
	}

	totalPages := (int(total) + size - 1) / size
	result := &model.SearchResult{
		Pagination: &model.SearchPagination{
			Total:      total,
			Page:       page,
			Size:       size,
			TotalPages: totalPages,
		},
		Hits: searchHits,
	}

	return result, nil
}

// IndexArticle 索引文章
func (rs *RedisSearcher) IndexArticle(ctx context.Context, article *model.Article) error {
	indexedArticle := &model.IndexedArticle{
		ID:          article.ID,
		Title:       article.Title,
		Content:     article.ContentHTML,
		Author:      "安知鱼",
		PublishDate: article.CreatedAt,
		CoverURL:    article.CoverURL,
		Abbrlink:    article.Abbrlink,
		ViewCount:   article.ViewCount,
		WordCount:   article.WordCount,
		ReadingTime: article.ReadingTime,
		Status:      article.Status,
		CreatedAt:   article.CreatedAt,
		UpdatedAt:   article.UpdatedAt,
	}

	if len(article.PostCategories) > 0 {
		indexedArticle.Category = article.PostCategories[0].Name
	}
	tags := make([]string, len(article.PostTags))
	for i, tag := range article.PostTags {
		tags[i] = tag.Name
	}
	indexedArticle.Tags = tags

	articleKey := fmt.Sprintf("search:article:%s", article.ID)
	// 额外添加一个 "publish_timestamp" 字段，值为Unix时间戳
	articleData := map[string]interface{}{
		"id":                indexedArticle.ID,
		"title":             indexedArticle.Title,
		"content":           indexedArticle.Content,
		"author":            indexedArticle.Author,
		"category":          indexedArticle.Category,
		"tags":              strings.Join(indexedArticle.Tags, ","),
		"publish_date":      indexedArticle.PublishDate.Format(time.RFC3339),
		"publish_timestamp": indexedArticle.PublishDate.Unix(),
		"cover_url":         indexedArticle.CoverURL,
		"abbrlink":          indexedArticle.Abbrlink,
		"view_count":        indexedArticle.ViewCount,
		"word_count":        indexedArticle.WordCount,
		"reading_time":      indexedArticle.ReadingTime,
		"status":            indexedArticle.Status,
		"created_at":        indexedArticle.CreatedAt.Format(time.RFC3339),
		"updated_at":        indexedArticle.UpdatedAt.Format(time.RFC3339),
	}

	pipe := rs.client.Pipeline()
	pipe.HSet(ctx, articleKey, articleData)

	oldWordsKey := fmt.Sprintf("search:words:%s", article.ID)
	oldWords, _ := rs.client.SMembers(ctx, oldWordsKey).Result()
	for _, word := range oldWords {
		indexKey := fmt.Sprintf("search:index:%s", word)
		pipe.SRem(ctx, indexKey, article.ID)
	}

	searchText := indexedArticle.Title + " " + indexedArticle.Content
	newWords := tokenize(searchText)

	for _, word := range newWords {
		indexKey := fmt.Sprintf("search:index:%s", word)
		pipe.SAdd(ctx, indexKey, article.ID)
	}

	pipe.Del(ctx, oldWordsKey)
	if len(newWords) > 0 {
		newWordsInterface := make([]interface{}, len(newWords))
		for i, v := range newWords {
			newWordsInterface[i] = v
		}
		pipe.SAdd(ctx, oldWordsKey, newWordsInterface...)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("索引文章 %s 失败: %w", article.ID, err)
	}

	return nil
}

// DeleteArticle 删除文章索引
func (rs *RedisSearcher) DeleteArticle(ctx context.Context, articleID string) error {
	pipe := rs.client.Pipeline()

	articleKey := fmt.Sprintf("search:article:%s", articleID)
	pipe.Del(ctx, articleKey)

	oldWordsKey := fmt.Sprintf("search:words:%s", articleID)
	oldWords, err := rs.client.SMembers(ctx, oldWordsKey).Result()
	if err != nil && err != redis.Nil {
		log.Printf("警告: 获取文章 %s 的旧索引词失败: %v", articleID, err)
	}
	for _, word := range oldWords {
		indexKey := fmt.Sprintf("search:index:%s", word)
		pipe.SRem(ctx, indexKey, articleID)
	}
	pipe.Del(ctx, oldWordsKey)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("删除文章索引 %s 失败: %w", articleID, err)
	}

	return nil
}

// HealthCheck 健康检查
func (rs *RedisSearcher) HealthCheck(ctx context.Context) error {
	return rs.client.Ping(ctx).Err()
}
