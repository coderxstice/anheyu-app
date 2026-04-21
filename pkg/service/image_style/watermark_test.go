/*
 * @Description: Watermark 占位接口测试
 * @Author: 安知鱼
 */
package image_style

import (
	"image"
	"image/color"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

func TestNoopWatermarker_ReturnsSameImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	src.Set(1, 1, color.RGBA{100, 150, 200, 255})

	w := NewNoopWatermarker()

	// cfg != nil 也应原样返回
	got, err := w.Apply(src, &model.WatermarkConfig{Type: "text", Text: "x"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got == nil {
		t.Fatalf("Noop 不应返回 nil")
	}
	if got.Bounds() != src.Bounds() {
		t.Errorf("尺寸应保持一致，实际 %v", got.Bounds())
	}

	// cfg == nil 也应安全
	got, err = w.Apply(src, nil)
	if err != nil || got == nil {
		t.Errorf("nil cfg 不应报错；err=%v got=%v", err, got)
	}
}

// TestWatermarker_InterfaceCompliance 用编译时静态断言确保 NoopWatermarker 满足 Watermarker。
func TestWatermarker_InterfaceCompliance(t *testing.T) {
	var _ Watermarker = NoopWatermarker{}
	var _ Watermarker = NewNoopWatermarker()
}
