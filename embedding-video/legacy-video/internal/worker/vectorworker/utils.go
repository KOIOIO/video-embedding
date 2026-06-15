package vectorworker

import (
	"strings"
)

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normalizeBaseURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"'")
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}
	if strings.HasSuffix(s, "/v1") {
		return s
	}
	return s + "/v1"
}

func normalizeASRBaseURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"'")
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}
	if strings.Contains(s, "/api/v1") {
		return s
	}
	if strings.Contains(s, "/compatible-mode") {
		return normalizeBaseURL(s)
	}
	if strings.Contains(s, "dashscope.aliyuncs.com") {
		return s + "/v1"
	}
	return normalizeBaseURL(s)
}

func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	parts := strings.Fields(s)
	return strings.TrimSpace(strings.Join(parts, " "))
}
