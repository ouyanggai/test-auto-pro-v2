package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"test-auto-pro-v2/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

// main 查询目标库里最近 6 条实例（只读核对写后事实）。
func main() {
	cfg := config.LoadTargetBizDBConfig()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=false&charset=utf8mb4&loc=Local&timeout=5s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)
	db, err := sql.Open("mysql", dsn)
	if err != nil { panic(err) }
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx,
		`SELECT i.id, i.name, i.status, i.create_date FROM t_flow_instance i ORDER BY i.create_date DESC LIMIT 6`)
	if err != nil { panic(err) }
	defer rows.Close()
	for rows.Next() {
		var id, name, status, created string
		_ = rows.Scan(&id, &name, &status, &created)
		fmt.Printf("%s | %s | %s | %s\n", created, id, name, status)
	}
}
