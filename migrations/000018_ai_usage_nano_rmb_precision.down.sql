-- Do not silently round durable usage when rolling back the display precision.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ai_analysis_usage_records
        WHERE estimated_charge_rmb <> trunc(estimated_charge_rmb, 6)
           OR actual_charge_rmb <> trunc(actual_charge_rmb, 6)
    ) THEN
        RAISE EXCEPTION 'cannot roll back 000018 while nano-RMB usage exists';
    END IF;
END
$$;

ALTER TABLE ai_analysis_usage_records
    ALTER COLUMN estimated_charge_rmb TYPE NUMERIC(14, 6),
    ALTER COLUMN actual_charge_rmb TYPE NUMERIC(14, 6);
