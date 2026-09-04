package step

import (
	"path/filepath"
	"strconv"

	"test-auto-pro-v2/internal/logging"
)

// NewRouterStepLogFactory 复用 F-013 的运行目录路由，把每条路径运行的 step.log 写入器交给执行器。
// 日志目录形如 logs/plans/<计划>__plan-<ID>/runs/<执行路径>__path-<ID>/<运行号>/step.log，
// 每行贯穿 plan/path/run/path_run/step 关联键，使日志能查回运行记录（纲领第 6.2、6.3 节）。
// 同一路径运行的多次调用共享同一个底层写入器，行号全运行连续。
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
		}
		writer := router.Bucket(scope, "step.log")
		log := NewStepLog(writer, scope, nil)
		dir := router.BucketDir(scope)
		if relative, err := filepath.Rel(router.Root(), dir); err == nil {
			log.SetRelativePath(filepath.ToSlash(filepath.Join(relative, "step.log")))
		}
		return log
	}
}
