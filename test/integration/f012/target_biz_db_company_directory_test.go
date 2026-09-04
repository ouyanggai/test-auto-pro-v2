package f012_test

import (
	"context"
	"database/sql"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"

	"github.com/go-sql-driver/mysql"
)

// TestTargetBizDBCompanyDirectory 用真实目标用户中心库验证公司目录查询的闭环：
// 从真实候选行的发起公司出发，按名称取 ID、再按 ID 取回名称，两个方向必须一致；
// 未知的 ID 与名称都必须返回未找到。用例不绑定任何特定公司数据。
// 未配置 TARGET_DB_* 时跳过：该连接是可选只读加速路径。
func TestTargetBizDBCompanyDirectory(t *testing.T) {
	bizConfig := config.LoadTargetBizDBConfig()
	if !bizConfig.Enabled() {
		t.Skip("未配置目标业务库只读连接，跳过公司目录集成用例")
	}
	targetConfig := config.LoadTargetConfig()
	if strings.TrimSpace(targetConfig.CustomerCode) == "" {
		t.Skip("未配置目标客户编码，无法执行租户内公司查询")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := planmysql.NewTargetHistoryRepository(ctx, bizConfig, targetConfig.CustomerCode)
	if err != nil {
		t.Fatalf("打开目标业务库只读连接失败：%v", err)
	}
	defer store.Close()

	candidates, _, err := store.TargetHistoryCandidates(ctx, repository.TargetHistoryCandidateFilter{FlowName: "员工请假单（智慧斯能）"}, 1, 20)
	if err != nil {
		t.Fatalf("读取候选失败：%v", err)
	}
	knownID := ""
	knownName := ""
	for _, row := range candidates {
		companyName := strings.TrimSpace(row.CompanyName)
		if companyName == "" {
			continue
		}
		ids, err := store.CompanyIDByName(ctx, companyName)
		if err != nil {
			t.Fatalf("按名称查询公司失败：%v", err)
		}
		if len(ids) == 0 {
			continue
		}
		name, found, err := store.CompanyNameByID(ctx, ids[0])
		if err != nil || !found {
			t.Fatalf("按 ID 查询真实公司失败：found=%v err=%v", found, err)
		}
		if name != companyName {
			t.Fatalf("公司名称与 ID 互查不一致：期望=%s 实际=%s", companyName, name)
		}
		knownID, knownName = ids[0], companyName
		break
	}
	if knownID == "" {
		t.Skip("目标业务库候选行没有可解析的公司身份，无法验证目录查询")
	}
	// 闭环后再确认一次双向一致，锁定"ID→名称"与"名称→ID"返回同一身份。
	if name, found, err := store.CompanyNameByID(ctx, knownID); err != nil || !found || name != knownName {
		t.Fatalf("公司目录双向查询不一致：found=%v name=%s err=%v", found, name, err)
	}
}

// TestTargetBizDBCompanyDirectoryCoversProjectCompanies 在一次性临时库上用真实 MySQL 锁定目录的来源并集：
// 目标公司下拉的真实来源是 t_company 主数据与 t_project_company 项目公司的并集，请款单历史付款单位
// （广西润兴、临猗县斯能等）都是项目公司，只查主数据会把它们误判为不存在并永远无法同步真实 ID。
// 同时锁定同名双表歧义必须原样返回多条 ID，由调用方拒绝解析，以及已删除行不可解析。
func TestTargetBizDBCompanyDirectoryCoversProjectCompanies(t *testing.T) {
	planConfig := config.LoadPlanDBConfig()
	if missing := planConfig.MissingRequired(); len(missing) != 0 {
		t.Skipf("未配置测试数据库，跳过公司目录来源并集用例：%v", missing)
	}
	// 公司目录查询要求工作流库三张身份表存在（verifyTables 的租户边界校验），
	// 这里用最小 DDL 建一个一次性临时库，同时充当主数据库与用户中心库。
	dbName := newHistoryTemporaryDatabaseName(t)
	server := openCompanyDirectoryServer(t, planConfig)
	t.Cleanup(func() { dropHistoryTemporaryDatabase(t, planConfig) })
	// dropHistoryTemporaryDatabase 按 planConfig.Name 清理，这里复用同一个名字边界。
	planConfig.Name = dbName
	if _, err := server.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("创建公司目录临时库失败：%v", err)
	}
	db, err := sql.Open("mysql", companyDirectoryDSN(planConfig, dbName))
	if err != nil {
		t.Fatalf("连接公司目录临时库失败：%v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	seedStatements := []string{
		`CREATE TABLE t_flow_instance (id varchar(32) NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE t_flow_proxy (id varchar(32) NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE t_form_proxy (id varchar(32) NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE t_company (
  id varchar(32) NOT NULL PRIMARY KEY,
  name varchar(255) NOT NULL DEFAULT '',
  is_delete tinyint NOT NULL DEFAULT 0,
  customer_code varchar(32) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE t_project_company (
  id varchar(32) NOT NULL PRIMARY KEY,
  name varchar(255) NOT NULL DEFAULT '',
  company_id varchar(32) NOT NULL DEFAULT '',
  status tinyint NOT NULL DEFAULT 1,
  is_delete tinyint NOT NULL DEFAULT 0,
  customer_code varchar(32) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`INSERT INTO t_company (id, name, is_delete, customer_code) VALUES
  ('company-main', '主数据甲公司', 0, 'tenant-a'),
  ('company-ambiguous', '同名公司', 0, 'tenant-a'),
  ('company-deleted', '已删除公司', 1, 'tenant-a')`,
		`INSERT INTO t_project_company (id, name, company_id, status, is_delete, customer_code) VALUES
  ('project-linying', '项目公司乙', 'company-main', 1, 0, 'tenant-a'),
  ('project-ambiguous', '同名公司', 'company-main', 1, 0, 'tenant-a'),
  ('project-deleted', '已删除项目公司', 'company-main', 1, 1, 'tenant-a')`,
	}
	for _, statement := range seedStatements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("准备公司目录临时表失败：%v", err)
		}
	}

	bizConfig := config.TargetBizDBConfig{
		Host: planConfig.Host, Port: planConfig.Port, User: planConfig.User, Password: planConfig.Password,
		Name: dbName, UserCenterName: dbName,
	}
	store, err := planmysql.NewTargetHistoryRepository(context.Background(), bizConfig, "tenant-a")
	if err != nil {
		t.Fatalf("打开临时公司目录连接失败：%v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 项目公司必须能按名称解析出真实 ID：这是请款单付款单位同步的主路径。
	ids, err := store.CompanyIDByName(ctx, "项目公司乙")
	if err != nil || len(ids) != 1 || ids[0] != "project-linying" {
		t.Fatalf("项目公司没有被目录解析：ids=%v err=%v", ids, err)
	}
	name, found, err := store.CompanyNameByID(ctx, "project-linying")
	if err != nil || !found || name != "项目公司乙" {
		t.Fatalf("项目公司 ID 反查名称失败：found=%v name=%s err=%v", found, name, err)
	}
	// 主数据公司同样必须可解析，证明并集两个方向都生效。
	if name, found, err := store.CompanyNameByID(ctx, "company-main"); err != nil || !found || name != "主数据甲公司" {
		t.Fatalf("主数据公司查询失败：found=%v name=%s err=%v", found, name, err)
	}
	// 已删除行与未知身份都必须返回未找到，禁止把历史删除数据当成可用公司。
	if _, found, err := store.CompanyNameByID(ctx, "company-deleted"); err != nil || found {
		t.Fatalf("已删除公司应返回未找到：found=%v err=%v", found, err)
	}
	if _, found, err := store.CompanyNameByID(ctx, "project-deleted"); err != nil || found {
		t.Fatalf("已删除项目公司应返回未找到：found=%v err=%v", found, err)
	}
	if _, found, err := store.CompanyNameByID(ctx, "no-such-company"); err != nil || found {
		t.Fatalf("未知公司应返回未找到：found=%v err=%v", found, err)
	}
	// 同名双表歧义必须原样返回多条，由数据工作区拒绝解析而不是随机挑选。
	ambiguous, err := store.CompanyIDByName(ctx, "同名公司")
	if err != nil || len(ambiguous) != 2 {
		t.Fatalf("同名歧义没有返回多条 ID：ids=%v err=%v", ambiguous, err)
	}
}

// openCompanyDirectoryServer 打开仅用于建库和清理的服务器级连接。
func openCompanyDirectoryServer(t *testing.T, cfg config.PlanDBConfig) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", companyDirectoryDSN(cfg, ""))
	if err != nil {
		t.Fatalf("连接测试数据库服务器失败：%v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("测试数据库服务器不可达：%v", err)
	}
	return db
}

// companyDirectoryDSN 组装公司目录用例的 MySQL 连接串；databaseName 为空时连接服务器本身。
func companyDirectoryDSN(cfg config.PlanDBConfig, databaseName string) string {
	mysqlConfig := mysql.NewConfig()
	mysqlConfig.User = cfg.User
	mysqlConfig.Passwd = cfg.Password
	mysqlConfig.Net = "tcp"
	mysqlConfig.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	mysqlConfig.DBName = databaseName
	mysqlConfig.ParseTime = true
	mysqlConfig.Loc = time.UTC
	mysqlConfig.Timeout = 5 * time.Second
	return mysqlConfig.FormatDSN()
}
