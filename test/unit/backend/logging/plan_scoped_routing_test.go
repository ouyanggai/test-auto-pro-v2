package logging_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/logging"
)

// writeBusinessLogs 用给定作用域写一条普通日志与一次目标请求日志，返回该作用域实际落盘的目录。
func writeBusinessLogs(t *testing.T, root string, scope logging.Scope) string {
	t.Helper()
	router := logging.NewRouter(root, fixedTime)
	logger := logging.NewLogger(router, fixedTime)
	logger.Info(scope, "读取路径配置")
	logger.Network(scope, logging.NetworkRecord{
		Method: "POST", Endpoint: "/web/flowProxy/findById", StatusCode: 200, Result: "success",
		Curl: "curl -sS -X POST 'https://target/web/flowProxy/findById'", ResponseBody: `{"isSuccess":true}`,
	})
	return router.BucketDir(scope)
}

// TestConfigurationLogsLandUnderPlanAndPath 验证配置阶段日志按计划与执行路径归档：
// 落点是 logs/plans/<计划名+ID>/configuration/<路径名+ID>/<日期>/，不再有共享日期目录，
// 也不重复写进应用程序日志，且每行都带上业务归属字段。
func TestConfigurationLogsLandUnderPlanAndPath(t *testing.T) {
	root := t.TempDir()
	scope := businessScope("req-config")
	dir := writeBusinessLogs(t, root, scope)
	if dir != businessBucketDir(root) {
		t.Fatalf("配置日志目录不正确：\n实际 %s\n期望 %s", dir, businessBucketDir(root))
	}
	for _, name := range []string{"operation.log", "network.log", "curl.log", logging.MetaFileName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s 没有落盘：%v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "config")); err == nil {
		t.Fatal("不应再出现所有计划共用的 logs/config 目录")
	}
	if content := readApplicationFile(root, "application.log"); strings.Contains(content, "读取路径配置") {
		t.Fatalf("业务日志重复写进了应用程序日志：%s", content)
	}
	operation := readBucketFile(t, root, "operation.log")
	for _, expected := range []string{
		"plan_id=7", "plan_name=员工请假单（集团）-自动回归",
		"execution_path_id=13", "execution_path_name=执行路径_1", "request_id=req-config",
	} {
		if !strings.Contains(operation, expected) {
			t.Fatalf("业务日志缺少归属字段 %s：%s", expected, operation)
		}
	}
}

// TestSameDisplayNameDifferentIDNeverShareDirectory 验证同名计划、同名执行路径不会因为显示名相同而串目录，
// 目录名必须是中文显示名在前、不可变 ID 在后。
func TestSameDisplayNameDifferentIDNeverShareDirectory(t *testing.T) {
	root := t.TempDir()
	base := logging.Scope{
		RequestID: "req-1", PlanID: "7", PlanName: "同名计划",
		ExecutionPathID: "13", ExecutionPathName: "执行路径 1",
	}
	otherPlan := base
	otherPlan.PlanID, otherPlan.RequestID = "8", "req-2"
	otherPath := base
	otherPath.ExecutionPathID, otherPath.RequestID = "14", "req-3"
	dirs := map[string]bool{}
	for _, scope := range []logging.Scope{base, otherPlan, otherPath} {
		dirs[writeBusinessLogs(t, root, scope)] = true
	}
	if len(dirs) != 3 {
		t.Fatalf("同名不同 ID 共用了日志目录：%v", dirs)
	}
	for _, expected := range []string{
		filepath.Join("同名计划__plan-7", "configuration", "执行路径 1__path-13"),
		filepath.Join("同名计划__plan-8", "configuration", "执行路径 1__path-13"),
		filepath.Join("同名计划__plan-7", "configuration", "执行路径 1__path-14"),
	} {
		if _, err := os.Stat(filepath.Join(root, "plans", expected, fixedTime().Format("2006-01-02"), "operation.log")); err != nil {
			t.Fatalf("目录 %s 下没有日志：%v", expected, err)
		}
	}
}

// TestPlanLevelAndApplicationRouting 验证只知道计划的操作进计划级目录，
// 确实无法归属业务对象的日志才进应用程序目录。
func TestPlanLevelAndApplicationRouting(t *testing.T) {
	root := t.TempDir()
	planLevel := writeBusinessLogs(t, root, logging.Scope{RequestID: "req-plan", PlanID: "7", PlanName: "员工请假单（集团）"})
	expected := filepath.Join(root, "plans", "员工请假单（集团）__plan-7", "configuration", "_plan", fixedTime().Format("2006-01-02"))
	if planLevel != expected {
		t.Fatalf("计划级操作没有进 _plan：\n实际 %s\n期望 %s", planLevel, expected)
	}
	if _, err := os.Stat(filepath.Join(planLevel, "operation.log")); err != nil {
		t.Fatalf("计划级日志没有落盘：%v", err)
	}
	systemLevel := writeBusinessLogs(t, root, logging.Scope{RequestID: "req-system"})
	if systemLevel != filepath.Join(root, "application", fixedTime().Format("2006-01-02")) {
		t.Fatalf("无业务归属的日志没有进应用程序目录：%s", systemLevel)
	}
	if _, err := os.Stat(filepath.Join(systemLevel, "application.log")); err != nil {
		t.Fatalf("应用程序日志没有落盘：%v", err)
	}
	if _, err := os.Stat(filepath.Join(systemLevel, logging.MetaFileName)); err == nil {
		t.Fatal("应用程序目录没有业务归属，不应写 meta.json")
	}
}

// TestRunScopeRoutesToPlanRunsAndWritesExecutionLog 验证真实运行作用域进 logs/runs 的页面层级，
// 配置阶段的 operation.log 与执行阶段的 execution.log 不混用。
func TestRunScopeRoutesToPlanRunsAndWritesExecutionLog(t *testing.T) {
	root := t.TempDir()
	scope := businessScope("req-run")
	scope.RunID, scope.RunSeq, scope.PathRunID = "501", "3", "88"
	dir := writeBusinessLogs(t, root, scope)
	expected := filepath.Join(root, "runs", "运行_3__员工请假单（集团）-自动回归__run-501",
		"paths", "执行路径 1__path-run-88__path-13")
	if dir != expected {
		t.Fatalf("运行日志目录不正确：\n实际 %s\n期望 %s", dir, expected)
	}
	for _, name := range []string{"execution.log", "network.log", "curl.log", logging.MetaFileName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("运行目录缺少 %s：%v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "operation.log")); err == nil {
		t.Fatal("执行阶段不应写 operation.log")
	}
}

// TestMetaJSONMatchesDirectory 验证配置目录与运行目录的 meta.json 与目录名指向同一个计划和执行路径。
func TestMetaJSONMatchesDirectory(t *testing.T) {
	root := t.TempDir()
	configurationDir := writeBusinessLogs(t, root, businessScope("req-meta"))
	configuration := readMeta(t, configurationDir)
	if configuration["planId"] != "7" || configuration["executionPathId"] != "13" {
		t.Fatalf("配置目录 meta.json 的 ID 与目录不一致：%v", configuration)
	}
	if !strings.Contains(configurationDir, configuration["planName"]) ||
		!strings.Contains(configurationDir, configuration["executionPathName"]) {
		t.Fatalf("配置目录 meta.json 的名称与目录不一致：%v", configuration)
	}
	if configuration["date"] != fixedTime().Format("2006-01-02") {
		t.Fatalf("配置目录 meta.json 缺少正确日期：%v", configuration)
	}
	runScope := businessScope("req-meta-run")
	runScope.RunID, runScope.RunSeq, runScope.PathRunID = "9", "2", "88"
	runDir := writeBusinessLogs(t, root, runScope)
	run := readMetaAny(t, runDir)
	if run["runId"] != "9" || strings.TrimSpace(anyString(run["startedAt"])) == "" {
		t.Fatalf("运行目录 meta.json 缺少运行 ID 或开始时间：%v", run)
	}
	// runNo / pathRunId / pathId / pathName 与页面每行一一对应，必须同时落进 meta.json。
	for key, expected := range map[string]string{
		"runNo": "2", "pathRunId": "88", "pathId": "13", "pathName": "执行路径 1", "planId": "7",
	} {
		if anyString(run[key]) != expected {
			t.Fatalf("运行目录 meta.json 的 %s 不正确：%v", key, run)
		}
	}
	// 实例名称在没有事实时如实标为不可用，不用计划名或路径名冒充。
	if run["instanceNameAvailable"] != false {
		t.Fatalf("没有实例名称时 instanceNameAvailable 应为 false：%v", run)
	}
	if strings.TrimSpace(anyString(run["instanceNameNote"])) == "" {
		t.Fatalf("实例名称不可用必须带原因：%v", run)
	}
	// 实例如实补进作用域后：名称只进元数据，目录不变。
	updated := runScope
	updated.InstanceID, updated.InstanceName = "instance-9", "请假单-路径1-2026-09-12 10:00:00"
	router := logging.NewRouter(root, fixedTime)
	router.UpdateRunInstanceMeta(updated)
	merged := readMetaAny(t, runDir)
	if merged["instanceId"] != "instance-9" || merged["instanceName"] != updated.InstanceName || merged["instanceNameAvailable"] != true {
		t.Fatalf("实例身份没有补进 meta.json：%v", merged)
	}
	if merged["startedAt"] != run["startedAt"] {
		t.Fatalf("补写实例身份改写了开始时间：%v", merged)
	}
	if dir := router.BucketDir(updated); dir != runDir {
		t.Fatalf("实例名称变化不得改变日志目录：%s", dir)
	}
}

// readMeta 读取并解析配置目录 meta.json 的字符串字段。
func readMeta(t *testing.T, dir string) map[string]string {
	t.Helper()
	parsed := map[string]string{}
	for key, value := range readMetaAny(t, dir) {
		parsed[key] = anyString(value)
	}
	return parsed
}

// readMetaAny 读取并解析一个业务日志目录里的 meta.json；字段类型原样保留，便于断言布尔字段。
func readMetaAny(t *testing.T, dir string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, logging.MetaFileName))
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", logging.MetaFileName, err)
	}
	parsed := map[string]any{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("meta.json 不是合法 JSON：%v", err)
	}
	return parsed
}

// anyString 把 meta.json 里的字段值收敛为字符串，缺值与布尔值都按原样表达。
func anyString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// TestConsecutiveRunsAndPathsKeepTheirOwnDirectories 锁定运行日志的页面层级映射：
// 同一计划连续运行 1、2、3 次各用各自 runId 的目录，同一运行内同名路径按 pathRunId 分开，
// 实例名称只进元数据字段、不改变目录；换一个 Router（服务重启）后按 runId/pathRunId 复用同一目录。
func TestConsecutiveRunsAndPathsKeepTheirOwnDirectories(t *testing.T) {
	root := t.TempDir()
	dirs := map[string]string{}
	for index, runID := range []string{"101", "102", "103"} {
		scope := businessScope("req-run-" + runID)
		scope.RunID, scope.RunSeq = runID, fmt.Sprint(index+1)
		scope.PathRunID = fmt.Sprint(880 + index)
		dirs[runID] = writeBusinessLogs(t, root, scope)
		expected := filepath.Join(root, "runs",
			fmt.Sprintf("运行_%d__员工请假单（集团）-自动回归__run-%s", index+1, runID),
			"paths", fmt.Sprintf("执行路径 1__path-run-%d__path-13", 880+index))
		if dirs[runID] != expected {
			t.Fatalf("第 %d 次运行的目录不正确：\n实际 %s\n期望 %s", index+1, dirs[runID], expected)
		}
	}
	if dirs["101"] == dirs["102"] || dirs["102"] == dirs["103"] || dirs["101"] == dirs["103"] {
		t.Fatalf("连续运行共用了目录：%v", dirs)
	}
	// 同一运行内两条同名路径按 pathRunId 分开，绝不互相覆盖。
	sameRun := businessScope("req-same-path")
	sameRun.RunID, sameRun.RunSeq, sameRun.PathRunID = "201", "7", "901"
	first := writeBusinessLogs(t, root, sameRun)
	sameRun.PathRunID, sameRun.RequestID = "902", "req-same-path-2"
	second := writeBusinessLogs(t, root, sameRun)
	if first == second {
		t.Fatalf("同名路径共用了目录：%s", first)
	}
	// 实例名称含中文、空格、斜杠与超长文本：只进元数据与日志字段，目录不受影响。
	named := businessScope("req-instance-name")
	named.RunID, named.RunSeq, named.PathRunID = "301", "9", "903"
	named.InstanceID = "instance-301"
	named.InstanceName = "请假单/审批 流程 " + strings.Repeat("超长名称", 40)
	namedDir := writeBusinessLogs(t, root, named)
	if strings.Contains(namedDir, "超长名称") || strings.Contains(namedDir, named.InstanceName) {
		t.Fatalf("实例名称参与了目录寻址：%s", namedDir)
	}
	meta := readMetaAny(t, namedDir)
	if meta["instanceName"] != named.InstanceName || meta["instanceNameAvailable"] != true {
		t.Fatalf("实例名称没有原样写进元数据：%v", meta)
	}
	// 服务重启：新 Router 按同一 runId/pathRunId 复用同一目录与既有元数据。
	restarted := logging.NewRouter(root, fixedTime)
	if dir := restarted.BucketDir(named); dir != namedDir {
		t.Fatalf("重启后目录不一致：\n实际 %s\n期望 %s", dir, namedDir)
	}
	if _, err := os.Stat(filepath.Join(namedDir, "execution.log")); err != nil {
		t.Fatalf("重启后复用的目录里找不到原有日志：%v", err)
	}
}
