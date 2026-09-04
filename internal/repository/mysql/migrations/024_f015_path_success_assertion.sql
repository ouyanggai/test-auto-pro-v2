-- F-015 成功断言：每条执行路径配一个可判定的成功断言，运行结果由目标事实与它比对得出。
--
-- 为什么独立成表而不并入 test_execution_path_configs：
-- 后者的 revision 已经承载 F-012 的节点、数据、动作三条修订与幂等结果，回放任务会按它做检查点复验。
-- 断言是用户对"这条路径跑成什么样才算成功"的独立表达，改断言不应该让回放检查点失效，
-- 反之改配置也不应该让断言修订跳变，因此两者用各自的 revision，解耦并发语义。
--
-- 一条路径只允许一个断言（path_id 作主键），这是纲领第 7.4 节的硬约束：不做多断言、不做表达式。

CREATE TABLE test_path_success_assertions (
  path_id BIGINT UNSIGNED NOT NULL,
  -- end_node_key 是工具侧语义节点键，取值只能来自该路径真实线路上类型为结束的节点。
  end_node_key VARCHAR(255) NOT NULL,
  -- end_node_name 只用于目标不可读时的界面回显，绝不参与判定。
  -- 判定一律重新从真实流程结构取候选并复验 end_node_key，避免出现第二个事实来源。
  end_node_name VARCHAR(255) NOT NULL DEFAULT '',
  -- expected_status 只接受目标平台真实存在的实例状态取值，不自造枚举、不合并同义状态。
  expected_status VARCHAR(32) NOT NULL,
  -- arrival_ordinal 是第几次到达该结束节点；该节点在本路径上只出现一次时固定为 1。
  arrival_ordinal INT UNSIGNED NOT NULL DEFAULT 1,
  -- revision 是断言自己的修订号，与路径配置修订互不影响。
  revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
  -- idempotency_key 让同一次保存请求重复到达时不产生第二次修订。
  idempotency_key CHAR(36) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (path_id),
  KEY idx_test_path_success_assertions_updated (updated_at),
  CONSTRAINT fk_test_path_success_assertions_path FOREIGN KEY (path_id) REFERENCES test_execution_paths (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
