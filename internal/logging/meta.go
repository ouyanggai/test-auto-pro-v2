package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MetaFileName 是业务日志目录里的归属说明文件名。
const MetaFileName = "meta.json"

// bucketMeta 说明一个业务日志目录属于哪个计划、哪条执行路径，让人从目录本身就能确认归属。
// 配置阶段目录写日期，运行目录写运行号与开始时间，字段名与前端使用的驼峰命名保持一致。
type bucketMeta struct {
	PlanID            string `json:"planId"`
	PlanName          string `json:"planName"`
	ExecutionPathID   string `json:"executionPathId"`
	ExecutionPathName string `json:"executionPathName"`
	Date              string `json:"date,omitempty"`
	RunID             string `json:"runId,omitempty"`
	StartedAt         string `json:"startedAt,omitempty"`
}

// ensureMeta 在业务日志目录首次使用时写入 meta.json。
// application 目录没有业务归属，不写；已经存在的文件不覆盖，避免把先前运行的开始时间改掉。
// 写失败只降级为一次标准错误输出，绝不影响主流程。
func (r *Router) ensureMeta(scope Scope, dir string) {
	if r == nil || !scope.HasPlan() {
		return
	}
	r.mu.Lock()
	if r.metaWritten[dir] {
		r.mu.Unlock()
		return
	}
	r.metaWritten[dir] = true
	r.mu.Unlock()
	path := filepath.Join(dir, MetaFileName)
	if _, err := os.Stat(path); err == nil {
		return
	}
	meta := bucketMeta{
		PlanID:            strings.TrimSpace(scope.PlanID),
		PlanName:          strings.TrimSpace(scope.PlanName),
		ExecutionPathID:   strings.TrimSpace(scope.ExecutionPathID),
		ExecutionPathName: strings.TrimSpace(scope.ExecutionPathName),
	}
	if scope.IsRun() {
		meta.RunID = scope.RunFolder()
		if runID := strings.TrimSpace(scope.RunID); runID != "" {
			meta.RunID = runID
		}
		meta.StartedAt = r.now().Format(time.RFC3339)
	} else {
		meta.Date = r.Day()
	}
	encoded, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		reportWriteFailure(path, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		reportWriteFailure(path, err)
		return
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		reportWriteFailure(path, err)
	}
}
