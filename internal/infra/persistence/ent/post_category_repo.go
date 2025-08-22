package ent

import (
	"anheyu-app/ent"
	"anheyu-app/ent/postcategory"
	"anheyu-app/internal/domain/model"
	"anheyu-app/internal/domain/repository"
	"anheyu-app/internal/pkg/idgen"
	"context"
)

type postCategoryRepo struct {
	db *ent.Client
}

// NewPostCategoryRepo 是 postCategoryRepo 的构造函数。
func NewPostCategoryRepo(db *ent.Client) repository.PostCategoryRepository {
	return &postCategoryRepo{db: db}
}

// DeleteIfUnused 实现了删除未使用分类的逻辑
func (r *postCategoryRepo) DeleteIfUnused(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.PostCategory.Delete().
		Where(
			postcategory.IDIn(ids...),
			postcategory.CountLTE(0), // 检查条件：引用计数小于或等于0
		).
		Exec(ctx)
	return err
}

// UpdateCount 更新指定 ID 集合的计数值
func (r *postCategoryRepo) UpdateCount(ctx context.Context, incIDs, decIDs []uint) error {
	if len(incIDs) > 0 {
		// 对 incIDs 列表中的所有分类 count+1
		_, err := r.db.PostCategory.Update().Where(postcategory.IDIn(incIDs...)).AddCount(1).Save(ctx)
		if err != nil {
			return err
		}
	}
	if len(decIDs) > 0 {
		// 对 decIDs 列表中的所有分类 count-1
		_, err := r.db.PostCategory.Update().Where(postcategory.IDIn(decIDs...)).AddCount(-1).Save(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// toModel 是一个私有辅助函数，将 ent 实体转换为领域模型。
func (r *postCategoryRepo) toModel(c *ent.PostCategory) *model.PostCategory {
	if c == nil {
		return nil
	}
	publicID, _ := idgen.GeneratePublicID(c.ID, idgen.EntityTypePostCategory)
	return &model.PostCategory{
		ID:          publicID,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
		Name:        c.Name,
		Description: c.Description,
		Count:       c.Count,
	}
}

func (r *postCategoryRepo) Create(ctx context.Context, req *model.CreatePostCategoryRequest) (*model.PostCategory, error) {
	newCategory, err := r.db.PostCategory.Create().
		SetName(req.Name).
		SetNillableDescription(&req.Description).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(newCategory), nil
}

func (r *postCategoryRepo) Update(ctx context.Context, publicID string, req *model.UpdatePostCategoryRequest) (*model.PostCategory, error) {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return nil, err
	}
	updater := r.db.PostCategory.UpdateOneID(dbID)
	if req.Name != nil {
		updater.SetName(*req.Name)
	}
	if req.Description != nil {
		updater.SetDescription(*req.Description)
	}
	updatedCategory, err := updater.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(updatedCategory), nil
}

func (r *postCategoryRepo) Delete(ctx context.Context, publicID string) error {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return err
	}
	return r.db.PostCategory.DeleteOneID(dbID).Exec(ctx)
}

func (r *postCategoryRepo) List(ctx context.Context) ([]*model.PostCategory, error) {
	entities, err := r.db.PostCategory.Query().
		Where(postcategory.DeletedAtIsNil()).
		Order(ent.Desc(postcategory.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	models := make([]*model.PostCategory, len(entities))
	for i, entity := range entities {
		models[i] = r.toModel(entity)
	}
	return models, nil
}

func (r *postCategoryRepo) GetByID(ctx context.Context, publicID string) (*model.PostCategory, error) {
	dbID, _, err := idgen.DecodePublicID(publicID)
	if err != nil {
		return nil, err
	}
	entity, err := r.db.PostCategory.Query().
		Where(postcategory.ID(dbID), postcategory.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(entity), nil
}
