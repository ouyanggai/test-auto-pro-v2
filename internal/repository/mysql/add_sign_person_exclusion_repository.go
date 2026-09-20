package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"test-auto-pro-v2/internal/model"
)

// AddSignPersonExclusionRepository 处理排除名单的数据库操作
type AddSignPersonExclusionRepository struct {
	db *sql.DB
}

// NewAddSignPersonExclusionRepository 创建仓库实例
func NewAddSignPersonExclusionRepository(db *sql.DB) *AddSignPersonExclusionRepository {
	return &AddSignPersonExclusionRepository{db: db}
}

// Add 添加一条排除记录
func (r *AddSignPersonExclusionRepository) Add(ctx context.Context, item *model.AddSignPersonExclusionItem) error {
	if item.PersonID == "" || item.PersonName == "" {
		return errors.New("personId 和 personName 不能为空")
	}

	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now

	query := `
		INSERT INTO add_sign_person_exclusion_list (person_id, person_name, person_phone, reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			person_name = VALUES(person_name),
			person_phone = VALUES(person_phone),
			reason = VALUES(reason),
			updated_at = VALUES(updated_at)
	`

	result, err := r.db.ExecContext(ctx, query, item.PersonID, item.PersonName, item.PersonPhone, item.Reason, now, now)
	if err != nil {
		return fmt.Errorf("添加排除名单失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil && id > 0 {
		item.ID = uint64(id)
	}

	return nil
}

// Remove 删除一条排除记录
func (r *AddSignPersonExclusionRepository) Remove(ctx context.Context, personID string) error {
	if personID == "" {
		return errors.New("personId 不能为空")
	}

	result, err := r.db.ExecContext(ctx, `DELETE FROM add_sign_person_exclusion_list WHERE person_id = ?`, personID)
	if err != nil {
		return fmt.Errorf("删除排除名单失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("检查删除结果失败: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("排除名单中未找到该人员")
	}

	return nil
}

// List 查询排除名单
func (r *AddSignPersonExclusionRepository) List(ctx context.Context, nameFilter string) ([]model.AddSignPersonExclusionItem, error) {
	query := `SELECT id, person_id, person_name, person_phone, reason, created_at, updated_at FROM add_sign_person_exclusion_list`
	args := []interface{}{}

	if nameFilter != "" {
		query += ` WHERE person_name LIKE ?`
		args = append(args, "%"+nameFilter+"%")
	}

	query += ` ORDER BY updated_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询排除名单失败: %w", err)
	}
	defer rows.Close()

	items := []model.AddSignPersonExclusionItem{}
	for rows.Next() {
		var item model.AddSignPersonExclusionItem
		if err := rows.Scan(&item.ID, &item.PersonID, &item.PersonName, &item.PersonPhone, &item.Reason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描行失败: %w", err)
		}
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("行迭代失败: %w", err)
	}

	return items, nil
}

// GetExcluded 获取所有排除的 personID 集合（用于过滤候选）
func (r *AddSignPersonExclusionRepository) GetExcluded(ctx context.Context) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT person_id FROM add_sign_person_exclusion_list`)
	if err != nil {
		return nil, fmt.Errorf("查询排除 ID 集合失败: %w", err)
	}
	defer rows.Close()

	excluded := make(map[string]bool)
	for rows.Next() {
		var personID string
		if err := rows.Scan(&personID); err != nil {
			return nil, fmt.Errorf("扫描 personId 失败: %w", err)
		}
		excluded[personID] = true
	}

	return excluded, rows.Err()
}
