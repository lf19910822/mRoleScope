# Spec Delta

## Purpose

This capability keeps live trend intent analysis responsive and ensures the
model preserves the user's requested category or value when selecting curves.

## ADDED Requirements

### Requirement: Compact intent responses complete promptly

The trend provider SHALL request a bounded compact response budget appropriate
for the strict intent contract and SHALL return provider usage when available.

#### Scenario: Normal category question

- **WHEN** an authenticated user asks a supported trend question
- **THEN** intent analysis completes within the route timeout without waiting
  for an unnecessarily large output budget

### Requirement: User expressions remain distinct from metrics

The intent contract prompt SHALL require expressions to contain the user's
requested value or category phrase, while ranking metrics remain in the
ranking metric field.

#### Scenario: Database category question

- **WHEN** the user asks “哪些数据库正在被使用”
- **THEN** the intent contains the technology dimension and a database-related
  expression, and does not use `avg_daily_active_jobs` as an expression
