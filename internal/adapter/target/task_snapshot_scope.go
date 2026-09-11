// F-030/T02：同一次事实读取边界内的任务列表复用。
// 作用域用 context 传递：一次 readInstanceFacts（一次 gate 预览或一次核验重读）内，
// 对同一 (实例, 状态) 的多次 FindTaskSnapshot（计划账号/候选人/当前处理人切换、节点回退）
// 只发一次目标分页扫描，其余命中 context 内 memo；每次新的事实读取从空 memo 开始，
// 写请求后的核验自然全新——写后缓存失效屏障由作用域本身保证，不存在跨步污染。
// 会话维度不进 memo 键：同一账号锁内会话不会中途切换，不同账号各自查 pending 由
// 目标按 SID 过滤，本来就该各自扫描，memo 只在同一会话读取边界内生效（见 AttachTaskSnapshotScope）。
package target

import "context"

type taskSnapshotScopeKey struct{}

// taskSnapshotScope 是一次事实读取边界内的任务列表 memo。
type taskSnapshotScope struct {
	// lists 按 (sessionID, instanceID, status) 缓存一次扫描结果。
	lists map[string][]TaskSnapshot
}

// AttachTaskSnapshotScope 开启一次事实读取边界：返回的 context 内所有 FindTaskSnapshot
// 共享同一 memo。由执行器在 readInstanceFacts 入口调用。
func AttachTaskSnapshotScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, taskSnapshotScopeKey{}, &taskSnapshotScope{lists: map[string][]TaskSnapshot{}})
}

// taskListFromScope 读取 memo；无作用域时返回 nil。
func taskListFromScope(ctx context.Context, key string) []TaskSnapshot {
	scope, _ := ctx.Value(taskSnapshotScopeKey{}).(*taskSnapshotScope)
	if scope == nil {
		return nil
	}
	return scope.lists[key]
}

// storeTaskListToScope 写入 memo；无作用域时忽略。
func storeTaskListToScope(ctx context.Context, key string, snapshots []TaskSnapshot) {
	scope, _ := ctx.Value(taskSnapshotScopeKey{}).(*taskSnapshotScope)
	if scope == nil {
		return
	}
	scope.lists[key] = snapshots
}
