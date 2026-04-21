/*
 * @Description: libvips CLI 引擎实现
 * @Author: 安知鱼
 *
 * Phase 2 Task 2.2：通过管道驱动 `vips thumbnail_source` 完成格式转换 + 缩放。
 *
 * 设计要点：
 *   1. 命令形式：
 *        vips --vips-concurrency=2 thumbnail_source '[descriptor=0]' '.{fmt}[Q=q]' W [--height H] [--size down] [--crop centre] [--no-rotate]
 *      stdin 作为 source，stdout 直接拿到目标格式。避免落盘。
 *   2. 尺寸处理：
 *        - cover   → thumbnail + --crop centre
 *        - contain/fit-inside → thumbnail（不加 crop）
 *        - scale   → 先用 image.DecodeConfig 读源图尺寸（JPEG/PNG/GIF/WebP 支持），再算 W/H
 *        - 其他 mode 或三者都 ≤ 0 → width 填超大常量 + --size down 保持原尺寸
 *   3. 超时：默认 30s；若 ctx 已带 deadline 则遵循 ctx 的 deadline。
 *   4. 并发：--vips-concurrency=2 抑制内部线程风暴，配合 Service 层 singleflight 足够。
 *   5. EXIF 自动旋转：thumbnail_source 默认会按 EXIF 旋转；AutoRotate=false 时加 --no-rotate。
 *   6. 错误识别：命令退出非 0 + stderr 命中"no known saver/no loader"等关键字 →
 *      返回 ErrFormatUnsupported，由 AutoEngine 走格式降级。
 *
 * 不支持场景：
 *   - Watermark（Phase 3 单独接入 vips composite）
 *   - scale 模式 + HEIC/AVIF（Go 标准库读不到尺寸，退化为不缩放仅做格式转换）
 */
package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

const (
	// vipsDefaultTimeout 为未在 ctx 指定 deadline 时的兜底超时。
	vipsDefaultTimeout = 30 * time.Second
	// vipsDefaultConcurrency 限制单次命令使用的线程数，避免多请求时线程爆炸。
	vipsDefaultConcurrency = 2
	// vipsNoResizeWidth 是"不需要 resize"的占位 width，
	// 配合 --size down 可在不放大原图的前提下保持原尺寸输出。
	vipsNoResizeWidth = 1000000
)

// VipsEngine 是基于外部 `vips` CLI 的图像处理引擎。
// 构造前建议调用 Probe() 获取 Capability，以便决定是否启用本引擎。
type VipsEngine struct {
	binaryPath string
	capability VipsCapability
}

// NewVipsEngine 按探测结果构造 VipsEngine。
// 调用方需要保证 cap.Available 为 true 且 BinaryPath 非空；
// 否则构造出来的引擎会在 Process 时报错。
func NewVipsEngine(cap VipsCapability) *VipsEngine {
	return &VipsEngine{
		binaryPath: cap.BinaryPath,
		capability: cap,
	}
}

// Name 返回引擎标识。
func (v *VipsEngine) Name() string { return "vips" }

// SupportedInputFormats 从 capability 映射；返回副本避免外部篡改。
func (v *VipsEngine) SupportedInputFormats() []string {
	return append([]string(nil), v.capability.InputFormats...)
}

// SupportedOutputFormats 从 capability 映射；返回副本避免外部篡改。
func (v *VipsEngine) SupportedOutputFormats() []string {
	return append([]string(nil), v.capability.OutputFormats...)
}

// Process 将 src 按 style 处理并写入 dst，返回输出 MIME。
// 若目标格式不受 vips 支持（capability 预检或 stderr 命中关键字），返回 ErrFormatUnsupported。
func (v *VipsEngine) Process(ctx context.Context, src io.Reader, style model.ImageStyleConfig, dst io.Writer) (string, error) {
	if v.binaryPath == "" {
		return "", errors.New("vips engine: 未配置 vips 可执行路径")
	}

	// 读入内存：一来 scale 模式需要先读 header；二来 stderr 若报格式错误，我们还需要带上源字节数做诊断（最小化）。
	buf, err := io.ReadAll(src)
	if err != nil {
		return "", fmt.Errorf("读取源图失败: %w", err)
	}
	if len(buf) == 0 {
		return "", errors.New("源图为空")
	}

	inputFormat := detectFormatFromMagic(buf)
	outFormat := resolveOutputFormat(style.Format, inputFormat)
	if !v.isOutputFormatSupported(outFormat) {
		return "", ErrFormatUnsupported
	}

	args := v.buildArgs(outFormat, style, buf)

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, vipsDefaultTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, v.binaryPath, args...)
	cmd.Stdin = bytes.NewReader(buf)
	cmd.Stdout = dst
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if isVipsFormatUnsupportedErr(stderr.String()) {
			return "", ErrFormatUnsupported
		}
		return "", fmt.Errorf("vips 执行失败: %w; stderr: %s",
			err, strings.TrimSpace(stderr.String()))
	}
	return mimeForFormat(outFormat), nil
}

// buildArgs 组装 vips 命令行参数（不含可执行路径本身）。
func (v *VipsEngine) buildArgs(outFormat string, style model.ImageStyleConfig, raw []byte) []string {
	args := []string{
		fmt.Sprintf("--vips-concurrency=%d", vipsDefaultConcurrency),
		"thumbnail_source",
		"[descriptor=0]",
		buildVipsOutputSpec(outFormat, style.Quality),
	}

	width, height, crop := resolveVipsResizeArgs(style.Resize, raw)
	args = append(args, strconv.Itoa(width))
	if height > 0 {
		args = append(args, "--height", strconv.Itoa(height))
	}
	// 默认禁止放大；style.Resize.Enlarge=true 时允许放大。
	if !style.Resize.Enlarge {
		args = append(args, "--size", "down")
	}
	if crop != "" {
		args = append(args, "--crop", crop)
	}
	if !style.AutoRotate {
		args = append(args, "--no-rotate")
	}
	return args
}

// buildVipsOutputSpec 构造 vips 输出后缀串，形如 `.webp[Q=80]` / `.png` 等。
// 对没有 quality 概念的 PNG/GIF 省略方括号。
func buildVipsOutputSpec(format string, quality int) string {
	q := clampQuality(quality)
	switch format {
	case "jpg", "jpeg":
		return fmt.Sprintf(".jpg[Q=%d]", defaultQualityIfZero(q, 75))
	case "webp":
		return fmt.Sprintf(".webp[Q=%d]", defaultQualityIfZero(q, 80))
	case "avif":
		return fmt.Sprintf(".avif[Q=%d]", defaultQualityIfZero(q, 50))
	case "heic", "heif":
		return fmt.Sprintf(".heic[Q=%d]", defaultQualityIfZero(q, 80))
	case "png":
		return ".png"
	case "gif":
		return ".gif"
	case "tiff":
		return ".tiff"
	default:
		return "." + format
	}
}

func clampQuality(q int) int {
	if q < 0 {
		return 0
	}
	if q > 100 {
		return 100
	}
	return q
}

func defaultQualityIfZero(q, def int) int {
	if q == 0 {
		return def
	}
	return q
}

// resolveVipsResizeArgs 把 Resize 配置翻译成 thumbnail_source 参数。
func resolveVipsResizeArgs(rc model.ImageResizeConfig, raw []byte) (width, height int, crop string) {
	switch rc.Mode {
	case "cover":
		w, h := rc.Width, rc.Height
		if w <= 0 && h <= 0 {
			return vipsNoResizeWidth, 0, ""
		}
		if w <= 0 {
			w = vipsNoResizeWidth
		}
		if h < 0 {
			h = 0
		}
		return w, h, "centre"

	case "contain", "fit-inside":
		w, h := rc.Width, rc.Height
		if w <= 0 && h <= 0 {
			return vipsNoResizeWidth, 0, ""
		}
		if w <= 0 {
			w = vipsNoResizeWidth
		}
		if h < 0 {
			h = 0
		}
		return w, h, ""

	case "scale":
		if rc.Scale <= 0 {
			return vipsNoResizeWidth, 0, ""
		}
		srcW, srcH, ok := decodeDimensions(raw)
		if !ok {
			// 无法读到尺寸（如 HEIC/AVIF）→ 退化为不 resize。
			return vipsNoResizeWidth, 0, ""
		}
		w := int(float64(srcW) * rc.Scale)
		h := int(float64(srcH) * rc.Scale)
		if w <= 0 || h <= 0 {
			return vipsNoResizeWidth, 0, ""
		}
		return w, h, ""

	default:
		return vipsNoResizeWidth, 0, ""
	}
}

// decodeDimensions 使用 image.DecodeConfig 读 JPEG/PNG/GIF/WebP 的尺寸。
// HEIC/AVIF 未注册在标准库里，会返回 false；由上层决定如何降级。
func decodeDimensions(data []byte) (w, h int, ok bool) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// detectFormatFromMagic 通过魔数识别输入格式。
// 只覆盖 image/image_style 目前声明的几类；识别失败返回 ""。
func detectFormatFromMagic(buf []byte) string {
	if len(buf) < 12 {
		return ""
	}
	switch {
	case buf[0] == 0xFF && buf[1] == 0xD8 && buf[2] == 0xFF:
		return "jpeg"
	case bytes.HasPrefix(buf, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case bytes.HasPrefix(buf, []byte("GIF87a")) || bytes.HasPrefix(buf, []byte("GIF89a")):
		return "gif"
	case bytes.HasPrefix(buf, []byte("RIFF")) && bytes.Equal(buf[8:12], []byte("WEBP")):
		return "webp"
	case bytes.Equal(buf[4:8], []byte("ftyp")):
		// HEIF family，通过 ftyp brand 区分。
		brand := string(buf[8:12])
		switch brand {
		case "avif", "avis":
			return "avif"
		case "heic", "heix", "mif1", "msf1", "heis":
			return "heic"
		}
		return "heic"
	}
	return ""
}

// isOutputFormatSupported 通过 capability 预检输出格式。
func (v *VipsEngine) isOutputFormatSupported(format string) bool {
	norm := normalizeImageFormat(format)
	for _, f := range v.capability.OutputFormats {
		if normalizeImageFormat(f) == norm {
			return true
		}
	}
	return false
}

// mimeForFormat 把内部格式标识映射为 HTTP MIME 字符串。
func mimeForFormat(f string) string {
	switch f {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "avif":
		return "image/avif"
	case "heic", "heif":
		return "image/heic"
	case "tiff":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}

// isVipsFormatUnsupportedErr 判断 stderr 是否表达"不支持该格式"。
// vips 在不同版本可能输出不同文案；这里取常见关键字。
func isVipsFormatUnsupportedErr(stderr string) bool {
	s := strings.ToLower(stderr)
	for _, kw := range []string{
		"no known saver",
		"no known target",
		"no loader",
		"unknown suffix",
		"no such operation",
		"not supported",
	} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// 静态断言 VipsEngine 实现 Engine 接口。
var _ Engine = (*VipsEngine)(nil)
