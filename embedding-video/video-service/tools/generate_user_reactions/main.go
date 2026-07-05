package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"nlp-video-analysis/internal/config"
)

const (
	defaultConfigPath = "configs/video_prod.yml"
	defaultCount      = 500
	defaultDaysBack   = 30
)

type videoSegment struct {
	ID      uint64
	VideoID uint64
}

type reactionPlan struct {
	UserID         uint64
	VideoID        uint64
	VideoSegmentID uint64
	ReactionType   string
	CreateTime     time.Time
	UpdateTime     time.Time
}

type pairKey struct {
	UserID    uint64
	SegmentID uint64
}

type columnInfo struct {
	Name     string
	DataType string
}

func main() {
	config.EnsureProjectRoot()
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("generate_user_reactions", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", defaultConfigPath, "config file used for PostgreSQL DSN")
	count := fs.Int("count", defaultCount, "number of active edu_user_reaction rows to generate")
	daysBack := fs.Int("days-back", defaultDaysBack, "spread create_time/update_time across the last N days")
	seed := fs.Int64("seed", time.Now().UnixNano(), "random seed")
	dryRun := fs.Bool("dry-run", false, "build plans and print a summary without writing data")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *count < 0 {
		return errors.New("count must be >= 0")
	}
	if *daysBack <= 0 {
		return errors.New("days-back must be > 0")
	}

	cfg := config.MustLoad(*configPath)
	if strings.TrimSpace(cfg.Postgres.DSN) == "" {
		return errors.New("Postgres DSN is required")
	}

	db, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	userColumns, err := listColumns(ctx, db, "sys_user")
	if err != nil {
		return fmt.Errorf("inspect sys_user columns: %w", err)
	}
	userIDColumn, err := detectUserIDColumn(ctx, db, userColumns)
	if err != nil {
		return err
	}
	users, err := loadUsers(ctx, db, userIDColumn, userColumns)
	if err != nil {
		return fmt.Errorf("load sys_user ids: %w", err)
	}
	segments, err := loadSegments(ctx, db)
	if err != nil {
		return fmt.Errorf("load edu_video_segment ids: %w", err)
	}
	existingPairs, err := loadExistingPairs(ctx, db)
	if err != nil {
		return fmt.Errorf("load existing edu_user_reaction pairs: %w", err)
	}

	rng := rand.New(rand.NewSource(*seed))
	plans, err := buildReactionPlansExcluding(users, segments, *count, existingPairs, rng, time.Now(), time.Duration(*daysBack)*24*time.Hour)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "config=%s\n", *configPath)
	fmt.Fprintf(out, "postgres=%s\n", maskDSN(cfg.Postgres.DSN))
	fmt.Fprintf(out, "user_source=sys_user.%s users=%d\n", userIDColumn, len(users))
	fmt.Fprintf(out, "segment_source=edu_video_segment(deleted=0) segments=%d\n", len(segments))
	fmt.Fprintf(out, "existing_user_segment_pairs=%d\n", len(existingPairs))
	fmt.Fprintf(out, "seed=%d count=%d dry_run=%v\n", *seed, len(plans), *dryRun)
	printPlanSummary(out, plans)

	if *dryRun {
		fmt.Fprintln(out, "dry run complete; no data was written")
		return nil
	}
	inserted, err := insertPlans(ctx, db, plans)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "inserted=%d\n", inserted)
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

func listColumns(ctx context.Context, db *sql.DB, table string) (map[string]columnInfo, error) {
	rows, err := db.QueryContext(ctx, `
SELECT column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = $1`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := map[string]columnInfo{}
	for rows.Next() {
		var col columnInfo
		if err := rows.Scan(&col.Name, &col.DataType); err != nil {
			return nil, err
		}
		columns[col.Name] = col
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("public.%s was not found", table)
	}
	return columns, nil
}

func detectUserIDColumn(ctx context.Context, db *sql.DB, columns map[string]columnInfo) (string, error) {
	pkRows, err := db.QueryContext(ctx, `
SELECT kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
 AND tc.table_name = kcu.table_name
WHERE tc.table_schema = 'public'
  AND tc.table_name = 'sys_user'
  AND tc.constraint_type = 'PRIMARY KEY'
ORDER BY kcu.ordinal_position`)
	if err != nil {
		return "", err
	}
	defer pkRows.Close()
	for pkRows.Next() {
		var name string
		if err := pkRows.Scan(&name); err != nil {
			return "", err
		}
		if name == "user_id" || name == "id" {
			return name, nil
		}
	}
	if err := pkRows.Err(); err != nil {
		return "", err
	}

	for _, candidate := range []string{"user_id", "id"} {
		if _, ok := columns[candidate]; ok {
			return candidate, nil
		}
	}
	return "", errors.New("sys_user must have a user_id or id column")
}

func loadUsers(ctx context.Context, db *sql.DB, idColumn string, columns map[string]columnInfo) ([]uint64, error) {
	conditions := []string{quoteIdent(idColumn) + " IS NOT NULL", quoteIdent(idColumn) + " > 0"}
	if col, ok := columns["deleted"]; ok {
		if strings.EqualFold(col.DataType, "boolean") {
			conditions = append(conditions, "deleted = FALSE")
		} else {
			conditions = append(conditions, "COALESCE(deleted, 0) = 0")
		}
	}
	if _, ok := columns["del_flag"]; ok {
		conditions = append(conditions, "COALESCE(del_flag::text, '0') = '0'")
	}

	query := fmt.Sprintf(
		"SELECT %s FROM (SELECT DISTINCT %s FROM public.sys_user WHERE %s) users ORDER BY random()",
		quoteIdent(idColumn),
		quoteIdent(idColumn),
		strings.Join(conditions, " AND "),
	)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []uint64
	for rows.Next() {
		var userID uint64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		users = append(users, userID)
	}
	return users, rows.Err()
}

func loadSegments(ctx context.Context, db *sql.DB) ([]videoSegment, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, video_id
FROM public.edu_video_segment
WHERE deleted = 0 AND id > 0 AND video_id > 0
ORDER BY random()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []videoSegment
	for rows.Next() {
		var segment videoSegment
		if err := rows.Scan(&segment.ID, &segment.VideoID); err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}
	return segments, rows.Err()
}

func loadExistingPairs(ctx context.Context, db *sql.DB) (map[pairKey]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT user_id, video_segment_id FROM public.edu_user_reaction`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pairs := map[pairKey]bool{}
	for rows.Next() {
		var pair pairKey
		if err := rows.Scan(&pair.UserID, &pair.SegmentID); err != nil {
			return nil, err
		}
		pairs[pair] = true
	}
	return pairs, rows.Err()
}

func buildReactionPlans(users []uint64, segments []videoSegment, count int, rng *rand.Rand, now time.Time) ([]reactionPlan, error) {
	return buildReactionPlansExcluding(users, segments, count, nil, rng, now, defaultDaysBack*24*time.Hour)
}

func buildReactionPlansExcluding(users []uint64, segments []videoSegment, count int, excluded map[pairKey]bool, rng *rand.Rand, now time.Time, window time.Duration) ([]reactionPlan, error) {
	if count < 0 {
		return nil, errors.New("count must be >= 0")
	}
	if count == 0 {
		return nil, nil
	}
	if len(users) == 0 {
		return nil, errors.New("sys_user has no eligible users")
	}
	if len(segments) == 0 {
		return nil, errors.New("edu_video_segment has no eligible deleted=0 segments")
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if window <= 0 {
		window = defaultDaysBack * 24 * time.Hour
	}

	users = append([]uint64(nil), users...)
	segments = append([]videoSegment(nil), segments...)
	rng.Shuffle(len(users), func(i, j int) { users[i], users[j] = users[j], users[i] })
	rng.Shuffle(len(segments), func(i, j int) { segments[i], segments[j] = segments[j], segments[i] })

	capacity := availableCapacity(users, segments, excluded)
	if count > capacity {
		return nil, fmt.Errorf("not enough unique user/segment pairs: requested=%d available=%d", count, capacity)
	}

	used := make(map[pairKey]bool, count)
	plans := make([]reactionPlan, 0, count)
	maxAttempts := count*200 + 1000
	for attempts := 0; len(plans) < count && attempts < maxAttempts; attempts++ {
		user := users[pickSkewedIndex(len(users), rng, 1.8)]
		segment := segments[pickSkewedIndex(len(segments), rng, 2.2)]
		pair := pairKey{UserID: user, SegmentID: segment.ID}
		if excluded[pair] || used[pair] {
			continue
		}
		used[pair] = true
		plans = append(plans, newReactionPlan(user, segment, rng, now, window))
	}

	if len(plans) < count {
		remaining := enumerateAvailablePairs(users, segments, excluded, used)
		rng.Shuffle(len(remaining), func(i, j int) { remaining[i], remaining[j] = remaining[j], remaining[i] })
		for _, pair := range remaining {
			if len(plans) >= count {
				break
			}
			plans = append(plans, newReactionPlan(pair.UserID, videoSegment{ID: pair.SegmentID, VideoID: pair.VideoID}, rng, now, window))
		}
	}

	return plans, nil
}

type availablePair struct {
	UserID    uint64
	SegmentID uint64
	VideoID   uint64
}

func availableCapacity(users []uint64, segments []videoSegment, excluded map[pairKey]bool) int {
	capacity := len(users) * len(segments)
	if len(excluded) == 0 {
		return capacity
	}

	userSet := make(map[uint64]bool, len(users))
	segmentSet := make(map[uint64]bool, len(segments))
	for _, user := range users {
		userSet[user] = true
	}
	for _, segment := range segments {
		segmentSet[segment.ID] = true
	}
	for pair := range excluded {
		if userSet[pair.UserID] && segmentSet[pair.SegmentID] {
			capacity--
		}
	}
	return capacity
}

func enumerateAvailablePairs(users []uint64, segments []videoSegment, excluded map[pairKey]bool, used map[pairKey]bool) []availablePair {
	var pairs []availablePair
	for _, user := range users {
		for _, segment := range segments {
			pair := pairKey{UserID: user, SegmentID: segment.ID}
			if excluded[pair] || used[pair] {
				continue
			}
			pairs = append(pairs, availablePair{UserID: user, SegmentID: segment.ID, VideoID: segment.VideoID})
		}
	}
	return pairs
}

func newReactionPlan(userID uint64, segment videoSegment, rng *rand.Rand, now time.Time, window time.Duration) reactionPlan {
	createTime := randomPastTime(rng, now, window)
	updateTime := createTime
	if rng.Intn(100) < 18 {
		available := now.Sub(createTime)
		if available > time.Second {
			updateTime = createTime.Add(time.Duration(rng.Int63n(int64(available))))
		}
	}
	return reactionPlan{
		UserID:         userID,
		VideoID:        segment.VideoID,
		VideoSegmentID: segment.ID,
		ReactionType:   chooseReactionType(rng),
		CreateTime:     createTime,
		UpdateTime:     updateTime,
	}
}

func chooseReactionType(rng *rand.Rand) string {
	n := rng.Intn(100)
	switch {
	case n < 70:
		return "like"
	case n < 92:
		return "double_like"
	default:
		return "dislike"
	}
}

func pickSkewedIndex(n int, rng *rand.Rand, exponent float64) int {
	if n <= 1 {
		return 0
	}
	index := int(math.Pow(rng.Float64(), exponent) * float64(n))
	if index >= n {
		return n - 1
	}
	return index
}

func randomPastTime(rng *rand.Rand, now time.Time, window time.Duration) time.Time {
	if window <= 0 {
		return now
	}
	offset := time.Duration(rng.Int63n(int64(window)))
	return now.Add(-offset).Truncate(time.Second)
}

func insertPlans(ctx context.Context, db *sql.DB, plans []reactionPlan) (int, error) {
	if len(plans) == 0 {
		return 0, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	inserted := 0
	for _, plan := range plans {
		result, err := tx.ExecContext(ctx, `
INSERT INTO public.edu_user_reaction
  (user_id, video_id, video_segment_id, reaction_type, create_time, update_time, deleted)
VALUES
  ($1, $2, $3, $4, $5, $6, 0)`,
			plan.UserID,
			plan.VideoID,
			plan.VideoSegmentID,
			plan.ReactionType,
			plan.CreateTime,
			plan.UpdateTime,
		)
		if err != nil {
			return 0, fmt.Errorf("insert user=%d segment=%d: %w", plan.UserID, plan.VideoSegmentID, err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if rowsAffected != 1 {
			return 0, fmt.Errorf("insert user=%d segment=%d affected %d rows", plan.UserID, plan.VideoSegmentID, rowsAffected)
		}
		inserted++
	}

	for _, segmentID := range uniqueSegmentIDs(plans) {
		result, err := tx.ExecContext(ctx, `
UPDATE public.edu_video_segment
SET like_count = COALESCE((
      SELECT COUNT(*) FROM public.edu_user_reaction
      WHERE video_segment_id = $1 AND reaction_type = 'like' AND deleted = 0
    ), 0),
    double_like_count = COALESCE((
      SELECT COUNT(*) FROM public.edu_user_reaction
      WHERE video_segment_id = $1 AND reaction_type = 'double_like' AND deleted = 0
    ), 0),
    dislike_count = COALESCE((
      SELECT COUNT(*) FROM public.edu_user_reaction
      WHERE video_segment_id = $1 AND reaction_type = 'dislike' AND deleted = 0
    ), 0)
WHERE id = $1 AND deleted = 0`,
			segmentID,
		)
		if err != nil {
			return 0, fmt.Errorf("update segment %d counts: %w", segmentID, err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if rowsAffected != 1 {
			return 0, fmt.Errorf("update segment %d counts affected %d rows", segmentID, rowsAffected)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return inserted, nil
}

func uniqueSegmentIDs(plans []reactionPlan) []uint64 {
	seen := make(map[uint64]bool, len(plans))
	ids := make([]uint64, 0, len(plans))
	for _, plan := range plans {
		if seen[plan.VideoSegmentID] {
			continue
		}
		seen[plan.VideoSegmentID] = true
		ids = append(ids, plan.VideoSegmentID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func printPlanSummary(out io.Writer, plans []reactionPlan) {
	reactionCounts := map[string]int{}
	segments := map[uint64]bool{}
	users := map[uint64]bool{}
	for _, plan := range plans {
		reactionCounts[plan.ReactionType]++
		segments[plan.VideoSegmentID] = true
		users[plan.UserID] = true
	}
	keys := make([]string, 0, len(reactionCounts))
	for key := range reactionCounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintf(out, "planned_distinct_users=%d planned_distinct_segments=%d\n", len(users), len(segments))
	for _, key := range keys {
		fmt.Fprintf(out, "planned_%s=%d\n", key, reactionCounts[key])
	}
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func maskDSN(dsn string) string {
	parts := strings.Fields(dsn)
	for i, part := range parts {
		if strings.HasPrefix(part, "password=") {
			parts[i] = "password=<hidden>"
		}
	}
	return strings.Join(parts, " ")
}
