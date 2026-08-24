ALTER TABLE test_template_rule_catalog
  ADD COLUMN stale TINYINT(1) NOT NULL DEFAULT 0 AFTER status,
  ADD KEY idx_template_rule_catalog_stale (stale);
