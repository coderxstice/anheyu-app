/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-06-15 11:30:55
 * @LastEditTime: 2025-08-07 14:22:55
 * @LastEditors: 安知鱼
 */
package database

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/anzhiyu-c/anheyu-app/pkg/config"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient 是一个新的构造函数，它接收配置并返回 Redis 客户端或错误
func NewRedisClient(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	// 1. 从注入的 cfg 对象获取配置
	redisAddr := cfg.GetString(config.KeyRedisAddr)
	redisPassword := cfg.GetString(config.KeyRedisPassword)

	redisDBStr := "10"
	if cfg.GetString(config.KeyRedisDB) != "" {
		redisDBStr = cfg.GetString(config.KeyRedisDB)
	}

	if redisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR 未在配置中设置")
	}

	var redisDB int
	if redisDBStr != "" {
		var err error
		redisDB, err = strconv.Atoi(redisDBStr)
		if err != nil {
			return nil, fmt.Errorf("无效的 REDIS_DB 值 '%s': %w", redisDBStr, err)
		}
	} // 如果为空，redisDB 默认为 10，符合预期

	// 2. 创建 Redis 客户端实例
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// 3. 检查连接，并返回 error 而不是 Fatal
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis (%s, DB %d) 失败: %w", redisAddr, redisDB, err)
	}

	log.Printf("成功连接到 Redis (%s, DB %d)", redisAddr, redisDB)
	// 4. 返回创建的客户端实例和 nil 错误
	return rdb, nil
}
