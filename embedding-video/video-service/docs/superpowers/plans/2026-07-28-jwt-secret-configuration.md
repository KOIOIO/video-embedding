# JWT Secret Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make local HTTP API startup work through committed development configuration or ignored environment files, without manually exporting `JWT_SECRET`.

**Architecture:** Keep the existing configuration precedence and validation. Add a safe development fallback to local YAML, put distinct generated values in ignored local/deployment env files, and keep production YAML secret-free.

**Tech Stack:** Go, YAML, dotenv-style environment files

---

### Task 1: Lock Down Local Fallback

**Files:**
- Modify: `internal/config/loader_test.go`
- Modify: `configs/video.yml`

- [ ] Add a test that loads `configs/video.yml` with `JWT_SECRET` unset and requires `config.JWTSecret` to contain at least 32 characters.
- [ ] Run `go test ./internal/config -run TestLocalConfigProvidesValidJWTSecret -count=1` and confirm it fails with the empty YAML value.
- [ ] Set `Auth.JWTSecret` in `configs/video.yml` to a clearly development-only value of at least 32 characters.
- [ ] Rerun the focused test and confirm it passes.

### Task 2: Configure Ignored Environment Files

**Files:**
- Modify: `../.env.local` (ignored)
- Modify: `../.env.deploy` (ignored)

- [ ] Generate two distinct 32-byte random secrets.
- [ ] Add one as `JWT_SECRET` to `.env.local` and the other to `.env.deploy` without printing either value.
- [ ] Verify both keys exist, both values are at least 32 characters, and the values differ.

### Task 3: Verify Startup and Tests

**Files:**
- Verify: `internal/config/loader_test.go`
- Verify: `cmd/httpapi/main.go`

- [ ] Run `go test ./internal/config ./cmd/httpapi -count=1`.
- [ ] Start `go run ./cmd/httpapi` from the service root with no shell `JWT_SECRET`; accept successful configuration initialization even if external infrastructure later prevents serving.
- [ ] Run `git diff --check` and confirm ignored env files are not staged.
- [ ] Commit the tracked test, YAML, design, and plan files.
