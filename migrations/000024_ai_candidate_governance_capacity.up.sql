-- Dictionary-rule state describes whether a proposed rule may match. Candidate
-- governance is a separate lifecycle: it records whether a term still needs a
-- human decision and supplies the global bounded review queue used by AI.

ALTER TABLE technology_dictionary_candidates
    ADD COLUMN governance_status TEXT NOT NULL DEFAULT 'pending_review'
    CHECK (governance_status IN ('pending_review', 'approved', 'rejected'));

CREATE INDEX technology_dictionary_candidates_pending_term_idx
    ON technology_dictionary_candidates (lower(candidate_text))
    WHERE governance_status = 'pending_review';
