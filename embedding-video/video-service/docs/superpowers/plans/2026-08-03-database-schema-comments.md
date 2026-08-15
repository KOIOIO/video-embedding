# Database Schema Comments Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add durable Chinese comments for every table and column owned by the HTTP video service and apply them to the requested PostgreSQL database without persisting connection credentials.

**Architecture:** Keep one ordered comment registry in the persistence package for the 11 `public` GORM tables and 3 hand-written `recsys` tables. Generate PostgreSQL `COMMENT ON` statements from that registry and execute them inside the existing advisory-locked startup migration after all tables and columns exist.

**Tech Stack:** Go 1.26, GORM, PostgreSQL 13+, pgvector, Go testing.

---

### Task 1: Lock schema-comment coverage with tests

**Files:**
- Create: `internal/infrastructure/persistence/schema_comments_test.go`
- Modify: `internal/infrastructure/persistence/migration.go`

- [x] Add a test that parses every model passed to `AutoMigrate` and requires an exact, non-empty table and column comment definition.
- [x] Add an explicit expected-column test for the three `recsys` tables defined by raw SQL.
- [x] Run `go test ./internal/infrastructure/persistence -run SchemaComment -count=1` and confirm it fails because the comment registry does not exist.

### Task 2: Add the idempotent PostgreSQL comment migration

**Files:**
- Create: `internal/infrastructure/persistence/schema_comments.go`
- Modify: `internal/infrastructure/persistence/migration.go`

- [x] Define Chinese comments for all 14 service-owned tables and all 171 columns.
- [x] Generate quoted `COMMENT ON TABLE` and `COMMENT ON COLUMN` statements and execute them sequentially.
- [x] Call `EnsureSchemaComments` after `AutoMigrate` and before integrity cleanup while the existing advisory lock is held.
- [x] Run the focused persistence tests and confirm they pass.

### Task 3: Update the requested database without persisting credentials

- [x] Keep `configs/video.yml`, `configs/video_prod.yml`, and all other tracked files free of connection credentials.
- [x] Pass the requested connection only through the temporary process environment and execute the generated comment statements; do not run `EnsureIntegrity` because it performs data cleanup.
- [x] Query `pg_catalog` and require zero missing table comments and zero missing column comments for every service-owned table/column that currently exists.

### Task 4: Verify the repository change

**Files:**
- Verify: `internal/infrastructure/persistence/schema_comments.go`
- Verify: `internal/infrastructure/persistence/schema_comments_test.go`
- Verify: `internal/infrastructure/persistence/migration.go`
- Verify unchanged: `configs/video.yml`
- Verify unchanged: `configs/video_prod.yml`

- [x] Run `gofmt` on changed Go files.
- [x] Run `go test ./internal/infrastructure/persistence -count=1`.
- [x] Run `go test ./...`.
- [x] Scan the repository for the supplied password and require zero matches.
- [x] Review `git diff` to ensure only the requested schema comments, tests, migration integration, and this plan changed.
