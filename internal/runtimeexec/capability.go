// T012/T014 runtime 能力登记：只有真实执行成功并完成目标结果核验的组合才标记可运行。
// 登记键为「流程类型 + 动作 + 表单摘要」三维：同流程提交成功不自动解锁同意/不同意；
// 同端点一个动作成功不自动解锁另一个动作（docs/s/02-执行任务清单.md T014）。
package runtimeexec

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// CapabilityEntry 是一条能力登记事实。
type CapabilityEntry struct {
	FlowType       string    `json:"flowType"`
	Action         string    `json:"action"`
	FormDigest     string    `json:"formDigest"`
	RuntimeVersion string    `json:"runtimeVersion"`
	TargetVersion  string    `json:"targetVersion"`
	Evidence       string    `json:"evidence"` // 目标事实证据（实例 ID/trace 等）
	LogRef         string    `json:"logRef"`   // 请求轨迹日志引用
	RegisteredAt   time.Time `json:"registeredAt"`
}

// CapabilityRegistry 是能力登记的唯一入口；执行器路由据此决定是否走 runtime。
type CapabilityRegistry struct {
	mu      sync.Mutex
	entries map[string]CapabilityEntry
}

// NewCapabilityRegistry 构造空登记表：未登记即不可运行。
func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{entries: map[string]CapabilityEntry{}}
}

// capabilityKey 生成登记键；表单摘要为空时按「流程+动作」粒度登记（同流程其他动作仍不可运行）。
func capabilityKey(flowType, action, formDigest string) string {
	return strings.Join([]string{strings.TrimSpace(flowType), strings.TrimSpace(action), strings.TrimSpace(formDigest)}, "|")
}

// Register 登记一条已验证能力；重复登记覆盖旧值并更新时间。
func (r *CapabilityRegistry) Register(entry CapabilityEntry) error {
	if strings.TrimSpace(entry.FlowType) == "" || strings.TrimSpace(entry.Action) == "" {
		return fmt.Errorf("能力登记缺少流程类型或动作")
	}
	entry.FlowType = strings.TrimSpace(entry.FlowType)
	entry.Action = strings.TrimSpace(entry.Action)
	entry.FormDigest = strings.TrimSpace(entry.FormDigest)
	entry.RegisteredAt = time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = map[string]CapabilityEntry{}
	}
	r.entries[capabilityKey(entry.FlowType, entry.Action, entry.FormDigest)] = entry
	return nil
}

// Registered 判定组合是否已登记可运行；表单摘要非空时先按精确键，再回落到无摘要粒度。
func (r *CapabilityRegistry) Registered(flowType, action, formDigest string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		return false
	}
	if _, ok := r.entries[capabilityKey(flowType, action, formDigest)]; ok {
		return true
	}
	_, ok := r.entries[capabilityKey(flowType, action, "")]
	return ok
}

// Snapshot 返回全部登记（按键排序），供运行详情与能力登记表导出。
func (r *CapabilityRegistry) Snapshot() []CapabilityEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	keys := make([]string, 0, len(r.entries))
	for key := range r.entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]CapabilityEntry, 0, len(keys))
	for _, key := range keys {
		result = append(result, r.entries[key])
	}
	return result
}

// UnlockFromVerifiedRun 仅在动作真实执行成功且完成目标结果核验后登记能力。
// 纪律：target_rejected/business_failed/uncertain/timeout 一律不登记——
// 提交成功不自动解锁同流程同意/不同意，同端点一个动作成功不自动解锁另一个动作。
func (r *CapabilityRegistry) UnlockFromVerifiedRun(entry CapabilityEntry, verifiedVerdict string) error {
	if strings.TrimSpace(verifiedVerdict) != "confirmed_success" {
		return fmt.Errorf("核验判定 %s 不允许登记能力（仅 confirmed_success）", verifiedVerdict)
	}
	return r.Register(entry)
}

// BlockedReason 说明组合不可运行的常见原因（登记表中如实记录，不用代码伪造通过）。
const (
	BlockedUnsupportedComponent   = "包含未登记的目标自定义组件"
	BlockedOnlineEditor           = "依赖在线 Office 编辑器且初始化流程未验证"
	BlockedUnconfirmableResult    = "该表单的动作结果无法由请求与回读确认"
	BlockedDragOrRealSize         = "依赖真实尺寸或拖拽交互，无头环境不支持"
	BlockedUnregisteredExternalWS = "存在未登记的外部网络请求"
)
