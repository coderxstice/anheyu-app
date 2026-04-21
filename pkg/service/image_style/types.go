/*
 * @Description: image_style 服务的内部类型定义
 * @Author: 安知鱼
 *
 * 这些类型只在 image_style 包内部以及与之直接交互的 handler / service 之间流转，
 * 不作为 HTTP API 的序列化 DTO。对外公开的持久化配置结构体位于 pkg/domain/model。
 */
package image_style

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// StyleRequest 描述一次样式请求的完整上下文。
// 由 handler 层解析 URL + 查询数据库后装配，传入 ImageStyleService.Process。
type StyleRequest struct {
	// Policy 文件所属的存储策略；内含 ImageProcessConfig / ImageStyles。
	Policy *model.StoragePolicy
	// File 文件本身（主要关心 ID / 名称 / 主存储实体），必填。
	File *model.File
	// Filename 原始文件名（可能与 File.Name 一致，也可能来自上传时的原始名）。
	// Matcher 判定扩展名时以此为准。
	Filename string
	// StyleName 从 URL 分离出的命名样式；为空字符串表示未显式指定。
	StyleName string
	// DynamicOpts 原始 URL query；可能为空。保留 url.Values 类型以便兼容多值参数。
	DynamicOpts url.Values
}

// StyleResult 是 ImageStyleService.Process 的成功结果，
// 调用方需要负责消费 Reader 并在消费完后调用 Close。
type StyleResult struct {
	// ContentType 输出 MIME 类型，如 "image/webp"。
	ContentType string
	// Reader 指向处理后的图片字节流（内存缓冲或磁盘文件）。
	Reader io.ReadCloser
	// Size 处理后的字节大小。
	Size int64
	// FromCache 是否命中磁盘缓存（未命中时由引擎实时处理）。
	FromCache bool
	// StyleHash ResolvedStyle 的稳定 hash（前 16 位 hex），也作为 ETag 响应头。
	StyleHash string
	// LastModified 缓存文件的最后修改时间；未命中缓存时为零值。
	LastModified time.Time
	// RequestedFormat 是样式配置（或动态参数）中用户原始请求的输出格式，
	// 如 "webp" / "avif" / "jpg" / "original" / ""。Handler 层会把它与实际
	// ContentType 对比，若不一致写入 `X-Style-Fallback: requested->actual` 响应头
	// 便于客户端与运维排查格式降级（Spec §6.3.3）。
	// 为空或 "original" 表示"无显式格式要求"，不触发降级头。
	RequestedFormat string
}

// ResolvedStyle 是 Matcher 经过命名样式/动态参数/默认样式合并后得到的最终处理参数。
// 此结构体不会被持久化，仅作为引擎入参与缓存键计算输入。
// 字段顺序严格按照 JSON tag 序列化，用于 Hash() 计算。
type ResolvedStyle struct {
	Format     string                   `json:"format"`
	Quality    int                      `json:"quality"`
	AutoRotate bool                     `json:"auto_rotate"`
	Resize     model.ImageResizeConfig  `json:"resize"`
	Watermark  *model.WatermarkConfig   `json:"watermark,omitempty"`
}

// Hash 返回 ResolvedStyle 的稳定哈希（sha256 前 16 位 hex）。
// 用于：1) 缓存文件名的一部分；2) HTTP ETag；3) singleflight 合并 key。
//
// 稳定性来源：
//   - encoding/json 默认按字段定义顺序输出，结构体内字段顺序固定。
//   - map 类型不会出现在 ResolvedStyle 中（避免随机顺序问题）。
//   - omitempty 仅对 Watermark 生效，nil 与 &{} 产生的 JSON 不同 -> hash 不同。
func (r ResolvedStyle) Hash() string {
	b, err := json.Marshal(&r)
	if err != nil {
		// 在实际运行中几乎不会发生：结构体字段均为基础类型或嵌套结构体。
		// 为避免 panic 影响热路径，退化到带错误标记的空值 hash，方便排查。
		sum := sha256.Sum256([]byte("image_style/marshal_error"))
		return hex.EncodeToString(sum[:8])
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// CacheStats 描述单个策略的缓存统计信息。
// 管理员 API / 启动日志会以此为 DTO 直接返回。
type CacheStats struct {
	PolicyID  uint  `json:"policy_id"`
	TotalSize int64 `json:"total_size"` // 字节
	Count     int   `json:"count"`      // 条目数
	// HitCount / MissCount 分别统计命中与未命中次数（进程级累积）。
	HitCount  int64 `json:"hit_count"`
	MissCount int64 `json:"miss_count"`
}
