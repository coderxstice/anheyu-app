/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-08-18 15:08:16
 * @LastEditTime: 2025-08-19 16:07:25
 * @LastEditors: 安知鱼
 */
package repository

import (
	"anheyu-app/internal/domain/model"
	"context"
)

type LinkCategoryRepository interface {
	Create(ctx context.Context, category *model.CreateLinkCategoryRequest) (*model.LinkCategoryDTO, error)
	FindAll(ctx context.Context) ([]*model.LinkCategoryDTO, error)
	DeleteIfUnused(ctx context.Context, categoryID int) (bool, error)
	DeleteAllUnused(ctx context.Context) (int, error)
	DeleteAllUnusedExcluding(ctx context.Context, excludeIDs []int) (int, error)
	Update(ctx context.Context, id int, req *model.UpdateLinkCategoryRequest) (*model.LinkCategoryDTO, error)
}
