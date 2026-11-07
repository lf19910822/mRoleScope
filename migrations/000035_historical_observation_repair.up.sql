-- Older installations already applied migration 000002 before the repair
-- trigger was added to its source file. Add the enum label in its own
-- migration so the next migration can safely use it after this transaction
-- commits.
ALTER TYPE collection_trigger_kind ADD VALUE IF NOT EXISTS 'repair';
