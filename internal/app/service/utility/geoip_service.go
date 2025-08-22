/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-07-25 16:15:59
 * @LastEditTime: 2025-08-02 17:34:17
 * @LastEditors: 安知鱼
 */
package utility

import (
	"github.com/anzhiyu-c/anheyu-app/internal/app/service/setting"
	"github.com/anzhiyu-c/anheyu-app/internal/constant"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// GeoIPService 定义了 IP 地理位置查询服务的统一接口。
type GeoIPService interface {
	Lookup(ipString string) (location string, err error)
	Close()
}

// apiResponse 定义了远程 IP API 返回的 JSON 数据的结构。
type apiResponse struct {
	Code int `json:"code"`
	Data struct {
		Country  string `json:"country"`
		Province string `json:"province"`
		City     string `json:"city"`
	} `json:"data"`
}

// localGeoIPService 实现了使用本地 GeoLite2 数据库文件的查询。
type localGeoIPService struct {
	db *geoip2.Reader
}

// smartGeoIPService 是新的主服务实现，它会根据配置选择查询方式。
type smartGeoIPService struct {
	settingSvc     setting.SettingService
	localLookupSvc *localGeoIPService
	httpClient     *http.Client
}

// NewGeoIPService 是新的构造函数，注入了配置服务。
// 它现在是创建 GeoIPService 的唯一入口。
func NewGeoIPService(dbPath string, settingSvc setting.SettingService) (GeoIPService, error) {
	// 尝试初始化本地数据库服务，作为备用
	localSvc, err := newLocalGeoIPService(dbPath)
	if err != nil {
		// 即使本地库加载失败，也不应阻塞程序，因为可能配置了 API
		log.Printf("警告: 无法加载本地 GeoIP 数据库: %v。服务将仅依赖于远程 API。", err)
	}

	return &smartGeoIPService{
		settingSvc:     settingSvc,
		localLookupSvc: localSvc, // 如果加载失败，这里会是 nil
		httpClient: &http.Client{
			Timeout: 5 * time.Second, // 为 API 请求设置5秒超时
		},
	}, nil
}

// Lookup 是核心的智能查询方法。
func (s *smartGeoIPService) Lookup(ipStr string) (string, error) {
	apiURL := s.settingSvc.Get(constant.KeyIPAPI.String())
	apiToken := s.settingSvc.Get(constant.KeyIPAPIToKen.String())

	// 如果 API 和 Token 都已配置，则优先使用 API
	if apiURL != "" && apiToken != "" {
		location, err := s.lookupViaAPI(apiURL, apiToken, ipStr)
		if err == nil && location != "" {
			return location, nil // API 查询成功，直接返回结果
		}
		if err != nil {
			log.Printf("警告: API IP 查询失败: %v。回退至本地数据库。", err)
		}
	}

	// 如果 API 未配置或查询失败，则使用本地数据库
	if s.localLookupSvc != nil {
		return s.localLookupSvc.Lookup(ipStr)
	}

	// 如果两种方式都不可用
	return "未知", fmt.Errorf("IP 查询失败：所有查询方式均不可用")
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

// Close 优雅地关闭资源。
func (s *smartGeoIPService) Close() {
	if s.localLookupSvc != nil {
		s.localLookupSvc.Close()
	}
}

// newLocalGeoIPService 是 localGeoIPService 的私有构造函数。
func newLocalGeoIPService(dbPath string) (*localGeoIPService, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("GeoIP 数据库路径未提供")
	}
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开 GeoIP 数据库: %w", err)
	}
	return &localGeoIPService{db: db}, nil
}

// Lookup 使用本地数据库执行查询。
func (s *localGeoIPService) Lookup(ipString string) (string, error) {
	ip := net.ParseIP(ipString)
	if ip == nil {
		return "未知", fmt.Errorf("无效的IP地址格式: %s", ipString)
	}

	record, err := s.db.City(ip)
	if err != nil {
		return "未知", err // 查询失败或找不到记录
	}

	// 优先使用中文名称
	var province string
	if len(record.Subdivisions) > 0 {
		province = record.Subdivisions[0].Names["zh-CN"]
	}
	city := record.City.Names["zh-CN"]

	if province != "" && city != "" && province != city {
		return fmt.Sprintf("%s %s", province, city), nil
	}
	if city != "" {
		return city, nil
	}
	if province != "" {
		return province, nil
	}
	if record.Country.Names["zh-CN"] != "" {
		return record.Country.Names["zh-CN"], nil
	}

	// 如果中文名不存在，使用英文名作为后备
	if len(record.Subdivisions) > 0 {
		province = record.Subdivisions[0].Names["en"]
	}
	city = record.City.Names["en"]
	if province != "" && city != "" && province != city {
		return fmt.Sprintf("%s %s", province, city), nil
	}
	if city != "" {
		return city, nil
	}
	if province != "" {
		return province, nil
	}
	if record.Country.Names["en"] != "" {
		return record.Country.Names["en"], nil
	}

	return "未知", nil // 找到了记录，但没有可用的位置名称
}

// Close 关闭数据库连接。
func (s *localGeoIPService) Close() {
	if s.db != nil {
		s.db.Close()
	}
}
