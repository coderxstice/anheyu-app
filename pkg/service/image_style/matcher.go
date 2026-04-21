/*
 * @Description: 样式匹配决策树实现
 * @Author: 安知鱼
 *
 * 对应规范 §6.2：
 *     命名样式 > 动态参数 > 默认样式 > 不处理
 *
 * 动态参数映射表（§5.4）：
 *     w/width     -> resize.width
 *     h/height    -> resize.height
 *     fm/format   -> format
 *     q/quality   -> quality
 *     fit         -> resize.mode（cover/contain/inside/scale）
 *     s/scale     -> resize.scale
 *     rotate      -> auto_rotate（0/1）
 *
 * Phase 1 基础实现，Phase 3 会补全严格的合法性校验（见 Plan Task 3.1）。
 */
package image_style

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// Match 根据策略配置 + URL 语义产出 ResolvedStyle，错误语义见 errors.go。
//   - filename：文件原始名（用于判定扩展名是否在处理白名单内）
//   - styleName：URL 分隔符后的命名样式；空串表示未显式指定
//   - query：原始 URL query（可能为 nil）
func Match(policy *model.StoragePolicy, filename, styleName string, query url.Values) (*ResolvedStyle, error) {
	if policy == nil {
		return nil, ErrStyleNotApplicable
	}

	process := parseImageProcess(policy)
	if !process.Enabled || len(process.ApplyToExtensions) == 0 {
		return nil, ErrStyleNotApplicable
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if ext == "" || !containsString(process.ApplyToExtensions, ext) {
		return nil, ErrStyleNotApplicable
	}

	// 1. 命名样式显式请求（URL 带 !styleName）
	if styleName != "" {
		s, ok := findStyleByName(policy, styleName)
		if !ok {
			return nil, ErrStyleNotFound
		}
		resolved := styleToResolved(s)
		applyQueryOverrides(&resolved, query)
		return &resolved, nil
	}

	// 2. 纯动态参数请求（URL 带 ?w=... 等，但没有命名样式）
	if hasAnyDynamicOpt(query) {
		resolved := ResolvedStyle{
			// Phase 1 默认 JPEG / 质量 80 / 自动旋转 true；Phase 3 可由 DefaultThumbParam 等覆盖
			Format:     "jpg",
			Quality:    80,
			AutoRotate: true,
			Resize:     model.ImageResizeConfig{Mode: "cover"},
		}
		applyQueryOverrides(&resolved, query)
		return &resolved, nil
	}

	// 3. 默认样式
	if process.DefaultStyle != "" {
		s, ok := findStyleByName(policy, process.DefaultStyle)
		if !ok {
			// 默认样式配置了但未找到实体 → 视为不适用（不是 404）
			return nil, ErrStyleNotApplicable
		}
		resolved := styleToResolved(s)
		return &resolved, nil
	}

	return nil, ErrStyleNotApplicable
}

// parseImageProcess 从 policy.Settings 中解析 image_process 子段。
// 允许字段缺失，缺失时返回零值（等效未启用）。
func parseImageProcess(policy *model.StoragePolicy) model.ImageProcessConfig {
	var cfg model.ImageProcessConfig
	raw, ok := policy.Settings[constant.ImageProcessSettingsKey]
	if !ok || raw == nil {
		return cfg
	}
	if err := reencode(raw, &cfg); err != nil {
		return model.ImageProcessConfig{}
	}
	// 规范化扩展名：去点、转小写
	if len(cfg.ApplyToExtensions) > 0 {
		normalized := make([]string, 0, len(cfg.ApplyToExtensions))
		for _, ext := range cfg.ApplyToExtensions {
			ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
			if ext != "" {
				normalized = append(normalized, ext)
			}
		}
		cfg.ApplyToExtensions = normalized
	}
	return cfg
}

// parseImageStyles 从 policy.Settings 中解析 image_styles 列表。
func parseImageStyles(policy *model.StoragePolicy) []model.ImageStyleConfig {
	raw, ok := policy.Settings[constant.ImageStylesSettingsKey]
	if !ok || raw == nil {
		return nil
	}
	var styles []model.ImageStyleConfig
	if err := reencode(raw, &styles); err != nil {
		return nil
	}
	return styles
}

// findStyleByName 在策略样式列表中按名字查找，name 大小写敏感（与 URL 一致）。
func findStyleByName(policy *model.StoragePolicy, name string) (model.ImageStyleConfig, bool) {
	for _, s := range parseImageStyles(policy) {
		if s.Name == name {
			return s, true
		}
	}
	return model.ImageStyleConfig{}, false
}

// styleToResolved 将持久化的 ImageStyleConfig 转换为引擎可直接消费的 ResolvedStyle。
// 若 style.Watermark 非 nil，保留指针（不深拷贝内部字段，对热路径友好）。
func styleToResolved(s model.ImageStyleConfig) ResolvedStyle {
	return ResolvedStyle{
		Format:     s.Format,
		Quality:    s.Quality,
		AutoRotate: s.AutoRotate,
		Resize:     s.Resize,
		Watermark:  s.Watermark,
	}
}

// hasAnyDynamicOpt 判定 query 是否包含至少一个被识别的图片处理参数。
func hasAnyDynamicOpt(query url.Values) bool {
	if len(query) == 0 {
		return false
	}
	keys := []string{"w", "width", "h", "height", "fm", "format", "q", "quality", "fit", "s", "scale", "rotate"}
	for _, k := range keys {
		if _, ok := query[k]; ok {
			return true
		}
	}
	return false
}

// applyQueryOverrides 按 Spec §5.4 的映射表对 ResolvedStyle 做就地覆盖。
// Phase 1 仅解析、不校验；非法值会被丢弃（保留原值）。Phase 3 再引入严格校验。
func applyQueryOverrides(r *ResolvedStyle, query url.Values) {
	if len(query) == 0 {
		return
	}

	if v := pickFirst(query, "fm", "format"); v != "" {
		r.Format = strings.ToLower(v)
	}
	if v := pickFirst(query, "q", "quality"); v != "" {
		if q, err := strconv.Atoi(v); err == nil {
			r.Quality = q
		}
	}
	if v := pickFirst(query, "w", "width"); v != "" {
		if w, err := strconv.Atoi(v); err == nil {
			r.Resize.Width = w
		}
	}
	if v := pickFirst(query, "h", "height"); v != "" {
		if h, err := strconv.Atoi(v); err == nil {
			r.Resize.Height = h
		}
	}
	if v := pickFirst(query, "fit"); v != "" {
		// 规范化别名：inside -> fit-inside
		mode := strings.ToLower(v)
		if mode == "inside" {
			mode = "fit-inside"
		}
		r.Resize.Mode = mode
	}
	if v := pickFirst(query, "s", "scale"); v != "" {
		if s, err := strconv.ParseFloat(v, 64); err == nil {
			r.Resize.Scale = s
			if r.Resize.Mode == "" {
				r.Resize.Mode = "scale"
			}
		}
	}
	if v := pickFirst(query, "rotate"); v != "" {
		r.AutoRotate = (v == "1" || strings.EqualFold(v, "true"))
	}
}

// pickFirst 从 query 中按给定 key 列表依次寻找，返回第一个非空字符串。
func pickFirst(query url.Values, keys ...string) string {
	for _, k := range keys {
		vs := query[k]
		if len(vs) > 0 && vs[0] != "" {
			return vs[0]
		}
	}
	return ""
}

func containsString(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

// reencode 把 Settings map 中的 interface{} 值借道 JSON 转换到目标结构体。
// Settings 底层是 map[string]any，可能来自 JSON 反序列化也可能来自代码构造，
// 统一走一次 json 序列化再反序列化最为稳妥。
func reencode(src any, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
