package backend

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/service"
)

type mockCandidateProvider struct {
	materials map[string][]target.MaterialCandidate
	projects  []target.ProjectCandidate
	orders    []target.OrderCandidate
}

func (m *mockCandidateProvider) GetMaterialCandidates(ctx context.Context, account, flowCode, direction string) ([]target.MaterialCandidate, error) {
	if m.materials == nil {
		return []target.MaterialCandidate{}, nil
	}
	return m.materials[direction], nil
}

func (m *mockCandidateProvider) GetProjectCandidates(ctx context.Context, account, flowCode string) ([]target.ProjectCandidate, error) {
	return m.projects, nil
}

func (m *mockCandidateProvider) GetOrderCandidates(ctx context.Context, account, flowCode string) ([]target.OrderCandidate, error) {
	return m.orders, nil
}

func (m *mockCandidateProvider) GetFlowListCandidates(ctx context.Context, account, flowCode string) ([]target.FlowListCandidate, error) {
	return []target.FlowListCandidate{}, nil
}

func (m *mockCandidateProvider) GetExpenseBudgetTypes(ctx context.Context, account string) ([]target.ExpenseBudgetType, error) {
	return []target.ExpenseBudgetType{}, nil
}

func (m *mockCandidateProvider) GetCityCandidates(ctx context.Context, account string) ([]target.CityCandidate, error) {
	return []target.CityCandidate{
		{Code: "110000", Name: "北京市", Province: "北京", Level: "province"},
		{Code: "310000", Name: "上海市", Province: "上海", Level: "province"},
	}, nil
}

func (m *mockCandidateProvider) GetTravelRoutes(ctx context.Context, account string) ([]target.TravelRoute, error) {
	return []target.TravelRoute{}, nil
}

func TestComponentCandidateCache_GetCandidateSet(t *testing.T) {
	provider := &mockCandidateProvider{
		materials: map[string][]target.MaterialCandidate{
			"out": {
				{ID: "M001", Name: "材料A", Code: "MAT-001", Price: 100.0},
				{ID: "M002", Name: "材料B", Code: "MAT-002", Price: 200.0},
			},
			"in": {
				{ID: "M003", Name: "材料C", Code: "MAT-003", Price: 150.0},
			},
		},
		projects: []target.ProjectCandidate{
			{ID: "P001", Name: "项目A", Code: "PROJ-001", Status: "active"},
			{ID: "P002", Name: "项目B", Code: "PROJ-002", Status: "active"},
		},
		orders: []target.OrderCandidate{
			{ID: "O001", OrderNo: "ORD-001", Amount: 5000.0, Status: "active"},
		},
	}

	cache := service.NewComponentCandidateCache(provider, 100, 1*time.Minute)
	ctx := context.Background()

	// 第一次加载
	set1, err := cache.GetCandidateSet(ctx, "testuser", "FLOW001", "v1")
	if err != nil {
		t.Fatalf("GetCandidateSet failed: %v", err)
	}

	if len(set1.Materials["out"]) != 2 {
		t.Errorf("Expected 2 out materials, got %d", len(set1.Materials["out"]))
	}

	if len(set1.Materials["in"]) != 1 {
		t.Errorf("Expected 1 in material, got %d", len(set1.Materials["in"]))
	}

	if len(set1.Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(set1.Projects))
	}

	if len(set1.Orders) != 1 {
		t.Errorf("Expected 1 order, got %d", len(set1.Orders))
	}

	if len(set1.Cities) != 2 {
		t.Errorf("Expected 2 cities, got %d", len(set1.Cities))
	}

	// 第二次加载（应该命中缓存）
	set2, err := cache.GetCandidateSet(ctx, "testuser", "FLOW001", "v1")
	if err != nil {
		t.Fatalf("GetCandidateSet (cached) failed: %v", err)
	}

	if len(set2.Materials["out"]) != 2 {
		t.Errorf("Expected 2 out materials from cache, got %d", len(set2.Materials["out"]))
	}

	// 检查统计
	stats := cache.Stats()
	if stats.TotalEntries != 1 {
		t.Errorf("Expected 1 cache entry, got %d", stats.TotalEntries)
	}
}

func TestComponentCandidateCache_Invalidate(t *testing.T) {
	provider := &mockCandidateProvider{
		projects: []target.ProjectCandidate{
			{ID: "P001", Name: "项目A", Code: "PROJ-001", Status: "active"},
		},
	}

	cache := service.NewComponentCandidateCache(provider, 100, 1*time.Minute)
	ctx := context.Background()

	// 加载缓存
	_, err := cache.GetCandidateSet(ctx, "testuser", "FLOW001", "v1")
	if err != nil {
		t.Fatalf("GetCandidateSet failed: %v", err)
	}

	stats1 := cache.Stats()
	if stats1.TotalEntries != 1 {
		t.Errorf("Expected 1 cache entry, got %d", stats1.TotalEntries)
	}

	// 失效缓存
	cache.Invalidate("testuser", "FLOW001", "v1")

	stats2 := cache.Stats()
	if stats2.TotalEntries != 0 {
		t.Errorf("Expected 0 cache entries after invalidate, got %d", stats2.TotalEntries)
	}
}

func TestComponentCandidateCache_GetFieldCandidates(t *testing.T) {
	provider := &mockCandidateProvider{
		materials: map[string][]target.MaterialCandidate{
			"out": {
				{ID: "M001", Name: "材料A", Code: "MAT-001", Price: 100.0},
			},
		},
	}

	cache := service.NewComponentCandidateCache(provider, 100, 1*time.Minute)
	ctx := context.Background()

	// 获取材料字段候选
	candidates, err := cache.GetFieldCandidates(ctx, "testuser", "FLOW001", "v1", "material", "out-bound-material-select")
	if err != nil {
		t.Fatalf("GetFieldCandidates failed: %v", err)
	}

	if len(candidates) != 1 {
		t.Errorf("Expected 1 material candidate, got %d", len(candidates))
	}

	if material, ok := candidates[0].(target.MaterialCandidate); ok {
		if material.ID != "M001" {
			t.Errorf("Expected material ID M001, got %s", material.ID)
		}
	} else {
		t.Errorf("Candidate is not a MaterialCandidate")
	}
}

func TestComponentCandidateCache_LRU(t *testing.T) {
	provider := &mockCandidateProvider{
		projects: []target.ProjectCandidate{
			{ID: "P001", Name: "项目A", Code: "PROJ-001", Status: "active"},
		},
	}

	// 创建只能容纳2个条目的缓存
	cache := service.NewComponentCandidateCache(provider, 2, 1*time.Minute)
	ctx := context.Background()

	// 加载3个不同的候选集
	_, _ = cache.GetCandidateSet(ctx, "user1", "FLOW001", "v1")
	_, _ = cache.GetCandidateSet(ctx, "user2", "FLOW002", "v1")
	_, _ = cache.GetCandidateSet(ctx, "user3", "FLOW003", "v1")

	stats := cache.Stats()
	if stats.TotalEntries > 2 {
		t.Errorf("Expected at most 2 cache entries (LRU), got %d", stats.TotalEntries)
	}
}

func TestComponentCandidateCache_Expiration(t *testing.T) {
	provider := &mockCandidateProvider{
		projects: []target.ProjectCandidate{
			{ID: "P001", Name: "项目A", Code: "PROJ-001", Status: "active"},
		},
	}

	// 创建1秒过期的缓存
	cache := service.NewComponentCandidateCache(provider, 100, 1*time.Second)
	ctx := context.Background()

	// 加载缓存
	_, err := cache.GetCandidateSet(ctx, "testuser", "FLOW001", "v1")
	if err != nil {
		t.Fatalf("GetCandidateSet failed: %v", err)
	}

	// 等待过期
	time.Sleep(1500 * time.Millisecond)

	// 再次加载（应该重新获取）
	_, err = cache.GetCandidateSet(ctx, "testuser", "FLOW001", "v1")
	if err != nil {
		t.Fatalf("GetCandidateSet after expiration failed: %v", err)
	}

	stats := cache.Stats()
	if stats.TotalEntries != 1 {
		t.Errorf("Expected 1 cache entry after reload, got %d", stats.TotalEntries)
	}
}
