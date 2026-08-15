package main

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

func TestParseOptionsRequiresOutputDir(t *testing.T) {
	_, err := parseOptions([]string{})
	if err == nil {
		t.Fatal("parseOptions returned nil error, want output-dir required")
	}
}

func TestParseOptionsDefaultsAndValidation(t *testing.T) {
	opts, err := parseOptions([]string{"--output-dir", "data/recbole"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.dataset != "video_app" {
		t.Fatalf("dataset = %q, want video_app", opts.dataset)
	}
	if opts.limit != 10000 {
		t.Fatalf("limit = %d, want 10000", opts.limit)
	}
	if opts.daysBack != 30 {
		t.Fatalf("daysBack = %d, want 30", opts.daysBack)
	}
	if _, err := parseOptions([]string{"--output-dir", "x", "--limit", "0"}); err == nil {
		t.Fatal("limit=0 returned nil error")
	}
	if _, err := parseOptions([]string{"--output-dir", "x", "--days-back", "0"}); err == nil {
		t.Fatal("days-back=0 returned nil error")
	}
}

func TestWriteInteractionsUsesRecBoleAtomicHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := writeInteractions(&buf, nil); err != nil {
		t.Fatalf("writeInteractions returned error: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	got := strings.Join(records[0], ",")
	want := "user_id:token,item_id:token,rating:float,timestamp:float,source:token,weight:float"
	if got != want {
		t.Fatalf("header = %q, want %q", got, want)
	}
}

func TestBuildInteractionQueryUsesVideoAndQuestionSearchSignals(t *testing.T) {
	query := buildInteractionQuery()
	for _, fragment := range []string{
		"edu_user_reaction",
		"edu_video_user_reaction",
		"edu_user_video_recommend",
		"edu_recommend_exposure",
		"edu_question_search_record",
		"recommend_videos_json",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
}

func TestBuildInteractionQueryUsesPortableQuestionSearchFields(t *testing.T) {
	query := buildInteractionQuery()
	for _, fragment := range []string{
		"qs.create_time AS event_time",
		"videoSegmentId",
		"watchDuration",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
	if strings.Contains(query, "qsr.update_time") {
		t.Fatalf("query references missing qsr.update_time column:\n%s", query)
	}
}

func TestBuildUserFeatureQueryUsesFullSystemSignals(t *testing.T) {
	query := buildUserFeatureQuery()
	for _, fragment := range []string{
		"sys_user",
		"edu_user_knowledge_mastery",
		"edu_knowledge_answer_record",
		"edu_user_question_feedback",
		"edu_generated_question_feedback",
		"edu_question_search_record",
		"edu_special_practice_session",
		"edu_student_word_record",
		"english_reading_history",
		"english_listening_session",
		"english_storybook_session",
		"student_profile_snapshot",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
}

func TestBuildUserFeatureQueryUsesProfileSnapshotSchema(t *testing.T) {
	query := buildUserFeatureQuery()
	for _, fragment := range []string{
		"student_id AS user_id",
		"MAX(profile_json::text)",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
	if strings.Contains(query, "snapshot_json") {
		t.Fatalf("query references missing snapshot_json column:\n%s", query)
	}
}

func TestInteractionFromEventMapsRatingsConservatively(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		event      interactionEvent
		wantRating float64
	}{
		{
			name: "double like",
			event: interactionEvent{
				UserID: 7, VideoSegmentID: 101, Source: "segment_reaction", ReactionType: "double_like", EventTime: now,
			},
			wantRating: 3.0,
		},
		{
			name: "like",
			event: interactionEvent{
				UserID: 7, VideoSegmentID: 101, Source: "segment_reaction", ReactionType: "like", EventTime: now,
			},
			wantRating: 2.0,
		},
		{
			name: "sufficient watch",
			event: interactionEvent{
				UserID: 7, VideoSegmentID: 101, Source: "watch", Watched: true, WatchDuration: 60, SegmentDuration: 100, EventTime: now,
			},
			wantRating: 1.5,
		},
		{
			name: "question search watch",
			event: interactionEvent{
				UserID: 7, VideoSegmentID: 101, Source: "question_search_watch", Watched: true, WatchDuration: 60, SegmentDuration: 100, EventTime: now,
			},
			wantRating: 1.5,
		},
		{
			name: "short watch",
			event: interactionEvent{
				UserID: 7, VideoSegmentID: 101, Source: "watch", Watched: true, WatchDuration: 10, SegmentDuration: 100, EventTime: now,
			},
			wantRating: 0.3,
		},
		{
			name: "exposure only",
			event: interactionEvent{
				UserID: 7, VideoSegmentID: 101, Source: "exposure", EventTime: now,
			},
			wantRating: 0.1,
		},
		{
			name: "dislike",
			event: interactionEvent{
				UserID: 7, VideoSegmentID: 101, Source: "segment_reaction", ReactionType: "dislike", EventTime: now,
			},
			wantRating: 0.0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row, ok := interactionFromEvent(tc.event)
			if !ok {
				t.Fatal("interactionFromEvent returned ok=false")
			}
			if row.Rating != tc.wantRating {
				t.Fatalf("rating = %v, want %v", row.Rating, tc.wantRating)
			}
		})
	}
}

func TestBuildInteractionRowsKeepsHigherValueEventPerUserAndSegment(t *testing.T) {
	older := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	events := []interactionEvent{
		{
			UserID: 7, VideoSegmentID: 101, Source: "segment_reaction", ReactionType: "like", EventTime: older,
		},
		{
			UserID: 7, VideoSegmentID: 101, Source: "exposure", EventTime: newer,
		},
		{
			UserID: 7, VideoSegmentID: 102, Source: "segment_reaction", ReactionType: "double_like", EventTime: older,
		},
	}

	rows := buildInteractionRows(events)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2: %+v", len(rows), rows)
	}
	if rows[0].VideoSegmentID != 101 || rows[0].Source != "segment_reaction" || rows[0].Timestamp != float64(older.Unix()) {
		t.Fatalf("rows[0] = %+v, want higher-value event for user 7 segment 101", rows[0])
	}
	if rows[1].VideoSegmentID != 102 {
		t.Fatalf("rows[1] = %+v, want user 7 segment 102", rows[1])
	}
}

func TestBuildKnowledgeWatchRowsIncludesOnlyEffectiveAggregates(t *testing.T) {
	rows, stats := buildKnowledgeWatchRows([]knowledgeWatchSession{
		{UserID: 7, KnowledgeVideoID: 88, SessionID: "a", Duration: 100, WatchedSeconds: 20, UpdatedAt: time.Unix(100, 0)},
		{UserID: 7, KnowledgeVideoID: 88, SessionID: "b", Duration: 100, WatchedSeconds: 40, UpdatedAt: time.Unix(200, 0)},
		{UserID: 8, KnowledgeVideoID: 99, Duration: 1000, WatchedSeconds: 599, UpdatedAt: time.Unix(300, 0)},
	})
	if len(rows) != 1 || rows[0].ItemID != "knowledge_video:88" || rows[0].Source != "knowledge_video_watch" {
		t.Fatalf("rows = %+v", rows)
	}
	if stats.SessionRows != 3 || stats.EffectiveAggregates != 1 || stats.ShortAggregates != 1 || stats.EffectiveUsers != 1 || stats.EffectiveVideos != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestBenchmarkSplitKeepsVirtualItemsInTrainOnly(t *testing.T) {
	rows := []interactionRow{
		{UserID: 7, ItemID: "101", Timestamp: 100},
		{UserID: 7, ItemID: "102", Timestamp: 200},
		{UserID: 7, ItemID: "knowledge_video:88", Timestamp: 250},
		{UserID: 7, ItemID: "103", Timestamp: 300},
	}
	train, valid, test := benchmarkSplit(rows)
	if !containsInteractionItem(train, "knowledge_video:88") {
		t.Fatalf("train = %+v", train)
	}
	if containsInteractionItem(valid, "knowledge_video:88") || containsInteractionItem(test, "knowledge_video:88") {
		t.Fatalf("virtual item leaked: valid=%+v test=%+v", valid, test)
	}
	if !containsInteractionItem(valid, "102") || !containsInteractionItem(test, "103") {
		t.Fatalf("normal split invalid: train=%+v valid=%+v test=%+v", train, valid, test)
	}
}

func containsInteractionItem(rows []interactionRow, itemID string) bool {
	for _, row := range rows {
		if row.ItemID == itemID {
			return true
		}
	}
	return false
}
