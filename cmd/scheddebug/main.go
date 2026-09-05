package main

import (
	"context"
	"fmt"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

func main() {
	cfg := config.LoadPlanDBConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer db.DB.Close()
	store := planmysql.NewRunRepository(db.DB)
	ids, err := store.ListRunIDsNeedingScheduling(context.Background())
	fmt.Println("needing:", ids, err)
	pathRuns, _ := store.ListPathRunsByRun(context.Background(), 14)
	for _, p := range pathRuns {
		fmt.Printf("pathRun %d path %d status %s\n", p.ID, p.ExecutionPathID, p.Status)
	}
	run, _ := store.GetRun(context.Background(), 14)
	fmt.Printf("run status=%s mode=%s maxc=%v preset=%q\n", run.Status, run.Mode, run.MaxConcurrency, run.PresetBreakpoints)
	var columnCheck string
	err = db.DB.QueryRowContext(context.Background(), "SELECT COALESCE(scheduled_consumed_at, '1970') FROM test_plans WHERE id = 11").Scan(&columnCheck)
	fmt.Println("col check:", columnCheck, err)
	_ = repository.ErrRunNotFound
}
