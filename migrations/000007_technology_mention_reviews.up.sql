CREATE TYPE technology_mention_review_verdict AS ENUM ('correct', 'incorrect', 'uncertain');

CREATE TABLE technology_mention_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_result_id UUID NOT NULL REFERENCES technology_analysis_results(id),
    technology_mention_evidence_id UUID NOT NULL REFERENCES technology_mention_evidence(id),
    verdict technology_mention_review_verdict NOT NULL,
    error_category TEXT CHECK (error_category IS NULL OR char_length(error_category) BETWEEN 1 AND 128),
    reviewer TEXT NOT NULL CHECK (char_length(reviewer) BETWEEN 1 AND 256),
    reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT technology_mention_reviews_incorrect_has_category CHECK (
        (verdict = 'incorrect' AND error_category IS NOT NULL)
        OR (verdict <> 'incorrect' AND error_category IS NULL)
    )
);

CREATE INDEX technology_mention_reviews_result_time_idx
    ON technology_mention_reviews (analysis_result_id, reviewed_at DESC);
CREATE INDEX technology_mention_reviews_evidence_time_idx
    ON technology_mention_reviews (technology_mention_evidence_id, reviewed_at DESC);
