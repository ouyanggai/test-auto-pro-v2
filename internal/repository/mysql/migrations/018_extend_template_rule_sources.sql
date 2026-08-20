ALTER TABLE test_template_rule_catalog
  ADD COLUMN target_digest CHAR(64) NOT NULL DEFAULT '' AFTER source_version,
  ADD COLUMN formmaking_digest CHAR(64) NOT NULL DEFAULT '' AFTER target_digest,
  ADD COLUMN vue_source_digest CHAR(64) NOT NULL DEFAULT '' AFTER formmaking_digest,
  ADD COLUMN java_source_digest CHAR(64) NOT NULL DEFAULT '' AFTER vue_source_digest,
  ADD COLUMN component_digest CHAR(64) NOT NULL DEFAULT '' AFTER java_source_digest;

ALTER TABLE test_template_rule_analysis_jobs
  ADD COLUMN listed_count INT NOT NULL DEFAULT 0 AFTER total_count,
  ADD COLUMN pagination_complete TINYINT(1) NOT NULL DEFAULT 0 AFTER failed_count;
