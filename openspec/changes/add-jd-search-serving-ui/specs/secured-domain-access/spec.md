# Spec Delta

## Purpose

Provide a domain-bound HTTPS entrypoint for the JD browser that authenticates
viewers before it serves static pages or data, while keeping application and
database services off the public network.

## ADDED Requirements

### Requirement: Domain-registration deployment prerequisite

The deployment plan SHALL require an approved domain before it enables a
public browser entrypoint. The documented prerequisites MUST include registrar
account hardening, transfer protection, DNS ownership, and any hosting filing
required by the selected jurisdiction or provider. The system MUST NOT collect
registrar, payment, or identity-verification credentials.

#### Scenario: Domain is not ready

- **WHEN** no approved domain and prerequisite confirmation are available
- **THEN** deployment does not enable a public JD-browser listener

#### Scenario: Manual domain work is completed

- **WHEN** an operator records that domain and hosting prerequisites are
  complete through the approved operational process
- **THEN** the secure browser deployment can be configured without storing the
  operator's registrar or payment credentials in the repository or CI

### Requirement: HTTPS and authenticated browser entrypoint

The deployment SHALL expose the JD browser only through its approved domain
over HTTPS. It MUST require an access credential before serving browser assets
or `/api/` responses, and MUST NOT treat direct raw-IP HTTP as a supported
product entrypoint.

#### Scenario: Unauthenticated request

- **WHEN** a visitor requests the browser root or an `/api/` endpoint without
  valid access credentials
- **THEN** the entrypoint denies the request without returning JD data or
  browser assets

#### Scenario: Authenticated HTTPS request

- **WHEN** an authorized visitor accesses the approved domain over HTTPS
- **THEN** the entrypoint serves browser assets and permits same-origin `/api/`
  requests

### Requirement: Same-origin static and API delivery

The secure entrypoint SHALL serve the browser assets at its root path and
proxy REST API requests under `/api/` to the application API. It MUST keep the
application API and database unavailable on public host interfaces and MUST
not require permissive CORS configuration.

#### Scenario: Browser loads data

- **WHEN** an authorized browser loads the JD list
- **THEN** it fetches data through the same origin's `/api/` path

#### Scenario: Public port inspection

- **WHEN** the secure deployment is active
- **THEN** only the authenticated HTTPS entrypoint is publicly reachable and
  the application API listener remains bound to a non-public interface

### Requirement: Deferred access-control evolution

The first secure release SHALL use an access-control mechanism whose
credentials are stored only in root-owned runtime configuration. The deployment
MUST permit a later replacement with an identity-provider-based access layer
without changing browser API paths or exposing the application API.

#### Scenario: Access mechanism is replaced later

- **WHEN** an operator replaces the initial access-control layer with an
  approved identity provider
- **THEN** the browser continues to use the same origin and `/api/` paths
