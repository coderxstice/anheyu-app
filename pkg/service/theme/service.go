/*
 * @Description: 主题管理服务（优化版）
 * @Author: 安知鱼
 * @Date: 2025-09-18 11:00:00
 * @LastEditTime: 2025-09-19 11:16:05
 * @LastEditors: 安知鱼
 *
 * 设计原则：
 * 1. 本地只存储必需信息，主题详情从外部API获取
 * 2. 文件系统即状态：通过static目录存在性控制渲染模式
 * 3. 数据组合：本地安装状态 + 外部API详细信息
 */
package theme

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/ent/userinstalledtheme"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

const (
	// 主题目录名称
	ThemesDirName = "themes"
	StaticDirName = "static"

	// 官方主题名称
	OfficialThemeName = "theme-anheyu"

	// 备份目录名称
	BackupDirName = "backup"

	// 外部主题商城API地址
	ThemeMarketAPI = "https://anheyuofficialwebsiteapi.anheyu.com/api/v1/themes"
)

// ThemeInfo 主题信息结构（与主题商城格式保持一致，并添加本地状态）
type ThemeInfo struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Author         string   `json:"author"`
	Description    string   `json:"description"`
	ThemeType      string   `json:"themeType"`
	RepoURL        string   `json:"repoUrl"`
	InstructionURL string   `json:"instructionUrl"`
	Price          int      `json:"price"`
	DownloadURL    string   `json:"downloadUrl"`
	Tags           []string `json:"tags"`
	PreviewURL     string   `json:"previewUrl"`
	DemoURL        string   `json:"demoUrl"`
	Version        string   `json:"version"`
	DownloadCount  int      `json:"downloadCount"`
	Rating         float64  `json:"rating"`
	IsOfficial     bool     `json:"isOfficial"`
	IsActive       bool     `json:"isActive"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`

	// 本地状态字段
	IsCurrent        bool                   `json:"is_current"`                  // 是否为当前使用的主题
	IsInstalled      bool                   `json:"is_installed"`                // 是否已安装（对于已安装主题列表始终为true）
	InstallTime      *time.Time             `json:"install_time,omitempty"`      // 安装时间
	UserConfig       map[string]interface{} `json:"user_config,omitempty"`       // 用户配置
	InstalledVersion string                 `json:"installed_version,omitempty"` // 已安装版本
}

// ThemeInstallRequest 主题安装请求（简化版）
type ThemeInstallRequest struct {
	MarketID    int    `json:"market_id"`
	ThemeName   string `json:"theme_name"`
	DownloadURL string `json:"download_url"`
	Version     string `json:"version,omitempty"`
}

// MarketTheme 主题商城主题信息（外部API格式）
type MarketTheme struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Author         string   `json:"author"`
	Description    string   `json:"description"`
	ThemeType      string   `json:"themeType"`
	RepoURL        string   `json:"repoUrl"`
	InstructionURL string   `json:"instructionUrl"`
	Price          int      `json:"price"`
	DownloadURL    string   `json:"downloadUrl"`
	Tags           []string `json:"tags"`
	PreviewURL     string   `json:"previewUrl"`
	DemoURL        string   `json:"demoUrl"`
	Version        string   `json:"version"`
	DownloadCount  int      `json:"downloadCount"`
	Rating         float64  `json:"rating"`
	IsOfficial     bool     `json:"isOfficial"`
	IsActive       bool     `json:"isActive"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// ThemeMetadata 主题元信息（theme.json格式）
type ThemeMetadata struct {
	Name        string      `json:"name" binding:"required"`
	DisplayName string      `json:"displayName" binding:"required"`
	Version     string      `json:"version" binding:"required"`
	Description string      `json:"description" binding:"required"`
	Author      interface{} `json:"author" binding:"required"` // 可以是string或object
	License     string      `json:"license"`
	Homepage    string      `json:"homepage"`
	Repository  *struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"repository"`
	Keywords    []string          `json:"keywords"`
	Category    string            `json:"category"`
	Screenshots interface{}       `json:"screenshots"` // 支持字符串或字符串数组
	Engines     map[string]string `json:"engines"`
	Features    []string          `json:"features"`
}

// AuthorInfo 作者信息结构
type AuthorInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

// ThemeValidationResult 主题验证结果
type ThemeValidationResult struct {
	IsValid       bool           `json:"is_valid"`
	Errors        []string       `json:"errors"`
	Warnings      []string       `json:"warnings"`
	Metadata      *ThemeMetadata `json:"metadata"`
	FileList      []string       `json:"file_list"`
	TotalSize     int64          `json:"total_size"`
	ExistingTheme *ThemeInfo     `json:"existing_theme,omitempty"`
}

// ThemeService 主题服务接口
type ThemeService interface {
	// 获取当前使用的主题
	GetCurrentTheme(ctx context.Context, userID uint) (*ThemeInfo, error)

	// 获取用户已安装的主题列表（组合本地数据和外部API数据）
	GetInstalledThemes(ctx context.Context, userID uint) ([]*ThemeInfo, error)

	// 安装主题（简化流程）
	InstallTheme(ctx context.Context, userID uint, req *ThemeInstallRequest) error

	// 切换到指定主题
	SwitchToTheme(ctx context.Context, userID uint, themeName string) error

	// 切换到官方主题
	SwitchToOfficial(ctx context.Context, userID uint) error

	// 卸载主题
	UninstallTheme(ctx context.Context, userID uint, themeName string) error

	// 检查是否使用静态模式（是否存在static目录）
	IsStaticModeActive() bool

	// 获取主题商城列表（从外部API获取）
	GetThemeMarketList(ctx context.Context) ([]*MarketTheme, error)

	// 上传主题压缩包
	UploadTheme(ctx context.Context, userID uint, file *multipart.FileHeader, forceUpdate ...bool) (*ThemeInfo, error)

	// 验证主题压缩包
	ValidateThemePackage(ctx context.Context, userID uint, file *multipart.FileHeader) (*ThemeValidationResult, error)

	// 修复用户主题的当前状态数据一致性
	FixThemeCurrentStatus(ctx context.Context, userID uint) error
}

// themeService 主题服务实现
type themeService struct {
	db       *ent.Client
	userRepo repository.UserRepository
}

// NewThemeService 创建主题服务实例
func NewThemeService(db *ent.Client, userRepo repository.UserRepository) ThemeService {
	return &themeService{
		db:       db,
		userRepo: userRepo,
	}
}

// GetThemeMarketList 获取主题商城列表（从外部API获取）
func (s *themeService) GetThemeMarketList(ctx context.Context) ([]*MarketTheme, error) {
	// 创建HTTP客户端请求
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", ThemeMarketAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Anheyu-App/1.0")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		// 如果外部API调用失败，返回空列表而不是错误，确保系统仍可用
		log.Printf("调用主题商城API失败: %v，返回空列表", err)
		return []*MarketTheme{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("主题商城API返回错误状态码: %d，返回空列表", resp.StatusCode)
		return []*MarketTheme{}, nil
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取API响应失败: %v，返回空列表", err)
		return []*MarketTheme{}, nil
	}

	// 定义API响应结构
	type APIResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			List  []*MarketTheme `json:"list"`
			Total int            `json:"total"`
		} `json:"data"`
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.Printf("解析API响应失败: %v，返回空列表", err)
		return []*MarketTheme{}, nil
	}

	// 检查API响应码
	if apiResp.Code != 200 {
		log.Printf("API返回错误码: %d, 消息: %s，返回空列表", apiResp.Code, apiResp.Message)
		return []*MarketTheme{}, nil
	}

	// 返回主题列表
	if apiResp.Data.List == nil {
		return []*MarketTheme{}, nil
	}

	log.Printf("成功从主题商城API获取到 %d 个主题", len(apiResp.Data.List))
	return apiResp.Data.List, nil
}

// GetCurrentTheme 获取当前使用的主题
func (s *themeService) GetCurrentTheme(ctx context.Context, userID uint) (*ThemeInfo, error) {
	// 核心逻辑：如果没有static目录，一定是使用官方主题
	staticModeActive := s.IsStaticModeActive()
	now := time.Now()

	if !staticModeActive {
		// 没有static目录，使用官方主题
		log.Printf("用户 %d 当前使用官方主题（无static目录）", userID)
		return &ThemeInfo{
			ID:             1,
			Name:           "安和鱼官方主题",
			Author:         "安知鱼",
			Description:    "安知鱼官方主题",
			ThemeType:      "community",
			RepoURL:        "https://github.com/anzhiyu-c/anheyu-app-frontend",
			InstructionURL: "",
			Price:          0,
			DownloadURL:    "",
			Tags:           []string{"极致性能", "简洁", "不简单"},
			PreviewURL:     "https://upload-bbs.miyoushe.com/upload/2025/09/18/125766904/359dbf5b0ce07e56a960b31063c44280_4491727436207297404.png",
			DemoURL:        "https://demo.anheyu.com/",
			Version:        "1.0.0",
			DownloadCount:  0,
			Rating:         0,
			IsOfficial:     true,
			IsActive:       true,
			CreatedAt:      "2025-09-18 07:58:32",
			UpdatedAt:      "2025-09-18 13:17:10",
			IsCurrent:      true,
			IsInstalled:    false,
			InstallTime:    &now,
		}, nil
	}

	// 有static目录，查找数据库中的当前主题
	localTheme, err := s.db.UserInstalledTheme.
		Query().
		Where(
			userinstalledtheme.UserID(userID),
			userinstalledtheme.IsCurrent(true),
		).
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			// 有static目录但没有数据库记录，这是异常情况
			log.Printf("警告：用户 %d 有static目录但数据库中没有当前主题记录", userID)
			// 返回一个未知的外部主题
			return &ThemeInfo{
				ID:          0,
				Name:        "外部主题",
				Author:      "Unknown",
				Description: "当前使用的外部主题",
				ThemeType:   "community",
				Version:     "Unknown",
				Tags:        []string{},
				IsOfficial:  false,
				IsActive:    true,
				CreatedAt:   now.Format("2006-01-02 15:04:05"),
				UpdatedAt:   now.Format("2006-01-02 15:04:05"),
				IsCurrent:   true,
				IsInstalled: false,
				InstallTime: &now,
			}, nil
		}
		return nil, fmt.Errorf("查询当前主题失败: %w", err)
	}

	// 组合本地数据和外部API数据
	themeInfo := &ThemeInfo{
		ID:               int(localTheme.ID),
		Name:             localTheme.ThemeName,
		Author:           "未知",
		Description:      "本地安装的主题",
		ThemeType:        "community",
		Version:          localTheme.InstalledVersion,
		Tags:             []string{},
		IsOfficial:       false,
		IsActive:         true,
		CreatedAt:        localTheme.InstallTime.Format("2006-01-02 15:04:05"),
		UpdatedAt:        localTheme.InstallTime.Format("2006-01-02 15:04:05"),
		IsCurrent:        localTheme.IsCurrent,
		IsInstalled:      true,
		InstallTime:      &localTheme.InstallTime,
		UserConfig:       localTheme.UserThemeConfig,
		InstalledVersion: localTheme.InstalledVersion,
	}

	return themeInfo, nil
}

// GetInstalledThemes 获取用户已安装的主题列表
func (s *themeService) GetInstalledThemes(ctx context.Context, userID uint) ([]*ThemeInfo, error) {
	// 首先自动修复数据状态不一致问题
	if err := s.FixThemeCurrentStatus(ctx, userID); err != nil {
		log.Printf("警告：自动修复用户 %d 主题状态失败: %v", userID, err)
		// 继续执行，不因修复失败而中断获取主题列表
	}

	// 从数据库获取已安装主题
	localThemes, err := s.db.UserInstalledTheme.
		Query().
		Where(userinstalledtheme.UserID(userID)).
		Order(ent.Desc(userinstalledtheme.FieldInstallTime)).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("查询已安装主题失败: %w", err)
	}

	// 获取主题商城数据（用于组合）
	marketThemes, err := s.GetThemeMarketList(ctx)
	if err != nil {
		log.Printf("获取主题商城数据失败: %v", err)
		marketThemes = []*MarketTheme{} // 继续处理，只是没有商城数据
	}

	// 创建主题映射表
	marketThemeMap := make(map[string]*MarketTheme)
	for _, theme := range marketThemes {
		marketThemeMap[theme.Name] = theme
	}

	// 组合本地数据和外部API数据
	result := make([]*ThemeInfo, 0, len(localThemes)+1)

	for _, localTheme := range localThemes {
		marketTheme := marketThemeMap[localTheme.ThemeName]

		// 组合本地数据和市场数据
		themeInfo := &ThemeInfo{
			// 本地状态字段
			IsCurrent:        localTheme.IsCurrent,
			IsInstalled:      true,
			InstallTime:      &localTheme.InstallTime,
			UserConfig:       localTheme.UserThemeConfig,
			InstalledVersion: localTheme.InstalledVersion,
		}

		// 如果有市场数据，使用市场数据填充详细信息
		if marketTheme != nil {
			themeInfo.ID = marketTheme.ID
			themeInfo.Name = marketTheme.Name
			themeInfo.Author = marketTheme.Author
			themeInfo.Description = marketTheme.Description
			themeInfo.ThemeType = marketTheme.ThemeType
			themeInfo.RepoURL = marketTheme.RepoURL
			themeInfo.InstructionURL = marketTheme.InstructionURL
			themeInfo.Price = marketTheme.Price
			themeInfo.DownloadURL = marketTheme.DownloadURL
			themeInfo.Tags = marketTheme.Tags
			themeInfo.PreviewURL = marketTheme.PreviewURL
			themeInfo.DemoURL = marketTheme.DemoURL
			themeInfo.Version = marketTheme.Version
			themeInfo.DownloadCount = marketTheme.DownloadCount
			themeInfo.Rating = marketTheme.Rating
			themeInfo.IsOfficial = marketTheme.IsOfficial
			themeInfo.IsActive = marketTheme.IsActive
			themeInfo.CreatedAt = marketTheme.CreatedAt
			themeInfo.UpdatedAt = marketTheme.UpdatedAt
		} else {
			// 如果没有市场数据，尝试从本地 theme.json 读取信息
			localMetadata, err := s.loadThemeMetadataFromDisk(localTheme.ThemeName)
			if err != nil {
				log.Printf("读取本地主题 %s 的 theme.json 失败: %v", localTheme.ThemeName, err)
				// 使用默认信息作为备用
				themeInfo.ID = int(localTheme.ID)
				themeInfo.Name = localTheme.ThemeName
				themeInfo.Author = "未知"
				themeInfo.Description = "本地安装的主题"
				themeInfo.ThemeType = "community"
				themeInfo.Version = localTheme.InstalledVersion
				themeInfo.Tags = []string{}
				themeInfo.IsOfficial = false
				themeInfo.IsActive = true
				themeInfo.CreatedAt = localTheme.InstallTime.Format("2006-01-02 15:04:05")
				themeInfo.UpdatedAt = localTheme.InstallTime.Format("2006-01-02 15:04:05")
			} else {
				// 使用本地 theme.json 的数据
				authorName := s.extractAuthorName(localMetadata.Author)
				previewURL := s.extractFirstScreenshot(localMetadata.Screenshots)

				// 处理仓库URL
				repoURL := ""
				if localMetadata.Repository != nil {
					repoURL = localMetadata.Repository.URL
				}

				// 处理主题类型
				themeType := "community"
				if localMetadata.Category != "" {
					themeType = localMetadata.Category
				}

				themeInfo.ID = int(localTheme.ID)
				themeInfo.Name = localMetadata.Name
				themeInfo.Author = authorName
				themeInfo.Description = localMetadata.Description
				themeInfo.ThemeType = themeType
				themeInfo.RepoURL = repoURL
				themeInfo.InstructionURL = localMetadata.Homepage
				themeInfo.Price = 0
				themeInfo.DownloadURL = ""
				themeInfo.Tags = localMetadata.Keywords
				themeInfo.PreviewURL = previewURL
				themeInfo.DemoURL = ""
				themeInfo.Version = localMetadata.Version
				themeInfo.DownloadCount = 0
				themeInfo.Rating = 0
				themeInfo.IsOfficial = false
				themeInfo.IsActive = true
				themeInfo.CreatedAt = localTheme.InstallTime.Format("2006-01-02 15:04:05")
				themeInfo.UpdatedAt = localTheme.InstallTime.Format("2006-01-02 15:04:05")
			}
		}

		result = append(result, themeInfo)
	}

	// 检查静态模式状态
	staticModeActive := s.IsStaticModeActive()

	// 根据静态模式状态调整主题的当前使用状态
	if !staticModeActive {
		// 如果没有static目录，官方主题为当前使用，所有数据库主题都不应该是当前使用
		for _, theme := range result {
			if theme.IsCurrent {
				log.Printf("警告：用户 %d 在官方主题模式下，数据库主题 %s 仍标记为当前使用，将被修正", userID, theme.Name)
				theme.IsCurrent = false
			}
		}
	}

	// 检查是否有官方主题，如果没有则添加
	hasOfficial := false
	for _, theme := range result {
		if s.isOfficialTheme(theme.Name) {
			hasOfficial = true
			break
		}
	}

	if !hasOfficial {
		now := time.Now()
		// 核心逻辑：如果没有static目录，官方主题就是当前使用的
		isOfficialCurrent := !staticModeActive

		officialTheme := &ThemeInfo{
			ID:             1,
			Name:           "安和鱼官方主题",
			Author:         "安知鱼",
			Description:    "这是一款简洁而不简单的主题。",
			ThemeType:      "community",
			RepoURL:        "https://github.com/anzhiyu-c/anheyu-app-frontend",
			InstructionURL: "",
			Price:          0,
			DownloadURL:    "",
			Tags:           []string{"极致性能", "简洁", "不简单"},
			PreviewURL:     "https://upload-bbs.miyoushe.com/upload/2025/09/18/125766904/359dbf5b0ce07e56a960b31063c44280_4491727436207297404.png",
			DemoURL:        "https://demo.anheyu.com/",
			Version:        "latest",
			DownloadCount:  0,
			Rating:         0,
			IsOfficial:     true,
			IsActive:       true,
			CreatedAt:      "2025-09-18 07:58:32",
			UpdatedAt:      "2025-09-18 13:17:10",

			// 本地状态字段
			IsCurrent:   isOfficialCurrent,
			IsInstalled: false,
			InstallTime: &now,
		}
		// 将官方主题插入到列表开头
		result = append([]*ThemeInfo{officialTheme}, result...)

		log.Printf("添加官方主题，是否为当前使用: %v (静态模式激活: %v)",
			isOfficialCurrent, staticModeActive)
	}

	// 数据一致性检查和最终验证
	currentThemeCount := 0
	var currentThemeNames []string
	for _, theme := range result {
		if theme.IsCurrent {
			currentThemeCount++
			currentThemeNames = append(currentThemeNames, theme.Name)
		}
	}

	if currentThemeCount != 1 {
		log.Printf("警告：用户 %d 有 %d 个当前主题 %v，期望只有1个 (静态模式: %v)",
			userID, currentThemeCount, currentThemeNames, staticModeActive)
	} else {
		log.Printf("用户 %d 当前主题状态正常: %s (静态模式: %v)",
			userID, currentThemeNames[0], staticModeActive)
	}

	return result, nil
}

// InstallTheme 安装主题（简化流程）
func (s *themeService) InstallTheme(ctx context.Context, userID uint, req *ThemeInstallRequest) error {
	// 1. 检查主题是否已经安装
	exists, err := s.db.UserInstalledTheme.
		Query().
		Where(
			userinstalledtheme.UserID(userID),
			userinstalledtheme.ThemeName(req.ThemeName),
		).
		Exist(ctx)

	if err != nil {
		return fmt.Errorf("检查主题是否存在失败: %w", err)
	}

	if exists {
		return fmt.Errorf("主题 %s 已经安装", req.ThemeName)
	}

	// 2. 下载并解压主题文件
	themeDir := filepath.Join(ThemesDirName, req.ThemeName)
	if err := s.downloadAndExtractTheme(req.DownloadURL, themeDir); err != nil {
		return fmt.Errorf("下载主题失败: %w", err)
	}

	// 3. 验证主题文件完整性
	if err := s.validateThemeFiles(themeDir); err != nil {
		// 清理已下载的文件
		os.RemoveAll(themeDir)
		return fmt.Errorf("主题文件验证失败: %w", err)
	}

	// 4. 在数据库中记录主题信息（只存储必要的本地信息）
	createBuilder := s.db.UserInstalledTheme.
		Create().
		SetUserID(userID).
		SetThemeName(req.ThemeName).
		SetInstallTime(time.Now())

	if req.MarketID > 0 {
		createBuilder = createBuilder.SetThemeMarketID(req.MarketID)
	}

	if req.Version != "" {
		createBuilder = createBuilder.SetInstalledVersion(req.Version)
	}

	_, err = createBuilder.Save(ctx)
	if err != nil {
		// 清理已下载的文件
		os.RemoveAll(themeDir)
		return fmt.Errorf("保存主题信息失败: %w", err)
	}

	log.Printf("主题 %s 安装成功", req.ThemeName)
	return nil
}

// combineThemeInfo 组合本地数据和外部API数据
func (s *themeService) combineThemeInfo(ctx context.Context, localTheme *ent.UserInstalledTheme, marketTheme *MarketTheme) (*ThemeInfo, error) {
	themeInfo := &ThemeInfo{
		// 本地状态字段
		ID:               int(localTheme.ID),
		IsCurrent:        localTheme.IsCurrent,
		IsInstalled:      true,
		InstallTime:      &localTheme.InstallTime,
		UserConfig:       localTheme.UserThemeConfig,
		InstalledVersion: localTheme.InstalledVersion,
	}

	// 如果有商城数据，填充详细信息
	if marketTheme != nil {
		themeInfo.ID = marketTheme.ID
		themeInfo.Name = marketTheme.Name
		themeInfo.Author = marketTheme.Author
		themeInfo.Description = marketTheme.Description
		themeInfo.Version = marketTheme.Version
		themeInfo.ThemeType = marketTheme.ThemeType
		themeInfo.Tags = marketTheme.Tags
		themeInfo.RepoURL = marketTheme.RepoURL
		themeInfo.InstructionURL = marketTheme.InstructionURL
		themeInfo.Price = marketTheme.Price
		themeInfo.DownloadURL = marketTheme.DownloadURL
		themeInfo.PreviewURL = marketTheme.PreviewURL
		themeInfo.DemoURL = marketTheme.DemoURL
		themeInfo.DownloadCount = marketTheme.DownloadCount
		themeInfo.Rating = marketTheme.Rating
		themeInfo.IsOfficial = marketTheme.IsOfficial
		themeInfo.IsActive = marketTheme.IsActive
		themeInfo.CreatedAt = marketTheme.CreatedAt
		themeInfo.UpdatedAt = marketTheme.UpdatedAt
	} else {
		// 如果没有商城数据，使用基本信息
		themeInfo.Name = localTheme.ThemeName
		themeInfo.Author = "未知"
		themeInfo.Description = "本地安装的主题"
		themeInfo.ThemeType = "community"
		themeInfo.Version = localTheme.InstalledVersion
		themeInfo.Tags = []string{}
		themeInfo.IsOfficial = localTheme.ThemeName == OfficialThemeName
		themeInfo.IsActive = true
		themeInfo.CreatedAt = localTheme.InstallTime.Format("2006-01-02 15:04:05")
		themeInfo.UpdatedAt = localTheme.InstallTime.Format("2006-01-02 15:04:05")
	}

	return themeInfo, nil
}

// isOfficialTheme 判断是否是官方主题
func (s *themeService) isOfficialTheme(themeName string) bool {
	officialNames := []string{
		OfficialThemeName, // "theme-anheyu"
		"安和鱼官方主题",         // 显示名称
		"安知鱼官方主题",         // 另一个可能的显示名称
		"官方主题",            // 可能的简称
	}

	for _, officialName := range officialNames {
		if themeName == officialName {
			return true
		}
	}
	return false
}

// SwitchToTheme 切换到指定主题
func (s *themeService) SwitchToTheme(ctx context.Context, userID uint, themeName string) error {
	// 检查是否是官方主题
	if s.isOfficialTheme(themeName) {
		log.Printf("用户 %d 请求切换到官方主题: %s", userID, themeName)
		return s.SwitchToOfficial(ctx, userID)
	}

	// 1. 检查主题是否已安装
	theme, err := s.db.UserInstalledTheme.
		Query().
		Where(
			userinstalledtheme.UserID(userID),
			userinstalledtheme.ThemeName(themeName),
		).
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("主题 %s 未安装", themeName)
		}
		return fmt.Errorf("查询主题失败: %w", err)
	}

	// 2. 检查主题文件是否存在
	themeDir := filepath.Join(ThemesDirName, themeName)
	if err := s.validateThemeFiles(themeDir); err != nil {
		return fmt.Errorf("主题文件不完整: %w", err)
	}

	// 3. 备份当前static目录（如果存在）
	backupPath := ""
	if s.IsStaticModeActive() {
		backupPath = filepath.Join(BackupDirName, fmt.Sprintf("static_backup_%d", time.Now().Unix()))
		if err := s.backupDirectory(StaticDirName, backupPath); err != nil {
			return fmt.Errorf("备份静态文件失败: %w", err)
		}
	}

	// 4. 复制主题文件到static目录
	if err := s.copyThemeToStatic(themeDir); err != nil {
		// 如果失败，恢复备份
		if backupPath != "" {
			s.restoreFromBackup(backupPath, StaticDirName)
		}
		return fmt.Errorf("复制主题文件失败: %w", err)
	}

	// 5. 更新数据库记录
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	// 取消当前所有主题的激活状态
	_, err = tx.UserInstalledTheme.
		Update().
		Where(userinstalledtheme.UserID(userID)).
		SetIsCurrent(false).
		Save(ctx)

	if err != nil {
		tx.Rollback()
		if backupPath != "" {
			s.restoreFromBackup(backupPath, StaticDirName)
		}
		return fmt.Errorf("更新主题状态失败: %w", err)
	}

	// 设置新主题为当前主题
	_, err = tx.UserInstalledTheme.
		UpdateOneID(theme.ID).
		SetIsCurrent(true).
		Save(ctx)

	if err != nil {
		tx.Rollback()
		if backupPath != "" {
			s.restoreFromBackup(backupPath, StaticDirName)
		}
		return fmt.Errorf("设置当前主题失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		if backupPath != "" {
			s.restoreFromBackup(backupPath, StaticDirName)
		}
		return fmt.Errorf("提交事务失败: %w", err)
	}

	// 6. 验证切换后的状态一致性
	currentThemesCount, err := s.db.UserInstalledTheme.
		Query().
		Where(
			userinstalledtheme.UserID(userID),
			userinstalledtheme.IsCurrent(true),
		).
		Count(ctx)

	if err != nil {
		log.Printf("警告：验证用户 %d 当前主题状态失败: %v", userID, err)
	} else if currentThemesCount != 1 {
		log.Printf("警告：用户 %d 在主题切换后有 %d 个当前主题，状态异常", userID, currentThemesCount)
	}

	// 7. 清理备份文件
	if backupPath != "" {
		os.RemoveAll(backupPath)
	}

	log.Printf("成功切换到主题 %s", themeName)
	return nil
}

// SwitchToOfficial 切换到官方主题
func (s *themeService) SwitchToOfficial(ctx context.Context, userID uint) error {
	// 1. 备份当前static目录（如果存在）
	backupPath := ""
	if s.IsStaticModeActive() {
		backupPath = filepath.Join(BackupDirName, fmt.Sprintf("static_backup_%d", time.Now().Unix()))
		if err := s.backupDirectory(StaticDirName, backupPath); err != nil {
			return fmt.Errorf("备份静态文件失败: %w", err)
		}
	}

	// 2. 安全删除static目录
	if err := s.safeRemoveStaticDir(); err != nil {
		if backupPath != "" {
			s.restoreFromBackup(backupPath, StaticDirName)
		}
		return fmt.Errorf("删除静态目录失败: %w", err)
	}

	// 3. 更新数据库记录
	_, err := s.db.UserInstalledTheme.
		Update().
		Where(userinstalledtheme.UserID(userID)).
		SetIsCurrent(false).
		Save(ctx)

	if err != nil {
		if backupPath != "" {
			s.restoreFromBackup(backupPath, StaticDirName)
		}
		return fmt.Errorf("更新主题状态失败: %w", err)
	}

	// 4. 验证切换后的状态一致性
	currentThemesCount, err := s.db.UserInstalledTheme.
		Query().
		Where(
			userinstalledtheme.UserID(userID),
			userinstalledtheme.IsCurrent(true),
		).
		Count(ctx)

	if err != nil {
		log.Printf("警告：验证用户 %d 当前主题状态失败: %v", userID, err)
	} else if currentThemesCount > 0 {
		log.Printf("警告：用户 %d 切换到官方主题后仍有 %d 个数据库主题标记为当前，状态异常", userID, currentThemesCount)
	}

	// 5. 清理备份文件
	if backupPath != "" {
		os.RemoveAll(backupPath)
	}

	log.Printf("成功切换到官方主题")
	return nil
}

// UninstallTheme 卸载主题
func (s *themeService) UninstallTheme(ctx context.Context, userID uint, themeName string) error {
	if s.isOfficialTheme(themeName) {
		return fmt.Errorf("不能卸载官方主题")
	}

	// 1. 查询主题记录
	theme, err := s.db.UserInstalledTheme.
		Query().
		Where(
			userinstalledtheme.UserID(userID),
			userinstalledtheme.ThemeName(themeName),
		).
		First(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("主题 %s 未安装", themeName)
		}
		return fmt.Errorf("查询主题失败: %w", err)
	}

	// 2. 检查是否是真正的当前使用主题（考虑静态模式）
	staticModeActive := s.IsStaticModeActive()

	// 判断是否真的是当前使用的主题
	isReallyCurrentTheme := false
	if staticModeActive {
		// 有static目录时，检查数据库状态
		isReallyCurrentTheme = theme.IsCurrent
	} else {
		// 没有static目录时，官方主题是当前使用的，数据库主题都不是当前使用
		isReallyCurrentTheme = false

		// 如果数据库中标记为当前使用但实际不是，记录警告并自动修正
		if theme.IsCurrent {
			log.Printf("警告：用户 %d 的主题 %s 在数据库中标记为当前使用，但实际使用的是官方主题，将自动修正", userID, themeName)
			// 自动修正数据库状态
			_, updateErr := s.db.UserInstalledTheme.
				UpdateOneID(theme.ID).
				SetIsCurrent(false).
				Save(ctx)
			if updateErr != nil {
				log.Printf("警告：自动修正主题 %s 的当前状态失败: %v", themeName, updateErr)
			}
		}
	}

	if isReallyCurrentTheme {
		return fmt.Errorf("不能卸载当前使用的主题，请先切换到其他主题")
	}

	// 3. 删除主题文件
	themeDir := filepath.Join(ThemesDirName, themeName)
	if err := os.RemoveAll(themeDir); err != nil {
		log.Printf("警告：删除主题文件夹失败: %v", err)
		// 继续执行，不因为文件删除失败而中断
	}

	// 4. 删除数据库记录
	if err := s.db.UserInstalledTheme.DeleteOneID(theme.ID).Exec(ctx); err != nil {
		return fmt.Errorf("删除主题记录失败: %w", err)
	}

	log.Printf("主题 %s 卸载成功", themeName)
	return nil
}

// IsStaticModeActive 检查是否使用静态模式
func (s *themeService) IsStaticModeActive() bool {
	// 检查 static 目录是否存在
	if _, err := os.Stat(StaticDirName); os.IsNotExist(err) {
		return false
	}

	// 检查 index.html 是否存在
	indexPath := filepath.Join(StaticDirName, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return false
	}

	// 额外检查：确保 index.html 不是空文件
	if fileInfo, err := os.Stat(indexPath); err == nil {
		if fileInfo.Size() == 0 {
			log.Printf("警告：发现空的 index.html 文件，视为非静态模式")
			return false
		}
	}

	// 检查是否有其他必要的静态文件（可选）
	// 确保这是一个真正的主题目录，而不是意外创建的空目录
	entries, err := os.ReadDir(StaticDirName)
	if err != nil {
		return false
	}

	// 如果目录只有 index.html 且没有其他文件，可能是意外创建的
	if len(entries) == 1 && entries[0].Name() == "index.html" {
		// 检查 index.html 内容是否像一个真正的 HTML 文件
		content, err := os.ReadFile(indexPath)
		if err != nil {
			return false
		}

		contentStr := string(content)
		// 简单检查是否包含基本的 HTML 结构
		if !strings.Contains(strings.ToLower(contentStr), "<html") &&
			!strings.Contains(strings.ToLower(contentStr), "<!doctype") {
			log.Printf("警告：index.html 似乎不是有效的 HTML 文件，视为非静态模式")
			return false
		}
	}

	return true
}

// downloadAndExtractTheme 下载并解压主题
func (s *themeService) downloadAndExtractTheme(downloadURL, themeDir string) error {
	// 创建临时文件
	tempFile, err := os.CreateTemp("", "theme_*.zip")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 下载文件
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 复制到临时文件
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return fmt.Errorf("保存下载文件失败: %w", err)
	}

	// 解压到主题目录
	return s.extractZip(tempFile.Name(), themeDir)
}

// extractZip 解压zip文件
func (s *themeService) extractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 检测是否有根目录前缀
	var rootPrefix string
	for _, file := range reader.File {
		if strings.Contains(file.Name, "/") {
			parts := strings.Split(file.Name, "/")
			if len(parts) > 1 {
				// 检查是否有 theme.json 或 index.html 在这个子目录中
				potentialPrefix := parts[0] + "/"
				if strings.HasSuffix(file.Name, "theme.json") || strings.HasSuffix(file.Name, "index.html") {
					rootPrefix = potentialPrefix
					log.Printf("解压时检测到主题文件在子目录中: %s", rootPrefix)
					break
				}
			}
		}
	}

	// 创建目标目录
	os.MkdirAll(destDir, 0755)

	for _, file := range reader.File {
		// 防止路径遍历攻击
		if strings.Contains(file.Name, "..") {
			continue
		}

		// 处理子目录前缀
		targetPath := file.Name
		if rootPrefix != "" && strings.HasPrefix(file.Name, rootPrefix) {
			targetPath = strings.TrimPrefix(file.Name, rootPrefix)
		}

		// 如果去除前缀后路径为空，跳过
		if targetPath == "" {
			continue
		}

		path := filepath.Join(destDir, targetPath)

		// 确保目标路径在目标目录内（再次防止路径遍历）
		if !strings.HasPrefix(path, destDir) {
			log.Printf("跳过不安全的路径: %s", path)
			continue
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.FileInfo().Mode())
			continue
		}

		// 创建文件的父目录
		os.MkdirAll(filepath.Dir(path), 0755)

		// 创建文件
		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
		if err != nil {
			return err
		}
		defer targetFile.Close()

		_, err = io.Copy(targetFile, fileReader)
		if err != nil {
			return err
		}

		log.Printf("解压文件: %s -> %s", file.Name, targetPath)
	}

	return nil
}

// validateThemeFiles 验证主题文件完整性
func (s *themeService) validateThemeFiles(themeDir string) error {
	// 检查index.html是否存在
	indexPath := filepath.Join(themeDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("缺少必需的 index.html 文件")
	}

	// 检查static目录是否存在
	staticPath := filepath.Join(themeDir, "static")
	if _, err := os.Stat(staticPath); os.IsNotExist(err) {
		return fmt.Errorf("缺少必需的 static 目录")
	}

	return nil
}

// backupDirectory 备份目录
func (s *themeService) backupDirectory(srcDir, backupDir string) error {
	os.MkdirAll(filepath.Dir(backupDir), 0755)
	return s.copyDirectory(srcDir, backupDir)
}

// restoreFromBackup 从备份恢复
func (s *themeService) restoreFromBackup(backupDir, destDir string) error {
	// 如果目标是static目录，使用安全删除方法
	if destDir == StaticDirName {
		if err := s.safeRemoveStaticDir(); err != nil {
			log.Printf("警告：恢复时清空static目录失败，继续尝试恢复: %v", err)
		}
		// 确保目录存在
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("创建恢复目录失败: %w", err)
		}
	} else {
		// 对于非static目录，直接删除
		os.RemoveAll(destDir)
	}

	// 从备份恢复
	return s.copyDirectory(backupDir, destDir)
}

// copyThemeToStatic 复制主题文件到static目录
func (s *themeService) copyThemeToStatic(themeDir string) error {
	// 先安全清空static目录
	if err := s.safeRemoveStaticDir(); err != nil {
		log.Printf("警告：清空static目录失败，继续尝试复制: %v", err)
		// 即使清空失败也继续，让copyDirectory去处理文件覆盖
	}

	// 确保static目录存在
	if err := os.MkdirAll(StaticDirName, 0755); err != nil {
		return fmt.Errorf("创建static目录失败: %w", err)
	}

	// 复制整个主题目录内容到static
	return s.copyDirectory(themeDir, StaticDirName)
}

// copyDirectory 复制目录
func (s *themeService) copyDirectory(srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return s.copyFile(path, destPath)
	})
}

// copyFile 复制文件
func (s *themeService) copyFile(srcPath, destPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建目标目录
	os.MkdirAll(filepath.Dir(destPath), 0755)

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

// UploadTheme 上传主题压缩包
func (s *themeService) UploadTheme(ctx context.Context, userID uint, file *multipart.FileHeader, forceUpdate ...bool) (*ThemeInfo, error) {
	// 解析可选的 forceUpdate 参数
	isForceUpdate := len(forceUpdate) > 0 && forceUpdate[0]
	// 1. 验证主题压缩包
	validationResult, err := s.ValidateThemePackage(ctx, userID, file)
	if err != nil {
		return nil, fmt.Errorf("验证主题包失败: %w", err)
	}

	if !validationResult.IsValid {
		return nil, fmt.Errorf("主题包验证失败: %s", strings.Join(validationResult.Errors, "; "))
	}

	metadata := validationResult.Metadata
	if metadata == nil {
		return nil, fmt.Errorf("无法获取主题元信息")
	}

	// 2. 检查主题是否已安装
	existingInstallation, err := s.db.UserInstalledTheme.
		Query().
		Where(
			userinstalledtheme.UserID(userID),
			userinstalledtheme.ThemeName(metadata.Name),
		).
		Only(ctx)

	var isUpdate bool
	if err == nil {
		// 主题已安装
		if !isForceUpdate {
			// 如果没有强制更新标志，说明前端没有经过版本比较流程，直接返回错误
			// 正常流程应该是前端先调用 ValidateThemePackage，发现重复后进行版本比较和用户确认
			return nil, fmt.Errorf("主题 %s 已经安装，请使用版本更新功能。当前版本: %s",
				metadata.Name, existingInstallation.InstalledVersion)
		}
		isUpdate = true
		log.Printf("强制更新主题 %s: %s -> %s", metadata.Name, existingInstallation.InstalledVersion, metadata.Version)
	} else if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("检查主题是否存在失败: %w", err)
	}

	// 3. 保存上传的文件到临时位置
	tempFile, err := s.saveUploadedFile(file)
	if err != nil {
		return nil, fmt.Errorf("保存上传文件失败: %w", err)
	}
	defer os.Remove(tempFile)

	// 4. 解压主题到目标目录
	themeDir := filepath.Join(ThemesDirName, metadata.Name)
	if err := s.extractZip(tempFile, themeDir); err != nil {
		return nil, fmt.Errorf("解压主题失败: %w", err)
	}

	// 5. 再次验证解压后的文件
	if err := s.validateExtractedTheme(themeDir, metadata); err != nil {
		// 清理已解压的文件
		os.RemoveAll(themeDir)
		return nil, fmt.Errorf("解压后验证失败: %w", err)
	}

	// 6. 在数据库中记录主题信息
	if isUpdate {
		// 更新现有记录
		_, err = existingInstallation.
			Update().
			SetInstalledVersion(metadata.Version).
			SetInstallTime(time.Now()).
			Save(ctx)

		if err != nil {
			// 清理已解压的文件
			os.RemoveAll(themeDir)
			return nil, fmt.Errorf("更新主题信息失败: %w", err)
		}
	} else {
		// 创建新记录
		createBuilder := s.db.UserInstalledTheme.
			Create().
			SetUserID(userID).
			SetThemeName(metadata.Name).
			SetInstallTime(time.Now()).
			SetInstalledVersion(metadata.Version)

		// 设置默认用户配置（空配置）
		createBuilder = createBuilder.SetUserThemeConfig(map[string]interface{}{})

		_, err = createBuilder.Save(ctx)
		if err != nil {
			// 清理已解压的文件
			os.RemoveAll(themeDir)
			return nil, fmt.Errorf("保存主题信息失败: %w", err)
		}
	}

	// 7. 构造返回的主题信息
	authorName := s.extractAuthorName(metadata.Author)
	previewURL := s.extractFirstScreenshot(metadata.Screenshots)
	now := time.Now()

	// 处理仓库URL
	repoURL := ""
	if metadata.Repository != nil {
		repoURL = metadata.Repository.URL
	}

	// 处理主题类型，默认为 "community"
	themeType := "community"
	if metadata.Category != "" {
		themeType = metadata.Category
	}

	themeInfo := &ThemeInfo{
		ID:               0,             // 上传的主题暂时没有市场ID
		Name:             metadata.Name, // 使用主题标识名称
		Author:           authorName,
		Description:      metadata.Description,
		Version:          metadata.Version,
		ThemeType:        themeType,
		RepoURL:          repoURL,
		InstructionURL:   metadata.Homepage, // 使用 homepage 作为说明地址
		Price:            0,
		DownloadURL:      "",
		Tags:             metadata.Keywords,
		PreviewURL:       previewURL, // 从 screenshots 提取预览图
		DemoURL:          "",
		DownloadCount:    0,
		Rating:           0,
		IsOfficial:       false,
		IsActive:         true,
		CreatedAt:        now.Format("2006-01-02 15:04:05"),
		UpdatedAt:        now.Format("2006-01-02 15:04:05"),
		IsCurrent:        false,
		IsInstalled:      true,
		InstallTime:      &now,
		InstalledVersion: metadata.Version,
		UserConfig:       nil, // 不使用 Configuration 作为用户配置
	}

	if isUpdate {
		log.Printf("主题 %s 更新成功，版本: %s", metadata.Name, metadata.Version)
	} else {
		log.Printf("主题 %s 上传并安装成功，版本: %s", metadata.Name, metadata.Version)
	}
	return themeInfo, nil
}

// ValidateThemePackage 验证主题压缩包
func (s *themeService) ValidateThemePackage(ctx context.Context, userID uint, file *multipart.FileHeader) (*ThemeValidationResult, error) {
	result := &ThemeValidationResult{
		IsValid:       false,
		Errors:        []string{},
		Warnings:      []string{},
		FileList:      []string{},
		TotalSize:     file.Size,
		ExistingTheme: nil,
	}

	// 1. 基础验证
	if file.Size == 0 {
		result.Errors = append(result.Errors, "文件为空")
		return result, nil
	}

	if file.Size > 50*1024*1024 { // 50MB
		result.Errors = append(result.Errors, "文件大小超过50MB限制")
		return result, nil
	}

	// 2. 保存临时文件用于验证
	tempFile, err := s.saveUploadedFile(file)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("保存临时文件失败: %v", err))
		return result, nil
	}
	defer os.Remove(tempFile)

	// 3. 验证ZIP文件格式
	zipReader, err := zip.OpenReader(tempFile)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("ZIP文件格式错误: %v", err))
		return result, nil
	}
	defer zipReader.Close()

	// 4. 验证文件结构和内容
	var themeJsonFile *zip.File
	var indexHtmlFile *zip.File
	hasStaticDir := false
	var rootPrefix string // 检测是否有根目录前缀

	// 第一遍扫描：检测压缩包结构
	for _, file := range zipReader.File {
		// 防止路径遍历攻击
		if strings.Contains(file.Name, "..") {
			result.Errors = append(result.Errors, fmt.Sprintf("发现危险路径: %s", file.Name))
			continue
		}

		result.FileList = append(result.FileList, file.Name)

		// 检测是否所有文件都在同一个子目录中
		if strings.Contains(file.Name, "/") && rootPrefix == "" {
			parts := strings.Split(file.Name, "/")
			if len(parts) > 1 {
				// 检查是否有 theme.json 或 index.html 在这个子目录中
				potentialPrefix := parts[0] + "/"
				if strings.HasSuffix(file.Name, "theme.json") || strings.HasSuffix(file.Name, "index.html") {
					rootPrefix = potentialPrefix
					log.Printf("检测到主题文件在子目录中: %s", rootPrefix)
				}
			}
		}
	}

	// 第二遍扫描：根据检测到的结构验证文件
	for _, file := range zipReader.File {
		if strings.Contains(file.Name, "..") {
			continue // 已处理过的危险路径
		}

		// 移除根目录前缀进行匹配
		normalizedName := file.Name
		if rootPrefix != "" && strings.HasPrefix(file.Name, rootPrefix) {
			normalizedName = strings.TrimPrefix(file.Name, rootPrefix)
		}

		// 检查必需文件
		switch {
		case normalizedName == "theme.json":
			themeJsonFile = file
		case normalizedName == "index.html":
			indexHtmlFile = file
		case strings.HasPrefix(normalizedName, "static/"):
			hasStaticDir = true
		}

		// 验证文件类型安全性
		if err := s.validateFileType(file.Name); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	// 5. 检查必需文件
	if themeJsonFile == nil {
		result.Errors = append(result.Errors, "缺少必需的 theme.json 文件")
	}

	if indexHtmlFile == nil {
		result.Errors = append(result.Errors, "缺少必需的 index.html 文件")
	}

	if !hasStaticDir {
		result.Warnings = append(result.Warnings, "建议包含 static/ 目录用于存放静态资源")
	}

	// 6. 验证theme.json内容
	if themeJsonFile != nil {
		metadata, err := s.parseThemeJson(themeJsonFile)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("theme.json解析失败: %v", err))
		} else {
			result.Metadata = metadata
			log.Printf("[ValidateTheme] 解析到主题元信息: 名称=%s, 版本=%s", metadata.Name, metadata.Version)
			// 验证元信息
			if validationErrors := s.validateThemeMetadata(metadata); len(validationErrors) > 0 {
				result.Errors = append(result.Errors, validationErrors...)
			}
		}
	}

	// 7. 检查是否存在重复主题
	if result.Metadata != nil {
		log.Printf("[ValidateTheme] 检查主题 %s 是否已被用户 %d 安装", result.Metadata.Name, userID)
		existingTheme, err := s.db.UserInstalledTheme.
			Query().
			Where(
				userinstalledtheme.UserID(userID),
				userinstalledtheme.ThemeName(result.Metadata.Name),
			).
			Only(ctx)

		if err == nil {
			// 找到重复主题，构建主题信息
			log.Printf("[ValidateTheme] 找到重复主题: %s, 已安装版本: %s, 新版本: %s",
				result.Metadata.Name, existingTheme.InstalledVersion, result.Metadata.Version)

			authorName := s.extractAuthorName(result.Metadata.Author)

			result.ExistingTheme = &ThemeInfo{
				ID:               0, // 本地安装的主题没有市场ID
				Name:             existingTheme.ThemeName,
				Author:           authorName,
				Description:      result.Metadata.Description,
				Version:          existingTheme.InstalledVersion,
				ThemeType:        "community", // 本地上传的主题默认为社区版
				InstalledVersion: existingTheme.InstalledVersion,
				InstallTime:      &existingTheme.InstallTime,
				IsInstalled:      true,
				CreatedAt:        existingTheme.InstallTime.Format("2006-01-02 15:04:05"),
				UpdatedAt:        existingTheme.InstallTime.Format("2006-01-02 15:04:05"),
			}
		} else if ent.IsNotFound(err) {
			// 未找到重复主题，这是正常情况
			log.Printf("[ValidateTheme] 未找到重复主题，可以正常安装")
		} else {
			// 数据库查询出错
			log.Printf("[ValidateTheme] 检查重复主题时发生数据库错误: %v", err)
		}
	}

	// 8. 设置验证结果
	result.IsValid = len(result.Errors) == 0

	return result, nil
}

// saveUploadedFile 保存上传的文件到临时位置
func (s *themeService) saveUploadedFile(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// 创建临时文件
	tempFile, err := os.CreateTemp("", "theme_upload_*.zip")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// 复制文件内容
	_, err = io.Copy(tempFile, src)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// parseThemeJson 解析theme.json文件
func (s *themeService) parseThemeJson(file *zip.File) (*ThemeMetadata, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var metadata ThemeMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("JSON格式错误: %w", err)
	}

	return &metadata, nil
}

// validateThemeMetadata 验证主题元信息
func (s *themeService) validateThemeMetadata(metadata *ThemeMetadata) []string {
	var errors []string

	// 验证必需字段
	if metadata.Name == "" {
		errors = append(errors, "name字段不能为空")
	} else {
		// 验证主题名称格式
		if !strings.HasPrefix(metadata.Name, "theme-") {
			errors = append(errors, "主题名称必须以'theme-'开头")
		}

		// 验证主题名称字符
		validName := regexp.MustCompile(`^theme-[a-z0-9\-]+$`)
		if !validName.MatchString(metadata.Name) {
			errors = append(errors, "主题名称只能包含小写字母、数字和连字符")
		}
	}

	if metadata.DisplayName == "" {
		errors = append(errors, "displayName字段不能为空")
	}

	if metadata.Version == "" {
		errors = append(errors, "version字段不能为空")
	} else {
		// 验证版本格式（简单的语义化版本检查）
		validVersion := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9\-\.]+)?$`)
		if !validVersion.MatchString(metadata.Version) {
			errors = append(errors, "version必须符合语义化版本规范（如：1.0.0）")
		}
	}

	if metadata.Description == "" {
		errors = append(errors, "description字段不能为空")
	}

	if metadata.Author == nil {
		errors = append(errors, "author字段不能为空")
	}

	// 验证分类
	if metadata.Category != "" {
		validCategories := []string{
			"blog", "portfolio", "business", "magazine", "minimal",
			"creative", "photography", "education", "technology", "other",
		}
		isValidCategory := false
		for _, cat := range validCategories {
			if metadata.Category == cat {
				isValidCategory = true
				break
			}
		}
		if !isValidCategory {
			errors = append(errors, fmt.Sprintf("不支持的主题分类: %s", metadata.Category))
		}
	}

	return errors
}

// validateFileType 验证文件类型安全性
func (s *themeService) validateFileType(filename string) error {
	// 跳过 macOS 系统文件
	if strings.Contains(filename, "__MACOSX/") || strings.HasPrefix(filepath.Base(filename), "._") {
		log.Printf("跳过系统文件: %s", filename)
		return nil
	}

	// 允许的文件扩展名
	allowedExtensions := map[string]bool{
		".html": true, ".htm": true, ".css": true, ".scss": true, ".sass": true, ".less": true,
		".js": true, ".ts": true, ".json": true, ".xml": true, ".yml": true, ".yaml": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".svg": true, ".webp": true,
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
		".md": true, ".txt": true, ".ico": true,
		// 允许压缩文件（通常是构建工具生成的）
		".gz": true, ".br": true,
	}

	// 禁止的文件扩展名（移除了 .gz）
	forbiddenExtensions := map[string]bool{
		".exe": true, ".bat": true, ".sh": true, ".cmd": true, ".com": true,
		".php": true, ".asp": true, ".jsp": true, ".py": true, ".rb": true,
		".dll": true, ".so": true, ".dylib": true,
		".zip": true, ".rar": true, ".tar": true, ".7z": true,
	}

	ext := strings.ToLower(filepath.Ext(filename))

	if forbiddenExtensions[ext] {
		return fmt.Errorf("禁止的文件类型: %s", filename)
	}

	// 如果不在允许列表中，给出警告（但不阻止）
	if ext != "" && !allowedExtensions[ext] {
		log.Printf("警告：未知文件类型 %s，文件名：%s", ext, filename)
	}

	return nil
}

// validateExtractedTheme 验证解压后的主题文件
func (s *themeService) validateExtractedTheme(themeDir string, metadata *ThemeMetadata) error {
	// 检查theme.json文件
	themeJsonPath := filepath.Join(themeDir, "theme.json")
	if _, err := os.Stat(themeJsonPath); os.IsNotExist(err) {
		return fmt.Errorf("解压后缺少 theme.json 文件")
	}

	// 检查index.html文件
	indexPath := filepath.Join(themeDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("解压后缺少 index.html 文件")
	}

	// 验证HTML文件基本格式
	if err := s.validateHtmlFile(indexPath); err != nil {
		return fmt.Errorf("index.html文件验证失败: %w", err)
	}

	return nil
}

// validateHtmlFile 验证HTML文件基本格式
func (s *themeService) validateHtmlFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	contentStr := strings.ToLower(string(content))

	// 基本的HTML结构检查
	if !strings.Contains(contentStr, "<!doctype html>") && !strings.Contains(contentStr, "<html") {
		return fmt.Errorf("不是有效的HTML文件")
	}

	if !strings.Contains(contentStr, "<head>") || !strings.Contains(contentStr, "</head>") {
		return fmt.Errorf("HTML文件缺少head标签")
	}

	if !strings.Contains(contentStr, "<body>") || !strings.Contains(contentStr, "</body>") {
		return fmt.Errorf("HTML文件缺少body标签")
	}

	return nil
}

// extractAuthorName 从作者信息中提取作者名称
func (s *themeService) extractAuthorName(author interface{}) string {
	switch v := author.(type) {
	case string:
		// 如果是字符串格式，可能是 "Name <email>" 格式
		if strings.Contains(v, "<") {
			parts := strings.Split(v, "<")
			return strings.TrimSpace(parts[0])
		}
		return v
	case map[string]interface{}:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return "Unknown"
}

// extractFirstScreenshot 从 screenshots 字段提取第一个预览图URL
func (s *themeService) extractFirstScreenshot(screenshots interface{}) string {
	if screenshots == nil {
		return ""
	}

	switch v := screenshots.(type) {
	case string:
		// 单个字符串
		return v
	case []string:
		// 字符串数组，返回第一个
		if len(v) > 0 {
			return v[0]
		}
	case []interface{}:
		// interface{}数组，尝试转换第一个为字符串
		if len(v) > 0 {
			if str, ok := v[0].(string); ok {
				return str
			}
		}
	}
	return ""
}

// loadThemeMetadataFromDisk 从磁盘读取主题的 theme.json 文件
func (s *themeService) loadThemeMetadataFromDisk(themeName string) (*ThemeMetadata, error) {
	themeDir := filepath.Join(ThemesDirName, themeName)
	themeJsonPath := filepath.Join(themeDir, "theme.json")

	// 检查文件是否存在
	if _, err := os.Stat(themeJsonPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("theme.json 文件不存在: %s", themeJsonPath)
	}

	// 读取文件内容
	content, err := os.ReadFile(themeJsonPath)
	if err != nil {
		return nil, fmt.Errorf("读取 theme.json 失败: %w", err)
	}

	// 解析 JSON
	var metadata ThemeMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("解析 theme.json 失败: %w", err)
	}

	return &metadata, nil
}

// safeRemoveStaticDir 安全地删除static目录，处理Docker挂载等特殊情况
func (s *themeService) safeRemoveStaticDir() error {
	maxRetries := 3
	retryDelay := time.Second * 2

	for attempt := 0; attempt < maxRetries; attempt++ {
		// 首先尝试删除目录内容，而不是整个目录
		if err := s.clearStaticDirContents(); err == nil {
			log.Printf("成功清空static目录内容")
			return nil
		} else {
			log.Printf("第 %d 次尝试清空static目录失败: %v", attempt+1, err)
		}

		// 如果不是最后一次尝试，等待后重试
		if attempt < maxRetries-1 {
			log.Printf("等待 %v 后重试...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	// 所有重试都失败了，尝试最后的手段：强制删除
	log.Printf("警告：常规删除失败，尝试强制清理static目录")
	return s.forceCleanStaticDir()
}

// clearStaticDirContents 清空static目录的内容，但保留目录本身
func (s *themeService) clearStaticDirContents() error {
	if _, err := os.Stat(StaticDirName); os.IsNotExist(err) {
		return nil // 目录不存在，认为成功
	}

	entries, err := os.ReadDir(StaticDirName)
	if err != nil {
		return err
	}

	var lastError error
	for _, entry := range entries {
		entryPath := filepath.Join(StaticDirName, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			log.Printf("删除 %s 失败: %v", entryPath, err)
			lastError = err
		}
	}

	return lastError
}

// forceCleanStaticDir 强制清理static目录（最后手段）
func (s *themeService) forceCleanStaticDir() error {
	// 在Docker环境中，我们可能无法删除挂载的目录本身
	// 但我们可以确保目录是空的
	if err := s.clearStaticDirContents(); err != nil {
		log.Printf("强制清理static目录内容也失败: %v", err)
		return err
	}

	// 尝试删除目录本身，如果失败就忽略（Docker挂载的目录无法删除是正常的）
	if err := os.Remove(StaticDirName); err != nil {
		log.Printf("无法删除static目录本身（这在Docker环境中是正常的）: %v", err)
		// 不返回错误，因为目录内容已经清空了
	}

	return nil
}

// FixThemeCurrentStatus 修复用户主题的当前状态数据一致性
func (s *themeService) FixThemeCurrentStatus(ctx context.Context, userID uint) error {
	staticModeActive := s.IsStaticModeActive()

	if !staticModeActive {
		// 没有static目录时，所有数据库主题都不应该是当前使用
		updatedCount, err := s.db.UserInstalledTheme.
			Update().
			Where(
				userinstalledtheme.UserID(userID),
				userinstalledtheme.IsCurrent(true),
			).
			SetIsCurrent(false).
			Save(ctx)

		if err != nil {
			return fmt.Errorf("修复主题状态失败: %w", err)
		}

		if updatedCount > 0 {
			log.Printf("已修复用户 %d 的 %d 个主题状态（从当前使用改为非当前使用）", userID, updatedCount)
		}
	} else {
		// 有static目录时，确保只有一个主题是当前使用
		currentThemes, err := s.db.UserInstalledTheme.
			Query().
			Where(
				userinstalledtheme.UserID(userID),
				userinstalledtheme.IsCurrent(true),
			).
			All(ctx)

		if err != nil {
			return fmt.Errorf("查询当前主题失败: %w", err)
		}

		if len(currentThemes) > 1 {
			// 多个主题标记为当前使用，只保留第一个
			log.Printf("发现用户 %d 有 %d 个主题标记为当前使用，将修复为只保留一个", userID, len(currentThemes))

			for i := 1; i < len(currentThemes); i++ {
				_, err := s.db.UserInstalledTheme.
					UpdateOneID(currentThemes[i].ID).
					SetIsCurrent(false).
					Save(ctx)

				if err != nil {
					log.Printf("修复主题 %s 状态失败: %v", currentThemes[i].ThemeName, err)
				} else {
					log.Printf("已将主题 %s 的当前状态设置为 false", currentThemes[i].ThemeName)
				}
			}
		}
	}

	return nil
}
