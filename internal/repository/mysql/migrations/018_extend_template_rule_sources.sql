ALTER TABLE test_template_rule_catalog
  ADD COLUMN target_digest CHAR(64) NOT NULL DEFAULT '' AFTER source_version,
  ADD COLUMN formmaking_digest CHAR(64) NOT NULL DEFAULT '' AFTER target_digest,
  ADD COLUMN vue_source_digest CHAR(64) NOT NULL DEFAULT '' AFTER formmaking_digest,
  ADD COLUMN java_source_digest CHAR(64) NOT NULL DEFAULT '' AFTER vue_source_digest,
  ADD COLUMN component_digest CHAR(64) NOT NULL DEFAULT '' AFTER java_source_digest;
