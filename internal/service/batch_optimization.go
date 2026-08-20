package service

import (
	"sync"
	"time"
)

// BatchProgressNotifier 批量任务进度通知器，支持实时推送。
type BatchProgressNotifier struct {
	mu        sync.RWMutex
	listeners map[uint64][]chan BatchProgressEvent
}

// BatchProgressEvent 批量任务进度事件。
type BatchProgressEvent struct {
	JobID          uint64    `json:"jobId"`
	Type           string    `json:"type"` // started/progress/item_complete/completed/failed
	TotalItems     int       `json:"totalItems,omitempty"`
	CompletedItems int       `json:"completedItems,omitempty"`
	FailedItems    int       `json:"failedItems,omitempty"`
	CurrentItem    string    `json:"currentItem,omitempty"`
	Message        string    `json:"message,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// NewBatchProgressNotifier 创建进度通知器。
func NewBatchProgressNotifier() *BatchProgressNotifier {
	return &BatchProgressNotifier{
		listeners: make(map[uint64][]chan BatchProgressEvent),
	}
}

// Subscribe 订阅批量任务进度。
func (n *BatchProgressNotifier) Subscribe(jobID uint64) <-chan BatchProgressEvent {
	n.mu.Lock()
	defer n.mu.Unlock()

	ch := make(chan BatchProgressEvent, 100)
	n.listeners[jobID] = append(n.listeners[jobID], ch)
	return ch
}

// Unsubscribe 取消订阅。
func (n *BatchProgressNotifier) Unsubscribe(jobID uint64, ch chan BatchProgressEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()

	listeners := n.listeners[jobID]
	for i, listener := range listeners {
		if listener == ch {
			n.listeners[jobID] = append(listeners[:i], listeners[i+1:]...)
			close(listener)
			break
		}
	}

	if len(n.listeners[jobID]) == 0 {
		delete(n.listeners, jobID)
	}
}

// Notify 发送进度事件给所有订阅者。
func (n *BatchProgressNotifier) Notify(event BatchProgressEvent) {
	n.mu.RLock()
	listeners := n.listeners[event.JobID]
	n.mu.RUnlock()

	for _, ch := range listeners {
		select {
		case ch <- event:
		default:
			// 如果通道已满，跳过此事件
		}
	}
}

// Close 关闭所有订阅通道。
func (n *BatchProgressNotifier) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, listeners := range n.listeners {
		for _, ch := range listeners {
			close(ch)
		}
	}
	n.listeners = make(map[uint64][]chan BatchProgressEvent)
}

// BatchFailureAggregator 批量任务失败原因聚合器。
type BatchFailureAggregator struct {
	failures map[string]*FailureGroup
	mu       sync.RWMutex
}

// FailureGroup 失败原因分组。
type FailureGroup struct {
	Reason string   `json:"reason"`
	Count  int      `json:"count"`
	Items  []string `json:"items"`
}

// NewBatchFailureAggregator 创建失败原因聚合器。
func NewBatchFailureAggregator() *BatchFailureAggregator {
	return &BatchFailureAggregator{
		failures: make(map[string]*FailureGroup),
	}
}

// Add 添加失败记录。
func (a *BatchFailureAggregator) Add(item, reason string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if group, exists := a.failures[reason]; exists {
		group.Count++
		if len(group.Items) < 10 { // 只保留前10个示例
			group.Items = append(group.Items, item)
		}
	} else {
		a.failures[reason] = &FailureGroup{
			Reason: reason,
			Count:  1,
			Items:  []string{item},
		}
	}
}

// GetGroups 获取所有失败分组，按数量降序排列。
func (a *BatchFailureAggregator) GetGroups() []*FailureGroup {
	a.mu.RLock()
	defer a.mu.RUnlock()

	groups := make([]*FailureGroup, 0, len(a.failures))
	for _, group := range a.failures {
		groups = append(groups, group)
	}

	// 按数量降序排序
	for i := 0; i < len(groups)-1; i++ {
		for j := i + 1; j < len(groups); j++ {
			if groups[j].Count > groups[i].Count {
				groups[i], groups[j] = groups[j], groups[i]
			}
		}
	}

	return groups
}

// Reset 重置聚合器。
func (a *BatchFailureAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failures = make(map[string]*FailureGroup)
}
