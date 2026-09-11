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

type versionedLoginClient struct {
	mu           sync.Mutex
	count        int
	firstStarted chan struct{}
	releaseFirst chan struct{}
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

// Login 返回可区分代次的会话，并让首个登录停住，供验证迟到 SID 覆盖保护。
func (c *versionedLoginClient) Login(ctx context.Context, account string) (target.Session, error) {
	c.mu.Lock()
	c.count++
	count := c.count
	c.mu.Unlock()
	if count == 1 {
		close(c.firstStarted)
		select {
		case <-c.releaseFirst:
		case <-ctx.Done():
			return target.Session{}, ctx.Err()
		}
	}
	return target.Session{SID: fmt.Sprintf("sid-%d", count), Summary: target.AccountSummary{Account: account}}, nil
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

// TestSessionManagerSerializesReadUsageByAccount 验证同账号读操作在完整回调期间排队，
// 不同账号仍可并行进入，避免读请求与执行器刷新同一 SID。
func TestSessionManagerSerializesReadUsageByAccount(t *testing.T) {
	client := newFakeLoginClient()
	manager := session.NewManager(client, time.Hour)
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- manager.DoRead(context.Background(), "account-a", func(context.Context, target.Session) error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- manager.DoRead(context.Background(), "account-a", func(context.Context, target.Session) error {
			close(secondEntered)
			return nil
		})
	}()
	select {
	case <-secondEntered:
		t.Fatal("同账号第二个读操作未等待第一个操作释放")
	case <-time.After(50 * time.Millisecond):
	}
	differentDone := make(chan error, 1)
	go func() {
		differentDone <- manager.DoRead(context.Background(), "account-b", func(context.Context, target.Session) error { return nil })
	}()
	select {
	case err := <-differentDone:
		if err != nil {
			t.Fatalf("不同账号读操作失败：%v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("不同账号读操作被错误阻塞")
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("第一个读操作失败：%v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("第二个读操作失败：%v", err)
	}
}

// TestSessionManagerDropsLateLoginResult 验证刷新代次变化后，迟到的旧登录结果不会覆盖新 SID。
func TestSessionManagerDropsLateLoginResult(t *testing.T) {
	client := &versionedLoginClient{firstStarted: make(chan struct{}), releaseFirst: make(chan struct{})}
	manager := session.NewManager(client, time.Hour)
	firstDone := make(chan target.Session, 1)
	go func() {
		session, _ := manager.Current(context.Background(), "account-a")
		firstDone <- session
	}()
	<-client.firstStarted
	refreshDone := make(chan target.Session, 1)
	go func() {
		session, err := manager.Refresh(context.Background(), "account-a")
		if err != nil {
			t.Errorf("刷新会话失败：%v", err)
		}
		refreshDone <- session
	}()
	close(client.releaseFirst)
	first := <-firstDone
	refreshed := <-refreshDone
	if first.SID != "sid-1" || refreshed.SID != "sid-2" {
		t.Fatalf("刷新会话代次异常：first=%q refreshed=%q", first.SID, refreshed.SID)
	}
	current, err := manager.Current(context.Background(), "account-a")
	if err != nil || current.SID != "sid-2" {
		t.Fatalf("刷新后的缓存被旧登录结果覆盖：current=%q err=%v", current.SID, err)
	}
	if got := client.count; got != 2 {
		t.Fatalf("会话刷新产生了 %d 次登录，期望 2", got)
	}
}

// TestSessionManagerRetriesReadNetworkFailure 验证只读连接失败会按预算重试，业务错误仍只执行一次。
func TestSessionManagerRetriesReadNetworkFailure(t *testing.T) {
	client := newFakeLoginClient()
	manager := session.NewManager(client, time.Hour, session.WithReadRetry(3, 0, 0))
	calls := 0
	err := manager.DoRead(context.Background(), "account-a", func(context.Context, target.Session) error {
		calls++
		if calls < 3 {
			failure := target.NewError(target.ErrorTimeout, nil).(*target.Error)
			failure.Transport = target.TransportConnectFailed
			return failure
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("只读连接失败应重试至成功：calls=%d err=%v", calls, err)
	}

	calls = 0
	err = manager.DoRead(context.Background(), "account-a", func(context.Context, target.Session) error {
		calls++
		return &target.BusinessRejection{Code: "FLOW_409", Message: "业务拒绝"}
	})
	if err == nil || calls != 1 {
		t.Fatalf("业务错误不得重试：calls=%d err=%v", calls, err)
	}
}
