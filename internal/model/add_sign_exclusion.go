package model

import "time"

// AddSignPersonExclusionItem 代表排除名单中的一条记录
type AddSignPersonExclusionItem struct {
	ID          uint64    `db:"id" json:"id"`
	PersonID    string    `db:"person_id" json:"personId"`
	PersonName  string    `db:"person_name" json:"personName"`
	PersonPhone string    `db:"person_phone" json:"personPhone"`
	Reason      string    `db:"reason" json:"reason"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

// AddSignPersonExclusionRequest 是排除名单的 API 请求
type AddSignPersonExclusionRequest struct {
	PersonID    string `json:"personId"`
	PersonName  string `json:"personName"`
	PersonPhone string `json:"personPhone,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// AddSignPersonExclusionResponse 是排除名单的 API 响应
type AddSignPersonExclusionResponse struct {
	Items []AddSignPersonExclusionItem `json:"items"`
	Total int64                        `json:"total"`
}
