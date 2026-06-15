## Background

The current hierarchical vectorization pipeline has improved ASR cost by reusing coarse ASR text for most fresh-path segments and limiting refine ASR to low-confidence segments. That reduced repeated ASR calls, but it exposed a more important quality issue:

- the cut segment content often does not match its `content_summary`
- some segments stop halfway through the topic they claim to summarize
- some segments contain only a very short slice of actual content, but the summary describes a complete knowledge unit

This is no longer just a boundary naturalness issue. It is a semantic alignment issue between:

1. the segment boundary
2. the actual spoken content captured inside the segment
3. the `content_summary` generated for that segment

The optimization target therefore changes from “reduce ASR while preserving rough quality” to “ensure each segment is a semantically closed unit whose `content_summary` is fully supported by the captured text, while still keeping ASR well below the old full-refine cost.”

## Goals

- Ensure `content_summary` is fully supported by the text content inside the same segment.
- Reduce cases where segment text ends mid-thought or only captures half of the described concept.
- Prefer slightly longer but semantically complete segments over shorter but incomplete ones.
- Keep total ASR usage below the previous near-2x behavior and target about 1.5x video duration or lower.

## Non-Goals

- Restoring full multi-probe refine ASR for all segments.
- Introducing word-level timestamp alignment.
- Redesigning the entire hierarchical pipeline or changing storage schema.
- Replacing the current embedding or LLM providers.

## Problem Summary

The current behavior can produce mismatches like:

- `content_summary` describes a full definition, example, or conclusion, but the transcript only contains the first half.
- The actual captured text belongs partly to the next topic, but the summary still claims the current topic is complete.
- A very short segment inherits a title-like summary that is too broad for the amount of text in that segment.

This means the pipeline is currently optimizing too early for cost and not enough for semantic closure.

## Chosen Direction

Use a three-layer quality strategy with explicit priority:

1. Improve segmentation semantics first.
2. Add post-segmentation summary-content alignment checks second.
3. Use selective refine ASR only as the last repair tool.

This means ASR is no longer the primary mechanism for fixing bad segments. The primary mechanism becomes better segment semantics and targeted repair when segment text cannot support the summary.

## High-Level Design

### 1. Strengthen LLM segmentation constraints

The segmentation prompt should be made stricter about the relationship between segment boundaries and `content_summary`.

New required prompt behavior:

- `content_summary` must only describe content fully covered inside the current segment.
- If a concept, step, example, or conclusion is not finished in the segment, it must not be summarized as complete.
- A segment should prefer semantic closure over equal duration.
- If one knowledge unit spans a bit longer, the segment may be longer instead of splitting at a fragile midpoint.

This directly targets the root cause where the LLM generates a neat title before the spoken content has actually reached closure.

### 2. Add summary-content alignment validation after LLM segmentation

After `NormalizeLLMSegments`, the pipeline should evaluate whether each segment looks semantically self-consistent.

This validation is not a full NLP system. It should be a lightweight rule-based gate using the information already available:

- segment duration
- `content_summary`
- neighboring segment summaries
- coarse ASR text overlapped by the segment
- boundary confidence

The validator should label a segment as risky when it appears that the summary is broader or more complete than the text actually captured in that segment.

### 3. Repair order: merge first, extend second, refine ASR last

When a segment is flagged as summary-content mismatched, the repair sequence should be:

1. Try merging with the next segment if both appear to belong to the same topic.
2. If merge is not appropriate, try small boundary extension.
3. Only then trigger single-shot refine ASR.

This keeps ASR as a last-mile repair tool instead of the default answer.

### 4. Upgrade selective refine trigger logic

Refine ASR should no longer depend only on `boundary_confidence == low`.

The new trigger should include:

- `boundary_confidence == low`
- summary-content mismatch risk
- very short segment with an overly broad summary
- obvious half-sentence start or end patterns

This lets the pipeline repair segments that the LLM mislabeled as medium or high confidence even though the segment text is not self-contained.

## Detailed Rules

### Prompt constraints

The hierarchical prompt should explicitly say:

- `content_summary` must be supported by the actual spoken content inside this segment only.
- Do not summarize a concept, example, step, or conclusion if it is only partially covered in the segment.
- If the current topic continues into the next sentence and the current boundary would cut it, keep it in the same segment when possible.
- Prefer complete knowledge units over evenly sized segments.

The retry prompt should repeat the same rules so the correction path does not regress.

### Summary-content mismatch heuristics

The first iteration should use a small rule set.

Flag a segment as risky if any of the following is true:

- The segment is short, but `content_summary` implies a complete concept or complete explanation.
- The coarse transcript text overlapped by the segment is too short to plausibly support the summary.
- The transcript looks like it starts in the middle of an explanation.
- The transcript looks like it ends before the sentence or topic closes.
- The current and next segment summaries appear to belong to the same topic and the current segment text looks incomplete.

The implementation should stay heuristic and local. It should not introduce external NLP dependencies.

### Merge-first repair

When the mismatch suggests the current segment is just the first half of a topic, prefer merging with the next segment.

Expected merge conditions:

- the next segment begins as a continuation of the same topic
- current text looks incomplete
- current and next summaries are topically close or obviously sequential

After merging:

- keep one summary that reflects the combined content
- preserve the merged time span
- reuse existing normalization patterns where possible

### Small extension repair

If merging is too aggressive, allow a small extension of the segment boundary before refine ASR.

Expected behavior:

- extend only within safe bounds
- avoid excessive overlap into the next knowledge unit
- prefer just enough extra context to close the sentence or idea

This extension should be lighter than the old repeated boundary probing.

### Selective refine ASR repair

If mismatch remains after the earlier checks, trigger single-shot refine ASR.

Refine ASR should remain constrained:

- one ASR call per repaired segment
- expanded window allowed
- no multi-candidate probing
- if ASR fails, degrade gracefully to the best available coarse transcript path

## Cost Strategy

To keep ASR near or below the 1.5x target:

- high-confidence, self-consistent segments should continue using coarse transcript reuse
- low-confidence segments should still refine
- medium or high-confidence segments should only refine when mismatch heuristics are triggered
- merge and extension should be preferred over ASR where they can solve the issue

This should recover a large part of the lost semantic quality without returning to near-2x ASR consumption.

## Implementation Outline

### `internal/worker/vectorworker/tasks/hierarchical.go`

- tighten prompt language for `content_summary`
- add explicit “summary must be fully supported by this segment” instructions to both main and retry prompts

### `internal/worker/vectorworker/tasks`

- add lightweight summary-content mismatch helpers
- keep them local to the task package
- reuse existing normalization and boundary utilities where possible

### `internal/worker/vectorworker/tasks/asr.go`

- expand selective refine trigger logic beyond low confidence
- add a path for mismatch-driven single-shot refine
- keep coarse transcript reuse as the default for clearly healthy segments

### `internal/worker/vectorworker/task.go`

- preserve the existing fresh-path hint passing
- allow repaired segment decisions to flow into refine stage selection without broad architectural changes

## Testing Strategy

Add tests before implementation for the following behaviors:

- prompt includes the stronger summary-support constraints
- mismatch heuristics flag short or incomplete segments with oversized summaries
- segments that clearly continue into the next topic are merged or marked for repair
- high-confidence but mismatched segments can still trigger refine repair
- healthy high-confidence segments still avoid unnecessary ASR

Prefer focused unit tests in `internal/worker/vectorworker/tasks`.

## Risks

### Risk: heuristics merge too aggressively

If mismatch detection is too sensitive, unrelated neighboring topics may be merged.

Mitigation:

- start with a small conservative rule set
- require multiple mismatch signals for merge where possible

### Risk: prompt improvements alone are not enough

The LLM may still occasionally produce good-looking summaries for incomplete segments.

Mitigation:

- keep the post-segmentation validator as an independent guard

### Risk: ASR cost creeps upward

Expanding refine triggers can increase cost.

Mitigation:

- prefer merge and extension before ASR
- only refine segments that are actually risky
- log the number of mismatch-triggered refine calls

## Verification Plan

After implementation, validate using long videos with the same failure pattern.

Success indicators:

- fewer segments where `content_summary` overstates the text actually captured
- fewer obviously truncated segment texts
- fewer very short segments carrying overly broad summaries
- total ASR consumption remains clearly below the old full-refine behavior and near the new 1.5x target

## Recommendation

Implement this as an incremental quality correction on top of the current selective refine design.

The key principle is:

- first make segments semantically complete
- then validate whether the summary matches the captured text
- only then spend additional ASR budget on the risky cases

This directly addresses the current production issue: `content_summary` and segment content must match, and incomplete half-topic cuts are more damaging than slightly longer segment lengths.
