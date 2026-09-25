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

	validMu     sync.Mutex
	validCache  map[string]bool
	validExpiry time.Time
}
// NewAddSignPersonExclusionService 创建服务实例
func NewAddSignPersonExclusionService(repo *mysql.AddSignPersonExclusionRepository) *AddSignPersonExclusionService {
	return &AddSignPersonExclusionService{repo: repo}
}

// SetUserDirectory 注入目标人员目录只读来源，用于按姓名检索候选；未注入时检索端点不注册。
func (s *AddSignPersonExclusionService) SetUserDirectory(directory repository.TargetUserDirectory) {
	s.userDirectory = directory
}

// ValidPlatformUserSet 返回当前租户具备平台角色的有效用户白名单集合；未接入目标目录或白名单不可用时返回 nil。
func (s *AddSignPersonExclusionService) ValidPlatformUserSet(ctx context.Context) map[string]bool {
	if s.userDirectory == nil {
		return nil
	}
	s.validMu.Lock()
	defer s.validMu.Unlock()
	if s.validCache != nil && time.Now().Before(s.validExpiry) {
		return s.validCache
	}
	validIDs, err := s.userDirectory.ListValidPlatformUserIDs(ctx)
	if err != nil || len(validIDs) == 0 {
		return nil
	}
	set := make(map[string]bool, len(validIDs))
	for _, id := range validIDs {
		set[id] = true
	}
	s.validCache = set
	s.validExpiry = time.Now().Add(disabledUsersTTL)
	return set
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

// AutoExcludedPerson 是系统默认排除的人员及原因，供列表展示（区别于手动名单）。
// Reason: disabled（账号已停用）| no_platform_role（未分配当前平台权限）。
type AutoExcludedPerson struct {
	PersonID   string
	PersonName string
	Phone      string
	Reason     string
}

// ListAutoExcluded 返回系统默认排除的人员（停用 + 无平台角色），供人员排除页展示；未接入目标目录时返回空。
func (s *AddSignPersonExclusionService) ListAutoExcluded(r *http.Request, nameFilter string) ([]AutoExcludedPerson, error) {
	if s.userDirectory == nil {
		return []AutoExcludedPerson{}, nil
	}
	rows, err := s.userDirectory.ListAutoExcludedUsers(r.Context(), strings.TrimSpace(nameFilter))
	if err != nil {
		return nil, err
	}
	items := make([]AutoExcludedPerson, 0, len(rows))
	for _, row := range rows {
		items = append(items, AutoExcludedPerson{
			PersonID:   row.PersonID,
			PersonName: row.Name,
			Phone:      row.Phone,
			Reason:     row.Reason,
		})
	}
	return items, nil
}

// GetExcludedPersonIDs 获取应从随机选择候选中排除的全部 personID 集合：
// 手动排除名单 + 目标平台不可用账号（已停用、或已启用但当前平台无角色——两类登录都必然失败，
// 默认排除，无需手动维护）。
func (s *AddSignPersonExclusionService) GetExcludedPersonIDs(ctx context.Context) (map[string]bool, error) {
	excluded, err := s.repo.GetExcluded(ctx)
	if err != nil {
		return nil, err
	}
	unusable, err := s.unusableUserIDs(ctx)
	if err != nil {
		// 不可用名单读取失败不阻断手动排除名单生效：仍返回手动名单，避免整条一键配置因目标库抖动失败。
		return excluded, nil
	}
	if excluded == nil {
		excluded = make(map[string]bool, len(unusable))
	}
	for id := range unusable {
		excluded[id] = true
	}
	return excluded, nil
}

// unusableUserIDs 读取目标平台「登录必然失败」的人员 ID 集合，带 TTL 缓存；未接入目标目录时返回空集合。
// 优先使用白名单反向排除：读取本租户具备当前平台有效角色的白名单用户 ID（ListValidPlatformUserIDs）；
// 若白名单可用，凡是不在白名单中的人员（包含停用、无权限、或已从用户中心物理删除的幽灵人员）一律进入排除集。
// 若白名单方法未实现或返回空，回退到历史的停用+无角色黑名单并集。
func (s *AddSignPersonExclusionService) unusableUserIDs(ctx context.Context) (map[string]bool, error) {
	if s.userDirectory == nil {
		return map[string]bool{}, nil
	}
	s.disabledMu.Lock()
	defer s.disabledMu.Unlock()
	if s.disabledCache != nil && time.Now().Before(s.disabledExpiry) {
		return s.disabledCache, nil
	}
	validIDs, err := s.userDirectory.ListValidPlatformUserIDs(ctx)
	if err == nil && len(validIDs) > 0 {
		validSet := make(map[string]bool, len(validIDs))
		for _, id := range validIDs {
			validSet[id] = true
		}
		// 结合停用与无角色人员（供明确标记）并构建兜底黑名单标记
		disabled, _ := s.userDirectory.ListDisabledUserIDs(ctx)
		noRole, _ := s.userDirectory.ListNoPlatformRoleUserIDs(ctx)
		set := make(map[string]bool, len(disabled)+len(noRole))
		for _, id := range disabled {
			set[id] = true
		}
		for _, id := range noRole {
			set[id] = true
		}
		// 保存白名单到排除服务以便针对外部候选人快速过滤
		s.validMu.Lock()
		s.validCache = validSet
		s.validExpiry = time.Now().Add(disabledUsersTTL)
		s.validMu.Unlock()

		s.disabledCache = set
		s.disabledExpiry = time.Now().Add(disabledUsersTTL)
		return set, nil
	}

	disabled, err := s.userDirectory.ListDisabledUserIDs(ctx)
	if err != nil {
		return nil, err
	}
	noRole, err := s.userDirectory.ListNoPlatformRoleUserIDs(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(disabled)+len(noRole))
	for _, id := range disabled {
		set[id] = true
	}
	for _, id := range noRole {
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

