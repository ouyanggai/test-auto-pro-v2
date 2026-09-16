package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/session"
)

// main 查询指定实例的当前事实与处理人（只读）。
func main() {
	instanceID := os.Args[1]
	cfg := config.LoadTargetConfig()
	account := os.Getenv("TEST_AUTO_PRO_TARGET_ACCOUNT")
	if account == "" {
		raw, _ := os.ReadFile(".env.local")
		for _, line := range strings.Split(string(raw), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "TEST_AUTO_PRO_TARGET_ACCOUNT="); ok {
				account = strings.Trim(v, `"'`)
			}
		}
	}
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: cfg.APIGateway, LoginPassword: cfg.LoginPassword, LoginAESKey: cfg.LoginAESKey,
		LoginCode: cfg.LoginCode, PlatformCode: cfg.PlatformCode, TemplatePlatformCodes: cfg.TemplatePlatformCodes,
		CustomerCode: cfg.CustomerCode, Timeout: cfg.HTTPTimeout,
	})
	if err != nil { panic(err) }
	sessions := session.NewManager(client, cfg.SessionTTL)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	s, err := sessions.Current(ctx, account)
	if err != nil { panic(err) }
	facts, err := client.FindSubmittedFlowFacts(ctx, s, instanceID)
	if err != nil { panic(err) }
	out, _ := json.MarshalIndent(map[string]any{
		"found": facts.Found, "status": facts.Status, "currentNodes": facts.CurrentNodes,
		"handlers": facts.Handlers, "name": facts.Name,
	}, "", "  ")
	fmt.Println(string(out))
}
