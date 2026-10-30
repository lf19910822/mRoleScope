# Tasks

## 1. V3 contract regressions

- [x] 1.1 Add failing title-only parser regressions for the approved AI algorithm, AI application, platform-boundary, and retained-unclassified titles; verify `go test ./internal/jdanalysis -run TestParseJobFamilyTitleClassifiesReviewedDictionaryV3Titles` fails against V2.
- [x] 1.2 Add regression assertions that preserve separately owned backend/platform evidence and prove bare AI-related terms do not create either new family; verify the focused parser test fails against V2.

## 2. V3 dictionary implementation

- [x] 2.1 Add the two approved family-key constants and extend dictionary validation to require all fifteen approved families; verify the focused parser suite compiles and passes with V3.
- [x] 2.2 Add immutable `job_family_dictionary_v3.json`, migrate and add only approved aliases, retain V1/V2, and switch the embedded current asset to V3; verify the focused parser suite proves primary-family and cross-family outcomes.

## 3. Dictionary governance documentation

- [x] 3.1 Publish `Docs/开发指导/JD建模/岗位族词典_latest.md` with the complete reviewed V3 dictionary, V2 migrations, excluded bare terms, and matching/tie-break contract; verify every JSON V3 alias and family key is represented exactly once in the document.
- [x] 3.2 Update the job-family workflow plan with the two V3 families and their approved literal boundaries; verify the document retains V1/V2 history and does not describe a body- or AI-based classifier.

## 4. Integrated verification and delivery

- [x] 4.1 Run focused parser tests, `make test`, `sqlc generate`, `git diff --exit-code -- internal/database/sqlc`, strict OpenSpec validation, and `git diff --check`; verify all commands succeed.
- [ ] 4.2 Commit the reviewed change, merge it through the protected GitHub main workflow, and deploy it through the remote-development deployment workflow; verify the deployed image reports V3 without invoking an external model.
- [ ] 4.3 Run the provider-free pending analysis for eligible historical titles and record read-only V3 quality aggregates; verify processing failures do not block unrelated results and no Qwen/Flash command is invoked.

## Workflow follow-up

- Archive this change after deployed V3 results and their quality audit have been reviewed.
