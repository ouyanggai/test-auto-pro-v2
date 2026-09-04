-- F-015 成功断言按用户决定移除：后续用断点功能表达"跑到哪里算成功"，断言这层多余。
-- 表刚建立且没有生产数据，直接删除，不保留兼容层，也不留下无人写入的空表。
DROP TABLE IF EXISTS test_path_success_assertions;
