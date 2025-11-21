/*
 * @Description: 配置导入导出服务
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

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

// ImportExportService 定义了配置导入导出服务的接口
type ImportExportService interface {
	// ExportConfig 导出数据库配置表数据
	ExportConfig(ctx context.Context) ([]byte, error)
	// ImportConfig 导入配置数据到数据库
	ImportConfig(ctx context.Context, content io.Reader) error
}

// importExportService 是 ImportExportService 接口的实现
type importExportService struct {
	settingRepo    repository.SettingRepository // 配置仓库
	settingService setting.SettingService       // 配置服务，用于刷新缓存
}

// NewImportExportService 创建一个新的配置导入导出服务实例
func NewImportExportService(settingRepo repository.SettingRepository, settingService setting.SettingService) ImportExportService {
	return &importExportService{
		settingRepo:    settingRepo,
		settingService: settingService,
	}
}

// ExportConfig 导出数据库配置表数据
func (s *importExportService) ExportConfig(ctx context.Context) ([]byte, error) {
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
func (s *importExportService) ImportConfig(ctx context.Context, content io.Reader) error {
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

	// 刷新内存缓存，使配置立即生效
	if err := s.settingService.LoadAllSettings(ctx); err != nil {
		log.Printf("⚠️ 警告: 刷新配置缓存失败: %v", err)
		// 这里不返回错误，因为数据库已经更新成功了
		// 只是缓存刷新失败，用户可以通过重启应用来加载新配置
	} else {
		log.Printf("✅ 配置缓存已刷新，新配置已立即生效")
	}

	log.Printf("✅ 配置数据导入成功，共导入 %d 项配置", len(configMap))
	return nil
}
