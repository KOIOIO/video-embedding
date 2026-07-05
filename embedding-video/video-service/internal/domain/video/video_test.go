package video

import (
	"testing"
	"time"
)

func TestNewUploadedInitializesDefaults(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)

	v, err := NewUploaded(" lesson 1 ", "desc", "raw/lesson.mp4", 1, now)
	if err != nil {
		t.Fatalf("NewUploaded returned error: %v", err)
	}

	if v.Title != " lesson 1 " {
		t.Fatalf("Title = %q", v.Title)
	}
	if v.UserID != 1 {
		t.Fatalf("UserID = %d, want 1", v.UserID)
	}
	if v.Status != StatusUploaded {
		t.Fatalf("Status = %v, want %v", v.Status, StatusUploaded)
	}
	if !v.IsPublished {
		t.Fatal("expected uploaded video to be published by default")
	}
	if v.IsRecommend {
		t.Fatal("expected uploaded video not to be recommended by default")
	}
	if !v.CreateTime.Equal(now) || !v.UpdateTime.Equal(now) {
		t.Fatalf("times = %v/%v, want %v", v.CreateTime, v.UpdateTime, now)
	}
}

func TestNewUploadedRejectsMissingRequiredFields(t *testing.T) {
	now := time.Now()

	if _, err := NewUploaded(" ", "desc", "raw.mp4", 1, now); err == nil {
		t.Fatal("expected missing title to fail")
	}
	if _, err := NewUploaded("title", "desc", " ", 1, now); err == nil {
		t.Fatal("expected missing video url to fail")
	}
	if _, err := NewUploaded("title", "desc", "raw.mp4", 0, now); err == nil {
		t.Fatal("expected missing user id to fail")
	}
}

func TestVideoStatusTransitions(t *testing.T) {
	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	v, err := NewUploaded("title", "desc", "raw.mp4", 1, now)
	if err != nil {
		t.Fatalf("NewUploaded returned error: %v", err)
	}

	processingAt := now.Add(time.Minute)
	if err := v.MarkProcessing(processingAt); err != nil {
		t.Fatalf("MarkProcessing returned error: %v", err)
	}
	if v.Status != StatusProcessing || !v.UpdateTime.Equal(processingAt) {
		t.Fatalf("processing state = %v at %v", v.Status, v.UpdateTime)
	}

	v.ErrorMsg = "previous"
	doneAt := processingAt.Add(time.Minute)
	if err := v.MarkDone(doneAt); err != nil {
		t.Fatalf("MarkDone returned error: %v", err)
	}
	if v.Status != StatusDone || v.ErrorMsg != "" || !v.UpdateTime.Equal(doneAt) {
		t.Fatalf("done state = status %v err %q at %v", v.Status, v.ErrorMsg, v.UpdateTime)
	}
}

func TestVideoRejectsInvalidStatusTransitions(t *testing.T) {
	now := time.Now()
	v, err := NewUploaded("title", "desc", "raw.mp4", 1, now)
	if err != nil {
		t.Fatalf("NewUploaded returned error: %v", err)
	}

	if err := v.MarkDone(now.Add(time.Minute)); err == nil {
		t.Fatal("expected uploaded -> done to fail")
	}
	if err := v.MarkFailed("bad media", now.Add(time.Minute)); err != nil {
		t.Fatalf("uploaded -> failed should be allowed: %v", err)
	}
	if v.Status != StatusFailed || v.ErrorMsg != "bad media" {
		t.Fatalf("failed state = %v %q", v.Status, v.ErrorMsg)
	}
	if err := v.MarkProcessing(now.Add(2 * time.Minute)); err == nil {
		t.Fatal("expected failed -> processing to fail")
	}
}
