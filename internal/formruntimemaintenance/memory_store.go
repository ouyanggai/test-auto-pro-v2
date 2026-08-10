package formruntimemaintenance

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryStore 提供与 MySQL 相同的单活动任务、租约和 fencing 语义，供定向测试使用。
type MemoryStore struct {
	mu       sync.Mutex
	jobs     map[uint64]Job
	nextID   uint64
	activeID uint64
	now      func() time.Time
}

// NewMemoryStore 创建内存任务仓储。
func NewMemoryStore(now func() time.Time) *MemoryStore {
	if now == nil {
		now = time.Now
	}
	return &MemoryStore{jobs: make(map[uint64]Job), nextID: 1, now: now}
}

// Create 在内存中原子保证只有一个未完成任务。
func (s *MemoryStore) Create(_ context.Context, source SourceState) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeID != 0 {
		return Job{}, ErrJobAlreadyActive
	}
	now := s.now().UTC()
	job := Job{ID: s.nextID, Status: JobPending, Stage: StageQueued, Source: cloneSource(source), CreatedAt: now, UpdatedAt: now, RecoveryStatus: RecoveryNotRequired}
	s.nextID++
	s.activeID = job.ID
	s.jobs[job.ID] = job
	return cloneJob(job), nil
}

// Get 返回指定任务副本。
func (s *MemoryStore) Get(_ context.Context, id uint64) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return Job{}, ErrJobNotFound
	}
	return cloneJob(job), nil
}

// Latest 返回最新任务副本。
func (s *MemoryStore) Latest(_ context.Context) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.jobs) == 0 {
		return Job{}, ErrJobNotFound
	}
	ids := make([]uint64, 0, len(s.jobs))
	for id := range s.jobs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left] > ids[right] })
	return cloneJob(s.jobs[ids[0]]), nil
}

// ClaimNext 领取 pending 或租约已过期任务，并递增 fencing token 阻断旧 Worker。
func (s *MemoryStore) ClaimNext(_ context.Context, claim Claim) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[s.activeID]
	if !ok || strings.TrimSpace(claim.WorkerID) == "" || claim.LeaseDuration <= 0 {
		return Job{}, ErrJobNotReady
	}
	now := s.now().UTC()
	if job.Status == JobRunning && job.LeaseExpiresAt != nil && job.LeaseExpiresAt.After(now) {
		return Job{}, ErrJobNotReady
	}
	expiresAt := now.Add(claim.LeaseDuration)
	job.Status = JobRunning
	if job.Stage == StageQueued {
		job.Stage = StageInspect
	}
	job.LeaseOwner = claim.WorkerID
	job.LeaseExpiresAt = &expiresAt
	job.FencingToken++
	job.AttemptCount++
	job.UpdatedAt = now
	if job.StartedAt == nil {
		startedAt := now
		job.StartedAt = &startedAt
	}
	s.jobs[job.ID] = job
	return cloneJob(job), nil
}

// RenewLease 仅允许当前 fencing token 续期，过期 Worker 不能恢复写入。
func (s *MemoryStore) RenewLease(_ context.Context, renewal LeaseRenewal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[renewal.ID]
	now := s.now().UTC()
	if !ok || renewal.LeaseDuration <= 0 || job.Status != JobRunning || job.LeaseOwner != renewal.WorkerID ||
		job.FencingToken != renewal.FencingToken || job.LeaseExpiresAt == nil || !job.LeaseExpiresAt.After(now) {
		return ErrStaleLease
	}
	expiresAt := now.Add(renewal.LeaseDuration)
	job.LeaseExpiresAt = &expiresAt
	job.UpdatedAt = now
	s.jobs[job.ID] = job
	return nil
}

// UpdateProgress 持久化阶段与候选/previous 版本，供崩溃后恢复。
func (s *MemoryStore) UpdateProgress(_ context.Context, progress Progress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[progress.ID]
	if !ok {
		return ErrJobNotFound
	}
	if job.Status != JobRunning || job.LeaseOwner != progress.WorkerID || job.FencingToken != progress.FencingToken {
		return ErrStaleLease
	}
	now := s.now().UTC()
	job.Stage = progress.Stage
	job.UpdatedAt = now
	if progress.LeaseDuration > 0 {
		expiresAt := now.Add(progress.LeaseDuration)
		job.LeaseExpiresAt = &expiresAt
	}
	if progress.Candidate != "" {
		job.Candidate = progress.Candidate
	}
	if progress.Previous != "" {
		job.Previous = progress.Previous
	}
	s.jobs[job.ID] = job
	return nil
}

// Complete 终结任务并释放单活动任务保护位。
func (s *MemoryStore) Complete(_ context.Context, completion Completion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[completion.ID]
	if !ok {
		return ErrJobNotFound
	}
	if job.Status != JobRunning || job.LeaseOwner != completion.WorkerID || job.FencingToken != completion.FencingToken {
		return ErrStaleLease
	}
	now := s.now().UTC()
	if strings.TrimSpace(completion.FailureReason) == "" {
		job.Status = JobSucceeded
		job.Stage = StageCompleted
	} else {
		job.Status = JobFailed
		job.FailureReason = completion.FailureReason
		job.RecoveryStatus = completion.RecoveryStatus
		job.RecoveryMessage = completion.RecoveryMessage
	}
	job.LeaseOwner = ""
	job.LeaseExpiresAt = nil
	job.UpdatedAt = now
	job.CompletedAt = &now
	s.jobs[job.ID] = job
	s.activeID = 0
	return nil
}

// cloneSource 防止测试或 API 修改仓储内部切片。
func cloneSource(source SourceState) SourceState {
	source.ChangedFiles = append([]ChangedFile(nil), source.ChangedFiles...)
	return source
}

// cloneJob 返回隔离副本。
func cloneJob(job Job) Job {
	job.Source = cloneSource(job.Source)
	return job
}
