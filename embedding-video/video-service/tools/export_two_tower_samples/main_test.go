package main

import (
	"bytes"
	"encoding/csv"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseOptionsDefaultsToLocalConfig(t *testing.T) {
	opts, err := parseOptions([]string{})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.configFile != "configs/video.yml" {
		t.Fatalf("configFile = %q, want local configs/video.yml", opts.configFile)
	}
	if opts.outputFile != "" {
		t.Fatalf("outputFile = %q, want stdout", opts.outputFile)
	}
	if opts.itemOutputFile != "" {
		t.Fatalf("itemOutputFile = %q, want empty", opts.itemOutputFile)
	}
	if opts.seedCount != 0 {
		t.Fatalf("seedCount = %d, want 0", opts.seedCount)
	}
	if opts.limit != 10000 {
		t.Fatalf("limit = %d, want 10000", opts.limit)
	}
}

func TestParseOptionsAcceptsItemOutput(t *testing.T) {
	opts, err := parseOptions([]string{"--item-output", "storage/two_tower_items.csv"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.itemOutputFile != "storage/two_tower_items.csv" {
		t.Fatalf("itemOutputFile = %q, want storage/two_tower_items.csv", opts.itemOutputFile)
	}
}

func TestParseOptionsAcceptsUserOutput(t *testing.T) {
	opts, err := parseOptions([]string{"--user-output", "storage/two_tower_users.csv"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.userOutputFile != "storage/two_tower_users.csv" {
		t.Fatalf("userOutputFile = %q, want storage/two_tower_users.csv", opts.userOutputFile)
	}
}

func TestBuildSampleRowsAssignsLabelsAndWeights(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	events := []sampleEvent{
		{UserID: 7, VideoID: 11, VideoSegmentID: 101, Source: sourceSegmentReaction, ReactionType: "double_like", EventTime: now},
		{UserID: 7, VideoID: 11, VideoSegmentID: 102, Source: sourceSegmentReaction, ReactionType: "dislike", EventTime: now},
		{UserID: 8, VideoID: 12, VideoSegmentID: 201, Source: sourceWatch, WatchDuration: 35, SegmentDuration: 40, EventTime: now},
		{UserID: 9, VideoID: 13, VideoSegmentID: 301, Source: sourceExposure, Clicked: false, Watched: false, Rank: 3, EventTime: now},
		{UserID: 10, VideoID: 14, VideoSegmentID: 401, Source: sourceVideoReaction, ReactionType: "like", EventTime: now},
		{UserID: 10, VideoID: 14, VideoSegmentID: 402, Source: sourceVideoReaction, ReactionType: "dislike", EventTime: now},
		{UserID: 11, VideoID: 15, VideoSegmentID: 501, Source: sourceExposure, Clicked: true, Watched: false, Rank: 2, EventTime: now},
		{UserID: 11, VideoID: 15, VideoSegmentID: 502, Source: sourceExposure, Clicked: true, Watched: true, Rank: 2, EventTime: now},
	}

	rows := buildSampleRows(events)
	if len(rows) != 8 {
		t.Fatalf("rows = %d, want 8", len(rows))
	}
	assertSample(t, rows[0], 1, 3.0, "double_like")
	assertSample(t, rows[1], 0, 2.0, "dislike")
	assertSample(t, rows[2], 1, 1.2, "watched")
	assertSample(t, rows[3], 0, 0.2, "exposure_no_click")
	assertSample(t, rows[4], 1, 1.0, "video_like")
	assertSample(t, rows[5], 0, 1.0, "video_dislike")
	assertSample(t, rows[6], 1, 0.8, "exposure_clicked")
	assertSample(t, rows[7], 1, 1.0, "exposure_watched")
}

func TestBuildSeedPlansUsesOnlyEligibleUsersAndSegments(t *testing.T) {
	users := []uint64{7, 8}
	segments := []eligibleSegment{
		{ID: 101, VideoID: 11, SegmentDuration: 30},
		{ID: 102, VideoID: 11, SegmentDuration: 45},
	}
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	plans, err := buildSeedPlans(users, segments, 4, rand.New(rand.NewSource(3)), now, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("buildSeedPlans returned error: %v", err)
	}
	if len(plans) != 4 {
		t.Fatalf("plans = %d, want 4", len(plans))
	}

	seen := map[[2]uint64]bool{}
	for _, plan := range plans {
		if plan.UserID != 7 && plan.UserID != 8 {
			t.Fatalf("unexpected user id: %+v", plan)
		}
		if plan.VideoSegmentID != 101 && plan.VideoSegmentID != 102 {
			t.Fatalf("unexpected segment id: %+v", plan)
		}
		if plan.VideoID != 11 {
			t.Fatalf("unexpected video id: %+v", plan)
		}
		pair := [2]uint64{plan.UserID, plan.VideoSegmentID}
		if seen[pair] {
			t.Fatalf("duplicate user/segment pair: %+v", plan)
		}
		seen[pair] = true
		if plan.EventTime.After(now) {
			t.Fatalf("future event time: %+v", plan)
		}
	}
}

func TestBuildSeedPlansClampsToAvailablePairs(t *testing.T) {
	users := []uint64{7, 8}
	segments := []eligibleSegment{
		{ID: 101, VideoID: 11, SegmentDuration: 30},
		{ID: 102, VideoID: 11, SegmentDuration: 45},
	}
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	plans, err := buildSeedPlans(users, segments, 5, rand.New(rand.NewSource(3)), now, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("buildSeedPlans returned error: %v", err)
	}
	if len(plans) != 4 {
		t.Fatalf("plans = %d, want 4", len(plans))
	}

	seen := map[[2]uint64]bool{}
	for _, plan := range plans {
		pair := [2]uint64{plan.UserID, plan.VideoSegmentID}
		if seen[pair] {
			t.Fatalf("duplicate user/segment pair: %+v", plan)
		}
		seen[pair] = true
	}
}

func TestWriteSamplesCSV(t *testing.T) {
	var buf bytes.Buffer
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	err := writeSamplesCSV(&buf, []sampleRow{{
		UserID:         7,
		VideoID:        11,
		VideoSegmentID: 101,
		Label:          1,
		Weight:         2.0,
		Source:         sourceSegmentReaction,
		Reason:         "like",
		EventTime:      now,
	}})
	if err != nil {
		t.Fatalf("writeSamplesCSV returned error: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want header + row: %q", len(records), buf.String())
	}
	if got := strings.Join(records[0], ","); got != "user_id,video_id,video_segment_id,label,weight,source,reason,event_time" {
		t.Fatalf("header = %q", got)
	}
	if records[1][0] != "7" || records[1][3] != "1" || records[1][4] != strconv.FormatFloat(2.0, 'f', 3, 64) {
		t.Fatalf("row = %+v", records[1])
	}
}

func TestWriteItemCatalogCSV(t *testing.T) {
	var buf bytes.Buffer
	err := writeItemCatalogCSV(&buf, []itemCatalogRow{{
		VideoSegmentID:  101,
		VideoID:         11,
		SegmentDuration: 45,
		VideoDuration:   300,
		LikeCount:       5,
		DoubleLikeCount: 2,
		DislikeCount:    1,
		ContentSummary:  "函数单调性",
		KnowledgeTags:   "函数|导数",
		VideoTitle:      "高一数学",
	}})
	if err != nil {
		t.Fatalf("writeItemCatalogCSV returned error: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want header + row: %q", len(records), buf.String())
	}
	if got := strings.Join(records[0], ","); got != "video_segment_id,video_id,segment_duration,video_duration,like_count,double_like_count,dislike_count,content_summary,knowledge_tags,video_title" {
		t.Fatalf("header = %q", got)
	}
	if records[1][0] != "101" || records[1][7] != "函数单调性" || records[1][8] != "函数|导数" {
		t.Fatalf("row = %+v", records[1])
	}
}

func TestWriteUserFeatureCSV(t *testing.T) {
	var buf bytes.Buffer
	err := writeUserFeatureCSV(&buf, []userFeatureRow{{
		UserID:                         7,
		GradeID:                        10,
		ClassID:                        20,
		UserType:                       1,
		MasteryAvg:                     0.72,
		MasteryMin:                     0.31,
		WeakKnowledgeCount:             3,
		StrongKnowledgeCount:           4,
		KnowledgeCorrectCount:          12,
		KnowledgeIncorrectCount:        5,
		AnswerCount:                    8,
		AnswerCorrectCount:             6,
		AnswerIncorrectCount:           2,
		AvgScoreRate:                   0.81,
		AvgCostSeconds:                 42.5,
		QuestionFeedbackCount:          2,
		GeneratedFeedbackCount:         3,
		GeneratedCorrectCount:          2,
		GeneratedAvgScoreRate:          0.76,
		QuestionSearchCount:            4,
		RecentKnowledgePointIDs:        "101|102",
		RecentSubjects:                 "math|english",
		QuestionSearchKnowledgeText:    "函数 单调性",
		GeneratedFeedbackKnowledgeText: "导数",
	}})
	if err != nil {
		t.Fatalf("writeUserFeatureCSV returned error: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want header + row: %q", len(records), buf.String())
	}
	if got := strings.Join(records[0], ","); !strings.Contains(got, "user_id,grade_id,class_id,user_type") || !strings.Contains(got, "question_search_knowledge_text") {
		t.Fatalf("header = %q", got)
	}
	if records[1][0] != "7" || records[1][1] != "10" || records[1][20] != "101|102" {
		t.Fatalf("row = %+v", records[1])
	}
}

func TestSelectOutputWritersKeepsStdoutPureCSV(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	csvOut, logOut := selectOutputWriters("", &stdout, &stderr)
	if _, err := logOut.Write([]byte("config=configs/video.yml\n")); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := writeSamplesCSV(csvOut, nil); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	if strings.Contains(stdout.String(), "config=") {
		t.Fatalf("stdout contains log output: %q", stdout.String())
	}
	if !strings.HasPrefix(stdout.String(), "user_id,video_id,video_segment_id,label,weight,source,reason,event_time\n") {
		t.Fatalf("stdout does not start with csv header: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "config=configs/video.yml") {
		t.Fatalf("stderr = %q, want log output", stderr.String())
	}
}

func TestBuildSampleEventsQueryFiltersThroughSysUser(t *testing.T) {
	query := buildSampleEventsQuery("id", map[string]columnInfo{
		"id":       {Name: "id", DataType: "bigint"},
		"deleted":  {Name: "deleted", DataType: "boolean"},
		"del_flag": {Name: "del_flag", DataType: "character varying"},
	})

	for _, fragment := range []string{
		"FROM public.sys_user u",
		`u."id" AS user_id`,
		`u."id" IS NOT NULL`,
		`u."id" > 0`,
		`u."deleted" = FALSE`,
		`COALESCE(u."del_flag"::text, '0') = '0'`,
		"JOIN valid_users vu ON vu.user_id = e.user_id",
		"FROM public.edu_video_user_reaction vur",
		"'video_reaction' AS source",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
	if strings.Contains(query, "SELECT user_id FROM public.edu_user_reaction\n  UNION") {
		t.Fatalf("query builds valid_users from behavior tables instead of sys_user:\n%s", query)
	}
}

func TestBuildItemCatalogQueryExportsAllEligibleFeatureRows(t *testing.T) {
	query := buildItemCatalogQuery()
	for _, fragment := range []string{
		"FROM public.edu_video_segment s",
		"JOIN public.edu_video_resource r ON r.id = s.video_id",
		"s.content_summary",
		"array_to_string",
		"AS video_title",
		"s.embedding IS NOT NULL",
		"s.deleted = 0",
		"s.status = 1",
		"r.deleted = 0",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
}

func TestBuildUserFeatureCatalogQueryUsesEducationSignals(t *testing.T) {
	query := buildUserFeatureCatalogQuery("id", map[string]columnInfo{
		"id":        {Name: "id", DataType: "bigint"},
		"grade_id":  {Name: "grade_id", DataType: "bigint"},
		"class_id":  {Name: "class_id", DataType: "bigint"},
		"user_type": {Name: "user_type", DataType: "smallint"},
		"deleted":   {Name: "deleted", DataType: "smallint"},
	})
	for _, fragment := range []string{
		"FROM public.sys_user u",
		"edu_user_knowledge_mastery",
		"edu_knowledge_answer_record",
		"edu_user_question_feedback",
		"edu_generated_question_feedback",
		"edu_question_search_record",
		"grade_id",
		"class_id",
		"user_type",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
}

func TestBuildUserFeatureCatalogQueryOmitsMissingOptionalColumns(t *testing.T) {
	query := buildUserFeatureCatalogQueryWithColumns("id", map[string]columnInfo{
		"id": {Name: "id", DataType: "bigint"},
	}, map[string]map[string]columnInfo{
		"edu_knowledge_answer_record": {
			"user_id": {Name: "user_id", DataType: "bigint"},
		},
		"edu_generated_question_feedback": {
			"user_id": {Name: "user_id", DataType: "bigint"},
		},
	})

	for _, fragment := range []string{
		"COUNT(*) AS answer_count",
		"0 AS answer_correct_count",
		"0 AS answer_incorrect_count",
		"0 AS avg_score_rate",
		"0 AS avg_cost_seconds",
		"'' AS recent_knowledge_point_ids",
		"COUNT(*) AS generated_feedback_count",
		"0 AS generated_correct_count",
		"0 AS generated_avg_score_rate",
		"'' AS generated_feedback_knowledge_text",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
	for _, fragment := range []string{
		"is_correct",
		`"score_rate"`,
		`"cost_seconds"`,
		`"knowledge_point_id"`,
		`"feedback_type"`,
		"edu_question_search_record",
	} {
		if strings.Contains(query, fragment) {
			t.Fatalf("query references missing column/table %q:\n%s", fragment, query)
		}
	}
}

func assertSample(t *testing.T, row sampleRow, label int, weight float64, reason string) {
	t.Helper()
	if row.Label != label || row.Weight != weight || row.Reason != reason {
		t.Fatalf("row = %+v, want label=%d weight=%v reason=%s", row, label, weight, reason)
	}
}
