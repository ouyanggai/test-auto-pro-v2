package mysql

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"test-auto-pro-v2/internal/config"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var (
	ErrConnection = errors.New("计划数据库连接失败")
	ErrMigration  = errors.New("计划数据库迁移失败")
)

type Database struct {
	DB *sql.DB
}

func OpenAndMigrate(ctx context.Context, cfg config.PlanDBConfig) (*Database, error) {
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		return nil, &config.MissingPlanDBConfigError{Names: missing}
	}
	if !config.ValidDatabaseName(cfg.Name) {
		return nil, &config.MissingPlanDBConfigError{Names: []string{"PLAN_DB_NAME"}}
	}

	server, err := sql.Open("mysql", dsn(cfg, ""))
	if err != nil {
		return nil, ErrConnection
	}
	server.SetConnMaxLifetime(3 * time.Minute)
	server.SetMaxOpenConns(2)
	server.SetMaxIdleConns(1)
	defer server.Close()
	if err := server.PingContext(ctx); err != nil {
		return nil, ErrConnection
	}
	createDatabase := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci",
		cfg.Name,
	)
	if _, err := server.ExecContext(ctx, createDatabase); err != nil {
		return nil, ErrConnection
	}

	db, err := sql.Open("mysql", dsn(cfg, cfg.Name))
	if err != nil {
		return nil, ErrConnection
	}
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetConnMaxIdleTime(time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, ErrConnection
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, ErrMigration
	}
	return &Database{DB: db}, nil
}

func (d *Database) Close() error {
	if d == nil || d.DB == nil {
		return nil
	}
	return d.DB.Close()
}

func dsn(cfg config.PlanDBConfig, databaseName string) string {
	mysqlConfig := driver.NewConfig()
	mysqlConfig.User = cfg.User
	mysqlConfig.Passwd = cfg.Password
	mysqlConfig.Net = "tcp"
	mysqlConfig.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	mysqlConfig.DBName = databaseName
	mysqlConfig.ParseTime = true
	mysqlConfig.Loc = time.UTC
	mysqlConfig.Collation = "utf8mb4_0900_ai_ci"
	mysqlConfig.Timeout = 5 * time.Second
	mysqlConfig.ReadTimeout = 10 * time.Second
	mysqlConfig.WriteTimeout = 10 * time.Second
	mysqlConfig.Params = map[string]string{"time_zone": "'+00:00'"}
	return mysqlConfig.FormatDSN()
}

func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(128) NOT NULL PRIMARY KEY,
  applied_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`); err != nil {
		return err
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var applied int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", entry.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied != 0 {
			continue
		}
		statement, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, string(statement)); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", entry.Name(), time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}
