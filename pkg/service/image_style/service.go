/*
 * @Description: ImageStyleService 对外实现
 * @Author: 安知鱼
 *
 * 承担 Spec §6.1 的四个对外职责：
 *   Process                    按需处理并缓存
 *   ResolveUploadURLSuffix     上传返回时拼默认样式后缀
 *   PurgeCache                 按策略 / 样式名 / 文件过滤清理缓存
 *   Stats                      查询缓存统计
 *
 * 流程（Process）：
 *   1. Matcher.Match → ResolvedStyle (或 ErrStyleNotApplicable / NotFound)
 *   2. ResolvedStyle.Hash() → styleHash
 *   3. cache.Get 命中 → 返回 StyleResult{FromCache: true}
 *   4. singleflight.Do(cacheKey)：
 *      - double-check cache
 *      - provider.Get 读原图字节 → engine.Process → cache.Put
 *   5. cache.Get 二次读取（拿独立 ReadCloser）→ 返回 StyleResult
 *
 * 并发保障：singleflight 合并相同 key 的并发请求，确保 engine.Process 仅执行一次。
 */
package image_style

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"golang.org/x/sync/singleflight"

	"github.com/anzhiyu-c/anheyu-app/internal/infra/storage"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/image_style/engine"
)

// ImageStyleService 是图片样式处理服务的对外接口。
type ImageStyleService interface {
	Process(ctx context.Context, req *StyleRequest) (*StyleResult, error)
	ResolveUploadURLSuffix(policy *model.StoragePolicy, filename string) string
	PurgeCache(ctx context.Context, policyID uint, styleName string, fileID uint) (int, error)
	Stats(ctx context.Context, policyID uint) (*CacheStats, error)
}

// Service 是 ImageStyleService 的唯一实现。
// Phase 2 接入 vips 后，engine 字段会被替换为 AutoEngine(VipsEngine/NativeGoEngine)；
// 其他字段保持不变。
type Service struct {
	engine      engine.Engine
	cache       Cache
	providers   map[constant.StoragePolicyType]storage.IStorageProvider
	policyRepo  repository.StoragePolicyRepository
	watermarker Watermarker

	sfGroup singleflight.Group
}

// NewService 构造 ImageStyleService。watermarker == nil 时自动使用 NoopWatermarker。
func NewService(
	eng engine.Engine,
	cache Cache,
	providers map[constant.StoragePolicyType]storage.IStorageProvider,
	policyRepo repository.StoragePolicyRepository,
	watermarker Watermarker,
) *Service {
	if watermarker == nil {
		watermarker = NewNoopWatermarker()
	}
	return &Service{
		engine:      eng,
		cache:       cache,
		providers:   providers,
		policyRepo:  policyRepo,
		watermarker: watermarker,
	}
}

// 静态断言 Service 满足 ImageStyleService 接口。
var _ ImageStyleService = (*Service)(nil)

// Process 对齐 Spec §6.1。
func (s *Service) Process(ctx context.Context, req *StyleRequest) (*StyleResult, error) {
	if req == nil || req.Policy == nil || req.File == nil {
		return nil, ErrStyleProcessFailed
	}

	// 1. Matcher 决策
	resolved, err := Match(req.Policy, req.Filename, req.StyleName, req.DynamicOpts)
	if err != nil {
		return nil, err
	}
	styleHash := resolved.Hash()
	policyID := req.Policy.ID
	fileID := req.File.ID

	// 2. 快速路径：直接查 cache
	if entry, rc, err := s.cache.Get(ctx, policyID, fileID, styleHash); err == nil {
		return &StyleResult{
			ContentType:     entry.MIME,
			Reader:          rc,
			Size:            entry.Size,
			FromCache:       true,
			StyleHash:       styleHash,
			LastModified:    entry.LastAccessAt,
			RequestedFormat: resolved.Format,
		}, nil
	}

	// 3. 进入 singleflight，合并并发请求
	key := cacheKey(policyID, fileID, styleHash)
	_, errDo, _ := s.sfGroup.Do(key, func() (any, error) {
		// double-check：其他并发 goroutine 可能已填好 cache
		if _, rc, err := s.cache.Get(ctx, policyID, fileID, styleHash); err == nil {
			_ = rc.Close()
			return nil, nil
		}
		return nil, s.processAndPut(ctx, req, resolved, styleHash)
	})
	if errDo != nil {
		return nil, errDo
	}

	// 4. 拿独立的 ReadCloser
	entry, rc, err := s.cache.Get(ctx, policyID, fileID, styleHash)
	if err != nil {
		return nil, fmt.Errorf("%w: 处理完成但缓存查询失败: %v", ErrStyleProcessFailed, err)
	}
	return &StyleResult{
		ContentType:     entry.MIME,
		Reader:          rc,
		Size:            entry.Size,
		FromCache:       false,
		StyleHash:       styleHash,
		LastModified:    entry.CreatedAt,
		RequestedFormat: resolved.Format,
	}, nil
}

// processAndPut 读原图、走引擎、写缓存；失败返回包装后的 ErrStyleProcessFailed。
func (s *Service) processAndPut(ctx context.Context, req *StyleRequest, resolved *ResolvedStyle, styleHash string) error {
	rawBytes, err := s.readOriginalBytes(ctx, req.Policy, req.File)
	if err != nil {
		return fmt.Errorf("%w: 读取原图失败: %v", ErrStyleProcessFailed, err)
	}

	styleCfg := model.ImageStyleConfig{
		Format:     resolved.Format,
		Quality:    resolved.Quality,
		AutoRotate: resolved.AutoRotate,
		Resize:     resolved.Resize,
		Watermark:  resolved.Watermark,
	}

	var buf bytes.Buffer
	mime, err := s.engine.Process(ctx, bytes.NewReader(rawBytes), styleCfg, &buf)
	if err != nil {
		return fmt.Errorf("%w: 引擎处理失败: %v", ErrStyleProcessFailed, err)
	}

	ext := extFromMIME(mime)
	if _, err := s.cache.Put(ctx, req.Policy.ID, req.File.ID, styleHash, mime, ext, buf.Bytes()); err != nil {
		return fmt.Errorf("%w: 写入缓存失败: %v", ErrStyleProcessFailed, err)
	}
	return nil
}

// readOriginalBytes 通过 storage provider 读取原图完整字节。
// 注意：Phase 1 为简化，直接读到内存。Phase 3 若要引入大图分块需再做改造。
func (s *Service) readOriginalBytes(ctx context.Context, policy *model.StoragePolicy, file *model.File) ([]byte, error) {
	if file.PrimaryEntity == nil || !file.PrimaryEntity.Source.Valid {
		return nil, errors.New("文件缺少物理存储实体")
	}
	provider, ok := s.providers[policy.Type]
	if !ok {
		return nil, fmt.Errorf("未注册的存储类型: %s", policy.Type)
	}
	// 保留 source 的原始形态：LocalProvider 看前导 "/" 判定绝对路径 / 相对路径；
	// 云端 provider（OSS/COS 等）也期望原始 key，不要剥 "/"。
	rc, err := provider.Get(ctx, policy, file.PrimaryEntity.Source.String)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// ResolveUploadURLSuffix 实现 Spec §8.4，薄壳代理到纯函数。
func (s *Service) ResolveUploadURLSuffix(policy *model.StoragePolicy, filename string) string {
	return resolveUploadURLSuffix(policy, filename)
}

// PurgeCache 按过滤器清理缓存条目；
//
//   - policyID == 0 → 不按策略过滤（代表"全部策略"）
//   - fileID   == 0 → 不按文件过滤
//   - styleName != "" → 查策略找到该样式，计算 hash，按 StyleHash 过滤
//     Phase 1 的 styleName 支持依赖 policyRepo；未注入时若传了非空 styleName 会返回错误。
func (s *Service) PurgeCache(ctx context.Context, policyID uint, styleName string, fileID uint) (int, error) {
	opts := PurgeOpts{}
	if policyID != 0 {
		p := policyID
		opts.PolicyID = &p
	}
	if fileID != 0 {
		f := fileID
		opts.FileID = &f
	}
	if styleName != "" {
		if s.policyRepo == nil {
			return 0, errors.New("PurgeCache: 按 style_name 过滤需要 policyRepo 注入")
		}
		if policyID == 0 {
			return 0, errors.New("PurgeCache: 按 style_name 过滤必须同时指定 policyID")
		}
		policy, err := s.policyRepo.FindByID(ctx, policyID)
		if err != nil {
			return 0, fmt.Errorf("PurgeCache: 查询策略失败: %w", err)
		}
		styleCfg, ok := findStyleByName(policy, styleName)
		if !ok {
			return 0, nil // 样式不存在 → 无可清理
		}
		resolved := styleToResolved(styleCfg)
		hash := resolved.Hash()
		opts.StyleHash = &hash
	}
	return s.cache.Purge(ctx, opts)
}

// Stats 返回指定策略的缓存统计。policyID == 0 表示跨策略聚合。
func (s *Service) Stats(ctx context.Context, policyID uint) (*CacheStats, error) {
	stats, err := s.cache.Stats(ctx, policyID)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// extFromMIME 将 engine 返回的 MIME 映射回文件扩展名（用于缓存文件命名）。
func extFromMIME(mime string) string {
	switch mime {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/avif":
		return "avif"
	case "image/heic", "image/heif":
		return "heic"
	case "image/gif":
		return "gif"
	default:
		log.Printf("[image_style] 未识别的 MIME=%s，缓存扩展名退化为 bin", mime)
		return "bin"
	}
}
