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
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"nlp-video-analysis/internal/config"
)

const (
	defaultConfigPath = "configs/video.yml"
	defaultLimit      = 10000
	defaultDaysBack   = 30

	sourceSegmentReaction = "segment_reaction"
	sourceVideoReaction   = "video_reaction"
	sourceWatch           = "watch"
	sourceExposure        = "exposure"
)

type options struct {
	configFile     string
	outputFile     string
	itemOutputFile string
	userOutputFile string
	limit          int
	seedCount      int
	seed           int64
	daysBack       int
	dryRun         bool
}

type columnInfo struct {
	Name     string
	DataType string
}

type eligibleSegment struct {
	ID              uint64
	VideoID         uint64
	SegmentDuration int
}

type seedPlan struct {
	UserID          uint64
	VideoID         uint64
	VideoSegmentID  uint64
	SegmentDuration int
	ReactionType    string
	Score           float64
	Rank            int
	Clicked         bool
	Watched         bool
	WatchDuration   int
	EventTime       time.Time
}

type sampleEvent struct {
	UserID          uint64
	VideoID         uint64
	VideoSegmentID  uint64
	Source          string
	ReactionType    string
	Clicked         bool
	Watched         bool
	WatchDuration   int
	SegmentDuration int
	Rank            int
	EventTime       time.Time
}

type sampleRow struct {
	UserID         uint64
	VideoID        uint64
	VideoSegmentID uint64
	Label          int
	Weight         float64
	Source         string
	Reason         string
	EventTime      time.Time
}

type itemCatalogRow struct {
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
	UserID                         uint64
	GradeID                        uint64
	ClassID                        uint64
	UserType                       int
	MasteryAvg                     float64
	MasteryMin                     float64
	WeakKnowledgeCount             int
	StrongKnowledgeCount           int
	KnowledgeCorrectCount          int
	KnowledgeIncorrectCount        int
	AnswerCount                    int
	AnswerCorrectCount             int
	AnswerIncorrectCount           int
	AvgScoreRate                   float64
	AvgCostSeconds                 float64
	QuestionFeedbackCount          int
	GeneratedFeedbackCount         int
	GeneratedCorrectCount          int
	GeneratedAvgScoreRate          float64
	QuestionSearchCount            int
	RecentKnowledgePointIDs        string
	RecentSubjects                 string
	QuestionSearchKnowledgeText    string
	GeneratedFeedbackKnowledgeText string
}

type pairKey struct {
	UserID    uint64
	SegmentID uint64
}

func main() {
	config.EnsureProjectRoot()
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{
		configFile: defaultConfigPath,
		limit:      defaultLimit,
		seed:       time.Now().UnixNano(),
		daysBack:   defaultDaysBack,
	}
	fs := flag.NewFlagSet("export_two_tower_samples", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configFile, "config", opts.configFile, "config file used for PostgreSQL DSN")
	fs.StringVar(&opts.outputFile, "output", "", "CSV output path; empty means stdout")
	fs.StringVar(&opts.itemOutputFile, "item-output", "", "optional CSV output path for all eligible item features")
	fs.StringVar(&opts.userOutputFile, "user-output", "", "optional CSV output path for all eligible user learning features")
	fs.IntVar(&opts.limit, "limit", opts.limit, "maximum exported sample rows")
	fs.IntVar(&opts.seedCount, "seed-count", 0, "generate N reasonable synthetic behavior rows before export")
	fs.Int64Var(&opts.seed, "seed", opts.seed, "random seed for generated data")
	fs.IntVar(&opts.daysBack, "days-back", opts.daysBack, "spread generated event_time across the last N days")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print plan without writing generated data")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.configFile = strings.TrimSpace(opts.configFile)
	opts.outputFile = strings.TrimSpace(opts.outputFile)
	opts.itemOutputFile = strings.TrimSpace(opts.itemOutputFile)
	opts.userOutputFile = strings.TrimSpace(opts.userOutputFile)
	if opts.configFile == "" {
		return options{}, errors.New("config is required")
	}
	if opts.limit <= 0 {
		return options{}, errors.New("limit must be greater than 0")
	}
	if opts.seedCount < 0 {
		return options{}, errors.New("seed-count must be >= 0")
	}
	if opts.daysBack <= 0 {
		return options{}, errors.New("days-back must be greater than 0")
	}
	return opts, nil
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	csvOut, logOut := selectOutputWriters(opts.outputFile, stdout, stderr)
	cfg := config.MustLoad(opts.configFile)
	if strings.TrimSpace(cfg.Postgres.DSN) == "" {
		return errors.New("postgres dsn is required")
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
	segments, err := loadEligibleSegments(ctx, db)
	if err != nil {
		return fmt.Errorf("load eligible segments: %w", err)
	}

	fmt.Fprintf(logOut, "config=%s\n", opts.configFile)
	fmt.Fprintf(logOut, "postgres=%s\n", maskDSN(cfg.Postgres.DSN))
	fmt.Fprintf(logOut, "user_source=sys_user.%s users=%d\n", userIDColumn, len(users))
	fmt.Fprintf(logOut, "segment_source=edu_video_segment JOIN edu_video_resource eligible_segments=%d\n", len(segments))
	if opts.itemOutputFile != "" {
		items, err := loadItemCatalog(ctx, db)
		if err != nil {
			return fmt.Errorf("load item catalog: %w", err)
		}
		file, err := os.Create(opts.itemOutputFile)
		if err != nil {
			return err
		}
		if err := writeItemCatalogCSV(file, items); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		fmt.Fprintf(logOut, "item_output=%s exported_items=%d\n", opts.itemOutputFile, len(items))
	}
	if opts.userOutputFile != "" {
		users, err := loadUserFeatureCatalog(ctx, db, userIDColumn, userColumns)
		if err != nil {
			return fmt.Errorf("load user feature catalog: %w", err)
		}
		file, err := os.Create(opts.userOutputFile)
		if err != nil {
			return err
		}
		if err := writeUserFeatureCSV(file, users); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		fmt.Fprintf(logOut, "user_output=%s exported_users=%d\n", opts.userOutputFile, len(users))
	}

	if opts.seedCount > 0 {
		existingPairs, err := loadExistingBehaviorPairs(ctx, db)
		if err != nil {
			return fmt.Errorf("load existing behavior pairs: %w", err)
		}
		plans, err := buildSeedPlansExcluding(users, segments, opts.seedCount, existingPairs, rand.New(rand.NewSource(opts.seed)), time.Now(), time.Duration(opts.daysBack)*24*time.Hour)
		if err != nil {
			return err
		}
		fmt.Fprintf(logOut, "seed=%d seed_count=%d dry_run=%v\n", opts.seed, len(plans), opts.dryRun)
		printSeedSummary(logOut, plans)
		if !opts.dryRun {
			inserted, err := insertSeedPlans(ctx, db, plans)
			if err != nil {
				return err
			}
			fmt.Fprintf(logOut, "seed_inserted=%d\n", inserted)
		}
	}

	events, err := loadSampleEvents(ctx, db, opts.limit, userIDColumn, userColumns)
	if err != nil {
		return err
	}
	samples := buildSampleRows(events)
	fmt.Fprintf(logOut, "sample_events=%d exported_samples=%d\n", len(events), len(samples))

	if opts.outputFile == "" {
		return writeSamplesCSV(csvOut, samples)
	}
	file, err := os.Create(opts.outputFile)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeSamplesCSV(file, samples); err != nil {
		return err
	}
	fmt.Fprintf(logOut, "output=%s\n", opts.outputFile)
	return nil
}

func selectOutputWriters(outputFile string, stdout io.Writer, stderr io.Writer) (csvOut io.Writer, logOut io.Writer) {
	if outputFile == "" {
		return stdout, stderr
	}
	return stdout, stdout
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

func listExistingTables(ctx context.Context, db *sql.DB, names ...string) (map[string]bool, error) {
	if len(names) == 0 {
		return map[string]bool{}, nil
	}
	placeholders := make([]string, 0, len(names))
	args := make([]any, 0, len(names))
	for i, name := range names {
		placeholders = append(placeholders, "$"+strconv.Itoa(i+1))
		args = append(args, name)
	}
	rows, err := db.QueryContext(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		existing[name] = true
	}
	return existing, rows.Err()
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
	conditions := sysUserConditions("u", idColumn, columns)
	query := fmt.Sprintf(
		"SELECT user_id FROM (SELECT DISTINCT u.%s AS user_id FROM public.sys_user u WHERE %s) users ORDER BY random()",
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

func sysUserConditions(alias string, idColumn string, columns map[string]columnInfo) []string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	id := prefix + quoteIdent(idColumn)
	conditions := []string{id + " IS NOT NULL", id + " > 0"}
	if col, ok := columns["deleted"]; ok {
		deleted := prefix + quoteIdent("deleted")
		if strings.EqualFold(col.DataType, "boolean") {
			conditions = append(conditions, deleted+" = FALSE")
		} else {
			conditions = append(conditions, "COALESCE("+deleted+", 0) = 0")
		}
	}
	if _, ok := columns["del_flag"]; ok {
		delFlag := prefix + quoteIdent("del_flag")
		conditions = append(conditions, "COALESCE("+delFlag+"::text, '0') = '0'")
	}
	return conditions
}

func loadEligibleSegments(ctx context.Context, db *sql.DB) ([]eligibleSegment, error) {
	rows, err := db.QueryContext(ctx, `
SELECT s.id,
       s.video_id,
       GREATEST(COALESCE(s.end_time, 0) - COALESCE(s.start_time, 0), 1) AS segment_duration
FROM public.edu_video_segment s
JOIN public.edu_video_resource r ON r.id = s.video_id
WHERE s.deleted = 0
  AND s.status = 1
  AND s.id > 0
  AND s.video_id > 0
  AND r.deleted = 0
  AND r.id > 0
ORDER BY random()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []eligibleSegment
	for rows.Next() {
		var segment eligibleSegment
		if err := rows.Scan(&segment.ID, &segment.VideoID, &segment.SegmentDuration); err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}
	return segments, rows.Err()
}

func loadItemCatalog(ctx context.Context, db *sql.DB) ([]itemCatalogRow, error) {
	rows, err := db.QueryContext(ctx, buildItemCatalogQuery())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []itemCatalogRow
	for rows.Next() {
		var item itemCatalogRow
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

func loadUserFeatureCatalog(ctx context.Context, db *sql.DB, userIDColumn string, userColumns map[string]columnInfo) ([]userFeatureRow, error) {
	tableColumns, err := listExistingTableColumns(
		ctx,
		db,
		"edu_user_knowledge_mastery",
		"edu_knowledge_answer_record",
		"edu_user_question_feedback",
		"edu_generated_question_feedback",
		"edu_question_search_record",
	)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, buildUserFeatureCatalogQueryWithColumns(userIDColumn, userColumns, tableColumns))
	if err != nil {
		return nil, err
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
			&user.MasteryMin,
			&user.WeakKnowledgeCount,
			&user.StrongKnowledgeCount,
			&user.KnowledgeCorrectCount,
			&user.KnowledgeIncorrectCount,
			&user.AnswerCount,
			&user.AnswerCorrectCount,
			&user.AnswerIncorrectCount,
			&user.AvgScoreRate,
			&user.AvgCostSeconds,
			&user.QuestionFeedbackCount,
			&user.GeneratedFeedbackCount,
			&user.GeneratedCorrectCount,
			&user.GeneratedAvgScoreRate,
			&user.QuestionSearchCount,
			&user.RecentKnowledgePointIDs,
			&user.RecentSubjects,
			&user.QuestionSearchKnowledgeText,
			&user.GeneratedFeedbackKnowledgeText,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func buildItemCatalogQuery() string {
	return `
SELECT s.id AS video_segment_id,
       s.video_id AS video_id,
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
  AND s.embedding IS NOT NULL
  AND r.deleted = 0
  AND r.id > 0
ORDER BY s.id`
}

func buildUserFeatureCatalogQuery(userIDColumn string, userColumns map[string]columnInfo) string {
	return buildUserFeatureCatalogQueryWithColumns(userIDColumn, userColumns, map[string]map[string]columnInfo{
		"edu_user_knowledge_mastery": {
			"user_id":         {Name: "user_id"},
			"mastery":         {Name: "mastery"},
			"correct_count":   {Name: "correct_count"},
			"incorrect_count": {Name: "incorrect_count"},
		},
		"edu_knowledge_answer_record": {
			"user_id":            {Name: "user_id"},
			"is_correct":         {Name: "is_correct"},
			"score_rate":         {Name: "score_rate"},
			"cost_seconds":       {Name: "cost_seconds"},
			"knowledge_point_id": {Name: "knowledge_point_id"},
		},
		"edu_user_question_feedback": {
			"user_id":        {Name: "user_id"},
			"feedback_count": {Name: "feedback_count"},
		},
		"edu_generated_question_feedback": {
			"user_id":       {Name: "user_id"},
			"is_correct":    {Name: "is_correct"},
			"score_rate":    {Name: "score_rate"},
			"feedback_type": {Name: "feedback_type"},
		},
		"edu_question_search_record": {
			"user_id":                  {Name: "user_id"},
			"deleted":                  {Name: "deleted"},
			"first_question_knowledge": {Name: "first_question_knowledge"},
		},
	})
}

func buildUserFeatureCatalogQueryWithColumns(userIDColumn string, userColumns map[string]columnInfo, tableColumns map[string]map[string]columnInfo) string {
	baseUserSelects := []string{
		"u." + quoteIdent(userIDColumn) + " AS user_id",
		optionalNumericColumn("u", "grade_id", userColumns) + " AS grade_id",
		optionalNumericColumn("u", "class_id", userColumns) + " AS class_id",
		optionalNumericColumn("u", "user_type", userColumns) + " AS user_type",
	}
	ctes := []string{
		`valid_users AS (
  SELECT ` + strings.Join(baseUserSelects, ",\n         ") + `
  FROM public.sys_user u
  WHERE ` + strings.Join(sysUserConditions("u", userIDColumn, userColumns), " AND ") + `
)`,
	}
	joins := []string{}
	if columns := tableColumns["edu_user_knowledge_mastery"]; hasColumn(columns, "user_id") {
		ctes = append(ctes, `mastery AS (
  SELECT user_id,
         `+averageColumn(columns, "mastery")+` AS mastery_avg,
         `+minColumn(columns, "mastery")+` AS mastery_min,
         `+countWhereColumn(columns, "mastery", "mastery < 0.6")+` AS weak_knowledge_count,
         `+countWhereColumn(columns, "mastery", "mastery >= 0.8")+` AS strong_knowledge_count,
         `+sumColumn(columns, "correct_count")+` AS knowledge_correct_count,
         `+sumColumn(columns, "incorrect_count")+` AS knowledge_incorrect_count
  FROM public.edu_user_knowledge_mastery
  GROUP BY user_id
)`)
		joins = append(joins, "LEFT JOIN mastery m ON m.user_id = vu.user_id")
	}
	if columns := tableColumns["edu_knowledge_answer_record"]; hasColumn(columns, "user_id") {
		ctes = append(ctes, `answer AS (
  SELECT user_id,
         COUNT(*) AS answer_count,
         `+countWhereColumn(columns, "is_correct", "is_correct = 1")+` AS answer_correct_count,
         `+countWhereColumn(columns, "is_correct", "is_correct = 0")+` AS answer_incorrect_count,
         `+averageColumn(columns, "score_rate")+` AS avg_score_rate,
         `+averageColumn(columns, "cost_seconds")+` AS avg_cost_seconds,
         `+stringAggColumn(columns, "knowledge_point_id", "|")+` AS recent_knowledge_point_ids
  FROM public.edu_knowledge_answer_record
  WHERE user_id > 0
  GROUP BY user_id
)`)
		joins = append(joins, "LEFT JOIN answer a ON a.user_id = vu.user_id")
	}
	if columns := tableColumns["edu_user_question_feedback"]; hasColumn(columns, "user_id") {
		ctes = append(ctes, `question_feedback AS (
  SELECT user_id,
         `+sumColumn(columns, "feedback_count")+` AS question_feedback_count
  FROM public.edu_user_question_feedback
  GROUP BY user_id
)`)
		joins = append(joins, "LEFT JOIN question_feedback qf ON qf.user_id = vu.user_id")
	}
	if columns := tableColumns["edu_generated_question_feedback"]; hasColumn(columns, "user_id") {
		ctes = append(ctes, `generated_feedback AS (
  SELECT user_id,
         COUNT(*) AS generated_feedback_count,
         `+countWhereColumn(columns, "is_correct", "is_correct = 1")+` AS generated_correct_count,
         `+averageColumn(columns, "score_rate")+` AS generated_avg_score_rate,
         `+stringAggColumn(columns, "feedback_type", "|")+` AS generated_feedback_knowledge_text
  FROM public.edu_generated_question_feedback
  WHERE user_id > 0
  GROUP BY user_id
)`)
		joins = append(joins, "LEFT JOIN generated_feedback gf ON gf.user_id = vu.user_id")
	}
	if columns := tableColumns["edu_question_search_record"]; hasColumn(columns, "user_id") {
		ctes = append(ctes, `question_search AS (
  SELECT user_id,
         COUNT(*) AS question_search_count,
         `+stringAggColumn(columns, "first_question_knowledge", " ")+` AS question_search_knowledge_text
  FROM public.edu_question_search_record
  WHERE `+deletedFilter(columns)+`
  GROUP BY user_id
)`)
		joins = append(joins, "LEFT JOIN question_search qs ON qs.user_id = vu.user_id")
	}
	selects := []string{
		"vu.user_id",
		"COALESCE(vu.grade_id, 0) AS grade_id",
		"COALESCE(vu.class_id, 0) AS class_id",
		"COALESCE(vu.user_type, 0) AS user_type",
		coalesceColumn(hasColumn(tableColumns["edu_user_knowledge_mastery"], "user_id"), "m.mastery_avg", "0") + " AS mastery_avg",
		coalesceColumn(hasColumn(tableColumns["edu_user_knowledge_mastery"], "user_id"), "m.mastery_min", "0") + " AS mastery_min",
		coalesceColumn(hasColumn(tableColumns["edu_user_knowledge_mastery"], "user_id"), "m.weak_knowledge_count", "0") + " AS weak_knowledge_count",
		coalesceColumn(hasColumn(tableColumns["edu_user_knowledge_mastery"], "user_id"), "m.strong_knowledge_count", "0") + " AS strong_knowledge_count",
		coalesceColumn(hasColumn(tableColumns["edu_user_knowledge_mastery"], "user_id"), "m.knowledge_correct_count", "0") + " AS knowledge_correct_count",
		coalesceColumn(hasColumn(tableColumns["edu_user_knowledge_mastery"], "user_id"), "m.knowledge_incorrect_count", "0") + " AS knowledge_incorrect_count",
		coalesceColumn(hasColumn(tableColumns["edu_knowledge_answer_record"], "user_id"), "a.answer_count", "0") + " AS answer_count",
		coalesceColumn(hasColumn(tableColumns["edu_knowledge_answer_record"], "user_id"), "a.answer_correct_count", "0") + " AS answer_correct_count",
		coalesceColumn(hasColumn(tableColumns["edu_knowledge_answer_record"], "user_id"), "a.answer_incorrect_count", "0") + " AS answer_incorrect_count",
		coalesceColumn(hasColumn(tableColumns["edu_knowledge_answer_record"], "user_id"), "a.avg_score_rate", "0") + " AS avg_score_rate",
		coalesceColumn(hasColumn(tableColumns["edu_knowledge_answer_record"], "user_id"), "a.avg_cost_seconds", "0") + " AS avg_cost_seconds",
		coalesceColumn(hasColumn(tableColumns["edu_user_question_feedback"], "user_id"), "qf.question_feedback_count", "0") + " AS question_feedback_count",
		coalesceColumn(hasColumn(tableColumns["edu_generated_question_feedback"], "user_id"), "gf.generated_feedback_count", "0") + " AS generated_feedback_count",
		coalesceColumn(hasColumn(tableColumns["edu_generated_question_feedback"], "user_id"), "gf.generated_correct_count", "0") + " AS generated_correct_count",
		coalesceColumn(hasColumn(tableColumns["edu_generated_question_feedback"], "user_id"), "gf.generated_avg_score_rate", "0") + " AS generated_avg_score_rate",
		coalesceColumn(hasColumn(tableColumns["edu_question_search_record"], "user_id"), "qs.question_search_count", "0") + " AS question_search_count",
		coalesceColumn(hasColumn(tableColumns["edu_knowledge_answer_record"], "user_id"), "a.recent_knowledge_point_ids", "''") + " AS recent_knowledge_point_ids",
		"'' AS recent_subjects",
		coalesceColumn(hasColumn(tableColumns["edu_question_search_record"], "user_id"), "qs.question_search_knowledge_text", "''") + " AS question_search_knowledge_text",
		coalesceColumn(hasColumn(tableColumns["edu_generated_question_feedback"], "user_id"), "gf.generated_feedback_knowledge_text", "''") + " AS generated_feedback_knowledge_text",
	}
	return `WITH ` + strings.Join(ctes, ",\n") + `
SELECT ` + strings.Join(selects, ",\n       ") + `
FROM valid_users vu
` + strings.Join(joins, "\n") + `
ORDER BY vu.user_id`
}

func listExistingTableColumns(ctx context.Context, db *sql.DB, names ...string) (map[string]map[string]columnInfo, error) {
	existing, err := listExistingTables(ctx, db, names...)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]columnInfo{}
	for _, name := range names {
		if !existing[name] {
			continue
		}
		columns, err := listColumns(ctx, db, name)
		if err != nil {
			return nil, err
		}
		result[name] = columns
	}
	return result, nil
}

func hasColumn(columns map[string]columnInfo, name string) bool {
	_, ok := columns[name]
	return ok
}

func averageColumn(columns map[string]columnInfo, name string) string {
	if !hasColumn(columns, name) {
		return "0"
	}
	return "COALESCE(AVG(" + quoteIdent(name) + "), 0)"
}

func minColumn(columns map[string]columnInfo, name string) string {
	if !hasColumn(columns, name) {
		return "0"
	}
	return "COALESCE(MIN(" + quoteIdent(name) + "), 0)"
}

func sumColumn(columns map[string]columnInfo, name string) string {
	if !hasColumn(columns, name) {
		return "0"
	}
	return "COALESCE(SUM(" + quoteIdent(name) + "), 0)"
}

func countWhereColumn(columns map[string]columnInfo, name string, condition string) string {
	if !hasColumn(columns, name) {
		return "0"
	}
	return "COUNT(*) FILTER (WHERE " + condition + ")"
}

func stringAggColumn(columns map[string]columnInfo, name string, separator string) string {
	if !hasColumn(columns, name) {
		return "''"
	}
	column := "COALESCE(" + quoteIdent(name) + "::text, '')"
	return "COALESCE(string_agg(DISTINCT " + column + ", '" + separator + "' ORDER BY " + column + "), '')"
}

func deletedFilter(columns map[string]columnInfo) string {
	if !hasColumn(columns, "deleted") {
		return "TRUE"
	}
	return "COALESCE(" + quoteIdent("deleted") + ", 0) = 0"
}

func optionalNumericColumn(alias string, name string, columns map[string]columnInfo) string {
	if _, ok := columns[name]; !ok {
		return "0"
	}
	return alias + "." + quoteIdent(name)
}

func coalesceColumn(enabled bool, column string, fallback string) string {
	if !enabled {
		return fallback
	}
	return "COALESCE(" + column + ", " + fallback + ")"
}

func loadExistingBehaviorPairs(ctx context.Context, db *sql.DB) (map[pairKey]bool, error) {
	rows, err := db.QueryContext(ctx, `
SELECT user_id, video_segment_id FROM public.edu_user_reaction
UNION
SELECT user_id, video_segment_id FROM public.edu_user_video_recommend
UNION
SELECT user_id, video_segment_id FROM public.edu_recommend_exposure`)
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

func buildSeedPlans(users []uint64, segments []eligibleSegment, count int, rng *rand.Rand, now time.Time, window time.Duration) ([]seedPlan, error) {
	return buildSeedPlansExcluding(users, segments, count, nil, rng, now, window)
}

func buildSeedPlansExcluding(users []uint64, segments []eligibleSegment, count int, excluded map[pairKey]bool, rng *rand.Rand, now time.Time, window time.Duration) ([]seedPlan, error) {
	if count < 0 {
		return nil, errors.New("seed-count must be >= 0")
	}
	if count == 0 {
		return nil, nil
	}
	if len(users) == 0 {
		return nil, errors.New("sys_user has no eligible users")
	}
	if len(segments) == 0 {
		return nil, errors.New("edu_video_segment has no eligible segments")
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if window <= 0 {
		window = defaultDaysBack * 24 * time.Hour
	}
	capacity := availableCapacity(users, segments, excluded)
	if count > capacity {
		return nil, fmt.Errorf("not enough unique user/segment pairs: requested=%d available=%d", count, capacity)
	}

	users = append([]uint64(nil), users...)
	segments = append([]eligibleSegment(nil), segments...)
	rng.Shuffle(len(users), func(i, j int) { users[i], users[j] = users[j], users[i] })
	rng.Shuffle(len(segments), func(i, j int) { segments[i], segments[j] = segments[j], segments[i] })

	used := make(map[pairKey]bool, count)
	plans := make([]seedPlan, 0, count)
	maxAttempts := count*200 + 1000
	for attempts := 0; len(plans) < count && attempts < maxAttempts; attempts++ {
		userID := users[rng.Intn(len(users))]
		segment := segments[rng.Intn(len(segments))]
		pair := pairKey{UserID: userID, SegmentID: segment.ID}
		if excluded[pair] || used[pair] {
			continue
		}
		used[pair] = true
		plans = append(plans, newSeedPlan(userID, segment, rng, now, window))
	}
	if len(plans) < count {
		for _, userID := range users {
			for _, segment := range segments {
				if len(plans) >= count {
					break
				}
				pair := pairKey{UserID: userID, SegmentID: segment.ID}
				if excluded[pair] || used[pair] {
					continue
				}
				used[pair] = true
				plans = append(plans, newSeedPlan(userID, segment, rng, now, window))
			}
		}
	}
	return plans, nil
}

func availableCapacity(users []uint64, segments []eligibleSegment, excluded map[pairKey]bool) int {
	capacity := len(users) * len(segments)
	if len(excluded) == 0 {
		return capacity
	}
	userSet := make(map[uint64]bool, len(users))
	segmentSet := make(map[uint64]bool, len(segments))
	for _, userID := range users {
		userSet[userID] = true
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

func newSeedPlan(userID uint64, segment eligibleSegment, rng *rand.Rand, now time.Time, window time.Duration) seedPlan {
	eventTime := randomPastTime(rng, now, window)
	reactionType, clicked, watched := chooseSyntheticBehavior(rng)
	watchDuration := 0
	if watched {
		minDuration := maxInt(1, segment.SegmentDuration/3)
		watchDuration = minDuration + rng.Intn(maxInt(1, segment.SegmentDuration-minDuration+1))
	}
	return seedPlan{
		UserID:          userID,
		VideoID:         segment.VideoID,
		VideoSegmentID:  segment.ID,
		SegmentDuration: segment.SegmentDuration,
		ReactionType:    reactionType,
		Score:           0.25 + rng.Float64()*0.7,
		Rank:            1 + rng.Intn(20),
		Clicked:         clicked,
		Watched:         watched,
		WatchDuration:   watchDuration,
		EventTime:       eventTime,
	}
}

func chooseSyntheticBehavior(rng *rand.Rand) (reactionType string, clicked bool, watched bool) {
	n := rng.Intn(100)
	switch {
	case n < 42:
		return "like", true, true
	case n < 55:
		return "double_like", true, true
	case n < 68:
		return "", true, true
	case n < 82:
		return "dislike", false, false
	default:
		return "", false, false
	}
}

func randomPastTime(rng *rand.Rand, now time.Time, window time.Duration) time.Time {
	if window <= 0 {
		return now
	}
	offset := time.Duration(rng.Int63n(int64(window)))
	return now.Add(-offset).Truncate(time.Second)
}

func insertSeedPlans(ctx context.Context, db *sql.DB, plans []seedPlan) (int, error) {
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
		requestID := fmt.Sprintf("seed-%d-%d-%d", plan.UserID, plan.VideoSegmentID, plan.EventTime.UnixNano())
		if _, err := tx.ExecContext(ctx, `
INSERT INTO public.edu_recommend_exposure
  (request_id, user_id, question_id, video_id, video_segment_id, rank, score, strategy, model_version,
   clicked, watched, clicked_time, watched_time, deleted, create_time, update_time)
VALUES
  ($1, $2, 0, $3, $4, $5, $6, 'seed_training', 'seed_v1',
   $7, $8, $9, $10, 0, $11, $11)`,
			requestID,
			plan.UserID,
			plan.VideoID,
			plan.VideoSegmentID,
			plan.Rank,
			plan.Score,
			plan.Clicked,
			plan.Watched,
			nullableTime(plan.Clicked, plan.EventTime),
			nullableTime(plan.Watched, plan.EventTime),
			plan.EventTime,
		); err != nil {
			return 0, fmt.Errorf("insert exposure user=%d segment=%d: %w", plan.UserID, plan.VideoSegmentID, err)
		}
		inserted++
		if plan.Watched {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO public.edu_user_video_recommend
  (user_id, video_id, question_id, video_segment_id, recommend_score, is_watched, watch_duration, deleted, create_time, update_time)
VALUES
  ($1, $2, 0, $3, $4, TRUE, $5, 0, $6, $6)
ON CONFLICT (user_id, question_id, video_segment_id)
DO UPDATE SET
  video_id = EXCLUDED.video_id,
  recommend_score = EXCLUDED.recommend_score,
  is_watched = TRUE,
  watch_duration = GREATEST(COALESCE(public.edu_user_video_recommend.watch_duration, 0), EXCLUDED.watch_duration),
  deleted = 0,
  update_time = EXCLUDED.update_time`,
				plan.UserID,
				plan.VideoID,
				plan.VideoSegmentID,
				plan.Score,
				plan.WatchDuration,
				plan.EventTime,
			); err != nil {
				return 0, fmt.Errorf("upsert watch user=%d segment=%d: %w", plan.UserID, plan.VideoSegmentID, err)
			}
		}
		if plan.ReactionType != "" {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO public.edu_user_reaction
  (user_id, video_id, video_segment_id, reaction_type, create_time, update_time, deleted)
VALUES
  ($1, $2, $3, $4, $5, $5, 0)
ON CONFLICT (user_id, video_segment_id)
DO UPDATE SET
  video_id = EXCLUDED.video_id,
  reaction_type = EXCLUDED.reaction_type,
  deleted = 0,
  update_time = EXCLUDED.update_time`,
				plan.UserID,
				plan.VideoID,
				plan.VideoSegmentID,
				plan.ReactionType,
				plan.EventTime,
			); err != nil {
				return 0, fmt.Errorf("upsert reaction user=%d segment=%d: %w", plan.UserID, plan.VideoSegmentID, err)
			}
		}
	}

	for _, segmentID := range uniqueSegmentIDs(plans) {
		if _, err := tx.ExecContext(ctx, `
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
WHERE id = $1 AND deleted = 0`, segmentID); err != nil {
			return 0, fmt.Errorf("update segment %d counts: %w", segmentID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return inserted, nil
}

func nullableTime(ok bool, t time.Time) any {
	if !ok {
		return nil
	}
	return t
}

func uniqueSegmentIDs(plans []seedPlan) []uint64 {
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

func loadSampleEvents(ctx context.Context, db *sql.DB, limit int, userIDColumn string, userColumns map[string]columnInfo) ([]sampleEvent, error) {
	rows, err := db.QueryContext(ctx, buildSampleEventsQuery(userIDColumn, userColumns), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]sampleEvent, 0, limit)
	for rows.Next() {
		var event sampleEvent
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
			&event.Rank,
			&event.EventTime,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func buildSampleEventsQuery(userIDColumn string, userColumns map[string]columnInfo) string {
	return fmt.Sprintf(`
WITH valid_segments AS (
  SELECT s.id AS video_segment_id,
         s.video_id AS video_id,
         GREATEST(COALESCE(s.end_time, 0) - COALESCE(s.start_time, 0), 1) AS segment_duration
  FROM public.edu_video_segment s
  JOIN public.edu_video_resource r ON r.id = s.video_id
  WHERE s.deleted = 0
    AND s.status = 1
    AND s.id > 0
    AND s.video_id > 0
    AND r.deleted = 0
),
valid_users AS (
  SELECT DISTINCT u.%s AS user_id
  FROM public.sys_user u
  WHERE %s
),
events AS (
  SELECT ur.user_id,
         vs.video_id,
         ur.video_segment_id,
         'segment_reaction' AS source,
         ur.reaction_type,
         FALSE AS clicked,
         FALSE AS watched,
         0 AS watch_duration,
         vs.segment_duration,
         0 AS rank,
         ur.update_time AS event_time
  FROM public.edu_user_reaction ur
  JOIN valid_segments vs ON vs.video_segment_id = ur.video_segment_id AND vs.video_id = ur.video_id
  WHERE ur.deleted = 0
    AND ur.reaction_type IN ('like', 'double_like', 'dislike')

  UNION ALL

  SELECT vur.user_id,
         vs.video_id,
         vs.video_segment_id,
         'video_reaction' AS source,
         vur.reaction_type,
         FALSE AS clicked,
         FALSE AS watched,
         0 AS watch_duration,
         vs.segment_duration,
         0 AS rank,
         vur.update_time AS event_time
  FROM public.edu_video_user_reaction vur
  JOIN valid_segments vs ON vs.video_id = vur.video_id
  WHERE vur.deleted = 0
    AND vur.reaction_type IN ('like', 'double_like', 'dislike')

  UNION ALL

  SELECT uvr.user_id,
         vs.video_id,
         uvr.video_segment_id,
         'watch' AS source,
         '' AS reaction_type,
         TRUE AS clicked,
         COALESCE(uvr.is_watched, FALSE) AS watched,
         COALESCE(uvr.watch_duration, 0) AS watch_duration,
         vs.segment_duration,
         0 AS rank,
         uvr.update_time AS event_time
  FROM public.edu_user_video_recommend uvr
  JOIN valid_segments vs ON vs.video_segment_id = uvr.video_segment_id AND vs.video_id = uvr.video_id
  WHERE uvr.deleted = 0
    AND COALESCE(uvr.is_watched, FALSE) = TRUE

  UNION ALL

  SELECT e.user_id,
         vs.video_id,
         e.video_segment_id,
         'exposure' AS source,
         '' AS reaction_type,
         COALESCE(e.clicked, FALSE) AS clicked,
         COALESCE(e.watched, FALSE) AS watched,
         0 AS watch_duration,
         vs.segment_duration,
         COALESCE(e.rank, 0) AS rank,
         e.create_time AS event_time
  FROM public.edu_recommend_exposure e
  JOIN valid_segments vs ON vs.video_segment_id = e.video_segment_id AND vs.video_id = e.video_id
  WHERE e.deleted = 0
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
       e.rank,
       e.event_time
FROM events e
JOIN valid_users vu ON vu.user_id = e.user_id
ORDER BY e.event_time DESC
LIMIT $1`,
		quoteIdent(userIDColumn),
		strings.Join(sysUserConditions("u", userIDColumn, userColumns), " AND "),
	)
}

func buildSampleRows(events []sampleEvent) []sampleRow {
	rows := make([]sampleRow, 0, len(events))
	for _, event := range events {
		row, ok := sampleRowFromEvent(event)
		if !ok {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func sampleRowFromEvent(event sampleEvent) (sampleRow, bool) {
	row := sampleRow{
		UserID:         event.UserID,
		VideoID:        event.VideoID,
		VideoSegmentID: event.VideoSegmentID,
		Source:         event.Source,
		EventTime:      event.EventTime,
	}
	switch event.Source {
	case sourceSegmentReaction:
		switch event.ReactionType {
		case "double_like":
			row.Label = 1
			row.Weight = 3.0
			row.Reason = "double_like"
		case "like":
			row.Label = 1
			row.Weight = 2.0
			row.Reason = "like"
		case "dislike":
			row.Label = 0
			row.Weight = 2.0
			row.Reason = "dislike"
		default:
			return sampleRow{}, false
		}
	case sourceVideoReaction:
		switch event.ReactionType {
		case "double_like":
			row.Label = 1
			row.Weight = 1.5
			row.Reason = "video_double_like"
		case "like":
			row.Label = 1
			row.Weight = 1.0
			row.Reason = "video_like"
		case "dislike":
			row.Label = 0
			row.Weight = 1.0
			row.Reason = "video_dislike"
		default:
			return sampleRow{}, false
		}
	case sourceWatch:
		if event.WatchDuration <= 0 {
			return sampleRow{}, false
		}
		ratio := float64(event.WatchDuration) / float64(maxInt(event.SegmentDuration, 1))
		row.Label = 1
		row.Reason = "watched"
		switch {
		case ratio >= 0.8:
			row.Weight = 1.2
		case ratio >= 0.4:
			row.Weight = 0.7
		default:
			row.Weight = 0.3
		}
	case sourceExposure:
		switch {
		case event.Watched:
			row.Label = 1
			row.Weight = 1.0
			row.Reason = "exposure_watched"
		case event.Clicked:
			row.Label = 1
			row.Weight = 0.8
			row.Reason = "exposure_clicked"
		default:
			row.Label = 0
			row.Weight = 0.2
			row.Reason = "exposure_no_click"
		}
	default:
		return sampleRow{}, false
	}
	return row, true
}

func writeSamplesCSV(out io.Writer, rows []sampleRow) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{"user_id", "video_id", "video_segment_id", "label", "weight", "source", "reason", "event_time"}); err != nil {
		return err
	}
	for _, row := range rows {
		record := []string{
			strconv.FormatUint(row.UserID, 10),
			strconv.FormatUint(row.VideoID, 10),
			strconv.FormatUint(row.VideoSegmentID, 10),
			strconv.Itoa(row.Label),
			strconv.FormatFloat(row.Weight, 'f', 3, 64),
			row.Source,
			row.Reason,
			row.EventTime.Format(time.RFC3339),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeItemCatalogCSV(out io.Writer, rows []itemCatalogRow) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{
		"video_segment_id",
		"video_id",
		"segment_duration",
		"video_duration",
		"like_count",
		"double_like_count",
		"dislike_count",
		"content_summary",
		"knowledge_tags",
		"video_title",
	}); err != nil {
		return err
	}
	for _, row := range rows {
		record := []string{
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
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeUserFeatureCSV(out io.Writer, rows []userFeatureRow) error {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{
		"user_id",
		"grade_id",
		"class_id",
		"user_type",
		"mastery_avg",
		"mastery_min",
		"weak_knowledge_count",
		"strong_knowledge_count",
		"knowledge_correct_count",
		"knowledge_incorrect_count",
		"answer_count",
		"answer_correct_count",
		"answer_incorrect_count",
		"avg_score_rate",
		"avg_cost_seconds",
		"question_feedback_count",
		"generated_feedback_count",
		"generated_correct_count",
		"generated_avg_score_rate",
		"question_search_count",
		"recent_knowledge_point_ids",
		"recent_subjects",
		"question_search_knowledge_text",
		"generated_feedback_knowledge_text",
	}); err != nil {
		return err
	}
	for _, row := range rows {
		record := []string{
			strconv.FormatUint(row.UserID, 10),
			strconv.FormatUint(row.GradeID, 10),
			strconv.FormatUint(row.ClassID, 10),
			strconv.Itoa(row.UserType),
			strconv.FormatFloat(row.MasteryAvg, 'f', 6, 64),
			strconv.FormatFloat(row.MasteryMin, 'f', 6, 64),
			strconv.Itoa(row.WeakKnowledgeCount),
			strconv.Itoa(row.StrongKnowledgeCount),
			strconv.Itoa(row.KnowledgeCorrectCount),
			strconv.Itoa(row.KnowledgeIncorrectCount),
			strconv.Itoa(row.AnswerCount),
			strconv.Itoa(row.AnswerCorrectCount),
			strconv.Itoa(row.AnswerIncorrectCount),
			strconv.FormatFloat(row.AvgScoreRate, 'f', 6, 64),
			strconv.FormatFloat(row.AvgCostSeconds, 'f', 6, 64),
			strconv.Itoa(row.QuestionFeedbackCount),
			strconv.Itoa(row.GeneratedFeedbackCount),
			strconv.Itoa(row.GeneratedCorrectCount),
			strconv.FormatFloat(row.GeneratedAvgScoreRate, 'f', 6, 64),
			strconv.Itoa(row.QuestionSearchCount),
			row.RecentKnowledgePointIDs,
			row.RecentSubjects,
			row.QuestionSearchKnowledgeText,
			row.GeneratedFeedbackKnowledgeText,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func printSeedSummary(out io.Writer, plans []seedPlan) {
	counts := map[string]int{}
	users := map[uint64]bool{}
	segments := map[uint64]bool{}
	for _, plan := range plans {
		users[plan.UserID] = true
		segments[plan.VideoSegmentID] = true
		key := "exposure_no_click"
		if plan.ReactionType != "" {
			key = plan.ReactionType
		} else if plan.Watched {
			key = "watch"
		}
		counts[key]++
	}
	fmt.Fprintf(out, "planned_distinct_users=%d planned_distinct_segments=%d\n", len(users), len(segments))
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(out, "planned_%s=%d\n", key, counts[key])
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

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
