package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

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
	server := &http.Server{
		Addr:              config.ServerAddress(),
		Handler:           api.NewHandlerWithFlowGraphServices(targetReader, planService, flowGraphService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("后端服务监听 %s", server.Addr)
	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
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
