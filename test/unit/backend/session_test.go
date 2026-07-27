package backend_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/session"
)

type fakeLoginClient struct {
	mu       sync.Mutex
	count    map[string]int
	gate     chan struct{}
	sessions map[string][]target.Session
}

type selectiveLoginClient struct {
	slowStarted chan struct{}
	releaseSlow chan struct{}
}

func (c *selectiveLoginClient) Login(ctx context.Context, account string) (target.Session, error) {
	if account == "slow-account" {
		close(c.slowStarted)
		select {
		case <-c.releaseSlow:
		case <-ctx.Done():
			return target.Session{}, ctx.Err()
		}
	}
	return target.Session{SID: "runtime-session", Summary: target.AccountSummary{Account: account}}, nil
}

func newFakeLoginClient() *fakeLoginClient {
	return &fakeLoginClient{count: make(map[string]int), sessions: make(map[string][]target.Session)}
}

func (f *fakeLoginClient) Login(ctx context.Context, account string) (target.Session, error) {
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return target.Session{}, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count[account]++
	active := target.Session{
		SID:     fmt.Sprintf("runtime-session-%d", f.count[account]),
		Summary: target.AccountSummary{Account: account, DisplayName: "测试人员"},
	}
	f.sessions[account] = append(f.sessions[account], active)
	return active, nil
}

func (f *fakeLoginClient) loginCount(account string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count[account]
}

func TestSessionManagerDeduplicatesConcurrentLoginByAccount(t *testing.T) {
	client := newFakeLoginClient()
	client.gate = make(chan struct{})
	manager := session.NewManager(client, time.Hour)
	const workers = 12
	var wait sync.WaitGroup
	wait.Add(workers)
	errorsFound := make(chan error, workers)
	for range workers {
		go func() {
			defer wait.Done()
			_, err := manager.Verify(context.Background(), " account-a ")
			errorsFound <- err
		}()
	}
	close(client.gate)
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatalf("并发验证失败：%v", err)
		}
	}
	if got := client.loginCount("account-a"); got != 1 {
		t.Fatalf("同账号实际登录次数 = %d，期望 1", got)
	}
}

func TestSessionManagerSeparatesAccountsAndExpiresByTTL(t *testing.T) {
	client := newFakeLoginClient()
	now := time.Unix(1000, 0)
	manager := session.NewManager(client, time.Hour, session.WithClock(func() time.Time { return now }))

	for _, account := range []string{"account-a", "account-b"} {
		if _, err := manager.Verify(context.Background(), account); err != nil {
			t.Fatalf("验证账号失败：%v", err)
		}
	}
	if client.loginCount("account-a") != 1 || client.loginCount("account-b") != 1 {
		t.Fatal("不同账号未独立登录")
	}
	now = now.Add(time.Hour)
	if _, err := manager.Verify(context.Background(), "account-a"); err != nil {
		t.Fatalf("TTL 后重新验证失败：%v", err)
	}
	if got := client.loginCount("account-a"); got != 2 {
		t.Fatalf("TTL 后登录次数 = %d，期望 2", got)
	}
}

func TestSessionManagerDoesNotBlockDifferentAccounts(t *testing.T) {
	client := &selectiveLoginClient{slowStarted: make(chan struct{}), releaseSlow: make(chan struct{})}
	manager := session.NewManager(client, time.Hour)
	slowDone := make(chan error, 1)
	go func() {
		_, err := manager.Verify(context.Background(), "slow-account")
		slowDone <- err
	}()
	<-client.slowStarted

	fastDone := make(chan error, 1)
	go func() {
		_, err := manager.Verify(context.Background(), "fast-account")
		fastDone <- err
	}()
	select {
	case err := <-fastDone:
		if err != nil {
			t.Fatalf("不同账号验证失败：%v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("慢账号登录阻塞了其他账号")
	}
	close(client.releaseSlow)
	if err := <-slowDone; err != nil {
		t.Fatalf("慢账号验证失败：%v", err)
	}
}

func TestSessionManagerRelogsAndReplaysOnlyOnce(t *testing.T) {
	client := newFakeLoginClient()
	manager := session.NewManager(client, time.Hour)
	callCount := 0
	err := manager.DoRead(context.Background(), "account-a", func(_ context.Context, _ target.Session) error {
		callCount++
		return target.NewError(target.ErrorSessionExpired, nil)
	})
	if !target.IsKind(err, target.ErrorSessionExpired) {
		t.Fatalf("最终错误分类不正确：%v", err)
	}
	if callCount != 2 || client.loginCount("account-a") != 2 {
		t.Fatalf("调用次数 = %d，登录次数 = %d，期望均为 2", callCount, client.loginCount("account-a"))
	}
}

func TestSessionManagerReusesValidSession(t *testing.T) {
	client := newFakeLoginClient()
	manager := session.NewManager(client, time.Hour)
	for range 3 {
		if _, err := manager.Verify(context.Background(), "account-a"); err != nil {
			t.Fatalf("验证失败：%v", err)
		}
	}
	if got := client.loginCount("account-a"); got != 1 {
		t.Fatalf("有效缓存下登录次数 = %d，期望 1", got)
	}
}
