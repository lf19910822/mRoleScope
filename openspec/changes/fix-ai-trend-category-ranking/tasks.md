# Tasks

- [x] 1.1 Add a failing regression test for plural category queries whose model Intent incorrectly uses `limit=1`, and define the explicit default limit contract.
- [x] 1.2 Add a failing regression test proving candidates beyond the first eight summary items are fetched and ranked.
- [x] 1.3 Add a failing regression test proving merged `sub_query_ids` are unique.
- [x] 2.1 Implement default limit normalization and update embedded/documented prompts.
- [x] 2.2 Implement bounded server-side summary completion across batches, with partial failure and tool-budget handling.
- [x] 2.3 Fix candidate provenance de-duplication.
- [x] 3.1 Add end-to-end regression coverage for “哪些数据库正在被使用？”.
- [x] 3.2 Run Go tests, OpenSpec strict validation, and `git diff --check`; record results.
