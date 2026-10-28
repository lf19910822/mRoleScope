# Proposal

## Why

The current source `projectId`/`projectName` fields describe the recruitment
source rather than a game or internal direction, while many JD titles contain
an explicit, human-recognizable project or direction label.  Product-facing
project filtering needs a reproducible title-based result without guessing
from a JD body, technology stack, source metadata name, or AI model output.

## What Changes

- Add a deterministic project-tagging pipeline that matches only an approved,
  versioned manual dictionary against normalized JD titles.
- Classify dictionary entries explicitly as either a named game or a source
  direction/research label; retain all simultaneous title matches rather than
  inferring one winner.
- Persist a completed result even when a title has no match, so `unidentified`
  is distinguishable from not-yet-processed work and daily processing remains
  idempotent.
- Record dictionary version, parser identity, matched keyword, and Unicode
  title coordinates for every title assignment.  Do not treat title assignment
  as a JD-body analysis or an AI result.
- Reprocess a title when the title itself or the immutable project dictionary
  changes; reuse the result when unrelated source metadata changes.
- Remove project names from the AI portion of the JD parsing objective and
  document source project fields and body mentions as separate concepts.

## Capabilities

### New Capabilities

- `deterministic-project-tagging`: Derive, persist, and expose evidence-backed
  game-project and direction labels from approved title keywords without AI.

### Modified Capabilities

- None.

## Impact

- New additive PostgreSQL tables and repository/read APIs for immutable
  project dictionaries, title-level results, and match evidence.
- A deterministic worker stage which runs after successful synchronization,
  independently of body analysis, technology publication, and provider
  credentials.
- Updates to the JD modelling objective and operating guidance.  Existing
  source metadata, body versions, AI results, and technology analysis records
  remain unchanged.
