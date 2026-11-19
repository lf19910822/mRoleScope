# Design

## Context

The Qwen trend adapter currently requests 5,000 output tokens although the
intent contract is a small bounded object. Live testing showed that malformed
category interpretation can make the provider spend the full budget. The
existing runner already enforces a larger server-side output budget and keeps
raw provider content transient.

## Goals / Non-Goals

**Goals:**

- Reduce provider wait time for the trend intent request.
- Make category and metric roles explicit in the prompt.
- Preserve strict validation, usage accounting, retries, and safe errors.

**Non-Goals:**

- No change to the public NDJSON event contract or Serving tools.
- No provider fallback, arbitrary model selection, or client supplied prompt.

## Decisions

- Request `1200` provider output tokens, enough for the bounded multi-subquery
  intent while preventing a runaway 5,000-token generation.
- Add explicit examples and prohibitions to the fixed prompt: expressions are
  user terms such as “数据库” or “Redis”; metrics belong only in ranking.
- Test the request profile and prompt semantics with synthetic transport tests;
  the real end-to-end run remains an operator validation step.

## Risks / Trade-offs

- **Complex multi-subquestion input exceeds 1200 tokens** → strict validation
  returns a safe failure instead of waiting indefinitely; the fixed budget can
  be revised in a later reviewed change.
- **Prompt wording still permits a wrong category** → resolver and draft tests
  retain formal-value validation and expose the resulting match for review.
