CREATE TABLE IF NOT EXISTS add_sign_person_exclusion_list (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  person_id VARCHAR(100) NOT NULL,
  person_name VARCHAR(100) NOT NULL,
  person_phone VARCHAR(20) DEFAULT '',
  reason TEXT,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_add_sign_person_id (person_id),
  KEY idx_person_name (person_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
