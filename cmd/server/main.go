package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/formruntimemaintenance"
	"test-auto-pro-v2/internal/logging"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// main 在监听端口前完成向前迁移并组装目标读取、计划、流程图和路径服务。
func main() {
	startupContext, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	planDatabase, err := planmysql.OpenAndMigrate(startupContext, config.LoadPlanDBConfig())
	cancelStartup()
	if err != nil {
		log.Fatal(err)
	}
	defer planDatabase.Close()

	targetConfig := config.LoadTargetConfig()
	targetReader := service.NewTargetReadService(targetConfig)
	// 目标业务库只读连接可选：配置后基础表单数据候选走一次联表查询，未配置时回落到目标只读 API。
	if bizDBConfig := config.LoadTargetBizDBConfig(); bizDBConfig.Enabled() {
		bizContext, cancelBiz := context.WithTimeout(context.Background(), 10*time.Second)
		candidateStore, bizErr := planmysql.NewTargetHistoryRepository(bizContext, bizDBConfig, targetConfig.CustomerCode)
		cancelBiz()
		if bizErr != nil {
			log.Printf("目标业务库只读候选查询未启用：%v", bizErr)
		} else {
			defer candidateStore.Close()
			targetReader.SetHistoryCandidateStore(candidateStore)
		}
	}
	planService := service.NewPlanService(planmysql.NewPlanRepository(planDatabase.DB))
	flowGraphService := service.NewFlowGraphService(planService, targetReader, analyzer.NewFlowGraphAnalyzer())
	pathRepository := planmysql.NewExecutionPathRepository(planDatabase.DB)
	executionPathService := service.NewExecutionPathService(
		planService, flowGraphService, analyzer.NewExecutionPathAnalyzer(), pathRepository,
	)
	pathRequirementService := service.NewPathRequirementService(
		planService, targetReader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(),
		analyzer.NewPathRequirementAnalyzer(), pathRepository,
	)
	pathConfigService := service.NewPathConfigService(
		planService, targetReader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(),
		analyzer.NewPathConfigAnalyzer(), pathRepository,
	)
	workspaceRoot := os.Getenv("TEST_AUTO_PRO_WORKSPACE_ROOT")
	if workspaceRoot == "" {
		workspaceRoot, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	}
	workspaceRoot, err = filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		log.Fatal(err)
	}
	// F-013 日志底座：目标请求日志、程序日志与程序错误日志共用同一份目录路由。
	logRouter := logging.NewRouter(logging.Root(workspaceRoot), time.Now)
	appLogger := logging.NewLogger(logRouter, time.Now)
	targetReader.SetNetworkLogger(appLogger)
	// 日志归属解析器按路由里的计划与执行路径 ID 读取真实显示名，让业务日志落进对应的计划目录。
	logScopeResolver := service.NewLogScopeService(planmysql.NewPlanRepository(planDatabase.DB), pathRepository, time.Now)
	if removed := logging.CleanupExpired(logRouter.Root(), logging.RetentionDays(), time.Now()); len(removed) > 0 {
		log.Printf("已清理过期日志目录 %d 个", len(removed))
		appLogger.Info(logging.Scope{}, "已清理过期日志目录", logging.Field{Key: "removed", Value: strconv.Itoa(len(removed))})
	}

	historyDataService := service.NewHistoryDataService(planService, pathRepository, targetReader, planmysql.NewHistoryReplayStore(planDatabase.DB))
	historyReplayService := service.NewHistoryReplayService(planService, pathRepository, targetReader, planmysql.NewHistoryReplayStore(planDatabase.DB))
	historyWorkspaceStore := planmysql.NewHistoryReplayRepository(planDatabase.DB)
	pathConfigService.SetHistoryWorkspaceStores(historyWorkspaceStore, historyWorkspaceStore)
	// 一键配置在业务数据回放后按真实门禁补齐节点动作配置，让节点状态与列表一致。
	historyReplayService.SetActionConfigurator(pathConfigService)
	if err := historyReplayService.Recover(context.Background()); err != nil {
		log.Printf("恢复批量准备任务失败：%v", err)
	}
	manifestPath := filepath.Join(workspaceRoot, "form-runtime", "sync-manifest.json")
	manifest, err := formruntimemaintenance.LoadManifest(workspaceRoot, manifestPath)
	if err != nil {
		log.Fatal(err)
	}
	inspector, err := formruntimemaintenance.NewGitSourceInspector(workspaceRoot, manifest, time.Now)
	if err != nil {
		log.Fatal(err)
	}
	syncer, err := formruntimemaintenance.NewSyncer(workspaceRoot, inspector.SourceRoot(), manifest)
	if err != nil {
		log.Fatal(err)
	}
	maintenanceRoot := filepath.Join(workspaceRoot, ".runtime", "form-runtime-maintenance")
	maintenanceLogs, err := formruntimemaintenance.NewFileLogStore(filepath.Join(maintenanceRoot, "logs"), 512*1024)
	if err != nil {
		log.Fatal(err)
	}
	healthURL := os.Getenv("FORM_RUNTIME_HEALTH_URL")
	if healthURL == "" {
		// 开发环境也必须核对真正运行在 19001 的服务，不能把静态目录替换冒充重启成功。
		healthURL = "http://127.0.0.1:19001/form-runtime/runtime-health.json"
	}
	operator, err := formruntimemaintenance.NewPnpmOperator(formruntimemaintenance.PnpmOperatorOptions{
		WorkspaceRoot: workspaceRoot,
		RuntimeDir:    filepath.Join(workspaceRoot, "form-runtime"),
		LiveSourceDir: filepath.Join(workspaceRoot, "form-runtime", "runtime-source"),
		LiveDir:       filepath.Join(workspaceRoot, "web", "dist", "form-runtime"),
		StateRoot:     maintenanceRoot,
		HealthURL:     healthURL,
	}, syncer, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	maintenanceStore := formruntimemaintenance.NewMySQLStore(planDatabase.DB, time.Now)
	maintenanceService := formruntimemaintenance.NewService(inspector, maintenanceStore, maintenanceLogs)
	maintenancePipeline := formruntimemaintenance.NewPipeline(maintenanceStore, inspector, operator, maintenanceLogs, formruntimemaintenance.WorkerOptions{
		WorkerID: "server-maintenance-worker", LeaseDuration: 2 * time.Minute, RenewalInterval: 30 * time.Second,
	})
	// F-015 成功断言与运行准备：只读聚合，真实结构读一次，断言一次取齐，路径配置按路径读数据库。
	runReadinessService := service.NewRunReadinessService(
		planService, pathRepository, flowGraphService,
		planmysql.NewPathSuccessAssertionRepository(planDatabase.DB), historyWorkspaceStore,
		analyzer.NewExecutionPathAnalyzer(), time.Now,
	)
	server := &http.Server{
		Addr: config.ServerAddress(),
		Handler: api.WithRequestLogging(
			api.NewHandlerWithRunReadiness(
				api.NewHandlerWithHistoryReplayAndDataServices(targetReader, planService, flowGraphService, executionPathService, pathRequirementService, pathConfigService, pathConfigService, maintenanceService, historyDataService, historyReplayService),
				runReadinessService,
			),
			appLogger,
			logScopeResolver,
		),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("后端服务监听 %s", server.Addr)
	// 启动、监听与停止属于无业务归属的系统事件，用零值作用域落进 logs/application/<日期>/。
	appLogger.Info(logging.Scope{}, "后端服务开始监听", logging.Field{Key: "address", Value: server.Addr})
	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownContext.Done():
				return
			case <-ticker.C:
				logging.CleanupExpired(logRouter.Root(), logging.RetentionDays(), time.Now())
			}
		}
	}()
	maintenanceRunner := formruntimemaintenance.NewRunner(maintenancePipeline, time.Second).WithErrorHandler(func(runErr error) {
		log.Printf("表单运行时维护任务失败: %v", runErr)
	})
	go func() {
		if runErr := maintenanceRunner.Run(shutdownContext); runErr != nil {
			log.Printf("表单运行时维护 Worker 停止: %v", runErr)
		}
	}()
	serverError := make(chan error, 1)
	go func() {
		serverError <- server.ListenAndServe()
	}()
	select {
	case err := <-serverError:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-shutdownContext.Done():
		gracefulContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(gracefulContext); err != nil {
			log.Printf("后端服务停止失败")
			appLogger.Error(logging.Scope{}, logging.ErrorRecord{
				Message: "后端服务停止失败", Class: logging.ClassToolBug, Err: err,
			})
		}
		appLogger.Info(logging.Scope{}, "后端服务已停止")
	}
}
