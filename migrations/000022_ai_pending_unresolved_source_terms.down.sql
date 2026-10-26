DROP TABLE IF EXISTS ai_pending_unresolved_source_terms;

-- PostgreSQL does not support removing one enum value safely. This migration
-- is intentionally irreversible; rolling back code keeps the additive value.
SELECT 1;
