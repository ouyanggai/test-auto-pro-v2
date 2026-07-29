CREATE TABLE IF NOT EXISTS test_execution_path_batch_items (
  batch_id BIGINT UNSIGNED NOT NULL,
  item_order INT UNSIGNED NOT NULL,
  path_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (batch_id, item_order),
  UNIQUE KEY uk_execution_path_batch_path (batch_id, path_id),
  CONSTRAINT fk_execution_path_batch_items_batch FOREIGN KEY (batch_id) REFERENCES test_execution_path_batches (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_path_batch_items_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
