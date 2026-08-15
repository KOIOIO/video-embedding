# MinIO Knowledge Video Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dry-run-capable CLI that imports existing MinIO objects as knowledge videos and permits one source object to serve multiple knowledge points.

**Architecture:** A new application service parses and validates the XLSX, resolves source objects below an allowed prefix, then uses existing batch repository and Redis queue contracts. The command wires PostgreSQL, Redis, and object storage from existing config; HTTP behavior remains unchanged.

**Tech Stack:** Go, excelize, GORM/PostgreSQL, MinIO Go SDK, Redis Streams.

---

### Task 1: Direct Mapping Validation

**Files:**
- Create: `internal/application/knowledgevideo/direct_import.go`
- Create: `internal/application/knowledgevideo/direct_import_test.go`

- [ ] Write tests for exact headers, positive IDs, required names/object references, dictionary leaf-name normalization, missing/ambiguous objects, metadata filtering, extensions, and repeated source objects.
- [ ] Run `go test ./internal/application/knowledgevideo -run TestDirectImport` and confirm the tests fail because the service is absent.
- [ ] Implement parsing and complete issue aggregation without any writes.
- [ ] Run the focused tests and confirm they pass.

### Task 2: Batch Creation And Queueing

**Files:**
- Modify: `internal/application/knowledgevideo/direct_import.go`
- Modify: `internal/application/knowledgevideo/direct_import_test.go`
- Modify: `internal/application/knowledgevideo/contracts.go`

- [ ] Add failing tests proving dry-run performs no writes, repeated object keys create distinct video IDs/HLS prefixes, one task is queued per row, and an existing batch key is idempotent.
- [ ] Extend the repository contract with import-key lookup and implement the minimal write flow using existing ID reservation, batch creation, manifest storage, queue, and enqueue-time contracts.
- [ ] Run `go test ./internal/application/knowledgevideo -run TestDirectImport` and confirm all focused tests pass.

### Task 3: Infrastructure Support

**Files:**
- Modify: `internal/infrastructure/objectstorage/rustfs.go`
- Modify: `internal/infrastructure/objectstorage/rustfs_test.go`
- Modify: `internal/infrastructure/persistence/gorm_knowledge_video_repository.go`
- Modify: `internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`

- [ ] Add failing tests for recursive prefix listing and lookup of an existing direct-import batch key.
- [ ] Implement object listing through the MinIO SDK and batch-key lookup through the existing batch source-name field.
- [ ] Run `go test ./internal/infrastructure/objectstorage ./internal/infrastructure/persistence` and confirm success.

### Task 4: CLI Wiring

**Files:**
- Create: `cmd/knowledgevideo-import/main.go`
- Create: `cmd/knowledgevideo-import/main_test.go`
- Modify: `README.md`

- [ ] Add failing tests for required flags, source-prefix normalization, dry-run output, and credential-free error output.
- [ ] Implement config loading, PostgreSQL/Redis/MinIO construction, signal handling, service invocation, and structured summary output.
- [ ] Document exact dry-run and formal-import commands plus mapping rules.
- [ ] Run `go test ./cmd/knowledgevideo-import` and confirm success.

### Task 5: Verification

**Files:**
- Review all files above.

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./internal/application/knowledgevideo ./internal/infrastructure/objectstorage ./internal/infrastructure/persistence ./cmd/knowledgevideo-import`.
- [ ] Run `go test ./...`.
- [ ] Run `git diff --check` and inspect `git diff --stat` for unexpected changes.
