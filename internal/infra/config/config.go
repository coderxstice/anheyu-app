/*
 * @Description: 统一配置管理 (终极健壮版，手动加载)
 * @Author: 安知鱼
 * @Date: 2025-06-28 00:21:55
 * @LastEditTime: 2025-08-14 13:00:20
 * @LastEditors: 安知鱼
 */
package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-ini/ini"
	"github.com/spf13/viper"
)

// 定义所有已知的配置键
var allKeys = []string{
	KeyServerPort, KeyServerDebug,
	KeyDBType, KeyDBHost, KeyDBPort, KeyDBUser, KeyDBPassword, KeyDBName, KeyDBDebug,
	KeyRedisAddr, KeyRedisPassword, KeyRedisDB,
}

const (
	KeyServerPort    = "System.Port"
	KeyServerDebug   = "System.Debug"
	KeyDBType        = "Database.Type"
	KeyDBHost        = "Database.Host"
	KeyDBPort        = "Database.Port"
	KeyDBUser        = "Database.User"
	KeyDBPassword    = "Database.Password"
	KeyDBName        = "Database.Name"
	KeyDBDebug       = "Database.Debug"
	KeyRedisAddr     = "Redis.Addr"
	KeyRedisPassword = "Redis.Password"
	KeyRedisDB       = "Redis.DB"
)

type Config struct {
	vp *viper.Viper
}

// NewConfig 是最终的构造函数，手动加载配置，确保可靠性
func NewConfig() (*Config, error) {
	vp := viper.New()
	filePath := "data/conf.ini"

	// --- 步骤 1: 使用 go-ini 从文件加载配置 (作为默认值) ---
	iniCfg, err := ini.Load(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("提示: 未找到 %s，将仅依赖环境变量或内部默认值。", filePath)
		} else {
			// 如果文件存在但格式错误
			return nil, fmt.Errorf("错误: 解析配置文件 '%s' 失败: %w", filePath, err)
		}
	}

	// 如果文件成功加载，则将其中的值全部设置到 Viper 中
	if iniCfg != nil {
		for _, section := range iniCfg.Sections() {
			for _, key := range section.Keys() {
				// 构建 Viper 使用的 key，例如 "Database.Host"
				viperKey := fmt.Sprintf("%s.%s", section.Name(), key.Name())
				// 特殊处理默认分区 "DEFAULT"
				if section.Name() == "DEFAULT" {
					viperKey = key.Name()
				}
				vp.Set(viperKey, key.Value())
			}
		}
		log.Println("从 data/conf.ini 文件加载了默认配置。")
	}

	// --- 步骤 2: 手动检查并覆盖环境变量 ---
	envReplacer := strings.NewReplacer(".", "_")
	envPrefix := "ANHEYU"

	for _, key := range allKeys {
		// 构建环境变量名，例如 ANHEYU_DATABASE_HOST
		envVarName := fmt.Sprintf("%s_%s", envPrefix, envReplacer.Replace(strings.ToUpper(key)))

		// 检查环境变量是否存在
		if value, found := os.LookupEnv(envVarName); found {
			// 如果存在，就用环境变量的值覆盖 Viper 中的值
			vp.Set(key, value)
			log.Printf("发现环境变量: %s, 已覆盖配置 '%s'。", envVarName, key)
		}
	}

	log.Println("✅ 配置加载器初始化完成。")
	return &Config{vp: vp}, nil
}

func (c *Config) GetString(key string) string {
	return c.vp.GetString(key)
}

func (c *Config) GetInt(key string) int {
	return c.vp.GetInt(key)
}

func (c *Config) GetBool(key string) bool {
	return c.vp.GetBool(key)
}
