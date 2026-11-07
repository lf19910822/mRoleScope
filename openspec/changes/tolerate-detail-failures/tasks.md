# Tasks

## 1. Collector and manifest

- [x] 1.1 Add a generic per-detail outcome collector and manifest failed-ID/count fields.
- [x] 1.2 Accept a valid partial detail set, reject zero usable details, and cover malformed, transport, ID mismatch, and missing-field cases with tests.

## 2. Go validation and publication

- [x] 2.1 Validate successful/failed list-ID partition and parse valid detail files.
- [x] 2.2 Persist partial observations and skip failed IDs during missing-period closure; audit the partial count.
- [x] 2.3 Add integration/unit coverage for preserving an existing job when its detail fails.

## 3. Verification

- [x] 3.1 Run Python/Go tests, type/build checks, OpenSpec strict validation, and diff checks.
- [ ] 3.2 Deploy through the reviewed self-release flow and verify the next synchronization publishes valid jobs while recording failed IDs.
