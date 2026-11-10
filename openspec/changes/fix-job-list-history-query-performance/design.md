# Design

## Context

The last-modified-date query's CTE is inlined, and the measured PostgreSQL
plan evaluates historical transitions once per matching current job. The
production read-only comparison on 2026-11-10 showed 166 aggregate executions
and 545–552 ms for the main query, versus one execution and 30–32 ms when the
aggregate is materialized. The test compared 20 result rows without differences;
it did not establish parity for every filter or data state.

## Goals / Non-Goals

Avoid repeated aggregate work per result while maintaining history and sorting
semantics. Do not add a schema, cache, new API, background process, or a
timing threshold susceptible to machine load.

## Decisions

Materialize the history aggregate in the current statement, before it is joined
to current jobs. It is still computed from published history in the same MVCC
snapshot, including repairs, closures and reopenings. Unlike a persisted
projection, it introduces no synchronization or invalidation obligations.

Exercise the real list query with a synthetic PostgreSQL fixture and a forced
nested-loop join plan. Observe the actual execution count of the historical
transition subplan: the aggregate must not re-execute for each matching job.
This plan-level assertion targets the specific observed performance regression;
existing repository and HTTP tests remain the behavioral contract for results.
No wall-clock threshold is used as a test assertion.

## Risks / Trade-offs

- Materialization can restrict optimizer choices for other data shapes; measure
  both realistic synthetic data and the current read-only database before release.
- PostgreSQL planner nodes may change across major versions; constrain the
  plan regression to the supported PostgreSQL 16 and keep behavior tests independent.
