UPDATE test_plans SET status = 'not_started' WHERE status IN ('pending_configuration', 'ready');
