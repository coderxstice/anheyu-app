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
	log.Printf("[阿里云OSS] 开始上传文件: virtualPath=%s, BasePath=%s", virtualPath, policy.BasePath)

	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		log.Printf("[阿里云OSS] 创建客户端失败: %v", err)
		return nil, err
	}

	// 构建对象键
	// virtualPath 可能是文件名（如"abc.jpg"）或完整路径（如"/comment_image_cos/abc.jpg"）
	basePath := strings.TrimPrefix(strings.TrimSuffix(policy.BasePath, "/"), "/")
	filename := filepath.Base(virtualPath)

	var objectKey string
	if basePath == "" {
		objectKey = filename
	} else {
		objectKey = basePath + "/" + filename
	}

	log.Printf("[阿里云OSS] 上传对象: objectKey=%s (basePath=%s, filename=%s)", objectKey, basePath, filename)

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
// Delete 从阿里云OSS删除多个文件
// sources 是完整的对象键列表（如 "article_image_cos/logo.png"），已包含 basePath，无需再拼接
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
		// source 已经是完整的对象键，直接使用
		objectKey := source
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
// source 是完整的对象键（如 "article_image_cos/logo.png"），已包含 basePath，无需再拼接
func (p *AliOSSProvider) GetDownloadURL(ctx context.Context, policy *model.StoragePolicy, source string, options DownloadURLOptions) (string, error) {
	log.Printf("[阿里云OSS] GetDownloadURL调用 - source: %s, policy.Server: %s, policy.IsPrivate: %v", source, policy.Server, policy.IsPrivate)

	// 检查访问域名配置
	if policy.Server == "" {
		log.Printf("[阿里云OSS] 错误: 访问域名配置为空")
		return "", fmt.Errorf("阿里云OSS策略缺少访问域名配置")
	}

	// source 已经是完整的对象键，直接使用
	objectKey := source
	log.Printf("[阿里云OSS] 使用对象键: %s", objectKey)

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

	// 获取样式分隔符配置
	styleSeparator := ""
	if val, ok := policy.Settings["style_separator"].(string); ok {
		styleSeparator = val
	}

	log.Printf("[阿里云OSS] 配置信息 - cdnDomain: %s, sourceAuth: %v, styleSeparator: %s", cdnDomain, sourceAuth, styleSeparator)

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

		// 处理图片处理参数
		var signOptions []oss.Option
		if options.QueryParams != "" {
			// 阿里云OSS的图片处理参数格式: x-oss-process=image/format,webp
			params := strings.TrimSpace(options.QueryParams)
			params = strings.TrimPrefix(params, "?")
			if params != "" {
				signOptions = append(signOptions, oss.Process(params))
			}
		}

		// 生成预签名URL
		signedURL, err := bucket.SignURL(objectKey, oss.HTTPGet, int64(expiresIn), signOptions...)
		if err != nil {
			log.Printf("[阿里云OSS] 生成预签名URL失败: %v", err)
			return "", fmt.Errorf("生成阿里云OSS预签名URL失败: %w", err)
		}

		log.Printf("[阿里云OSS] 预签名URL生成成功: %s", signedURL)
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

		// 添加图片处理参数
		if options.QueryParams != "" {
			fullURL = appendOSSImageParams(fullURL, options.QueryParams, styleSeparator)
			log.Printf("[阿里云OSS] 添加图片处理参数后的URL: %s", fullURL)
		}

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
// Rename 重命名或移动阿里云OSS中的文件或目录
// oldVirtualPath 和 newVirtualPath 是相对于 policy.VirtualPath 的路径，需要通过 buildObjectKey 转换为完整对象键
func (p *AliOSSProvider) Rename(ctx context.Context, policy *model.StoragePolicy, oldVirtualPath, newVirtualPath string) error {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return err
	}

	// 使用 buildObjectKey 将相对路径转换为完整对象键
	oldObjectKey := p.buildObjectKey(policy, oldVirtualPath)
	newObjectKey := p.buildObjectKey(policy, newVirtualPath)

	log.Printf("[阿里云OSS] Rename: %s -> %s", oldObjectKey, newObjectKey)

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
// IsExist 检查文件是否存在于阿里云OSS中
// source 是完整的对象键（如 "article_image_cos/logo.png"），已包含 basePath，无需再拼接
func (p *AliOSSProvider) IsExist(ctx context.Context, policy *model.StoragePolicy, source string) (bool, error) {
	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		return false, err
	}

	// source 已经是完整的对象键，直接使用
	exist, err := bucket.IsObjectExist(source)
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

// appendOSSImageParams 智能地将图片处理参数附加到URL中
// 支持阿里云OSS的图片处理参数格式，如: x-oss-process=image/format,webp
// 也支持样式分隔符格式，如: !ArticleImage 或 /ArticleImage
// params 可能的格式：
// - "x-oss-process=image/format,webp" (纯查询参数)
// - "!ArticleImage" (纯样式分隔符)
// - "!ArticleImage?x-oss-process=image/format,webp" (样式分隔符 + 查询参数)
func appendOSSImageParams(baseURL, params, separator string) string {
	params = strings.TrimSpace(params)
	if params == "" {
		return baseURL
	}

	// 检查是否以样式分隔符开头（!、/、|、-）
	// 如果是，直接拼接，不做任何处理
	styleSeparatorChars := []string{"!", "/", "|", "-"}
	for _, sep := range styleSeparatorChars {
		if strings.HasPrefix(params, sep) {
			// 这是一个样式分隔符或包含样式分隔符的完整参数
			// 直接拼接到URL后面
			return baseURL + params
		}
	}

	// 移除开头的 ? 如果有的话（传统的查询参数格式）
	params = strings.TrimPrefix(params, "?")
	if params == "" {
		return baseURL
	}

	// 如果没有指定分隔符，使用默认的 ? 分隔符
	if separator == "" {
		separator = "?"
	}

	// 解析URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		// 如果解析失败，直接拼接
		if strings.Contains(baseURL, "?") {
			return baseURL + "&" + params
		}
		return baseURL + separator + params
	}

	// 阿里云OSS图片处理参数格式检测
	// 支持两种格式：
	// 1. x-oss-process=image/format,webp (标准格式)
	// 2. image/format,webp (简化格式，自动添加x-oss-process=)
	if !strings.Contains(params, "x-oss-process=") && !strings.Contains(params, "=") {
		// 简化格式，需要添加 x-oss-process= 前缀
		params = "x-oss-process=" + params
	}

	// 将参数添加到URL中
	if parsedURL.RawQuery != "" {
		parsedURL.RawQuery += "&" + params
	} else {
		// 使用配置的样式分隔符
		// 注意：如果使用了非标准分隔符（如!），需要构建特殊的URL格式
		if separator != "?" {
			// 对于非标准分隔符，直接拼接字符串
			return baseURL + separator + params
		}
		parsedURL.RawQuery = params
	}

	return parsedURL.String()
}

// CreatePresignedUploadURL 为客户端直传创建一个预签名的上传URL
// 客户端可以使用此URL直接PUT文件到阿里云OSS，无需经过服务器中转
func (p *AliOSSProvider) CreatePresignedUploadURL(ctx context.Context, policy *model.StoragePolicy, virtualPath string) (*PresignedUploadResult, error) {
	log.Printf("[阿里云OSS] 创建预签名上传URL - virtualPath: %s", virtualPath)

	_, bucket, err := p.getOSSClient(policy)
	if err != nil {
		log.Printf("[阿里云OSS] 创建客户端失败: %v", err)
		return nil, err
	}

	// 构建对象键 - 使用与 Upload 方法相同的逻辑
	// virtualPath 是完整的虚拟路径（如 "/article_image_cos/logo.png"），需要提取文件名后与 basePath 拼接
	basePath := strings.TrimPrefix(strings.TrimSuffix(policy.BasePath, "/"), "/")
	filename := filepath.Base(virtualPath)

	var objectKey string
	if basePath == "" {
		objectKey = filename
	} else {
		objectKey = basePath + "/" + filename
	}

	log.Printf("[阿里云OSS] 生成预签名URL - objectKey: %s (basePath: %s, filename: %s)", objectKey, basePath, filename)

	// 设置预签名URL的过期时间为1小时（3600秒）
	expiresIn := int64(3600)
	expirationDateTime := time.Now().Add(time.Duration(expiresIn) * time.Second)

	// 生成预签名PUT URL
	signedURL, err := bucket.SignURL(objectKey, oss.HTTPPut, expiresIn)
	if err != nil {
		log.Printf("[阿里云OSS] 生成预签名上传URL失败: %v", err)
		return nil, fmt.Errorf("生成阿里云OSS预签名上传URL失败: %w", err)
	}

	log.Printf("[阿里云OSS] 预签名上传URL生成成功，过期时间: %s", expirationDateTime.Format(time.RFC3339))

	return &PresignedUploadResult{
		UploadURL:          signedURL,
		ExpirationDateTime: expirationDateTime,
	}, nil
}
