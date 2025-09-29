/*
 * @Description: AWS S3存储提供者实现（使用aws-sdk-go-v2）
 * @Author: 安知鱼
 * @Date: 2025-09-28 19:00:00
 * @LastEditTime: 2025-09-28 19:30:00
 * @LastEditors: 安知鱼
 */
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// AWSS3Provider 实现了 IStorageProvider 接口，用于处理与AWS S3的所有交互。
type AWSS3Provider struct {
}

// NewAWSS3Provider 是 AWSS3Provider 的构造函数。
func NewAWSS3Provider() IStorageProvider {
	return &AWSS3Provider{}
}

// getS3Client 获取AWS S3客户端（使用aws-sdk-go-v2）
func (p *AWSS3Provider) getS3Client(ctx context.Context, policy *model.StoragePolicy) (*s3.Client, error) {
	// 添加调试日志，打印策略的关键信息
	log.Printf("[AWS S3] 创建客户端 - 策略名称: %s, 策略ID: %d, Server: %s", policy.Name, policy.ID, policy.Server)

	// 从策略中获取配置信息
	bucketName := policy.BucketName
	if bucketName == "" {
		log.Printf("[AWS S3] 错误: 存储桶名称为空")
		return nil, fmt.Errorf("AWS S3策略缺少存储桶名称")
	}

	accessKeyID := policy.AccessKey
	if accessKeyID == "" {
		return nil, fmt.Errorf("AWS S3策略缺少AccessKey")
	}

	secretAccessKey := policy.SecretKey
	if secretAccessKey == "" {
		return nil, fmt.Errorf("AWS S3策略缺少SecretKey")
	}

	// 从Server字段获取区域和endpoint
	// Server格式可能是: "us-west-2" 或 "https://s3.us-west-2.amazonaws.com" 或自定义endpoint
	region := "us-east-1" // 默认区域
	var customEndpoint *string

	if policy.Server != "" {
		if strings.HasPrefix(policy.Server, "http") {
			// 如果是完整的URL，提取区域和endpoint
			parsedURL, err := url.Parse(policy.Server)
			if err == nil {
				customEndpoint = &policy.Server
				// 尝试从URL中提取区域信息
				if strings.Contains(parsedURL.Host, "amazonaws.com") {
					parts := strings.Split(parsedURL.Host, ".")
					if len(parts) >= 3 && strings.HasPrefix(parts[1], "s3") {
						if len(parts) >= 4 {
							region = parts[2] // s3.us-west-2.amazonaws.com
						}
					}
				}
			}
		} else {
			// 假设是区域名称
			region = policy.Server
		}
	}

	// 创建AWS配置选项
	var opts []func(*config.LoadOptions) error

	// 设置区域
	opts = append(opts, config.WithRegion(region))

	// 设置凭证
	opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
		accessKeyID,
		secretAccessKey,
		"",
	)))

	// 如果有自定义endpoint，会在创建客户端时设置

	// 加载配置
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		log.Printf("[AWS S3] 创建配置失败: %v", err)
		return nil, fmt.Errorf("创建AWS S3配置失败: %w", err)
	}

	// 创建S3客户端
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if customEndpoint != nil {
			o.BaseEndpoint = aws.String(*customEndpoint)
			o.UsePathStyle = true // 对于自定义endpoint通常需要path-style
		}
	})

	log.Printf("[AWS S3] 成功创建客户端 - 区域: %s", region)
	return client, nil
}

// buildObjectKey 构建S3对象键
func (p *AWSS3Provider) buildObjectKey(policy *model.StoragePolicy, virtualPath string) string {
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

	// 确保不以斜杠开头（S3对象键不应该以/开头）
	objectKey = strings.TrimPrefix(objectKey, "/")

	log.Printf("[AWS S3] 路径转换 - basePath: %s, virtualPath: %s -> objectKey: %s", basePath, virtualPath, objectKey)
	return objectKey
}

// Upload 上传文件到AWS S3
func (p *AWSS3Provider) Upload(ctx context.Context, file io.Reader, policy *model.StoragePolicy, virtualPath string) (*UploadResult, error) {
	log.Printf("[AWS S3] 开始上传文件: virtualPath=%s", virtualPath)

	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		log.Printf("[AWS S3] 创建客户端失败: %v", err)
		return nil, err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if objectKey == "" {
		objectKey = filepath.Base(virtualPath)
		log.Printf("[AWS S3] objectKey为空，使用文件名: %s", objectKey)
	}

	log.Printf("[AWS S3] 上传对象: objectKey=%s", objectKey)

	// 上传文件
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(objectKey),
		Body:   file,
	})
	if err != nil {
		log.Printf("[AWS S3] 上传失败: %v", err)
		return nil, fmt.Errorf("上传文件到AWS S3失败: %w", err)
	}

	log.Printf("[AWS S3] 上传成功: objectKey=%s", objectKey)

	// 获取文件信息
	headOutput, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("获取上传后的文件信息失败: %w", err)
	}

	// 解析文件大小
	var fileSize int64 = 0
	if headOutput.ContentLength != nil {
		fileSize = *headOutput.ContentLength
	}

	// 获取MIME类型
	mimeType := "application/octet-stream"
	if headOutput.ContentType != nil {
		mimeType = *headOutput.ContentType
	} else {
		detectedMimeType := mime.TypeByExtension(filepath.Ext(virtualPath))
		if detectedMimeType != "" {
			mimeType = detectedMimeType
		}
	}

	return &UploadResult{
		Source:   objectKey, // 返回对象键作为source
		Size:     fileSize,
		MimeType: mimeType,
	}, nil
}

// Get 从AWS S3获取文件流
func (p *AWSS3Provider) Get(ctx context.Context, policy *model.StoragePolicy, source string) (io.ReadCloser, error) {
	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return nil, err
	}

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(source),
	})
	if err != nil {
		return nil, fmt.Errorf("从AWS S3获取文件失败: %w", err)
	}

	return output.Body, nil
}

// List 列出AWS S3存储桶中的对象
func (p *AWSS3Provider) List(ctx context.Context, policy *model.StoragePolicy, virtualPath string) ([]FileInfo, error) {
	log.Printf("[AWS S3] List方法调用 - 策略名称: %s, virtualPath: %s", policy.Name, virtualPath)

	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return nil, err
	}

	prefix := p.buildObjectKey(policy, virtualPath)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	output, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(policy.BucketName),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return nil, fmt.Errorf("列出AWS S3对象失败: %w", err)
	}

	var fileInfos []FileInfo

	// 处理文件对象
	for _, obj := range output.Contents {
		if obj.Key == nil {
			continue
		}

		// 跳过目录本身
		if strings.HasSuffix(*obj.Key, "/") {
			continue
		}

		// 移除前缀，获取相对路径
		name := strings.TrimPrefix(*obj.Key, prefix)
		if name == "" {
			continue
		}

		// 只显示直接子文件，不显示子目录中的文件
		if strings.Contains(name, "/") {
			continue
		}

		var fileSize int64 = 0
		if obj.Size != nil {
			fileSize = *obj.Size
		}

		var modTime time.Time
		if obj.LastModified != nil {
			modTime = *obj.LastModified
		}

		fileInfo := FileInfo{
			Name:    name,
			Size:    fileSize,
			ModTime: modTime,
			IsDir:   false,
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	// 处理公共前缀（目录）
	for _, commonPrefix := range output.CommonPrefixes {
		if commonPrefix.Prefix == nil {
			continue
		}

		// 移除前缀和尾随的斜杠，获取目录名
		dirName := strings.TrimSuffix(strings.TrimPrefix(*commonPrefix.Prefix, prefix), "/")
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

	log.Printf("[AWS S3] List完成 - 返回 %d 个项目", len(fileInfos))
	return fileInfos, nil
}

// Delete 从AWS S3删除多个文件
func (p *AWSS3Provider) Delete(ctx context.Context, policy *model.StoragePolicy, sources []string) error {
	if len(sources) == 0 {
		return nil
	}

	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return err
	}

	log.Printf("[AWS S3] Delete方法调用 - 策略: %s, 删除文件数量: %d", policy.Name, len(sources))

	for _, source := range sources {
		objectKey := p.buildObjectKey(policy, source)
		if objectKey == "" {
			objectKey = filepath.Base(source)
		}

		log.Printf("[AWS S3] 删除对象: %s", objectKey)
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(policy.BucketName),
			Key:    aws.String(objectKey),
		})
		if err != nil {
			log.Printf("[AWS S3] 删除对象失败: %s, 错误: %v", objectKey, err)
			return fmt.Errorf("删除AWS S3对象 %s 失败: %w", source, err)
		}
		log.Printf("[AWS S3] 成功删除对象: %s", objectKey)
	}

	return nil
}

// DeleteSingle 从AWS S3删除单个文件（内部使用）
func (p *AWSS3Provider) DeleteSingle(ctx context.Context, policy *model.StoragePolicy, source string) error {
	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return err
	}

	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(source),
	})
	if err != nil {
		return fmt.Errorf("从AWS S3删除文件失败: %w", err)
	}

	return nil
}

// Stream 从AWS S3流式传输文件到writer
func (p *AWSS3Provider) Stream(ctx context.Context, policy *model.StoragePolicy, source string, writer io.Writer) error {
	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return err
	}

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(source),
	})
	if err != nil {
		return fmt.Errorf("从AWS S3获取文件失败: %w", err)
	}
	defer output.Body.Close()

	_, err = io.Copy(writer, output.Body)
	if err != nil {
		return fmt.Errorf("流式传输文件失败: %w", err)
	}

	return nil
}

// GetDownloadURL 根据存储策略权限设置生成AWS S3下载URL
func (p *AWSS3Provider) GetDownloadURL(ctx context.Context, policy *model.StoragePolicy, source string, options DownloadURLOptions) (string, error) {
	log.Printf("[AWS S3] GetDownloadURL调用 - source: %s, policy.Server: %s, policy.IsPrivate: %v", source, policy.Server, policy.IsPrivate)

	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		log.Printf("[AWS S3] 创建客户端失败: %v", err)
		return "", err
	}

	// 将虚拟路径转换为对象存储路径
	objectKey := p.buildObjectKey(policy, source)
	if objectKey == "" {
		objectKey = filepath.Base(source)
		log.Printf("[AWS S3] objectKey为空，使用文件名: %s", objectKey)
	}
	log.Printf("[AWS S3] 转换路径 - source: %s -> objectKey: %s", source, objectKey)

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

	log.Printf("[AWS S3] 配置信息 - cdnDomain: %s, sourceAuth: %v", cdnDomain, sourceAuth)

	// 根据是否为私有存储策略决定URL类型
	if policy.IsPrivate && !sourceAuth {
		log.Printf("[AWS S3] 生成预签名URL (私有策略)")

		// 设置过期时间，默认1小时
		expiresIn := time.Duration(options.ExpiresIn) * time.Second
		if expiresIn <= 0 {
			expiresIn = time.Hour // 1小时
		}

		// 使用aws-sdk-go-v2的预签名客户端
		presignClient := s3.NewPresignClient(client)

		presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(policy.BucketName),
			Key:    aws.String(objectKey),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = expiresIn
		})
		if err != nil {
			log.Printf("[AWS S3] 生成预签名URL失败: %v", err)
			return "", fmt.Errorf("生成AWS S3预签名URL失败: %w", err)
		}

		log.Printf("[AWS S3] 预签名URL生成成功")
		return presignResult.URL, nil
	} else {
		log.Printf("[AWS S3] 生成公共访问URL")

		// 公共访问策略或开启了CDN回源鉴权：生成公共访问URL
		var baseURL string
		if cdnDomain != "" {
			// 使用CDN域名
			baseURL = cdnDomain
			if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
				baseURL = "https://" + baseURL
			}
		} else {
			// 使用S3直接访问域名
			if policy.Server != "" && strings.HasPrefix(policy.Server, "http") {
				// 如果Server是完整的endpoint URL
				baseURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(policy.Server, "/"), policy.BucketName)
			} else {
				// 构建标准的S3 URL
				region := policy.Server
				if region == "" {
					region = "us-east-1"
				}
				baseURL = fmt.Sprintf("https://s3.%s.amazonaws.com/%s", region, policy.BucketName)
			}
		}

		// 构建完整的访问URL
		fullURL := fmt.Sprintf("%s/%s", baseURL, objectKey)

		log.Printf("[AWS S3] 公共访问URL: %s", fullURL)
		return fullURL, nil
	}
}

// CreateDirectory 在AWS S3中创建目录（通过创建空对象模拟）
func (p *AWSS3Provider) CreateDirectory(ctx context.Context, policy *model.StoragePolicy, virtualPath string) error {
	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if !strings.HasSuffix(objectKey, "/") {
		objectKey += "/"
	}

	// S3通过创建一个以"/"结尾的空对象来模拟目录
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(objectKey),
		Body:   strings.NewReader(""),
	})
	if err != nil {
		return fmt.Errorf("在AWS S3中创建目录失败: %w", err)
	}

	return nil
}

// DeleteDirectory 删除AWS S3中的空目录
func (p *AWSS3Provider) DeleteDirectory(ctx context.Context, policy *model.StoragePolicy, virtualPath string) error {
	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return err
	}

	objectKey := p.buildObjectKey(policy, virtualPath)
	if !strings.HasSuffix(objectKey, "/") {
		objectKey += "/"
	}

	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("删除AWS S3目录失败: %w", err)
	}

	return nil
}

// Rename 重命名或移动AWS S3中的文件或目录
func (p *AWSS3Provider) Rename(ctx context.Context, policy *model.StoragePolicy, oldVirtualPath, newVirtualPath string) error {
	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return err
	}

	oldObjectKey := p.buildObjectKey(policy, oldVirtualPath)
	newObjectKey := p.buildObjectKey(policy, newVirtualPath)

	// 复制对象到新位置
	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(policy.BucketName),
		Key:        aws.String(newObjectKey),
		CopySource: aws.String(fmt.Sprintf("%s/%s", policy.BucketName, oldObjectKey)),
	})
	if err != nil {
		return fmt.Errorf("复制AWS S3对象失败: %w", err)
	}

	// 删除原对象
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(oldObjectKey),
	})
	if err != nil {
		return fmt.Errorf("删除原AWS S3对象失败: %w", err)
	}

	return nil
}

// IsExist 检查文件是否存在于AWS S3中
func (p *AWSS3Provider) IsExist(ctx context.Context, policy *model.StoragePolicy, source string) (bool, error) {
	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return false, err
	}

	objectKey := p.buildObjectKey(policy, source)
	if objectKey == "" {
		objectKey = filepath.Base(source)
	}

	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// GetThumbnail 获取缩略图（AWS S3不直接支持）
func (p *AWSS3Provider) GetThumbnail(ctx context.Context, policy *model.StoragePolicy, source string, size string) (*ThumbnailResult, error) {
	// AWS S3本身不提供缩略图生成服务，返回不支持
	return nil, ErrFeatureNotSupported
}

// Exists 检查文件是否存在于AWS S3中（带policy参数的版本）
func (p *AWSS3Provider) Exists(ctx context.Context, policy *model.StoragePolicy, source string) (bool, error) {
	client, err := p.getS3Client(ctx, policy)
	if err != nil {
		return false, err
	}

	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(policy.BucketName),
		Key:    aws.String(source),
	})
	if err != nil {
		// 检查是否是NoSuchKey错误
		var noSuchKey *types.NoSuchKey
		var notFound *types.NotFound
		if errors.As(err, &noSuchKey) || errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("检查AWS S3文件是否存在失败: %w", err)
	}

	return true, nil
}
