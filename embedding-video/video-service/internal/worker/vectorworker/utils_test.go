package vectorworker

import "testing"

func TestFirstNonEmptyTrimsCandidates(t *testing.T) {
	got := firstNonEmpty(" ", "\t", " value ", "later")
	if got != " value " {
		t.Fatalf("firstNonEmpty() = %q", got)
	}
	if got := firstNonEmpty(" ", "\n"); got != "" {
		t.Fatalf("firstNonEmpty blank values = %q, want empty", got)
	}
}

func TestNormalizeBaseURLAddsCompatibleV1Suffix(t *testing.T) {
	got := normalizeBaseURL(`"https://dashscope.aliyuncs.com/compatible-mode"`)
	want := "https://dashscope.aliyuncs.com/compatible-mode/v1"
	if got != want {
		t.Fatalf("normalizeBaseURL() = %q, want %q", got, want)
	}

	got = normalizeBaseURL("https://example.test/v1/")
	if got != "https://example.test/v1" {
		t.Fatalf("normalizeBaseURL existing v1 = %q", got)
	}
}

func TestNormalizeASRBaseURLHandlesDashscopeForms(t *testing.T) {
	tests := map[string]string{
		"https://dashscope.aliyuncs.com/api/v1":                "https://dashscope.aliyuncs.com/api/v1",
		"https://dashscope.aliyuncs.com/compatible-mode":       "https://dashscope.aliyuncs.com/compatible-mode/v1",
		"https://dashscope.aliyuncs.com":                       "https://dashscope.aliyuncs.com/v1",
		"`https://dashscope.aliyuncs.com/compatible-mode/v1/`": "https://dashscope.aliyuncs.com/compatible-mode/v1",
		"https://example.test":                                 "https://example.test/v1",
	}
	for input, want := range tests {
		if got := normalizeASRBaseURL(input); got != want {
			t.Fatalf("normalizeASRBaseURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeTextCollapsesWhitespace(t *testing.T) {
	got := normalizeText("  第一行\r\n  第二\t行  ")
	want := "第一行 第二 行"
	if got != want {
		t.Fatalf("normalizeText() = %q, want %q", got, want)
	}
}
