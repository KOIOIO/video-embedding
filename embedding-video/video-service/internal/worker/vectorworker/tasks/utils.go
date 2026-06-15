package tasks

import (
	"strings"
	"unicode"
)

// normalizeEmbeddingDim 标准化嵌入向量维度
func normalizeEmbeddingDim(v []float32, dim int) []float32 {
	if dim <= 0 {
		return v
	}
	if len(v) == dim {
		return v
	}
	if len(v) == 0 {
		return v
	}
	if len(v) > dim {
		return v[:dim]
	}
	out := make([]float32, dim)
	copy(out, v)
	return out
}

// normalizeTags 标准化标签
func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return tags
	}
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		t = strings.ToLower(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// NormalizeText 标准化文本
func NormalizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// 转换为小写
	s = strings.ToLower(s)
	// 移除多余的空格
	s = strings.Join(strings.Fields(s), " ")
	// 移除控制字符
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ExtractFirstJSONObject 提取第一个 JSON 对象
func ExtractFirstJSONObject(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	start := strings.Index(s, "{")
	if start == -1 {
		return "", false
	}
	end := -1
	braceCount := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			braceCount++
		} else if s[i] == '}' {
			braceCount--
			if braceCount == 0 {
				end = i + 1
				break
			}
		}
	}
	if end == -1 {
		return "", false
	}
	return s[start:end], true
}

// NormalizeTags 标准化标签
func NormalizeTags(tags []string) []string {
	return normalizeTags(tags)
}

// MergeTags 合并标签
func MergeTags(tags1, tags2 []string) []string {
	tags1 = normalizeTags(tags1)
	tags2 = normalizeTags(tags2)
	merged := make([]string, 0, len(tags1)+len(tags2))
	seen := make(map[string]struct{}, len(tags1)+len(tags2))
	for _, t := range tags1 {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			merged = append(merged, t)
		}
	}
	for _, t := range tags2 {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			merged = append(merged, t)
		}
	}
	return merged
}

// BuildSampleOffsets 构建采样偏移
func BuildSampleOffsets(durationSec int, sampleCount int) []int {
	if durationSec <= 0 || sampleCount <= 0 {
		return nil
	}
	if sampleCount == 1 {
		return []int{durationSec / 2}
	}
	step := durationSec / (sampleCount + 1)
	offsets := make([]int, sampleCount)
	for i := 0; i < sampleCount; i++ {
		offsets[i] = (i + 1) * step
	}
	return offsets
}

// NormalizeEmbeddingDim 标准化嵌入向量维度
func NormalizeEmbeddingDim(vec []float32, dim int) []float32 {
	return normalizeEmbeddingDim(vec, dim)
}
