/*
 * @Description: 水印抽象与占位实现
 * @Author: 安知鱼
 *
 * Phase 1 只保留接口与 Noop 实现，Service 持有一个 Watermarker 字段。
 * Phase 3 Task 3.4 才真正渲染文本 / 图片水印，并在 Engine 执行后接入。
 *
 * 设计约束：
 *   - Watermarker.Apply 接收已解码的 image.Image 与水印配置，返回叠加水印后的新图。
 *   - WatermarkConfig == nil 时 Service 必须不调用 Apply，以避免无谓分配。
 *   - Phase 1 的 Noop 实现即使被误调也会原样返回，不产生副作用。
 */
package image_style

import (
	"image"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// Watermarker 描述水印渲染能力。Phase 3 会引入文字 / 图片两种实现。
type Watermarker interface {
	// Apply 叠加水印；cfg 非 nil 时才由 Service 调用。
	// 当前约定：实现不得修改传入的 img 原位字节，总是返回新 image。
	Apply(img image.Image, cfg *model.WatermarkConfig) (image.Image, error)
}

// NoopWatermarker 是 Phase 1 使用的占位实现；原样返回图片。
type NoopWatermarker struct{}

// Apply 实现 Watermarker 接口；无副作用。
func (NoopWatermarker) Apply(img image.Image, _ *model.WatermarkConfig) (image.Image, error) {
	return img, nil
}

// NewNoopWatermarker 构造一个占位水印实现，供 Phase 1 Service DI 使用。
func NewNoopWatermarker() Watermarker {
	return NoopWatermarker{}
}
