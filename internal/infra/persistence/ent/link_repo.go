package ent

import (
	"context"

	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/ent/link"
	"github.com/anzhiyu-c/anheyu-app/ent/linkcategory"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"

	"entgo.io/ent/dialect/sql"
)

type linkRepo struct {
	client *ent.Client
	dbType string
}

func NewLinkRepo(client *ent.Client, dbType string) repository.LinkRepository {
	return &linkRepo{
		client: client,
		dbType: dbType,
	}
}

func (r *linkRepo) Create(ctx context.Context, req *model.ApplyLinkRequest, categoryID int) (*model.LinkDTO, error) {
	create := r.client.Link.Create().
		SetName(req.Name).
		SetURL(req.URL).
		SetStatus(link.StatusPENDING).
		SetCategoryID(categoryID)

	if req.Logo != "" {
		create.SetLogo(req.Logo)
	}
	if req.Description != "" {
		create.SetDescription(req.Description)
	}
	if req.Siteshot != "" {
		create.SetSiteshot(req.Siteshot)
	}

	savedLink, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}
	return mapEntLinkToDTO(savedLink), nil
}

func (r *linkRepo) List(ctx context.Context, req *model.ListLinksRequest) ([]*model.LinkDTO, int, error) {
	query := r.client.Link.Query().WithCategory().WithTags()
	if req.Name != nil && *req.Name != "" {
		query = query.Where(link.NameContains(*req.Name))
	}
	if req.Status != nil && *req.Status != "" {
		query = query.Where(link.StatusEQ(link.Status(*req.Status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	entLinks, err := query.
		Offset((req.GetPage()-1)*req.GetPageSize()).
		Limit(req.GetPageSize()).
		Order(ent.Desc(link.FieldSortOrder), ent.Desc(link.FieldID)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return mapEntLinksToDTOs(entLinks), total, nil
}

func (r *linkRepo) GetByID(ctx context.Context, id int) (*model.LinkDTO, error) {
	entLink, err := r.client.Link.Query().
		Where(link.ID(id)).
		WithCategory().
		WithTags().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return mapEntLinkToDTO(entLink), nil
}

func (r *linkRepo) AdminCreate(ctx context.Context, req *model.AdminCreateLinkRequest) (*model.LinkDTO, error) {
	create := r.client.Link.Create().
		SetName(req.Name).
		SetURL(req.URL).
		SetStatus(link.Status(req.Status)).
		SetSiteshot(req.Siteshot).
		SetCategoryID(req.CategoryID).
		SetSortOrder(req.SortOrder).
		SetSkipHealthCheck(req.SkipHealthCheck)

	// 处理单个标签
	if req.TagID != nil {
		create.AddTagIDs(*req.TagID)
	}

	if req.Logo != "" {
		create.SetLogo(req.Logo)
	}

	if req.Description != "" {
		create.SetDescription(req.Description)
	}

	savedLink, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}

	// 重新查询以加载关联数据
	refetchedLink, err := r.client.Link.Query().
		Where(link.ID(savedLink.ID)).
		WithCategory().
		WithTags().
		Only(ctx)
	if err != nil {
		return nil, err
	}

	return mapEntLinkToDTO(refetchedLink), nil
}

func (r *linkRepo) Update(ctx context.Context, id int, req *model.AdminUpdateLinkRequest) (*model.LinkDTO, error) {
	// 1. 执行更新操作，使用 _ 忽略用不到的返回值
	updater := r.client.Link.UpdateOneID(id).
		SetName(req.Name).
		SetURL(req.URL).
		SetLogo(req.Logo).
		SetSiteshot(req.Siteshot).
		SetDescription(req.Description).
		SetStatus(link.Status(req.Status)).
		SetCategoryID(req.CategoryID).
		SetSortOrder(req.SortOrder).
		SetSkipHealthCheck(req.SkipHealthCheck).
		ClearTags()

	// 处理单个标签
	if req.TagID != nil {
		updater.AddTagIDs(*req.TagID)
	}

	_, err := updater.Save(ctx)

	if err != nil {
		return nil, err
	}

	// 2. 查询更新后的完整数据并返回
	refetchedLink, err := r.client.Link.Query().
		Where(link.ID(id)).
		WithCategory().
		WithTags().
		Only(ctx)
	if err != nil {
		return nil, err
	}

	return mapEntLinkToDTO(refetchedLink), nil
}

func (r *linkRepo) Delete(ctx context.Context, id int) error {
	return r.client.Link.DeleteOneID(id).Exec(ctx)
}

func (r *linkRepo) ListPublic(ctx context.Context, req *model.ListPublicLinksRequest) ([]*model.LinkDTO, int, error) {
	query := r.client.Link.Query().
		WithCategory().
		WithTags().
		Where(link.StatusEQ(link.StatusAPPROVED))

	if req.CategoryID != nil {
		query = query.Where(link.HasCategoryWith(linkcategory.ID(*req.CategoryID)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	results, err := query.
		Offset((req.GetPage()-1)*req.GetPageSize()).
		Limit(req.GetPageSize()).
		Order(ent.Desc(link.FieldSortOrder), ent.Desc(link.FieldID)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return mapEntLinksToDTOs(results), total, nil
}

func (r *linkRepo) UpdateStatus(ctx context.Context, id int, status string, siteshot *string) error {
	update := r.client.Link.UpdateOneID(id).
		SetStatus(link.Status(status))

	// 如果 siteshot 不是 nil，则更新它（允许更新为空字符串以清空）
	if siteshot != nil {
		update.SetSiteshot(*siteshot)
	}

	_, err := update.Save(ctx)
	return err
}

// --- 辅助函数 ---

func mapEntLinkToDTO(entLink *ent.Link) *model.LinkDTO {
	if entLink == nil {
		return nil
	}
	dto := &model.LinkDTO{
		ID:              entLink.ID,
		Name:            entLink.Name,
		URL:             entLink.URL,
		Logo:            entLink.Logo,
		Description:     entLink.Description,
		Status:          string(entLink.Status),
		Siteshot:        entLink.Siteshot,
		SortOrder:       entLink.SortOrder,
		SkipHealthCheck: entLink.SkipHealthCheck,
	}
	if entLink.Edges.Category != nil {
		dto.Category = &model.LinkCategoryDTO{
			ID:          entLink.Edges.Category.ID,
			Name:        entLink.Edges.Category.Name,
			Style:       string(entLink.Edges.Category.Style),
			Description: entLink.Edges.Category.Description,
		}
	}
	// 处理单个标签
	if len(entLink.Edges.Tags) > 0 {
		// 只取第一个标签
		entTag := entLink.Edges.Tags[0]
		dto.Tag = &model.LinkTagDTO{
			ID:    entTag.ID,
			Name:  entTag.Name,
			Color: entTag.Color,
		}
	}
	return dto
}

func (r *linkRepo) GetRandomPublic(ctx context.Context, num int) ([]*model.LinkDTO, error) {
	randomFunc := "RAND()"
	if r.dbType == "postgres" || r.dbType == "sqlite3" {
		randomFunc = "RANDOM()"
	}

	entLinks, err := r.client.Link.Query().
		WithCategory().
		WithTags().
		Where(link.StatusEQ(link.StatusAPPROVED)).
		Modify(func(s *sql.Selector) {
			s.OrderExpr(sql.Expr(randomFunc))
		}).
		Limit(num).
		All(ctx)

	if err != nil {
		return nil, err
	}

	return mapEntLinksToDTOs(entLinks), nil
}

func mapEntLinksToDTOs(entLinks []*ent.Link) []*model.LinkDTO {
	dtos := make([]*model.LinkDTO, len(entLinks))
	for i, entLink := range entLinks {
		dtos[i] = mapEntLinkToDTO(entLink)
	}
	return dtos
}

// ExistsByURL 检查指定URL的友链是否已存在
func (r *linkRepo) ExistsByURL(ctx context.Context, url string) (bool, error) {
	exists, err := r.client.Link.Query().
		Where(link.URLEQ(url)).
		Exist(ctx)
	return exists, err
}

// GetAllApprovedLinks 获取所有已审核通过的友链
func (r *linkRepo) GetAllApprovedLinks(ctx context.Context) ([]*model.LinkDTO, error) {
	entLinks, err := r.client.Link.Query().
		WithCategory().
		WithTags().
		Where(
			link.StatusEQ(link.StatusAPPROVED),
			link.SkipHealthCheckEQ(false), // 排除跳过健康检查的友链
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return mapEntLinksToDTOs(entLinks), nil
}

// GetAllInvalidLinks 获取所有失联状态的友链
func (r *linkRepo) GetAllInvalidLinks(ctx context.Context) ([]*model.LinkDTO, error) {
	entLinks, err := r.client.Link.Query().
		WithCategory().
		WithTags().
		Where(
			link.StatusEQ(link.StatusINVALID),
			link.SkipHealthCheckEQ(false), // 排除跳过健康检查的友链
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return mapEntLinksToDTOs(entLinks), nil
}

// BatchUpdateSortOrder 批量更新友链的排序权重
func (r *linkRepo) BatchUpdateSortOrder(ctx context.Context, items []model.LinkSortItem) error {
	// 使用事务确保原子性
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}

	for _, item := range items {
		err := tx.Link.UpdateOneID(item.ID).
			SetSortOrder(item.SortOrder).
			Exec(ctx)
		if err != nil {
			// 回滚事务
			_ = tx.Rollback()
			return err
		}
	}

	// 提交事务
	return tx.Commit()
}

// BatchUpdateStatus 批量更新友链状态
func (r *linkRepo) BatchUpdateStatus(ctx context.Context, linkIDs []int, status string) error {
	if len(linkIDs) == 0 {
		return nil
	}
	_, err := r.client.Link.Update().
		Where(link.IDIn(linkIDs...)).
		SetStatus(link.Status(status)).
		Save(ctx)
	return err
}
