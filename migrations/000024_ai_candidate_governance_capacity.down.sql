DROP INDEX IF EXISTS technology_dictionary_candidates_pending_term_idx;

ALTER TABLE technology_dictionary_candidates
    DROP COLUMN IF EXISTS governance_status;
