## 1. Route view retention

- [x] 1.1 Add a red application-shell test for list reuse after detail/statistics navigation.
- [x] 1.2 Render only `JobListView` inside the application shell's `KeepAlive` boundary.
- [x] 1.3 Verify changed list route queries still trigger the existing request watcher.

## 2. Verification

- [x] 2.1 Run focused frontend tests and type-check/build checks.
- [x] 2.2 Run OpenSpec strict validation and `git diff --check`.
