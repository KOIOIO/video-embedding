package recommendation

import (
	"reflect"
	"testing"
	"time"
)

func TestMapGorseUserAggregatesLearningLabels(t *testing.T) {
	got := MapGorseUser(GorseUserSource{
		UserID:          7,
		GradeID:         8,
		ClassID:         3,
		UserType:        "student",
		RecentSubjects:  []string{"math", "physics", "math"},
		RecentKnowledge: []string{"函数", "力学"},
		LearningLabels:  []string{"mastery:weak", "answer:low_accuracy", "mastery:weak"},
	})

	if got.UserID != "7" {
		t.Fatalf("UserID = %q, want 7", got.UserID)
	}
	wantLabels := []string{"grade:8", "class:3", "type:student", "subject:math", "subject:physics", "knowledge:函数", "knowledge:力学", "mastery:weak", "answer:low_accuracy"}
	if !reflect.DeepEqual(got.Labels, wantLabels) {
		t.Fatalf("Labels = %#v, want %#v", got.Labels, wantLabels)
	}
}

func TestMapGorseItemUsesCategoriesAndHiddenFlag(t *testing.T) {
	publishedAt := time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC)
	got := MapGorseItem(GorseItemSource{
		VideoSegmentID:  101,
		VideoID:         11,
		Title:           "函数课",
		Summary:         "一次函数",
		Subject:         "math",
		KnowledgeTags:   []string{"函数", "一次函数"},
		DurationSec:     120,
		ViewCount:       9,
		LikeCount:       2,
		DoubleLikeCount: 1,
		DislikeCount:    0,
		Embedding:       []float32{0.1, 0.2},
		IsDeleted:       false,
		IsPublished:     true,
		IsPlayable:      true,
		IsRecommend:     true,
		PublishedAt:     publishedAt,
	})

	if got.ItemID != "101" || got.Timestamp != publishedAt {
		t.Fatalf("item identity = %+v", got)
	}
	if got.IsHidden {
		t.Fatal("IsHidden = true, want false")
	}
	wantCategories := []string{"subject:math", "knowledge:函数", "knowledge:一次函数"}
	if !reflect.DeepEqual(got.Categories, wantCategories) {
		t.Fatalf("Categories = %#v, want %#v", got.Categories, wantCategories)
	}
	if got.Labels["video_id"] != "11" || got.Labels["title"] != "函数课" || got.Labels["duration_sec"] != float64(120) {
		t.Fatalf("Labels = %#v", got.Labels)
	}
	if !reflect.DeepEqual(got.Labels["embedding"], []float32{0.1, 0.2}) {
		t.Fatalf("embedding label = %#v", got.Labels["embedding"])
	}
}

func TestMapGorseItemHidesUnavailableSegments(t *testing.T) {
	tests := []GorseItemSource{
		{VideoSegmentID: 1, IsDeleted: true, IsPublished: true, IsPlayable: true, IsRecommend: true},
		{VideoSegmentID: 2, IsDeleted: false, IsPublished: false, IsPlayable: true, IsRecommend: true},
		{VideoSegmentID: 3, IsDeleted: false, IsPublished: true, IsPlayable: false, IsRecommend: true},
	}
	for _, tc := range tests {
		got := MapGorseItem(tc)
		if !got.IsHidden {
			t.Fatalf("source %+v mapped IsHidden=false, want true", tc)
		}
	}
}

func TestMapGorseItemKeepsPlayableNonRecommendedSegmentsVisible(t *testing.T) {
	got := MapGorseItem(GorseItemSource{
		VideoSegmentID: 4,
		IsDeleted:      false,
		IsPublished:    true,
		IsPlayable:     true,
		IsRecommend:    false,
	})

	if got.IsHidden {
		t.Fatal("IsHidden = true, want false")
	}
}

func TestMapGorseFeedbackPreservesExplicitSignalSemantics(t *testing.T) {
	now := time.Date(2026, 6, 26, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		src  GorseFeedbackSource
		want GorseFeedback
	}{
		{
			name: "double like",
			src:  GorseFeedbackSource{UserID: 7, VideoSegmentID: 101, Kind: GorseFeedbackDoubleLike, EventTime: now},
			want: GorseFeedback{FeedbackType: "double_like", UserID: "7", ItemID: "101", Timestamp: now, Value: 3},
		},
		{
			name: "like",
			src:  GorseFeedbackSource{UserID: 7, VideoSegmentID: 101, Kind: GorseFeedbackLike, EventTime: now},
			want: GorseFeedback{FeedbackType: "like", UserID: "7", ItemID: "101", Timestamp: now, Value: 2},
		},
		{
			name: "watch",
			src:  GorseFeedbackSource{UserID: 7, VideoSegmentID: 101, Kind: GorseFeedbackWatch, WatchRatio: 0.75, EventTime: now},
			want: GorseFeedback{FeedbackType: "watch", UserID: "7", ItemID: "101", Timestamp: now, Value: 0.75},
		},
		{
			name: "exposure",
			src:  GorseFeedbackSource{UserID: 7, VideoSegmentID: 101, Kind: GorseFeedbackExposure, EventTime: now},
			want: GorseFeedback{FeedbackType: "exposure", UserID: "7", ItemID: "101", Timestamp: now, Value: 1},
		},
		{
			name: "dislike",
			src:  GorseFeedbackSource{UserID: 7, VideoSegmentID: 101, Kind: GorseFeedbackDislike, EventTime: now},
			want: GorseFeedback{FeedbackType: "dislike", UserID: "7", ItemID: "101", Timestamp: now, Value: 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := MapGorseFeedback(tc.src)
			if !ok {
				t.Fatal("MapGorseFeedback returned ok=false")
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("feedback = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestMapGorseFeedbackDropsInvalidExposureNoClickNegative(t *testing.T) {
	got, ok := MapGorseFeedback(GorseFeedbackSource{
		UserID:         7,
		VideoSegmentID: 101,
		Kind:           GorseFeedbackExposureNoClick,
	})
	if ok || got.FeedbackType != "" {
		t.Fatalf("exposure no click mapped to %+v ok=%v, want omitted", got, ok)
	}
}
