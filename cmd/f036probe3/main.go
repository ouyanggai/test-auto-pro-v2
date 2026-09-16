package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/session"
)

// main 打印目标模板树的节点 ID 与名称（只读）。
func main() {
	templateID := "1179794e2da3403cb22cb0cad7e5a978"
	cfg := config.LoadTargetConfig()
	account := "骆蒙恩"
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
	tree, err := client.ReadTemplateTree(ctx, s, templateID)
	if err != nil { panic(err) }
	var walk func(node *target.FlowNodeTemplate, depth int)
	walk = func(node *target.FlowNodeTemplate, depth int) {
		if node == nil { return }
		fmt.Printf("%s%s | %s | type=%s\n", strings.Repeat("  ", depth), node.ID, node.Name, node.Type)
		walk(node.Child, depth+1)
		for i := range node.ConditionNodes {
			fmt.Printf("%s[条件分支 %d] %s | %s\n", strings.Repeat("  ", depth+1), i, node.ConditionNodes[i].ID, node.ConditionNodes[i].Name)
			walk(node.ConditionNodes[i].Child, depth+2)
		}
		for i := range node.ParallelNodes {
			fmt.Printf("%s[并行分支 %d] %s | %s\n", strings.Repeat("  ", depth+1), i, node.ParallelNodes[i].ID, node.ParallelNodes[i].Name)
			walk(node.ParallelNodes[i].Child, depth+2)
		}
	}
	walk(tree, 0)
}
