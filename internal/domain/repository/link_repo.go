/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-08-18 15:09:15
 * @LastEditTime: 2025-08-18 18:17:05
 * @LastEditors: 安知鱼
 */
package repository

import (
	"github.com/anzhiyu-c/anheyu-app/internal/domain/model"
	"context"
)

type LinkRepository interface {
	Create(ctx context.Context, req *model.ApplyLinkRequest, categoryID int) (*model.LinkDTO, error)
	List(ctx context.Context, req *model.ListLinksRequest) ([]*model.LinkDTO, int, error)
	ListPublic(ctx context.Context, req *model.ListPublicLinksRequest) ([]*model.LinkDTO, int, error)
	UpdateStatus(ctx context.Context, id int, status string, siteshot *string) error
	GetByID(ctx context.Context, id int) (*model.LinkDTO, error)
	Update(ctx context.Context, id int, req *model.AdminUpdateLinkRequest) (*model.LinkDTO, error)
	Delete(ctx context.Context, id int) error
	AdminCreate(ctx context.Context, req *model.AdminCreateLinkRequest) (*model.LinkDTO, error)
	GetRandomPublic(ctx context.Context, num int) ([]*model.LinkDTO, error)
}
