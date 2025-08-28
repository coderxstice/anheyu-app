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
	apiURL := s.settingSvc.Get(constant.KeyIPAPI.String())
	apiToken := s.settingSvc.Get(constant.KeyIPAPIToKen.String())

	// 如果 API 和 Token 未配置，则直接返回错误
	if apiURL == "" || apiToken == "" {
		return "未知", fmt.Errorf("IP 查询失败：远程 API 未配置")
	}

	location, err := s.lookupViaAPI(apiURL, apiToken, ipStr)
	if err != nil {
		// 记录错误，但返回统一的“未知”给上层调用者
		log.Printf("警告: API IP 查询失败: %v", err)
		return "未知", err
	}

	return location, nil
}

// lookupViaAPI 封装了调用远程 API 的逻辑。
func (s *smartGeoIPService) lookupViaAPI(apiURL, apiToken, ipStr string) (string, error) {
	reqURL := fmt.Sprintf("%s?key=%s&ip=%s", apiURL, apiToken, ipStr)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建 API 请求失败: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API 请求网络错误: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 返回非 200 状态码: %s", resp.Status)
	}

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 API 响应 JSON 失败: %w", err)
	}

	if result.Code != 200 {
		return "", fmt.Errorf("API 返回业务错误码: %d", result.Code)
	}

	province := result.Data.Province
	city := result.Data.City

	if province != "" && city != "" && province != city {
		return fmt.Sprintf("%s %s", province, city), nil
	}
	if city != "" {
		return city, nil
	}
	if province != "" {
		return province, nil
	}
	if result.Data.Country != "" {
		return result.Data.Country, nil
	}

	return "", fmt.Errorf("API 响应中未包含位置信息")
}

// Close 在这个实现中不需要做任何事，但为了满足接口要求而保留。
func (s *smartGeoIPService) Close() {
	// httpClient 不需要显式关闭
}
