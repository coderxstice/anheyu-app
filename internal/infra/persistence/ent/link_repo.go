package ent

import (
	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/ent/link"
	"github.com/anzhiyu-c/anheyu-app/ent/linkcategory"
	"github.com/anzhiyu-c/anheyu-app/internal/domain/model"
	"github.com/anzhiyu-c/anheyu-app/internal/domain/repository"
	"context"

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
	// ... 其他筛选条件 ...
	if req.Status != nil && *req.Status != "" {
		query = query.Where(link.StatusEQ(link.Status(*req.Status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	entLinks, err := query.
		Offset((req.GetPage() - 1) * req.GetPageSize()).
		Limit(req.GetPageSize()).
		Order(ent.Desc(link.FieldID)).
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
		AddTagIDs(req.TagIDs...)

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
	_, err := r.client.Link.UpdateOneID(id).
		SetName(req.Name).
		SetURL(req.URL).
		SetLogo(req.Logo).
		SetSiteshot(req.Siteshot).
		SetDescription(req.Description).
		SetStatus(link.Status(req.Status)).
		SetCategoryID(req.CategoryID).
		ClearTags().
		AddTagIDs(req.TagIDs...).
		Save(ctx)

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
		Offset((req.GetPage() - 1) * req.GetPageSize()).
		Limit(req.GetPageSize()).
		Order(ent.Desc(link.FieldID)).
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
		ID:          entLink.ID,
		Name:        entLink.Name,
		URL:         entLink.URL,
		Logo:        entLink.Logo,
		Description: entLink.Description,
		Status:      string(entLink.Status),
		Siteshot:    entLink.Siteshot,
	}
	if entLink.Edges.Category != nil {
		dto.Category = &model.LinkCategoryDTO{
			ID:          entLink.Edges.Category.ID,
			Name:        entLink.Edges.Category.Name,
			Style:       string(entLink.Edges.Category.Style),
			Description: entLink.Edges.Category.Description,
		}
	}
	if len(entLink.Edges.Tags) > 0 {
		dto.Tags = make([]*model.LinkTagDTO, len(entLink.Edges.Tags))
		for i, entTag := range entLink.Edges.Tags {
			dto.Tags[i] = &model.LinkTagDTO{ID: entTag.ID, Name: entTag.Name, Color: entTag.Color}
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
