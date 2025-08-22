/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-07-25 11:50:43
 * @LastEditTime: 2025-08-05 10:44:27
 * @LastEditors: 安知鱼
 */
package post_category

import (
	"github.com/anzhiyu-c/anheyu-app/internal/domain/model"
	"github.com/anzhiyu-c/anheyu-app/internal/domain/repository"
	"context"
)

// Service 封装了文章分类的业务逻辑。
type Service struct {
	repo repository.PostCategoryRepository
}

// NewService 是 PostCategory Service 的构造函数。
func NewService(repo repository.PostCategoryRepository) *Service {
	return &Service{repo: repo}
}

// toAPIResponse 是一个私有的辅助函数，将领域模型转换为用于API响应的DTO。
func (s *Service) toAPIResponse(c *model.PostCategory) *model.PostCategoryResponse {
	if c == nil {
		return nil
	}
	return &model.PostCategoryResponse{
		ID:          c.ID,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
		Name:        c.Name,
		Description: c.Description,
		Count:       c.Count,
	}
}

// Create 处理创建新分类的业务逻辑。
func (s *Service) Create(ctx context.Context, req *model.CreatePostCategoryRequest) (*model.PostCategoryResponse, error) {
	newCategory, err := s.repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.toAPIResponse(newCategory), nil
}

// List 处理获取所有分类的业务逻辑。
func (s *Service) List(ctx context.Context) ([]*model.PostCategoryResponse, error) {
	categories, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]*model.PostCategoryResponse, len(categories))
	for i, category := range categories {
		responses[i] = s.toAPIResponse(category)
	}

	return responses, nil
}

// Update 处理更新分类的业务逻辑。
func (s *Service) Update(ctx context.Context, publicID string, req *model.UpdatePostCategoryRequest) (*model.PostCategoryResponse, error) {
	updatedCategory, err := s.repo.Update(ctx, publicID, req)
	if err != nil {
		return nil, err
	}
	return s.toAPIResponse(updatedCategory), nil
}

// Delete 处理删除分类的业务逻辑。
func (s *Service) Delete(ctx context.Context, publicID string) error {
	return s.repo.Delete(ctx, publicID)
}
