// anheyu-app/pkg/service/utility/primary_color_service.go
package utility

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/anzhiyu-c/anheyu-app/internal/infra/storage"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

const (
	defaultPrimaryColor = "#b4bfe2"
)

// PrimaryColorService 主色调服务，根据不同的存储策略采用不同的方法获取图片主色调
type PrimaryColorService struct {
	colorSvc          *ColorService
	settingSvc        setting.SettingService
	fileRepo          repository.FileRepository
	storagePolicyRepo repository.StoragePolicyRepository
	httpClient        *http.Client
	storageProviders  map[constant.StoragePolicyType]storage.IStorageProvider
}

// NewPrimaryColorService 创建主色调服务实例
func NewPrimaryColorService(
	colorSvc *ColorService,
	settingSvc setting.SettingService,
	fileRepo repository.FileRepository,
	storagePolicyRepo repository.StoragePolicyRepository,
	httpClient *http.Client,
	storageProviders map[constant.StoragePolicyType]storage.IStorageProvider,
) *PrimaryColorService {
	return &PrimaryColorService{
		colorSvc:          colorSvc,
		settingSvc:        settingSvc,
		fileRepo:          fileRepo,
		storagePolicyRepo: storagePolicyRepo,
		httpClient:        httpClient,
		storageProviders:  storageProviders,
	}
}

// GetPrimaryColorFromURL 根据图片URL智能获取主色调
// 支持本地存储、阿里云OSS、腾讯云COS、OneDrive等多种存储方式
// 返回空字符串表示获取失败，前端应使用默认值
func (s *PrimaryColorService) GetPrimaryColorFromURL(ctx context.Context, imageURL string) string {
	if imageURL == "" {
		log.Printf("[主色调服务] 图片URL为空，返回空字符串")
		return ""
	}

	// 清理URL中的特殊字符（包括零宽字符等）
	imageURL = strings.TrimSpace(imageURL)
	// 移除常见的零宽字符和不可见字符
	imageURL = strings.ReplaceAll(imageURL, "\u200B", "") // 零宽空格
	imageURL = strings.ReplaceAll(imageURL, "\u200C", "") // 零宽非连接符
	imageURL = strings.ReplaceAll(imageURL, "\u200D", "") // 零宽连接符
	imageURL = strings.ReplaceAll(imageURL, "\uFEFF", "") // 零宽非断空格 (BOM)
	imageURL = strings.ReplaceAll(imageURL, "\u2060", "") // 字连接符

	log.Printf("[主色调服务] 开始处理图片URL: %s", imageURL)

	// 判断是否是系统内的图片
	if isSystemImage, filePublicID := s.isSystemImage(imageURL); isSystemImage {
		log.Printf("[主色调服务] 检测到系统内图片，FileID: %s", filePublicID)
		return s.getColorFromSystemFile(ctx, filePublicID)
	}

	// 判断是否是米游社的图片
	if s.isMiyousheImage(imageURL) {
		log.Printf("[主色调服务] 检测到米游社图片，使用OSS图片处理API")
		return s.getColorFromMiyoushe(ctx, imageURL)
	}

	// 外部图片，直接下载处理
	log.Printf("[主色调服务] 检测到外部图片，使用HTTP下载方式获取")
	return s.getColorFromExternalURL(ctx, imageURL)
}

// isSystemImage 判断URL是否是系统内的图片
// 返回：(是否系统图片, 文件公共ID)
func (s *PrimaryColorService) isSystemImage(imageURL string) (bool, string) {
	siteURL := s.settingSvc.Get(constant.KeySiteURL.String())
	if siteURL == "" || siteURL == "https://" || siteURL == "http://" {
		siteURL = "https://anheyu.com"
	}
	trimmedSiteURL := strings.TrimSuffix(siteURL, "/")

	// 检查是否是代理URL格式: /api/v1/pro/images/{fileID}
	if strings.HasPrefix(imageURL, trimmedSiteURL+"/api/v1/pro/images/") {
		filePublicID := strings.TrimPrefix(imageURL, trimmedSiteURL+"/api/v1/pro/images/")
		// 移除可能的查询参数
		if idx := strings.Index(filePublicID, "?"); idx != -1 {
			filePublicID = filePublicID[:idx]
		}
		return true, filePublicID
	}

	// 检查是否是直链URL格式: /api/f/{linkID}/{filename}
	if strings.HasPrefix(imageURL, trimmedSiteURL+"/api/f/") {
		pathParts := strings.Split(strings.TrimPrefix(imageURL, trimmedSiteURL+"/api/f/"), "/")
		if len(pathParts) >= 1 {
			linkPublicID := pathParts[0]
			// 通过linkID获取文件ID（这里需要查询direct_links表）
			// 暂时返回false，使用HTTP下载方式
			log.Printf("[主色调服务] 检测到直链格式，LinkID: %s，暂使用HTTP方式", linkPublicID)
			return false, ""
		}
	}

	return false, ""
}

// isMiyousheImage 判断URL是否是米游社的图片
func (s *PrimaryColorService) isMiyousheImage(imageURL string) bool {
	return strings.Contains(imageURL, "upload-bbs.miyoushe.com")
}

// getColorFromSystemFile 从系统内的文件获取主色调
func (s *PrimaryColorService) getColorFromSystemFile(ctx context.Context, filePublicID string) string {
	// 解码文件公共ID
	fileID, entityType, err := idgen.DecodePublicID(filePublicID)
	if err != nil {
		log.Printf("[主色调服务] 解码文件ID失败: %v，返回空字符串", err)
		return ""
	}

	if entityType != idgen.EntityTypeFile {
		log.Printf("[主色调服务] ID类型不是文件类型: %v，返回空字符串", entityType)
		return ""
	}

	// 查找文件信息
	file, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		log.Printf("[主色调服务] 查找文件失败: %v，返回空字符串", err)
		return ""
	}

	if file == nil || file.PrimaryEntity == nil {
		log.Printf("[主色调服务] 文件或实体信息不完整，返回空字符串")
		return ""
	}

	// 获取存储策略
	policy, err := s.storagePolicyRepo.FindByID(ctx, file.PrimaryEntity.PolicyID)
	if err != nil {
		log.Printf("[主色调服务] 查找存储策略失败: %v，返回空字符串", err)
		return ""
	}

	if policy == nil {
		log.Printf("[主色调服务] 存储策略不存在，返回空字符串")
		return ""
	}

	log.Printf("[主色调服务] 文件所属存储策略类型: %s", policy.Type)

	// 根据存储策略类型选择不同的处理方式
	switch policy.Type {
	case constant.PolicyTypeLocal:
		return s.getColorFromLocalFile(ctx, file, policy)
	case constant.PolicyTypeOneDrive:
		return s.getColorFromOneDriveFile(ctx, file, policy)
	case constant.PolicyTypeTencentCOS:
		return s.getColorFromTencentCOS(ctx, file, policy)
	case constant.PolicyTypeAliOSS:
		return s.getColorFromAliOSS(ctx, file, policy)
	default:
		log.Printf("[主色调服务] 不支持的存储策略类型: %s，返回空字符串", policy.Type)
		return ""
	}
}

// getColorFromLocalFile 从本地存储的文件获取主色调
func (s *PrimaryColorService) getColorFromLocalFile(ctx context.Context, file *model.File, policy *model.StoragePolicy) string {
	log.Printf("[主色调服务] 使用本地文件读取方式")

	// 检查Source字段
	if !file.PrimaryEntity.Source.Valid {
		log.Printf("[主色调服务] 文件Source字段无效，返回空字符串")
		return ""
	}

	// 构建完整的文件路径
	fullPath := filepath.Join(policy.BasePath, file.PrimaryEntity.Source.String)

	// 打开文件
	f, err := os.Open(fullPath)
	if err != nil {
		log.Printf("[主色调服务] 打开本地文件失败: %v，返回空字符串", err)
		return ""
	}
	defer f.Close()

	// 使用ColorService提取主色调
	color, err := s.colorSvc.GetPrimaryColor(f)
	if err != nil {
		log.Printf("[主色调服务] 从本地文件提取主色调失败: %v，返回空字符串", err)
		return ""
	}

	log.Printf("[主色调服务] 成功从本地文件提取主色调: %s", color)
	return color
}

// getColorFromOneDriveFile 从OneDrive存储的文件获取主色调
func (s *PrimaryColorService) getColorFromOneDriveFile(ctx context.Context, file *model.File, policy *model.StoragePolicy) string {
	log.Printf("[主色调服务] 使用OneDrive下载方式")

	// 检查Source字段
	if !file.PrimaryEntity.Source.Valid {
		log.Printf("[主色调服务] 文件Source字段无效，返回空字符串")
		return ""
	}

	// 获取OneDrive存储提供者
	provider, exists := s.storageProviders[constant.PolicyTypeOneDrive]
	if !exists {
		log.Printf("[主色调服务] OneDrive存储提供者不存在，返回空字符串")
		return ""
	}

	// 获取下载URL
	downloadURL, err := provider.GetDownloadURL(ctx, policy, file.PrimaryEntity.Source.String, storage.DownloadURLOptions{})
	if err != nil {
		log.Printf("[主色调服务] 获取OneDrive下载URL失败: %v，返回空字符串", err)
		return ""
	}

	// 下载并处理图片
	return s.downloadAndExtractColor(ctx, downloadURL)
}

// getColorFromTencentCOS 从腾讯云COS获取主色调
func (s *PrimaryColorService) getColorFromTencentCOS(ctx context.Context, file *model.File, policy *model.StoragePolicy) string {
	log.Printf("[主色调服务] 使用腾讯云COS数据万象API")

	var imageAveURL string

	// 根据存储策略是否为私有来决定URL构建方式
	if policy.IsPrivate {
		log.Printf("[主色调服务] 私有存储桶，使用带签名的图片处理URL")
		// 私有存储桶：使用storage provider获取带签名的URL，然后添加图片处理参数
		downloadURL := s.getTencentCOSDownloadURL(ctx, file, policy)
		if downloadURL == "" {
			log.Printf("[主色调服务] 获取腾讯云COS签名URL失败，返回空字符串")
			return ""
		}
		// 在签名URL后添加图片处理参数
		if strings.Contains(downloadURL, "?") {
			imageAveURL = downloadURL + "&imageAve"
		} else {
			imageAveURL = downloadURL + "?imageAve"
		}
	} else {
		log.Printf("[主色调服务] 公有存储桶，使用直接图片处理URL")
		// 公有存储桶：直接构建图片处理URL
		baseURL := s.buildCOSURL(file, policy)
		if baseURL == "" {
			log.Printf("[主色调服务] 构建腾讯云COS URL失败，返回空字符串")
			return ""
		}
		imageAveURL = baseURL + "?imageAve"
	}

	log.Printf("[主色调服务] 腾讯云COS imageAve URL: %s", imageAveURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageAveURL, nil)
	if err != nil {
		log.Printf("[主色调服务] 创建腾讯云请求失败: %v，返回空字符串", err)
		return ""
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[主色调服务] 请求腾讯云接口失败: %v，返回空字符串", err)
		return ""
	}
	defer resp.Body.Close()

	// 如果返回403/401等权限错误，或404，降级到下载图片方式
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
		log.Printf("[主色调服务] 腾讯云接口返回%d状态码（可能是权限问题或服务未开通），降级到下载图片方式", resp.StatusCode)
		// 尝试使用storage provider获取签名URL（适用于私有存储桶）
		downloadURL := s.getTencentCOSDownloadURL(ctx, file, policy)
		if downloadURL == "" {
			// 降级到直接URL
			downloadURL = s.buildCOSURL(file, policy)
		}
		return s.downloadAndExtractColor(ctx, downloadURL)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[主色调服务] 腾讯云接口返回非200状态码: %d，尝试降级处理", resp.StatusCode)
		downloadURL := s.getTencentCOSDownloadURL(ctx, file, policy)
		if downloadURL == "" {
			downloadURL = s.buildCOSURL(file, policy)
		}
		return s.downloadAndExtractColor(ctx, downloadURL)
	}

	// 解析返回的JSON
	var result struct {
		RGB string `json:"RGB"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[主色调服务] 解析腾讯云返回数据失败: %v，返回空字符串", err)
		return ""
	}

	// RGB格式: "0xRRGGBB"
	if strings.HasPrefix(result.RGB, "0x") {
		hexColor := "#" + result.RGB[2:]
		log.Printf("[主色调服务] 从腾讯云COS获取主色调成功: %s", hexColor)
		return hexColor
	}

	log.Printf("[主色调服务] 腾讯云返回格式异常: %s，返回空字符串", result.RGB)
	return ""
}

// getColorFromAliOSS 从阿里云OSS获取主色调
func (s *PrimaryColorService) getColorFromAliOSS(ctx context.Context, file *model.File, policy *model.StoragePolicy) string {
	log.Printf("[主色调服务] 使用阿里云OSS图片处理API")

	var averageHueURL string

	// 根据存储策略是否为私有来决定URL构建方式
	if policy.IsPrivate {
		log.Printf("[主色调服务] 私有存储桶，使用带签名的图片处理URL")
		// 私有存储桶：使用storage provider获取带签名的URL，然后添加图片处理参数
		downloadURL := s.getAliyunOSSDownloadURL(ctx, file, policy)
		if downloadURL == "" {
			log.Printf("[主色调服务] 获取阿里云OSS签名URL失败，返回空字符串")
			return ""
		}
		// 在签名URL后添加图片处理参数
		if strings.Contains(downloadURL, "?") {
			averageHueURL = downloadURL + "&x-oss-process=image/average-hue"
		} else {
			averageHueURL = downloadURL + "?x-oss-process=image/average-hue"
		}
	} else {
		log.Printf("[主色调服务] 公有存储桶，使用直接图片处理URL")
		// 公有存储桶：直接构建图片处理URL
		baseURL := s.buildOSSURL(file, policy)
		if baseURL == "" {
			log.Printf("[主色调服务] 构建阿里云OSS URL失败，返回空字符串")
			return ""
		}
		averageHueURL = baseURL + "?x-oss-process=image/average-hue"
	}

	log.Printf("[主色调服务] 阿里云OSS average-hue URL: %s", averageHueURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, averageHueURL, nil)
	if err != nil {
		log.Printf("[主色调服务] 创建阿里云请求失败: %v，返回空字符串", err)
		return ""
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[主色调服务] 请求阿里云接口失败: %v，返回空字符串", err)
		return ""
	}
	defer resp.Body.Close()

	// 如果返回403/401等权限错误，或404，降级到下载图片方式
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
		log.Printf("[主色调服务] 阿里云接口返回%d状态码（可能是权限问题或服务未开通），降级到下载图片方式", resp.StatusCode)
		// 尝试使用storage provider获取签名URL（适用于私有存储桶）
		downloadURL := s.getAliyunOSSDownloadURL(ctx, file, policy)
		if downloadURL == "" {
			// 降级到直接URL
			downloadURL = s.buildOSSURL(file, policy)
		}
		return s.downloadAndExtractColor(ctx, downloadURL)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[主色调服务] 阿里云接口返回非200状态码: %d，尝试降级处理", resp.StatusCode)
		downloadURL := s.getAliyunOSSDownloadURL(ctx, file, policy)
		if downloadURL == "" {
			downloadURL = s.buildOSSURL(file, policy)
		}
		return s.downloadAndExtractColor(ctx, downloadURL)
	}

	// 解析返回的JSON
	var result struct {
		RGB string `json:"RGB"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[主色调服务] 解析阿里云返回数据失败: %v，返回空字符串", err)
		return ""
	}

	// RGB格式: "0xRRGGBB"
	if strings.HasPrefix(result.RGB, "0x") {
		hexColor := "#" + result.RGB[2:]
		log.Printf("[主色调服务] 从阿里云OSS获取主色调成功: %s", hexColor)
		return hexColor
	}

	log.Printf("[主色调服务] 阿里云返回格式异常: %s，返回空字符串", result.RGB)
	return ""
}

// buildCOSURL 构建腾讯云COS的完整URL
func (s *PrimaryColorService) buildCOSURL(file *model.File, policy *model.StoragePolicy) string {
	// 腾讯云COS的URL格式: https://{bucket}-{appid}.cos.{region}.myqcloud.com/{source}
	// Server字段通常包含完整的域名或endpoint
	if policy.Server == "" || !file.PrimaryEntity.Source.Valid {
		return ""
	}

	server := strings.TrimSuffix(policy.Server, "/")
	source := strings.TrimPrefix(file.PrimaryEntity.Source.String, "/")

	return fmt.Sprintf("%s/%s", server, source)
}

// buildOSSURL 构建阿里云OSS的完整URL
func (s *PrimaryColorService) buildOSSURL(file *model.File, policy *model.StoragePolicy) string {
	// 阿里云OSS的URL格式: https://{bucket}.{endpoint}/{source}
	if policy.Server == "" || !file.PrimaryEntity.Source.Valid {
		return ""
	}

	server := strings.TrimSuffix(policy.Server, "/")
	source := strings.TrimPrefix(file.PrimaryEntity.Source.String, "/")

	return fmt.Sprintf("%s/%s", server, source)
}

// getColorFromMiyoushe 从米游社图片获取主色调
func (s *PrimaryColorService) getColorFromMiyoushe(ctx context.Context, imageURL string) string {
	// 米游社使用阿里云OSS，支持通过添加 x-oss-process=image/average-hue 参数获取主色调
	var averageHueURL string
	if strings.Contains(imageURL, "?") {
		averageHueURL = imageURL + "&x-oss-process=image/average-hue"
	} else {
		averageHueURL = imageURL + "?x-oss-process=image/average-hue"
	}

	log.Printf("[主色调服务] 米游社 average-hue URL: %s", averageHueURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, averageHueURL, nil)
	if err != nil {
		log.Printf("[主色调服务] 创建米游社请求失败: %v，返回空字符串", err)
		return ""
	}

	// 添加常用的 HTTP headers，米游社可能需要 Referer
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json,*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.miyoushe.com/")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[主色调服务] 请求米游社接口失败: %v，返回空字符串", err)
		return ""
	}
	defer resp.Body.Close()

	// 如果返回403/401等权限错误，或404，降级到下载图片方式
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
		log.Printf("[主色调服务] 米游社接口返回%d状态码（可能是权限问题或服务未开通），降级到下载图片方式", resp.StatusCode)
		return s.downloadAndExtractColor(ctx, imageURL)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[主色调服务] 米游社接口返回非200状态码: %d，尝试降级处理", resp.StatusCode)
		return s.downloadAndExtractColor(ctx, imageURL)
	}

	// 解析返回的JSON
	var result struct {
		RGB string `json:"RGB"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[主色调服务] 解析米游社返回数据失败: %v，返回空字符串", err)
		return ""
	}

	// RGB格式: "0xRRGGBB"
	if strings.HasPrefix(result.RGB, "0x") {
		hexColor := "#" + result.RGB[2:]
		log.Printf("[主色调服务] 从米游社获取主色调成功: %s", hexColor)
		return hexColor
	}

	log.Printf("[主色调服务] 米游社返回格式异常: %s，返回空字符串", result.RGB)
	return ""
}

// getColorFromExternalURL 从外部URL下载图片并提取主色调
func (s *PrimaryColorService) getColorFromExternalURL(ctx context.Context, imageURL string) string {
	return s.downloadAndExtractColor(ctx, imageURL)
}

// downloadAndExtractColor 下载图片并提取主色调的通用方法
func (s *PrimaryColorService) downloadAndExtractColor(ctx context.Context, imageURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		log.Printf("[主色调服务] 创建HTTP请求失败: %v，返回空字符串", err)
		return ""
	}

	// 添加常用的 HTTP headers，避免服务器拒绝请求
	// 注意：不设置 Accept-Encoding，让 Go 的 http.Client 自动处理 gzip 解压
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[主色调服务] 下载图片失败: %v，返回空字符串", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[主色调服务] 图片URL返回非200状态码: %d，返回空字符串", resp.StatusCode)
		return ""
	}

	// 检查 Content-Type
	contentType := resp.Header.Get("Content-Type")
	log.Printf("[主色调服务] 响应Content-Type: %s", contentType)

	// 如果返回的不是图片类型，检查是否是反爬虫机制
	if !strings.HasPrefix(contentType, "image/") {
		// 检查服务器类型
		server := resp.Header.Get("Server")
		if strings.Contains(server, "EdgeOne") || strings.Contains(server, "Cloudflare") {
			log.Printf("[主色调服务] 检测到CDN反爬虫保护 (Server: %s)，该图床需要JavaScript挑战验证，无法直接获取图片，返回空字符串", server)
		} else {
			log.Printf("[主色调服务] 响应类型不是图片: %s，可能是服务器返回了错误页面或反爬虫保护，返回空字符串", contentType)
		}
		return ""
	}

	color, err := s.colorSvc.GetPrimaryColor(resp.Body)
	if err != nil {
		log.Printf("[主色调服务] 提取主色调失败: %v，返回空字符串", err)
		return ""
	}

	log.Printf("[主色调服务] 成功提取主色调: %s", color)
	return color
}

// getTencentCOSDownloadURL 获取腾讯云COS的下载URL（可能包含签名）
func (s *PrimaryColorService) getTencentCOSDownloadURL(ctx context.Context, file *model.File, policy *model.StoragePolicy) string {
	if !file.PrimaryEntity.Source.Valid {
		log.Printf("[主色调服务] 文件Source字段无效")
		return ""
	}

	provider, exists := s.storageProviders[constant.PolicyTypeTencentCOS]
	if !exists {
		log.Printf("[主色调服务] 腾讯云COS存储提供者不存在")
		return ""
	}

	downloadURL, err := provider.GetDownloadURL(ctx, policy, file.PrimaryEntity.Source.String, storage.DownloadURLOptions{})
	if err != nil {
		log.Printf("[主色调服务] 获取腾讯云COS下载URL失败: %v", err)
		return ""
	}

	return downloadURL
}

// getAliyunOSSDownloadURL 获取阿里云OSS的下载URL（可能包含签名）
func (s *PrimaryColorService) getAliyunOSSDownloadURL(ctx context.Context, file *model.File, policy *model.StoragePolicy) string {
	if !file.PrimaryEntity.Source.Valid {
		log.Printf("[主色调服务] 文件Source字段无效")
		return ""
	}

	provider, exists := s.storageProviders[constant.PolicyTypeAliOSS]
	if !exists {
		log.Printf("[主色调服务] 阿里云OSS存储提供者不存在")
		return ""
	}

	downloadURL, err := provider.GetDownloadURL(ctx, policy, file.PrimaryEntity.Source.String, storage.DownloadURLOptions{})
	if err != nil {
		log.Printf("[主色调服务] 获取阿里云OSS下载URL失败: %v", err)
		return ""
	}

	return downloadURL
}
