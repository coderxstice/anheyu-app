/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-08-18 15:09:56
 * @LastEditTime: 2025-08-19 16:11:26
 * @LastEditors: 安知鱼
 */
package ent

import (
	"anheyu-app/ent"
	"anheyu-app/ent/link"
	"anheyu-app/ent/linktag"
	"anheyu-app/internal/domain/model"
	"anheyu-app/internal/domain/repository"
	"context"
)

type linkTagRepo struct {
	client *ent.Client
}

func NewLinkTagRepo(client *ent.Client) repository.LinkTagRepository {
	return &linkTagRepo{client: client}
}

func (r *linkTagRepo) DeleteAllUnused(ctx context.Context) (int, error) {
	// 查找所有没有关联任何 Link 的 LinkTag 并删除它们
	return r.client.LinkTag.Delete().
		Where(linktag.Not(linktag.HasLinks())).
		Exec(ctx)
}

func (r *linkTagRepo) DeleteIfUnused(ctx context.Context, tagIDs []int) (int64, error) {
	var deletedCount int64 = 0
	for _, tagID := range tagIDs {
		exists, err := r.client.Link.Query().
			Where(link.HasTagsWith(linktag.ID(tagID))).
			Exist(ctx)
		if err != nil {
			return deletedCount, err
		}

		if !exists {
			err = r.client.LinkTag.DeleteOneID(tagID).Exec(ctx)
			if err != nil && !ent.IsNotFound(err) {
				return deletedCount, err
			}
			if err == nil {
				deletedCount++
			}
		}
	}
	return deletedCount, nil
}

func (r *linkTagRepo) Update(ctx context.Context, id int, req *model.UpdateLinkTagRequest) (*model.LinkTagDTO, error) {
	updatedTag, err := r.client.LinkTag.UpdateOneID(id).
		SetName(req.Name).
		SetColor(req.Color).
		Save(ctx)

	if err != nil {
		return nil, err
	}
	return mapEntLinkTagToDTO(updatedTag), nil
}

func (r *linkTagRepo) Create(ctx context.Context, req *model.CreateLinkTagRequest) (*model.LinkTagDTO, error) {
	create := r.client.LinkTag.Create().
		SetName(req.Name)

	if req.Color != "" {
		create.SetColor(req.Color)
	}

	savedTag, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}

	return mapEntLinkTagToDTO(savedTag), nil
}

func (r *linkTagRepo) FindAll(ctx context.Context) ([]*model.LinkTagDTO, error) {
	entTags, err := r.client.LinkTag.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	return mapEntLinkTagsToDTOs(entTags), nil
}

// --- 辅助函数 ---

func mapEntLinkTagToDTO(entTag *ent.LinkTag) *model.LinkTagDTO {
	if entTag == nil {
		return nil
	}
	return &model.LinkTagDTO{
		ID:    entTag.ID,
		Name:  entTag.Name,
		Color: entTag.Color,
	}
}

func mapEntLinkTagsToDTOs(entTags []*ent.LinkTag) []*model.LinkTagDTO {
	dtos := make([]*model.LinkTagDTO, len(entTags))
	for i, tag := range entTags {
		dtos[i] = mapEntLinkTagToDTO(tag)
	}
	return dtos
}
