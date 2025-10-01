package model

// PaginationInput 是分页输入的基础结构，可被其他请求 DTO 嵌入。
type PaginationInput struct {
	Page     int `form:"page" binding:"omitempty,gte=1"`
	PageSize int `form:"pageSize" binding:"omitempty,gte=1,lte=1000"`
}

// GetPage 获取经过处理的安全页码，默认为 1。
func (p *PaginationInput) GetPage() int {
	if p.Page <= 0 {
		return 1
	}
	return p.Page
}

// GetPageSize 获取经过处理的安全每页数量，默认为 10。
func (p *PaginationInput) GetPageSize() int {
	if p.PageSize <= 0 {
		return 10
	}
	return p.PageSize
}

type LinkCategoryDTO struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Style       string `json:"style"`
	Description string `json:"description,omitempty"`
}

type LinkTagDTO struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type LinkDTO struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	URL         string           `json:"url"`
	Logo        string           `json:"logo"`
	Description string           `json:"description"`
	Status      string           `json:"status"`
	Siteshot    string           `json:"siteshot,omitempty"`
	SortOrder   int              `json:"sort_order"`
	Category    *LinkCategoryDTO `json:"category"`
	Tag         *LinkTagDTO      `json:"tag"` // 改为单个标签
}

// --- API 请求/响应 DTO ---

// ApplyLinkRequest 是前台用户申请友链的请求结构。
type ApplyLinkRequest struct {
	Name        string `json:"name" binding:"required"`
	URL         string `json:"url" binding:"required,url"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
	Siteshot    string `json:"siteshot"` // 网站快照URL，可选字段
}

// CreateLinkCategoryRequest 是后台管理员创建友链分类的请求结构。
type CreateLinkCategoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Style       string `json:"style" binding:"required,oneof=card list"`
	Description string `json:"description"`
}

// CreateLinkTagRequest 是后台管理员创建友链标签的请求结构。
type CreateLinkTagRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

// AdminCreateLinkRequest 是后台管理员直接创建友链的请求结构。
type AdminCreateLinkRequest struct {
	Name        string `json:"name" binding:"required"`
	URL         string `json:"url" binding:"required,url"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
	CategoryID  int    `json:"category_id" binding:"required"`
	TagID       *int   `json:"tag_id"` // 改为单个标签，可选
	Status      string `json:"status" binding:"required,oneof=PENDING APPROVED REJECTED INVALID"`
	Siteshot    string `json:"siteshot"`
	SortOrder   int    `json:"sort_order"`
}

// ReviewLinkRequest 是后台管理员审核友链的请求结构。
type ReviewLinkRequest struct {
	Status   string  `json:"status" binding:"required,oneof=APPROVED REJECTED"`
	Siteshot *string `json:"siteshot"`
}

// ListLinksRequest 是后台查询友链列表的请求结构，支持筛选和分页。
type ListLinksRequest struct {
	PaginationInput
	Name        *string `form:"name"`
	URL         *string `form:"url"`
	Description *string `form:"description"`
	Status      *string `form:"status" binding:"omitempty,oneof=PENDING APPROVED REJECTED INVALID"`
}

// ListPublicLinksRequest 是前台查询友链列表的请求结构，仅支持分页。
type ListPublicLinksRequest struct {
	PaginationInput
	CategoryID *int `form:"category_id"`
}

// LinkListResponse 是友链列表的统一 API 响应结构，包含分页信息。
type LinkListResponse struct {
	List     []*LinkDTO `json:"list"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

// AdminUpdateLinkRequest 是后台管理员更新友链的请求结构。
type AdminUpdateLinkRequest struct {
	Name        string `json:"name" binding:"required"`
	URL         string `json:"url" binding:"required,url"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
	CategoryID  int    `json:"category_id" binding:"required"`
	TagID       *int   `json:"tag_id"` // 改为单个标签，可选
	Status      string `json:"status" binding:"required,oneof=PENDING APPROVED REJECTED INVALID"`
	Siteshot    string `json:"siteshot"`
	SortOrder   int    `json:"sort_order"`
}

// UpdateLinkCategoryRequest 是后台管理员更新友链分类的请求结构。
type UpdateLinkCategoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Style       string `json:"style" binding:"required,oneof=card list"`
	Description string `json:"description"`
}

// UpdateLinkTagRequest 是后台管理员更新友链标签的请求结构。
type UpdateLinkTagRequest struct {
	Name  string `json:"name" binding:"required"`
	Color string `json:"color"`
}

// ImportLinkItem 是导入友链时的单个友链数据结构。
type ImportLinkItem struct {
	Name         string `json:"name" binding:"required"`
	URL          string `json:"url" binding:"required,url"`
	Logo         string `json:"logo"`
	Description  string `json:"description"`
	Siteshot     string `json:"siteshot"`
	CategoryName string `json:"category_name"`                                                      // 分类名称，如果不存在会自动创建
	TagName      string `json:"tag_name"`                                                           // 标签名称，可选，如果不存在会自动创建
	Status       string `json:"status" binding:"omitempty,oneof=PENDING APPROVED REJECTED INVALID"` // 默认为 PENDING
}

// ImportLinksRequest 是批量导入友链的请求结构。
type ImportLinksRequest struct {
	Links             []ImportLinkItem `json:"links" binding:"required,dive"`
	SkipDuplicates    bool             `json:"skip_duplicates"`     // 是否跳过重复的友链（基于URL判断）
	CreateCategories  bool             `json:"create_categories"`   // 是否自动创建不存在的分类
	CreateTags        bool             `json:"create_tags"`         // 是否自动创建不存在的标签
	DefaultCategoryID *int             `json:"default_category_id"` // 如果分类不存在且不允许创建时使用的默认分类ID
}

// ImportLinksResponse 是批量导入友链的响应结构。
type ImportLinksResponse struct {
	Total       int                 `json:"total"`        // 总共尝试导入的数量
	Success     int                 `json:"success"`      // 成功导入的数量
	Failed      int                 `json:"failed"`       // 失败的数量
	Skipped     int                 `json:"skipped"`      // 跳过的数量（重复）
	SuccessList []*LinkDTO          `json:"success_list"` // 成功导入的友链列表
	FailedList  []ImportLinkFailure `json:"failed_list"`  // 失败的友链及原因
	SkippedList []ImportLinkSkipped `json:"skipped_list"` // 跳过的友链及原因
}

// ImportLinkFailure 表示导入失败的友链信息。
type ImportLinkFailure struct {
	Link   ImportLinkItem `json:"link"`
	Reason string         `json:"reason"`
}

// ImportLinkSkipped 表示跳过的友链信息。
type ImportLinkSkipped struct {
	Link   ImportLinkItem `json:"link"`
	Reason string         `json:"reason"`
}

// LinkHealthCheckResponse 是友链健康检查的响应结构。
type LinkHealthCheckResponse struct {
	Total        int   `json:"total"`         // 总共检查的友链数量
	Healthy      int   `json:"healthy"`       // 健康的友链数量
	Unhealthy    int   `json:"unhealthy"`     // 失联的友链数量
	UnhealthyIDs []int `json:"unhealthy_ids"` // 失联的友链ID列表
}

// LinkSortItem 是单个友链排序项。
type LinkSortItem struct {
	ID        int `json:"id" binding:"required"`
	SortOrder int `json:"sort_order"`
}

// BatchUpdateLinkSortRequest 是批量更新友链排序的请求结构。
type BatchUpdateLinkSortRequest struct {
	Items []LinkSortItem `json:"items" binding:"required,min=1"`
}
