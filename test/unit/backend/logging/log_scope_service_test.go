package logging_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// fakeLogScopeStore 记录读取次数，用来证明归属解析不会为每条日志重复查询数据库。
type fakeLogScopeStore struct {
	planCalls int
	pathCalls int
	planName  string
	pathName  string
	planErr   error
	pathErr   error
}

// Get 返回计划记录；planErr 非空时模拟数据库读取失败。
func (f *fakeLogScopeStore) Get(_ context.Context, planID uint64) (model.Plan, error) {
	f.planCalls++
	if f.planErr != nil {
		return model.Plan{}, f.planErr
	}
	return model.Plan{ID: planID, Name: f.planName}, nil
}

// GetPath 按计划读取执行路径；路径不属于该计划时返回错误，等价于目标仓储的归属校验。
func (f *fakeLogScopeStore) GetPath(_ context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	f.pathCalls++
	if f.pathErr != nil {
		return model.ExecutionPath{}, f.pathErr
	}
	return model.ExecutionPath{ID: pathID, PlanID: planID, Name: f.pathName}, nil
}

// logScopePathStore 把 GetPath 适配成执行路径仓储的 Get 签名，避免与计划的 Get 重名。
type logScopePathStore struct{ store *fakeLogScopeStore }

// Get 转发到底层假仓储的 GetPath。
func (a logScopePathStore) Get(ctx context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	return a.store.GetPath(ctx, planID, pathID)
}

// TestLogScopeServiceResolvesRealNamesOnce 验证显示名来自真实业务记录，
// 并且同一计划与路径在缓存有效期内只查库一次。
func TestLogScopeServiceResolvesRealNamesOnce(t *testing.T) {
	store := &fakeLogScopeStore{planName: "员工请假单（集团）-自动回归", pathName: "执行路径 1"}
	resolver := service.NewLogScopeService(store, logScopePathStore{store: store}, func() time.Time {
		return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	})
	for index := 0; index < 3; index++ {
		scope := resolver.ResolveLogScope(context.Background(), 7, 13)
		if scope.PlanID != "7" || scope.PlanName != "员工请假单（集团）-自动回归" {
			t.Fatalf("计划归属解析不正确：%+v", scope)
		}
		if scope.ExecutionPathID != "13" || scope.ExecutionPathName != "执行路径 1" {
			t.Fatalf("执行路径归属解析不正确：%+v", scope)
		}
	}
	if store.planCalls != 1 || store.pathCalls != 1 {
		t.Fatalf("同一请求对象重复查询了数据库：plan=%d path=%d", store.planCalls, store.pathCalls)
	}
}

// TestLogScopeServiceKeepsPlanIDWhenNameUnavailable 验证名称查不到时仍然保留计划 ID，
// 日志继续落在该计划目录，不因为上下文没接通就降级到应用程序目录。
func TestLogScopeServiceKeepsPlanIDWhenNameUnavailable(t *testing.T) {
	store := &fakeLogScopeStore{planErr: errors.New("数据库暂时不可用"), pathErr: errors.New("路径不属于该计划")}
	resolver := service.NewLogScopeService(store, logScopePathStore{store: store}, time.Now)
	scope := resolver.ResolveLogScope(context.Background(), 7, 13)
	if scope.PlanID != "7" || scope.ExecutionPathID != "13" {
		t.Fatalf("解析失败时丢掉了不可变 ID：%+v", scope)
	}
	if scope.PlanName != "" || scope.ExecutionPathName != "" {
		t.Fatalf("读取失败时不得编造显示名：%+v", scope)
	}
	if !scope.HasPlan() {
		t.Fatal("只要能确定计划 ID 就必须按计划归属处理")
	}
}

// TestLogScopeServiceIgnoresMissingPlan 验证没有计划 ID 时返回空作用域，交由调用方按系统级日志处理。
func TestLogScopeServiceIgnoresMissingPlan(t *testing.T) {
	store := &fakeLogScopeStore{planName: "计划", pathName: "路径"}
	resolver := service.NewLogScopeService(store, logScopePathStore{store: store}, time.Now)
	if scope := resolver.ResolveLogScope(context.Background(), 0, 0); scope.HasPlan() {
		t.Fatalf("没有计划时不应编造归属：%+v", scope)
	}
	if store.planCalls != 0 {
		t.Fatalf("没有计划时不应查询数据库：%d", store.planCalls)
	}
}
