-- F-012 目标表单数据必须按目标返回的字面量原样保存。
-- MySQL 原生 JSON 列会把数字重新解析成自己的数值表示：12.30 变成 12.3、0.10 变成 0.1、
-- 1.28E2 变成 128.0、1234567890123456789012 变成 1.2345678901234568e21。
-- 目标 Java 的 JudgeEnum.eq 用 BigDecimal.equals 同时比较数值与小数位，条件分支因此会改变；
-- 复制的 form-runtime 回显的金额和长数字也会与目标实例不一致。
-- 因此这三张表的载荷列改为 LONGTEXT，按原始 JSON 文本存取；合法性由仓储写入前后的 json.Valid 保证。

ALTER TABLE test_history_data_snapshots
  MODIFY instance_summary LONGTEXT NOT NULL,
  MODIFY template_summary LONGTEXT NOT NULL,
  MODIFY raw_form_data LONGTEXT NOT NULL;

ALTER TABLE test_history_replay_items
  MODIFY issue LONGTEXT NOT NULL,
  MODIFY branch_patches LONGTEXT NOT NULL,
  MODIFY effective_form_data LONGTEXT NOT NULL;

ALTER TABLE test_execution_path_configs
  MODIFY person_strategies LONGTEXT NOT NULL,
  MODIFY user_actions LONGTEXT NOT NULL,
  MODIFY compiled_steps LONGTEXT NOT NULL,
  MODIFY confirmed_node_keys LONGTEXT NOT NULL,
  MODIFY effective_form_data LONGTEXT NOT NULL,
  MODIFY branch_patches LONGTEXT NOT NULL,
  MODIFY runtime_validation LONGTEXT NOT NULL,
  MODIFY issues LONGTEXT NOT NULL,
  MODIFY latest_idempotency_result LONGTEXT NOT NULL;
