DROP TRIGGER IF EXISTS trend_formal_release_components_immutable ON trend_formal_release_components;
DROP FUNCTION IF EXISTS reject_trend_formal_release_component_mutation();
DROP TRIGGER IF EXISTS trend_formal_releases_immutable ON trend_formal_releases;
DROP FUNCTION IF EXISTS reject_trend_formal_release_mutation();
DROP TABLE IF EXISTS trend_formal_release_components;
DROP INDEX IF EXISTS trend_formal_releases_one_published_idx;
DROP TABLE IF EXISTS trend_formal_releases;
DROP TYPE IF EXISTS trend_formal_release_dimension;
DROP TYPE IF EXISTS trend_formal_release_status;
