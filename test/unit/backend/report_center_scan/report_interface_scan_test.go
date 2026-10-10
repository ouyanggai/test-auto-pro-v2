package report_center_scan_test

// F-042 本轮：报告中心接口耗时快速聚合的口径与性能性质。
// 这三个用例锁定本轮的三个硬性门禁：
//   - 只读 network.log 就能算（目录里没有 step.log，读到才算对）；
//   - 接口调用按路径运行全量计（含 step_id=- 的登录/结构读取行，旧口径会整行漏计）；
//   - 终态路径运行写缓存，第二次同一文件不再打开（L0→L1→L2 来回不重复读盘）。
// 运行：go test ./test/unit/backend/report_center_scan/
import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// writeNetworkLog 在 root/runs/run1/paths/<dirName>/ 下写一份 network.log，并返回该路径运行的
// step.log 相对路径（与库里 attempt.log_path 同形态：runs/<目录>/step.log）。
// dirName 是目录名，linePathRunID 是日志行里携带的 path_run_id，两者分开传以便构造同名目录场景。
func writeNetworkLog(t *testing.T, root, dirName, linePathRunID, body string) string {
	t.Helper()
	dir := filepath.Join(root, "runs", "run1", "paths", dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建目录失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "network.log"), []byte(body), 0o644); err != nil {
		t.Fatalf("写 network.log 失败：%v", err)
	}
	return filepath.ToSlash(filepath.Join("runs", "run1", "paths", dirName, "step.log"))
}

// writeNetworkLogInto 在已给定的目录里写一份 network.log（供服务层测试直接指定目录）。
func writeNetworkLogInto(t *testing.T, dir, linePathRunID, body string) error {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "network.log"), []byte(body), 0o644)
}

// networkLine 生成一行统一格式的 network.log 记录。
func networkLine(pathRunID, endpoint, class string, seconds string) string {
	classField := "request_class=" + class + " "
	if class == "" {
		classField = ""
	}
	return "time=2026-10-10_10:00:00 level=info path_run_id=" + pathRunID +
		" step_id=- attempt=- endpoint=" + endpoint + " " + classField +
		" status_code=200 duration_s=" + seconds + " result=success\n"
}

// TestReportInterfaceScanCountsEveryRequestOfPathRun 锁定本轮口径修正：
// step_id=-（不归属任何步骤尝试）的登录/结构读取行必须计入，读/写各自归对，
// 缺 duration_s 的请求计入次数但不产生耗时样本，别的路径运行的行不能混进来。
func TestReportInterfaceScanCountsEveryRequestOfPathRun(t *testing.T) {
	root := t.TempDir()
	logPath := writeNetworkLog(t, root, "773", "773", networkLine("773", "/api/web/flowTemplateApi/list", "", "0.619")+
		networkLine("773", "/api/web/flowJobTaskLink/list", "read", "0.100")+
		networkLine("773", "/api/web/flowInstanceApi/audit", "write", "1.500")+
		networkLine("773", "/api/web/flowInstanceApi/submit", "write", "-")+
		networkLine("999", "/api/web/flowInstanceApi/audit", "write", "9.999"))

	scanner := service.NewReportInterfaceScanner(root)
	stats, capped := scanner.Aggregate(t.Context(), []repository.ReportPathRunLogDir{
		{PathRunID: 773, Status: model.PathRunStatusCompleted, LogPath: logPath},
	})
	if capped {
		t.Fatalf("单条路径运行不应触达采样上限")
	}
	got := stats[773]
	// 旧口径会丢掉第一行（step_id=- 归属不到尝试），这里必须计 4 次：读 2 + 写 2。
	if got.ReadCount != 2 || got.WriteCount != 2 {
		t.Fatalf("读写次数应为 2/2，实际 读 %d 写 %d", got.ReadCount, got.WriteCount)
	}
	if !got.AnyKnown {
		t.Fatalf("存在真实耗时的请求，AnyKnown 必须为真")
	}
	if len(got.DurationsMs) != 3 {
		t.Fatalf("有真实耗时的样本应为 3 条，实际 %d", len(got.DurationsMs))
	}
	if len(got.WriteDurationsMs) != 1 || got.WriteDurationsMs[0] != 1500 {
		t.Fatalf("写请求耗时样本应为 [1500]，实际 %v", got.WriteDurationsMs)
	}
	if _, ok := stats[999]; ok {
		t.Fatalf("目录外的路径运行 999 不应出现在结果里")
	}
}

// TestReportInterfaceScanFallsBackToWriteEndpointWhitelist 锁定旧日志兜底：
// 缺 request_class 的日志行按写端点白名单归类，写请求不能被误计成读取。
func TestReportInterfaceScanFallsBackToWriteEndpointWhitelist(t *testing.T) {
	root := t.TempDir()
	logPath := writeNetworkLog(t, root, "5", "5", networkLine("5", "/api/web/flowInstanceApi/reSubmit", "", "0.300")+
		networkLine("5", "/api/web/flowInstanceApi/list", "", "0.200"))

	scanner := service.NewReportInterfaceScanner(root)
	stats, _ := scanner.Aggregate(t.Context(), []repository.ReportPathRunLogDir{
		{PathRunID: 5, Status: model.PathRunStatusCompleted, LogPath: logPath},
	})
	got := stats[5]
	if got.WriteCount != 1 || got.ReadCount != 1 {
		t.Fatalf("白名单兜底应判 读 1 写 1，实际 读 %d 写 %d", got.ReadCount, got.WriteCount)
	}
}

// TestReportInterfaceScanCachesTerminalPathRun 锁定缓存性质：
// 终态路径运行第二次命中签名缓存不再打开文件；文件被追加后签名变化，必须重算，
// 运行中（非终态）的路径运行每次重算，绝不用旧数冒充新数。
func TestReportInterfaceScanCachesTerminalPathRun(t *testing.T) {
	root := t.TempDir()
	logPath := writeNetworkLog(t, root, "12", "12", networkLine("12", "/api/web/flowInstanceApi/audit", "write", "1.000"))
	terminal := []repository.ReportPathRunLogDir{{PathRunID: 12, Status: model.PathRunStatusCompleted, LogPath: logPath}}

	scanner := service.NewReportInterfaceScanner(root)
	if _, capped := scanner.Aggregate(t.Context(), terminal); capped {
		t.Fatalf("单条不应触达采样上限")
	}
	if opens := scanner.OpenCount(); opens != 1 {
		t.Fatalf("首次应打开 1 次，实际 %d", opens)
	}
	if _, _ = scanner.Aggregate(t.Context(), terminal); scanner.OpenCount() != 1 {
		t.Fatalf("终态第二次应命中缓存不再打开，实际打开 %d 次", scanner.OpenCount())
	}

	// 追加一行：大小与修改时间都会变，必须重算并反映新增请求。
	file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(filepath.Dir(logPath)), "network.log"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("追加重放失败：%v", err)
	}
	if _, err := file.WriteString(networkLine("12", "/api/web/flowInstanceApi/list", "read", "0.500")); err != nil {
		t.Fatalf("写入追加密失败：%v", err)
	}
	file.Close()
	// 追加可能落在同一秒内导致 mtime 相同，显式把修改时间推到未来，保证签名变化可见。
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(filepath.Dir(logPath)), "network.log"), future, future); err != nil {
		t.Fatalf("改修改时间失败：%v", err)
	}
	stats, _ := scanner.Aggregate(t.Context(), terminal)
	if stats[12].ReadCount != 1 || stats[12].WriteCount != 1 {
		t.Fatalf("追加后应重算为 读 1 写 1，实际 读 %d 写 %d", stats[12].ReadCount, stats[12].WriteCount)
	}

	// 运行中的路径运行不写缓存：日志继续增长，每次都要读到最新事实。
	running := []repository.ReportPathRunLogDir{{PathRunID: 12, Status: model.PathRunStatusRunning, LogPath: logPath}}
	runningScanner := service.NewReportInterfaceScanner(root)
	_, _ = runningScanner.Aggregate(t.Context(), running)
	_, _ = runningScanner.Aggregate(t.Context(), running)
	if opens := runningScanner.OpenCount(); opens != 2 {
		t.Fatalf("运行中的路径运行不应缓存，两次应打开 2 次，实际 %d", opens)
	}
}

// TestReportInterfaceScanMissingLogDegrades 锁定缺失降级：
// 日志目录已被清理、或该路径运行从没落过日志时，返回空事实而不是 0 次调用，
// 前端据此显示「暂无真实请求耗时」，绝不用 0 冒充。
func TestReportInterfaceScanMissingLogDegrades(t *testing.T) {
	root := t.TempDir()
	scanner := service.NewReportInterfaceScanner(root)
	stats, capped := scanner.Aggregate(t.Context(), []repository.ReportPathRunLogDir{
		{PathRunID: 1, Status: model.PathRunStatusCompleted, LogPath: "logs/gone/step.log"},
		{PathRunID: 2, Status: model.PathRunStatusCompleted, LogPath: ""},
	})
	if capped {
		t.Fatalf("空目录不应触达采样上限")
	}
	for _, id := range []uint64{1, 2} {
		got := stats[id]
		if got.ReadCount != 0 || got.WriteCount != 0 || got.AnyKnown {
			t.Fatalf("路径运行 %d 无日志时必须全零且 AnyKnown=false，实际 %+v", id, got)
		}
	}
	if opens := scanner.OpenCount(); opens != 0 {
		t.Fatalf("没有日志时不应打开任何文件，实际 %d", opens)
	}
}

// TestReportInterfaceScanSamplesCapKeepsMostRecent 锁定采样上限：
// 超过上限只统计最新的若干条（rows 已按最新在前），并如实报告已截断。
func TestReportInterfaceScanSamplesCapKeepsMostRecent(t *testing.T) {
	root := t.TempDir()
	total := service.ReportInterfaceSampleCapForTest() + 3
	dirs := make([]repository.ReportPathRunLogDir, 0, total)
	for index := 0; index < total; index++ {
		id := uint64(index + 1)
		dirName := fmt.Sprintf("path%d", id)
		dirs = append(dirs, repository.ReportPathRunLogDir{
			PathRunID: id, Status: model.PathRunStatusCompleted,
			// 日志行里的 path_run_id 必须与目录对应的路径运行 ID 一致，否则归属判断不通过。
			LogPath: writeNetworkLog(t, root, dirName, fmt.Sprintf("%d", id), networkLine(fmt.Sprintf("%d", id), "/api/web/flowInstanceApi/audit", "write", "1.000")),
		})
	}
	scanner := service.NewReportInterfaceScanner(root)
	stats, capped := scanner.Aggregate(t.Context(), dirs)
	if !capped {
		t.Fatalf("超过采样上限必须标记已截断")
	}
	if len(stats) != service.ReportInterfaceSampleCapForTest() {
		t.Fatalf("只应统计最近的 %d 条，实际 %d", service.ReportInterfaceSampleCapForTest(), len(stats))
	}
	if stats[uint64(total)].WriteCount != 0 {
		t.Fatalf("被截掉的旧样本不应出现在结果里")
	}
	if stats[1].WriteCount != 1 {
		t.Fatalf("最新样本应被统计到，实际 %+v", stats[1])
	}
}
