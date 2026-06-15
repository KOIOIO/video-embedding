package model

import "testing"

func TestTextArrayValueEscapesPostgresArrayElements(t *testing.T) {
	value, err := TextArray{`algebra`, `quote"tag`, `path\tag`}.Value()
	if err != nil {
		t.Fatalf("Value returned error: %v", err)
	}

	want := `{"algebra","quote\"tag","path\\tag"}`
	if value != want {
		t.Fatalf("Value() = %q, want %q", value, want)
	}
}

func TestTextArrayScanParsesQuotedElements(t *testing.T) {
	var got TextArray
	if err := got.Scan(`{"algebra","quote\"tag","path\\tag"}`); err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	want := TextArray{"algebra", `quote"tag`, `path\tag`}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTextArrayScanHandlesEmptyAndNil(t *testing.T) {
	var got TextArray
	if err := got.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Scan(nil) = %#v, want nil", got)
	}

	if err := got.Scan("{}"); err != nil {
		t.Fatalf("Scan({}) returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Scan({}) len = %d, want 0", len(got))
	}
}

func TestTextArrayScanRejectsInvalidInput(t *testing.T) {
	var got TextArray
	if err := got.Scan(123); err == nil {
		t.Fatal("expected unsupported input type to fail")
	}
	if err := got.Scan("not-array"); err == nil {
		t.Fatal("expected invalid array literal to fail")
	}

	var nilReceiver *TextArray
	if err := nilReceiver.Scan("{}"); err == nil {
		t.Fatal("expected nil receiver to fail")
	}
}

func TestVideoModelTableNames(t *testing.T) {
	if got := (EduVideoResource{}).TableName(); got != "edu_video_resource" {
		t.Fatalf("EduVideoResource.TableName() = %q", got)
	}
	if got := (EduVideoUserReaction{}).TableName(); got != "edu_video_user_reaction" {
		t.Fatalf("EduVideoUserReaction.TableName() = %q", got)
	}
	if got := (EduUserReaction{}).TableName(); got != "edu_user_reaction" {
		t.Fatalf("EduUserReaction.TableName() = %q", got)
	}
	if got := (EduVideoSegment{}).TableName(); got != "edu_video_segment" {
		t.Fatalf("EduVideoSegment.TableName() = %q", got)
	}
	if got := (EduUserVideoRecommend{}).TableName(); got != "edu_user_video_recommend" {
		t.Fatalf("EduUserVideoRecommend.TableName() = %q", got)
	}
	if got := (EduVideoVectorStage{}).TableName(); got != "edu_video_vector_stage" {
		t.Fatalf("EduVideoVectorStage.TableName() = %q", got)
	}
}
