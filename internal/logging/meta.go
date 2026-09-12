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

// bucketMeta 说明一个业务日志目录属于哪个计划、哪条执行路径、哪次运行，让人从目录本身就能确认归属。
// 运行目录写运行号与运行 ID、路径运行 ID 与路径 ID，以及目标实例名称；配置阶段目录写日期。
// 实例名称只作业务信息：为空时 instanceNameAvailable 为假，note 写明原因，绝不用计划名或路径名冒充。
type bucketMeta struct {
	PlanID            string `json:"planId"`
	PlanName          string `json:"planName"`
	ExecutionPathID   string `json:"executionPathId"`
	ExecutionPathName string `json:"executionPathName"`
	Date              string `json:"date,omitempty"`
	RunID             string `json:"runId,omitempty"`
	RunNo             string `json:"runNo,omitempty"`
	PathRunID         string `json:"pathRunId,omitempty"`
	PathID            string `json:"pathId,omitempty"`
	PathName          string `json:"pathName,omitempty"`
	StartedAt         string `json:"startedAt,omitempty"`
	InstanceID        string `json:"instanceId,omitempty"`
	InstanceName      string `json:"instanceName,omitempty"`
	// InstanceNameAvailable 表示 instanceName 是不是真的从目标读到了名称。
	InstanceNameAvailable bool `json:"instanceNameAvailable"`
	// InstanceNameNote 只在名称为空时说明原因（读取失败或目标未返回名称）。
	InstanceNameNote string `json:"instanceNameNote,omitempty"`
}

// BucketMeta 是对外可读的目录归属信息，供运行详情直接展示实例名称与日志位置。
type BucketMeta = bucketMeta

// ensureMeta 在业务日志目录首次使用时写入 meta.json。
// application 目录没有业务归属，不写；已经存在的文件只补齐缺失字段，不覆盖既有内容
// （先写入的 startedAt 是这次运行真实的开始时间，后到的实例名称只能补进去，不能改写它）。
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
	r.writeMeta(scope, dir, true)
}

// UpdateRunInstanceMeta 把目标实例身份补进运行目录的 meta.json（F-031/T05）。
// 目标实例名称要等实例真实存在、且能用发起人会话精确读到之后才知道，而目录在运行开始时就已建立，
// 所以这一步只补字段、不改目录、不搬文件：目录键始终是页面的 runId/pathRunId。
// 写失败只降级为一次标准错误输出，绝不影响运行本身。
func (r *Router) UpdateRunInstanceMeta(scope Scope) {
	if r == nil || !scope.HasPlan() || !scope.IsRun() {
		return
	}
	r.writeMeta(scope, r.BucketDir(scope), false)
}

// ReadMeta 读取某个作用域目录的 meta.json，供运行详情展示实例名称与日志位置。
// 文件不存在或无法解析时返回 found=false，调用方按「实例名称不可用」如实展示，不补造名称。
func (r *Router) ReadMeta(scope Scope) (BucketMeta, bool) {
	if r == nil || !scope.HasPlan() {
		return BucketMeta{}, false
	}
	data, err := os.ReadFile(filepath.Join(r.BucketDir(scope), MetaFileName))
	if err != nil {
		return BucketMeta{}, false
	}
	var meta BucketMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return BucketMeta{}, false
	}
	return meta, true
}

// ReadMetaAt 读取指定目录的 meta.json，供历史运行按已落账的 step.log 相对路径回查旧目录。
func (r *Router) ReadMetaAt(dir string) (BucketMeta, bool) {
	if r == nil || strings.TrimSpace(dir) == "" {
		return BucketMeta{}, false
	}
	data, err := os.ReadFile(filepath.Join(filepath.Clean(dir), MetaFileName))
	if err != nil {
		return BucketMeta{}, false
	}
	var meta BucketMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return BucketMeta{}, false
	}
	return meta, true
}

// writeMeta 写入或补写 meta.json：alreadyWritten=true 只在文件不存在时新建，
// 否则按已有内容合并——同一次运行的开始时间与已有名称不能被后续写入抹掉。
func (r *Router) writeMeta(scope Scope, dir string, alreadyWritten bool) {
	path := filepath.Join(dir, MetaFileName)
	existing, hasExisting := BucketMeta{}, false
	if data, err := os.ReadFile(path); err == nil {
		if jsonErr := json.Unmarshal(data, &existing); jsonErr == nil {
			hasExisting = true
		}
	} else if alreadyWritten && !os.IsNotExist(err) {
		return
	}
	meta := bucketMeta{
		PlanID:            strings.TrimSpace(scope.PlanID),
		PlanName:          strings.TrimSpace(scope.PlanName),
		ExecutionPathID:   strings.TrimSpace(scope.ExecutionPathID),
		ExecutionPathName: strings.TrimSpace(scope.ExecutionPathName),
	}
	if scope.IsRun() {
		meta.RunID = strings.TrimSpace(scope.RunID)
		meta.RunNo = strings.TrimSpace(scope.RunSeq)
		meta.PathRunID = strings.TrimSpace(scope.PathRunID)
		meta.PathID = strings.TrimSpace(scope.ExecutionPathID)
		meta.PathName = strings.TrimSpace(scope.ExecutionPathName)
		meta.StartedAt = r.now().Format(time.RFC3339)
		meta.InstanceID = strings.TrimSpace(scope.InstanceID)
		meta.InstanceName = strings.TrimSpace(scope.InstanceName)
		meta.InstanceNameAvailable = meta.InstanceName != ""
		if !meta.InstanceNameAvailable {
			meta.InstanceNameNote = strings.TrimSpace(scope.InstanceNameNote)
			if meta.InstanceNameNote == "" {
				meta.InstanceNameNote = "目标实例名称尚未读取"
			}
		}
		if hasExisting && strings.TrimSpace(existing.StartedAt) != "" {
			meta.StartedAt = existing.StartedAt
		}
		// 已有名称不能被这次没读到名称的写入抹掉：名称只增不减，实例不会换。
		if !meta.InstanceNameAvailable && strings.TrimSpace(existing.InstanceName) != "" {
			meta.InstanceName = existing.InstanceName
			meta.InstanceNameAvailable = true
			meta.InstanceNameNote = ""
		}
		if meta.InstanceID == "" {
			meta.InstanceID = strings.TrimSpace(existing.InstanceID)
		}
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
