# Spec Delta

## Purpose

Provide a single shared-invite entry flow for the public JD browser, allowing invited viewers to unlock every existing browser feature for three days while keeping business assets and data unavailable before verification.

## ADDED Requirements

### Requirement: Shared invite entry flow
The public entrypoint SHALL provide an invite-code page and a same-origin verification endpoint. A successful verification SHALL unlock all existing browser features without another user-facing permission step.

#### Scenario: First visit without a session
- **WHEN** a visitor opens `https://mrolescope.com/` without a valid access session
- **THEN** the visitor sees only the invite-code entry flow and no JD data or business application assets

#### Scenario: Correct invite code
- **WHEN** a visitor submits the configured shared invite code to the verification endpoint
- **THEN** the service creates a valid access session and the visitor can open job filtering, job detail, job history, and trend dashboard features

#### Scenario: Incorrect invite code
- **WHEN** a visitor submits an incorrect invite code
- **THEN** the service denies verification, does not create a session, and returns the visitor to the invite flow with a non-sensitive failure message

### Requirement: Three-day access session
The access session SHALL remain valid for three days from successful verification and SHALL be sent as a browser-managed secure cookie. A missing, expired, malformed, or revoked session MUST be treated as unauthenticated.

#### Scenario: Reopen within three days
- **WHEN** an authorized visitor revisits the approved domain within three days without clearing browser cookies
- **THEN** the visitor enters the complete browser without re-entering the invite code

#### Scenario: Session expires
- **WHEN** a visitor requests a business page or API after the three-day session lifetime
- **THEN** the service denies the business request and shows the invite flow instead of returning JD data

### Requirement: Business asset and API enforcement
The access boundary SHALL protect the complete business SPA, its business static assets, and every business `/api/` response. Only the invite flow and its verification request may be accessed anonymously.

#### Scenario: Direct API request without access
- **WHEN** an unauthenticated client requests a business endpoint such as `/api/v1/jobs`
- **THEN** the request is denied without returning job data

#### Scenario: Direct business asset request without access
- **WHEN** an unauthenticated client requests the business SPA or one of its business assets
- **THEN** the request is denied or redirected to the invite flow without returning the business asset

#### Scenario: Authorized same-origin request
- **WHEN** a client presents a valid access session and requests an existing same-origin business API
- **THEN** the existing API path and response contract remain available without permissive cross-origin access

### Requirement: Invite-code protection and rotation
The configured shared invite code SHALL be represented at runtime only by an irreversible password hash. Replacing that hash SHALL invalidate existing access sessions and require visitors to verify with the replacement code.

#### Scenario: Runtime secret handling
- **WHEN** the service starts with the invite configuration
- **THEN** the plaintext invite code is not present in source control, image layers, browser URLs, application responses, or ordinary request logs

#### Scenario: Invite code replacement
- **WHEN** the operator replaces the configured invite-code hash after suspected disclosure
- **THEN** sessions issued under the previous configuration are rejected and the replacement code is required

### Requirement: Session and verification defenses
The access session SHALL use `Secure`, `HttpOnly`, and an appropriate `SameSite` attribute, and verification failures SHALL be rate-limited. The system SHALL NOT collect viewer accounts, roles, invite attribution, or business browsing profiles.

#### Scenario: Failed verification burst
- **WHEN** one source repeatedly submits invalid invite codes
- **THEN** verification requests are rate-limited without revealing the configured code or returning business data

#### Scenario: Cookie inspection
- **WHEN** a browser receives a successful verification response
- **THEN** the access cookie cannot be read by page JavaScript, is sent only over HTTPS, and is scoped to the approved site
