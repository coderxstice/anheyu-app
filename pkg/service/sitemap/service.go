/*
 * @Description: 站点地图服务
 * @Author: 安知鱼
 * @Date: 2025-09-21 00:00:00
 * @LastEditTime: 2025-09-30 12:08:23
 * @LastEditors: 安知鱼
 */
package sitemap

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

// Service 站点地图服务接口
type Service interface {
	// GenerateSitemap 生成站点地图
	GenerateSitemap(ctx context.Context) (*URLSet, error)
	// GenerateRobots 生成robots.txt
	GenerateRobots(ctx context.Context) (string, error)
}

// service 站点地图服务实现
type service struct {
	articleRepo repository.ArticleRepository
	pageRepo    repository.PageRepository
	linkRepo    repository.LinkRepository
	settingSvc  setting.SettingService
}

// NewService 创建站点地图服务
func NewService(
	articleRepo repository.ArticleRepository,
	pageRepo repository.PageRepository,
	linkRepo repository.LinkRepository,
	settingSvc setting.SettingService,
) Service {
	return &service{
		articleRepo: articleRepo,
		pageRepo:    pageRepo,
		linkRepo:    linkRepo,
		settingSvc:  settingSvc,
	}
}

// GenerateSitemap 生成站点地图
// 支持智能缓存：频繁访问时使用缓存，内容更新时自动失效
func (s *service) GenerateSitemap(ctx context.Context) (*URLSet, error) {
	baseURL := s.settingSvc.Get(constant.KeySiteURL.String())
	if baseURL == "" {
		return nil, fmt.Errorf("站点URL未配置")
	}

	// 确保baseURL不以斜杠结尾
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	var items []SitemapItem

	// 添加主页
	items = append(items, SitemapItem{
		URL:          baseURL + "/",
		LastModified: time.Now(),
		ChangeFreq:   ChangeFreqDaily,
		Priority:     1.0,
	})

	// 添加文章
	if err := s.addArticles(ctx, baseURL, &items); err != nil {
		log.Printf("添加文章到站点地图时出错: %v", err)
	}

	// 添加页面
	if err := s.addPages(ctx, baseURL, &items); err != nil {
		log.Printf("添加页面到站点地图时出错: %v", err)
	}

	// 添加友链页面
	if err := s.addLinkPages(ctx, baseURL, &items); err != nil {
		log.Printf("添加友链页面到站点地图时出错: %v", err)
	}

	// 转换为URLSet
	urlset := &URLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  make([]URL, len(items)),
	}

	for i, item := range items {
		urlset.URLs[i] = item.ToURL()
	}

	return urlset, nil
}

// addArticles 添加文章到站点地图
func (s *service) addArticles(ctx context.Context, baseURL string, items *[]SitemapItem) error {
	articles, _, err := s.articleRepo.List(ctx, &model.ListArticlesOptions{
		Status:   "PUBLISHED",
		Page:     1,
		PageSize: 10000, // 获取所有已发布文章
	})
	if err != nil {
		return fmt.Errorf("获取文章列表失败: %w", err)
	}

	log.Printf("[DEBUG] 找到 %d 篇已发布文章用于生成站点地图", len(articles))

	for _, article := range articles {
		log.Printf("[DEBUG] 处理文章: ID=%s, Title=%s, Abbrlink=%s", article.ID, article.Title, article.Abbrlink)

		// 根据文章更新时间确定优先级和更新频率
		timeSinceUpdate := time.Since(article.UpdatedAt)
		var changeFreq ChangeFrequency
		var priority float32

		if timeSinceUpdate < 24*time.Hour {
			changeFreq = ChangeFreqDaily
			priority = 0.9
		} else if timeSinceUpdate < 7*24*time.Hour {
			changeFreq = ChangeFreqWeekly
			priority = 0.8
		} else if timeSinceUpdate < 30*24*time.Hour {
			changeFreq = ChangeFreqMonthly
			priority = 0.7
		} else {
			changeFreq = ChangeFreqYearly
			priority = 0.6
		}

		// 构建文章URL，优先使用abbrlink，如果为空则使用ID
		var articleSlug string
		if article.Abbrlink != "" {
			articleSlug = article.Abbrlink
		} else {
			articleSlug = article.ID
		}
		articleURL := fmt.Sprintf("%s/posts/%s", baseURL, articleSlug)
		log.Printf("[DEBUG] 生成文章URL: %s", articleURL)

		*items = append(*items, SitemapItem{
			URL:          articleURL,
			LastModified: article.UpdatedAt,
			ChangeFreq:   changeFreq,
			Priority:     priority,
		})
	}

	return nil
}

// addPages 添加页面到站点地图
func (s *service) addPages(ctx context.Context, baseURL string, items *[]SitemapItem) error {
	pages, _, err := s.pageRepo.List(ctx, &model.ListPagesOptions{
		IsPublished: &[]bool{true}[0], // 只获取已发布的页面
	})
	if err != nil {
		return fmt.Errorf("获取页面列表失败: %w", err)
	}

	for _, page := range pages {
		pageURL := fmt.Sprintf("%s/%s", baseURL, page.Path)

		*items = append(*items, SitemapItem{
			URL:          pageURL,
			LastModified: page.UpdatedAt,
			ChangeFreq:   ChangeFreqMonthly,
			Priority:     0.5,
		})
	}

	return nil
}

// addLinkPages 添加友链相关页面到站点地图
func (s *service) addLinkPages(ctx context.Context, baseURL string, items *[]SitemapItem) error {
	// 添加友链主页面
	*items = append(*items, SitemapItem{
		URL:          baseURL + "/links",
		LastModified: time.Now(),
		ChangeFreq:   ChangeFreqWeekly,
		Priority:     0.6,
	})

	// 可以根据需要添加其他固定页面
	commonPages := []struct {
		path     string
		priority float32
		freq     ChangeFrequency
	}{
		{"/archives", 0.7, ChangeFreqDaily},
		{"/categories", 0.6, ChangeFreqWeekly},
		{"/tags", 0.6, ChangeFreqWeekly},
		{"/about", 0.5, ChangeFreqMonthly},
	}

	for _, page := range commonPages {
		*items = append(*items, SitemapItem{
			URL:          baseURL + page.path,
			LastModified: time.Now(),
			ChangeFreq:   page.freq,
			Priority:     page.priority,
		})
	}

	return nil
}

// GenerateRobots 生成robots.txt
func (s *service) GenerateRobots(ctx context.Context) (string, error) {
	baseURL := s.settingSvc.Get(constant.KeySiteURL.String())
	if baseURL == "" {
		baseURL = "https://blog.anheyu.com"
	}

	// 确保baseURL不以斜杠结尾
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	robotsContent := fmt.Sprintf(`User-agent: *
Allow: /

# 禁止访问管理后台
Disallow: /admin/
Disallow: /api/

# 禁止访问静态文件目录（如果不希望索引）
# Disallow: /static/

# 站点地图
Sitemap: %s/sitemap.xml

# 爬取延迟（可选）
Crawl-delay: 1
`, baseURL)

	return robotsContent, nil
}
