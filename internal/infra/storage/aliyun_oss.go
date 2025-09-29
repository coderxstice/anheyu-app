/*
 * @Description: 阿里云OSS存储提供者实现
 * @Author: 安知鱼
 * @Date: 2025-09-28 18:00:00
 * @LastEditTime: 2025-09-28 18:00:00
 * @LastEditors: 安知鱼
 */
package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// AliOSSProvider 实现了 IStorageProvider 接口，用于处理与阿里云OSS的所有交互。
type AliOSSProvider struct {
}

// NewAliOSSProvider 是 AliOSSProvider 的构造函数。
func NewAliOSSProvider() IStorageProvider {
	return &AliOSSProvider{}
}

// getOSSClient 获取阿里云OSS客户端
func (p *AliOSSProvider) getOSSClient(policy *model.StoragePolicy) (*oss.Client, *oss.Bucket, error) {
	// 添加调试日志，打印策略的关键信息
	log.Printf("[阿里云OSS] 创建客户端 - 策略名称: %s, 策略ID: %d, Server: %s", policy.Name, policy.ID, policy.Server)

	// 从策略中获取配置信息
	bucketName := policy.BucketName
	if bucketName == "" {
		log.Printf("[阿里云OSS] 错误: 存储桶名称为空")
		return nil, nil, fmt.Errorf("阿里云OSS策略缺少存储桶名称")
	}

	accessKeyID := policy.AccessKey
	if accessKeyID == "" {
		return nil, nil, fmt.Errorf("阿里云OSS策略缺少AccessKey")
	}

	accessKeySecret := policy.SecretKey
	if accessKeySecret == "" {
		return nil, nil, fmt.Errorf("阿里云OSS策略缺少SecretKey")
	}

	// 从Server字段获取Endpoint，格式如: https://oss-cn-shanghai.aliyuncs.com
	endpoint := policy.Server
	if endpoint == "" {
		return nil, nil, fmt.Errorf("阿里云OSS策略缺少Endpoint配置")
	}

	// 创建OSS客户端
	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		log.Printf("[阿里云OSS] 创建客户端失败: %v", err)
		return nil, nil, fmt.Errorf("创建阿里云OSS客户端失败: %w", err)
	}

	// 获取存储桶
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		log.Printf("[阿里云OSS] 获取存储桶失败: %v", err)
		return nil, nil, fmt.Errorf("获取阿里云OSS存储桶失败: %w", err)
	}

	log.Printf("[阿里云OSS] 成功创建客户端和存储桶")
	return client, bucket, nil
}

// buildObjectKey 构建OSS对象键
func (p *AliOSSProvider) buildObjectKey(policy *model.StoragePolicy, virtualPath string) string {
	// 基础前缀路径处理
	basePath := strings.TrimSuffix(policy.BasePath, "/")
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	// 虚拟路径处理：移除开头的斜杠
	virtualPath = strings.TrimPrefix(virtualPath, "/")

	var objectKey string
	if basePath == "" || basePath == "/" {
		objectKey = virtualPath
	} else {
		objectKey = strings.TrimPrefix(basePath, "/") + "/" + virtualPath
	}

	// 确保不以斜杠开头（OSS对象键不应该以/开头）
	objectKey = strings.TrimPrefix(objectKey, "/")

	log.Printf("[阿里云OSS] 路径转换 - basePath: %s, virtualPath: %s -> objectKey: %s", basePath, virtualPath, objectKey)
	return objectKey
}

// Upload 上传文件到阿里云OSS
func (p *AliOSSProvider) Upload(ctx context.Context, file io.Reader, policy *model.StoragePolicy, virtualPath string) (*UploadResult, error) {
	log.Printf("[阿里云OSS] 开始上传文件: virtualPath=%s", virtualPath)

	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		log.Printf("[阿里云OSS] 创建客户端失败: %v", err)
		return nil, err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if objectKey == "" {
		objectKey = filepath.Base(virtualPath)
		log.Printf("[阿里云OSS] objectKey为空，使用文件名: %s", objectKey)
	}

	log.Printf("[阿里云OSS] 上传对象: objectKey=%s", objectKey)

	// 上传文件
	err = bucket.PutObject(objectKey, file)
	if err != nil {
		log.Printf("[阿里云OSS] 上传失败: %v", err)
		return nil, fmt.Errorf("上传文件到阿里云OSS失败: %w", err)
	}

	log.Printf("[阿里云OSS] 上传成功: objectKey=%s", objectKey)

	// 获取文件信息
	headers, err := bucket.GetObjectMeta(objectKey)
	if err != nil {
		return nil, fmt.Errorf("获取上传后的文件信息失败: %w", err)
	}

	// 解析文件大小
	var fileSize int64 = 0
	if contentLengthStr := headers.Get("Content-Length"); contentLengthStr != "" {
		if size, parseErr := strconv.ParseInt(contentLengthStr, 10, 64); parseErr == nil {
			fileSize = size
		}
	}

	// 获取MIME类型
	mimeType := headers.Get("Content-Type")
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(virtualPath))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	return &UploadResult{
		Source:   objectKey, // 返回对象键作为source
		Size:     fileSize,
		MimeType: mimeType,
	}, nil
}

// Get 从阿里云OSS获取文件流
func (p *AliOSSProvider) Get(ctx context.Context, policy *model.StoragePolicy, source string) (io.ReadCloser, error) {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return nil, err
	}

	body, err := bucket.GetObject(source)
	if err != nil {
		return nil, fmt.Errorf("从阿里云OSS获取文件失败: %w", err)
	}

	return body, nil
}

// List 列出阿里云OSS存储桶中的对象
func (p *AliOSSProvider) List(ctx context.Context, policy *model.StoragePolicy, virtualPath string) ([]FileInfo, error) {
	log.Printf("[阿里云OSS] List方法调用 - 策略名称: %s, virtualPath: %s", policy.Name, virtualPath)

	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return nil, err
	}

	prefix := p.buildObjectKey(policy, virtualPath)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	result, err := bucket.ListObjects(oss.Prefix(prefix), oss.Delimiter("/"))
	if err != nil {
		return nil, fmt.Errorf("列出阿里云OSS对象失败: %w", err)
	}

	var fileInfos []FileInfo

	// 处理文件对象
	for _, obj := range result.Objects {
		// 跳过目录本身
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}

		// 移除前缀，获取相对路径
		name := strings.TrimPrefix(obj.Key, prefix)
		if name == "" {
			continue
		}

		// 只显示直接子文件，不显示子目录中的文件
		if strings.Contains(name, "/") {
			continue
		}

		fileInfo := FileInfo{
			Name:    name,
			Size:    obj.Size,
			ModTime: obj.LastModified,
			IsDir:   false,
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	// 处理公共前缀（目录）
	for _, commonPrefix := range result.CommonPrefixes {
		// 移除前缀和尾随的斜杠，获取目录名
		dirName := strings.TrimSuffix(strings.TrimPrefix(commonPrefix, prefix), "/")
		if dirName == "" {
			continue
		}

		fileInfo := FileInfo{
			Name:    dirName,
			Size:    0,
			ModTime: time.Time{}, // 目录没有修改时间
			IsDir:   true,
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	log.Printf("[阿里云OSS] List完成 - 返回 %d 个项目", len(fileInfos))
	return fileInfos, nil
}

// Delete 从阿里云OSS删除多个文件
func (p *AliOSSProvider) Delete(ctx context.Context, policy *model.StoragePolicy, sources []string) error {
	if len(sources) == 0 {
		return nil
	}

	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return err
	}

	log.Printf("[阿里云OSS] Delete方法调用 - 策略: %s, 删除文件数量: %d", policy.Name, len(sources))

	for _, source := range sources {
		objectKey := p.buildObjectKey(policy, source)
		if objectKey == "" {
			objectKey = filepath.Base(source)
		}

		log.Printf("[阿里云OSS] 删除对象: %s", objectKey)
		err := bucket.DeleteObject(objectKey)
		if err != nil {
			log.Printf("[阿里云OSS] 删除对象失败: %s, 错误: %v", objectKey, err)
			return fmt.Errorf("删除阿里云OSS对象 %s 失败: %w", source, err)
		}
		log.Printf("[阿里云OSS] 成功删除对象: %s", objectKey)
	}

	return nil
}

// DeleteSingle 从阿里云OSS删除单个文件（内部使用）
func (p *AliOSSProvider) DeleteSingle(ctx context.Context, policy *model.StoragePolicy, source string) error {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return err
	}

	err = bucket.DeleteObject(source)
	if err != nil {
		return fmt.Errorf("从阿里云OSS删除文件失败: %w", err)
	}

	return nil
}

// Stream 从阿里云OSS流式传输文件到writer
func (p *AliOSSProvider) Stream(ctx context.Context, policy *model.StoragePolicy, source string, writer io.Writer) error {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return err
	}

	body, err := bucket.GetObject(source)
	if err != nil {
		return fmt.Errorf("从阿里云OSS获取文件失败: %w", err)
	}
	defer body.Close()

	_, err = io.Copy(writer, body)
	if err != nil {
		return fmt.Errorf("流式传输文件失败: %w", err)
	}

	return nil
}

// GetDownloadURL 根据存储策略权限设置生成阿里云OSS下载URL
func (p *AliOSSProvider) GetDownloadURL(ctx context.Context, policy *model.StoragePolicy, source string, options DownloadURLOptions) (string, error) {
	log.Printf("[阿里云OSS] GetDownloadURL调用 - source: %s, policy.Server: %s, policy.IsPrivate: %v", source, policy.Server, policy.IsPrivate)

	// 检查访问域名配置
	if policy.Server == "" {
		log.Printf("[阿里云OSS] 错误: 访问域名配置为空")
		return "", fmt.Errorf("阿里云OSS策略缺少访问域名配置")
	}

	// 将虚拟路径转换为对象存储路径
	objectKey := p.buildObjectKey(policy, source)
	if objectKey == "" {
		objectKey = filepath.Base(source)
		log.Printf("[阿里云OSS] objectKey为空，使用文件名: %s", objectKey)
	}
	log.Printf("[阿里云OSS] 转换路径 - source: %s -> objectKey: %s", source, objectKey)

	// 检查是否配置了CDN域名
	cdnDomain := ""
	if val, ok := policy.Settings["cdn_domain"].(string); ok && val != "" {
		// 处理CDN域名的尾随斜杠
		cdnDomain = strings.TrimSuffix(val, "/")
	}

	sourceAuth := false
	if val, ok := policy.Settings["source_auth"].(bool); ok {
		sourceAuth = val
	}

	log.Printf("[阿里云OSS] 配置信息 - cdnDomain: %s, sourceAuth: %v", cdnDomain, sourceAuth)

	// 根据是否为私有存储策略决定URL类型
	if policy.IsPrivate && !sourceAuth {
		log.Printf("[阿里云OSS] 生成预签名URL (私有策略)")

		// 私有存储策略且未开启CDN回源鉴权：生成预签名URL
		_, bucket, err := p.getOSSClient(policy)
		if err != nil {
			log.Printf("[阿里云OSS] 创建客户端失败: %v", err)
			return "", err
		}

		// 设置过期时间，默认1小时
		expiresIn := options.ExpiresIn
		if expiresIn <= 0 {
			expiresIn = 3600 // 1小时
		}

		// 生成预签名URL
		signedURL, err := bucket.SignURL(objectKey, oss.HTTPGet, int64(expiresIn))
		if err != nil {
			log.Printf("[阿里云OSS] 生成预签名URL失败: %v", err)
			return "", fmt.Errorf("生成阿里云OSS预签名URL失败: %w", err)
		}

		log.Printf("[阿里云OSS] 预签名URL生成成功")
		return signedURL, nil
	} else {
		log.Printf("[阿里云OSS] 生成公共访问URL")

		// 公共访问策略或开启了CDN回源鉴权：生成公共访问URL
		var baseURL string
		if cdnDomain != "" {
			// 使用CDN域名
			baseURL = cdnDomain
			if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
				baseURL = "https://" + baseURL
			}
		} else {
			// 使用OSS直接访问域名
			// policy.Server应该是完整的endpoint，如: https://oss-cn-shanghai.aliyuncs.com
			// 需要转换为bucket域名：https://bucket-name.oss-cn-shanghai.aliyuncs.com
			endpoint := policy.Server
			bucketName := policy.BucketName

			if strings.Contains(endpoint, "://") {
				// 解析endpoint
				parsedURL, err := url.Parse(endpoint)
				if err != nil {
					return "", fmt.Errorf("解析OSS endpoint失败: %w", err)
				}
				// 构建bucket域名
				baseURL = fmt.Sprintf("%s://%s.%s", parsedURL.Scheme, bucketName, parsedURL.Host)
			} else {
				// 如果没有协议，默认使用https
				baseURL = fmt.Sprintf("https://%s.%s", bucketName, endpoint)
			}
		}

		// 构建完整的访问URL
		fullURL := fmt.Sprintf("%s/%s", baseURL, objectKey)

		log.Printf("[阿里云OSS] 公共访问URL: %s", fullURL)
		return fullURL, nil
	}
}

// CreateDirectory 在阿里云OSS中创建目录（通过创建空对象模拟）
func (p *AliOSSProvider) CreateDirectory(ctx context.Context, policy *model.StoragePolicy, virtualPath string) error {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if !strings.HasSuffix(objectKey, "/") {
		objectKey += "/"
	}

	// OSS通过创建一个以"/"结尾的空对象来模拟目录
	err = bucket.PutObject(objectKey, strings.NewReader(""))
	if err != nil {
		return fmt.Errorf("在阿里云OSS中创建目录失败: %w", err)
	}

	return nil
}

// DeleteDirectory 删除阿里云OSS中的空目录
func (p *AliOSSProvider) DeleteDirectory(ctx context.Context, policy *model.StoragePolicy, virtualPath string) error {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if !strings.HasSuffix(objectKey, "/") {
		objectKey += "/"
	}

	err = bucket.DeleteObject(objectKey)
	if err != nil {
		return fmt.Errorf("删除阿里云OSS目录失败: %w", err)
	}

	return nil
}

// Rename 重命名或移动阿里云OSS中的文件或目录
func (p *AliOSSProvider) Rename(ctx context.Context, policy *model.StoragePolicy, oldVirtualPath, newVirtualPath string) error {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return err
	}

	oldObjectKey := p.buildObjectKey(policy, oldVirtualPath)
	newObjectKey := p.buildObjectKey(policy, newVirtualPath)

	// 复制对象到新位置
	_, err = bucket.CopyObject(oldObjectKey, newObjectKey)
	if err != nil {
		return fmt.Errorf("复制阿里云OSS对象失败: %w", err)
	}

	// 删除原对象
	err = bucket.DeleteObject(oldObjectKey)
	if err != nil {
		return fmt.Errorf("删除原阿里云OSS对象失败: %w", err)
	}

	return nil
}

// IsExist 检查文件是否存在于阿里云OSS中
func (p *AliOSSProvider) IsExist(ctx context.Context, policy *model.StoragePolicy, source string) (bool, error) {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return false, err
	}

	objectKey := p.buildObjectKey(policy, source)
	if objectKey == "" {
		objectKey = filepath.Base(source)
	}

	exist, err := bucket.IsObjectExist(objectKey)
	if err != nil {
		return false, err
	}

	return exist, nil
}

// GetThumbnail 获取缩略图（阿里云OSS不直接支持）
func (p *AliOSSProvider) GetThumbnail(ctx context.Context, policy *model.StoragePolicy, source string, size string) (*ThumbnailResult, error) {
	// 阿里云OSS本身不提供缩略图生成服务，返回不支持
	return nil, ErrFeatureNotSupported
}

// Exists 检查文件是否存在于阿里云OSS中（带policy参数的版本）
func (p *AliOSSProvider) Exists(ctx context.Context, policy *model.StoragePolicy, source string) (bool, error) {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return false, err
	}

	exists, err := bucket.IsObjectExist(source)
	if err != nil {
		return false, fmt.Errorf("检查阿里云OSS文件是否存在失败: %w", err)
	}

	return exists, nil
}
