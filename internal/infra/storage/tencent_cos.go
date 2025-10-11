/*
 * @Description: 腾讯云COS存储提供者实现
 * @Author: 安知鱼
 * @Date: 2025-09-28 12:00:00
 * @LastEditTime: 2025-10-11 20:38:54
 * @LastEditors: 安知鱼
 */
package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/tencentyun/cos-go-sdk-v5"
)

// TencentCOSProvider 实现了 IStorageProvider 接口，用于处理与腾讯云COS的所有交互。
type TencentCOSProvider struct {
}

// NewTencentCOSProvider 是 TencentCOSProvider 的构造函数。
func NewTencentCOSProvider() IStorageProvider {
	return &TencentCOSProvider{}
}

// getCOSClient 获取腾讯云COS客户端
func (p *TencentCOSProvider) getCOSClient(policy *model.StoragePolicy) (*cos.Client, error) {
	// 添加调试日志，打印策略的关键信息
	log.Printf("[腾讯云COS] 创建客户端 - 策略名称: %s, 策略ID: %d, Server: %s", policy.Name, policy.ID, policy.Server)

	// 从策略中获取配置信息
	bucketName := policy.BucketName
	if bucketName == "" {
		log.Printf("[腾讯云COS] 错误: 存储桶名称为空")
		return nil, fmt.Errorf("腾讯云COS策略缺少存储桶名称")
	}

	secretID := policy.AccessKey
	if secretID == "" {
		return nil, fmt.Errorf("腾讯云COS策略缺少SecretID")
	}

	secretKey := policy.SecretKey
	if secretKey == "" {
		return nil, fmt.Errorf("腾讯云COS策略缺少SecretKey")
	}

	// 直接使用策略中的Server字段作为访问域名
	if policy.Server == "" {
		log.Printf("[腾讯云COS] 错误: 访问域名为空")
		return nil, fmt.Errorf("腾讯云COS策略缺少访问域名配置")
	}

	u, err := url.Parse(policy.Server)
	if err != nil {
		return nil, fmt.Errorf("解析存储桶URL失败: %w", err)
	}

	// 创建COS客户端
	b := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(b, &http.Client{
		Timeout: 100 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  secretID,
			SecretKey: secretKey,
		},
	})

	return client, nil
}

// buildObjectKey 构建对象存储路径
func (p *TencentCOSProvider) buildObjectKey(policy *model.StoragePolicy, virtualPath string) string {
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

	// 确保不以斜杠开头
	objectKey = strings.TrimPrefix(objectKey, "/")

	log.Printf("[腾讯云COS] 路径转换 - basePath: %s, virtualPath: %s -> objectKey: %s", basePath, virtualPath, objectKey)
	return objectKey
}

// Upload 上传文件到腾讯云COS
func (p *TencentCOSProvider) Upload(ctx context.Context, file io.Reader, policy *model.StoragePolicy, virtualPath string) (*UploadResult, error) {
	log.Printf("[腾讯云COS] 开始上传文件: virtualPath=%s", virtualPath)

	client, err := p.getCOSClient(policy)
	if err != nil {
		log.Printf("[腾讯云COS] 创建客户端失败: %v", err)
		return nil, err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if objectKey == "" {
		objectKey = filepath.Base(virtualPath)
		log.Printf("[腾讯云COS] objectKey为空，使用文件名: %s", objectKey)
	}

	log.Printf("[腾讯云COS] 上传对象: objectKey=%s", objectKey)

	// 上传文件
	_, err = client.Object.Put(ctx, objectKey, file, nil)
	if err != nil {
		log.Printf("[腾讯云COS] 上传失败: %v", err)
		return nil, fmt.Errorf("上传文件到腾讯云COS失败: %w", err)
	}

	log.Printf("[腾讯云COS] 上传成功: objectKey=%s", objectKey)

	// 获取文件信息
	resp, err := client.Object.Head(ctx, objectKey, nil)
	if err != nil {
		return nil, fmt.Errorf("获取上传后的文件信息失败: %w", err)
	}

	// 解析文件大小
	var fileSize int64 = 0
	if contentLengthStr := resp.Header.Get("Content-Length"); contentLengthStr != "" {
		if size, parseErr := strconv.ParseInt(contentLengthStr, 10, 64); parseErr == nil {
			fileSize = size
		}
	}

	// 获取MIME类型
	mimeType := resp.Header.Get("Content-Type")
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

// Get 从腾讯云COS获取文件流
func (p *TencentCOSProvider) Get(ctx context.Context, policy *model.StoragePolicy, source string) (io.ReadCloser, error) {
	client, err := p.getCOSClient(policy)
	if err != nil {
		return nil, err
	}

	resp, err := client.Object.Get(ctx, source, nil)
	if err != nil {
		return nil, fmt.Errorf("从腾讯云COS获取文件失败: %w", err)
	}

	return resp.Body, nil
}

// List 列出腾讯云COS存储桶中的对象
func (p *TencentCOSProvider) List(ctx context.Context, policy *model.StoragePolicy, virtualPath string) ([]FileInfo, error) {
	log.Printf("[腾讯云COS] List方法调用 - 策略名称: %s, virtualPath: %s", policy.Name, virtualPath)

	client, err := p.getCOSClient(policy)
	if err != nil {
		return nil, err
	}

	prefix := p.buildObjectKey(policy, virtualPath)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	opt := &cos.BucketGetOptions{
		Prefix:    prefix,
		Delimiter: "/", // 只列出直接子对象，不递归
	}

	result, _, err := client.Bucket.Get(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("列出腾讯云COS对象失败: %w", err)
	}

	var fileInfos []FileInfo

	// 处理"目录"（CommonPrefixes）
	for _, commonPrefix := range result.CommonPrefixes {
		name := strings.TrimSuffix(strings.TrimPrefix(commonPrefix, prefix), "/")
		if name != "" {
			fileInfos = append(fileInfos, FileInfo{
				Name:  name,
				Size:  0,
				IsDir: true,
			})
		}
	}

	// 处理文件
	for _, content := range result.Contents {
		// 跳过"目录"对象（以/结尾的对象）
		if strings.HasSuffix(content.Key, "/") {
			continue
		}

		name := strings.TrimPrefix(content.Key, prefix)
		if name != "" && !strings.Contains(name, "/") {
			// 解析最后修改时间
			var modTime time.Time
			if content.LastModified != "" {
				if t, parseErr := time.Parse("2006-01-02T15:04:05.000Z", content.LastModified); parseErr == nil {
					modTime = t
				}
			}

			fileInfos = append(fileInfos, FileInfo{
				Name:    name,
				Size:    int64(content.Size),
				IsDir:   false,
				ModTime: modTime,
			})
		}
	}

	return fileInfos, nil
}

// Delete 删除腾讯云COS中的文件
func (p *TencentCOSProvider) Delete(ctx context.Context, policy *model.StoragePolicy, sources []string) error {
	if len(sources) == 0 {
		return nil
	}

	client, err := p.getCOSClient(policy)
	if err != nil {
		return err
	}

	log.Printf("[腾讯云COS] Delete方法调用 - 策略: %s, 删除文件数量: %d", policy.Name, len(sources))

	for _, source := range sources {
		objectKey := p.buildObjectKey(policy, source)
		if objectKey == "" {
			objectKey = filepath.Base(source)
		}

		log.Printf("[腾讯云COS] 删除对象: %s", objectKey)
		_, err := client.Object.Delete(ctx, objectKey)
		if err != nil {
			log.Printf("[腾讯云COS] 删除对象失败: %s, 错误: %v", objectKey, err)
			return fmt.Errorf("删除腾讯云COS对象 %s 失败: %w", source, err)
		}
		log.Printf("[腾讯云COS] 成功删除对象: %s", objectKey)
	}

	return nil
}

// DeleteWithPolicy 使用策略信息删除文件（扩展方法）
func (p *TencentCOSProvider) DeleteWithPolicy(ctx context.Context, policy *model.StoragePolicy, sources []string) error {
	if len(sources) == 0 {
		return nil
	}

	client, err := p.getCOSClient(policy)
	if err != nil {
		return err
	}

	for _, source := range sources {
		objectKey := p.buildObjectKey(policy, source)
		if objectKey == "" {
			objectKey = filepath.Base(source)
		}
		_, err := client.Object.Delete(ctx, objectKey)
		if err != nil {
			return fmt.Errorf("删除腾讯云COS对象 %s 失败: %w", source, err)
		}
	}

	return nil
}

// CreateDirectory 在腾讯云COS中创建"目录"
func (p *TencentCOSProvider) CreateDirectory(ctx context.Context, policy *model.StoragePolicy, virtualPath string) error {
	client, err := p.getCOSClient(policy)
	if err != nil {
		return err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if !strings.HasSuffix(objectKey, "/") {
		objectKey += "/"
	}

	// 上传一个空对象来表示目录
	_, err = client.Object.Put(ctx, objectKey, strings.NewReader(""), nil)
	if err != nil {
		return fmt.Errorf("在腾讯云COS中创建目录失败: %w", err)
	}

	return nil
}

// DeleteDirectory 删除腾讯云COS中的"目录"
func (p *TencentCOSProvider) DeleteDirectory(ctx context.Context, policy *model.StoragePolicy, virtualPath string) error {
	client, err := p.getCOSClient(policy)
	if err != nil {
		return err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if !strings.HasSuffix(objectKey, "/") {
		objectKey += "/"
	}

	_, err = client.Object.Delete(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("删除腾讯云COS目录失败: %w", err)
	}

	return nil
}

// GetDownloadURL 根据存储策略权限设置生成腾讯云COS下载URL
func (p *TencentCOSProvider) GetDownloadURL(ctx context.Context, policy *model.StoragePolicy, source string, options DownloadURLOptions) (string, error) {
	log.Printf("[腾讯云COS] GetDownloadURL调用 - source: %s, policy.Server: %s, policy.IsPrivate: %v", source, policy.Server, policy.IsPrivate)

	// 检查访问域名配置
	if policy.Server == "" {
		log.Printf("[腾讯云COS] 错误: 访问域名配置为空")
		return "", fmt.Errorf("腾讯云COS策略缺少访问域名配置")
	}

	// 将虚拟路径转换为对象存储路径
	objectKey := p.buildObjectKey(policy, source)
	if objectKey == "" {
		objectKey = filepath.Base(source)
		log.Printf("[腾讯云COS] objectKey为空，使用文件名: %s", objectKey)
	}
	log.Printf("[腾讯云COS] 转换路径 - source: %s -> objectKey: %s", source, objectKey)

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

	log.Printf("[腾讯云COS] 配置信息 - cdnDomain: %s, sourceAuth: %v", cdnDomain, sourceAuth)

	// 根据是否为私有存储策略决定URL类型
	if policy.IsPrivate && !sourceAuth {
		log.Printf("[腾讯云COS] 生成预签名URL (私有策略)")

		// 私有存储策略且未开启CDN回源鉴权：生成预签名URL
		client, err := p.getCOSClient(policy)
		if err != nil {
			log.Printf("[腾讯云COS] 创建客户端失败: %v", err)
			return "", err
		}

		// 设置过期时间，默认1小时
		expiresIn := options.ExpiresIn
		if expiresIn <= 0 {
			expiresIn = 3600 // 1小时
		}

		// 从策略中获取密钥信息用于签名
		secretID := policy.AccessKey
		secretKey := policy.SecretKey

		presignedURL, err := client.Object.GetPresignedURL(ctx, http.MethodGet, objectKey,
			secretID, secretKey, time.Duration(expiresIn)*time.Second, nil)
		if err != nil {
			log.Printf("[腾讯云COS] 生成预签名URL失败: %v", err)
			return "", fmt.Errorf("生成腾讯云COS预签名URL失败: %w", err)
		}

		finalURL := presignedURL.String()
		log.Printf("[腾讯云COS] 生成的预签名URL: %s", finalURL)
		return finalURL, nil
	} else {
		log.Printf("[腾讯云COS] 生成公开URL (公开策略或CDN回源鉴权)")

		// 公开存储策略或开启CDN回源鉴权：使用配置的域名
		var finalURL string
		if cdnDomain != "" {
			// 如果配置了CDN域名，使用CDN域名替换协议和主机名
			finalURL = fmt.Sprintf("%s/%s", cdnDomain, objectKey)
			log.Printf("[腾讯云COS] 使用CDN域名生成URL: %s", finalURL)
		} else {
			// 否则使用原始Server域名
			baseURL := strings.TrimSuffix(policy.Server, "/")
			finalURL = fmt.Sprintf("%s/%s", baseURL, objectKey)
			log.Printf("[腾讯云COS] 使用Server域名生成URL: %s", finalURL)
		}

		return finalURL, nil
	}
}

// Rename 重命名或移动腾讯云COS中的对象
func (p *TencentCOSProvider) Rename(ctx context.Context, policy *model.StoragePolicy, oldVirtualPath, newVirtualPath string) error {
	client, err := p.getCOSClient(policy)
	if err != nil {
		return err
	}

	oldObjectKey := p.buildObjectKey(policy, oldVirtualPath)
	newObjectKey := p.buildObjectKey(policy, newVirtualPath)

	// 腾讯云COS不支持直接重命名，需要先复制后删除
	// 从Server URL提取域名部分构建源URL
	serverURL := strings.TrimSuffix(policy.Server, "/")
	// 移除协议部分，只保留域名部分用于Copy操作
	serverURL = strings.TrimPrefix(serverURL, "https://")
	serverURL = strings.TrimPrefix(serverURL, "http://")
	sourceURL := fmt.Sprintf("%s/%s", serverURL, oldObjectKey)

	_, _, err = client.Object.Copy(ctx, newObjectKey, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("复制腾讯云COS对象失败: %w", err)
	}

	// 删除原对象
	_, err = client.Object.Delete(ctx, oldObjectKey)
	if err != nil {
		return fmt.Errorf("删除原腾讯云COS对象失败: %w", err)
	}

	return nil
}

// Stream 将腾讯云COS文件内容流式传输到写入器
func (p *TencentCOSProvider) Stream(ctx context.Context, policy *model.StoragePolicy, source string, writer io.Writer) error {
	// 生成合适的下载URL（根据权限设置）
	downloadURL, err := p.GetDownloadURL(ctx, policy, source, DownloadURLOptions{ExpiresIn: 3600})
	if err != nil {
		return err
	}

	if w, ok := writer.(http.ResponseWriter); ok {
		w.Header().Set("Location", downloadURL)
		w.WriteHeader(http.StatusFound)
		return nil
	}

	return fmt.Errorf("腾讯云COS流式传输需要http.ResponseWriter来执行重定向")
}

// IsExist 检查腾讯云COS中的对象是否存在
func (p *TencentCOSProvider) IsExist(ctx context.Context, policy *model.StoragePolicy, source string) (bool, error) {
	client, err := p.getCOSClient(policy)
	if err != nil {
		return false, err
	}

	objectKey := p.buildObjectKey(policy, source)
	if objectKey == "" {
		objectKey = filepath.Base(source)
	}

	_, err = client.Object.Head(ctx, objectKey, nil)
	if err != nil {
		// 如果是404错误，表示对象不存在
		if cosErr, ok := err.(*cos.ErrorResponse); ok && cosErr.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// IsExistWithPolicy 使用策略信息检查对象是否存在（扩展方法）
func (p *TencentCOSProvider) IsExistWithPolicy(ctx context.Context, policy *model.StoragePolicy, source string) (bool, error) {
	client, err := p.getCOSClient(policy)
	if err != nil {
		return false, err
	}

	objectKey := p.buildObjectKey(policy, source)
	if objectKey == "" {
		objectKey = filepath.Base(source)
	}

	_, err = client.Object.Head(ctx, objectKey, nil)
	if err != nil {
		// 如果是404错误，表示对象不存在
		if cosErr, ok := err.(*cos.ErrorResponse); ok && cosErr.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// GetThumbnail 腾讯云COS的数据万象服务支持实时图片处理，可以生成缩略图
// 如果用户在腾讯云控制台开通了数据万象服务，此功能将自动可用
func (p *TencentCOSProvider) GetThumbnail(ctx context.Context, policy *model.StoragePolicy, source string, size string) (*ThumbnailResult, error) {

	// 解析尺寸参数
	width, height := parseThumbnailSize(size)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("无效的缩略图尺寸: %s", size)
	}

	// 将虚拟路径转换为对象存储路径
	objectKey := p.buildObjectKey(policy, source)
	if objectKey == "" {
		objectKey = filepath.Base(source)
	}

	// 构建数据万象处理URL
	serverURL := strings.TrimSuffix(policy.Server, "/")
	thumbnailURL := fmt.Sprintf("%s/%s?imageMogr2/thumbnail/%dx%d",
		serverURL, objectKey, width, height)

	// 获取缩略图数据（这里简化为返回URL，实际使用中可能需要下载数据）
	// 由于ThumbnailResult需要Data字段，我们需要下载缩略图数据
	resp, err := http.Get(thumbnailURL)
	if err != nil {
		return nil, fmt.Errorf("获取缩略图数据失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取缩略图数据失败: %w", err)
	}

	return &ThumbnailResult{
		ContentType: "image/jpeg", // 数据万象默认输出JPEG
		Data:        data,
	}, nil
}

// parseThumbnailSize 解析缩略图尺寸字符串（如"300x200"）
func parseThumbnailSize(size string) (int, int) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0
	}

	width, err1 := strconv.Atoi(parts[0])
	height, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0
	}

	return width, height
}

// SetupCORS 为腾讯云COS存储桶配置跨域策略
// 配置允许所有来源访问，支持GET, POST, PUT, DELETE, HEAD方法
func (p *TencentCOSProvider) SetupCORS(ctx context.Context, policy *model.StoragePolicy) error {
	client, err := p.getCOSClient(policy)
	if err != nil {
		return fmt.Errorf("创建腾讯云COS客户端失败: %w", err)
	}

	// 定义CORS规则
	corsRule := &cos.BucketCORSRule{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "HEAD"},
		AllowedHeaders: []string{"*"},
		ExposeHeaders:  []string{"ETag"},
		MaxAgeSeconds:  3600,
	}

	corsConfig := &cos.BucketPutCORSOptions{
		Rules: []cos.BucketCORSRule{*corsRule},
	}

	_, err = client.Bucket.PutCORS(ctx, corsConfig)
	if err != nil {
		return fmt.Errorf("配置腾讯云COS跨域策略失败: %w", err)
	}

	return nil
}

// GetCORSConfig 获取腾讯云COS存储桶的跨域配置
func (p *TencentCOSProvider) GetCORSConfig(ctx context.Context, policy *model.StoragePolicy) (*cos.BucketGetCORSResult, error) {
	client, err := p.getCOSClient(policy)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云COS客户端失败: %w", err)
	}

	result, _, err := client.Bucket.GetCORS(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取腾讯云COS跨域配置失败: %w", err)
	}

	return result, nil
}
