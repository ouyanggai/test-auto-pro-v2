package formruntimemaintenance

import (
	"context"
	"fmt"
	"time"
)

// Service 为固定来源状态、任务创建、任务详情和日志提供只读/单按钮 API。
type Service struct {
	inspector SourceInspector
	store     Store
	logs      LogStore
}

// NewService 创建维护服务。
func NewService(inspector SourceInspector, store Store, logs LogStore) *Service {
	return &Service{inspector: inspector, store: store, logs: logs}
}

// InspectSource 返回固定来源状态，不改变来源或运行时。
func (s *Service) InspectSource(ctx context.Context) (SourceState, error) {
	return s.inspector.Inspect(ctx)
}

// CreateJob 核对来源干净后创建唯一活动任务；请求不能指定路径、分支、HEAD 或命令。
func (s *Service) CreateJob(ctx context.Context) (Job, error) {
	source, err := s.inspector.Inspect(ctx)
	if err != nil {
		return Job{}, err
	}
	if source.Dirty {
		return Job{}, fmt.Errorf("%w: 来源工作树必须干净", ErrSourceInvalid)
	}
	return s.store.Create(ctx, source)
}

// GetJob 返回指定任务。
func (s *Service) GetJob(ctx context.Context, id uint64) (Job, error) {
	return s.store.Get(ctx, id)
}

// LatestJob 返回最新任务。
func (s *Service) LatestJob(ctx context.Context) (Job, error) {
	return s.store.Latest(ctx)
}

// GetJobLog 返回有界日志尾部。
func (s *Service) GetJobLog(ctx context.Context, id uint64) (Log, error) {
	return s.logs.Read(ctx, id)
}

// Processor 是 Runner 所需的单任务处理边界。
type Processor interface {
	ProcessNext(context.Context) (bool, error)
}

// Runner 轮询领取任务；持久化任务本身负责崩溃恢复。
type Runner struct {
	processor    Processor
	pollInterval time.Duration
	onError      func(error)
}

// NewRunner 创建维护任务轮询器。
func NewRunner(processor Processor, pollInterval time.Duration) *Runner {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	return &Runner{processor: processor, pollInterval: pollInterval}
}

// WithErrorHandler 注册非致命单任务错误记录器。
func (r *Runner) WithErrorHandler(handler func(error)) *Runner {
	r.onError = handler
	return r
}

// Run 持续领取任务，直到服务上下文结束。
func (r *Runner) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		processed, err := r.processor.ProcessNext(ctx)
		if err != nil && r.onError != nil {
			r.onError(err)
		}
		if err == nil && processed {
			continue
		}
		timer := time.NewTimer(r.pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}
