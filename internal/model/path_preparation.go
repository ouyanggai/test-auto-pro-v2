package model

import "time"

// PathPreparationJob 是批量路径准备任务的持久化进度快照。
type PathPreparationJob struct {
	ID              string     `json:"id"`
	PlanID          uint64     `json:"-"`
	Status          string     `json:"status"`
	Total           int        `json:"total"`
	Processed       int        `json:"processed"`
	NodeConfigured  int        `json:"nodeConfigured"`
	DataGenerated   int        `json:"dataGenerated"`
	NeedsAttention  int        `json:"needsAttention"`
	Failed          int        `json:"failed"`
	PreservedManual int        `json:"preservedManual"`
	Error           string     `json:"error,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

// PathPreparationItem 是任务内单条路径的检查点与用户可读结果。
type PathPreparationItem struct {
	ID              uint64 `json:"id"`
	JobID           string `json:"-"`
	PathID          uint64 `json:"pathId"`
	SequenceNo      uint   `json:"sequenceNo"`
	PathName        string `json:"pathName"`
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	NodeConfigured  bool   `json:"nodeConfigured"`
	DataGenerated   bool   `json:"dataGenerated"`
	NeedsAttention  bool   `json:"needsAttention"`
	PreservedManual bool   `json:"preservedManual"`
}

// PathPreparationItemResult 是 Worker 对单条路径的终态提交，不允许携带技术错误原文。
type PathPreparationItemResult struct {
	Status          string
	Reason          string
	NodeConfigured  bool
	DataGenerated   bool
	NeedsAttention  bool
	PreservedManual bool
}

// PathPreparationItemPage 是按明细 ID 游标分页的任务结果。
type PathPreparationItemPage struct {
	Items      []PathPreparationItem `json:"items"`
	NextCursor uint64                `json:"nextCursor,omitempty"`
}
