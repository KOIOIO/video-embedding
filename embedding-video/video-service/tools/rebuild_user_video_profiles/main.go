package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	profileapp "nlp-video-analysis/internal/application/videoapp/profile"
	"nlp-video-analysis/internal/config"
)

const profileVectorDim = 1536

type options struct {
	configFile   string
	userID       uint64
	limitUsers   int
	modelVersion string
	dryRun       bool
}

type profileEvent struct {
	sourceType      string
	reactionType    string
	vector          []float32
	watchDuration   int
	segmentDuration int
	eventTime       time.Time
}

type profileRow struct {
	UserID           uint64
	ProfileVector    []float32
	PositiveCount    int
	NegativeCount    int
	WatchCount       int
	SourceEventCount int
	LastEventTime    time.Time
	ModelVersion     string
	Status           int16
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseOptions(args []string) (options, error) {
	var opts options
	fs := flag.NewFlagSet("rebuild_user_video_profiles", flag.ContinueOnError)
	fs.StringVar(&opts.configFile, "config", "", "config file path")
	fs.Uint64Var(&opts.userID, "user-id", 0, "only rebuild one user profile")
	fs.IntVar(&opts.limitUsers, "limit-users", 1000, "maximum users to scan when user-id is absent")
	fs.StringVar(&opts.modelVersion, "model-version", "video_profile_v1", "profile model version")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print planned work without writing profiles")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.modelVersion = strings.TrimSpace(opts.modelVersion)
	if opts.modelVersion == "" {
		return options{}, fmt.Errorf("model-version is required")
	}
	if opts.limitUsers <= 0 {
		return options{}, fmt.Errorf("limit-users must be greater than 0")
	}
	return opts, nil
}

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	if opts.configFile != "" {
		_ = os.Setenv("CONFIG_FILE", opts.configFile)
	}
	config.EnsureProjectRoot()
	cfg := config.MustLoadDefault()
	if opts.configFile != "" {
		cfg = config.MustLoad(opts.configFile)
	}
	if cfg.Postgres.DSN == "" {
		return fmt.Errorf("postgres dsn is required")
	}
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	users, err := loadCandidateUsers(ctx, db, opts)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "model_version=%s dry_run=%v users=%d\n", opts.modelVersion, opts.dryRun, len(users))
	now := time.Now()
	active := 0
	inactive := 0
	for _, userID := range users {
		events, err := loadProfileEvents(ctx, db, userID)
		if err != nil {
			return fmt.Errorf("load profile events for user %d: %w", userID, err)
		}
		row := buildProfileRow(userID, opts.modelVersion, events, now)
		if row.Status == 1 {
			active++
		} else {
			inactive++
		}
		fmt.Fprintf(out, "user_id=%d events=%d status=%d positive=%d negative=%d watch=%d\n", userID, len(events), row.Status, row.PositiveCount, row.NegativeCount, row.WatchCount)
		if opts.dryRun {
			continue
		}
		if err := upsertProfile(ctx, db, row, now); err != nil {
			return fmt.Errorf("upsert profile for user %d: %w", userID, err)
		}
	}
	fmt.Fprintf(out, "active_profiles=%d inactive_profiles=%d\n", active, inactive)
	fmt.Fprintf(out, "finished_at=%s\n", time.Now().Format(time.RFC3339))
	return nil
}

func loadCandidateUsers(ctx context.Context, db *gorm.DB, opts options) ([]uint64, error) {
	userIDColumn, err := detectSysUserIDColumn(ctx, db)
	if err != nil {
		return nil, err
	}
	if opts.userID != 0 {
		var exists uint64
		query := fmt.Sprintf(`SELECT %s FROM public.sys_user WHERE %s = ? LIMIT 1`, userIDColumn, userIDColumn)
		if err := db.WithContext(ctx).Raw(query, opts.userID).Scan(&exists).Error; err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, nil
		}
		return []uint64{opts.userID}, nil
	}
	rows := make([]uint64, 0, opts.limitUsers)
	query := fmt.Sprintf(`
SELECT DISTINCT u.%[1]s AS user_id
FROM public.sys_user u
JOIN (
  SELECT user_id FROM public.edu_user_reaction WHERE deleted = 0
  UNION
  SELECT user_id FROM public.edu_video_user_reaction WHERE deleted = 0
  UNION
  SELECT user_id FROM public.edu_user_video_recommend WHERE deleted = 0
) b ON b.user_id = u.%[1]s
ORDER BY u.%[1]s
LIMIT ?`, userIDColumn)
	err = db.WithContext(ctx).Raw(query, opts.limitUsers).Scan(&rows).Error
	return rows, err
}

func detectSysUserIDColumn(ctx context.Context, db *gorm.DB) (string, error) {
	var columns []string
	if err := db.WithContext(ctx).Raw(`
SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'sys_user'`).Scan(&columns).Error; err != nil {
		return "", err
	}
	return chooseSysUserIDColumn(columns)
}

func chooseSysUserIDColumn(columns []string) (string, error) {
	seen := make(map[string]bool, len(columns))
	for _, column := range columns {
		seen[strings.ToLower(strings.TrimSpace(column))] = true
	}
	switch {
	case seen["user_id"]:
		return "user_id", nil
	case seen["id"]:
		return "id", nil
	default:
		return "", fmt.Errorf("sys_user must have a user_id or id column")
	}
}

func loadProfileEvents(ctx context.Context, db *gorm.DB, userID uint64) ([]profileEvent, error) {
	events := make([]profileEvent, 0, 128)
	segmentReactionEvents, err := loadSegmentReactionEvents(ctx, db, userID)
	if err != nil {
		return nil, err
	}
	events = append(events, segmentReactionEvents...)
	videoReactionEvents, err := loadVideoReactionEvents(ctx, db, userID)
	if err != nil {
		return nil, err
	}
	events = append(events, videoReactionEvents...)
	watchEvents, err := loadWatchEvents(ctx, db, userID)
	if err != nil {
		return nil, err
	}
	events = append(events, watchEvents...)
	return events, nil
}

func loadSegmentReactionEvents(ctx context.Context, db *gorm.DB, userID uint64) ([]profileEvent, error) {
	var rows []struct {
		ReactionType string    `gorm:"column:reaction_type"`
		Vector       string    `gorm:"column:embedding"`
		EventTime    time.Time `gorm:"column:event_time"`
	}
	err := db.WithContext(ctx).Raw(`
SELECT ur.reaction_type AS reaction_type,
       s.embedding::text AS embedding,
       ur.update_time AS event_time
FROM public.edu_user_reaction ur
JOIN public.edu_video_segment s ON s.id = ur.video_segment_id
JOIN public.edu_video_resource r ON r.id = s.video_id
WHERE ur.user_id = ?
  AND ur.deleted = 0
  AND s.deleted = 0
  AND s.status = 1
  AND s.embedding IS NOT NULL
  AND r.deleted = 0
ORDER BY ur.update_time DESC
LIMIT 500`, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	events := make([]profileEvent, 0, len(rows))
	for _, row := range rows {
		vector, err := parseVectorText(row.Vector)
		if err != nil {
			return nil, err
		}
		events = append(events, profileEvent{sourceType: string(profileapp.SourceSegmentReaction), reactionType: row.ReactionType, vector: vector, eventTime: row.EventTime})
	}
	return events, nil
}

func loadVideoReactionEvents(ctx context.Context, db *gorm.DB, userID uint64) ([]profileEvent, error) {
	var rows []struct {
		ReactionType string    `gorm:"column:reaction_type"`
		Vector       string    `gorm:"column:embedding"`
		EventTime    time.Time `gorm:"column:event_time"`
	}
	err := db.WithContext(ctx).Raw(`
WITH video_vectors AS (
  SELECT video_id, AVG(embedding)::text AS embedding
  FROM public.edu_video_segment
  WHERE deleted = 0 AND status = 1 AND embedding IS NOT NULL
  GROUP BY video_id
)
SELECT vur.reaction_type AS reaction_type,
       vv.embedding AS embedding,
       vur.update_time AS event_time
FROM public.edu_video_user_reaction vur
JOIN video_vectors vv ON vv.video_id = vur.video_id
JOIN public.edu_video_resource r ON r.id = vur.video_id
WHERE vur.user_id = ?
  AND vur.deleted = 0
  AND r.deleted = 0
ORDER BY vur.update_time DESC
LIMIT 500`, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	events := make([]profileEvent, 0, len(rows))
	for _, row := range rows {
		vector, err := parseVectorText(row.Vector)
		if err != nil {
			return nil, err
		}
		events = append(events, profileEvent{sourceType: string(profileapp.SourceVideoReaction), reactionType: row.ReactionType, vector: vector, eventTime: row.EventTime})
	}
	return events, nil
}

func loadWatchEvents(ctx context.Context, db *gorm.DB, userID uint64) ([]profileEvent, error) {
	var rows []struct {
		Vector          string    `gorm:"column:embedding"`
		WatchDuration   int       `gorm:"column:watch_duration"`
		SegmentDuration int       `gorm:"column:segment_duration"`
		EventTime       time.Time `gorm:"column:event_time"`
	}
	err := db.WithContext(ctx).Raw(`
SELECT s.embedding::text AS embedding,
       COALESCE(uvr.watch_duration, 0) AS watch_duration,
       GREATEST(s.end_time - s.start_time, 1) AS segment_duration,
       uvr.update_time AS event_time
FROM public.edu_user_video_recommend uvr
JOIN public.edu_video_segment s ON s.id = uvr.video_segment_id
JOIN public.edu_video_resource r ON r.id = s.video_id
WHERE uvr.user_id = ?
  AND uvr.deleted = 0
  AND uvr.is_watched = TRUE
  AND s.deleted = 0
  AND s.status = 1
  AND s.embedding IS NOT NULL
  AND r.deleted = 0
ORDER BY uvr.update_time DESC
LIMIT 500`, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	events := make([]profileEvent, 0, len(rows))
	for _, row := range rows {
		vector, err := parseVectorText(row.Vector)
		if err != nil {
			return nil, err
		}
		events = append(events, profileEvent{sourceType: string(profileapp.SourceWatch), vector: vector, watchDuration: row.WatchDuration, segmentDuration: row.SegmentDuration, eventTime: row.EventTime})
	}
	return events, nil
}

func buildProfileRow(userID uint64, modelVersion string, events []profileEvent, now time.Time) profileRow {
	weighted := make([]profileapp.WeightedEvent, 0, len(events))
	for _, event := range events {
		weighted = append(weighted, profileapp.WeightedEvent{
			SourceType:      profileapp.SourceType(event.sourceType),
			ReactionType:    profileapp.ReactionType(event.reactionType),
			Vector:          event.vector,
			WatchDuration:   event.watchDuration,
			SegmentDuration: event.segmentDuration,
			EventTime:       event.eventTime,
		})
	}
	built := profileapp.BuildUserVideoProfile(weighted, now)
	status := int16(0)
	vector := built.Vector
	if built.Valid {
		status = 1
	} else {
		vector = make([]float32, profileVectorDim)
	}
	return profileRow{
		UserID:           userID,
		ProfileVector:    vector,
		PositiveCount:    built.PositiveCount,
		NegativeCount:    built.NegativeCount,
		WatchCount:       built.WatchCount,
		SourceEventCount: built.SourceEventCount,
		LastEventTime:    built.LastEventTime,
		ModelVersion:     modelVersion,
		Status:           status,
	}
}

func upsertProfile(ctx context.Context, db *gorm.DB, row profileRow, now time.Time) error {
	vector := pgvector.NewVector(row.ProfileVector)
	return db.WithContext(ctx).Exec(`
INSERT INTO public.edu_user_video_profile
  (user_id, profile_vector, positive_count, negative_count, watch_count,
   source_event_count, last_event_time, model_version, status, deleted,
   create_time, update_time)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
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
  update_time = EXCLUDED.update_time`,
		row.UserID,
		vector,
		row.PositiveCount,
		row.NegativeCount,
		row.WatchCount,
		row.SourceEventCount,
		row.LastEventTime,
		row.ModelVersion,
		row.Status,
		now,
		now,
	).Error
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
