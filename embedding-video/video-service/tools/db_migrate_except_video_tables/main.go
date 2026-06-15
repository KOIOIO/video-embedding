package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var preservedTables = map[string]bool{
	"edu_user_video_recommend": true,
	"edu_video_resource":       true,
	"edu_video_segment":        true,
	"edu_video_segement":       true,
}

type tableColumn struct {
	Name string
}

func main() {
	dryRun := flag.Bool("dry-run", true, "inspect source and target without mutating target data")
	createMissing := flag.Bool("create-missing", true, "create source tables that are missing in the target database")
	sourceDSN := flag.String("source-dsn", strings.TrimSpace(os.Getenv("SOURCE_DSN")), "source PostgreSQL DSN, or SOURCE_DSN")
	targetDSN := flag.String("target-dsn", strings.TrimSpace(os.Getenv("TARGET_DSN")), "target PostgreSQL DSN, or TARGET_DSN")
	flag.Parse()
	if strings.TrimSpace(*sourceDSN) == "" {
		log.Fatal("source DSN is required: set -source-dsn or SOURCE_DSN")
	}
	if strings.TrimSpace(*targetDSN) == "" {
		log.Fatal("target DSN is required: set -target-dsn or TARGET_DSN")
	}

	ctx := context.Background()
	src, err := sql.Open("pgx", *sourceDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()

	dst, err := sql.Open("pgx", *targetDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer dst.Close()

	if err := ping(ctx, "source", src); err != nil {
		log.Fatal(err)
	}
	if err := ping(ctx, "target", dst); err != nil {
		log.Fatal(err)
	}

	sourceTables, err := listTables(ctx, src)
	if err != nil {
		log.Fatal(err)
	}
	targetTables, err := listTables(ctx, dst)
	if err != nil {
		log.Fatal(err)
	}

	targetSet := make(map[string]bool, len(targetTables))
	for _, table := range targetTables {
		targetSet[table] = true
	}

	var copyTables []string
	var missingToCreate []string
	var skippedMissing []string
	for _, table := range sourceTables {
		if preservedTables[table] {
			continue
		}
		if !targetSet[table] {
			if *createMissing {
				missingToCreate = append(missingToCreate, table)
				copyTables = append(copyTables, table)
			} else {
				skippedMissing = append(skippedMissing, table)
			}
			continue
		}
		copyTables = append(copyTables, table)
	}

	sort.Strings(copyTables)
	sort.Strings(missingToCreate)
	sort.Strings(skippedMissing)

	fmt.Printf("source tables: %d\n", len(sourceTables))
	fmt.Printf("target tables: %d\n", len(targetTables))
	fmt.Printf("tables to migrate: %d\n", len(copyTables))
	for _, table := range copyTables {
		srcCount, _ := countRows(ctx, src, table)
		if targetSet[table] {
			dstCount, _ := countRows(ctx, dst, table)
			fmt.Printf("  %s source=%d target_before=%d\n", table, srcCount, dstCount)
			continue
		}
		fmt.Printf("  %s source=%d target_before=<missing; will create>\n", table, srcCount)
	}
	if len(missingToCreate) > 0 {
		fmt.Printf("missing target tables to create: %s\n", strings.Join(missingToCreate, ", "))
	}
	if len(skippedMissing) > 0 {
		fmt.Printf("skipped because missing in target: %s\n", strings.Join(skippedMissing, ", "))
	}
	fmt.Println("preserved tables:")
	for table := range preservedTables {
		if targetSet[table] {
			count, _ := countRows(ctx, dst, table)
			fmt.Printf("  %s target_count=%d\n", table, count)
		}
	}

	if *dryRun {
		fmt.Println("dry run complete; rerun with -dry-run=false to migrate data")
		return
	}

	backupSchema := fmt.Sprintf("migration_backup_%s", time.Now().Format("20060102_150405"))
	if err := backupTarget(ctx, dst, backupSchema, targetTables); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("target backup schema created: %s\n", backupSchema)

	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, "SET session_replication_role = replica"); err != nil {
		log.Fatalf("disable target constraints/triggers: %v", err)
	}
	if _, err := tx.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		log.Fatalf("ensure vector extension: %v", err)
	}
	for _, table := range missingToCreate {
		if err := createTableLikeSource(ctx, src, tx, table); err != nil {
			log.Fatalf("create target table %s: %v", table, err)
		}
		fmt.Printf("created target table %s\n", table)
	}
	for _, table := range copyTables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM public."+quoteIdent(table)); err != nil {
			log.Fatalf("clear target table %s: %v", table, err)
		}
	}
	for _, table := range copyTables {
		n, err := copyTable(ctx, src, tx, table)
		if err != nil {
			log.Fatalf("copy table %s: %v", table, err)
		}
		fmt.Printf("copied %s rows=%d\n", table, n)
	}
	for _, table := range missingToCreate {
		if err := addNonForeignKeyConstraints(ctx, src, tx, table); err != nil {
			log.Fatalf("add constraints for %s: %v", table, err)
		}
		if err := addStandaloneIndexes(ctx, src, tx, table); err != nil {
			log.Fatalf("add indexes for %s: %v", table, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "SET session_replication_role = origin"); err != nil {
		log.Fatalf("restore target constraints/triggers: %v", err)
	}
	if err := resetSequences(ctx, tx); err != nil {
		log.Fatalf("reset target sequences: %v", err)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	committed = true

	fmt.Println("migration complete")
	for _, table := range copyTables {
		srcCount, _ := countRows(ctx, src, table)
		dstCount, _ := countRows(ctx, dst, table)
		fmt.Printf("  %s source=%d target_after=%d\n", table, srcCount, dstCount)
	}
}

func ping(ctx context.Context, name string, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping %s: %w", name, err)
	}
	fmt.Fprintf(os.Stderr, "connected to %s\n", name)
	return nil
}

func listTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		out = append(out, table)
	}
	return out, rows.Err()
}

func countRows(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM public."+quoteIdent(table)).Scan(&count)
	return count, err
}

func backupTarget(ctx context.Context, db *sql.DB, schema string, tables []string) error {
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+quoteIdent(schema)); err != nil {
		return err
	}
	for _, table := range tables {
		if _, err := db.ExecContext(ctx, "CREATE TABLE "+quoteIdent(schema)+"."+quoteIdent(table)+" AS TABLE public."+quoteIdent(table)); err != nil {
			return fmt.Errorf("backup %s: %w", table, err)
		}
	}
	return nil
}

func createTableLikeSource(ctx context.Context, src *sql.DB, tx *sql.Tx, table string) error {
	columns, err := describeColumns(ctx, src, table)
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return fmt.Errorf("source table has no columns")
	}

	defs := make([]string, 0, len(columns))
	for _, col := range columns {
		def := quoteIdent(col.Name) + " " + col.Type
		switch {
		case col.Generated != "":
			def += " GENERATED ALWAYS AS (" + col.GenerationExpr + ") STORED"
		case col.Identity != "":
			def += " GENERATED BY DEFAULT AS IDENTITY"
		case strings.HasPrefix(col.Default, "nextval("):
			def += " GENERATED BY DEFAULT AS IDENTITY"
		case col.Default != "":
			def += " DEFAULT " + col.Default
		}
		if col.NotNull {
			def += " NOT NULL"
		}
		defs = append(defs, def)
	}

	_, err = tx.ExecContext(ctx, "CREATE TABLE public."+quoteIdent(table)+" (\n  "+strings.Join(defs, ",\n  ")+"\n)")
	return err
}

type columnDescription struct {
	Name           string
	Type           string
	NotNull        bool
	Default        string
	Identity       string
	Generated      string
	GenerationExpr string
}

func describeColumns(ctx context.Context, db *sql.DB, table string) ([]columnDescription, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
  a.attname,
  pg_catalog.format_type(a.atttypid, a.atttypmod) AS column_type,
  a.attnotnull,
  COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '') AS column_default,
  a.attidentity,
  a.attgenerated,
  CASE WHEN a.attgenerated <> '' THEN COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '') ELSE '' END AS generation_expr
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
WHERE n.nspname = 'public'
  AND c.relname = $1
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY a.attnum`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []columnDescription
	for rows.Next() {
		var col columnDescription
		if err := rows.Scan(&col.Name, &col.Type, &col.NotNull, &col.Default, &col.Identity, &col.Generated, &col.GenerationExpr); err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func addNonForeignKeyConstraints(ctx context.Context, src *sql.DB, tx *sql.Tx, table string) error {
	rows, err := src.QueryContext(ctx, `
SELECT conname, pg_get_constraintdef(oid) AS constraint_def
FROM pg_constraint
WHERE conrelid = ('public.' || quote_ident($1))::regclass
  AND contype IN ('p', 'u', 'c')
ORDER BY CASE contype WHEN 'p' THEN 0 WHEN 'u' THEN 1 ELSE 2 END, conname`, table)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var def string
		if err := rows.Scan(&name, &def); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "ALTER TABLE public."+quoteIdent(table)+" ADD CONSTRAINT "+quoteIdent(name)+" "+def)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return rows.Err()
}

func addStandaloneIndexes(ctx context.Context, src *sql.DB, tx *sql.Tx, table string) error {
	rows, err := src.QueryContext(ctx, `
SELECT pg_get_indexdef(i.indexrelid)
FROM pg_index i
JOIN pg_class t ON t.oid = i.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
LEFT JOIN pg_constraint c ON c.conindid = i.indexrelid
WHERE n.nspname = 'public'
  AND t.relname = $1
  AND c.oid IS NULL
ORDER BY pg_get_indexdef(i.indexrelid)`, table)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var ddl string
		if err := rows.Scan(&ddl); err != nil {
			return err
		}
		if strings.Contains(ddl, " USING ivfflat ") || strings.Contains(ddl, " USING hnsw ") {
			continue
		}
		ddl = strings.Replace(ddl, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ", 1)
		ddl = strings.Replace(ddl, "CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS ", 1)
		if _, err := tx.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyTable(ctx context.Context, src *sql.DB, tx *sql.Tx, table string) (int64, error) {
	cols, err := listColumns(ctx, src, table)
	if err != nil {
		return 0, err
	}
	if len(cols) == 0 {
		return 0, nil
	}

	selectParts := make([]string, 0, len(cols))
	insertCols := make([]string, 0, len(cols))
	placeholders := make([]string, 0, len(cols))
	for i, col := range cols {
		selectParts = append(selectParts, quoteIdent(col.Name)+"::text")
		insertCols = append(insertCols, quoteIdent(col.Name))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	query := "SELECT " + strings.Join(selectParts, ", ") + " FROM public." + quoteIdent(table)
	rows, err := src.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	insertSQL := "INSERT INTO public." + quoteIdent(table) + " (" + strings.Join(insertCols, ", ") + ") OVERRIDING SYSTEM VALUE VALUES (" + strings.Join(placeholders, ", ") + ")"
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	values := make([]any, len(cols))
	scanTargets := make([]any, len(cols))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	var copied int64
	for rows.Next() {
		for i := range values {
			values[i] = nil
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return copied, err
		}
		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return copied, err
		}
		copied++
	}
	return copied, rows.Err()
}

func listColumns(ctx context.Context, db *sql.DB, table string) ([]tableColumn, error) {
	rows, err := db.QueryContext(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND is_generated = 'NEVER'
ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []tableColumn
	for rows.Next() {
		var col tableColumn
		if err := rows.Scan(&col.Name); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func resetSequences(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
SELECT table_name, column_name, pg_get_serial_sequence(format('%I.%I', table_schema, table_name), column_name) AS seq_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND pg_get_serial_sequence(format('%I.%I', table_schema, table_name), column_name) IS NOT NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type seqInfo struct {
		Table  string
		Column string
		Seq    string
	}
	var seqs []seqInfo
	for rows.Next() {
		var seq seqInfo
		if err := rows.Scan(&seq.Table, &seq.Column, &seq.Seq); err != nil {
			return err
		}
		seqs = append(seqs, seq)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, seq := range seqs {
		query := fmt.Sprintf(
			"SELECT setval($1, COALESCE((SELECT MAX(%s) FROM public.%s), 1), (SELECT COUNT(*) > 0 FROM public.%s))",
			quoteIdent(seq.Column),
			quoteIdent(seq.Table),
			quoteIdent(seq.Table),
		)
		if _, err := tx.ExecContext(ctx, query, seq.Seq); err != nil {
			return fmt.Errorf("reset %s: %w", seq.Seq, err)
		}
	}
	return nil
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
