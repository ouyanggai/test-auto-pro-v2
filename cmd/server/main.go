package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/formruntimemaintenance"
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

	targetReader := service.NewTargetReadService(config.LoadTargetConfig())
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
	pathConfigRepository := planmysql.NewPathConfigurationRepository(planDatabase.DB)
	pathConfigService := service.NewPathConfigService(
		planService, targetReader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(),
		analyzer.NewPathConfigAnalyzer(), pathRepository, pathConfigRepository,
	)
	// 候选提供者与计划读取共用当前发起人会话；缓存按模板实际组件加载且不扩大目标权限。
	pathConfigService.SetComponentCandidateCache(service.NewComponentCandidateCache(targetReader, 1000, 15*time.Minute))
	pathPreparationService := service.NewPathPreparationService(pathConfigService, planmysql.NewPathPreparationRepository(planDatabase.DB))
	if err := pathPreparationService.Recover(context.Background()); err != nil {
		log.Printf("恢复批量路径准备任务失败：%v", err)
	}
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
	templateCatalogService := service.NewTemplateCatalogService(targetReader, planmysql.NewTemplateCatalogRepository(planDatabase.DB), workspaceRoot)
	historyDataService := service.NewHistoryDataService(planService, pathRepository, targetReader, planmysql.NewHistoryReplayStore(planDatabase.DB))
	historyReplayService := service.NewHistoryReplayService(planService, pathRepository, targetReader, planmysql.NewHistoryReplayStore(planDatabase.DB))
	if err := historyReplayService.Recover(context.Background()); err != nil {
		log.Printf("恢复历史回放任务失败：%v", err)
	}
	pathConfigService.SetTemplateRuleCatalog(templateCatalogService)
	if err := templateCatalogService.Recover(context.Background()); err != nil {
		log.Printf("恢复模板规则目录任务失败：%v", err)
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
	server := &http.Server{
		Addr:              config.ServerAddress(),
		Handler:           api.NewHandlerWithHistoryReplayServices(targetReader, planService, flowGraphService, executionPathService, pathRequirementService, pathConfigService, maintenanceService, pathPreparationService, templateCatalogService, historyDataService, historyReplayService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("后端服务监听 %s", server.Addr)
	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
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
		}
	}
}
