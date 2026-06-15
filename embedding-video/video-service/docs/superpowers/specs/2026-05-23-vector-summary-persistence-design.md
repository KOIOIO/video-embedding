# Vector Summary Persistence Design

## Summary

The hierarchical `vector_worker` path already contains logic that can rewrite `content_summary` from the stable segment transcript. However, the final database update only persists `start_time`, `end_time`, `embedding`, and `status`. This means the embedding input may use a corrected summary while the stored segment title still comes from the earlier LLM segmentation draft.

This design closes that gap with the smallest safe change: make the refine stage return the final summary explicitly, then persist it in the same transaction that stores the final boundaries, embedding, and status.

The design must not increase ASR cost. It must not restore full per-segment refine ASR, add new boundary probes, or introduce any new ASR invocation path.

## Problem

The fresh hierarchical pipeline currently works like this:

1. Coarse ASR creates `coarseItems`.
2. LLM segmentation creates draft segments with `content_summary`.
3. Segment normalization and repair adjust segment boundaries and merge obvious continuations.
4. Draft segments are inserted into `edu_video_segment`.
5. Selective refine builds the embedding input from a summary plus transcript.
6. Embedding succeeds.
7. The final database update persists time boundaries, embedding, and status.

The issue is step 7. The refine stage may generate a better summary in step 5, but that summary is not written back to `edu_video_segment.content_summary`.

As a result:

- `embedding` can be based on corrected text.
- `content_summary` can still be the older LLM draft.
- Recommendation responses can return a `segment_title` that does not match the actual clip.

## Goals

1. Persist the final refined summary to `edu_video_segment.content_summary`.
2. Keep `content_summary`, segment boundaries, embedding, and status consistent by updating them in one transaction.
3. Preserve the current selective refine cost model.
4. Avoid implicit parsing of the summary from the first line of the embedding input.
5. Add tests that prevent this persistence gap from returning.

## Non-Goals

1. Do not add database columns or tables.
2. Do not redesign the hierarchical segmentation prompt.
3. Do not introduce a new summary scoring system.
4. Do not change ASR provider behavior.
5. Do not increase ASR calls for healthy segments.
6. Do not restore multi-probe boundary alignment in the fresh hierarchical path.
7. Do not implement historical segment backfill in this change.

## Existing Context

Relevant files:

- `internal/worker/vectorworker/tasks/asr.go`
  - Builds refine input.
  - Runs selective refine ASR.
  - Runs embedding.
  - Updates final segment state.
- `internal/worker/vectorworker/tasks/summary_alignment.go`
  - Detects summary/content mismatch.
  - Rewrites summary from stable text.
- `internal/worker/vectorworker/tasks/asr_refine_input_test.go`
  - Covers selective refine input behavior.
- `internal/worker/vectorworker/task.go`
  - Passes `RefineASRHints` for the fresh hierarchical path.

The recent summary alignment work established the intended rule:

> The final segment transcript is the source of truth, and `content_summary` must be allowed to follow it.

This design makes that rule durable by persisting the corrected summary.

## Proposed Design

### Refine Result Contract

Replace the implicit return contract of `buildRefineSegmentInputWithSummaryRewrite` with a structured result.

Suggested internal type:

```go
type refineSegmentInputResult struct {
	Input       string
	Summary     string
	StartSec    int
	EndSec      int
	SummaryMode string
}
```

Field meaning:

- `Input`: final text passed to embedding, still built as `summary + "\n" + transcript` when both are present.
- `Summary`: final display title to persist.
- `StartSec`: final segment start.
- `EndSec`: final segment end.
- `SummaryMode`: one of the existing summary alignment modes, such as `original`, `rule_rewrite`, `llm_rewrite`, or fallback variants.

The important change is that callers no longer infer the final summary by splitting `Input` on the first newline.

### Selective Refine Behavior

The current selective refine rules remain unchanged:

1. Healthy high/medium-confidence segments reuse coarse transcript.
2. Low-confidence segments may trigger one expanded-window ASR pass.
3. Segments with summary/content mismatch may trigger one expanded-window ASR pass.
4. If that ASR pass fails or returns empty text, the code falls back to coarse transcript.
5. Summary rewrite uses the stable text already chosen by the existing flow.

This design does not add any new ASR decision point.

### Persistence Behavior

`RefineSegmentsASRAndEmbed` should track final summaries alongside final inputs and final boundaries:

- `orderedInputs`
- `orderedSummaries`
- `orderedStartSecs`
- `orderedEndSecs`
- `orderedIDs`

When embedding succeeds, build update records that include `Summary`.

Suggested internal update shape:

```go
type embeddingUpdate struct {
	ID        uint64
	StartSec  int
	EndSec    int
	Summary   string
	Embedding pgvector.Vector
}
```

The final transaction should update:

- `start_time`
- `end_time`
- `content_summary`
- `embedding`
- `status`

`content_summary` should be normalized with the existing title normalization helper before persistence.

### Transaction Semantics

The summary must be updated only when embedding has succeeded.

This preserves the current all-or-nothing behavior:

- If embedding fails, do not update title, boundaries, embedding, or status.
- If the transaction fails, no partial state should be committed.
- If summary rewrite changed the title but embedding fails later, the stored draft title remains unchanged.

This avoids a state where the database shows a new title for a segment whose embedding/status still did not complete.

## ASR Cost Control

Cost control is a hard constraint for this design.

The implementation must preserve these rules:

1. Do not run refine ASR for every fresh hierarchical segment.
2. Do not add a new ASR call only for summary persistence.
3. Do not add boundary candidate probing.
4. Do not retry ASR more than the existing single expanded-window pass for low-confidence or mismatch segments.
5. Do not change healthy segment behavior from coarse transcript reuse to refine ASR.

The summary persistence fix reuses data already produced by the current selective refine flow. It should not change ASR call counts.

## Resume Path

When `RefineASRHints` is unavailable, the resume path should keep the current fallback behavior.

This means:

- Fresh hierarchical tasks get the full summary persistence fix.
- Resume/retry tasks without coarse items and LLM hints do not need a new dependency.
- A later design can decide whether to persist extra hints, but this change should not require it.

## Error Handling

1. If expanded-window ASR fails, keep the existing fallback to coarse transcript.
2. If LLM summary rewrite fails, keep the existing fallback mode.
3. If LLM summary rewrite returns empty text, keep the existing fallback mode.
4. If embedding fails, return the error and do not update the segment.
5. If the database transaction fails, return the error and do not commit partial segment updates.

## Testing Plan

Add focused tests around the changed contract and persistence payload.

### Unit Tests

1. `buildRefineSegmentInputWithSummaryRewrite` returns explicit `Summary`.
   - Given an overreaching summary and stable text, result summary should be rewritten.
   - Result input should still contain the rewritten summary plus transcript.

2. Healthy high-confidence segment does not trigger extra ASR.
   - Keep the existing no-call assertion.
   - Update expectations to use the structured result.

3. Low-confidence segment still triggers at most one expanded-window ASR call.
   - Keep the existing single-call assertion.
   - Update expectations to use the structured result.

4. Final update payload includes `content_summary`.
   - Prefer extracting a small helper for building update maps if direct database integration would make the test heavy.
   - Verify `content_summary` is present with the final summary.

### Package Verification

Run focused package tests first:

```bash
go test ./internal/worker/vectorworker/tasks
```

If the implementation touches call sites outside `tasks`, also run:

```bash
go test ./internal/worker/vectorworker/...
```

Full suite verification remains:

```bash
go test ./...
```

## Acceptance Criteria

1. For new fresh hierarchical vectorization tasks, the stored `edu_video_segment.content_summary` matches the final refined summary.
2. The embedding input and stored segment title use the same summary.
3. Recommendation results using `segment_title` return the corrected summary.
4. ASR call counts do not increase for healthy high/medium-confidence segments.
5. Low-confidence or mismatch segments still use at most one expanded-window ASR pass in the fresh hierarchical path.
6. If embedding fails, `content_summary` is not updated independently.
7. Existing resume behavior remains compatible.

## Implementation Notes

Keep the change narrow:

1. Introduce the structured refine result inside `tasks/asr.go`.
2. Update existing tests for the new return contract.
3. Track `orderedSummaries` through the refine and embedding flow.
4. Include `content_summary` in the final transactional update.
5. Avoid unrelated prompt, schema, or queue changes.

The expected code impact should be limited to the vector worker tasks package, with possible call-site adjustments only where the changed helper signature is used.

