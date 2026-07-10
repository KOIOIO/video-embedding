package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"nlp-video-analysis/internal/config"
)

const (
	defaultConfigPath = "configs/video.yml"
	defaultDataset    = "video_dataset"
	defaultLimit      = 10000
	defaultDaysBack   = 30
)

type options struct {
	configFile string
	outputDir  string
	dataset    string
	limit      int
	daysBack   int
}

type interactionEvent struct {
	UserID          uint64
	VideoID         uint64
	VideoSegmentID  uint64
	Source          string
	ReactionType    string
	Clicked         bool
	Watched         bool
	WatchDuration   int
	SegmentDuration int
	EventTime       time.Time
}

type interactionRow struct {
	UserID         uint64
	VideoSegmentID uint64
	Rating         float64
	Timestamp      float64
	Source         string
	Weight         float64
}

type itemRow struct {
	VideoSegmentID  uint64
	VideoID         uint64
	SegmentDuration int
	VideoDuration   int
	LikeCount       int
	DoubleLikeCount int
	DislikeCount    int
	ContentSummary  string
	KnowledgeTags   string
	VideoTitle      string
}

type userFeatureRow struct {
	UserID                  uint64
	GradeID                 uint64
	ClassID                 uint64
	UserType                int
	MasteryAvg              float64
	WeakKnowledgeCount      int
	AnswerCount             int
	AnswerCorrectCount      int
	QuestionFeedbackCount   int
	GeneratedFeedbackCount  int
	QuestionSearchCount     int
	SpecialPracticeCount    int
	StudentWordCount        int
	EnglishReadingCount     int
	EnglishListeningCount   int
	EnglishStorybookCount   int
	ProfileSnapshotFeatures string
	QuestionSearchKnowledge string
	RecentKnowledgePointIDs string
}

func main() {
	config.EnsureProjectRoot()
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{
		configFile: defaultConfigPath,
		dataset:    defaultDataset,
		limit:      defaultLimit,
		daysBack:   defaultDaysBack,
	}
	fs := flag.NewFlagSet("export_recbole_dataset", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configFile, "config", opts.configFile, "config file used for PostgreSQL DSN")
	fs.StringVar(&opts.outputDir, "output-dir", "", "directory for RecBole atomic files")
	fs.StringVar(&opts.dataset, "dataset", opts.dataset, "RecBole dataset name")
	fs.IntVar(&opts.limit, "limit", opts.limit, "maximum exported interaction rows")
	fs.IntVar(&opts.daysBack, "days-back", opts.daysBack, "interaction lookback window")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.configFile = strings.TrimSpace(opts.configFile)
	opts.outputDir = strings.TrimSpace(opts.outputDir)
	opts.dataset = strings.TrimSpace(opts.dataset)
	if opts.configFile == "" {
		return options{}, errors.New("config is required")
	}
	if opts.outputDir == "" {
		return options{}, errors.New("output-dir is required")
	}
	if opts.dataset == "" {
		return options{}, errors.New("dataset is required")
	}
	if opts.limit <= 0 {
		return options{}, errors.New("limit must be greater than 0")
	}
	if opts.daysBack <= 0 {
		return options{}, errors.New("days-back must be greater than 0")
	}
	return opts, nil
}

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	cfg := config.MustLoad(opts.configFile)
	if strings.TrimSpace(cfg.Postgres.DSN) == "" {
		return errors.New("postgres dsn is required")
	}
	db, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := os.MkdirAll(opts.outputDir, 0o755); err != nil {
		return err
	}
	events, err := loadInteractionEvents(ctx, db, opts.limit, opts.daysBack)
	if err != nil {
		return err
	}
	interactions := buildInteractionRows(events)
	items, err := loadItems(ctx, db)
	if err != nil {
		return err
	}
	users, err := loadUserFeatures(ctx, db)
	if err != nil {
		return err
	}

	if err := writeAtomicFile(filepath.Join(opts.outputDir, opts.dataset+".inter"), func(w io.Writer) error {
		return writeInteractions(w, interactions)
	}); err != nil {
		return err
	}
	if err := writeAtomicFile(filepath.Join(opts.outputDir, opts.dataset+".item"), func(w io.Writer) error {
		return writeItems(w, items)
	}); err != nil {
		return err
	}
	if err := writeAtomicFile(filepath.Join(opts.outputDir, opts.dataset+".user"), func(w io.Writer) error {
		return writeUsers(w, users)
	}); err != nil {
		return err
	}
	fmt.Fprintf(out, "dataset=%s interactions=%d items=%d users=%d output_dir=%s\n", opts.dataset, len(interactions), len(items), len(users), opts.outputDir)
	return nil
}

func openDB(cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		return nil, err
	}
	if cfg.Postgres.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
	}
	if cfg.Postgres.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)
	}
	if cfg.Postgres.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.Postgres.ConnMaxLifetime) * time.Second)
	}
	if cfg.Postgres.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(cfg.Postgres.ConnMaxIdleTime) * time.Second)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return db, nil
}

func loadInteractionEvents(ctx context.Context, db *sql.DB, limit int, daysBack int) ([]interactionEvent, error) {
	rows, err := db.QueryContext(ctx, buildInteractionQuery(), limit, daysBack)
	if err != nil {
		return nil, fmt.Errorf("query interaction events: %w", err)
	}
	defer rows.Close()
	events := make([]interactionEvent, 0, limit)
	for rows.Next() {
		var event interactionEvent
		if err := rows.Scan(
			&event.UserID,
			&event.VideoID,
			&event.VideoSegmentID,
			&event.Source,
			&event.ReactionType,
			&event.Clicked,
			&event.Watched,
			&event.WatchDuration,
			&event.SegmentDuration,
			&event.EventTime,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func buildInteractionQuery() string {
	return `
WITH valid_segments AS (
  SELECT s.id AS video_segment_id,
         s.video_id,
         GREATEST(COALESCE(s.end_time, 0) - COALESCE(s.start_time, 0), 1) AS segment_duration
  FROM public.edu_video_segment s
  JOIN public.edu_video_resource r ON r.id = s.video_id
  WHERE s.deleted = 0
    AND s.status = 1
    AND s.id > 0
    AND s.video_id > 0
    AND r.deleted = 0
),
events AS (
  SELECT ur.user_id, vs.video_id, ur.video_segment_id, 'segment_reaction' AS source,
         ur.reaction_type, FALSE AS clicked, FALSE AS watched, 0 AS watch_duration,
         vs.segment_duration, ur.update_time AS event_time
  FROM public.edu_user_reaction ur
  JOIN valid_segments vs ON vs.video_segment_id = ur.video_segment_id AND vs.video_id = ur.video_id
  WHERE ur.deleted = 0
    AND ur.reaction_type IN ('like', 'double_like', 'dislike')

  UNION ALL

  SELECT vur.user_id, vs.video_id, vs.video_segment_id, 'video_reaction' AS source,
         vur.reaction_type, FALSE AS clicked, FALSE AS watched, 0 AS watch_duration,
         vs.segment_duration, vur.update_time AS event_time
  FROM public.edu_video_user_reaction vur
  JOIN valid_segments vs ON vs.video_id = vur.video_id
  WHERE vur.deleted = 0
    AND vur.reaction_type IN ('like', 'double_like', 'dislike')

  UNION ALL

  SELECT uvr.user_id, vs.video_id, uvr.video_segment_id, 'watch' AS source,
         '' AS reaction_type, TRUE AS clicked, COALESCE(uvr.is_watched, FALSE) AS watched,
         COALESCE(uvr.watch_duration, 0) AS watch_duration, vs.segment_duration,
         uvr.update_time AS event_time
  FROM public.edu_user_video_recommend uvr
  JOIN valid_segments vs ON vs.video_segment_id = uvr.video_segment_id AND vs.video_id = uvr.video_id
  WHERE uvr.deleted = 0

  UNION ALL

  SELECT e.user_id, vs.video_id, e.video_segment_id, 'exposure' AS source,
         '' AS reaction_type, COALESCE(e.clicked, FALSE) AS clicked, COALESCE(e.watched, FALSE) AS watched,
         0 AS watch_duration, vs.segment_duration, e.create_time AS event_time
  FROM public.edu_recommend_exposure e
  JOIN valid_segments vs ON vs.video_segment_id = e.video_segment_id AND vs.video_id = e.video_id
  WHERE e.deleted = 0

  UNION ALL

  SELECT qs.user_id,
         vs.video_id,
         extracted.video_segment_id AS video_segment_id,
         'question_search_watch' AS source,
         '' AS reaction_type,
         TRUE AS clicked,
         TRUE AS watched,
         extracted.watch_duration AS watch_duration,
         vs.segment_duration,
         qs.create_time AS event_time
  FROM public.edu_question_search_record qs
  CROSS JOIN LATERAL jsonb_array_elements(
    CASE
      WHEN NULLIF(TRIM(COALESCE(qs.recommend_videos_json, '')), '') IS NULL THEN '[]'::jsonb
      WHEN LEFT(TRIM(qs.recommend_videos_json), 1) = '[' THEN qs.recommend_videos_json::jsonb
      ELSE '[]'::jsonb
    END
  ) rec(value)
  CROSS JOIN LATERAL (
    SELECT CASE
             WHEN COALESCE(NULLIF(rec.value->>'video_segment_id', ''), NULLIF(rec.value->>'videoSegmentId', ''), NULLIF(rec.value->>'id', '')) ~ '^[0-9]+$'
             THEN COALESCE(NULLIF(rec.value->>'video_segment_id', ''), NULLIF(rec.value->>'videoSegmentId', ''), NULLIF(rec.value->>'id', ''))::bigint
             ELSE 0
           END AS video_segment_id,
           CASE
             WHEN COALESCE(NULLIF(rec.value->>'watch_duration', ''), NULLIF(rec.value->>'watchDuration', '')) ~ '^[0-9]+$'
             THEN COALESCE(NULLIF(rec.value->>'watch_duration', ''), NULLIF(rec.value->>'watchDuration', ''))::int
             ELSE 0
           END AS watch_duration
  ) extracted
  JOIN valid_segments vs ON vs.video_segment_id = extracted.video_segment_id
  WHERE COALESCE(qs.deleted, 0) = 0
    AND extracted.video_segment_id > 0
)
SELECT e.user_id,
       e.video_id,
       e.video_segment_id,
       e.source,
       e.reaction_type,
       e.clicked,
       e.watched,
       e.watch_duration,
       e.segment_duration,
       e.event_time
FROM events e
JOIN public.sys_user u ON u.id = e.user_id
WHERE e.user_id > 0
  AND e.video_segment_id > 0
  AND e.event_time >= NOW() - ($2::int * INTERVAL '1 day')
ORDER BY e.event_time DESC
LIMIT $1`
}

func buildInteractionRows(events []interactionEvent) []interactionRow {
	rows := make([]interactionRow, 0, len(events))
	for _, event := range events {
		row, ok := interactionFromEvent(event)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func interactionFromEvent(event interactionEvent) (interactionRow, bool) {
	row := interactionRow{
		UserID:         event.UserID,
		VideoSegmentID: event.VideoSegmentID,
		Source:         event.Source,
		Timestamp:      float64(event.EventTime.Unix()),
	}
	switch event.Source {
	case "segment_reaction", "video_reaction":
		switch event.ReactionType {
		case "double_like":
			row.Rating, row.Weight = 3.0, 3.0
		case "like":
			row.Rating, row.Weight = 2.0, 2.0
		case "dislike":
			row.Rating, row.Weight = 0.0, 2.0
		default:
			return interactionRow{}, false
		}
	case "watch", "question_search_watch":
		ratio := 0.0
		if event.SegmentDuration > 0 {
			ratio = float64(event.WatchDuration) / float64(event.SegmentDuration)
		}
		if event.Watched && ratio >= 0.4 {
			row.Rating, row.Weight = 1.5, 1.5
		} else {
			row.Rating, row.Weight = 0.3, 0.3
		}
	case "exposure":
		switch {
		case event.Watched:
			row.Rating, row.Weight = 1.5, 1.0
		case event.Clicked:
			row.Rating, row.Weight = 1.0, 0.8
		default:
			row.Rating, row.Weight = 0.1, 0.2
		}
	default:
		return interactionRow{}, false
	}
	return row, true
}

func loadItems(ctx context.Context, db *sql.DB) ([]itemRow, error) {
	rows, err := db.QueryContext(ctx, buildItemQuery())
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()
	var items []itemRow
	for rows.Next() {
		var item itemRow
		if err := rows.Scan(
			&item.VideoSegmentID,
			&item.VideoID,
			&item.SegmentDuration,
			&item.VideoDuration,
			&item.LikeCount,
			&item.DoubleLikeCount,
			&item.DislikeCount,
			&item.ContentSummary,
			&item.KnowledgeTags,
			&item.VideoTitle,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func buildItemQuery() string {
	return `
SELECT s.id AS video_segment_id,
       s.video_id,
       GREATEST(COALESCE(s.end_time, 0) - COALESCE(s.start_time, 0), 1) AS segment_duration,
       COALESCE(r.duration, 0) AS video_duration,
       COALESCE(s.like_count, 0) AS like_count,
       COALESCE(s.double_like_count, 0) AS double_like_count,
       COALESCE(s.dislike_count, 0) AS dislike_count,
       COALESCE(s.content_summary, '') AS content_summary,
       COALESCE(array_to_string(s.knowledge_tags, '|'), '') AS knowledge_tags,
       COALESCE(r.title, '') AS video_title
FROM public.edu_video_segment s
JOIN public.edu_video_resource r ON r.id = s.video_id
WHERE s.deleted = 0
  AND s.status = 1
  AND s.id > 0
  AND s.video_id > 0
  AND r.deleted = 0
ORDER BY s.id`
}

func loadUserFeatures(ctx context.Context, db *sql.DB) ([]userFeatureRow, error) {
	rows, err := db.QueryContext(ctx, buildUserFeatureQuery())
	if err != nil {
		return nil, fmt.Errorf("query user features: %w", err)
	}
	defer rows.Close()
	var users []userFeatureRow
	for rows.Next() {
		var user userFeatureRow
		if err := rows.Scan(
			&user.UserID,
			&user.GradeID,
			&user.ClassID,
			&user.UserType,
			&user.MasteryAvg,
			&user.WeakKnowledgeCount,
			&user.AnswerCount,
			&user.AnswerCorrectCount,
			&user.QuestionFeedbackCount,
			&user.GeneratedFeedbackCount,
			&user.QuestionSearchCount,
			&user.SpecialPracticeCount,
			&user.StudentWordCount,
			&user.EnglishReadingCount,
			&user.EnglishListeningCount,
			&user.EnglishStorybookCount,
			&user.ProfileSnapshotFeatures,
			&user.QuestionSearchKnowledge,
			&user.RecentKnowledgePointIDs,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func buildUserFeatureQuery() string {
	return `
WITH valid_users AS (
  SELECT id AS user_id,
         COALESCE(grade_id, 0) AS grade_id,
         COALESCE(class_id, 0) AS class_id,
         COALESCE(user_type, 0) AS user_type
  FROM public.sys_user
  WHERE id > 0
    AND COALESCE(deleted, 0) = 0
),
mastery AS (
  SELECT user_id,
         COALESCE(AVG(mastery), 0) AS mastery_avg,
         COUNT(*) FILTER (WHERE mastery < 0.6) AS weak_knowledge_count
  FROM public.edu_user_knowledge_mastery
  GROUP BY user_id
),
answer AS (
  SELECT user_id,
         COUNT(*) AS answer_count,
         COUNT(*) FILTER (WHERE is_correct = 1) AS answer_correct_count,
         COALESCE(string_agg(DISTINCT knowledge_point_id::text, '|' ORDER BY knowledge_point_id::text), '') AS recent_knowledge_point_ids
  FROM public.edu_knowledge_answer_record
  WHERE user_id > 0
  GROUP BY user_id
),
question_feedback AS (
  SELECT user_id, COALESCE(SUM(feedback_count), 0) AS question_feedback_count
  FROM public.edu_user_question_feedback
  GROUP BY user_id
),
generated_feedback AS (
  SELECT user_id, COUNT(*) AS generated_feedback_count
  FROM public.edu_generated_question_feedback
  WHERE user_id > 0
  GROUP BY user_id
),
question_search AS (
  SELECT user_id,
         COUNT(*) AS question_search_count,
         COALESCE(string_agg(DISTINCT COALESCE(first_question_knowledge, ''), ' '), '') AS question_search_knowledge
  FROM public.edu_question_search_record
  WHERE COALESCE(deleted, 0) = 0
  GROUP BY user_id
),
special_practice AS (
  SELECT user_id, COUNT(*) AS special_practice_count
  FROM public.edu_special_practice_session
  WHERE user_id > 0
  GROUP BY user_id
),
student_words AS (
  SELECT user_id, COUNT(*) AS student_word_count
  FROM public.edu_student_word_record
  WHERE user_id > 0
  GROUP BY user_id
),
english_reading AS (
  SELECT user_id, COUNT(*) AS english_reading_count
  FROM public.english_reading_history
  WHERE user_id > 0
  GROUP BY user_id
),
english_listening AS (
  SELECT user_id, COUNT(*) AS english_listening_count
  FROM public.english_listening_session
  WHERE user_id > 0
  GROUP BY user_id
),
english_storybook AS (
  SELECT user_id, COUNT(*) AS english_storybook_count
  FROM public.english_storybook_session
  WHERE user_id > 0
  GROUP BY user_id
),
profile_snapshot AS (
  SELECT student_id AS user_id, COALESCE(MAX(profile_json::text), '') AS profile_snapshot_features
  FROM public.student_profile_snapshot
  WHERE student_id > 0
    AND COALESCE(deleted, 0) = 0
  GROUP BY student_id
)
SELECT vu.user_id,
       vu.grade_id,
       vu.class_id,
       vu.user_type,
       COALESCE(m.mastery_avg, 0) AS mastery_avg,
       COALESCE(m.weak_knowledge_count, 0) AS weak_knowledge_count,
       COALESCE(a.answer_count, 0) AS answer_count,
       COALESCE(a.answer_correct_count, 0) AS answer_correct_count,
       COALESCE(qf.question_feedback_count, 0) AS question_feedback_count,
       COALESCE(gf.generated_feedback_count, 0) AS generated_feedback_count,
       COALESCE(qs.question_search_count, 0) AS question_search_count,
       COALESCE(sp.special_practice_count, 0) AS special_practice_count,
       COALESCE(sw.student_word_count, 0) AS student_word_count,
       COALESCE(er.english_reading_count, 0) AS english_reading_count,
       COALESCE(el.english_listening_count, 0) AS english_listening_count,
       COALESCE(es.english_storybook_count, 0) AS english_storybook_count,
       COALESCE(ps.profile_snapshot_features, '') AS profile_snapshot_features,
       COALESCE(qs.question_search_knowledge, '') AS question_search_knowledge,
       COALESCE(a.recent_knowledge_point_ids, '') AS recent_knowledge_point_ids
FROM valid_users vu
LEFT JOIN mastery m ON m.user_id = vu.user_id
LEFT JOIN answer a ON a.user_id = vu.user_id
LEFT JOIN question_feedback qf ON qf.user_id = vu.user_id
LEFT JOIN generated_feedback gf ON gf.user_id = vu.user_id
LEFT JOIN question_search qs ON qs.user_id = vu.user_id
LEFT JOIN special_practice sp ON sp.user_id = vu.user_id
LEFT JOIN student_words sw ON sw.user_id = vu.user_id
LEFT JOIN english_reading er ON er.user_id = vu.user_id
LEFT JOIN english_listening el ON el.user_id = vu.user_id
LEFT JOIN english_storybook es ON es.user_id = vu.user_id
LEFT JOIN profile_snapshot ps ON ps.user_id = vu.user_id
ORDER BY vu.user_id`
}

func writeAtomicFile(path string, write func(io.Writer) error) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(file); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeInteractions(out io.Writer, rows []interactionRow) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{"user_id:token", "item_id:token", "rating:float", "timestamp:float", "source:token", "weight:float"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			strconv.FormatUint(row.UserID, 10),
			strconv.FormatUint(row.VideoSegmentID, 10),
			strconv.FormatFloat(row.Rating, 'f', 3, 64),
			strconv.FormatFloat(row.Timestamp, 'f', 0, 64),
			row.Source,
			strconv.FormatFloat(row.Weight, 'f', 3, 64),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeItems(out io.Writer, rows []itemRow) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{
		"item_id:token",
		"video_id:token",
		"segment_duration:float",
		"video_duration:float",
		"like_count:float",
		"double_like_count:float",
		"dislike_count:float",
		"content_summary:token_seq",
		"knowledge_tags:token_seq",
		"video_title:token_seq",
	}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			strconv.FormatUint(row.VideoSegmentID, 10),
			strconv.FormatUint(row.VideoID, 10),
			strconv.Itoa(row.SegmentDuration),
			strconv.Itoa(row.VideoDuration),
			strconv.Itoa(row.LikeCount),
			strconv.Itoa(row.DoubleLikeCount),
			strconv.Itoa(row.DislikeCount),
			row.ContentSummary,
			row.KnowledgeTags,
			row.VideoTitle,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeUsers(out io.Writer, rows []userFeatureRow) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{
		"user_id:token",
		"grade_id:token",
		"class_id:token",
		"user_type:token",
		"mastery_avg:float",
		"weak_knowledge_count:float",
		"answer_count:float",
		"answer_correct_count:float",
		"question_feedback_count:float",
		"generated_feedback_count:float",
		"question_search_count:float",
		"special_practice_count:float",
		"student_word_count:float",
		"english_reading_count:float",
		"english_listening_count:float",
		"english_storybook_count:float",
		"profile_snapshot_features:token_seq",
		"question_search_knowledge:token_seq",
		"recent_knowledge_point_ids:token_seq",
	}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			strconv.FormatUint(row.UserID, 10),
			strconv.FormatUint(row.GradeID, 10),
			strconv.FormatUint(row.ClassID, 10),
			strconv.Itoa(row.UserType),
			strconv.FormatFloat(row.MasteryAvg, 'f', 6, 64),
			strconv.Itoa(row.WeakKnowledgeCount),
			strconv.Itoa(row.AnswerCount),
			strconv.Itoa(row.AnswerCorrectCount),
			strconv.Itoa(row.QuestionFeedbackCount),
			strconv.Itoa(row.GeneratedFeedbackCount),
			strconv.Itoa(row.QuestionSearchCount),
			strconv.Itoa(row.SpecialPracticeCount),
			strconv.Itoa(row.StudentWordCount),
			strconv.Itoa(row.EnglishReadingCount),
			strconv.Itoa(row.EnglishListeningCount),
			strconv.Itoa(row.EnglishStorybookCount),
			row.ProfileSnapshotFeatures,
			row.QuestionSearchKnowledge,
			row.RecentKnowledgePointIDs,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
