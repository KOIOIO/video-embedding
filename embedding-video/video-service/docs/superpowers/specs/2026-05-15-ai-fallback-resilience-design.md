# AI Fallback Resilience Design for `video-service`

## Summary

This design adds resilience for two different AI workloads in `video-service`:

1. `vector_worker` should favor asynchronous compensation when Ali Bailian is unavailable.
2. interactive Q&A should favor availability through circuit breaking, fallback model routing, and a deterministic degraded answer path.

The goal is not to hide upstream outages. The goal is to keep the system usable when the primary LLM provider is fully or partially unavailable.

## Current State

The project already separates workloads into HTTP handlers, application services, and workers. That structure is a good fit for resilience behavior:

1. `internal/worker/vectorworker` already runs as a background pipeline and can tolerate delayed completion better than request/response APIs.
2. `internal/application/videoapp/question.go` and related HTTP handlers currently depend on live upstream LLM calls for answer generation.
3. The codebase already uses Redis for streams and short-lived state, so Redis can also carry circuit-breaker state, retry scheduling, and degraded-mode markers.

## Goals

1. Keep `vector_worker` tasks from being lost when Ali Bailian is down.
2. Keep Q&A endpoints responsive when the primary model is failing.
3. Avoid long request hangs caused by repeated upstream timeouts.
4. Preserve a clear recovery path when the upstream provider comes back.

## Non-Goals

1. Building a full multi-vendor model orchestration platform.
2. Guaranteeing identical answer quality during outages.
3. Rewriting the current vectorization or question domain model.

## Design Overview

The system will use two different resilience strategies:

1. `vector_worker`: async retry, backoff, and dead-letter handling.
2. Q&A: primary model circuit breaker, fallback model routing, and a final degraded answer path.

This keeps the background pipeline reliable without forcing the user-facing Q&A path to wait on repeated upstream retries.

## Detailed Design

### 1. `vector_worker` Resilience

#### Problem

If Ali Bailian is down, `vector_worker` should not fail the whole task immediately in a way that loses work. The worker can wait and retry because it is a background system, not a synchronous request path.

#### Behavior

1. On primary model timeout or provider outage, classify the failure as retryable.
2. Requeue the vector task with exponential backoff and jitter.
3. Limit retries with a fixed maximum retry count.
4. Move exhausted tasks to a dead-letter stream with error metadata.
5. Preserve task state so recovery can continue after the provider recovers.

#### Data Model

Extend vector tasks with retry metadata if needed:

1. retry count
2. last failure stage
3. last error summary
4. next visible time if delayed requeue is used

#### Worker Rules

1. Treat provider timeout, connection reset, 429, and 5xx as retryable.
2. Treat invalid payloads, schema errors, and repeated deterministic parse failures as terminal.
3. If the retry limit is reached, write the task to dead-letter and mark it failed.

#### Why this design

This matches the worker's nature. It preserves throughput and avoids user-visible failure loops while keeping the task durable.

### 2. Q&A Resilience

#### Problem

Q&A is request/response traffic. If the primary model fails, the API must return quickly. Waiting through long retries is worse than returning a degraded but useful answer.

#### Behavior

1. Wrap the primary Ali Bailian client in a circuit breaker.
2. Use a short request timeout for primary calls.
3. If the breaker is open, route directly to the fallback model.
4. If both primary and fallback fail, return a deterministic degraded response.
5. Cache successful answers for high-frequency prompts where safe.

#### Fallback Order

1. Primary Ali Bailian model.
2. Secondary model provider or local lightweight model.
3. Retrieval-based answer or templated degraded response.

#### Degraded Response

The degraded response should be explicit and stable. It should:

1. tell the user the system is under temporary AI provider degradation,
2. provide any safe retrieval or historical result already available,
3. avoid inventing unsupported details,
4. keep the API contract predictable.

#### Circuit Breaker Rules

1. Open the breaker after a small consecutive failure window.
2. Keep it open for a short cooldown window.
3. Allow periodic half-open probes to test recovery.
4. Close the breaker after successful probes.

### 3. Shared Support Layer

#### Problem

Both workloads need a consistent way to classify upstream errors and track degraded state.

#### Changes

1. Add a small provider-health helper that records recent failures in Redis or in-memory counters.
2. Add error classification helpers for timeout, throttling, transport failure, and terminal provider errors.
3. Add fallback routing config so the application can choose between providers without hardcoding behavior into handlers.

#### Why this design

It keeps fallback logic centralized and avoids spreading provider-specific conditionals across HTTP handlers and workers.

## File Scope

### Likely files to modify

1. `internal/application/videoapp/question.go`
2. `internal/http/handler/question.go`
3. `internal/worker/vectorworker/*`
4. `internal/infrastructure/embedding/client.go` or the provider client layer used by Q&A/vector flows
5. `internal/http/router/router.go` if a middleware hook is needed for observability or response shaping

### Likely new files

1. `internal/application/videoapp/ai_resilience.go`
2. `internal/application/videoapp/ai_resilience_test.go`
3. `internal/infrastructure/ai/fallback_router.go`
4. `internal/infrastructure/ai/fallback_router_test.go`

## Testing Strategy

1. Verify `vector_worker` retry classification with primary-provider timeout and terminal failure cases.
2. Verify Q&A switches to fallback after breaker open state.
3. Verify degraded response is returned when all providers fail.
4. Verify a recovered provider closes the breaker after successful probe traffic.

## Risks

1. Fallback answers may be lower quality than primary model answers.
2. Retry loops can amplify load if retry windows are too aggressive.
3. Cache staleness can surface outdated answers if caching rules are too broad.

## Decision

Use asynchronous compensation for `vector_worker`, and use circuit breaking plus fallback routing for Q&A. This is the smallest design that preserves both throughput and user-facing availability without adding a full orchestration layer.
