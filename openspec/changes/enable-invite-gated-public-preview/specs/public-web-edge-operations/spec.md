# Spec Delta

## Purpose

Provide a controlled HTTPS Web Edge lifecycle for the approved mRoleScope domain, including certificate issuance, renewal, public-port boundaries, operational observation, and safe rollback without exposing the API or database directly.

## ADDED Requirements

### Requirement: Approved-domain-only entrypoint
The public browser SHALL be served only at `https://mrolescope.com/`. The deployment MUST reject the raw IP, unapproved Host values, and `www.mrolescope.com`; application API and PostgreSQL ports MUST remain unavailable on public host interfaces.

#### Scenario: Canonical domain request
- **WHEN** a visitor requests the canonical domain over HTTPS
- **THEN** the request reaches the authenticated Web Edge and no alternate public application entrypoint is required

#### Scenario: Alternate host request
- **WHEN** a client requests the raw CVM IP or an unapproved Host value
- **THEN** the request is rejected without serving the browser or API

#### Scenario: Public port inspection
- **WHEN** the public preview is active
- **THEN** only Web Edge ports `80` and `443` are exposed, while API port `8080` and PostgreSQL port `5432` remain private

### Requirement: Trusted HTTPS certificate lifecycle
The approved domain SHALL use a trusted Let's Encrypt DV certificate obtained through HTTP-01 validation. The deployment MUST renew it automatically before expiry, reload the Web Edge after successful replacement, and retain the last usable certificate when renewal fails.

#### Scenario: Initial certificate issuance
- **WHEN** DNS points the approved domain to the CVM and the prelaunch ACME procedure runs
- **THEN** HTTP-01 validation completes before the Web Edge serves the public business application

#### Scenario: Successful renewal
- **WHEN** the root-controlled renewal task obtains a replacement certificate
- **THEN** the certificate is installed with root-only permissions and Web Edge reloads without changing application API paths

#### Scenario: Renewal failure
- **WHEN** a renewal attempt fails or the new certificate fails validation
- **THEN** the previous valid certificate remains in use, the failure is observable to the operator, and the deployment does not delete the working TLS material

### Requirement: Controlled public release gate
The Web Edge SHALL remain disabled until the approved domain, TLS material, runtime access configuration, hosting prerequisites, and release approval are recorded as complete. Public release MUST use the repository's reviewed immutable-image deployment path.

#### Scenario: Gate incomplete
- **WHEN** any required domain, certificate, runtime-secret, or approval prerequisite is incomplete
- **THEN** deployment keeps the Web Edge stopped and does not open the public browser listener

#### Scenario: Gate complete
- **WHEN** all prerequisites are complete and a reviewed release is selected
- **THEN** deployment starts the Web Edge only after API readiness and runs the HTTPS access, authentication, same-origin API, and raw-IP rejection checks

### Requirement: Minimal operational observation
The Web Edge SHALL expose enough operational signals to inspect request status, latency, errors, and rate-limit events for real-experience validation. It MUST NOT log invite codes, access cookies, authorization values, request bodies, JD bodies, viewer identities, or browsing profiles.

#### Scenario: Performance observation
- **WHEN** invited viewers use the public browser
- **THEN** the operator can inspect aggregate request outcomes and latency without identifying which viewer accessed which job

#### Scenario: Sensitive request data
- **WHEN** an invite verification or business request is handled
- **THEN** operational logs omit the invite code, session cookie, authorization value, request body, and JD response body

### Requirement: Reversible public rollback
The operator SHALL be able to withdraw the public entrypoint without modifying application data or exposing the private API. Rollback MUST support stopping Web Edge, removing the DNS route, or reverting to the last verified immutable Web Edge release.

#### Scenario: Edge release failure
- **WHEN** the new Web Edge fails its readiness or public smoke checks
- **THEN** the deployment stops or reverts only the Web Edge and reports failure without rolling back unrelated database or worker state

#### Scenario: Emergency withdrawal
- **WHEN** the operator detects a serious public failure or invite-code disclosure
- **THEN** the operator can stop the Web Edge or replace the invite configuration, while the database and internal processing remain private and intact
