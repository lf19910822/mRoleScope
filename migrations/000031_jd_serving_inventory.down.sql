DROP VIEW IF EXISTS serving_jd_current_inventory_memberships;

DROP TRIGGER IF EXISTS serving_jd_reconciliation_audits_immutable ON serving_jd_reconciliation_audits;
DROP TRIGGER IF EXISTS serving_jd_dimension_resolutions_immutable ON serving_jd_dimension_resolutions;
DROP TRIGGER IF EXISTS serving_jd_inventory_memberships_immutable ON serving_jd_inventory_memberships;
DROP FUNCTION IF EXISTS reject_serving_jd_inventory_mutation();

DROP TABLE IF EXISTS serving_jd_reconciliation_audits;
DROP TABLE IF EXISTS serving_jd_dimension_resolutions;
DROP TABLE IF EXISTS serving_jd_inventory_memberships;

DROP TYPE IF EXISTS serving_jd_formal_dimension;
DROP TYPE IF EXISTS serving_jd_activity_state;
