package step

import (
	"path/filepath"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/logging"
)

// NewRouterStepLogFactory 复用 F-013 的运行目录路由，把每条路径运行的 step.log 写入器交给执行器。
// 日志目录形如 logs/runs/运行_<运行号>__<计划名>__run-<运行ID>/paths/<路径名>__path-run-<路径运行ID>__path-<路径ID>/step.log，
// 与页面「运行记录 -> 路径运行」逐层对应，每行贯穿 plan/path/run/path_run/instance/step 关联键。
// 同一路径运行的多次调用共享同一个底层写入器，行号全运行连续。
// 实例身份只在读到时补进作用域与 meta.json，不参与目录寻址。
func NewRouterStepLogFactory(router *logging.Router) LogFactory {
	return func(runCtx RunContext) *StepLog {
		scope := logging.Scope{
			PlanID:            strconv.FormatUint(runCtx.Run.PlanID, 10),
			PlanName:          runCtx.PlanName,
			ExecutionPathID:   strconv.FormatUint(runCtx.PathRun.ExecutionPathID, 10),
			ExecutionPathName: runCtx.PathName,
			RunID:             strconv.FormatUint(runCtx.Run.ID, 10),
			RunSeq:            strconv.FormatUint(runCtx.Run.RunNo, 10),
			PathRunID:         strconv.FormatUint(runCtx.PathRun.ID, 10),
			// 待发/已发场景在运行开始就绑定真实实例；新发起要等提交成功才有实例 ID。
			InstanceID: strings.TrimSpace(runCtx.PathRun.MainInstanceRef),
		}
		writer := router.Bucket(scope, "step.log")
		log := NewStepLog(writer, scope, nil)
		log.SetInstanceScopeUpdater(func(updated logging.Scope) {
			router.UpdateRunInstanceMeta(updated)
		})
		dir := router.BucketDir(scope)
		if relative, err := filepath.Rel(router.Root(), dir); err == nil {
			log.SetRelativePath(filepath.ToSlash(filepath.Join(relative, "step.log")))
		}
		return log
	}
}
