CREATE TABLE IF NOT EXISTS test_execution_path_choices (
  path_id BIGINT UNSIGNED NOT NULL,
  route_node_id VARCHAR(100) NOT NULL,
  branch_id VARCHAR(100) NOT NULL,
  PRIMARY KEY (path_id, route_node_id),
  CONSTRAINT fk_test_execution_path_choices_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
