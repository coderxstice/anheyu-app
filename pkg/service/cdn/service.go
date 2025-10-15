// anheyu-app/pkg/service/cdn/service.go
package cdn

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

// CDNService CDN缓存管理服务接口
type CDNService interface {
	// PurgeCache 清除指定URL的CDN缓存
	PurgeCache(ctx context.Context, urls []string) error
	// PurgeByTags 根据缓存标签清除CDN缓存
	PurgeByTags(ctx context.Context, tags []string) error
	// PurgeArticleCache 清除文章相关的CDN缓存
	PurgeArticleCache(ctx context.Context, articleID string) error
}

type serviceImpl struct {
	settingSvc setting.SettingService
	httpClient *http.Client
}

// NewService 创建CDN服务实例
func NewService(settingSvc setting.SettingService) CDNService {
	return &serviceImpl{
		settingSvc: settingSvc,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// getConfig 从配置系统中获取CDN配置
func (s *serviceImpl) getConfig() (enabled bool, provider, secretID, secretKey, region, domain, zoneID string) {
	enabled = s.settingSvc.Get(constant.KeyCDNEnable.String()) == "true"
	provider = s.settingSvc.Get(constant.KeyCDNProvider.String())
	secretID = s.settingSvc.Get(constant.KeyCDNSecretID.String())
	secretKey = s.settingSvc.Get(constant.KeyCDNSecretKey.String())
	region = s.settingSvc.Get(constant.KeyCDNRegion.String())
	domain = s.settingSvc.Get(constant.KeyCDNDomain.String())
	zoneID = s.settingSvc.Get(constant.KeyCDNZoneID.String())

	// 设置默认地域
	if region == "" {
		if provider == "edgeone" {
			region = "ap-singapore"
		} else {
			region = "ap-beijing"
		}
	}

	return
}

// PurgeCache 清除指定URL的CDN缓存
func (s *serviceImpl) PurgeCache(ctx context.Context, urls []string) error {
	enabled, provider, _, _, _, _, _ := s.getConfig()
	if !enabled {
		log.Printf("[CDN] 缓存清除功能未启用，跳过清除操作")
		return nil
	}

	if len(urls) == 0 {
		return nil
	}

	log.Printf("[CDN] 开始清除缓存，URL数量: %d", len(urls))

	switch strings.ToLower(provider) {
	case "tencent":
		return s.purgeTencentCache(ctx, urls)
	case "edgeone":
		return s.purgeEdgeOneCache(ctx, urls)
	default:
		log.Printf("[CDN] 不支持的CDN提供商: %s", provider)
		return nil
	}
}

// PurgeByTags 根据缓存标签清除CDN缓存
func (s *serviceImpl) PurgeByTags(ctx context.Context, tags []string) error {
	enabled, provider, _, _, _, _, _ := s.getConfig()
	if !enabled {
		log.Printf("[CDN] 缓存清除功能未启用，跳过标签清除操作")
		return nil
	}

	if len(tags) == 0 {
		return nil
	}

	log.Printf("[CDN] 开始根据标签清除缓存，标签: %v", tags)

	switch strings.ToLower(provider) {
	case "edgeone":
		return s.purgeEdgeOneByTags(ctx, tags)
	default:
		log.Printf("[CDN] 提供商 %s 不支持按标签清除缓存", provider)
		return nil
	}
}

// PurgeArticleCache 清除文章相关的CDN缓存
func (s *serviceImpl) PurgeArticleCache(ctx context.Context, articleID string) error {
	enabled, _, _, _, _, _, _ := s.getConfig()
	if !enabled {
		return nil
	}

	// 获取网站基础URL
	baseURL := s.settingSvc.Get(constant.KeySiteURL.String())
	if baseURL == "" {
		baseURL = "https://yourdomain.com" // 默认值
	}

	// 只清除文章详情页（因为只有文章详情页做了SSR水合）
	// 首页、列表页等都是客户端渲染，数据通过API实时获取，不需要清除CDN
	urls := []string{
		fmt.Sprintf("%s/posts/%s", baseURL, articleID), // 文章详情页
	}

	// 同时使用标签清除
	tags := []string{
		fmt.Sprintf("article-%s", articleID),
		"article-detail",
		"home-page",
		"article-list",
	}

	// 并行执行URL清除和标签清除
	errChan := make(chan error, 2)

	go func() {
		errChan <- s.PurgeCache(ctx, urls)
	}()

	go func() {
		errChan <- s.PurgeByTags(ctx, tags)
	}()

	// 等待两个操作完成
	var errors []error
	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("CDN缓存清除部分失败: %v", errors)
	}

	log.Printf("[CDN] 文章 %s 的缓存清除完成", articleID)
	return nil
}

// purgeTencentCache 清除腾讯云CDN缓存
func (s *serviceImpl) purgeTencentCache(ctx context.Context, urls []string) error {
	_, _, secretID, secretKey, region, domain, _ := s.getConfig()
	if secretID == "" || secretKey == "" || domain == "" {
		log.Printf("[CDN] 腾讯云CDN配置不完整，跳过缓存清除")
		return nil
	}

	// 腾讯云CDN API地址
	host := "cdn.tencentcloudapi.com"
	service := "cdn"
	version := "2018-06-06"
	action := "PurgeUrlsCache"

	// 构建请求参数
	params := map[string]interface{}{
		"Urls": urls,
	}

	// 调用腾讯云API
	err := s.callTencentCloudAPI(ctx, host, service, version, action, region, secretID, secretKey, params)
	if err != nil {
		return fmt.Errorf("腾讯云CDN缓存清除失败: %w", err)
	}

	log.Printf("[CDN] 腾讯云CDN缓存清除成功: %v", urls)
	return nil
}

// purgeEdgeOneCache 清除EdgeOne缓存
func (s *serviceImpl) purgeEdgeOneCache(ctx context.Context, urls []string) error {
	_, _, secretID, secretKey, region, _, zoneID := s.getConfig()
	if secretID == "" || secretKey == "" || zoneID == "" {
		log.Printf("[CDN] EdgeOne配置不完整，跳过缓存清除")
		return nil
	}

	// EdgeOne API地址
	host := "teo.tencentcloudapi.com"
	service := "teo"
	version := "2022-09-01"
	action := "CreatePurgeTasks" // 注意：EdgeOne使用复数形式

	// 构建请求参数 - EdgeOne格式
	params := map[string]interface{}{
		"ZoneId":  zoneID,
		"Type":    "purge_url",  // 清除URL类型
		"Method":  "invalidate", // 刷新方式：invalidate（标记过期）或 delete（删除）
		"Targets": urls,
	}

	// 调用腾讯云API
	err := s.callTencentCloudAPI(ctx, host, service, version, action, region, secretID, secretKey, params)
	if err != nil {
		return fmt.Errorf("EdgeOne缓存清除失败: %w", err)
	}

	log.Printf("[CDN] EdgeOne缓存清除成功: %v", urls)
	return nil
}

// purgeEdgeOneByTags 根据标签清除EdgeOne缓存
func (s *serviceImpl) purgeEdgeOneByTags(ctx context.Context, tags []string) error {
	_, _, secretID, secretKey, region, _, zoneID := s.getConfig()
	if secretID == "" || secretKey == "" || zoneID == "" {
		log.Printf("[CDN] EdgeOne配置不完整，跳过标签缓存清除")
		return nil
	}

	// EdgeOne API地址
	host := "teo.tencentcloudapi.com"
	service := "teo"
	version := "2022-09-01"
	action := "CreatePurgeTasks" // 注意：EdgeOne使用复数形式

	// 构建请求参数 - EdgeOne按标签清除
	params := map[string]interface{}{
		"ZoneId":  zoneID,
		"Type":    "purge_cache_tag", // 按缓存标签清除
		"Method":  "invalidate",
		"Targets": tags,
	}

	// 调用腾讯云API
	err := s.callTencentCloudAPI(ctx, host, service, version, action, region, secretID, secretKey, params)
	if err != nil {
		return fmt.Errorf("EdgeOne按标签清除缓存失败: %w", err)
	}

	log.Printf("[CDN] EdgeOne按标签清除缓存成功: %v", tags)
	return nil
}

// callTencentCloudAPI 调用腾讯云API的通用方法
func (s *serviceImpl) callTencentCloudAPI(ctx context.Context, host, service, version, action, region, secretID, secretKey string, params map[string]interface{}) error {
	// 构建请求体
	jsonData, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("构建请求体失败: %w", err)
	}

	// 生成签名
	timestamp := time.Now().Unix()
	authorization, err := s.generateTencentCloudSignature(host, service, version, action, region, string(jsonData), timestamp, secretID, secretKey)
	if err != nil {
		return fmt.Errorf("生成签名失败: %w", err)
	}

	// 创建HTTP请求
	apiURL := fmt.Sprintf("https://%s", host)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Region", region)

	// 发送请求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("腾讯云API返回错误: %d, %s", resp.StatusCode, string(body))
	}

	// 解析响应，检查是否有错误
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if errorInfo, exists := response["Error"]; exists {
		return fmt.Errorf("腾讯云API业务错误: %v", errorInfo)
	}

	return nil
}

// generateTencentCloudSignature 生成腾讯云API签名
func (s *serviceImpl) generateTencentCloudSignature(host, service, version, action, region, payload string, timestamp int64, secretID, secretKey string) (string, error) {
	algorithm := "TC3-HMAC-SHA256"

	// 步骤1：拼接规范请求串
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json; charset=utf-8\nhost:%s\nx-tc-action:%s\n",
		strings.ToLower(host), strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"
	hashedRequestPayload := s.sha256hex(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod, canonicalURI, canonicalQueryString, canonicalHeaders, signedHeaders, hashedRequestPayload)

	// 步骤2：拼接待签名字符串
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := s.sha256hex(canonicalRequest)
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s", algorithm, timestamp, credentialScope, hashedCanonicalRequest)

	// 步骤3：计算签名
	secretDate := s.hmacSha256([]byte("TC3"+secretKey), date)
	secretService := s.hmacSha256(secretDate, service)
	secretSigning := s.hmacSha256(secretService, "tc3_request")
	signature := hex.EncodeToString(s.hmacSha256(secretSigning, stringToSign))

	// 步骤4：拼接Authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, secretID, credentialScope, signedHeaders, signature)

	return authorization, nil
}

// sha256hex 计算SHA256哈希值并返回十六进制字符串
func (s *serviceImpl) sha256hex(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// hmacSha256 计算HMAC-SHA256
func (s *serviceImpl) hmacSha256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
