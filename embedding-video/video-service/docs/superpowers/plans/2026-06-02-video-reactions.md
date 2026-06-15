# Video Reactions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-user video reactions with toggle and switch behavior, persist aggregate counters on `edu_video_resource`, and expose write/read APIs for reactions and reaction counts.

**Architecture:** Keep the feature inside the existing video stack. Extend the GORM models and repository so reaction state and aggregate counters are updated transactionally, then add narrow service methods and video handler endpoints that reuse the current DTO and routing conventions.

**Tech Stack:** Go, Gin, GORM, sqlite in repository unit tests, existing `go test` suite.

---

## File Structure

- Modify: `internal/model/video.go`
  - Add aggregate reaction counters to `EduVideoResource`.
  - Add a new `EduVideoUserReaction` GORM model and table name.
- Modify: `internal/model/video_test.go`
  - Assert the new table name and keep model-level coverage current.
- Modify: `internal/http/app/app.go`
  - Include the new reaction model in `AutoMigrate`.
- Modify: `internal/infrastructure/persistence/sqlqueries/sql.go`
  - Add SQL constants for the reaction table index, unique constraint, and optional cleanup SQL used by integrity setup.
- Modify: `internal/infrastructure/persistence/gorm_video_repository.go`
  - Extend the repository with reaction-count reads and transactional reaction updates.
  - Extend `EnsureIntegrity` to create the new index and unique constraint.
- Create: `internal/infrastructure/persistence/gorm_video_repository_test.go`
  - Add sqlite-backed repository tests for reaction insert, switch, cancel, revive, and count reads.
- Modify: `internal/application/videoapp/contracts.go`
  - Extend `VideoRepository` with reaction methods and add small reaction types/results.
- Modify: `internal/application/videoapp/types.go`
  - If the shared reaction request/result structs fit existing conventions better here, define them here; otherwise keep them in `contracts.go`. Avoid scattering names.
- Modify: `internal/application/videoapp/video.go`
  - Add service methods for submit reaction and get counts with validation.
- Modify: `internal/application/videoapp/video_test.go`
  - Add service-level validation and pass-through tests.
- Modify: `internal/http/dto/video.go`
  - Add reaction request/response DTOs.
- Modify: `internal/http/handler/videos/handler.go`
  - Add `SubmitVideoReaction` and `GetReactionCounts`, plus request validation helpers.
- Modify: `internal/http/handler/video.go`
  - Expose the new video handler methods and swagger annotations for the REST routes only.
- Modify: `internal/http/handler/video_test.go`
  - Add handler tests for success, cancel, invalid payload, not found, and count shape.
- Modify: `internal/http/router/router.go`
  - Register both REST and legacy compatibility routes.
- Modify: `internal/http/router/swagger_test.go`
  - Assert the new REST routes are documented and the legacy aliases are not.

No recommendation logic, watch-record flow, worker flow, or frontend files should change.

## Task 1: Add Reaction Models and Schema Wiring

**Files:**
- Modify: `internal/model/video.go`
- Modify: `internal/model/video_test.go`
- Modify: `internal/http/app/app.go`

- [ ] **Step 1: Write the failing model test for the new table name**

In `internal/model/video_test.go`, extend `TestVideoModelTableNames` with:

```go
	if got := (EduVideoUserReaction{}).TableName(); got != "edu_video_user_reaction" {
		t.Fatalf("EduVideoUserReaction.TableName() = %q", got)
	}
```

- [ ] **Step 2: Run the focused model test and verify it fails**

Run:

```bash
go test ./internal/model -run TestVideoModelTableNames
```

Expected: FAIL at compile time because `EduVideoUserReaction` does not exist yet.

- [ ] **Step 3: Add the reaction columns and new model**

In `internal/model/video.go`, update `EduVideoResource` to include:

```go
	LikeCount       int `gorm:"column:like_count;default:0" json:"like_count"`
	DoubleLikeCount int `gorm:"column:double_like_count;default:0" json:"double_like_count"`
	DislikeCount    int `gorm:"column:dislike_count;default:0" json:"dislike_count"`
```

Add this new model below `EduVideoResource`:

```go
type EduVideoUserReaction struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	UserID       uint64 `gorm:"column:user_id;not null;index" json:"user_id"`
	VideoID      uint64 `gorm:"column:video_id;not null;index" json:"video_id"`
	ReactionType string `gorm:"column:reaction_type;type:text;not null" json:"reaction_type"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduVideoUserReaction) TableName() string { return "edu_video_user_reaction" }
```

In `internal/http/app/app.go`, update `AutoMigrate(...)` to include `&model.EduVideoUserReaction{}` next to the existing video models.

- [ ] **Step 4: Run the model test and verify it passes**

Run:

```bash
go test ./internal/model -run TestVideoModelTableNames
```

Expected: PASS.

- [ ] **Step 5: Run the app package test that covers startup wiring**

Run:

```bash
go test ./internal/http/app
```

Expected: PASS. If it fails because a startup test hard-codes the migrate list, update that expectation in the same task and rerun until green.

## Task 2: Define Repository Contracts and Service-Level API

**Files:**
- Modify: `internal/application/videoapp/contracts.go`
- Modify: `internal/application/videoapp/video.go`
- Modify: `internal/application/videoapp/video_test.go`

- [ ] **Step 1: Write the failing service tests for reaction validation and pass-through**

In `internal/application/videoapp/video_test.go`, add these tests:

```go
func TestSubmitVideoReactionValidatesInputs(t *testing.T) {
	svc := NewService(&videoTestRepo{}, nil, nil, nil, nil, nil, nil, Paths{})

	_, _, err := svc.SubmitVideoReaction(context.Background(), 0, 7, VideoReactionLike)
	if err == nil {
		t.Fatal("expected validation error for video id")
	}
	assertValidationMessage(t, err, "video_id is required")

	_, _, err = svc.SubmitVideoReaction(context.Background(), 9, 0, VideoReactionLike)
	if err == nil {
		t.Fatal("expected validation error for user id")
	}
	assertValidationMessage(t, err, "user_id is required")

	_, _, err = svc.SubmitVideoReaction(context.Background(), 9, 7, VideoReactionType("bad"))
	if err == nil {
		t.Fatal("expected validation error for reaction type")
	}
	assertValidationMessage(t, err, "reaction_type must be one of like, double_like, dislike")
}

func TestSubmitVideoReactionPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{
		submitReactionActive: true,
		submitReactionOK:     true,
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	active, ok, err := svc.SubmitVideoReaction(context.Background(), 15, 23, VideoReactionDoubleLike)
	if err != nil {
		t.Fatalf("SubmitVideoReaction returned error: %v", err)
	}
	if !ok || !active {
		t.Fatalf("unexpected result: active=%v ok=%v", active, ok)
	}
	if repo.lastReactionVideoID != 15 || repo.lastReactionUserID != 23 || repo.lastReactionType != VideoReactionDoubleLike {
		t.Fatalf("unexpected repo call: %+v", repo)
	}
}

func TestGetVideoReactionCountsPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{
		reactionCounts: VideoReactionCounts{LikeCount: 4, DoubleLikeCount: 2},
		reactionCountsOK: true,
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	counts, ok, err := svc.GetVideoReactionCounts(context.Background(), 18)
	if err != nil {
		t.Fatalf("GetVideoReactionCounts returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if counts.LikeCount != 4 || counts.DoubleLikeCount != 2 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
}
```

- [ ] **Step 2: Run the focused service tests and verify they fail**

Run:

```bash
go test ./internal/application/videoapp -run 'TestSubmitVideoReaction|TestGetVideoReactionCounts'
```

Expected: FAIL at compile time because the reaction types, methods, and repo hooks do not exist yet.

- [ ] **Step 3: Add shared reaction types and repository methods**

In `internal/application/videoapp/contracts.go`, add:

```go
type VideoReactionType string

const (
	VideoReactionLike       VideoReactionType = "like"
	VideoReactionDoubleLike VideoReactionType = "double_like"
	VideoReactionDislike    VideoReactionType = "dislike"
)

type VideoReactionCounts struct {
	LikeCount       int64
	DoubleLikeCount int64
}
```

Add this helper:

```go
func (t VideoReactionType) IsValid() bool {
	switch t {
	case VideoReactionLike, VideoReactionDoubleLike, VideoReactionDislike:
		return true
	default:
		return false
	}
}
```

Extend `VideoRepository` with:

```go
	SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType VideoReactionType) (active bool, found bool, err error)
	GetVideoReactionCounts(ctx context.Context, videoID uint64) (VideoReactionCounts, bool, error)
```

- [ ] **Step 4: Add the minimal service methods**

In `internal/application/videoapp/video.go`, add:

```go
func (s *Service) SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType VideoReactionType) (bool, bool, error) {
	if videoID == 0 {
		return false, false, InvalidArgumentError("video_id is required")
	}
	if userID == 0 {
		return false, false, InvalidArgumentError("user_id is required")
	}
	if !reactionType.IsValid() {
		return false, false, InvalidArgumentError("reaction_type must be one of like, double_like, dislike")
	}
	return s.Repo.SubmitVideoReaction(ctx, videoID, userID, reactionType)
}

func (s *Service) GetVideoReactionCounts(ctx context.Context, videoID uint64) (VideoReactionCounts, bool, error) {
	if videoID == 0 {
		return VideoReactionCounts{}, false, InvalidArgumentError("video_id is required")
	}
	return s.Repo.GetVideoReactionCounts(ctx, videoID)
}
```

- [ ] **Step 5: Update the service test repo stub**

In `internal/application/videoapp/video_test.go`, extend `videoTestRepo` with:

```go
	submitReactionActive bool
	submitReactionOK     bool
	submitReactionErr    error
	lastReactionVideoID  uint64
	lastReactionUserID   uint64
	lastReactionType     VideoReactionType
	reactionCounts       VideoReactionCounts
	reactionCountsOK     bool
	reactionCountsErr    error
	lastReactionCountID  uint64
```

Add methods:

```go
func (r *videoTestRepo) SubmitVideoReaction(_ context.Context, videoID uint64, userID uint64, reactionType VideoReactionType) (bool, bool, error) {
	r.lastReactionVideoID = videoID
	r.lastReactionUserID = userID
	r.lastReactionType = reactionType
	return r.submitReactionActive, r.submitReactionOK, r.submitReactionErr
}

func (r *videoTestRepo) GetVideoReactionCounts(_ context.Context, videoID uint64) (VideoReactionCounts, bool, error) {
	r.lastReactionCountID = videoID
	return r.reactionCounts, r.reactionCountsOK, r.reactionCountsErr
}
```

- [ ] **Step 6: Run the focused service tests and verify they pass**

Run:

```bash
go test ./internal/application/videoapp -run 'TestSubmitVideoReaction|TestGetVideoReactionCounts'
```

Expected: PASS.

## Task 3: Implement Transactional Persistence for Reactions

**Files:**
- Modify: `internal/infrastructure/persistence/sqlqueries/sql.go`
- Modify: `internal/infrastructure/persistence/gorm_video_repository.go`
- Create: `internal/infrastructure/persistence/gorm_video_repository_test.go`

- [ ] **Step 1: Write the failing repository tests for set, switch, cancel, revive, and count read**

Create `internal/infrastructure/persistence/gorm_video_repository_test.go` with this test harness:

```go
package persistence

import (
	"context"
	"testing"

	"nlp-video-project/http/internal/application/videoapp"
	"nlp-video-project/http/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newVideoRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoUserReaction{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedVideoResource(t *testing.T, db *gorm.DB, id uint64) {
	t.Helper()
	if err := db.Create(&model.EduVideoResource{ID: id, Title: "video", VideoURL: "/videos/raw/2026/06/02/demo.mp4", Status: 1}).Error; err != nil {
		t.Fatalf("seed video: %v", err)
	}
}
```

Add these tests:

```go
func TestSubmitVideoReactionCreatesFirstLike(t *testing.T) { ... }
func TestSubmitVideoReactionSwitchesDoubleLikeToLike(t *testing.T) { ... }
func TestSubmitVideoReactionRepeatingSameReactionCancelsIt(t *testing.T) { ... }
func TestSubmitVideoReactionRevivesSoftDeletedReactionRow(t *testing.T) { ... }
func TestGetVideoReactionCountsReadsOnlyResourceCounters(t *testing.T) { ... }
func TestSubmitVideoReactionReturnsNotFoundWhenVideoMissing(t *testing.T) { ... }
```

Use these exact assertions inside the tests:

```go
if !active || !found {
	t.Fatalf("unexpected result: active=%v found=%v", active, found)
}
```

```go
if active || !found {
	t.Fatalf("unexpected cancel result: active=%v found=%v", active, found)
}
```

```go
if resource.LikeCount != 1 || resource.DoubleLikeCount != 0 || resource.DislikeCount != 0 {
	t.Fatalf("unexpected counters: %+v", resource)
}
```

```go
if resource.LikeCount != 1 || resource.DoubleLikeCount != 0 {
	t.Fatalf("unexpected counts: %+v", resource)
}
```

- [ ] **Step 2: Run the focused repository test and verify it fails**

Run:

```bash
go test ./internal/infrastructure/persistence -run TestSubmitVideoReactionCreatesFirstLike
```

Expected: FAIL at compile time because the repository methods and new model usage are not implemented yet.

- [ ] **Step 3: Add SQL constants for integrity setup**

In `internal/infrastructure/persistence/sqlqueries/sql.go`, add:

```go
const CreateVideoReactionUserIndexQuery = `CREATE INDEX IF NOT EXISTS idx_video_user_reaction_user ON edu_video_user_reaction(user_id);`

const CreateVideoReactionVideoIndexQuery = `CREATE INDEX IF NOT EXISTS idx_video_user_reaction_video ON edu_video_user_reaction(video_id);`

const CreateVideoReactionUniqueConstraintQuery = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_video_user_reaction_user_video') THEN
    ALTER TABLE edu_video_user_reaction
      ADD CONSTRAINT uk_video_user_reaction_user_video UNIQUE (user_id, video_id);
  END IF;
END$$;
`
```

- [ ] **Step 4: Implement the repository methods and integrity setup**

In `internal/infrastructure/persistence/gorm_video_repository.go`, add this helper near the repository methods:

```go
func reactionCounterColumn(reactionType videoapp.VideoReactionType) string {
	switch reactionType {
	case videoapp.VideoReactionLike:
		return "like_count"
	case videoapp.VideoReactionDoubleLike:
		return "double_like_count"
	case videoapp.VideoReactionDislike:
		return "dislike_count"
	default:
		return ""
	}
}
```

Add:

```go
func (r *GormVideoRepository) SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (bool, bool, error) {
	var active bool
	var found bool

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var video model.EduVideoResource
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted = ?", videoID, 0).
			First(&video).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				found = false
				return nil
			}
			return err
		}
		found = true

		var reaction model.EduVideoUserReaction
		err := tx.Where("user_id = ? AND video_id = ?", userID, videoID).First(&reaction).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		newColumn := reactionCounterColumn(reactionType)
		if err == gorm.ErrRecordNotFound {
			if err := tx.Create(&model.EduVideoUserReaction{
				UserID:       userID,
				VideoID:      videoID,
				ReactionType: string(reactionType),
				Deleted:      0,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.EduVideoResource{}).
				Where("id = ?", videoID).
				UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error; err != nil {
				return err
			}
			active = true
			return nil
		}

		oldType := videoapp.VideoReactionType(reaction.ReactionType)
		oldColumn := reactionCounterColumn(oldType)
		isActive := reaction.Deleted == 0

		if isActive && oldType == reactionType {
			if err := tx.Model(&model.EduVideoUserReaction{}).
				Where("id = ?", reaction.ID).
				Updates(map[string]any{"deleted": 1}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.EduVideoResource{}).
				Where("id = ?", videoID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
				return err
			}
			active = false
			return nil
		}

		updates := map[string]any{
			"reaction_type": string(reactionType),
			"deleted":       0,
		}
		if err := tx.Model(&model.EduVideoUserReaction{}).
			Where("id = ?", reaction.ID).
			Updates(updates).Error; err != nil {
			return err
		}
		if isActive {
			if err := tx.Model(&model.EduVideoResource{}).
				Where("id = ?", videoID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.EduVideoResource{}).
			Where("id = ?", videoID).
			UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error; err != nil {
			return err
		}
		active = true
		return nil
	})
	if err != nil {
		return false, false, err
	}
	return active, found, nil
}

func (r *GormVideoRepository) GetVideoReactionCounts(ctx context.Context, videoID uint64) (videoapp.VideoReactionCounts, bool, error) {
	var row model.EduVideoResource
	err := r.db.WithContext(ctx).
		Model(&model.EduVideoResource{}).
		Select("id", "like_count", "double_like_count").
		Where("id = ? AND deleted = ?", videoID, 0).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return videoapp.VideoReactionCounts{}, false, nil
		}
		return videoapp.VideoReactionCounts{}, false, err
	}
	return videoapp.VideoReactionCounts{
		LikeCount:       int64(row.LikeCount),
		DoubleLikeCount: int64(row.DoubleLikeCount),
	}, true, nil
}
```

Add imports:

```go
	"gorm.io/gorm/clause"
```

Extend `EnsureIntegrity` with:

```go
	_ = db.Exec(sqlqueries.CreateVideoReactionUserIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateVideoReactionVideoIndexQuery).Error
	if err := db.Exec(sqlqueries.CreateVideoReactionUniqueConstraintQuery).Error; err != nil {
		return err
	}
```

- [ ] **Step 5: Run the focused repository tests and verify they pass**

Run:

```bash
go test ./internal/infrastructure/persistence -run 'TestSubmitVideoReaction|TestGetVideoReactionCounts'
```

Expected: PASS.

- [ ] **Step 6: Run the full persistence package tests**

Run:

```bash
go test ./internal/infrastructure/persistence
```

Expected: PASS.

## Task 4: Add HTTP DTOs and Handlers for Reactions

**Files:**
- Modify: `internal/http/dto/video.go`
- Modify: `internal/http/handler/videos/handler.go`
- Modify: `internal/http/handler/video.go`
- Modify: `internal/http/handler/video_test.go`

- [ ] **Step 1: Write the failing handler tests**

In `internal/http/handler/video_test.go`, extend `stubVideoApp` with:

```go
	submitVideoReactionFunc func(context.Context, uint64, uint64, videoapp.VideoReactionType) (bool, bool, error)
	getReactionCountsFunc   func(context.Context, uint64) (videoapp.VideoReactionCounts, bool, error)
	submitReactionVideoID   uint64
	submitReactionUserID    uint64
	submitReactionType      videoapp.VideoReactionType
	getReactionCountsID     uint64
```

Add methods:

```go
func (s *stubVideoApp) SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (bool, bool, error) {
	s.submitReactionVideoID = videoID
	s.submitReactionUserID = userID
	s.submitReactionType = reactionType
	if s.submitVideoReactionFunc != nil {
		return s.submitVideoReactionFunc(ctx, videoID, userID, reactionType)
	}
	return false, false, nil
}

func (s *stubVideoApp) GetVideoReactionCounts(ctx context.Context, videoID uint64) (videoapp.VideoReactionCounts, bool, error) {
	s.getReactionCountsID = videoID
	if s.getReactionCountsFunc != nil {
		return s.getReactionCountsFunc(ctx, videoID)
	}
	return videoapp.VideoReactionCounts{}, false, nil
}
```

Add these tests:

```go
func TestSubmitVideoReaction_SetsReaction(t *testing.T) { ... }
func TestSubmitVideoReaction_RepeatingSameReactionCancelsIt(t *testing.T) { ... }
func TestSubmitVideoReaction_RejectsInvalidReactionType(t *testing.T) { ... }
func TestSubmitVideoReaction_RejectsMissingUserID(t *testing.T) { ... }
func TestSubmitVideoReaction_ReturnsNotFound(t *testing.T) { ... }
func TestGetVideoReactionCounts_Success(t *testing.T) { ... }
```

Use these exact assertions:

```go
assertBodyContains(t, w.Body.Bytes(), `"reaction_type":"double_like"`)
assertBodyContains(t, w.Body.Bytes(), `"active":true`)
```

```go
assertBodyContains(t, w.Body.Bytes(), `"active":false`)
```

```go
assertBodyContains(t, w.Body.Bytes(), `"message":"reaction_type must be one of like, double_like, dislike"`)
```

```go
assertBodyContains(t, w.Body.Bytes(), `"like_count":5`)
assertBodyContains(t, w.Body.Bytes(), `"double_like_count":2`)
```

Add one negative assertion:

```go
if bytes.Contains(w.Body.Bytes(), []byte(`"dislike_count"`)) {
	t.Fatalf("response must not include dislike_count: %s", w.Body.String())
}
```

- [ ] **Step 2: Run the focused handler tests and verify they fail**

Run:

```bash
go test ./internal/http/handler -run 'TestSubmitVideoReaction|TestGetVideoReactionCounts'
```

Expected: FAIL at compile time because the handler interface, DTOs, and methods do not exist yet.

- [ ] **Step 3: Add DTOs**

In `internal/http/dto/video.go`, add:

```go
type VideoReactionRequest struct {
	UserID       uint64 `json:"user_id"`
	ReactionType string `json:"reaction_type"`
}

type VideoReactionData struct {
	VideoID      uint64 `json:"video_id"`
	UserID       uint64 `json:"user_id"`
	ReactionType string `json:"reaction_type"`
	Active       bool   `json:"active"`
	Updated      bool   `json:"updated"`
}

type VideoReactionCountsData struct {
	VideoID         uint64 `json:"video_id"`
	LikeCount       int64  `json:"like_count"`
	DoubleLikeCount int64  `json:"double_like_count"`
}

type VideoReactionResponse struct {
	Success bool              `json:"success"`
	Data    VideoReactionData `json:"data"`
}

type VideoReactionCountsResponse struct {
	Success bool                   `json:"success"`
	Data    VideoReactionCountsData `json:"data"`
}
```

- [ ] **Step 4: Add handler methods and validation helper**

In `internal/http/handler/videos/handler.go`, extend `videoApp` with:

```go
	SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (bool, bool, error)
	GetVideoReactionCounts(ctx context.Context, videoID uint64) (videoapp.VideoReactionCounts, bool, error)
```

Add:

```go
func validateVideoReactionRequest(req dto.VideoReactionRequest) (videoapp.VideoReactionType, error) {
	if req.UserID == 0 {
		return "", videoapp.InvalidArgumentError("user_id is required")
	}
	reactionType := videoapp.VideoReactionType(strings.TrimSpace(req.ReactionType))
	if !reactionType.IsValid() {
		return "", videoapp.InvalidArgumentError("reaction_type must be one of like, double_like, dislike")
	}
	return reactionType, nil
}
```

Add the two handlers:

```go
func (h *Handler) SubmitVideoReaction(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	var req dto.VideoReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	reactionType, err := validateVideoReactionRequest(req)
	if err != nil {
		writeAppError(c, err, "submit video reaction failed")
		return
	}

	active, found, err := h.app.SubmitVideoReaction(c.Request.Context(), videoID, req.UserID, reactionType)
	if err != nil {
		writeAppError(c, err, "submit video reaction failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.VideoReactionData{
		VideoID:      videoID,
		UserID:       req.UserID,
		ReactionType: string(reactionType),
		Active:       active,
		Updated:      true,
	})
}

func (h *Handler) GetVideoReactionCounts(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	counts, found, err := h.app.GetVideoReactionCounts(c.Request.Context(), videoID)
	if err != nil {
		writeAppError(c, err, "get reaction counts failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.VideoReactionCountsData{
		VideoID:         videoID,
		LikeCount:       counts.LikeCount,
		DoubleLikeCount: counts.DoubleLikeCount,
	})
}
```

- [ ] **Step 5: Expose wrapper methods and swagger docs**

In `internal/http/handler/video.go`, add two wrapper methods with swagger comments for:

```go
// @Router /api/videos/{id}/reactions [post]
func (h *VideoHandler) SubmitVideoReaction(c *gin.Context) {
	h.inner.SubmitVideoReaction(c)
}

// @Router /api/videos/{id}/reaction-counts [get]
func (h *VideoHandler) GetVideoReactionCounts(c *gin.Context) {
	h.inner.GetVideoReactionCounts(c)
}
```

Use the existing `videos` tag and the new DTO response types.

- [ ] **Step 6: Run the focused handler tests and verify they pass**

Run:

```bash
go test ./internal/http/handler -run 'TestSubmitVideoReaction|TestGetVideoReactionCounts'
```

Expected: PASS.

## Task 5: Register Routes and Swagger Expectations

**Files:**
- Modify: `internal/http/router/router.go`
- Modify: `internal/http/router/swagger_test.go`

- [ ] **Step 1: Write the failing router test for the new legacy aliases**

In `internal/http/router/swagger_test.go`, extend `legacyPaths` with:

```go
		"/api/video/reaction/{id}",
		"/api/video/reaction_counts/{id}",
```

Extend `newPaths` with:

```go
		"/api/videos/{id}/reactions",
		"/api/videos/{id}/reaction-counts",
```

In `TestLegacyAliasRoutesAreRegistered`, add:

```go
		{name: "reaction alias", method: http.MethodPost, path: "/api/video/reaction/0", want: http.StatusBadRequest},
		{name: "reaction counts alias", method: http.MethodGet, path: "/api/video/reaction_counts/0", want: http.StatusBadRequest},
```

- [ ] **Step 2: Run the focused router tests and verify they fail**

Run:

```bash
go test ./internal/http/router -run 'TestSwaggerDocOmitsLegacyAliasPaths|TestLegacyAliasRoutesAreRegistered'
```

Expected: FAIL because the new routes are not registered yet.

- [ ] **Step 3: Register the REST and legacy routes**

In `internal/http/router/router.go`, add:

```go
	r.POST("/api/videos/:id/reactions", videoHandler.SubmitVideoReaction)
	r.POST("/api/video/reaction/:id", videoHandler.SubmitVideoReaction)
	r.GET("/api/videos/:id/reaction-counts", videoHandler.GetVideoReactionCounts)
	r.GET("/api/video/reaction_counts/:id", videoHandler.GetVideoReactionCounts)
```

Place them with the other video routes.

- [ ] **Step 4: Run the focused router tests and verify they pass**

Run:

```bash
go test ./internal/http/router -run 'TestSwaggerDocOmitsLegacyAliasPaths|TestLegacyAliasRoutesAreRegistered'
```

Expected: PASS.

## Task 6: Run the Broader Verification Set

**Files:**
- No code changes expected unless verification exposes an issue directly caused by earlier tasks.

- [ ] **Step 1: Run the full videoapp package tests**

Run:

```bash
go test ./internal/application/videoapp
```

Expected: PASS.

- [ ] **Step 2: Run the full handler package tests**

Run:

```bash
go test ./internal/http/handler
```

Expected: PASS.

- [ ] **Step 3: Run the full router package tests**

Run:

```bash
go test ./internal/http/router
```

Expected: PASS.

- [ ] **Step 4: Run the full model package tests**

Run:

```bash
go test ./internal/model
```

Expected: PASS.

- [ ] **Step 5: Run the full persistence package tests**

Run:

```bash
go test ./internal/infrastructure/persistence
```

Expected: PASS.

- [ ] **Step 6: Run the service-wide regression suite for the active module**

Run:

```bash
go test ./...
```

Expected: PASS. If unrelated environment-dependent tests fail, capture the exact failing package and stop before claiming completion.
