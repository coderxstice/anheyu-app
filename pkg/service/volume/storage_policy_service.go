/*
 * @Description: 存储策略核心服务，集成策略模式与CacheService
 * @Author: 安知鱼
 * @Date: 2025-06-23 15:23:24
 * @LastEditTime: 2025-08-17 04:09:16
 * @LastEditors: 安知鱼
 */
package volume

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/internal/infra/storage"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/utility"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/volume/strategy"
)

const (
	policyCacheTTL           = 1 * time.Hour
	oneDriveDefaultChunkSize = 50 * 1024 * 1024
)

func policyCacheKey(id uint) string {
	return fmt.Sprintf("policy:id:%d", id)
}
func policyPublicCacheKey(publicID string) string {
	return fmt.Sprintf("policy:public_id:%s", publicID)
}

type IStoragePolicyService interface {
	CreatePolicy(ctx context.Context, ownerID uint, policy *model.StoragePolicy) error
	GetPolicyByID(ctx context.Context, id string) (*model.StoragePolicy, error)
	UpdatePolicy(ctx context.Context, policy *model.StoragePolicy) error
	DeletePolicy(ctx context.Context, id string) error
	ListPolicies(ctx context.Context, page, pageSize int) ([]*model.StoragePolicy, int64, error)
	ListAll(ctx context.Context) ([]*model.StoragePolicy, error)
	GetPolicyByDatabaseID(ctx context.Context, dbID uint) (*model.StoragePolicy, error)
	GenerateAuthURL(ctx context.Context, publicPolicyID string) (string, error)
	FinalizeAuth(ctx context.Context, code string, state string) error
}

type storagePolicyService struct {
	repo             repository.StoragePolicyRepository
	fileRepo         repository.FileRepository
	txManager        repository.TransactionManager
	strategyManager  *strategy.Manager
	settingSvc       setting.SettingService
	cacheSvc         utility.CacheService
	storageProviders map[constant.StoragePolicyType]storage.IStorageProvider
}

func NewStoragePolicyService(
	repo repository.StoragePolicyRepository,
	fileRepo repository.FileRepository,
	txManager repository.TransactionManager,
	strategyManager *strategy.Manager,
	settingSvc setting.SettingService,
	cacheSvc utility.CacheService,
	storageProviders map[constant.StoragePolicyType]storage.IStorageProvider,
) IStoragePolicyService {
	return &storagePolicyService{
		repo:             repo,
		fileRepo:         fileRepo,
		txManager:        txManager,
		strategyManager:  strategyManager,
		settingSvc:       settingSvc,
		cacheSvc:         cacheSvc,
		storageProviders: storageProviders,
	}
}

// CreatePolicy 方法集成了正确的验证逻辑，并确保为每个策略创建根目录
func (s *storagePolicyService) CreatePolicy(ctx context.Context, ownerID uint, policy *model.StoragePolicy) error {

	// 1a. 基础类型验证
	if !policy.Type.IsValid() {
		return constant.ErrInvalidPolicyType
	}

	// 1b. 特定于类型的顶级字段验证
	switch policy.Type {
	case constant.PolicyTypeOneDrive:
		if policy.Server == "" || policy.BucketName == "" || policy.SecretKey == "" {
			return errors.New("对于OneDrive策略, server (endpoint), bucket_name (client_id), 和 secret_key (client_secret) 是必填项")
		}
	case constant.PolicyTypeTencentCOS:
		if policy.Server == "" || policy.BucketName == "" || policy.AccessKey == "" || policy.SecretKey == "" {
			return errors.New("对于腾讯云COS策略, server (地域endpoint), bucket_name (存储桶名称), access_key (SecretId), 和 secret_key (SecretKey) 是必填项")
		}
	case constant.PolicyTypeAliOSS:
		if policy.Server == "" || policy.BucketName == "" || policy.AccessKey == "" || policy.SecretKey == "" {
			return errors.New("对于阿里云OSS策略, server (地域endpoint), bucket_name (存储桶名称), access_key (AccessKeyId), 和 secret_key (AccessKeySecret) 是必填项")
		}
	case constant.PolicyTypeS3:
		if policy.BucketName == "" || policy.AccessKey == "" || policy.SecretKey == "" {
			return errors.New("对于AWS S3策略, bucket_name (存储桶名称), access_key (AccessKeyId), 和 secret_key (SecretAccessKey) 是必填项")
		}
		// AWS S3的endpoint是可选的，如果不提供则使用默认的AWS S3端点
	}

	// 1c. 委托给策略处理器，验证 settings 内部的字段
	strategyInstance, err := s.strategyManager.Get(policy.Type)
	if err != nil {
		return err
	}
	if err := strategyInstance.ValidateSettings(policy.Settings); err != nil {
		return fmt.Errorf("策略配置(settings)验证失败: %w", err)
	}

	// 1d. 虚拟路径规则验证
	policy.VirtualPath = strings.TrimSpace(policy.VirtualPath)
	if !strings.HasPrefix(policy.VirtualPath, "/") {
		return errors.New("策略的虚拟路径 (VirtualPath) 必须以'/'开头")
	}
	if policy.VirtualPath == "/" {
		// 此函数用于创建新的子目录策略，根策略应由系统初始化保证
		return errors.New("不能通过此方法创建根'/'策略")
	}

	// 1e. 路径唯一性验证
	existingPolicy, err := s.repo.FindByVirtualPath(ctx, policy.VirtualPath)
	if err != nil {
		return fmt.Errorf("检查虚拟路径冲突失败: %w", err)
	}
	if existingPolicy != nil {
		return fmt.Errorf("虚拟路径 '%s' 已被策略 '%s'占用", policy.VirtualPath, existingPolicy.Name)
	}

	// 1f. 名称冲突检查
	existingPolicy, err = s.repo.FindByName(ctx, policy.Name)
	if err != nil {
		return fmt.Errorf("检查策略名称失败: %w", err)
	}
	if existingPolicy != nil {
		return constant.ErrPolicyNameConflict
	}

	// 为 OneDrive 策略设置默认值
	if policy.Type == constant.PolicyTypeOneDrive {
		if policy.Settings == nil {
			policy.Settings = make(model.StoragePolicySettings)
		}
		if _, ok := policy.Settings["chunk_size"]; !ok {
			policy.Settings["chunk_size"] = oneDriveDefaultChunkSize
		}
		if _, ok := policy.Settings[constant.UploadMethodSettingKey]; !ok {
			policy.Settings[constant.UploadMethodSettingKey] = constant.UploadMethodClient
		}
	}

	// --- 第二步：启动数据库事务 ---
	err = s.txManager.Do(ctx, func(repos repository.Repositories) error {
		policyRepo := repos.StoragePolicy
		fileRepo := repos.File

		if policy.Flag != "" {
			existingFlagHolder, err := policyRepo.FindByFlag(ctx, policy.Flag)
			if err != nil {
				return fmt.Errorf("检查策略标志冲突失败: %w", err)
			}
			if existingFlagHolder != nil {
				return fmt.Errorf("策略标志 '%s' 已被策略 '%s' 使用", policy.Flag, existingFlagHolder.Name)
			}
		}

		// 2a: 创建存储策略记录
		if err := policyRepo.Create(ctx, policy); err != nil {
			return fmt.Errorf("创建存储策略记录失败: %w", err)
		}

		// 2b: 获取操作者(ownerID)自己的根目录
		ownerRootDir, err := fileRepo.FindOrCreateRootDirectory(ctx, ownerID)
		if err != nil {
			return fmt.Errorf("无法找到或创建用户(ID: %d)的根目录: %w", ownerID, err)
		}

		// 2c: 以用户的根目录为起点，递归创建虚拟路径对应的目录结构
		path := strings.Trim(policy.VirtualPath, "/")
		pathSegments := strings.Split(path, "/")

		currentParentID := ownerRootDir.ID
		var finalDir *model.File = ownerRootDir

		for _, segment := range pathSegments {
			if segment == "" {
				continue
			}
			// 使用 `FindOrCreateDirectory` 来原子性地查找或创建路径中的每一级目录
			createdOrFoundDir, err := fileRepo.FindOrCreateDirectory(ctx, currentParentID, segment, ownerID)
			if err != nil {
				return fmt.Errorf("创建或查找目录 '%s' 失败: %w", segment, err)
			}

			// 为下一次循环更新父目录ID
			currentParentID = createdOrFoundDir.ID
			finalDir = createdOrFoundDir
		}

		if finalDir == nil {
			return fmt.Errorf("无法为虚拟路径 '%s' 创建任何目录", policy.VirtualPath)
		}

		// 步骤 2c: 将最终目录的ID回写到策略的 NodeID 字段，建立强链接
		policy.NodeID = &finalDir.ID
		if err := policyRepo.Update(ctx, policy); err != nil {
			return fmt.Errorf("关联挂载点目录到存储策略失败: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// --- 第三步：更新缓存 ---
	policyBytes, jsonErr := json.Marshal(policy)
	if jsonErr == nil {
		publicID, _ := idgen.GeneratePublicID(policy.ID, idgen.EntityTypeStoragePolicy)
		s.cacheSvc.Set(ctx, policyCacheKey(policy.ID), policyBytes, policyCacheTTL)
		if publicID != "" {
			s.cacheSvc.Set(ctx, policyPublicCacheKey(publicID), policyBytes, policyCacheTTL)
		}
	}

	return nil
}

func (s *storagePolicyService) FinalizeAuth(ctx context.Context, code string, state string) error {
	policyID, err := strconv.ParseUint(state, 10, 32)
	if err != nil {
		return errors.New("无效的state参数")
	}

	policy, err := s.GetPolicyByDatabaseID(ctx, uint(policyID))
	if err != nil {
		return err
	}

	strategyInstance, err := s.strategyManager.Get(policy.Type)
	if err != nil {
		return err
	}

	authHandler := strategyInstance.GetAuthHandler()
	if authHandler == nil {
		return constant.ErrPolicyNotSupportAuth
	}

	siteURL := s.settingSvc.Get(constant.KeySiteURL.String())
	if siteURL == "" {
		return errors.New("系统未配置站点URL (siteURL), 无法完成授权")
	}

	if err := authHandler.FinalizeAuth(ctx, policy, code, siteURL); err != nil {
		return err
	}

	return s.UpdatePolicy(ctx, policy)
}

func (s *storagePolicyService) GetPolicyByDatabaseID(ctx context.Context, dbID uint) (*model.StoragePolicy, error) {
	key := policyCacheKey(dbID)
	result, err := s.cacheSvc.Get(ctx, key)
	if err == nil && result != "" {
		var policy model.StoragePolicy
		if json.Unmarshal([]byte(result), &policy) == nil {
			return &policy, nil
		}
	}
	policy, err := s.repo.FindByID(ctx, dbID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, constant.ErrPolicyNotFound
	}
	policyBytes, jsonErr := json.Marshal(policy)
	if jsonErr == nil {
		publicID, _ := idgen.GeneratePublicID(policy.ID, idgen.EntityTypeStoragePolicy)
		s.cacheSvc.Set(ctx, key, policyBytes, policyCacheTTL)
		if publicID != "" {
			s.cacheSvc.Set(ctx, policyPublicCacheKey(publicID), policyBytes, policyCacheTTL)
		}
	}
	return policy, nil
}

func (s *storagePolicyService) GetPolicyByID(ctx context.Context, publicID string) (*model.StoragePolicy, error) {
	key := policyPublicCacheKey(publicID)
	result, err := s.cacheSvc.Get(ctx, key)
	if err == nil && result != "" {
		var policy model.StoragePolicy
		if json.Unmarshal([]byte(result), &policy) == nil {
			return &policy, nil
		}
	}
	internalID, entityType, err := idgen.DecodePublicID(publicID)
	if err != nil || entityType != idgen.EntityTypeStoragePolicy {
		return nil, constant.ErrInvalidPublicID
	}
	return s.GetPolicyByDatabaseID(ctx, internalID)
}

// UpdatePolicy 实现了存储策略更新的完整业务逻辑，并包含了多重验证和保护措施。
//
// 此方法的核心功能是原子性地更新一个存储策略及其在虚拟文件系统中的表示。
// 它在一个数据库事务中完成所有操作，以确保数据的一致性。
//
// 主要逻辑包括：
//   - 保护逻辑: 严禁将默认根策略 ("/") 的挂载路径修改为任何其他值。
//   - 标志切换: 当为一个策略设置 "article_image" 或 "comment_image" 标志时，会自动移除之前持有该标志的策略的标志，确保唯一性。
//   - 完整性验证:
//   - 对所有策略类型，会验证其 `settings` 字段的内部一致性。
//   - 对 OneDrive 类型的策略，会强制校验 `server`, `bucket_name`, `secret_key` 等关键字段不能为空。
//   - 路径冲突验证: 如果策略的 `VirtualPath` 发生改变，会检查新的路径是否已被系统中的其他策略占用。
//   - 文件系统同步:
//   - 当 `VirtualPath` 改变时，此方法会自动在文件系统中“移动”或“重命名”对应的挂载点目录。
//   - 【安全特性】如果挂载点目录中已包含文件或子目录，为了防止数据结构混乱，更新操作将被阻止。
//   - 缓存管理: 在更新成功后，会自动清理相关的缓存条目。
//
// @param ctx - 请求上下文。
// @param policy - 包含待更新数据的存储策略模型，其 ID 必须有效。
// @return error - 如果更新过程中发生任何验证失败或数据库错误，则返回 error。
func (s *storagePolicyService) UpdatePolicy(ctx context.Context, policy *model.StoragePolicy) error {
	// --- 1. 在事务外获取策略的原始状态 ---
	target, err := s.repo.FindByID(ctx, policy.ID)
	if err != nil {
		return fmt.Errorf("查找待更新策略失败: %w", err)
	}
	if target == nil {
		return constant.ErrPolicyNotFound
	}

	// --- 2. 【保护逻辑】禁止更新默认的根存储策略的路径 ---
	if target.VirtualPath == "/" {
		// 允许更新根策略的除VirtualPath之外的其他字段，但禁止修改其路径
		if policy.VirtualPath != "/" {
			return errors.New("无法修改默认根存储策略的挂载路径'/'")
		}
	}

	// --- 3. 启动事务来执行所有验证和更新 ---
	err = s.txManager.Do(ctx, func(repos repository.Repositories) error {
		policyRepo := repos.StoragePolicy
		fileRepo := repos.File

		// 检查 Flag 是否有变化，并且新的 Flag 不是空字符串
		if policy.Flag != target.Flag && policy.Flag != "" {
			// 查找当前拥有该 Flag 的其他策略
			existingFlagHolder, err := policyRepo.FindByFlag(ctx, policy.Flag)
			if err != nil {
				return fmt.Errorf("检查策略标志冲突失败: %w", err)
			}
			// 如果存在另一个策略拥有此 Flag，则取消它的 Flag
			if existingFlagHolder != nil && existingFlagHolder.ID != policy.ID {
				// 直接更新特定字段，避免整个对象更新可能导致的约束冲突
				if err := policyRepo.ClearFlag(ctx, existingFlagHolder.ID); err != nil {
					return fmt.Errorf("移除旧策略的标志失败: %w", err)
				}
			}
		}

		// --- 3a. 基础和配置验证 ---
		strategyInstance, err := s.strategyManager.Get(policy.Type)
		if err != nil {
			return err
		}
		if err := strategyInstance.ValidateSettings(policy.Settings); err != nil {
			return fmt.Errorf("策略配置验证失败: %w", err)
		}

		// 在更新时，同样校验 OneDrive 策略的必填顶级字段
		if policy.Type == constant.PolicyTypeOneDrive {
			if policy.Server == "" || policy.BucketName == "" || policy.SecretKey == "" {
				return errors.New("对于OneDrive策略, server (endpoint), bucket_name (client_id), 和 secret_key (client_secret) 是必填项")
			}
		}

		// --- 3b. 检查 VirtualPath 是否发生改变 ---
		if policy.VirtualPath != target.VirtualPath {
			// 验证新的 VirtualPath 是否已被其他策略占用
			existingPolicy, err := policyRepo.FindByVirtualPath(ctx, policy.VirtualPath)
			if err != nil {
				return fmt.Errorf("检查虚拟路径冲突时出错: %w", err)
			}
			if existingPolicy != nil && existingPolicy.ID != policy.ID {
				return fmt.Errorf("无法更新策略：虚拟路径 '%s' 已被策略 '%s' 占用", policy.VirtualPath, existingPolicy.Name)
			}

			// 处理挂载点目录的移动/重命名
			if target.NodeID != nil && *target.NodeID > 0 {
				oldMountPointFile, findErr := fileRepo.FindByID(ctx, *target.NodeID)
				if findErr != nil {
					return fmt.Errorf("找不到策略 '%s' 的原始挂载点 (FileID: %d): %w", target.Name, *target.NodeID, findErr)
				}
				if oldMountPointFile.ChildrenCount > 0 {
					return fmt.Errorf("无法修改策略路径：其挂载点目录 '%s' 不为空", oldMountPointFile.Name)
				}
				newPath := strings.Trim(policy.VirtualPath, "/")
				newParentPathStr := filepath.Dir(newPath)
				newDirName := filepath.Base(newPath)
				ownerRootDir, err := fileRepo.FindOrCreateRootDirectory(ctx, oldMountPointFile.OwnerID)
				if err != nil {
					return fmt.Errorf("无法找到用户(ID: %d)的根目录: %w", oldMountPointFile.OwnerID, err)
				}
				newParentDir := ownerRootDir
				if newParentPathStr != "." && newParentPathStr != "/" {
					pathSegments := strings.Split(newParentPathStr, "/")
					currentParentID := ownerRootDir.ID
					for _, segment := range pathSegments {
						if segment == "" {
							continue
						}
						createdOrFoundDir, err := fileRepo.FindOrCreateDirectory(ctx, currentParentID, segment, oldMountPointFile.OwnerID)
						if err != nil {
							return err
						}
						currentParentID = createdOrFoundDir.ID
						newParentDir = createdOrFoundDir
					}
				}
				oldMountPointFile.ParentID = sql.NullInt64{Int64: int64(newParentDir.ID), Valid: true}
				oldMountPointFile.Name = newDirName
				if err := fileRepo.Update(ctx, oldMountPointFile); err != nil {
					return fmt.Errorf("移动/重命名挂载点目录失败: %w", err)
				}
			}
		}

		// --- 3c. 更新策略记录本身 ---
		if err := policyRepo.Update(ctx, policy); err != nil {
			return err
		}

		return nil // 事务成功
	})

	if err != nil {
		return err
	}

	// --- 4. 清理缓存 ---
	// 确保清理所有相关的缓存键
	publicID, _ := idgen.GeneratePublicID(policy.ID, idgen.EntityTypeStoragePolicy)

	// 分别清理每个缓存键，确保清理成功
	s.cacheSvc.Delete(ctx, policyCacheKey(policy.ID))
	if publicID != "" {
		s.cacheSvc.Delete(ctx, policyPublicCacheKey(publicID))
	}

	// 添加调试日志，确认缓存清理
	log.Printf("[缓存清理] 已清理存储策略缓存: ID=%d, PublicID=%s", policy.ID, publicID)

	// --- 5. 强制重新加载策略到缓存 ---
	// 确保下次访问时能立即获取到最新的策略数据
	if updatedPolicy, err := s.repo.FindByID(ctx, policy.ID); err == nil && updatedPolicy != nil {
		if policyBytes, jsonErr := json.Marshal(updatedPolicy); jsonErr == nil {
			s.cacheSvc.Set(ctx, policyCacheKey(policy.ID), policyBytes, policyCacheTTL)
			if publicID != "" {
				s.cacheSvc.Set(ctx, policyPublicCacheKey(publicID), policyBytes, policyCacheTTL)
			}
			log.Printf("[缓存预热] 已重新加载策略到缓存: ID=%d, Server=%s", policy.ID, updatedPolicy.Server)
		}
	}

	return nil
}

// DeletePolicy 方法实现了策略的删除逻辑，包括删除关联的挂载点目录
func (s *storagePolicyService) DeletePolicy(ctx context.Context, publicID string) error {
	// 1. 首先在事务外获取策略信息，以便知道要删除哪个File记录 (NodeID)
	policy, err := s.GetPolicyByID(ctx, publicID)
	if err != nil {
		if errors.Is(err, constant.ErrNotFound) {
			return nil // 尝试删除一个不存在的策略，不是错误，直接成功返回
		}
		return err
	}

	// 保护逻辑：禁止删除内置的系统策略
	if policy.Flag != "" {
		return errors.New("无法删除内置的系统策略")
	}

	// 检查是否为默认的根存储策略
	if policy.VirtualPath == "/" {
		return errors.New("无法删除默认的根存储策略")
	}

	// 2. 使用事务来原子性地删除策略及其关联的File记录
	err = s.txManager.Do(ctx, func(repos repository.Repositories) error {
		policyRepo := repos.StoragePolicy
		fileRepo := repos.File

		// 3. 【核心逻辑】删除策略关联的挂载点目录
		// 检查策略是否有对应的挂载点 (NodeID)
		if policy.NodeID != nil && *policy.NodeID > 0 {
			mountPointFile, findErr := fileRepo.FindByID(ctx, *policy.NodeID)

			// 如果找到对应的File记录
			if findErr == nil && mountPointFile != nil {
				// 【安全检查】如果目录不为空，则阻止删除
				if mountPointFile.ChildrenCount > 0 {
					return fmt.Errorf("无法删除策略 '%s'：其挂载点目录 '%s' 中包含文件或子目录", policy.Name, mountPointFile.Name)
				}

				// 目录为空，可以安全删除
				if deleteErr := fileRepo.Delete(ctx, mountPointFile.ID); deleteErr != nil {
					return fmt.Errorf("删除策略的挂载点目录失败: %w", deleteErr)
				}
			} else if !errors.Is(findErr, constant.ErrNotFound) {
				// 如果查找时发生其他错误，则终止
				return fmt.Errorf("查找策略的挂载点目录时出错: %w", findErr)
			}
			// 如果没找到对应的File记录，说明数据可能已不一致，忽略它，继续删除策略本身
		}

		// 4. 执行策略自己的删除前置任务 (例如清理云端凭证等)
		strategyInstance, strategyErr := s.strategyManager.Get(policy.Type)
		if strategyErr != nil {
			return strategyErr
		}
		if err := strategyInstance.BeforeDelete(ctx, policy); err != nil {
			return fmt.Errorf("执行策略删除前置任务失败: %w", err)
		}

		// 5. 删除策略记录本身
		if err := policyRepo.Delete(ctx, policy.ID); err != nil {
			return err
		}

		return nil // 事务成功
	})

	if err != nil {
		return err
	}

	// 6. 事务成功后，清理缓存
	s.cacheSvc.Delete(ctx, policyCacheKey(policy.ID), policyPublicCacheKey(publicID))

	return nil
}

func (s *storagePolicyService) ListPolicies(ctx context.Context, page, pageSize int) ([]*model.StoragePolicy, int64, error) {
	return s.repo.List(ctx, page, pageSize)
}
func (s *storagePolicyService) ListAll(ctx context.Context) ([]*model.StoragePolicy, error) {
	return s.repo.ListAll(ctx)
}
func (s *storagePolicyService) GenerateAuthURL(ctx context.Context, publicPolicyID string) (string, error) {
	policy, err := s.GetPolicyByID(ctx, publicPolicyID)
	if err != nil {
		return "", err
	}
	strategyInstance, err := s.strategyManager.Get(policy.Type)
	if err != nil {
		return "", err
	}
	authHandler := strategyInstance.GetAuthHandler()
	if authHandler == nil {
		return "", constant.ErrPolicyNotSupportAuth
	}
	siteURL := s.settingSvc.Get(constant.KeySiteURL.String())
	if siteURL == "" {
		return "", errors.New("系统未配置站点URL (siteURL), 无法生成回调地址")
	}
	return authHandler.GenerateAuthURL(ctx, policy, siteURL)
}
