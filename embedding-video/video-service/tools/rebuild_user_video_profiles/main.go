package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"video-service/internal/application/videoapp/profile"
	"video-service/internal/config"
)

const (
	defaultConfigPath  = "configs/video.yml"
	defaultModelVersion = "video_profile_v1"
)

type options struct {
	configFile  string
	userID      uint64
	modelVersion string
	dryRun      bool
}

type socialEventRow struct {
	UserID        uint64
	VideoSegmentID uint64
	VideoID       uint64
	EventTime     time.Time
	SourceType    string
	EmbeddingText string
}

func main() {
	config.EnsureProjectRoot()
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{
		configFile:   defaultConfigPath,
		modelVersion: defaultModelVersion,
	}
	fs := flag.NewFlagSet("rebuild_user_video_profiles", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configFile, "config", opts.configFile, "config file used for PostgreSQL DSN")
	fs.Uint64Var(&opts.userID, "user-id", 0, "rebuild only this user (0 = all users)")
	fs.StringVar(&opts.modelVersion, "model-version", opts.modelVersion, "profile model version tag")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "collect events and compute profiles without writing to DB")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.configFile = strings.TrimSpace(opts.configFile)
	opts.modelVersion = strings.TrimSpace(opts.modelVersion)
	if opts.configFile == "" {
		return options{}, errors.New("config is required")
	}
	if opts.modelVersion == "" {
		return options{}, errors.New("model-version is required")
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

	events, err := loadSocialEvents(ctx, db, opts.userID)
	if err != nil {
		return fmt.Errorf("load social events: %w", err)
	}
	fmt.Fprintf(out, "collected social events: %d (comments + comment_likes + user_publishes)\n", len(events))

	userEvents := groupEventsByUser(events)
	fmt.Fprintf(out, "users with social events: %d\n", len(userEvents))

	now := time.Now()
	built := 0
	skipped := 0
	for userID, evts := range userEvents {
		weightedEvents, err := toWeightedEvents(evts)
		if err != nil {
			return fmt.Errorf("user %d: convert weighted events: %w", userID, err)
		}
		result := profile.BuildUserVideoProfile(weightedEvents, now)
		if !result.Valid {
			skipped++
			continue
		}
		built++
		if opts.dryRun {
			continue
		}
		if err := upsertProfile(ctx, db, userID, opts.modelVersion, result); err != nil {
			return fmt.Errorf("user %d: upsert profile: %w", userID, err)
		}
	}

	mode := "dry-run"
	if !opts.dryRun {
		mode = "written"
	}
	fmt.Fprintf(out, "rebuild complete: built=%d skipped(invalid)=%d mode=%s model_version=%s\n",
		built, skipped, mode, opts.modelVersion)
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

func loadSocialEvents(ctx context.Context, db *sql.DB, userID uint64) ([]socialEventRow, error) {
	rows, err := db.QueryContext(ctx, buildSocialEventsQuery(), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []socialEventRow
	for rows.Next() {
		var row socialEventRow
		if err := rows.Scan(
			&row.UserID,
			&row.VideoSegmentID,
			&row.VideoID,
			&row.EventTime,
			&row.SourceType,
			&row.EmbeddingText,
		); err != nil {
			return nil, err
		}
		events = append(events, row)
	}
	return events, rows.Err()
}

func buildSocialEventsQuery() string {
	return `
WITH social_events AS (
  SELECT c.user_id,
         s.id AS video_segment_id,
         s.video_id,
         c.create_time AS event_time,
         'comment' AS source_type,
         s.embedding::text AS embedding_text
  FROM public.edu_video_comment c
  JOIN public.edu_video_segment s ON s.id = c.video_segment_id
  WHERE c.deleted = 0
    AND s.deleted = 0
    AND s.status = 1

  UNION ALL

  SELECT cl.user_id,
         s.id AS video_segment_id,
         s.video_id,
         cl.create_time AS event_time,
         'comment_like' AS source_type,
         s.embedding::text AS embedding_text
  FROM public.edu_comment_like cl
  JOIN public.edu_video_comment c ON c.id = cl.comment_id
  JOIN public.edu_video_segment s ON s.id = c.video_segment_id
  WHERE cl.deleted = 0
    AND c.deleted = 0
    AND s.deleted = 0
    AND s.status = 1

  UNION ALL

  SELECT r.user_id,
         s.id AS video_segment_id,
         s.video_id,
         r.create_time AS event_time,
         'user_publish' AS source_type,
         s.embedding::text AS embedding_text
  FROM public.edu_video_resource r
  JOIN public.edu_video_segment s ON s.video_id = r.id
  WHERE r.deleted = 0
    AND r.source_type = 'user_publish'
    AND r.is_published = true
    AND s.deleted = 0
    AND s.status = 1
)
SELECT se.user_id,
       se.video_segment_id,
       se.video_id,
       se.event_time,
       se.source_type,
       se.embedding_text
FROM social_events se
WHERE ($1 = 0 OR se.user_id = $1)
ORDER BY se.user_id, se.event_time`
}

func groupEventsByUser(events []socialEventRow) map[uint64][]socialEventRow {
	result := make(map[uint64][]socialEventRow, len(events)/4)
	for i := range events {
		result[events[i].UserID] = append(result[events[i].UserID], events[i])
	}
	return result
}

func toWeightedEvents(rows []socialEventRow) ([]profile.WeightedEvent, error) {
	events := make([]profile.WeightedEvent, 0, len(rows))
	for _, row := range rows {
		vec, err := parseVectorText(row.EmbeddingText)
		if err != nil {
			return nil, fmt.Errorf("segment %d embedding parse: %w", row.VideoSegmentID, err)
		}
		if len(vec) == 0 {
			continue
		}
		sourceType, ok := sourceTypeFromString(row.SourceType)
		if !ok {
			continue
		}
		events = append(events, profile.WeightedEvent{
			SourceType: sourceType,
			Vector:     vec,
			EventTime:  row.EventTime,
		})
	}
	return events, nil
}

func sourceTypeFromString(s string) (profile.SourceType, bool) {
	switch s {
	case "comment":
		return profile.SourceComment, true
	case "comment_like":
		return profile.SourceCommentLike, true
	case "user_publish":
		return profile.SourceUserPublish, true
	default:
		return "", false
	}
}

func upsertProfile(ctx context.Context, db *sql.DB, userID uint64, modelVersion string, p profile.UserVideoProfile) error {
	vectorLiteral := vectorToLiteral(p.Vector)
	_, err := db.ExecContext(ctx, upsertProfileQuery(),
		userID,
		vectorLiteral,
		p.PositiveCount,
		p.NegativeCount,
		p.WatchCount,
		p.SourceEventCount,
		p.LastEventTime,
		modelVersion,
	)
	return err
}

func upsertProfileQuery() string {
	return `
INSERT INTO public.edu_user_video_profile
  (user_id, profile_vector, positive_count, negative_count, watch_count,
   source_event_count, last_event_time, model_version, status, deleted,
   create_time, update_time)
VALUES
  ($1, $2::vector, $3, $4, $5, $6, $7, $8, 1, 0, NOW(), NOW())
ON CONFLICT (user_id, model_version)
DO UPDATE SET
  profile_vector = EXCLUDED.profile_vector,
  positive_count = EXCLUDED.positive_count,
  negative_count = EXCLUDED.negative_count,
  watch_count = EXCLUDED.watch_count,
  source_event_count = EXCLUDED.source_event_count,
  last_event_time = EXCLUDED.last_event_time,
  status = EXCLUDED.status,
  deleted = 0,
  update_time = EXCLUDED.update_time`
}

func vectorToLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, value := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

func parseVectorText(text string) ([]float32, error) {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	trimmed = strings.TrimPrefix(trimmed, "(")
	trimmed = strings.TrimSuffix(trimmed, ")")
	if strings.TrimSpace(trimmed) == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, ",")
	values := make([]float32, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return nil, err
		}
		values = append(values, float32(value))
	}
	return values, nil
}
