# Design

## Context

The current Web Edge terminates TLS, serves the Vue build, proxies same-origin `/api/` requests to the private Go API, and protects every route with browser Basic Auth. The API has no viewer authentication layer. The host already reserves root-owned TLS and ACME challenge directories, but no ACME client or renewal unit is installed. See `proposal.md` and the two capability specs for the desired external behavior.

The implementation must keep PostgreSQL on the Compose `backend` network and the API on the private `edge` network, while keeping the public surface limited to `mrolescope.com` on ports 80/443. The shared invite plaintext must never enter source control, images, logs, URLs, or this planning set.

## Goals / Non-Goals

**Goals:**

- Provide one browser-visible invite page, one successful verification step, and three days of access to the complete existing browser.
- Enforce the boundary at both the Web Edge and Go API so direct API calls cannot bypass the page.
- Invalidate all issued sessions when the configured invite hash changes, without introducing accounts or viewer identity records.
- Replace Basic Auth while preserving existing same-origin browser API paths and private service topology.
- Provide a root-controlled Let's Encrypt HTTP-01 lifecycle with safe initial issuance, renewal, reload, expiry checks, and rollback.
- Make real-experience validation observable through latency/status/error/rate-limit signals without recording JD content or viewer behavior.
- Keep the post-verification browser transition responsive by compressing and privately caching immutable hashed SPA assets, while leaving the invite page, verification endpoint, redirects, and business APIs non-cacheable.

**Non-Goals:**

- No user accounts, per-person invite codes, roles, identity provider, viewer attribution, or browsing analytics.
- No public `www` alias, raw-IP product route, API port, database port, permissive CORS, or DNS API credential on the CVM.
- No automatic DNS changes, security-group changes, plaintext invite injection, or automatic sending of invitations.
- No capacity guarantee or uncontrolled public load test.
- No change to existing job, history, trend, or response data contracts.

## Decisions

### 1. Put the browser gate at the Web Edge and API

Use Nginx `auth_request` for protected HTML and business static assets. Nginx sends an internal subrequest with the request Cookie to a private Go session-check endpoint; only the access page assets and exact invite verification endpoint bypass the check. The generic business `/api/` location also requires the same internal check, while the exact verification location remains anonymous.

This keeps business assets from being delivered before verification and preserves one same-origin API surface. A Vue-only route guard was rejected because a caller could still fetch `/api/` directly. Nginx-only Basic Auth was rejected because its browser prompt does not meet the requested invite-page experience.

The access page is a small static entrypoint with its own JS/CSS assets. The business SPA remains behind the gate. A browser request to the protected root is redirected to `/access`; an unauthenticated business API request remains a non-data `401` response. The access verification response sets the session cookie and redirects the browser to `/`.

### 2. Use a signed, short-lived opaque cookie with hash-rotation invalidation

The Go API owns verification and session validation. It compares the submitted invite code to a root-owned bcrypt hash. On success it signs an opaque cookie payload containing an expiry and a fingerprint of the current bcrypt hash. The payload is authenticated with a separate root-owned session signing secret. Validation requires a correct signature, unexpired timestamp, and current hash fingerprint. Replacing only the invite hash therefore invalidates all prior sessions while preserving the user-facing rotation operation.

The cookie is host-scoped, `Secure`, `HttpOnly`, and `SameSite=Lax`, with a three-day `Max-Age`/expiry. The verification endpoint returns only a generic success or failure result. The API does not persist sessions or viewer records in PostgreSQL.

Runtime values are held only in the root-owned `0600` environment file. The invite plaintext is used only by an operator-side interactive hashing/update procedure and is never accepted as a committed configuration value.

### 3. Keep the public route set narrow

The Web Edge will expose:

- HTTP `80`: ACME HTTP-01 challenge and redirect for the approved hostname; unknown hosts/IP literals are rejected.
- HTTPS `443`: exact approved hostname only.
- Anonymous HTTPS: `/access` and its dedicated static assets, plus exact `/api/v1/access/verify`.
- Authenticated HTTPS: the business SPA, business static assets, and existing `/api/` routes.
- Internal-only Web Edge subrequest: an Nginx `internal` location proxying session checks to the private API.

The API and database remain unbound to host ports. No CORS change is needed because the browser remains same-origin.

### 4. Use Certbot standalone for first issuance and webroot renewal afterward

Install a reviewed host ACME client through the existing Ubuntu root maintenance path. Before Web Edge has a certificate, use a root-controlled standalone HTTP-01 issuance while port 80 is free; do not start the business Web Edge or send invites during this step. Copy/install the resulting chain and key into `/etc/mrolescope/tls` with root-only ownership and permissions, then start Web Edge.

For normal continuous operation, use the existing read-only ACME challenge mount as a webroot. A root-owned renewal timer runs `certbot renew` with a deploy hook that validates the replacement, atomically installs the chain/key into the fixed TLS directory, reloads Web Edge, and records only sanitized status. The previous certificate remains untouched when renewal or reload validation fails. A separate expiry check makes an approaching expiry visible even if the renewal timer fails. The initial standalone issuance is the only step that intentionally runs while Web Edge is stopped; later renewals must not require public application downtime.

Certbot/ACME state stays outside the application image. DNS remains a manual DNSPod operation; no DNS API secret is introduced.

### 5. Keep operational logs minimal and sanitized

Nginx and API logs may retain request path, status, duration, rate-limit events, and a bounded source-IP signal needed for abuse control. Request bodies, invite codes, cookies, authorization values, JD response bodies, and viewer identity/profile data are redacted or never logged. The operational stream is not written to the business database.

### 6. Release and rollback sequencing

Implementation lands through the reviewed immutable-image release path. Runtime TLS files, invite hash, session secret, DNS records, and security-group changes remain manual operator actions. Before DNS is changed, local/container tests validate route protection and the deployment helper validates the candidate Web Edge digest. After DNS and certificate setup, the operator runs a smoke check covering HTTP redirect/ACME path, trusted HTTPS, invite success/failure, cookie persistence/expiry behavior, business API protection, same-origin delivery, and raw-IP/unknown-host rejection.

If the candidate Web Edge fails, roll back only the Web Edge digest or stop the edge and remove the DNS route; do not roll back database migrations or private worker state. If the invite is disclosed, replace the hash and session fingerprint source, then verify old cookies fail before resending the replacement code.

## Risks / Trade-offs

- [Nginx/API configuration coupling] -> Keep the internal session-check endpoint private, add config/container tests, and test both direct asset and direct API bypass attempts.
- [A stolen shared invite can be forwarded] -> Accept the product constraint; rotate the single hash when disclosed and do not claim individual accountability.
- [Certificate renewal failure] -> Use webroot renewal, a deploy hook, expiry checks, the previous-certificate fallback, and an operator smoke command.
- [Port 80 is required for HTTP-01] -> Keep only the ACME challenge and HTTPS redirect on port 80; reject unknown hosts and never serve JD data there.
- [Cookie signing secret or invite hash is mishandled] -> Keep both in root-owned `0600` runtime configuration, never print them, and scan changed files for accidental secret material.
- [Access-page split adds build surface] -> Keep the anonymous page minimal and exact-path, while all business SPA assets remain protected.
- [Shared cookie state across concurrent Web Edge replicas is not modeled] -> Keep the first release single-CVM/single-Web-Edge; a future identity/session store can replace the mechanism without changing browser API paths.

## Migration Plan

1. Implement API verification/session primitives and tests without enabling the public profile.
2. Add the dedicated access entrypoint, Web Edge `auth_request` rules, rate limiting, sanitized logging, and route-level tests.
3. Add root-controlled ACME issuance/renewal assets and update release/smoke checks; keep the existing Web Edge disabled by default.
4. Run local quality gates and container tests; publish immutable images through the existing local release path with Web Edge still disabled.
5. On the CVM, confirm domain/hosting prerequisites, enter the invite hash and session secret through the root-only runtime procedure, and configure the ACME client without printing secrets.
6. Manually add the DNSPod `A` record for `mrolescope.com` to `118.89.68.77`; do not add `www` or `AAAA`. Open only TCP 80/443 after the prerequisite gate is approved.
7. Complete the initial standalone ACME issuance, install the certificate, enable the Web Edge, and run the public smoke/visual checks.
8. Send the first batch of invitations only after all checks pass. Observe real usage through sanitized operational signals and expand in batches.

To roll back, stop/disable Web Edge or remove the DNS record, and restore the previous immutable Edge release through the controlled deployment helper. Leave API, worker, database, and historical data untouched.
