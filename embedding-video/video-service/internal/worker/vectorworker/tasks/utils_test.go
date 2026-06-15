package tasks

import "testing"

func TestNormalizeEmbeddingDimPadsAndTruncates(t *testing.T) {
	if got := NormalizeEmbeddingDim([]float32{1, 2, 3}, 2); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("truncate result = %#v", got)
	}

	got := NormalizeEmbeddingDim([]float32{1, 2}, 4)
	if len(got) != 4 || got[0] != 1 || got[1] != 2 || got[2] != 0 || got[3] != 0 {
		t.Fatalf("pad result = %#v", got)
	}

	empty := NormalizeEmbeddingDim(nil, 4)
	if empty != nil {
		t.Fatalf("nil vector normalized to %#v, want nil", empty)
	}
}

func TestNormalizeTagsTrimsLowercasesAndDeduplicates(t *testing.T) {
	got := NormalizeTags([]string{" Algebra ", "algebra", "", "Geometry"})
	want := []string{"algebra", "geometry"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeTags len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeTags[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractFirstJSONObjectReturnsBalancedObject(t *testing.T) {
	got, ok := ExtractFirstJSONObject("prefix {\"a\":{\"b\":1}} suffix")
	if !ok {
		t.Fatal("expected object to be found")
	}
	if got != "{\"a\":{\"b\":1}}" {
		t.Fatalf("ExtractFirstJSONObject() = %q", got)
	}

	if _, ok := ExtractFirstJSONObject("prefix {\"a\":1"); ok {
		t.Fatal("expected unbalanced object to fail")
	}
}

func TestBuildSampleOffsetsSpreadsSamples(t *testing.T) {
	got := BuildSampleOffsets(100, 4)
	want := []int{20, 40, 60, 80}
	if len(got) != len(want) {
		t.Fatalf("offset len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("offset[%d] = %d, want %d", i, got[i], want[i])
		}
	}

	one := BuildSampleOffsets(99, 1)
	if len(one) != 1 || one[0] != 49 {
		t.Fatalf("single offset = %#v, want [49]", one)
	}
	if got := BuildSampleOffsets(0, 4); got != nil {
		t.Fatalf("invalid duration offsets = %#v, want nil", got)
	}
}

func TestMergeTagsPreservesFirstSeenOrder(t *testing.T) {
	got := MergeTags([]string{" Algebra ", "geometry"}, []string{"algebra", "calculus"})
	want := []string{"algebra", "geometry", "calculus"}
	if len(got) != len(want) {
		t.Fatalf("MergeTags len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MergeTags[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
