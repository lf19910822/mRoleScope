-- A one-token Qwen input charge can be below 0.000001 RMB. Keep nine decimal
-- places in the redacted usage projection so it round-trips the canonical
-- integer nano-RMB ledger instead of inflating a hard-budget total.

ALTER TABLE ai_analysis_usage_records
    ALTER COLUMN estimated_charge_rmb TYPE NUMERIC(18, 9),
    ALTER COLUMN actual_charge_rmb TYPE NUMERIC(18, 9);
