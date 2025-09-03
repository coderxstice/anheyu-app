// anheyu-app/pkg/service/article/service.go
package article

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/anzhiyu-c/anheyu-app/internal/app/task"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/direct_link"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/file"
	appParser "github.com/anzhiyu-c/anheyu-app/pkg/service/parser"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/search"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/utility"
)

type Service interface {
	UploadArticleImage(ctx context.Context, ownerID uint, fileReader io.Reader, originalFilename string) (fileURL string, publicFileID string, err error)
	Create(ctx context.Context, req *model.CreateArticleRequest, ip string) (*model.ArticleResponse, error)
	Get(ctx context.Context, publicID string) (*model.ArticleResponse, error)
	Update(ctx context.Context, publicID string, req *model.UpdateArticleRequest, ip string) (*model.ArticleResponse, error)
	Delete(ctx context.Context, publicID string) error
	List(ctx context.Context, options *model.ListArticlesOptions) (*model.ArticleListResponse, error)
	GetPublicBySlugOrID(ctx context.Context, slugOrID string) (*model.ArticleDetailResponse, error)
	ListPublic(ctx context.Context, options *model.ListPublicArticlesOptions) (*model.ArticleListResponse, error)
	ListHome(ctx context.Context) ([]model.ArticleResponse, error)
	ListArchives(ctx context.Context) (*model.ArchiveSummaryResponse, error)
	GetRandom(ctx context.Context) (*model.ArticleResponse, error)
	ToAPIResponse(a *model.Article, useAbbrlinkAsID bool, includeHTML bool) *model.ArticleResponse
}

type serviceImpl struct {
	repo             repository.ArticleRepository
	postTagRepo      repository.PostTagRepository
	postCategoryRepo repository.PostCategoryRepository
	txManager        repository.TransactionManager
	cacheSvc         utility.CacheService
	geoService       utility.GeoIPService
	broker           *task.Broker
	settingSvc       setting.SettingService
	httpClient       *http.Client
	parserSvc        *appParser.Service
	fileSvc          file.FileService
	directLinkSvc    direct_link.Service
	searchSvc        *search.SearchService
	colorSvc         *utility.ColorService
}

func NewService(
	repo repository.ArticleRepository,
	postTagRepo repository.PostTagRepository,
	postCategoryRepo repository.PostCategoryRepository,
	txManager repository.TransactionManager,
	cacheSvc utility.CacheService,
	geoService utility.GeoIPService,
	broker *task.Broker,
	settingSvc setting.SettingService,
	parserSvc *appParser.Service,
	fileSvc file.FileService,
	directLinkSvc direct_link.Service,
	searchSvc *search.SearchService,
) Service {
	return &serviceImpl{
		repo:             repo,
		postTagRepo:      postTagRepo,
		postCategoryRepo: postCategoryRepo,
		txManager:        txManager,
		cacheSvc:         cacheSvc,
		geoService:       geoService,
		broker:           broker,
		settingSvc:       settingSvc,
		httpClient:       &http.Client{Timeout: 10 * time.Second},
		parserSvc:        parserSvc,
		fileSvc:          fileSvc,
		directLinkSvc:    directLinkSvc,
		searchSvc:        searchSvc,
		colorSvc:         utility.NewColorService(),
	}
}

// UploadArticleImage 处理文章图片的上传，并为其创建直链。
func (s *serviceImpl) UploadArticleImage(ctx context.Context, ownerID uint, fileReader io.Reader, originalFilename string) (string, string, error) {
	ext := path.Ext(originalFilename)
	uniqueFilename := strconv.FormatInt(time.Now().UnixNano(), 10) + ext

	log.Printf("[文章图片上传] 准备将 '%s' 作为 '%s' 上传到文章存储策略", originalFilename, uniqueFilename)
	fileItem, err := s.fileSvc.UploadFileByPolicyFlag(ctx, ownerID, fileReader, constant.PolicyFlagArticleImage, uniqueFilename)
	if err != nil {
		log.Printf("[文章图片上传] 调用 fileSvc.UploadFileByPolicyFlag 失败: %v", err)
		return "", "", fmt.Errorf("文件上传到系统策略失败: %w", err)
	}
	log.Printf("[文章图片上传] 文件上传成功，新文件公共ID: %s", fileItem.ID)

	// 将文件的公共ID(string)解码为数据库ID(uint)
	dbFileID, _, err := idgen.DecodePublicID(fileItem.ID)
	if err != nil {
		log.Printf("[文章图片上传] 解码文件公共ID '%s' 失败: %v", fileItem.ID, err)
		return "", "", fmt.Errorf("无效的文件ID: %w", err)
	}

	// 3. 为上传成功的文件创建直链
	linksMap, err := s.directLinkSvc.GetOrCreateDirectLinks(ctx, ownerID, []uint{dbFileID})
	if err != nil {
		log.Printf("[文章图片上传] 为文件 %d 创建直链时发生错误: %v", dbFileID, err)
		return "", "", fmt.Errorf("创建直链失败: %w", err)
	}

	// 4. 从 map 中通过文件数据库ID获取对应的结果
	linkResult, ok := linksMap[dbFileID]
	if !ok || linkResult.URL == "" {
		log.Printf("[文章图片上传] directLinkSvc 未能返回文件 %d 的直链结果", dbFileID)
		return "", "", fmt.Errorf("获取直链URL失败")
	}

	// 5. 直接使用 service 返回的、已经构建好的完整 URL
	finalURL := linkResult.URL
	log.Printf("[文章图片上传] 成功获取最终直链URL: %s", finalURL)

	return finalURL, fileItem.ID, nil
}

func (s *serviceImpl) determinePrimaryColor(ctx context.Context, topImgURL, coverURL string) string {
	const defaultColor = "#b4bfe2"

	imageURLToUse := ""
	if topImgURL != "" {
		imageURLToUse = topImgURL
	} else if coverURL != "" {
		imageURLToUse = coverURL
	}

	if imageURLToUse == "" {
		return defaultColor
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURLToUse, nil)
	if err != nil {
		log.Printf("[警告] 获取主色调失败，创建图片请求失败。图片URL: %s, 错误: %v", imageURLToUse, err)
		return defaultColor
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[警告] 获取主色调失败，请求图片失败。图片URL: %s, 错误: %v", imageURLToUse, err)
		return defaultColor
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[警告] 获取主色调失败，图片URL返回非200状态码: %d。图片URL: %s", resp.StatusCode, imageURLToUse)
		return defaultColor
	}

	color, err := s.colorSvc.GetPrimaryColor(resp.Body)
	if err != nil {
		log.Printf("[警告] 获取主色调失败，颜色提取失败。图片URL: %s, 错误: %v", imageURLToUse, err)
		return defaultColor
	}

	log.Printf("[信息] 成功从图片 %s 获取主色调: %s", imageURLToUse, color)
	return color
}

// updateSiteStatsInBackground 异步更新全站的文章和字数统计配置。
func (s *serviceImpl) updateSiteStatsInBackground() {
	go func() {
		ctx := context.Background()
		stats, err := s.repo.GetSiteStats(ctx)
		if err != nil {
			log.Printf("[错误] updateSiteStats: 无法获取站点统计数据: %v", err)
			return
		}

		settingsToUpdate := make(map[string]string)

		postCountKey := constant.KeySidebarSiteInfoTotalPostCount.String()
		currentPostCountStr := s.settingSvc.Get(postCountKey)
		if currentPostCountStr != "-1" {
			settingsToUpdate[postCountKey] = strconv.Itoa(stats.TotalPosts)
		} else {
			log.Printf("[信息] 跳过文章总数更新，因为其在后台被设置为禁用 (-1)。")
		}

		wordCountKey := constant.KeySidebarSiteInfoTotalWordCount.String()
		currentWordCountStr := s.settingSvc.Get(wordCountKey)
		if currentWordCountStr != "-1" {
			settingsToUpdate[wordCountKey] = strconv.Itoa(stats.TotalWords)
		} else {
			log.Printf("[信息] 跳过全站字数更新，因为其在后台被设置为禁用 (-1)。")
		}

		if len(settingsToUpdate) > 0 {
			if err := s.settingSvc.UpdateSettings(ctx, settingsToUpdate); err != nil {
				log.Printf("[错误] updateSiteStats: 更新站点统计配置失败: %v", err)
			} else {
				log.Printf("[信息] 站点统计已更新：%v", settingsToUpdate)
			}
		} else {
			log.Printf("[信息] 无需更新站点统计，所有项均被禁用。")
		}
	}()
}

// calculatePostStats 是一个私有辅助函数，用于从 Markdown 内容计算字数和预计阅读时长。
func calculatePostStats(content string) (wordCount, readingTime int) {
	chineseCharCount := 0
	for _, r := range content {
		if unicode.Is(unicode.Han, r) {
			chineseCharCount++
		}
	}
	englishWordCount := len(strings.Fields(content))
	wordCount = chineseCharCount + englishWordCount
	const wordsPerMinute = 200
	if wordCount > 0 {
		readingTime = int(math.Ceil(float64(wordCount) / wordsPerMinute))
	}
	if readingTime == 0 && wordCount > 0 {
		readingTime = 1
	}
	return wordCount, readingTime
}

// ToAPIResponse 将领域模型转换为用于API响应的DTO。
func (s *serviceImpl) ToAPIResponse(a *model.Article, useAbbrlinkAsID bool, includeHTML bool) *model.ArticleResponse {
	if a == nil {
		return nil
	}
	tags := make([]*model.PostTagResponse, len(a.PostTags))
	for i, t := range a.PostTags {
		tags[i] = &model.PostTagResponse{ID: t.ID, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, Name: t.Name, Count: t.Count}
	}
	categories := make([]*model.PostCategoryResponse, len(a.PostCategories))
	for i, c := range a.PostCategories {
		categories[i] = &model.PostCategoryResponse{ID: c.ID, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, Name: c.Name, Description: c.Description, Count: c.Count, IsSeries: c.IsSeries}
	}

	responseID := a.ID
	if useAbbrlinkAsID && a.Abbrlink != "" {
		responseID = a.Abbrlink
	}

	effectiveTopImgURL := a.TopImgURL
	if effectiveTopImgURL == "" {
		effectiveTopImgURL = a.CoverURL
	}

	resp := &model.ArticleResponse{
		ID:                   responseID,
		CreatedAt:            a.CreatedAt,
		UpdatedAt:            a.UpdatedAt,
		Title:                a.Title,
		ContentMd:            a.ContentMd,
		CoverURL:             a.CoverURL,
		Status:               a.Status,
		ViewCount:            a.ViewCount,
		WordCount:            a.WordCount,
		ReadingTime:          a.ReadingTime,
		IPLocation:           a.IPLocation,
		PrimaryColor:         a.PrimaryColor,
		IsPrimaryColorManual: a.IsPrimaryColorManual,
		PostTags:             tags,
		PostCategories:       categories,
		HomeSort:             a.HomeSort,
		PinSort:              a.PinSort,
		TopImgURL:            effectiveTopImgURL,
		Summaries:            a.Summaries,
		Abbrlink:             a.Abbrlink,
		Copyright:            a.Copyright,
		CopyrightAuthor:      a.CopyrightAuthor,
		CopyrightAuthorHref:  a.CopyrightAuthorHref,
		CopyrightURL:         a.CopyrightURL,
	}

	if includeHTML {
		resp.ContentHTML = a.ContentHTML
	}
	return resp
}

// 将领域模型转换为简化的 API 响应
func toSimpleAPIResponse(a *model.Article) *model.SimpleArticleResponse {
	if a == nil {
		return nil
	}
	responseID := a.ID
	if a.Abbrlink != "" {
		responseID = a.Abbrlink
	}
	return &model.SimpleArticleResponse{ID: responseID, Title: a.Title, CoverURL: a.CoverURL, Abbrlink: a.Abbrlink, CreatedAt: a.CreatedAt}
}

// getCacheKey 生成文章渲染结果的 Redis 缓存键。
func (s *serviceImpl) getCacheKey(publicID string) string {
	return fmt.Sprintf("article:html:%s", publicID)
}

// GetPublicBySlugOrID 为公开浏览，通过 slug 或 ID 获取单篇文章，并处理浏览量。
func (s *serviceImpl) GetPublicBySlugOrID(ctx context.Context, slugOrID string) (*model.ArticleDetailResponse, error) {
	article, err := s.repo.GetBySlugOrID(ctx, slugOrID)
	if err != nil {
		return nil, err
	}

	currentArticleDbID, _, _ := idgen.DecodePublicID(article.ID)

	var wg sync.WaitGroup
	var chronoPrev, chronoNext *model.Article
	var relatedArticles []*model.Article
	var prevErr, nextErr, relatedErr error

	wg.Add(3)

	go func() {
		defer wg.Done()
		chronoPrev, prevErr = s.repo.GetPrevArticle(ctx, currentArticleDbID, article.CreatedAt)
	}()

	go func() {
		defer wg.Done()
		chronoNext, nextErr = s.repo.GetNextArticle(ctx, currentArticleDbID, article.CreatedAt)
	}()

	go func() {
		defer wg.Done()
		relatedArticles, relatedErr = s.repo.FindRelatedArticles(ctx, article, 2)
	}()

	viewCacheKey := s.getArticleViewCacheKey(article.ID)
	go func() {
		if _, err := s.cacheSvc.Increment(context.Background(), viewCacheKey); err != nil {
			log.Printf("[错误] 无法在 Redis 中为文章 %s 增加浏览次数: %v", article.ID, err)
		}
	}()

	redisIncrStr, err := s.cacheSvc.Get(ctx, viewCacheKey)
	if err != nil {
		log.Printf("[警告] 无法从 Redis 获取文章 %s 的增量浏览量: %v。将只返回数据库中的值。", article.ID, err)
	}

	var redisIncr int
	if redisIncrStr != "" {
		val, convErr := strconv.Atoi(redisIncrStr)
		if convErr == nil {
			redisIncr = val
		}
	}
	article.ViewCount += redisIncr

	wg.Wait()

	if prevErr != nil {
		log.Printf("[警告] 获取上一篇文章失败: %v", prevErr)
	}
	if nextErr != nil {
		log.Printf("[警告] 获取下一篇文章失败: %v", nextErr)
	}
	if relatedErr != nil {
		log.Printf("[警告] 获取相关文章失败: %v", relatedErr)
	}

	var finalPrevArticle, finalNextArticle *model.Article
	if chronoPrev == nil {
		log.Printf("[信息] GetPublicBySlugOrID: 当前是最早文章 (ID: %s)。规则：'上一篇'显示时间上的下一篇, '下一篇'为null。", article.ID)
		finalPrevArticle = chronoNext
		finalNextArticle = nil
	} else {
		log.Printf("[信息] GetPublicBySlugOrID: 当前不是最早文章 (ID: %s)。规则：'下一篇'显示时间上的上一篇, '上一篇'为null。", article.ID)
		finalNextArticle = chronoPrev
		finalPrevArticle = nil
	}

	mainArticleResponse := s.ToAPIResponse(article, true, true)
	relatedResponses := make([]*model.SimpleArticleResponse, 0, len(relatedArticles))
	for _, rel := range relatedArticles {
		relatedResponses = append(relatedResponses, toSimpleAPIResponse(rel))
	}

	detailResponse := &model.ArticleDetailResponse{
		ArticleResponse: *mainArticleResponse,
		PrevArticle:     toSimpleAPIResponse(finalPrevArticle),
		NextArticle:     toSimpleAPIResponse(finalNextArticle),
		RelatedArticles: relatedResponses,
	}

	return detailResponse, nil
}

// Create 处理创建新文章的完整业务流程。
func (s *serviceImpl) Create(ctx context.Context, req *model.CreateArticleRequest, ip string) (*model.ArticleResponse, error) {
	var newArticle *model.Article
	sanitizedHTML := s.parserSvc.SanitizeHTML(req.ContentHTML)

	err := s.txManager.Do(ctx, func(repos repository.Repositories) error {
		wordCount, readingTime := calculatePostStats(req.ContentMd)

		var ipLocation string
		if req.IPLocation != "" {
			ipLocation = req.IPLocation
		} else {
			ipLocation = "未知"
			if ip != "" && s.geoService != nil {
				location, err := s.geoService.Lookup(ip)
				if err == nil {
					ipLocation = location
				} else {
					log.Printf("创建文章时自动获取 IP 属地失败: %v", err)
				}
			}
		}
		tagDBIDs, err := idgen.DecodePublicIDBatch(req.PostTagIDs)
		if err != nil {
			return err
		}
		categoryDBIDs, err := idgen.DecodePublicIDBatch(req.PostCategoryIDs)
		if err != nil {
			return err
		}
		// 如果文章关联了多个分类，检查其中是否包含“系列”
		if len(categoryDBIDs) > 1 {
			isSeries, err := repos.PostCategory.FindAnySeries(ctx, categoryDBIDs)
			if err != nil {
				return fmt.Errorf("检查系列分类失败: %w", err)
			}
			if isSeries {
				return errors.New("系列分类不能与其他分类同时选择")
			}
		}
		coverURL := req.CoverURL
		if coverURL == "" {
			coverURL = s.settingSvc.Get(constant.KeyPostDefaultCover.String())
		}

		var primaryColor string
		isManual := false
		if req.IsPrimaryColorManual != nil && *req.IsPrimaryColorManual {
			isManual = true
			primaryColor = req.PrimaryColor
		} else {
			primaryColor = s.determinePrimaryColor(ctx, req.TopImgURL, coverURL)
		}

		copyright := true
		if req.Copyright != nil {
			copyright = *req.Copyright
		}

		params := &model.CreateArticleParams{
			Title:                req.Title,
			ContentMd:            req.ContentMd, // 存储Markdown原文
			ContentHTML:          sanitizedHTML, // 存储安全过滤后的HTML
			CoverURL:             coverURL,
			Status:               req.Status,
			PostTagIDs:           tagDBIDs,
			PostCategoryIDs:      categoryDBIDs,
			WordCount:            wordCount,
			ReadingTime:          readingTime,
			IPLocation:           ipLocation,
			HomeSort:             req.HomeSort,
			PinSort:              req.PinSort,
			TopImgURL:            req.TopImgURL,
			Summaries:            req.Summaries,
			PrimaryColor:         primaryColor,
			IsPrimaryColorManual: isManual,
			Abbrlink:             req.Abbrlink,
			Copyright:            copyright,
			CopyrightAuthor:      req.CopyrightAuthor,
			CopyrightAuthorHref:  req.CopyrightAuthorHref,
			CopyrightURL:         req.CopyrightURL,
		}
		createdArticle, err := repos.Article.Create(ctx, params)
		if err != nil {
			return err
		}
		newArticle = createdArticle
		if err := repos.PostTag.UpdateCount(ctx, tagDBIDs, nil); err != nil {
			return fmt.Errorf("更新标签计数失败: %w", err)
		}
		if err := repos.PostCategory.UpdateCount(ctx, categoryDBIDs, nil); err != nil {
			return fmt.Errorf("更新分类计数失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.updateSiteStatsInBackground()

	// 异步更新搜索索引
	go func() {
		if err := s.searchSvc.IndexArticle(context.Background(), newArticle); err != nil {
			log.Printf("[警告] 更新搜索索引失败: %v", err)
		}
	}()

	return s.ToAPIResponse(newArticle, false, false), nil
}

// Get 根据公共ID检索单个文章。
func (s *serviceImpl) Get(ctx context.Context, publicID string) (*model.ArticleResponse, error) {
	article, err := s.repo.GetByID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	return s.ToAPIResponse(article, false, false), nil
}

// getArticleViewCacheKey 生成文章浏览量在 Redis 中的缓存键。
func (s *serviceImpl) getArticleViewCacheKey(publicID string) string {
	return fmt.Sprintf("article:view_count:%s", publicID)
}

// GetPublicByID (此方法似乎与 GetPublicBySlugOrID 功能重叠，暂时保留)
func (s *serviceImpl) GetPublicByID(ctx context.Context, publicID string) (*model.ArticleResponse, error) {
	viewCacheKey := s.getArticleViewCacheKey(publicID)
	go func() {
		if _, err := s.cacheSvc.Increment(context.Background(), viewCacheKey); err != nil {
			log.Printf("[错误] 无法在 Redis 中为文章 %s 增加浏览次数: %v", publicID, err)
		}
	}()

	article, err := s.repo.GetByID(ctx, publicID)
	if err != nil {
		return nil, err
	}

	redisIncrStr, err := s.cacheSvc.Get(ctx, viewCacheKey)
	if err != nil {
		log.Printf("[警告] 无法从 Redis 获取文章 %s 的增量浏览量: %v。将只返回数据库中的值。", publicID, err)
	}

	var redisIncr int
	if redisIncrStr != "" {
		val, convErr := strconv.Atoi(redisIncrStr)
		if convErr == nil {
			redisIncr = val
		}
	}

	article.ViewCount = article.ViewCount + redisIncr

	return s.ToAPIResponse(article, true, true), nil
}

// Update 处理更新文章的业务逻辑。
func (s *serviceImpl) Update(ctx context.Context, publicID string, req *model.UpdateArticleRequest, ip string) (*model.ArticleResponse, error) {
	var updatedArticle *model.Article

	err := s.txManager.Do(ctx, func(repos repository.Repositories) error {
		oldArticle, err := repos.Article.GetByID(ctx, publicID)
		if err != nil {
			return err
		}
		oldTagIDs := make([]uint, len(oldArticle.PostTags))
		for i, t := range oldArticle.PostTags {
			oldTagIDs[i], _, _ = idgen.DecodePublicID(t.ID)
		}
		oldCategoryIDs := make([]uint, len(oldArticle.PostCategories))
		for i, c := range oldArticle.PostCategories {
			oldCategoryIDs[i], _, _ = idgen.DecodePublicID(c.ID)
		}

		var newCategoryDBIDs []uint
		if req.PostCategoryIDs != nil {
			var err error
			newCategoryDBIDs, err = idgen.DecodePublicIDBatch(req.PostCategoryIDs)
			if err != nil {
				return fmt.Errorf("无效的分类ID: %w", err)
			}

			// 如果文章被分配到多个分类，则检查其中是否包含“系列”分类
			if len(newCategoryDBIDs) > 1 {
				isSeries, err := repos.PostCategory.FindAnySeries(ctx, newCategoryDBIDs)
				if err != nil {
					return fmt.Errorf("检查系列分类失败: %w", err)
				}
				if isSeries {
					return errors.New("系列分类不能与其他分类同时选择")
				}
			}
		}

		var computedParams model.UpdateArticleComputedParams

		// 如果 Markdown 内容有更新，则重新计算字数和阅读时间
		if req.ContentMd != nil {
			wordCount, readingTime := calculatePostStats(*req.ContentMd)
			computedParams.WordCount = wordCount
			computedParams.ReadingTime = readingTime
		}
		if req.ContentHTML != nil {
			sanitizedHTML := s.parserSvc.SanitizeHTML(*req.ContentHTML)
			computedParams.ContentHTML = sanitizedHTML
		}

		isManual := oldArticle.IsPrimaryColorManual
		if req.IsPrimaryColorManual != nil {
			isManual = *req.IsPrimaryColorManual
			computedParams.IsPrimaryColorManual = &isManual
		}

		newTopImgURL := oldArticle.TopImgURL
		if req.TopImgURL != nil {
			newTopImgURL = *req.TopImgURL
		}
		newCoverURL := oldArticle.CoverURL
		if req.CoverURL != nil {
			newCoverURL = *req.CoverURL
		}

		if isManual {
			if req.PrimaryColor != nil {
				computedParams.PrimaryColor = req.PrimaryColor
			}
		} else {
			oldImageSource := oldArticle.TopImgURL
			if oldImageSource == "" {
				oldImageSource = oldArticle.CoverURL
			}
			newImageSource := newTopImgURL
			if newImageSource == "" {
				newImageSource = newCoverURL
			}
			modeChangedToAuto := req.IsPrimaryColorManual != nil && !(*req.IsPrimaryColorManual)
			imageChangedInAuto := oldImageSource != newImageSource
			if modeChangedToAuto || imageChangedInAuto {
				log.Printf("[信息] 文章 %s 需要重新获取主色调。原因: 模式切换=%t, 图片改变=%t", publicID, modeChangedToAuto, imageChangedInAuto)
				newColor := s.determinePrimaryColor(ctx, newTopImgURL, newCoverURL)
				computedParams.PrimaryColor = &newColor
			}
		}

		if req.IPLocation != nil && *req.IPLocation == "" {
			location := "未知"
			if ip != "" && s.geoService != nil {
				fetchedLocation, err := s.geoService.Lookup(ip)
				if err == nil {
					location = fetchedLocation
				} else {
					log.Printf("更新文章时自动获取 IP 属地失败: %v", err)
				}
			}
			*req.IPLocation = location
		}
		if req.CoverURL != nil && *req.CoverURL == "" {
			*req.CoverURL = s.settingSvc.Get(constant.KeyPostDefaultCover.String())
		}

		articleAfterUpdate, err := repos.Article.Update(ctx, publicID, req, &computedParams)
		if err != nil {
			return err
		}
		updatedArticle = articleAfterUpdate

		var newTagIDs []uint
		// 仅当请求中提供了标签时才解码，用于后续计数
		if req.PostTagIDs != nil {
			newTagIDs, err = idgen.DecodePublicIDBatch(req.PostTagIDs)
			if err != nil {
				return err
			}
		}

		// 计算需要增加和减少计数的标签/分类
		incTag, decTag := diffIDs(oldTagIDs, newTagIDs)
		// 使用之前已解码的 newCategoryDBIDs，避免重复操作
		incCat, decCat := diffIDs(oldCategoryIDs, newCategoryDBIDs)

		if err := repos.PostTag.UpdateCount(ctx, incTag, decTag); err != nil {
			return fmt.Errorf("更新标签计数失败: %w", err)
		}
		if err := repos.PostTag.DeleteIfUnused(ctx, decTag); err != nil {
			return fmt.Errorf("删除未使用的标签失败: %w", err)
		}

		if err := repos.PostCategory.UpdateCount(ctx, incCat, decCat); err != nil {
			return fmt.Errorf("更新分类计数失败: %w", err)
		}
		if err := repos.PostCategory.DeleteIfUnused(ctx, decCat); err != nil {
			return fmt.Errorf("删除未使用的分类失败: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	_ = s.cacheSvc.Delete(ctx, s.getCacheKey(publicID))
	_ = s.cacheSvc.Delete(ctx, s.getCacheKey(updatedArticle.Abbrlink))

	s.updateSiteStatsInBackground()

	// 异步更新搜索索引
	go func() {
		if err := s.searchSvc.IndexArticle(context.Background(), updatedArticle); err != nil {
			log.Printf("[警告] 更新搜索索引失败: %v", err)
		}
	}()

	return s.ToAPIResponse(updatedArticle, false, false), nil
}

// Delete 处理删除文章的业务逻辑。
func (s *serviceImpl) Delete(ctx context.Context, publicID string) error {
	err := s.txManager.Do(ctx, func(repos repository.Repositories) error {
		article, err := repos.Article.GetByID(ctx, publicID)
		if err != nil {
			return err
		}
		tagIDs := make([]uint, len(article.PostTags))
		for i, t := range article.PostTags {
			tagIDs[i], _, _ = idgen.DecodePublicID(t.ID)
		}
		categoryIDs := make([]uint, len(article.PostCategories))
		for i, c := range article.PostCategories {
			categoryIDs[i], _, _ = idgen.DecodePublicID(c.ID)
		}

		if err := repos.Article.Delete(ctx, publicID); err != nil {
			return err
		}

		if err := repos.PostTag.UpdateCount(ctx, nil, tagIDs); err != nil {
			return fmt.Errorf("更新标签计数失败: %w", err)
		}
		if err := repos.PostTag.DeleteIfUnused(ctx, tagIDs); err != nil {
			return fmt.Errorf("删除未使用的标签失败: %w", err)
		}

		if err := repos.PostCategory.UpdateCount(ctx, nil, categoryIDs); err != nil {
			return fmt.Errorf("更新分类计数失败: %w", err)
		}
		if err := repos.PostCategory.DeleteIfUnused(ctx, categoryIDs); err != nil {
			return fmt.Errorf("删除未使用的分类失败: %w", err)
		}

		_ = s.cacheSvc.Delete(ctx, s.getCacheKey(publicID))
		if article.Abbrlink != "" {
			_ = s.cacheSvc.Delete(ctx, s.getCacheKey(article.Abbrlink))
		}
		return nil
	})

	if err != nil {
		return err
	}

	s.updateSiteStatsInBackground()

	// 异步删除搜索索引
	go func() {
		if err := s.searchSvc.DeleteArticle(context.Background(), publicID); err != nil {
			log.Printf("[警告] 删除搜索索引失败: %v", err)
		}
	}()

	return nil
}

// diffIDs 是一个辅助函数，用于计算两个 ID 切片的差异
func diffIDs(oldIDs, newIDs []uint) (inc, dec []uint) {
	oldMap := make(map[uint]bool)
	for _, id := range oldIDs {
		oldMap[id] = true
	}
	newMap := make(map[uint]bool)
	for _, id := range newIDs {
		newMap[id] = true
	}
	for _, id := range newIDs {
		if !oldMap[id] {
			inc = append(inc, id)
		}
	}
	for _, id := range oldIDs {
		if !newMap[id] {
			dec = append(dec, id)
		}
	}
	return
}

// List 检索分页的文章列表。
func (s *serviceImpl) List(ctx context.Context, options *model.ListArticlesOptions) (*model.ArticleListResponse, error) {
	options.WithContent = false
	articles, total, err := s.repo.List(ctx, options)
	if err != nil {
		return nil, err
	}
	list := make([]model.ArticleResponse, len(articles))
	for i, a := range articles {
		a.ContentMd = ""
		list[i] = *s.ToAPIResponse(a, false, false)
	}
	return &model.ArticleListResponse{List: list, Total: int64(total), Page: options.Page, PageSize: options.PageSize}, nil
}

// GetRandom 获取一篇随机文章。
func (s *serviceImpl) GetRandom(ctx context.Context) (*model.ArticleResponse, error) {
	article, err := s.repo.GetRandom(ctx)
	if err != nil {
		return nil, err
	}
	return s.ToAPIResponse(article, true, true), nil
}

// ListHome 获取首页推荐文章列表。
func (s *serviceImpl) ListHome(ctx context.Context) ([]model.ArticleResponse, error) {
	articles, err := s.repo.ListHome(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]model.ArticleResponse, len(articles))
	for i, a := range articles {
		a.ContentMd = ""
		list[i] = *s.ToAPIResponse(a, true, false)
	}
	return list, nil
}

// ListPublic 获取公开的、分页的文章列表。
func (s *serviceImpl) ListPublic(ctx context.Context, options *model.ListPublicArticlesOptions) (*model.ArticleListResponse, error) {
	articles, total, err := s.repo.ListPublic(ctx, options)
	if err != nil {
		return nil, err
	}
	list := make([]model.ArticleResponse, len(articles))
	for i, a := range articles {
		a.ContentMd = ""
		list[i] = *s.ToAPIResponse(a, true, false)
	}
	return &model.ArticleListResponse{List: list, Total: int64(total), Page: options.Page, PageSize: options.PageSize}, nil
}

// ListArchives 获取文章归档摘要列表
func (s *serviceImpl) ListArchives(ctx context.Context) (*model.ArchiveSummaryResponse, error) {
	items, err := s.repo.GetArchiveSummary(ctx)
	if err != nil {
		return nil, err
	}

	limitStr := s.settingSvc.Get(constant.KeySidebarArchiveCount.String())
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 8
	}

	if len(items) > limit {
		items = items[:limit]
	}

	return &model.ArchiveSummaryResponse{List: items}, nil
}
