package link

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

// TaskBroker 定义任务调度器的接口，用于解耦循环依赖。
type TaskBroker interface {
	DispatchLinkCleanup()
}

// Service 定义了友链相关的业务逻辑接口。
type Service interface {
	// --- 前台接口 ---
	ApplyLink(ctx context.Context, req *model.ApplyLinkRequest) (*model.LinkDTO, error)
	ListPublicLinks(ctx context.Context, req *model.ListPublicLinksRequest) (*model.LinkListResponse, error)
	ListCategories(ctx context.Context) ([]*model.LinkCategoryDTO, error)
	ListPublicCategories(ctx context.Context) ([]*model.LinkCategoryDTO, error) // 只返回有已审核通过友链的分类
	GetRandomLinks(ctx context.Context, num int) ([]*model.LinkDTO, error)

	// --- 后台接口 ---
	AdminCreateLink(ctx context.Context, req *model.AdminCreateLinkRequest) (*model.LinkDTO, error)
	AdminUpdateLink(ctx context.Context, id int, req *model.AdminUpdateLinkRequest) (*model.LinkDTO, error)
	AdminDeleteLink(ctx context.Context, id int) error
	ReviewLink(ctx context.Context, id int, req *model.ReviewLinkRequest) error
	UpdateCategory(ctx context.Context, id int, req *model.UpdateLinkCategoryRequest) (*model.LinkCategoryDTO, error)
	UpdateTag(ctx context.Context, id int, req *model.UpdateLinkTagRequest) (*model.LinkTagDTO, error)
	CreateCategory(ctx context.Context, req *model.CreateLinkCategoryRequest) (*model.LinkCategoryDTO, error)
	CreateTag(ctx context.Context, req *model.CreateLinkTagRequest) (*model.LinkTagDTO, error)
	DeleteCategory(ctx context.Context, id int) error
	DeleteTag(ctx context.Context, id int) error
	ListLinks(ctx context.Context, req *model.ListLinksRequest) (*model.LinkListResponse, error)
	AdminListAllTags(ctx context.Context) ([]*model.LinkTagDTO, error)
	ImportLinks(ctx context.Context, req *model.ImportLinksRequest) (*model.ImportLinksResponse, error)
	CheckLinksHealth(ctx context.Context) (*model.LinkHealthCheckResponse, error)
}

type service struct {
	// 用于数据库操作的 Repositories
	linkRepo         repository.LinkRepository
	linkCategoryRepo repository.LinkCategoryRepository
	linkTagRepo      repository.LinkTagRepository
	// 用于派发异步任务的 Broker
	broker TaskBroker
	// 保留事务管理器以备将来使用
	txManager repository.TransactionManager
	// 用于获取系统配置
	settingSvc setting.SettingService
}

// NewService 是 service 的构造函数，注入所有依赖。
func NewService(
	linkRepo repository.LinkRepository,
	linkCategoryRepo repository.LinkCategoryRepository,
	linkTagRepo repository.LinkTagRepository,
	txManager repository.TransactionManager,
	broker TaskBroker,
	settingSvc setting.SettingService,
) Service {
	return &service{
		linkRepo:         linkRepo,
		linkCategoryRepo: linkCategoryRepo,
		linkTagRepo:      linkTagRepo,
		txManager:        txManager,
		broker:           broker,
		settingSvc:       settingSvc,
	}
}

// AdminListAllTags 获取所有友链标签，供后台使用。
func (s *service) AdminListAllTags(ctx context.Context) ([]*model.LinkTagDTO, error) {
	return s.linkTagRepo.FindAll(ctx)
}

func (s *service) UpdateCategory(ctx context.Context, id int, req *model.UpdateLinkCategoryRequest) (*model.LinkCategoryDTO, error) {
	return s.linkCategoryRepo.Update(ctx, id, req)
}

func (s *service) UpdateTag(ctx context.Context, id int, req *model.UpdateLinkTagRequest) (*model.LinkTagDTO, error) {
	return s.linkTagRepo.Update(ctx, id, req)
}

func (s *service) GetRandomLinks(ctx context.Context, num int) ([]*model.LinkDTO, error) {
	// 业务逻辑：设置默认值和最大值，防止恶意请求
	if num <= 0 {
		num = 5 // 默认获取 5 条
	}
	const maxNum = 20 // 最多一次获取 20 条
	if num > maxNum {
		num = maxNum
	}

	return s.linkRepo.GetRandomPublic(ctx, num)
}

// ApplyLink 处理前台友链申请。
func (s *service) ApplyLink(ctx context.Context, req *model.ApplyLinkRequest) (*model.LinkDTO, error) {
	// 从配置中获取默认分类ID
	defaultCategoryIDStr := s.settingSvc.Get(constant.KeyFriendLinkDefaultCategory.String())
	defaultCategoryID := 2 // 默认值，如果配置获取失败
	if defaultCategoryIDStr != "" {
		if id, err := strconv.Atoi(defaultCategoryIDStr); err == nil && id > 0 {
			defaultCategoryID = id
		}
	}

	// 获取默认分类信息，用于验证样式要求
	defaultCategory, err := s.linkCategoryRepo.GetByID(ctx, defaultCategoryID)
	if err != nil {
		return nil, fmt.Errorf("获取默认分类失败: %w", err)
	}

	// 如果是卡片样式，则要求必须提供网站快照
	if defaultCategory.Style == "card" && req.Siteshot == "" {
		return nil, errors.New("卡片样式的友链申请时必须提供网站快照(siteshot)")
	}

	return s.linkRepo.Create(ctx, req, defaultCategoryID)
}

// ListPublicLinks 获取公开的友链列表。
func (s *service) ListPublicLinks(ctx context.Context, req *model.ListPublicLinksRequest) (*model.LinkListResponse, error) {
	links, total, err := s.linkRepo.ListPublic(ctx, req)
	if err != nil {
		return nil, err
	}

	return &model.LinkListResponse{
		List:     links,
		Total:    int64(total),
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	}, nil
}

// ListCategories 获取所有友链分类。
func (s *service) ListCategories(ctx context.Context) ([]*model.LinkCategoryDTO, error) {
	return s.linkCategoryRepo.FindAll(ctx)
}

// ListPublicCategories 获取有已审核通过友链的分类，用于前台公开接口。
func (s *service) ListPublicCategories(ctx context.Context) ([]*model.LinkCategoryDTO, error) {
	return s.linkCategoryRepo.FindAllWithLinks(ctx)
}

// AdminCreateLink 处理后台创建友链，并在成功后派发一个异步清理任务。
func (s *service) AdminCreateLink(ctx context.Context, req *model.AdminCreateLinkRequest) (*model.LinkDTO, error) {
	link, err := s.linkRepo.AdminCreate(ctx, req)
	if err != nil {
		return nil, err
	}
	// 操作成功后，派发清理任务，API 无需等待
	s.broker.DispatchLinkCleanup()
	return link, nil
}

// AdminUpdateLink 处理后台更新友链，并在成功后派发一个异步清理任务。
func (s *service) AdminUpdateLink(ctx context.Context, id int, req *model.AdminUpdateLinkRequest) (*model.LinkDTO, error) {
	link, err := s.linkRepo.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}
	// 操作成功后，派发清理任务
	s.broker.DispatchLinkCleanup()
	return link, nil
}

// AdminDeleteLink 处理后台删除友链，并在成功后派发一个异步清理任务。
func (s *service) AdminDeleteLink(ctx context.Context, id int) error {
	err := s.linkRepo.Delete(ctx, id)
	if err != nil {
		return err
	}
	// 操作成功后，派发清理任务
	s.broker.DispatchLinkCleanup()
	return nil
}

// ReviewLink 处理友链审核，这是一个简单操作，无需清理。
func (s *service) ReviewLink(ctx context.Context, id int, req *model.ReviewLinkRequest) error {
	// 只有在审核通过时才需要进行特殊校验
	if req.Status == "APPROVED" {
		// 1. 获取友链及其分类信息
		linkToReview, err := s.linkRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("获取友链信息失败: %w", err)
		}
		if linkToReview.Category == nil {
			return errors.New("无法审核：该友链未关联任何分类")
		}

		// 2. 检查分类样式是否为 'card'
		if linkToReview.Category.Style == "card" {
			// 3. 如果是 card 样式，则 siteshot 必须存在且不为空
			if req.Siteshot == nil || *req.Siteshot == "" {
				return errors.New("卡片样式的友链在审核通过时必须提供网站快照(siteshot)")
			}
		}
	}

	// 4. 执行更新状态操作
	return s.linkRepo.UpdateStatus(ctx, id, req.Status, req.Siteshot)
}

// CreateCategory 处理创建分类。
func (s *service) CreateCategory(ctx context.Context, req *model.CreateLinkCategoryRequest) (*model.LinkCategoryDTO, error) {
	return s.linkCategoryRepo.Create(ctx, req)
}

// CreateTag 处理创建标签。
func (s *service) CreateTag(ctx context.Context, req *model.CreateLinkTagRequest) (*model.LinkTagDTO, error) {
	return s.linkTagRepo.Create(ctx, req)
}

// DeleteCategory 删除分类。
func (s *service) DeleteCategory(ctx context.Context, id int) error {
	// 使用已有的 DeleteIfUnused 方法，它会检查是否有友链在使用
	deleted, err := s.linkCategoryRepo.DeleteIfUnused(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return errors.New("该分类正在被友链使用，无法删除")
	}
	return nil
}

// DeleteTag 删除标签。
func (s *service) DeleteTag(ctx context.Context, id int) error {
	// 使用已有的 DeleteIfUnused 方法，它会检查是否有友链在使用
	deletedCount, err := s.linkTagRepo.DeleteIfUnused(ctx, []int{id})
	if err != nil {
		return err
	}
	if deletedCount == 0 {
		return errors.New("该标签正在被友链使用，无法删除")
	}
	return nil
}

// ListLinks 获取后台友链列表。
func (s *service) ListLinks(ctx context.Context, req *model.ListLinksRequest) (*model.LinkListResponse, error) {
	links, total, err := s.linkRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &model.LinkListResponse{
		List:     links,
		Total:    int64(total),
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	}, nil
}

// ImportLinks 批量导入友链，支持重复检查、自动创建分类和标签等功能。
func (s *service) ImportLinks(ctx context.Context, req *model.ImportLinksRequest) (*model.ImportLinksResponse, error) {
	response := &model.ImportLinksResponse{
		Total:       len(req.Links),
		Success:     0,
		Failed:      0,
		Skipped:     0,
		SuccessList: make([]*model.LinkDTO, 0),
		FailedList:  make([]model.ImportLinkFailure, 0),
		SkippedList: make([]model.ImportLinkSkipped, 0),
	}

	// 创建分类和标签的缓存映射，避免重复查询
	categoryCache := make(map[string]*model.LinkCategoryDTO)
	tagCache := make(map[string]*model.LinkTagDTO)

	// 获取默认分类ID，如果没有指定或指定的分类不存在
	defaultCategoryID := 2 // 系统默认值
	if req.DefaultCategoryID != nil && *req.DefaultCategoryID > 0 {
		defaultCategoryID = *req.DefaultCategoryID
	} else {
		// 从配置中获取默认分类ID
		defaultCategoryIDStr := s.settingSvc.Get(constant.KeyFriendLinkDefaultCategory.String())
		if defaultCategoryIDStr != "" {
			if id, err := strconv.Atoi(defaultCategoryIDStr); err == nil && id > 0 {
				defaultCategoryID = id
			}
		}
	}

	// 逐个处理导入的友链
	for _, linkItem := range req.Links {
		// 1. 重复检查
		if req.SkipDuplicates {
			exists, err := s.linkRepo.ExistsByURL(ctx, linkItem.URL)
			if err != nil {
				response.Failed++
				response.FailedList = append(response.FailedList, model.ImportLinkFailure{
					Link:   linkItem,
					Reason: fmt.Errorf("检查重复链接失败: %w", err).Error(),
				})
				continue
			}
			if exists {
				response.Skipped++
				response.SkippedList = append(response.SkippedList, model.ImportLinkSkipped{
					Link:   linkItem,
					Reason: "友链URL已存在",
				})
				continue
			}
		}

		// 2. 处理分类
		var categoryID int
		if linkItem.CategoryName != "" {
			// 先从缓存查找
			if cachedCategory, exists := categoryCache[linkItem.CategoryName]; exists {
				categoryID = cachedCategory.ID
			} else {
				// 尝试查找现有分类
				category, err := s.linkCategoryRepo.GetByName(ctx, linkItem.CategoryName)
				if err != nil {
					// 分类不存在
					if req.CreateCategories {
						// 自动创建分类
						createReq := &model.CreateLinkCategoryRequest{
							Name:        linkItem.CategoryName,
							Style:       "list", // 默认为列表样式
							Description: fmt.Sprintf("导入时自动创建的分类：%s", linkItem.CategoryName),
						}
						newCategory, err := s.linkCategoryRepo.Create(ctx, createReq)
						if err != nil {
							response.Failed++
							response.FailedList = append(response.FailedList, model.ImportLinkFailure{
								Link:   linkItem,
								Reason: fmt.Errorf("创建分类失败: %w", err).Error(),
							})
							continue
						}
						categoryID = newCategory.ID
						categoryCache[linkItem.CategoryName] = newCategory
					} else {
						// 使用默认分类
						categoryID = defaultCategoryID
					}
				} else {
					categoryID = category.ID
					categoryCache[linkItem.CategoryName] = category
				}
			}
		} else {
			// 没有指定分类名称，使用默认分类
			categoryID = defaultCategoryID
		}

		// 3. 处理标签（可选）
		var tagID *int
		if linkItem.TagName != "" {
			// 先从缓存查找
			if cachedTag, exists := tagCache[linkItem.TagName]; exists {
				tagID = &cachedTag.ID
			} else {
				// 尝试查找现有标签
				tag, err := s.linkTagRepo.GetByName(ctx, linkItem.TagName)
				if err != nil {
					// 标签不存在
					if req.CreateTags {
						// 自动创建标签
						createReq := &model.CreateLinkTagRequest{
							Name:  linkItem.TagName,
							Color: "#409EFF", // 默认颜色
						}
						newTag, err := s.linkTagRepo.Create(ctx, createReq)
						if err != nil {
							response.Failed++
							response.FailedList = append(response.FailedList, model.ImportLinkFailure{
								Link:   linkItem,
								Reason: fmt.Errorf("创建标签失败: %w", err).Error(),
							})
							continue
						}
						tagID = &newTag.ID
						tagCache[linkItem.TagName] = newTag
					}
					// 如果不允许创建标签，就不设置标签（tagID 保持为 nil）
				} else {
					tagID = &tag.ID
					tagCache[linkItem.TagName] = tag
				}
			}
		}

		// 4. 设置默认状态
		status := linkItem.Status
		if status == "" {
			status = "PENDING" // 默认为待审核状态
		}

		// 5. 创建友链
		adminCreateReq := &model.AdminCreateLinkRequest{
			Name:        linkItem.Name,
			URL:         linkItem.URL,
			Logo:        linkItem.Logo,
			Description: linkItem.Description,
			CategoryID:  categoryID,
			TagID:       tagID,
			Status:      status,
			Siteshot:    linkItem.Siteshot,
		}

		createdLink, err := s.linkRepo.AdminCreate(ctx, adminCreateReq)
		if err != nil {
			response.Failed++
			response.FailedList = append(response.FailedList, model.ImportLinkFailure{
				Link:   linkItem,
				Reason: fmt.Errorf("创建友链失败: %w", err).Error(),
			})
			continue
		}

		// 成功创建
		response.Success++
		response.SuccessList = append(response.SuccessList, createdLink)
	}

	// 如果有成功创建的友链，派发清理任务
	if response.Success > 0 {
		s.broker.DispatchLinkCleanup()
	}

	return response, nil
}

// CheckLinksHealth 检查所有友链的健康状态，将无法访问的友链标记为 INVALID，将恢复的友链标记为 APPROVED。
func (s *service) CheckLinksHealth(ctx context.Context) (*model.LinkHealthCheckResponse, error) {
	// 1. 获取所有已审核通过的友链
	approvedLinks, err := s.linkRepo.GetAllApprovedLinks(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取已审核友链列表失败: %w", err)
	}

	// 2. 获取所有失联的友链（用于检查是否恢复）
	invalidLinks, err := s.linkRepo.GetAllInvalidLinks(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取失联友链列表失败: %w", err)
	}

	response := &model.LinkHealthCheckResponse{
		Total:        len(approvedLinks) + len(invalidLinks),
		Healthy:      0,
		Unhealthy:    0,
		UnhealthyIDs: make([]int, 0),
	}

	if response.Total == 0 {
		return response, nil
	}

	// 3. 创建 HTTP 客户端，设置超时时间
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 最多跟随 5 次重定向
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	// 4. 使用 WaitGroup 和互斥锁来并发检查友链
	var wg sync.WaitGroup
	var mu sync.Mutex
	toInvalidIDs := make([]int, 0)  // 需要标记为失联的友链ID
	toApprovedIDs := make([]int, 0) // 需要恢复的友链ID

	// 创建一个带缓冲的通道来限制并发数
	semaphore := make(chan struct{}, 10) // 最多同时检查 10 个友链

	// 5. 检查已审核通过的友链
	for _, link := range approvedLinks {
		wg.Add(1)
		go func(l *model.LinkDTO) {
			defer wg.Done()
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			isHealthy := checkLinkHealth(client, l.URL)
			mu.Lock()
			if isHealthy {
				response.Healthy++
			} else {
				response.Unhealthy++
				toInvalidIDs = append(toInvalidIDs, l.ID)
			}
			mu.Unlock()
		}(link)
	}

	// 6. 检查失联的友链（检查是否恢复）
	for _, link := range invalidLinks {
		wg.Add(1)
		go func(l *model.LinkDTO) {
			defer wg.Done()
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			isHealthy := checkLinkHealth(client, l.URL)
			mu.Lock()
			if isHealthy {
				response.Healthy++
				toApprovedIDs = append(toApprovedIDs, l.ID)
			} else {
				response.Unhealthy++
			}
			mu.Unlock()
		}(link)
	}

	wg.Wait()

	// 7. 批量更新失联友链的状态为 INVALID
	if len(toInvalidIDs) > 0 {
		if err := s.linkRepo.BatchUpdateStatus(ctx, toInvalidIDs, "INVALID"); err != nil {
			return nil, fmt.Errorf("更新失联友链状态失败: %w", err)
		}
		response.UnhealthyIDs = append(response.UnhealthyIDs, toInvalidIDs...)
	}

	// 8. 批量恢复健康友链的状态为 APPROVED
	if len(toApprovedIDs) > 0 {
		if err := s.linkRepo.BatchUpdateStatus(ctx, toApprovedIDs, "APPROVED"); err != nil {
			return nil, fmt.Errorf("恢复友链状态失败: %w", err)
		}
		// 恢复的友链也算在健康的友链中，但不放入 UnhealthyIDs
	}

	return response, nil
}

// checkLinkHealth 检查单个友链的健康状态。
func checkLinkHealth(client *http.Client, url string) bool {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	// 设置 User-Agent 避免被网站屏蔽
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; LinkHealthChecker/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 认为 2xx 和 3xx 状态码为健康
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
