-- A membership remains an immutable source snapshot. This view exposes the
-- newest selected deterministic value for each dimension of the current
-- inventory, always preferring a result tied to the observed source version
-- over a same-job fallback. AI semantic tables are intentionally absent.

CREATE VIEW serving_jd_current_dimension_resolutions AS
SELECT DISTINCT ON (resolution.serving_jd_inventory_membership_id, resolution.dimension)
       resolution.*
FROM serving_jd_dimension_resolutions AS resolution
JOIN serving_jd_current_inventory_memberships AS membership
  ON membership.id = resolution.serving_jd_inventory_membership_id
ORDER BY resolution.serving_jd_inventory_membership_id,
         resolution.dimension,
         resolution.selection_priority,
         resolution.created_at DESC,
         resolution.id DESC;

COMMENT ON VIEW serving_jd_current_dimension_resolutions IS
    'Latest current-inventory deterministic result per dimension; priority 0 is the observed source version and priority 1 is same-job fallback.';
