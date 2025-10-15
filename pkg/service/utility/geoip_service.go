/*
 * @Description: IP地理位置查询服务，仅支持远程API查询。
 * @Author: 安知鱼
 * @Date: 2025-07-25 16:15:59
 * @LastEditTime: 2025-08-27 21:34:38
 * @LastEditors: 安知鱼
 */
package utility

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/setting"
)

// GeoIPService 定义了 IP 地理位置查询服务的统一接口。
type GeoIPService interface {
	Lookup(ipString string) (location string, err error)
	Close()
}

// apiResponse 定义了远程 IP API 返回的 JSON 数据的结构。
// 如果你更换API服务商，可能需要修改这个结构体以匹配新的JSON格式。
type apiResponse struct {
	Code int `json:"code"`
	Data struct {
		Country  string `json:"country"`
		Province string `json:"province"`
		City     string `json:"city"`
	} `json:"data"`
}

// smartGeoIPService 是现在唯一的服务实现，仅通过远程API查询。
type smartGeoIPService struct {
	settingSvc setting.SettingService
	httpClient *http.Client
}

// NewGeoIPService 是构造函数，注入了配置服务。
// 它不再需要数据库路径参数。
func NewGeoIPService(settingSvc setting.SettingService) (GeoIPService, error) {
	return &smartGeoIPService{
		settingSvc: settingSvc,
		httpClient: &http.Client{
			Timeout: 5 * time.Second, // 为 API 请求设置5秒超时
		},
	}, nil
}

// Lookup 是核心的查询方法，只通过 API 进行。
func (s *smartGeoIPService) Lookup(ipStr string) (string, error) {
	log.Printf("[IP属地查询] 开始查询IP地址: %s", ipStr)

	apiURL := s.settingSvc.Get(constant.KeyIPAPI.String())
	apiToken := s.settingSvc.Get(constant.KeyIPAPIToKen.String())

	// 如果 API 和 Token 未配置，则直接返回错误
	if apiURL == "" || apiToken == "" {
		log.Printf("[IP属地查询] ❌ IP属地查询失败 - IP: %s, 原因: 远程API未配置 (apiURL: %s, apiToken配置: %t)",
			ipStr, apiURL, apiToken != "")
		return "未知", fmt.Errorf("IP 查询失败：远程 API 未配置")
	}

	log.Printf("[IP属地查询] API配置检查通过 - URL: %s, Token已配置: %t", apiURL, apiToken != "")

	location, err := s.lookupViaAPI(apiURL, apiToken, ipStr)
	if err != nil {
		// 记录错误，但返回统一的"未知"给上层调用者
		log.Printf("[IP属地查询] ❌ IP属地最终结果为'未知' - IP: %s, API调用失败: %v", ipStr, err)
		return "未知", err
	}

	log.Printf("[IP属地查询] ✅ IP属地查询成功 - IP: %s, 结果: %s", ipStr, location)
	return location, nil
}

// lookupViaAPI 封装了调用远程 API 的逻辑。
func (s *smartGeoIPService) lookupViaAPI(apiURL, apiToken, ipStr string) (string, error) {
	// 构建请求URL，但在日志中隐藏Token信息
	reqURL := fmt.Sprintf("%s?key=%s&ip=%s", apiURL, apiToken, ipStr)
	logSafeURL := fmt.Sprintf("%s?key=***&ip=%s", apiURL, ipStr)

	log.Printf("[IP属地查询] 准备调用第三方API - URL: %s", logSafeURL)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		// 避免在日志中暴露完整的URL和Token信息
		log.Printf("[IP属地查询] ❌ 创建HTTP请求失败 - IP: %s, 目标: %s", ipStr, logSafeURL)
		return "", fmt.Errorf("创建 API 请求失败: %w", err)
	}

	log.Printf("[IP属地查询] 发送HTTP请求到第三方API...")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		// 避免在日志中暴露完整的URL和Token信息
		log.Printf("[IP属地查询] ❌ HTTP请求失败 - IP: %s, 目标: %s, 错误类型: %T", ipStr, logSafeURL, err)
		return "", fmt.Errorf("API 请求网络错误: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[IP属地查询] 收到HTTP响应 - IP: %s, 状态码: %d", ipStr, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Printf("[IP属地查询] ❌ API返回非200状态码 - IP: %s, 状态: %s", ipStr, resp.Status)
		return "", fmt.Errorf("API 返回非 200 状态码: %s", resp.Status)
	}

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[IP属地查询] ❌ 解析API响应JSON失败 - IP: %s, 错误: %v", ipStr, err)
		return "", fmt.Errorf("解析 API 响应 JSON 失败: %w", err)
	}

	log.Printf("[IP属地查询] API响应解析成功 - IP: %s, 业务码: %d, 国家: %s, 省份: %s, 城市: %s",
		ipStr, result.Code, result.Data.Country, result.Data.Province, result.Data.City)

	if result.Code != 200 {
		log.Printf("[IP属地查询] ❌ API返回业务错误 - IP: %s, 错误码: %d", ipStr, result.Code)
		return "", fmt.Errorf("API 返回业务错误码: %d", result.Code)
	}

	province := result.Data.Province
	city := result.Data.City

	// 根据优先级组装位置信息
	var finalLocation string
	if province != "" && city != "" && province != city {
		finalLocation = fmt.Sprintf("%s %s", province, city)
		log.Printf("[IP属地查询] 使用省+市格式 - IP: %s, 结果: %s", ipStr, finalLocation)
	} else if city != "" {
		finalLocation = city
		log.Printf("[IP属地查询] 使用城市格式 - IP: %s, 结果: %s", ipStr, finalLocation)
	} else if province != "" {
		finalLocation = province
		log.Printf("[IP属地查询] 使用省份格式 - IP: %s, 结果: %s", ipStr, finalLocation)
	} else if result.Data.Country != "" {
		finalLocation = result.Data.Country
		log.Printf("[IP属地查询] 使用国家格式 - IP: %s, 结果: %s", ipStr, finalLocation)
	} else {
		log.Printf("[IP属地查询] ❌ API响应中无有效位置信息 - IP: %s, API返回的数据: 国家=%s, 省份=%s, 城市=%s",
			ipStr, result.Data.Country, result.Data.Province, result.Data.City)
		return "", fmt.Errorf("API 响应中未包含位置信息")
	}

	return finalLocation, nil
}

// Close 在这个实现中不需要做任何事，但为了满足接口要求而保留。
func (s *smartGeoIPService) Close() {
	// httpClient 不需要显式关闭
}
