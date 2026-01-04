/*
 * @Description: ID 生成和解码服务
 * @Author: 安知鱼
 * @Date: 2025-06-17 20:38:15
 * @LastEditTime: 2025-08-10 22:05:59
 * @LastEditors: 安知鱼
 */
package idgen

import (
	"fmt"

	"github.com/sqids/sqids-go"
)

// sqidsEncoder 是用于生成和解码短 ID 的 Sqids 编码器实例。
var sqidsEncoder *sqids.Sqids

// EntityType 定义了不同实体在生成公共 ID 时的类型标识。
const (
	EntityTypeUser           uint64 = 1  // 用户实体的类型标识
	EntityTypeFile           uint64 = 2  // 文件实体的类型标识
	EntityTypeAlbum          uint64 = 3  // 相册实体的类型标识
	EntityTypeUserGroup      uint64 = 4  // 用户组实体的类型标识
	EntityTypeStoragePolicy  uint64 = 5  // 存储策略实体的类型标识
	EntityTypeStorageEntity  uint64 = 6  // 物理存储实体的类型标识
	EntityTypeDirectLink     uint64 = 7  // 直链实体的类型标识
	EntityTypeArticle        uint64 = 8  // 文章实体的类型标识
	EntityTypePostTag        uint64 = 9  // 文章标签实体的类型标识
	EntityTypePostCategory   uint64 = 10 // 文章分类实体的类型标识
	EntityTypeComment        uint64 = 11 // 评论实体的类型标识
	EntityTypeDocSeries      uint64 = 12 // 文档系列实体的类型标识
	EntityTypeProduct        uint64 = 13 // 商品实体的类型标识
	EntityTypeProductVariant uint64 = 14 // 商品型号实体的类型标识
	EntityTypeStockItem      uint64 = 15 // 卡密实体的类型标识
	EntityTypeMembershipPlan uint64 = 16 // 会员套餐实体的类型标识
	EntityTypeUserMembership uint64 = 17 // 用户会员实体的类型标识
	EntityTypeSupportTicket  uint64 = 18 // 工单实体的类型标识
	EntityTypeTicketMessage  uint64 = 19 // 工单消息实体的类型标识
	EntityTypeNotification   uint64 = 20 // 通知实体的类型标识
)

// InitSqidsEncoder 初始化 Sqids 编码器。
func InitSqidsEncoder() error {
	s, err := sqids.New(
		sqids.Options{
			MinLength: 4,
			Alphabet:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		},
	)
	if err != nil {
		return fmt.Errorf("初始化 Sqids 编码器失败: %w", err)
	}
	sqidsEncoder = s
	return nil
}

// GeneratePublicID 增加了详细的调试日志。
func GeneratePublicID(dbID uint, entityType uint64) (string, error) {
	if sqidsEncoder == nil {
		return "", fmt.Errorf("Sqids 编码器未初始化")
	}

	numbersToEncode := []uint64{uint64(dbID), entityType}

	id, err := sqidsEncoder.Encode(numbersToEncode)
	if err != nil {
		return "", fmt.Errorf("编码公共ID失败: %w", err)
	}

	return id, nil
}

// DecodePublicID 解码公共 ID
func DecodePublicID(publicID string) (dbID uint, entityType uint64, err error) {
	if sqidsEncoder == nil {
		return 0, 0, fmt.Errorf("Sqids 编码器未初始化")
	}

	numbers := sqidsEncoder.Decode(publicID)

	if len(numbers) != 2 {
		return 0, 0, fmt.Errorf("无法从公共ID解码出预期数量的数字(期望2个，得到%d个)", len(numbers))
	}

	return uint(numbers[0]), numbers[1], nil
}

// 为了方便，可以添加一个批量解码的辅助函数
func DecodePublicIDBatch(publicIDs []string) ([]uint, error) {
	if publicIDs == nil {
		return nil, nil
	}
	dbIDs := make([]uint, len(publicIDs))
	for i, publicID := range publicIDs {
		dbID, _, err := DecodePublicID(publicID)
		if err != nil {
			return nil, fmt.Errorf("解码公共ID '%s' 失败: %w", publicID, err)
		}
		dbIDs[i] = dbID
	}
	return dbIDs, nil
}
