/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-07-25 11:41:57
 * @LastEditTime: 2025-08-05 11:15:54
 * @LastEditors: 安知鱼
 */
package model

import "time"

// --- 核心领域对象 (Domain Object) ---

// PostTag 是文章标签的核心领域模型。
type PostTag struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Count     int
}

// --- API 数据传输对象 (Data Transfer Objects) ---

// CreatePostTagRequest 定义了创建文章标签的请求体
type CreatePostTagRequest struct {
	Name string `json:"name" binding:"required"`
}

// UpdatePostTagRequest 定义了更新文章标签的请求体
type UpdatePostTagRequest struct {
	Name *string `json:"name"`
}

// PostTagResponse 定义了文章标签的标准 API 响应结构
type PostTagResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
	Count     int       `json:"count"`
}

const (
	SortByCount = "count" // 按引用数排序
	SortByName  = "name"  // 按字符排序
)

type ListPostTagsOptions struct {
	SortBy string
}
