/*
 * @Description: 配置文件备份服务
 * @Author: 安知鱼
 * @Date: 2025-10-19
 */
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

// BackupInfo 定义了备份的元数据信息
type BackupInfo struct {
	Filename    string    `json:"filename"`    // 备份文件名
	Size        int64     `json:"size"`        // 文件大小（字节）
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	Description string    `json:"description"` // 备份描述
	IsAuto      bool      `json:"is_auto"`     // 是否自动备份
}

// BackupService 定义了配置备份服务的接口
type BackupService interface {
	// CreateBackup 创建配置文件备份
	CreateBackup(ctx context.Context, description string, isAuto bool) (*BackupInfo, error)

	// ListBackups 列出所有备份
	ListBackups(ctx context.Context) ([]*BackupInfo, error)

	// RestoreBackup 从备份恢复配置
	RestoreBackup(ctx context.Context, filename string) error

	// DeleteBackup 删除指定的备份
	DeleteBackup(ctx context.Context, filename string) error

	// CleanOldBackups 清理旧的备份文件，保留最近的 keepCount 个
	CleanOldBackups(ctx context.Context, keepCount int) error

	// ExportConfig 导出当前配置文件内容
	ExportConfig(ctx context.Context) ([]byte, error)

	// ImportConfig 导入配置文件
	ImportConfig(ctx context.Context, content io.Reader) error

	// SetMaxBackupCount 设置最大备份数量
	SetMaxBackupCount(maxCount int) error

	// GetMaxBackupCount 获取最大备份数量
	GetMaxBackupCount() int
}

// backupService 是 BackupService 接口的实现
type backupService struct {
	configFilePath string                       // 配置文件路径
	backupDir      string                       // 备份目录
	settingRepo    repository.SettingRepository // 配置仓库
	maxBackupCount int                          // 最大备份数量，0表示无限制
}

// NewBackupService 创建一个新的配置备份服务实例
func NewBackupService(configFilePath string, settingRepo repository.SettingRepository) BackupService {
	return NewBackupServiceWithLimit(configFilePath, settingRepo, 10) // 默认保留10个备份
}

// NewBackupServiceWithLimit 创建一个新的配置备份服务实例，可指定最大备份数量
func NewBackupServiceWithLimit(configFilePath string, settingRepo repository.SettingRepository, maxBackupCount int) BackupService {
	backupDir := filepath.Join(filepath.Dir(configFilePath), "backup")

	// 确保备份目录存在
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Printf("警告: 创建备份目录失败: %v", err)
	}

	// 验证最大备份数量
	if maxBackupCount < 0 {
		log.Printf("警告: 最大备份数量不能为负数，设置为默认值10")
		maxBackupCount = 10
	}

	return &backupService{
		configFilePath: configFilePath,
		backupDir:      backupDir,
		settingRepo:    settingRepo,
		maxBackupCount: maxBackupCount,
	}
}

// CreateBackup 创建配置文件备份
func (s *backupService) CreateBackup(ctx context.Context, description string, isAuto bool) (*BackupInfo, error) {
	// 检查原配置文件是否存在
	if _, err := os.Stat(s.configFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置文件不存在: %s", s.configFilePath)
	}

	// 生成备份文件名：conf_backup_20250119_143022.ini
	timestamp := time.Now().Format("20060102_150405")
	backupFilename := fmt.Sprintf("conf_backup_%s.ini", timestamp)
	backupPath := filepath.Join(s.backupDir, backupFilename)

	// 复制配置文件到备份目录
	if err := s.copyFile(s.configFilePath, backupPath); err != nil {
		return nil, fmt.Errorf("备份配置文件失败: %w", err)
	}

	// 获取文件信息
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("获取备份文件信息失败: %w", err)
	}

	// 创建备份元数据文件
	metadata := BackupInfo{
		Filename:    backupFilename,
		Size:        fileInfo.Size(),
		CreatedAt:   time.Now(),
		Description: description,
		IsAuto:      isAuto,
	}

	// 保存元数据
	if err := s.saveMetadata(backupFilename, &metadata); err != nil {
		log.Printf("警告: 保存备份元数据失败: %v", err)
		// 即使元数据保存失败，备份文件也已经创建成功
	}

	log.Printf("✅ 配置备份成功: %s (大小: %d 字节)", backupFilename, metadata.Size)

	// 自动清理超出限制的旧备份
	if s.maxBackupCount > 0 {
		if err := s.CleanOldBackups(ctx, s.maxBackupCount); err != nil {
			log.Printf("警告: 自动清理旧备份失败: %v", err)
			// 不影响备份创建的成功
		}
	}

	return &metadata, nil
}

// ListBackups 列出所有备份
func (s *backupService) ListBackups(ctx context.Context) ([]*BackupInfo, error) {
	// 读取备份目录中的所有文件
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*BackupInfo{}, nil
		}
		return nil, fmt.Errorf("读取备份目录失败: %w", err)
	}

	var backups []*BackupInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只处理 .ini 文件
		if !strings.HasSuffix(entry.Name(), ".ini") {
			continue
		}

		// 尝试加载元数据
		metadata := s.loadMetadata(entry.Name())

		// 如果元数据不存在，从文件信息创建基本元数据
		if metadata == nil {
			info, err := entry.Info()
			if err != nil {
				log.Printf("警告: 获取文件 %s 信息失败: %v", entry.Name(), err)
				continue
			}

			metadata = &BackupInfo{
				Filename:    entry.Name(),
				Size:        info.Size(),
				CreatedAt:   info.ModTime(),
				Description: "旧版本备份",
				IsAuto:      false,
			}
		}

		backups = append(backups, metadata)
	}

	// 按创建时间倒序排序（最新的在前面）
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// RestoreBackup 从备份恢复配置
func (s *backupService) RestoreBackup(ctx context.Context, filename string) error {
	backupPath := filepath.Join(s.backupDir, filename)

	// 检查备份文件是否存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", filename)
	}

	// 在恢复之前，先备份当前配置（作为安全措施）
	_, err := s.CreateBackup(ctx, "恢复前自动备份", true)
	if err != nil {
		log.Printf("警告: 创建恢复前备份失败: %v", err)
		// 继续恢复操作
	}

	// 复制备份文件到配置文件位置
	if err := s.copyFile(backupPath, s.configFilePath); err != nil {
		return fmt.Errorf("恢复配置失败: %w", err)
	}

	log.Printf("✅ 配置已从备份恢复: %s", filename)
	return nil
}

// DeleteBackup 删除指定的备份
func (s *backupService) DeleteBackup(ctx context.Context, filename string) error {
	backupPath := filepath.Join(s.backupDir, filename)

	// 检查备份文件是否存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", filename)
	}

	// 删除备份文件
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("删除备份文件失败: %w", err)
	}

	// 删除元数据文件（如果存在）
	metadataPath := s.getMetadataPath(filename)
	if _, err := os.Stat(metadataPath); err == nil {
		os.Remove(metadataPath)
	}

	log.Printf("✅ 备份文件已删除: %s", filename)
	return nil
}

// CleanOldBackups 清理旧的备份文件，保留最近的 keepCount 个
func (s *backupService) CleanOldBackups(ctx context.Context, keepCount int) error {
	if keepCount < 1 {
		return fmt.Errorf("保留数量必须大于0")
	}

	backups, err := s.ListBackups(ctx)
	if err != nil {
		return err
	}

	// 如果备份数量不超过保留数量，不需要清理
	if len(backups) <= keepCount {
		log.Printf("当前备份数量 %d，不需要清理", len(backups))
		return nil
	}

	// 删除多余的旧备份
	deleteCount := 0
	for i := keepCount; i < len(backups); i++ {
		if err := s.DeleteBackup(ctx, backups[i].Filename); err != nil {
			log.Printf("警告: 删除旧备份 %s 失败: %v", backups[i].Filename, err)
			continue
		}
		deleteCount++
	}

	log.Printf("✅ 清理完成，删除了 %d 个旧备份，保留 %d 个最新备份", deleteCount, keepCount)
	return nil
}

// copyFile 复制文件
func (s *backupService) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// getMetadataPath 获取元数据文件路径
func (s *backupService) getMetadataPath(filename string) string {
	return filepath.Join(s.backupDir, strings.TrimSuffix(filename, ".ini")+".json")
}

// saveMetadata 保存备份元数据
func (s *backupService) saveMetadata(filename string, metadata *BackupInfo) error {
	metadataPath := s.getMetadataPath(filename)

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// loadMetadata 加载备份元数据
func (s *backupService) loadMetadata(filename string) *BackupInfo {
	metadataPath := s.getMetadataPath(filename)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil
	}

	var metadata BackupInfo
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil
	}

	return &metadata
}

// ExportConfig 导出数据库配置表数据
func (s *backupService) ExportConfig(ctx context.Context) ([]byte, error) {
	// 从数据库读取所有配置
	settings, err := s.settingRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("从数据库读取配置失败: %w", err)
	}

	// 转换为 map 格式，便于导出和导入
	configMap := make(map[string]string)
	for _, setting := range settings {
		configMap[setting.ConfigKey] = setting.Value
	}

	// 序列化为 JSON
	data, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化配置数据失败: %w", err)
	}

	log.Printf("✅ 配置数据导出成功，共 %d 项配置，大小: %d 字节", len(configMap), len(data))
	return data, nil
}

// ImportConfig 导入配置数据到数据库
func (s *backupService) ImportConfig(ctx context.Context, content io.Reader) error {
	// 读取上传的内容
	data, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("读取上传内容失败: %w", err)
	}

	// 验证内容不为空
	if len(data) == 0 {
		return fmt.Errorf("上传的配置文件为空")
	}

	// 解析 JSON 数据
	var configMap map[string]string
	if err := json.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("解析配置数据失败，请确保文件格式正确: %w", err)
	}

	if len(configMap) == 0 {
		return fmt.Errorf("配置文件中没有有效的配置项")
	}

	// 批量更新到数据库
	if err := s.settingRepo.Update(ctx, configMap); err != nil {
		return fmt.Errorf("更新配置到数据库失败: %w", err)
	}

	log.Printf("✅ 配置数据导入成功，共导入 %d 项配置", len(configMap))
	return nil
}

// SetMaxBackupCount 设置最大备份数量
func (s *backupService) SetMaxBackupCount(maxCount int) error {
	if maxCount < 0 {
		return fmt.Errorf("最大备份数量不能为负数")
	}

	s.maxBackupCount = maxCount
	log.Printf("✅ 最大备份数量已设置为: %d", maxCount)

	// 如果设置了新的限制，立即清理超出限制的备份
	if maxCount > 0 {
		ctx := context.Background()
		if err := s.CleanOldBackups(ctx, maxCount); err != nil {
			log.Printf("警告: 应用新的备份限制时清理失败: %v", err)
		}
	}

	return nil
}

// GetMaxBackupCount 获取最大备份数量
func (s *backupService) GetMaxBackupCount() int {
	return s.maxBackupCount
}
