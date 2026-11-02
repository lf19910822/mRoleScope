# Tasks

## 1. Version-aware DWD and serving foundation

- [x] 1.1 Map the immutable source, presence-period, content-version, and four deterministic formal-result seams; write a migration-backed red integration test that proves a published-run membership row retains those identities and a per-dimension resolved result without changing ODS tables; verify the test fails before the serving schema exists.
- [x] 1.2 Add additive DWD/serving migrations for append-only published-run membership/history, current lookup, resolved formal-dimension references, rank/search support, and targeted indexes; verify migrations apply and roll back in the integration database without modifying ODS source records.
- [x] 1.3 Implement idempotent, best-effort reconciliation from durable published facts into the serving projection; verify an isolated reconciliation failure is audited/retryable and does not fail source publication or unrelated downstream work.
- [x] 1.4 Implement transparent per-dimension formal-result resolution (current version first, then the same job's newest completed deterministic result, otherwise unavailable); verify red/green integration cases cover education, experience, technology, project, and job-family values, prohibit AI projections, and retain historical membership for future charts.

## 2. Read-only JD serving API

- [x] 2.1 Define typed REST/OpenAPI list, filter-option, and detail DTOs that expose only safe display data and opaque JD identifiers; verify contract tests reject provenance IDs, parser/dictionary versions, audit state, AI output, and credentials.
- [x] 2.2 Write failing repository/API tests for composable list filtering: OR within one dimension and AND across title, main family, other family, technology, education, experience, deterministic title-project, location, activity state, first-discovered date, and urgent flag; implement the bounded parameterized queries and make the tests pass.
- [x] 2.3 Add case-insensitive title exact/substring matching and fixed-threshold PostgreSQL fuzzy matching, title-only; verify exact/substrings rank first, typo cases are deterministic, and responsibility/requirement/supplemental body text and synonym expansion never match.
- [x] 2.4 Implement validated pagination, safe client errors, deterministic tie-breaking, and education/experience ascending and descending sorts with unavailable values always last; verify API tests cover invalid input, stable page boundaries, upper-bound education display, and missing values.
- [x] 2.5 Implement the JD detail handler using only the serving resolver and current normalized body sections; verify active and confirmed-closed details work, absent supplemental text is omitted, and history comparison/audit fields remain unavailable.
- [x] 2.6 Run representative `EXPLAIN (ANALYZE, BUFFERS)` checks against the deployed-scale fixture and refine only evidenced DWD/serving indexes; verify the documented query plan stays bounded without adding ODS-side indexes for this UI.

## 3. Vue JD browser

- [x] 3.1 Replace the starter Vue screen with typed API client, list route, detail route, and URL-backed list state; verify component/router tests preserve filters, sorting, pagination, and scroll-relevant list state after opening and returning from a detail.
- [x] 3.2 Build the list filters, sort controls, and option search using the approved field vocabulary: 主岗位族、其他命中岗位族、技术关键词、学历要求、最低工作年限、项目、工作地点、活跃状态、首次发现日期、紧急标记; verify UI tests show OR-within/AND-across behavior through API requests and default to still-effective jobs.
- [x] 3.3 Build the readable list/detail presentation following the repository's `frontend-design` guidance and the supplied Ocean Breeze light-theme tokens; verify one main job family and other matches remain distinct, education renders 专科及以下/本科/硕士/博士 with upper-bound meaning, and an absent requirement is not rendered as zero.
- [x] 3.4 Add focused browser-flow tests for fuzzy title search, filter combinations, active/confirmed-closed selection, pagination, and JD detail rendering; verify no screen renders current-coverage, fallback, processing, parser, audit, provider, or historical-difference information.

## 4. Secure web-edge packaging and release chain

- [x] 4.1 Add a reproducible Web Edge image that serves the built Vue assets and same-origin `/api/` proxy, terminates TLS for one configured hostname, denies unknown hosts, and makes only a redirect/certificate path available on HTTP; verify container tests show no assets or JD data are served before HTTPS authentication.
- [x] 4.2 Add initial HTTP Basic authentication and edge rate limiting, taking only a bcrypt password hash and hostname from root-owned runtime configuration; verify secrets are absent from Git, image layers, frontend assets, and GitHub Actions logs, and both page and `/api/` deny unauthenticated requests.
- [x] 4.3 Extend Compose networking so only Web Edge publishes the necessary 80/443 ports while API and PostgreSQL stay internal/non-public; verify `docker compose config`, listener inspection, and a negative direct-API access test.
- [x] 4.4 Extend Docker Hub build, GitHub Actions, root-owned deployment helper, bootstrap asset manifest, immutable digest allowlist, release record, and rollback path from three to four verified images; verify mutable tags, foreign namespaces, and an omitted Web Edge digest are rejected while the existing API/migrate/worker release path remains valid.
- [ ] 4.5 Add deployment smoke checks for the approved hostname: certificate validity, HTTPS-only authenticated list/detail access, same-origin API requests, unknown-host denial, and independent Web Edge rollback; verify API, worker, ODS sync, immutable history, and deterministic analysis remain operational when the edge is stopped.

## 5. Manual domain and hosting release gate

- [ ] 5.1 Before enabling any public browser listener, the operator selects and acquires the production domain through a registrar, completes required ownership/real-name steps, enables account MFA, auto-renewal, and transfer lock, and records only the hostname plus completion evidence in the operational runbook; verify no registrar password, payment data, or identity document enters Git, CI, CVM application configuration, or chat logs.
- [ ] 5.2 Before DNS points the domain at the mainland CVM, the operator confirms and completes every Tencent Cloud/provider/regulatory prerequisite applicable to public web hosting (including ICP filing where required); verify the operational runbook records the determination and approval/reference without storing sensitive filing material.

## 6. Integrated verification and handoff

- [x] 6.1 Run focused Go, migration-backed integration, Vue unit, browser-flow, deployment static-check, and `openspec validate add-jd-search-serving-ui --strict` checks; verify `git diff --check` is clean and record command results without JD bodies or credentials.
- [x] 6.2 Update the frontend delivery guide and operational deployment documentation with the approved scope, serving fallback boundary, security model, manual domain/hosting gate, release verification, and rollback behavior; verify commands and configuration names match the implemented assets.

## 7. Final public release

- [ ] 7.1 Obtain explicit release approval only after tasks 1–6, including the manual domain/hosting gate, are complete; verify the approval identifies the approved hostname and does not include credentials or personal verification material.
- [ ] 7.2 After task 7.1, delegate DNS and create the hostname record, then enable public 80/443 only for the Web Edge; verify the raw IP is not documented or accepted as a product entrypoint and the approved domain passes the task 4.5 smoke checks.

## Workflow follow-up

- Review the completed change before deployment.
- Do not alter DNS, open public ports, or deploy the Web Edge publicly until the final public-release tasks are explicitly completed and approved.
- Archive the change after the project's review requirements are satisfied.
