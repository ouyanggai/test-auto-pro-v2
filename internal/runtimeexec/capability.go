// T012/T014 runtime 能力登记：只有真实执行成功并完成目标结果核验的组合才标记可运行。
// 登记键为「流程类型 + 动作 + 表单摘要」三维：同流程提交成功不自动解锁同意/不同意；
// 同端点一个动作成功不自动解锁另一个动作（docs/s/02-执行任务清单.md T014）。
// 持久化（评审 P1-2 修复）：登记表按 JSON 文件落盘（.runtime/runtime-capabilities.json），
// 启动时由装配层加载、每次登记成功即原子重写，Go 服务重启不再丢失已验证能力。
package runtimeexec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Evidence       string    `json:"evidence"` // 目标事实证据（实例 ID + trace ID）
	LogRef         string    `json:"logRef"`   // 请求轨迹日志引用
	RegisteredAt   time.Time `json:"registeredAt"`
}

// CapabilityRegistry 是能力登记的唯一入口；执行器路由据此决定是否走 runtime。
// persistPath 非空时每次登记成功都会把全量登记原子写回该文件（进程内并发安全）。
type CapabilityRegistry struct {
	mu          sync.Mutex
	entries     map[string]CapabilityEntry
	persistPath string
}

// NewCapabilityRegistry 构造内存登记表：未登记即不可运行。测试与临时场景使用。
func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{entries: map[string]CapabilityEntry{}}
}

// OpenCapabilityRegistry 构造带文件持久化的登记表并从 path 加载既有登记：
// 文件不存在视为空表（首次使用），解析失败必须报错——带着损坏的证据文件静默起跑
// 会让「已验证」与「未验证」的边界失真，绝不允许。
func OpenCapabilityRegistry(path string) (*CapabilityRegistry, error) {
	registry := &CapabilityRegistry{entries: map[string]CapabilityEntry{}, persistPath: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return nil, fmt.Errorf("读取能力登记表失败：%w", err)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return registry, nil
	}
	var entries []CapabilityEntry
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return nil, fmt.Errorf("能力登记表 %s 不是合法 JSON：%w", path, err)
	}
	for _, entry := range entries {
		if err := registry.register(entry); err != nil {
			return nil, fmt.Errorf("能力登记表 %s 含不合法登记：%w", path, err)
		}
	}
	return registry, nil
}

// capabilityKey 生成登记键：流程类型 + 动作 + 表单摘要三维齐全。
func capabilityKey(flowType, action, formDigest string) string {
	return strings.Join([]string{strings.TrimSpace(flowType), strings.TrimSpace(action), strings.TrimSpace(formDigest)}, "|")
}

// Register 登记一条已验证能力；重复登记覆盖旧值并更新时间。
// 纪律（docs/s/03-能力登记表.md 登记纪律 3）：每条登记必须携带表单摘要、runtime 版本、
// 目标版本、目标事实证据与日志引用，缺一不得登记——证据不全的「已验证」无法事后追责。
func (r *CapabilityRegistry) Register(entry CapabilityEntry) error {
	if err := r.register(entry); err != nil {
		return err
	}
	return r.persist()
}

// register 只做内存登记与字段校验；持久化由调用方决定时机。
func (r *CapabilityRegistry) register(entry CapabilityEntry) error {
	entry.FlowType = strings.TrimSpace(entry.FlowType)
	entry.Action = strings.TrimSpace(entry.Action)
	entry.FormDigest = strings.TrimSpace(entry.FormDigest)
	entry.RuntimeVersion = strings.TrimSpace(entry.RuntimeVersion)
	entry.TargetVersion = strings.TrimSpace(entry.TargetVersion)
	entry.Evidence = strings.TrimSpace(entry.Evidence)
	entry.LogRef = strings.TrimSpace(entry.LogRef)
	missing := []string{}
	if entry.FlowType == "" {
		missing = append(missing, "流程类型")
	}
	if entry.Action == "" {
		missing = append(missing, "动作")
	}
	if entry.FormDigest == "" {
		missing = append(missing, "表单摘要")
	}
	if entry.RuntimeVersion == "" {
		missing = append(missing, "runtime版本")
	}
	if entry.TargetVersion == "" {
		missing = append(missing, "目标版本")
	}
	if entry.Evidence == "" {
		missing = append(missing, "证据")
	}
	if entry.LogRef == "" {
		missing = append(missing, "日志引用")
	}
	if len(missing) > 0 {
		return fmt.Errorf("能力登记缺少 %s，拒绝登记", strings.Join(missing, "、"))
	}
	entry.RegisteredAt = time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = map[string]CapabilityEntry{}
	}
	r.entries[capabilityKey(entry.FlowType, entry.Action, entry.FormDigest)] = entry
	return nil
}

// persist 把全量登记原子写回持久化文件：先写同目录临时文件再 rename，避免半截文件。
// 注意不能在持锁状态下调用 Snapshot（它自身也要拿同一把锁），这里在锁内手工复制。
func (r *CapabilityRegistry) persist() error {
	r.mu.Lock()
	keys := make([]string, 0, len(r.entries))
	for key := range r.entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]CapabilityEntry, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, r.entries[key])
	}
	path := r.persistPath
	r.mu.Unlock()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建能力登记目录失败：%w", err)
	}
	encoded, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("能力登记序列化失败：%w", err)
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, encoded, 0o644); err != nil {
		return fmt.Errorf("写能力登记临时文件失败：%w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("能力登记落盘失败：%w", err)
	}
	return nil
}

// Registered 判定组合是否已登记可运行：登记键三维必须与运行时同一把摘要精确一致，
// 不再回落到无摘要粒度——「表单摘要未知就放行」等于绕过三维登记纪律。
func (r *CapabilityRegistry) Registered(flowType, action, formDigest string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		return false
	}
	_, ok := r.entries[capabilityKey(flowType, action, formDigest)]
	return ok
}

// Snapshot 返回全部登记（按键排序），供运行详情与能力登记表导出。
func (r *CapabilityRegistry) Snapshot() []CapabilityEntry {
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
