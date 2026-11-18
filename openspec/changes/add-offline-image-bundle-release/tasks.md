# Tasks

## 1. Bundle contract and red tests

- [ ] 1.1 Define the versioned five-image manifest format and fixed archive names; add failing shell tests for missing images, unknown fields, path traversal, malformed revisions, and checksum mismatches.
- [ ] 1.2 Add the local bundle builder that saves the four reviewed application images plus the pinned PostgreSQL image, writes image IDs/architecture/revision labels, and emits a whole-bundle checksum; verify a synthetic bundle can be inspected without Docker Hub access.

## 2. Controlled CVM bundle activation

- [ ] 2.1 Parameterize only the pinned PostgreSQL Compose image with an unchanged default; verify normal Compose config output remains unchanged when offline mode is not selected.
- [ ] 2.2 Extend the root-controlled deployment boundary with an offline-bundle entry point that validates staging ownership, archive paths, manifest checksums, image IDs, architectures, and application revision labels before `docker load`; verify every invalid bundle stops before migration or service start.
- [ ] 2.3 Install the bundle entry point and its restricted sudo rule through the reviewed bootstrap; verify bootstrap asset, helper allowlist, and command-shape regression tests reject arbitrary paths, tags, commands, and namespaces.

## 3. Local orchestrator and transfer path

- [ ] 3.1 Add an explicit offline deployment mode that runs the existing quality gate, builds the bundle, stages it through SSH/scp, verifies the remote whole-file checksum, and invokes only the restricted bundle helper; verify the mocked release test performs no Docker Hub push or CVM pull.
- [ ] 3.2 Preserve the existing Docker Hub `--deploy` and `--dry-run` behavior; verify the release regression suite still requires four immutable Docker Hub digests for the default path and rejects dirty/non-main checkouts.
- [ ] 3.3 Add bounded cleanup and failure reporting for local and remote staging; verify interrupted transfers and failed activation do not advance `current.env` or leave unbounded bundle files.

## 4. Documentation and release verification

- [ ] 4.1 Update the autonomous release runbook with offline mode prerequisites, bundle invariants, transfer boundaries, rollback behavior, and the rule against manual `docker load`/Compose commands; verify every command is fixed-scope and contains no credentials.
- [ ] 4.2 Run focused shell tests, `make remote-release-verify`, strict OpenSpec validation, and `git diff --check`; record the exact checks and any environment-only limitations in this change.

## Workflow follow-up

- Merge the reviewed change into protected `main` before using offline deployment in the CVM.
- Perform one explicitly authorized offline deployment and verify the release record, API health, and Web Edge gate without exposing runtime secrets.
- Archive the OpenSpec change after implementation verification and deployment documentation are complete.
