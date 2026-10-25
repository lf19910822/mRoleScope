DROP TRIGGER IF EXISTS ai_analysis_release_attestations_immutable ON ai_analysis_release_attestations;
DROP FUNCTION IF EXISTS reject_ai_analysis_release_attestation_mutation();
DROP TRIGGER IF EXISTS ai_semantic_reviews_immutable ON ai_semantic_reviews;
DROP FUNCTION IF EXISTS reject_ai_semantic_review_mutation();
DROP TRIGGER IF EXISTS ai_analysis_publication_state_requires_published_qwen_plus ON ai_analysis_publication_state;
DROP FUNCTION IF EXISTS require_published_qwen_plus_ai_publication();

DROP TABLE IF EXISTS ai_analysis_release_attestations;
DROP TABLE IF EXISTS ai_analysis_publication_state;
DROP TABLE IF EXISTS ai_semantic_reviews;
DROP TABLE IF EXISTS ai_candidate_technology_mentions;
DROP TABLE IF EXISTS ai_technology_requirement_options;
DROP TABLE IF EXISTS ai_technology_requirement_groups;
DROP TABLE IF EXISTS ai_semantic_condition_branches;
DROP TABLE IF EXISTS ai_minimum_education_requirements;
DROP TABLE IF EXISTS ai_experience_requirement_items;
DROP TABLE IF EXISTS ai_experience_summaries;
DROP TABLE IF EXISTS ai_semantic_evidence;
DROP TABLE IF EXISTS ai_semantic_artifacts;

DROP TYPE IF EXISTS ai_semantic_review_error_category;
DROP TYPE IF EXISTS ai_semantic_review_verdict;
DROP TYPE IF EXISTS ai_proficiency_level;
DROP TYPE IF EXISTS ai_requirement_operator;
DROP TYPE IF EXISTS ai_requirement_necessity;
DROP TYPE IF EXISTS ai_requirement_role;
DROP TYPE IF EXISTS ai_education_level;
DROP TYPE IF EXISTS ai_semantic_review_tier;
DROP TYPE IF EXISTS ai_semantic_artifact_kind;
