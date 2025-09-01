package page

import (
	"context"
	"fmt"
	"strings"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

// Service 页面服务接口
type Service interface {
	// Create 创建页面
	Create(ctx context.Context, options *model.CreatePageOptions) (*model.Page, error)

	// GetByID 根据ID获取页面
	GetByID(ctx context.Context, id string) (*model.Page, error)

	// GetByPath 根据路径获取页面
	GetByPath(ctx context.Context, path string) (*model.Page, error)

	// List 列出页面
	List(ctx context.Context, options *model.ListPagesOptions) ([]*model.Page, int, error)

	// Update 更新页面
	Update(ctx context.Context, id string, options *model.UpdatePageOptions) (*model.Page, error)

	// Delete 删除页面
	Delete(ctx context.Context, id string) error

	// InitializeDefaultPages 初始化默认页面
	InitializeDefaultPages(ctx context.Context) error
}

// service 页面服务实现
type service struct {
	pageRepo repository.PageRepository
}

// NewService 创建页面服务
func NewService(pageRepo repository.PageRepository) Service {
	return &service{
		pageRepo: pageRepo,
	}
}

// Create 创建页面
func (s *service) Create(ctx context.Context, options *model.CreatePageOptions) (*model.Page, error) {
	// 验证路径格式
	if err := s.validatePath(options.Path); err != nil {
		return nil, err
	}

	// 检查路径是否已存在
	exists, err := s.pageRepo.ExistsByPath(ctx, options.Path, "")
	if err != nil {
		return nil, fmt.Errorf("检查路径是否存在失败: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("路径 %s 已存在", options.Path)
	}

	// 创建页面
	page, err := s.pageRepo.Create(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	return page, nil
}

// GetByID 根据ID获取页面
func (s *service) GetByID(ctx context.Context, id string) (*model.Page, error) {
	page, err := s.pageRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取页面失败: %w", err)
	}
	return page, nil
}

// GetByPath 根据路径获取页面
func (s *service) GetByPath(ctx context.Context, path string) (*model.Page, error) {
	page, err := s.pageRepo.GetByPath(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("获取页面失败: %w", err)
	}
	return page, nil
}

// List 列出页面
func (s *service) List(ctx context.Context, options *model.ListPagesOptions) ([]*model.Page, int, error) {
	pages, total, err := s.pageRepo.List(ctx, options)
	if err != nil {
		return nil, 0, fmt.Errorf("获取页面列表失败: %w", err)
	}
	return pages, total, nil
}

// Update 更新页面
func (s *service) Update(ctx context.Context, id string, options *model.UpdatePageOptions) (*model.Page, error) {
	// 获取当前页面
	currentPage, err := s.pageRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取页面失败: %w", err)
	}

	// 如果修改了路径，检查新路径是否已存在
	if options.Path != nil && *options.Path != currentPage.Path {
		if err := s.validatePath(*options.Path); err != nil {
			return nil, err
		}

		exists, err := s.pageRepo.ExistsByPath(ctx, *options.Path, id)
		if err != nil {
			return nil, fmt.Errorf("检查路径是否存在失败: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("路径 %s 已存在", *options.Path)
		}
	}

	// 更新页面
	page, err := s.pageRepo.Update(ctx, id, options)
	if err != nil {
		return nil, fmt.Errorf("更新页面失败: %w", err)
	}

	return page, nil
}

// Delete 删除页面
func (s *service) Delete(ctx context.Context, id string) error {
	// 删除页面
	if err := s.pageRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除页面失败: %w", err)
	}

	return nil
}

// InitializeDefaultPages 初始化默认页面
func (s *service) InitializeDefaultPages(ctx context.Context) error {
	defaultPages := []*model.CreatePageOptions{
		{
			Title: "隐私政策",
			Path:  "/privacy",
			Content: `<div class="container mx-auto px-4 py-8">
    <h1 class="text-3xl font-bold mb-6">隐私政策</h1>
    <div class="prose max-w-none">
        <p>本隐私政策描述了本站如何收集、使用和保护您的个人信息。</p>
        <h2>信息收集</h2>
        <p>我们可能收集以下类型的信息：</p>
        <ul>
            <li>您主动提供的信息</li>
            <li>自动收集的技术信息</li>
            <li>第三方来源的信息</li>
        </ul>
        <h2>信息使用</h2>
        <p>我们使用收集的信息来：</p>
        <ul>
            <li>提供和改进服务</li>
            <li>个性化用户体验</li>
            <li>发送通知和更新</li>
        </ul>
        <h2>信息保护</h2>
        <p>我们采取适当的安全措施来保护您的个人信息。</p>
    </div>
</div>`,
			Description: "本站的隐私政策说明",
			IsPublished: true,
			Sort:        1,
		},
		{
			Title: "Cookie 政策",
			Path:  "/cookies",
			Content: `<div class="container mx-auto px-4 py-8">
    <h1 class="text-3xl font-bold mb-6">Cookie 政策</h1>
    <div class="prose max-w-none">
        <p>本 Cookie 政策说明了本站如何使用 Cookie 和类似技术。</p>
        <h2>什么是 Cookie</h2>
        <p>Cookie 是存储在您设备上的小型文本文件，用于记住您的偏好和设置。</p>
        <h2>我们使用的 Cookie 类型</h2>
        <ul>
            <li><strong>必要 Cookie：</strong>网站正常运行所必需</li>
            <li><strong>功能 Cookie：</strong>记住您的偏好设置</li>
            <li><strong>分析 Cookie：</strong>帮助我们了解网站使用情况</li>
        </ul>
        <h2>管理 Cookie</h2>
        <p>您可以通过浏览器设置来管理 Cookie 偏好。</p>
    </div>
</div>`,
			Description: "本站的 Cookie 使用政策",
			IsPublished: true,
			Sort:        2,
		},
		{
			Title: "版权声明",
			Path:  "/copyright",
			Content: `<div class="container mx-auto px-4 py-8">
    <h1 class="text-3xl font-bold mb-6">版权声明</h1>
    <div class="prose max-w-none">
        <p>本版权声明适用于本站的所有内容。</p>
        <h2>版权保护</h2>
        <p>本站的所有内容，包括但不限于文字、图片、音频、视频等，均受版权法保护。</p>
        <h2>使用许可</h2>
        <p>未经明确许可，禁止复制、分发、修改或商业使用本站内容。</p>
        <h2>免责声明</h2>
        <p>本站内容仅供参考，不构成任何建议或承诺。</p>
        <h2>联系我们</h2>
        <p>如有版权相关问题，请联系我们。</p>
    </div>
</div>`,
			Description: "本站的版权声明",
			IsPublished: true,
			Sort:        3,
		},
	}

	for _, pageOptions := range defaultPages {
		// 检查页面是否已存在
		existingPage, err := s.pageRepo.GetByPath(ctx, pageOptions.Path)
		if err == nil && existingPage != nil {
			// 页面已存在，跳过
			continue
		}

		// 创建默认页面
		_, err = s.pageRepo.Create(ctx, pageOptions)
		if err != nil {
			return fmt.Errorf("创建默认页面 %s 失败: %w", pageOptions.Title, err)
		}
	}

	return nil
}

// validatePath 验证路径格式
func (s *service) validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("路径不能为空")
	}

	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("路径必须以 / 开头")
	}

	if strings.Contains(path, " ") {
		return fmt.Errorf("路径不能包含空格")
	}

	// 检查是否包含特殊字符
	for _, char := range []string{"<", ">", "\"", "'", "&", "?", "#", "=", "+", ";"} {
		if strings.Contains(path, char) {
			return fmt.Errorf("路径不能包含特殊字符: %s", char)
		}
	}

	return nil
}
