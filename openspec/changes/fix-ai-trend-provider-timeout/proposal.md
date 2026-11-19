# Proposal

## Why

The live trend route waits for the full Qwen output budget and can remain in a
thinking state for 75 seconds before returning provider failure. The current
prompt also allows a category question such as “哪些数据库” to be converted
into a metric name, producing an unusable draft.

## What Changes

- Bound the trend intent response to a compact output budget suitable for its
  small JSON contract.
- Clarify that expressions represent user requested values or categories and
  never metric names or schema fields.
- Add regression coverage for the compact budget and category wording.

## Capabilities

### New Capabilities

- `live-ai-trend-analysis`: Provides bounded, category-correct Qwen intent
  analysis for the authenticated trend draft route.

### Modified Capabilities

## Impact

Affected Go Qwen adapter, intent prompt, tests, and trend AI guidance. No
public endpoint shape, credential handling, Serving data, or chart application
contract changes.
