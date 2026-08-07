package repository

import (
	"context"
	"errors"
	"time"

	"test-auto-pro-v2/internal/model"
)

var (
	// ErrPathConfigConflict 表示并发保存的修订号与当前记录不一致，必须由用户刷新后重试。
	ErrPathConfigConflict = errors.New("路径配置修订号冲突")
	// ErrPathConfigDataInvalid 表示配置表数据无法解析或缺少必要字段。
	ErrPathConfigDataInvalid = errors.New("路径配置数据异常")
)

// PathConfigurationRepository 按路径唯一保存配置，并在同一事务内更新字段值与动作值。
type PathConfigurationRepository interface {
	// FindByPath 读取指定路径的当前配置；未保存时返回 found=false。
	FindByPath(context.Context, uint64) (model.StoredPathConfig, bool, error)
	// FindByPathAndKey 只在指定路径内按幂等键读取已保存结果，不跨路径泄露。
	FindByPathAndKey(context.Context, uint64, string) (model.StoredPathConfig, bool, error)
	// Save 在路径行锁内校验期望修订号后整份替换字段值与动作值并推进修订号。
	Save(context.Context, model.StoredPathConfig, uint64, time.Time) (model.StoredPathConfig, error)
}
