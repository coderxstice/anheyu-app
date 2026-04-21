/*
 * @Description: AutoEngine 降级路径测试
 * @Author: 安知鱼
 */
package engine

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// stubEngine 用于精细控制每次 Process 的返回值。
type stubEngine struct {
	name    string
	inFmts  []string
	outFmts []string
	calls   []model.ImageStyleConfig
	handler func(style model.ImageStyleConfig) (string, error)
}

func (s *stubEngine) Name() string                      { return s.name }
func (s *stubEngine) SupportedInputFormats() []string   { return s.inFmts }
func (s *stubEngine) SupportedOutputFormats() []string  { return s.outFmts }
func (s *stubEngine) Process(ctx context.Context, src io.Reader, style model.ImageStyleConfig, dst io.Writer) (string, error) {
	s.calls = append(s.calls, style)
	return s.handler(style)
}

// drainReader 消费 src，避免因未读完源数据导致的测试资源泄漏。
func drainReader(src io.Reader) { _, _ = io.Copy(io.Discard, src) }

func TestAutoEngine_PrimarySuccess_NoFallback(t *testing.T) {
	primary := &stubEngine{
		name:    "primary",
		outFmts: []string{"jpg"},
		handler: func(style model.ImageStyleConfig) (string, error) {
			return "image/jpeg", nil
		},
	}
	fallback := &stubEngine{
		name:    "fallback",
		outFmts: []string{"jpg"},
		handler: func(style model.ImageStyleConfig) (string, error) {
			t.Fatalf("primary 成功时不应调 fallback")
			return "", nil
		},
	}
	auto := NewAutoEngineWith(primary, fallback, VipsCapability{})

	var out writerSink
	mime, err := auto.Process(context.Background(), readerOf("src"), model.ImageStyleConfig{Format: "jpg"}, &out)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("mime 期望 image/jpeg，实际 %s", mime)
	}
	if len(primary.calls) != 1 || len(fallback.calls) != 0 {
		t.Errorf("primary 应被调 1 次，fallback 应为 0；实际 primary=%d fallback=%d",
			len(primary.calls), len(fallback.calls))
	}
}

func TestAutoEngine_FallbackOnFormatUnsupported_AvifToJpeg(t *testing.T) {
	primary := &stubEngine{
		name:    "primary",
		outFmts: []string{"jpg"},
		handler: func(style model.ImageStyleConfig) (string, error) {
			// primary 不支持 avif
			if style.Format == "avif" {
				return "", ErrFormatUnsupported
			}
			return "image/jpeg", nil
		},
	}
	fallback := &stubEngine{
		name:    "fallback",
		outFmts: []string{"jpg"},
		handler: func(style model.ImageStyleConfig) (string, error) {
			// fallback 支持 jpg
			if style.Format == "jpg" {
				return "image/jpeg", nil
			}
			return "", ErrFormatUnsupported
		},
	}
	auto := NewAutoEngineWith(primary, fallback, VipsCapability{})

	var out writerSink
	mime, err := auto.Process(context.Background(), readerOf("src"),
		model.ImageStyleConfig{Format: "avif", Quality: 80},
		&out)
	if err != nil {
		t.Fatalf("降级后不应失败，实际 err=%v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("降级后 mime 应为 image/jpeg，实际 %s", mime)
	}
	if len(primary.calls) != 1 {
		t.Errorf("primary 应被调 1 次，实际 %d", len(primary.calls))
	}
	if len(fallback.calls) != 1 {
		t.Fatalf("fallback 应被调 1 次（降级），实际 %d", len(fallback.calls))
	}
	if fallback.calls[0].Format != "jpg" {
		t.Errorf("降级后 format 应为 jpg，实际 %s", fallback.calls[0].Format)
	}
	if fallback.calls[0].Quality != 80 {
		t.Errorf("降级保留 quality=80，实际 %d", fallback.calls[0].Quality)
	}
}

func TestAutoEngine_FallbackOnFormatUnsupported_HeicToJpeg(t *testing.T) {
	primary := &stubEngine{
		outFmts: []string{"jpg"},
		handler: func(style model.ImageStyleConfig) (string, error) {
			return "", ErrFormatUnsupported
		},
	}
	fallback := &stubEngine{
		outFmts: []string{"jpg"},
		handler: func(style model.ImageStyleConfig) (string, error) {
			return "image/jpeg", nil
		},
	}
	auto := NewAutoEngineWith(primary, fallback, VipsCapability{})

	var out writerSink
	_, err := auto.Process(context.Background(), readerOf("src"),
		model.ImageStyleConfig{Format: "heic"},
		&out)
	if err != nil {
		t.Fatalf("降级后不应失败，实际 err=%v", err)
	}
	if fallback.calls[0].Format != "jpg" {
		t.Errorf("heic → jpg，实际 %s", fallback.calls[0].Format)
	}
}

func TestAutoEngine_FallbackOnFormatUnsupported_WebpToJpeg(t *testing.T) {
	primary := &stubEngine{
		handler: func(style model.ImageStyleConfig) (string, error) {
			return "", ErrFormatUnsupported
		},
	}
	fallback := &stubEngine{
		handler: func(style model.ImageStyleConfig) (string, error) {
			return "image/jpeg", nil
		},
	}
	auto := NewAutoEngineWith(primary, fallback, VipsCapability{})

	var out writerSink
	if _, err := auto.Process(context.Background(), readerOf("src"),
		model.ImageStyleConfig{Format: "webp"}, &out); err != nil {
		t.Fatalf("应当降级成功，实际 err=%v", err)
	}
	if fallback.calls[0].Format != "jpg" {
		t.Errorf("webp → jpg，实际 %s", fallback.calls[0].Format)
	}
}

func TestAutoEngine_JpegPngNotDegraded(t *testing.T) {
	// JPEG / PNG 不在降级表里；primary 返回 ErrFormatUnsupported 时直接返回该错误
	// （不再触发 fallback 的格式转换）
	primary := &stubEngine{
		handler: func(style model.ImageStyleConfig) (string, error) {
			return "", ErrFormatUnsupported
		},
	}
	fallback := &stubEngine{
		handler: func(style model.ImageStyleConfig) (string, error) {
			return "image/jpeg", nil
		},
	}
	auto := NewAutoEngineWith(primary, fallback, VipsCapability{})

	var out writerSink
	_, err := auto.Process(context.Background(), readerOf("src"),
		model.ImageStyleConfig{Format: "jpg"}, &out)
	if !errors.Is(err, ErrFormatUnsupported) {
		t.Errorf("jpg 不应触发降级；期望 ErrFormatUnsupported，实际 %v", err)
	}
	if len(fallback.calls) != 0 {
		t.Errorf("jpg 不应触发 fallback 调用；实际 %d 次", len(fallback.calls))
	}
}

func TestAutoEngine_PrimaryNonFormatError_NoFallback(t *testing.T) {
	customErr := errors.New("decode error")
	primary := &stubEngine{
		handler: func(style model.ImageStyleConfig) (string, error) {
			return "", customErr
		},
	}
	fallback := &stubEngine{
		handler: func(style model.ImageStyleConfig) (string, error) {
			t.Fatalf("非 ErrFormatUnsupported 错误不应降级")
			return "", nil
		},
	}
	auto := NewAutoEngineWith(primary, fallback, VipsCapability{})

	var out writerSink
	_, err := auto.Process(context.Background(), readerOf("src"),
		model.ImageStyleConfig{Format: "avif"}, &out)
	if !errors.Is(err, customErr) {
		t.Errorf("应透传原错误，实际 %v", err)
	}
}

func TestNewAutoEngine_DefaultsToNativeGo(t *testing.T) {
	auto := NewAutoEngine(VipsCapability{Available: false})
	if auto.Primary().Name() != "nativego" {
		t.Errorf("vips 不可用时 primary 应为 nativego，实际 %s", auto.Primary().Name())
	}
	if auto.Fallback().Name() != "nativego" {
		t.Errorf("fallback 应为 nativego，实际 %s", auto.Fallback().Name())
	}
}

func TestProbe_Phase1_ReturnsUnavailable(t *testing.T) {
	cap := Probe()
	if cap.Available {
		t.Errorf("Phase 1 Probe() 应返回 Available: false（vips 引擎尚未实现）；实际 %+v", cap)
	}
}

// ---- 测试工具 ----

type writerSink struct{ buf []byte }

func (w *writerSink) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

// readerOf 返回一个带预设字节流的 io.Reader；每次测试都创建新的 reader，避免 seek 污染。
func readerOf(s string) io.Reader {
	return &byteReader{src: []byte(s)}
}

type byteReader struct {
	src []byte
	pos int
}

func (b *byteReader) Read(p []byte) (int, error) {
	if b.pos >= len(b.src) {
		return 0, io.EOF
	}
	n := copy(p, b.src[b.pos:])
	b.pos += n
	return n, nil
}

var _ = drainReader // 抑制"未使用"提示；后续测试如果需要可直接引用
