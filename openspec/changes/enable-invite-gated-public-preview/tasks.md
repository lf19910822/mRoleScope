# Tasks

## 1. Access-session foundation

- [x] 1.1 Add root-runtime configuration for the shared invite hash and session signing secret, validate required formats and `0600` ownership assumptions, and verify malformed/missing configuration fails closed in focused Go tests.
- [x] 1.2 Implement invite verification, three-day opaque session issuance, signature/expiry/hash-fingerprint validation, and invite-rotation invalidation; verify correct, incorrect, expired, malformed, and rotated-session cases with unit tests.
- [x] 1.3 Add the anonymous verification endpoint and private session-check endpoint without persisting sessions or viewer records; verify response status/body redaction and existing business API response contracts in HTTP tests.
- [x] 1.4 Add verification rate limiting and sanitized request/error logging; verify burst failures are limited and invite codes, cookies, authorization values, request bodies, and JD bodies do not appear in captured logs.

## 2. Browser invite experience

- [x] 2.1 Add a minimal standalone invite entry page and its dedicated static assets without loading the business SPA or business API; verify the page build and the unauthenticated asset allowlist.
- [x] 2.2 Implement successful verification, generic failure feedback, redirect to the existing browser root, and three-day revisit behavior; verify these states with frontend unit/component tests.
- [x] 2.3 Handle expired/revoked-session responses by returning the visitor to the invite flow without losing the existing job-list/detail/trend navigation contracts; verify with router and API-client tests.
- [x] 2.4 Update the access-page and public-preview documentation while preserving the user-facing rule of one verification unlocking all existing features; verify the documented paths match the built application.

## 3. Web Edge enforcement

- [x] 3.1 Replace browser Basic Auth with exact anonymous access routes plus Nginx `auth_request` protection for the business SPA, business assets, and `/api/`; verify unknown hosts, raw IPs, anonymous assets, anonymous API calls, and authorized same-origin calls with Nginx/container tests.
- [x] 3.2 Configure secure response headers, cookie forwarding, per-source verification/API rate limits, and non-permissive CORS behavior; verify headers, rate-limit statuses, and absence of invite/session secrets in rendered configuration.
- [x] 3.3 Update Compose and Web Edge entrypoint validation for the new runtime values while keeping API/database networks private and the `web` profile disabled by default; verify deployment-config checks and container startup tests.
- [x] 3.4 Update the Web Edge readiness probe and deployment helper rollback path from Basic Auth `401` expectations to invite-gate smoke behavior; verify candidate failure rolls back only the Web Edge digest.

## 4. Certificate and host operations

- [x] 4.1 Add root-controlled ACME client installation/configuration for the Ubuntu CVM, initial HTTP-01 standalone issuance, root-only certificate installation, and a prelaunch check that Web Edge is not serving business traffic; verify shell syntax and a mocked issuance flow without real credentials.
- [x] 4.2 Add webroot renewal, atomic certificate replacement, Web Edge reload, previous-certificate fallback, and expiry/failure checks using the fixed TLS and ACME directories; verify renewal success/failure behavior with isolated integration tests.
- [x] 4.3 Add systemd service/timer assets and bootstrap installation for certificate renewal, without changing runtime secrets or enabling the public profile automatically; verify installed unit contents and timer validation in deployment-asset tests.
- [x] 4.4 Update the secure publishing runbook with Let's Encrypt trade-offs, HTTP-01 port requirements, DNSPod manual boundary, certificate permissions, renewal inspection, and emergency withdrawal; verify the documented commands do not print secrets or expose API/database ports.

## 5. Release and public acceptance tooling

- [x] 5.1 Update the public smoke script to validate HTTP redirect/ACME challenge, trusted HTTPS, invite success/failure, cookie persistence/expiry handling, business API protection, same-origin delivery, and raw-IP/unknown-host rejection without accepting plaintext secrets as arguments; verify the script with mocked responses and `bash -n`.
- [x] 5.2 Add release checks for immutable Web Edge/API image compatibility, runtime gate completeness, private-port boundaries, and sanitized operational logs; verify `make deploy-config-check` and the relevant release-helper tests.
- [x] 5.3 Update `Docs/开发指导/开放/开放决策.md` and the release/security runbooks with the final implementation status, manual DNS/security-group gates, rollback commands, and invitation-distribution boundary; verify no real invite, key, token, or connection string appears in the diff.

## 6. Integrated verification and controlled rollout

- [x] 6.1 Run focused Go/frontend/container tests, formatting, `git diff --check`, and the repository quality gate; verify all commands and any skipped database-dependent checks are reported accurately.
- [x] 6.2 Perform browser visual/interaction acceptance at desktop and 390px widths for the invite flow, three-day revisit simulation, job filtering, job detail/history, and trend dashboard; verify no business resource or API data is returned before verification.
- [ ] 6.3 After code review and approved immutable release, manually confirm DNSPod ownership, add only `@ A 118.89.68.77`, keep `www`/`AAAA` absent, and open only TCP 80/443; verify public DNS propagation and private 8080/5432 boundaries before sending invitations.
- [ ] 6.4 Complete initial certificate issuance, enable Web Edge, execute the public smoke check, and record non-sensitive release facts; verify HTTPS, invite rotation, rollback readiness, and certificate renewal timer before sending the first batch of invitations.
- [ ] 6.5 Observe the first real-experience batch through sanitized status/latency/error signals, pause expansion on regressions, and document findings without creating viewer identity or browsing profiles; verify the operational review record contains no invite or JD content.

## Workflow follow-up

- Archive the change only after implementation verification, public acceptance, and the required protected-main review flow are complete.
- Re-run strict OpenSpec validation and verify the archived capability specs before closing the rollout record.
