## Background

The latest hierarchical segmentation improvements strengthened prompt constraints, added mismatch heuristics, introduced merge-first repair, and widened selective refine ASR triggers. Those changes improved the pipeline, but the highest-priority requirement is now explicit:

`content_summary` must match the actual video content in the segment.

This means the system must no longer treat the first generated `content_summary` as a fixed truth. If the segment text changes during repair, or if the original summary overstates what the segment actually contains, the summary itself must become repairable.

The correct invariant is:

- the segment text and the summary must agree before persistence

If they do not agree, the pipeline should first try to fix the segment text, and then, if needed, rewrite the summary so that the final stored pair is consistent.

## Goals

- Make summary-content alignment the top priority in hierarchical segmentation.
- Ensure every persisted `content_summary` is supported by the final segment text.
- Prefer fixing segment text first, but allow summary rewrite when the text is already stable and the summary is still inaccurate.
- Keep selective ASR bounded and avoid returning to the old full-refine strategy.

## Non-Goals

- Restoring full multi-probe ASR for all segments.
- Introducing a database schema change.
- Building a full semantic parser or external NLP pipeline.
- Replacing the existing prompt-based segmentation architecture.

## Problem Statement

The current pipeline can still produce these failure modes:

- the segment text is incomplete, and the summary describes a larger completed topic
- the segment text has been repaired, but the old summary was never updated
- the summary is title-like and clean, but it no longer matches the final text span

This reveals a structural issue:

- the pipeline currently repairs text more than it repairs summary
- summary generation is treated as an early output, not a validated final output

That is incompatible with the product requirement that summary-to-video-content matching is the highest priority.

## Chosen Direction

Use a four-step closed loop:

1. Build candidate segment text.
2. Validate summary-text consistency.
3. Repair text first when possible.
4. Rewrite summary when text is stable but the summary is still inaccurate.

This changes `content_summary` from a one-time byproduct of initial segmentation into a value that can be corrected before persistence.

## High-Level Design

### 1. Segment text remains the primary factual source

The final segment text should be treated as the source of truth for what the segment actually contains.

Text collection order remains:

- default to coarse transcript reuse for healthy segments
- use merge/extension/repair for risky segments
- use single-shot refine ASR when required by risk heuristics

The summary should not outrun the final text.

### 2. Introduce a summary-text alignment gate before persistence

Before a segment is written as final, the pipeline should ask:

- does the final text fully support the current summary?

If no:

- attempt text repair first
- if the text is already stable or sufficiently complete, rewrite the summary

This alignment gate becomes mandatory for final persistence.

### 3. Prefer text repair before summary rewrite

The system should avoid rewriting summary too early, because some mismatches are caused by segment truncation rather than summary error.

Repair order:

1. Merge with adjacent continuation segment.
2. Small extension of the segment boundary.
3. Single-shot refine ASR.
4. Summary rewrite only after text repair is exhausted or text is already judged stable.

This keeps summaries grounded in the best available text.

### 4. Use hybrid summary rewrite

Summary rewrite should use a hybrid strategy:

- simple segments: rule-based title compression from final text
- complex segments: LLM rewrite from final text

This keeps cost controlled while preserving quality for harder segments.

## Detailed Design

### Final text states

Each segment should conceptually move through these states:

1. Initial text candidate from coarse transcript reuse or refine ASR.
2. Repaired text candidate after merge or extension.
3. Stable final text.
4. Final summary aligned to that stable text.

Only the final aligned pair should be persisted.

### Summary-text alignment rules

The first iteration should treat a summary as misaligned when any of the following is true:

- the summary describes a complete definition, step, or conclusion, but the text looks incomplete
- the summary implies more scope than the text plausibly covers
- the summary includes a concept not actually present in the final text
- the text still looks like a continuation fragment rather than a self-contained unit

These rules should remain heuristic and local to the task package.

### Stable-text decision

Summary rewrite should only happen after the segment text is considered stable enough.

Text can be considered stable when at least one of these holds:

- the segment did not trigger mismatch repair
- merge or extension repaired the segment and the final text now looks self-contained
- a selective refine ASR pass completed successfully

If text is not yet stable, the pipeline should continue text repair before attempting summary rewrite.

### Rule-based summary rewrite

For simple segments, generate the final summary using deterministic rules derived from the final text.

Requirements:

- keep the result short and title-like
- use the dominant concept in the first complete sentence or clause
- avoid multi-sentence descriptive summaries
- avoid introducing concepts not explicitly present in the text

The goal is not perfect wording. The goal is safe alignment.

### LLM summary rewrite

For more complex segments, call the LLM to rewrite `content_summary` based on the final text only.

Use LLM rewrite only when necessary:

- the text is long or contains multiple sub-parts
- rule-based compression yields a weak or awkward title
- the segment includes a richer explanation where deterministic compression is likely to degrade quality

The LLM prompt for rewrite should explicitly require:

- title-like output
- only concepts supported by the provided text
- no speculative or generalized phrasing

### When to choose rule-based vs LLM rewrite

Use rule-based rewrite when:

- the final text is short
- the topic is clearly singular
- a short deterministic title can be safely extracted

Use LLM rewrite when:

- the final text is longer
- multiple topic fragments exist but still belong to one segment
- deterministic rewriting would either over-shorten or mislabel the content

This is a hybrid quality/cost tradeoff.

## Cost Strategy

To protect the 1.5x ASR target while elevating summary alignment:

- continue to use selective refine ASR rather than full-refine ASR
- keep summary rewrite mostly rule-based for simple segments
- only use LLM rewrite for complex segments
- only repair text further when the alignment gate fails

This makes summary correctness the top priority without blindly paying ASR cost for every segment.

## Implementation Outline

### `internal/worker/vectorworker/tasks/summary_alignment.go`

- expand mismatch logic from “does this segment look risky?” to “is this summary aligned to the final text?”
- add stable-text checks
- add rule-based summary rewrite helpers
- add hybrid rewrite decision helpers

### `internal/worker/vectorworker/tasks/asr.go`

- keep text-first repair behavior
- after final text is chosen, route segments through the alignment gate
- if text is stable but summary fails alignment, invoke rule-based or LLM rewrite path

### `internal/worker/vectorworker/tasks/hierarchical.go`

- keep stronger segmentation prompt constraints
- add a dedicated prompt builder for summary rewrite if LLM rewrite is needed in the task package

### `internal/worker/vectorworker/task.go`

- keep fresh-path hint wiring
- ensure repaired text and possibly rewritten summary are the versions persisted and passed to embedding generation

## Testing Strategy

Add tests before implementation for the following behaviors:

- summary alignment gate flags summary overreach even when the text itself is stable
- stable text can trigger summary rewrite without forcing extra ASR
- simple final text uses rule-based summary rewrite
- complex final text is routed to LLM rewrite selection logic
- final persisted summary matches the final text-oriented expectations

Prefer focused unit tests in `internal/worker/vectorworker/tasks`, with existing package-level tests covering the integrated path.

## Risks

### Risk: over-rewriting summaries

If the alignment gate is too strict, summaries may be rewritten too often and lose some stylistic consistency.

Mitigation:

- prefer text repair first
- use conservative alignment rules for summary rewrite

### Risk: deterministic rewrite yields awkward titles

Rule-based rewrite may produce less elegant titles for complex educational content.

Mitigation:

- use LLM rewrite only for the complex cases where deterministic compression is weak

### Risk: extra LLM calls increase cost or latency

Hybrid rewrite adds a new optional LLM call path.

Mitigation:

- keep rule-based rewrite the default
- send only complex segments to LLM rewrite

## Verification Plan

After implementation, validate on long videos that previously showed summary-content mismatch.

Success indicators:

- fewer segments where `content_summary` overstates or outruns the final text
- fewer half-topic segments stored with complete-sounding summaries
- cases where summary changed now produce a better match to final content
- ASR remains bounded below the old full-refine cost profile

## Recommendation

Adopt this as the next stage of the hierarchical quality pipeline.

The final operating principle becomes:

- text is repaired until stable enough
- summary is validated against that text
- summary is rewritten when necessary
- only aligned summary-text pairs are allowed through the final gate

This is the design most consistent with the product requirement that summary-to-video-content matching has the highest priority.
