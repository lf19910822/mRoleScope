-- These queries expose only normalized, redacted AI-analysis facts. They
-- intentionally never select JD body text, model prompts/responses, spans'
-- source text, or credentials.

-- name: GetAIAnalysisConfiguration :one
SELECT *
FROM ai_analysis_configurations
WHERE id = $1;

-- name: GetAISemanticResult :one
SELECT *
FROM ai_semantic_results
WHERE body_content_version_id = $1
  AND ai_analysis_configuration_id = $2;

-- name: GetAIAnalysisWorkItem :one
SELECT *
FROM ai_analysis_work_items
WHERE id = $1;

-- name: ListPendingAIAnalysisWorkItems :many
SELECT *
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1
  AND status = 'pending'
ORDER BY queued_at, id;

-- name: GetAIAnalysisRun :one
SELECT *
FROM ai_analysis_runs
WHERE id = $1;

-- name: GetAIAnalysisAttempt :one
SELECT *
FROM ai_analysis_attempts
WHERE id = $1;

-- name: GetAIAnalysisUsageRecord :one
SELECT *
FROM ai_analysis_usage_records
WHERE attempt_id = $1;

-- name: ListAISemanticArtifactsForResult :many
SELECT *
FROM ai_semantic_artifacts
WHERE ai_semantic_result_id = $1
ORDER BY created_at, id;

-- name: ListAISemanticEvidenceForArtifact :many
SELECT *
FROM ai_semantic_evidence
WHERE semantic_artifact_id = $1
ORDER BY source_field, codepoint_start, codepoint_end, id;

-- name: GetAIExperienceSummary :one
SELECT *
FROM ai_experience_summaries
WHERE id = $1;

-- name: ListAIExperienceRequirementItems :many
SELECT *
FROM ai_experience_requirement_items
WHERE experience_summary_id = $1
ORDER BY id;

-- name: GetAIMinimumEducationRequirement :one
SELECT *
FROM ai_minimum_education_requirements
WHERE id = $1;

-- name: ListAISemanticConditionBranches :many
SELECT *
FROM ai_semantic_condition_branches
WHERE experience_summary_id = $1
ORDER BY branch_order, id;

-- name: GetAITechnologyRequirementGroup :one
SELECT *
FROM ai_technology_requirement_groups
WHERE id = $1;

-- name: ListAITechnologyRequirementOptions :many
SELECT *
FROM ai_technology_requirement_options
WHERE requirement_group_id = $1
ORDER BY option_order, id;

-- name: GetAICandidateTechnologyMention :one
SELECT *
FROM ai_candidate_technology_mentions
WHERE id = $1;

-- name: ListAISemanticReviewsForResult :many
SELECT *
FROM ai_semantic_reviews
WHERE ai_semantic_result_id = $1
ORDER BY reviewed_at, id;

-- name: GetAIAnalysisReleaseAttestation :one
SELECT *
FROM ai_analysis_release_attestations
WHERE ai_analysis_configuration_id = $1;

-- name: ListAISafeAuditEvents :many
SELECT *
FROM ai_analysis_audit_events
WHERE ai_analysis_configuration_id = $1
ORDER BY occurred_at, id;
