package tasks

import "testing"

func TestNormalizeTailAlignmentConfigDefaults(t *testing.T) {
	got := NormalizeTailAlignmentConfig(TailAlignmentConfig{})
	if got.Enabled {
		t.Fatalf("Enabled = true, want false before startup defaulting")
	}
	if got.MaxExtendSec != 3 {
		t.Fatalf("MaxExtendSec = %d, want 3", got.MaxExtendSec)
	}
	if got.ProbeStepSec != 1 {
		t.Fatalf("ProbeStepSec = %d, want 1", got.ProbeStepSec)
	}
	if got.MaxOverlapSec != 6 {
		t.Fatalf("MaxOverlapSec = %d, want 6", got.MaxOverlapSec)
	}
}

func TestNormalizeTailAlignmentConfigKeepsExplicitValues(t *testing.T) {
	got := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       false,
		MaxExtendSec:  5,
		ProbeStepSec:  2,
		MaxOverlapSec: 4,
	})
	if got.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if got.MaxExtendSec != 5 || got.ProbeStepSec != 2 || got.MaxOverlapSec != 4 {
		t.Fatalf("unexpected config: %+v", got)
	}
}

func TestLooksLikeSentenceEnd(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "punctuation", text: "所以这一步就完成了。", want: true},
		{name: "closing phrase", text: "这一题我们就讲到这里", want: true},
		{name: "connector tail", text: "接下来我们来看", want: false},
		{name: "half sentence", text: "所以这里我们可以得到", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LooksLikeSentenceEnd(tt.text)
			if got != tt.want {
				t.Fatalf("LooksLikeSentenceEnd(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestNeedsTailExtension(t *testing.T) {
	if !NeedsTailExtension("所以这里我们可以得到") {
		t.Fatal("NeedsTailExtension returned false for half sentence")
	}
	if NeedsTailExtension("所以这里我们可以得到结论。") {
		t.Fatal("NeedsTailExtension returned true for complete sentence")
	}
}

func TestNextAlignedEndSec(t *testing.T) {
	cfg := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       true,
		MaxExtendSec:  3,
		ProbeStepSec:  1,
		MaxOverlapSec: 2,
	})

	tests := []struct {
		name             string
		currentEndSec    int
		originalEndSec   int
		nextSegmentStart int
		videoDurationSec int
		want             int
	}{
		{name: "normal step", currentEndSec: 10, originalEndSec: 10, nextSegmentStart: 20, videoDurationSec: 60, want: 11},
		{name: "extend limit", currentEndSec: 13, originalEndSec: 10, nextSegmentStart: 20, videoDurationSec: 60, want: 13},
		{name: "overlap limit", currentEndSec: 10, originalEndSec: 10, nextSegmentStart: 11, videoDurationSec: 60, want: 11},
		{name: "duration limit", currentEndSec: 10, originalEndSec: 10, nextSegmentStart: 40, videoDurationSec: 11, want: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextAlignedEndSec(tt.currentEndSec, tt.originalEndSec, tt.nextSegmentStart, tt.videoDurationSec, cfg)
			if got != tt.want {
				t.Fatalf("NextAlignedEndSec() = %d, want %d", got, tt.want)
			}
		})
	}
}
