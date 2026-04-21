/*
 * @Description: 纯 Go 引擎测试
 * @Author: 安知鱼
 */
package engine

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// makeSolidJPEG 构造一张指定大小的纯色 JPEG 字节流，用于测试解码 + 处理流程。
func makeSolidJPEG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func makeSolidPNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestNativeGoEngine_Name(t *testing.T) {
	e := NewNativeGoEngine()
	if e.Name() != "nativego" {
		t.Errorf("Name 期望 nativego，实际 %s", e.Name())
	}
}

func TestNativeGoEngine_SupportedFormats(t *testing.T) {
	e := NewNativeGoEngine()
	in := e.SupportedInputFormats()
	out := e.SupportedOutputFormats()

	mustHave := func(list []string, v string) {
		for _, x := range list {
			if x == v {
				return
			}
		}
		t.Errorf("期望格式列表含 %s，实际 %v", v, list)
	}
	for _, ext := range []string{"jpg", "jpeg", "png", "gif", "webp"} {
		mustHave(in, ext)
	}
	for _, ext := range []string{"jpg", "jpeg", "png"} {
		mustHave(out, ext)
	}
}

func TestNativeGoEngine_CoverResize_JPEG_To_JPEG(t *testing.T) {
	e := NewNativeGoEngine()
	src := makeSolidJPEG(t, 800, 600, color.RGBA{255, 128, 64, 255})

	style := model.ImageStyleConfig{
		Format:  "jpg",
		Quality: 80,
		Resize:  model.ImageResizeConfig{Mode: "cover", Width: 400, Height: 300},
	}
	var out bytes.Buffer
	mime, err := e.Process(context.Background(), bytes.NewReader(src), style, &out)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("mime 期望 image/jpeg，实际 %s", mime)
	}

	// 解码输出 JPEG 验证尺寸
	got, err := jpeg.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("解码输出失败：%v", err)
	}
	b := got.Bounds()
	if b.Dx() != 400 || b.Dy() != 300 {
		t.Errorf("输出尺寸期望 400x300，实际 %dx%d", b.Dx(), b.Dy())
	}
}

func TestNativeGoEngine_ContainResize_PNG_To_PNG(t *testing.T) {
	e := NewNativeGoEngine()
	src := makeSolidPNG(t, 1000, 500, color.RGBA{0, 200, 0, 255})

	style := model.ImageStyleConfig{
		Format: "png",
		Resize: model.ImageResizeConfig{Mode: "contain", Width: 400, Height: 400},
	}
	var out bytes.Buffer
	mime, err := e.Process(context.Background(), bytes.NewReader(src), style, &out)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("mime 期望 image/png，实际 %s", mime)
	}

	got, err := png.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("解码输出失败：%v", err)
	}
	b := got.Bounds()
	// contain 模式下两边都不超过目标边界，且保持宽高比
	if b.Dx() > 400 || b.Dy() > 400 {
		t.Errorf("contain 输出超出目标边界：%dx%d", b.Dx(), b.Dy())
	}
	if b.Dx() != 400 {
		// 原图比例 1000x500=2:1，contain 到 400x400 应是 400x200
		t.Errorf("contain 输出尺寸不符合预期：%dx%d（期望 400x200）", b.Dx(), b.Dy())
	}
}

func TestNativeGoEngine_ScaleMode(t *testing.T) {
	e := NewNativeGoEngine()
	src := makeSolidJPEG(t, 800, 600, color.RGBA{10, 20, 30, 255})

	style := model.ImageStyleConfig{
		Format:  "jpg",
		Quality: 80,
		Resize:  model.ImageResizeConfig{Mode: "scale", Scale: 0.5},
	}
	var out bytes.Buffer
	if _, err := e.Process(context.Background(), bytes.NewReader(src), style, &out); err != nil {
		t.Fatalf("Process: %v", err)
	}

	got, err := jpeg.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("解码输出失败：%v", err)
	}
	b := got.Bounds()
	if b.Dx() != 400 || b.Dy() != 300 {
		t.Errorf("scale=0.5 的输出应为 400x300，实际 %dx%d", b.Dx(), b.Dy())
	}
}

func TestNativeGoEngine_OriginalFormat_KeepsInputType(t *testing.T) {
	e := NewNativeGoEngine()
	src := makeSolidPNG(t, 200, 200, color.RGBA{1, 2, 3, 255})

	style := model.ImageStyleConfig{
		Format: "original",
		Resize: model.ImageResizeConfig{Mode: "cover", Width: 100, Height: 100},
	}
	var out bytes.Buffer
	mime, err := e.Process(context.Background(), bytes.NewReader(src), style, &out)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("original 模式下应保持 PNG 输入类型，实际 mime=%s", mime)
	}
	if _, err := png.Decode(bytes.NewReader(out.Bytes())); err != nil {
		t.Errorf("输出应为 PNG，解码失败：%v", err)
	}
}

func TestNativeGoEngine_UnsupportedOutputFormat_ReturnsErr(t *testing.T) {
	e := NewNativeGoEngine()
	src := makeSolidJPEG(t, 100, 100, color.RGBA{0, 0, 0, 255})

	style := model.ImageStyleConfig{
		Format: "avif",
		Resize: model.ImageResizeConfig{Mode: "cover", Width: 50, Height: 50},
	}
	var out bytes.Buffer
	_, err := e.Process(context.Background(), bytes.NewReader(src), style, &out)
	if !errors.Is(err, ErrFormatUnsupported) {
		t.Errorf("期望 ErrFormatUnsupported，实际 %v", err)
	}
}

func TestNativeGoEngine_EnlargeDefault_DoesNotExceedSource(t *testing.T) {
	e := NewNativeGoEngine()
	src := makeSolidJPEG(t, 200, 200, color.RGBA{50, 50, 50, 255})

	// 未开启 Enlarge，目标 400x400 应被限制到 200x200
	style := model.ImageStyleConfig{
		Format: "jpg", Quality: 80,
		Resize: model.ImageResizeConfig{Mode: "cover", Width: 400, Height: 400, Enlarge: false},
	}
	var out bytes.Buffer
	if _, err := e.Process(context.Background(), bytes.NewReader(src), style, &out); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got, _ := jpeg.Decode(bytes.NewReader(out.Bytes()))
	b := got.Bounds()
	if b.Dx() > 200 || b.Dy() > 200 {
		t.Errorf("enlarge=false 不应放大超过原图；实际 %dx%d", b.Dx(), b.Dy())
	}
}

// TestApplyExifOrientation 直接覆盖每种 Orientation 值（1-8）的像素变换正确性。
// 使用 3x2 彩色矩阵：(红 绿 蓝) / (黄 紫 青) — 每个像素独立颜色便于判定旋转结果。
func TestApplyExifOrientation(t *testing.T) {
	// 生成源图像：3x2，左上红 右上绿 右下青 左下黄
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	// row 0: 红(0,0), 绿(1,0), 蓝(2,0)
	src.Set(0, 0, color.RGBA{255, 0, 0, 255})
	src.Set(1, 0, color.RGBA{0, 255, 0, 255})
	src.Set(2, 0, color.RGBA{0, 0, 255, 255})
	// row 1: 黄(0,1), 紫(1,1), 青(2,1)
	src.Set(0, 1, color.RGBA{255, 255, 0, 255})
	src.Set(1, 1, color.RGBA{255, 0, 255, 255})
	src.Set(2, 1, color.RGBA{0, 255, 255, 255})

	cases := []struct {
		name        string
		orientation int
		// expectedSize 处理后的宽高
		expectedW, expectedH int
		// checkPixel 验证 (x,y) 位置的颜色应该等于原图 (srcX,srcY) 的颜色
		checkX, checkY   int
		srcX, srcY       int
	}{
		{"Orientation=1 原样", 1, 3, 2, 0, 0, 0, 0},
		{"Orientation=2 水平翻转", 2, 3, 2, 0, 0, 2, 0}, // 输出(0,0) = 原(2,0) 蓝
		{"Orientation=3 旋转 180", 3, 3, 2, 0, 0, 2, 1}, // 输出(0,0) = 原(2,1) 青
		{"Orientation=4 垂直翻转", 4, 3, 2, 0, 0, 0, 1},   // 输出(0,0) = 原(0,1) 黄
		// 5-8 涉及 90 度旋转，输出尺寸会变为 2x3
		{"Orientation=5 Transpose", 5, 2, 3, 0, 0, 0, 0},       // 输出(0,0) = 原(0,0) 红
		{"Orientation=6 顺时针 90", 6, 2, 3, 0, 0, 0, 1},           // 输出(0,0) = 原(0,1) 黄
		{"Orientation=7 Transverse", 7, 2, 3, 0, 0, 2, 1},      // 输出(0,0) = 原(2,1) 青
		{"Orientation=8 逆时针 90", 8, 2, 3, 0, 0, 2, 0},           // 输出(0,0) = 原(2,0) 蓝
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := applyExifOrientation(src, c.orientation)
			b := got.Bounds()
			if b.Dx() != c.expectedW || b.Dy() != c.expectedH {
				t.Errorf("尺寸错误：期望 %dx%d，实际 %dx%d", c.expectedW, c.expectedH, b.Dx(), b.Dy())
				return
			}
			expected := src.At(c.srcX, c.srcY)
			actual := got.At(b.Min.X+c.checkX, b.Min.Y+c.checkY)
			er, eg, eb, ea := expected.RGBA()
			ar, ag, ab, aa := actual.RGBA()
			if er != ar || eg != ag || eb != ab || ea != aa {
				t.Errorf("像素不匹配 (expected=%v actual=%v)", expected, actual)
			}
		})
	}
}

func TestNativeGoEngine_AutoRotate_NoEXIF_Unchanged(t *testing.T) {
	// 无 EXIF 的 JPEG，auto_rotate=true 不应改变方向
	e := NewNativeGoEngine()
	src := makeSolidJPEG(t, 400, 200, color.RGBA{100, 100, 100, 255})

	style := model.ImageStyleConfig{
		Format: "jpg", Quality: 80, AutoRotate: true,
		Resize: model.ImageResizeConfig{Mode: "cover", Width: 400, Height: 200},
	}
	var out bytes.Buffer
	if _, err := e.Process(context.Background(), bytes.NewReader(src), style, &out); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got, _ := jpeg.Decode(bytes.NewReader(out.Bytes()))
	b := got.Bounds()
	if b.Dx() != 400 || b.Dy() != 200 {
		t.Errorf("无 EXIF 时 AutoRotate 不应改变方向；实际 %dx%d", b.Dx(), b.Dy())
	}
}
