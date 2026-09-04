package repository

import (
	"context"
	"errors"
	"time"

	"test-auto-pro-v2/internal/model"
)

var (
	// ErrPathSuccessAssertionNotFound 表示这条路径还没有配置成功断言。
	ErrPathSuccessAssertionNotFound = errors.New("执行路径成功断言不存在")
	// ErrPathSuccessAssertionPathNotFound 表示路径不存在或不属于该计划；按 ID 读取本身就是归属校验。
	ErrPathSuccessAssertionPathNotFound = errors.New("执行路径不存在或不属于该计划")
	// ErrPathSuccessAssertionRevisionConflict 表示断言在本次保存期间已被其他人改过。
	ErrPathSuccessAssertionRevisionConflict = errors.New("执行路径成功断言修订已变化")
)

// PathSuccessAssertionStore 只负责成功断言的读写。
// 候选推导与取值校验属于服务层职责，仓储不做业务判断，也不自动修正已保存的失效断言。
type PathSuccessAssertionStore interface {
	// Get 读取单条路径的断言；路径不属于该计划时返回 ErrPathSuccessAssertionPathNotFound，
	// 路径存在但没有断言时返回 ErrPathSuccessAssertionNotFound。
	Get(ctx context.Context, planID, pathID uint64) (model.PathSuccessAssertion, error)
	// ListByPlan 一次取齐计划下全部路径的断言，键为路径 ID。
	// 运行准备聚合要按计划逐路径判断，必须批量读取，禁止逐路径查询。
	ListByPlan(ctx context.Context, planID uint64) (map[uint64]model.PathSuccessAssertion, error)
	// Save 幂等写入断言并返回写入后的结果。
	// expectedRevision 为 0 表示调用方认为断言尚不存在，非 0 时必须与当前修订一致，否则返回修订冲突；
	// 同一 idempotencyKey 重复到达时直接返回已有结果，不产生第二次修订。
	Save(ctx context.Context, planID uint64, assertion model.PathSuccessAssertion, expectedRevision uint64, idempotencyKey string, now time.Time) (model.PathSuccessAssertion, error)
}
