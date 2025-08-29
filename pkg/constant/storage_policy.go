/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-06-23 15:10:56
 * @LastEditTime: 2025-08-29 10:13:27
 * @LastEditors: 安知鱼
 */
package constant

// StoragePolicyType 定义了存储策略的类型，提供了更强的类型安全
type StoragePolicyType string

// 定义支持的存储策略类型常量
const (
	PolicyTypeLocal    StoragePolicyType = "local"
	PolicyTypeOneDrive StoragePolicyType = "onedrive"
	// 如果未来要支持 Amazon S3，可以在这里添加:
	// PolicyTypeS3 StoragePolicyType = "s3"

	// UploadMethodSettingKey 是存储策略中定义上传方式的键
	UploadMethodSettingKey = "upload_method"
	// OneDriveClientIDSettingKey 是 OneDrive 策略中定义客户端 ID 的键
	DriveTypeSettingKey = "drive_type"
	// AllowedExtensionsSettingKey 是存储策略中定义允许扩展名列表的键
	AllowedExtensionsSettingKey = "allowed_extensions"

	// UploadMethodServer 代表服务端中转上传
	UploadMethodServer = "server"
	// UploadMethodClient 代表客户端直传
	UploadMethodClient = "client"
)

// Storage Policy Flags
const (
	PolicyFlagArticleImage = "article_image" // PolicyFlagArticleImage 标志着用于文章图片的策略 & 默认的VFS目录
	PolicyFlagCommentImage = "comment_image" // PolicyFlagCommentImage 标志着用于评论图片的策略 & 默认的VFS目录
)

// Default Storage Policy configurations
const (
	DefaultArticlePolicyName = "内置-文章图片"
	DefaultCommentPolicyName = "内置-评论图片"
	DefaultArticlePolicyPath = "data/storage/article_image" // 相对于应用根目录
	DefaultCommentPolicyPath = "data/storage/comment_image" // 相对于应用根目录
)

// IsValid 检查给定的类型是否是受支持的存储策略类型
func (t StoragePolicyType) IsValid() bool {
	switch t {
	case PolicyTypeLocal, PolicyTypeOneDrive:
		return true
	default:
		return false
	}
}
