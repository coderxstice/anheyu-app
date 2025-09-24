package link

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/anzhiyu-c/anheyu-app/internal/app/task"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

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
}

type service struct {
	// 用于数据库操作的 Repositories
	linkRepo         repository.LinkRepository
	linkCategoryRepo repository.LinkCategoryRepository
	linkTagRepo      repository.LinkTagRepository
	// 用于派发异步任务的 Broker
	broker *task.Broker
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
	broker *task.Broker,
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
