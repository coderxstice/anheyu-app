package album

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
	_ "golang.org/x/image/webp"
)

// CreateAlbumParams 定义了创建相册时需要的参数
type CreateAlbumParams struct {
	ImageUrl     string
	BigImageUrl  string
	DownloadUrl  string
	ThumbParam   string
	BigParam     string
	Tags         []string
	Width        int
	Height       int
	FileSize     int64
	Format       string
	FileHash     string
	DisplayOrder int
}

// UpdateAlbumParams 定义了更新相册时需要的参数
type UpdateAlbumParams struct {
	ImageUrl     string
	BigImageUrl  string
	DownloadUrl  string
	ThumbParam   string
	BigParam     string
	Tags         []string
	DisplayOrder *int
}

// FindAlbumsParams 定义了查询相册时需要的参数
type FindAlbumsParams struct {
	Page     int
	PageSize int
	Tag      string
	Start    *time.Time
	End      *time.Time
	Sort     string
}

// BatchImportResult 批量导入的结果
type BatchImportResult struct {
	SuccessCount int
	FailCount    int
	SkipCount    int
	Errors       []BatchImportError
	Duplicates   []string
}

// BatchImportError 批量导入的错误信息
type BatchImportError struct {
	URL    string
	Reason string
}

// BatchImportParams 批量导入的参数
type BatchImportParams struct {
	URLs         []string
	ThumbParam   string
	BigParam     string
	Tags         []string
	DisplayOrder int
}

// AlbumService 定义了相册相关的业务逻辑接口
type AlbumService interface {
	CreateAlbum(ctx context.Context, params CreateAlbumParams) (*model.Album, error)
	BatchImportAlbums(ctx context.Context, params BatchImportParams) (*BatchImportResult, error)
	DeleteAlbum(ctx context.Context, id uint) error
	UpdateAlbum(ctx context.Context, id uint, params UpdateAlbumParams) (*model.Album, error)
	FindAlbums(ctx context.Context, params FindAlbumsParams) (*repository.PageResult[model.Album], error)
	IncrementAlbumStat(ctx context.Context, id uint, statType string) error
}

// albumService 是 AlbumService 接口的实现
type albumService struct {
	albumRepo  repository.AlbumRepository
	tagRepo    repository.TagRepository
	settingSvc setting.SettingService
}

// NewAlbumService 是 albumService 的构造函数
func NewAlbumService(albumRepo repository.AlbumRepository, tagRepo repository.TagRepository, settingSvc setting.SettingService) AlbumService {
	return &albumService{
		albumRepo:  albumRepo,
		tagRepo:    tagRepo,
		settingSvc: settingSvc,
	}
}

// CreateAlbum 实现了创建相册的业务逻辑
func (s *albumService) CreateAlbum(ctx context.Context, params CreateAlbumParams) (*model.Album, error) {
	album := &model.Album{
		ImageUrl:     params.ImageUrl,
		BigImageUrl:  params.BigImageUrl,
		DownloadUrl:  params.DownloadUrl,
		ThumbParam:   params.ThumbParam,
		BigParam:     params.BigParam,
		Tags:         strings.Join(params.Tags, ","),
		Width:        params.Width,
		Height:       params.Height,
		FileSize:     params.FileSize,
		Format:       params.Format,
		FileHash:     params.FileHash,
		AspectRatio:  getSimplifiedAspectRatioString(params.Width, params.Height),
		DisplayOrder: params.DisplayOrder,
	}

	// 在存入数据库前，应用默认值
	s.applyDefaultAlbumParams(album)

	finalAlbum, status, err := s.albumRepo.CreateOrRestore(ctx, album)
	if err != nil {
		return nil, fmt.Errorf("处理相册时发生数据库错误: %w", err)
	}

	// 根据返回的状态处理业务逻辑
	switch status {
	case repository.StatusCreated:
		fmt.Printf("新图片已创建，ID: %d\n", finalAlbum.ID)
		if len(params.Tags) > 0 {
			if _, err := s.tagRepo.FindOrCreate(ctx, params.Tags); err != nil {
				fmt.Printf("处理新图片标签时发生错误: %v\n", err)
			}
		}
	case repository.StatusRestored:
		fmt.Printf("已恢复并更新了被删除的图片，ID: %d\n", finalAlbum.ID)
		if len(params.Tags) > 0 {
			if _, err := s.tagRepo.FindOrCreate(ctx, params.Tags); err != nil {
				fmt.Printf("处理已恢复图片标签时发生错误: %v\n", err)
			}
		}
	case repository.StatusExisted:
		return nil, fmt.Errorf("这张图片已存在，id是%d，请勿重复添加", finalAlbum.ID)
	default:
		return nil, fmt.Errorf("处理相册时发生未知状态")
	}

	// 在返回最终结果前，再次应用默认值，确保返回给上层的数据是完整的
	s.applyDefaultAlbumParams(finalAlbum)
	return finalAlbum, nil
}

// DeleteAlbum 实现了删除相册的业务逻辑
func (s *albumService) DeleteAlbum(ctx context.Context, id uint) error {
	return s.albumRepo.Delete(ctx, id)
}

// UpdateAlbum 实现了更新相册的业务逻辑
func (s *albumService) UpdateAlbum(ctx context.Context, id uint, params UpdateAlbumParams) (*model.Album, error) {
	album, err := s.albumRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("查找待更新相册失败: %w", err)
	}
	if album == nil {
		return nil, fmt.Errorf("ID为 %d 的相册不存在", id)
	}

	// 更新字段
	album.ImageUrl = params.ImageUrl
	album.BigImageUrl = params.BigImageUrl
	album.DownloadUrl = params.DownloadUrl
	album.ThumbParam = params.ThumbParam
	album.BigParam = params.BigParam
	album.Tags = strings.Join(params.Tags, ",")

	if params.DisplayOrder != nil {
		album.DisplayOrder = *params.DisplayOrder
	}

	if err := s.albumRepo.Update(ctx, album); err != nil {
		return nil, fmt.Errorf("更新相册失败: %w", err)
	}

	// 在返回更新后的 album 对象前，应用默认值，确保数据一致性
	s.applyDefaultAlbumParams(album)
	return album, nil
}

// FindAlbums 实现了查找相册的业务逻辑
func (s *albumService) FindAlbums(ctx context.Context, params FindAlbumsParams) (*repository.PageResult[model.Album], error) {
	opts := repository.AlbumQueryOptions{
		PageQuery: repository.PageQuery{
			Page:     params.Page,
			PageSize: params.PageSize,
		},
		Tag:   params.Tag,
		Start: params.Start,
		End:   params.End,
		Sort:  params.Sort,
	}

	pageResult, err := s.albumRepo.FindListByOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	// 遍历结果集，为每一项应用默认值
	for _, album := range pageResult.Items {
		s.applyDefaultAlbumParams(album)
	}

	return pageResult, nil
}

// IncrementAlbumStat 实现了更新统计数据的业务逻辑
func (s *albumService) IncrementAlbumStat(ctx context.Context, id uint, statType string) error {
	switch statType {
	case "view":
		return s.albumRepo.IncrementViewCount(ctx, id)
	case "download":
		return s.albumRepo.IncrementDownloadCount(ctx, id)
	default:
		return fmt.Errorf("无效的统计类型: %s", statType)
	}
}

// BatchImportAlbums 实现批量导入相册的业务逻辑
func (s *albumService) BatchImportAlbums(ctx context.Context, params BatchImportParams) (*BatchImportResult, error) {
	result := &BatchImportResult{
		Errors:     make([]BatchImportError, 0),
		Duplicates: make([]string, 0),
	}

	// 获取现有的所有图片哈希，用于去重
	existingHashesMap := make(map[string]bool)
	allExisting, err := s.albumRepo.FindListByOptions(ctx, repository.AlbumQueryOptions{
		PageQuery: repository.PageQuery{
			Page:     1,
			PageSize: 100000, // 获取所有记录用于去重
		},
	})
	if err != nil {
		log.Printf("获取现有图片列表失败: %v", err)
	} else {
		for _, album := range allExisting.Items {
			if album.FileHash != "" {
				existingHashesMap[album.FileHash] = true
			}
		}
	}

	// 批量处理每个URL
	for i, url := range params.URLs {
		displayOrder := params.DisplayOrder + i

		// 获取图片元数据
		metadata, err := s.fetchImageMetadata(url)
		if err != nil {
			result.FailCount++
			result.Errors = append(result.Errors, BatchImportError{
				URL:    url,
				Reason: fmt.Sprintf("获取图片元数据失败: %v", err),
			})
			log.Printf("获取图片元数据失败 [%s]: %v", url, err)
			continue
		}

		// 检查是否已存在
		if existingHashesMap[metadata.FileHash] {
			result.SkipCount++
			result.Duplicates = append(result.Duplicates, url)
			log.Printf("跳过重复图片 [%s]", url)
			continue
		}

		// 创建相册记录
		_, err = s.CreateAlbum(ctx, CreateAlbumParams{
			ImageUrl:     url,
			BigImageUrl:  url,
			DownloadUrl:  url,
			ThumbParam:   params.ThumbParam,
			BigParam:     params.BigParam,
			Tags:         params.Tags,
			Width:        metadata.Width,
			Height:       metadata.Height,
			FileSize:     metadata.FileSize,
			Format:       metadata.Format,
			FileHash:     metadata.FileHash,
			DisplayOrder: displayOrder,
		})

		if err != nil {
			// 检查是否为重复错误
			if strings.Contains(err.Error(), "已存在") || strings.Contains(err.Error(), "重复") {
				result.SkipCount++
				result.Duplicates = append(result.Duplicates, url)
				log.Printf("后端检测到重复图片 [%s]: %v", url, err)
			} else {
				result.FailCount++
				result.Errors = append(result.Errors, BatchImportError{
					URL:    url,
					Reason: err.Error(),
				})
				log.Printf("创建相册记录失败 [%s]: %v", url, err)
			}
		} else {
			result.SuccessCount++
			// 将新添加的哈希值加入集合，防止本批次内重复
			existingHashesMap[metadata.FileHash] = true
		}
	}

	return result, nil
}

// ImageMetadata 图片元数据
type ImageMetadata struct {
	Width    int
	Height   int
	FileSize int64
	Format   string
	FileHash string
}

// fetchImageMetadata 获取图片元数据
func (s *albumService) fetchImageMetadata(url string) (*ImageMetadata, error) {
	// 创建HTTP客户端，设置超时
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求图片失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态: %d", resp.StatusCode)
	}

	// 读取图片数据到内存
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取图片数据失败: %w", err)
	}

	// 解析图片以获取尺寸
	img, format, err := image.DecodeConfig(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("解析图片失败: %w", err)
	}

	// 计算文件哈希
	hash := sha256.Sum256(data)
	fileHash := hex.EncodeToString(hash[:])

	// 确定文件格式
	fileFormat := format
	if fileFormat == "" {
		fileFormat = path.Ext(url)
		if len(fileFormat) > 0 && fileFormat[0] == '.' {
			fileFormat = fileFormat[1:]
		}
		if fileFormat == "" {
			fileFormat = "unknown"
		}
	}

	return &ImageMetadata{
		Width:    img.Width,
		Height:   img.Height,
		FileSize: int64(len(data)),
		Format:   fileFormat,
		FileHash: fileHash,
	}, nil
}

// gcd 函数用于计算两个整数的最大公约数
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// getSimplifiedAspectRatioString 根据宽度和高度返回 "宽:高" 格式的最简比例字符串
func getSimplifiedAspectRatioString(width, height int) string {
	if width <= 0 || height <= 0 {
		return "0:0"
	}

	commonDivisor := gcd(width, height)
	return fmt.Sprintf("%d:%d", width/commonDivisor, height/commonDivisor)
}

// applyDefaultAlbumParams 是一个辅助方法，用于为一个相册模型填充默认值。
// 它检查几个关键字段，如果为空，则从配置中获取默认值或使用其他字段进行填充。
func (s *albumService) applyDefaultAlbumParams(album *model.Album) {
	if album == nil {
		return
	}

	if album.BigImageUrl == "" {
		album.BigImageUrl = album.ImageUrl
	}
	if album.DownloadUrl == "" {
		album.DownloadUrl = album.ImageUrl
	}

	if album.ThumbParam == "" {
		album.ThumbParam = s.settingSvc.Get(constant.KeyDefaultThumbParam.String())
	}
	if album.BigParam == "" {
		album.BigParam = s.settingSvc.Get(constant.KeyDefaultBigParam.String())
	}
}
