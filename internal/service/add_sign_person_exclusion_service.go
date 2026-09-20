package service

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/repository/mysql"
)

// AddSignCandidate 是按姓名检索目标人员目录的结果项（service 层类型，不依赖 api 包，避免循环导入）。
type AddSignCandidate struct {
	PersonID   string
	PersonName string
	Phone      string
	Enabled    bool
}

// disabledUsersTTL 是停用人员 ID 集合的缓存有效期：停用状态变化不频繁，缓存避免每次配置读都全表扫。
const disabledUsersTTL = 5 * time.Minute

// AddSignPersonExclusionService 提供排除名单的业务逻辑
type AddSignPersonExclusionService struct {
	repo          *mysql.AddSignPersonExclusionRepository
	userDirectory repository.TargetUserDirectory

	// 停用人员 ID 集合缓存：默认排除逻辑要用，带 TTL 避免高频全表扫描。
	disabledMu     sync.Mutex
	disabledCache  map[string]bool
	disabledExpiry time.Time
}

// NewAddSignPersonExclusionService 创建服务实例
func NewAddSignPersonExclusionService(repo *mysql.AddSignPersonExclusionRepository) *AddSignPersonExclusionService {
	return &AddSignPersonExclusionService{repo: repo}
}

// SetUserDirectory 注入目标人员目录只读来源，用于按姓名检索候选；未注入时检索端点不注册。
func (s *AddSignPersonExclusionService) SetUserDirectory(directory repository.TargetUserDirectory) {
	s.userDirectory = directory
}

// HasUserDirectory 报告是否已接入目标人员目录，供组装处决定要不要注册检索端点。
func (s *AddSignPersonExclusionService) HasUserDirectory() bool {
	return s.userDirectory != nil
}

// SearchCandidatePersons 按姓名检索目标人员目录，返回候选列表；未接入目录时返回空。
func (s *AddSignPersonExclusionService) SearchCandidatePersons(ctx context.Context, name string) ([]AddSignCandidate, error) {
	if s.userDirectory == nil {
		return []AddSignCandidate{}, nil
	}
	rows, err := s.userDirectory.SearchUsers(ctx, name, 20)
	if err != nil {
		return nil, err
	}
	items := make([]AddSignCandidate, 0, len(rows))
	for _, row := range rows {
		items = append(items, AddSignCandidate{
			PersonID:   row.PersonID,
			PersonName: row.Name,
			Phone:      row.Phone,
			Enabled:    row.Enabled,
		})
	}
	return items, nil
}

// Add 添加排除名单项
func (s *AddSignPersonExclusionService) Add(r *http.Request, item *model.AddSignPersonExclusionItem) error {
	return s.repo.Add(r.Context(), item)
}

// Remove 删除排除名单项
func (s *AddSignPersonExclusionService) Remove(r *http.Request, personID string) error {
	return s.repo.Remove(r.Context(), personID)
}

// List 查询排除名单
func (s *AddSignPersonExclusionService) List(r *http.Request, nameFilter string) ([]model.AddSignPersonExclusionItem, error) {
	return s.repo.List(r.Context(), nameFilter)
}

// GetExcludedPersonIDs 获取应从随机选择候选中排除的全部 personID 集合：
// 手动排除名单 + 目标平台已停用账号（停用账号必然登录失败，默认排除，无需手动维护）。
func (s *AddSignPersonExclusionService) GetExcludedPersonIDs(ctx context.Context) (map[string]bool, error) {
	excluded, err := s.repo.GetExcluded(ctx)
	if err != nil {
		return nil, err
	}
	disabled, err := s.disabledUserIDs(ctx)
	if err != nil {
		// 停用名单读取失败不阻断手动排除名单生效：仍返回手动名单，避免整条一键配置因目标库抖动失败。
		return excluded, nil
	}
	if excluded == nil {
		excluded = make(map[string]bool, len(disabled))
	}
	for id := range disabled {
		excluded[id] = true
	}
	return excluded, nil
}

// disabledUserIDs 读取目标平台停用人员 ID 集合，带 TTL 缓存；未接入目标目录时返回空集合。
func (s *AddSignPersonExclusionService) disabledUserIDs(ctx context.Context) (map[string]bool, error) {
	if s.userDirectory == nil {
		return map[string]bool{}, nil
	}
	s.disabledMu.Lock()
	defer s.disabledMu.Unlock()
	if s.disabledCache != nil && time.Now().Before(s.disabledExpiry) {
		return s.disabledCache, nil
	}
	ids, err := s.userDirectory.ListDisabledUserIDs(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	s.disabledCache = set
	s.disabledExpiry = time.Now().Add(disabledUsersTTL)
	return set, nil
}

// IsPersonExcluded 检查某个人是否在排除名单中
func (s *AddSignPersonExclusionService) IsPersonExcluded(ctx context.Context, personID string) (bool, error) {
	personID = strings.TrimSpace(personID)
	if personID == "" {
		return false, nil
	}

	excluded, err := s.repo.GetExcluded(ctx)
	if err != nil {
		return false, err
	}

	return excluded[personID], nil
}

