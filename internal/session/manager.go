package session

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

type LoginClient interface {
	Login(context.Context, string) (target.Session, error)
}

type cacheEntry struct {
	session   target.Session
	expiresAt time.Time
	version   uint64
}

type Manager struct {
	client LoginClient
	ttl    time.Duration
	now    func() time.Time
	// retryAttempts 只用于幂等读取和登录连接未建立两类安全重试。
	retryAttempts int
	retryBase     time.Duration
	retryMax      time.Duration

	mu           sync.RWMutex
	entries      map[string]cacheEntry
	versions     map[string]uint64
	lockMu       sync.Mutex
	accountLocks map[string]*sync.Mutex
	// useMu 与 useLocks 是账号「使用中」锁：同一账号同一时间只允许一个操作持有会话，
	// 其他需要该账号的操作原地等待。目标平台同账号重复登录会踢掉旧会话，
	// 审批链路持有此锁即可避免并发使用同一处理人账号时互相踢下线。
	useMu    sync.Mutex
	useLocks map[string]*sync.Mutex
}

type Option func(*Manager)

func WithClock(clock func() time.Time) Option {
	return func(manager *Manager) {
		if clock != nil {
			manager.now = clock
		}
	}
}

// WithReadRetry 配置幂等读取与登录连接失败的尝试次数和退避间隔。
func WithReadRetry(attempts int, baseDelay, maxDelay time.Duration) Option {
	return func(manager *Manager) {
		if attempts > 0 {
			manager.retryAttempts = attempts
		}
		manager.retryBase = baseDelay
		manager.retryMax = maxDelay
	}
}

func NewManager(client LoginClient, ttl time.Duration, options ...Option) *Manager {
	manager := &Manager{
		client:        client,
		ttl:           ttl,
		now:           time.Now,
		retryAttempts: 1,
		entries:       make(map[string]cacheEntry),
		versions:      make(map[string]uint64),
		accountLocks:  make(map[string]*sync.Mutex),
	}
	for _, option := range options {
		option(manager)
	}
	return manager
}

func (m *Manager) Verify(ctx context.Context, account string) (target.AccountSummary, error) {
	// 验证也占用账号使用锁，避免页面验证与执行器同时登录覆盖同一 SID。
	release := m.LockAccountUsage(account)
	defer release()
	session, err := m.getOrLogin(ctx, account)
	if err != nil {
		return target.AccountSummary{}, err
	}
	return session.Summary, nil
}

// Current 返回当前账号缓存或新登录得到的完整会话，仅供后端建立短期运行时上下文。
func (m *Manager) Current(ctx context.Context, account string) (target.Session, error) {
	return m.getOrLogin(ctx, account)
}

// Refresh 作废缓存并强制重新登录，返回全新会话。
// 写路径的 prepare 阶段使用它：目标会话可能随时失效（RESP401），而 submit 前是最后一次
// 只读刷新机会；一旦写请求发出就不再有任何自动重登或重发。
func (m *Manager) Refresh(ctx context.Context, account string) (target.Session, error) {
	lock := m.accountLock(normalizeAccount(account))
	lock.Lock()
	defer lock.Unlock()
	m.invalidate(account, "")
	return m.loginFresh(ctx, account)
}

// DoRead 只对会话失效执行一次重登和一次只读重放。
func (m *Manager) DoRead(ctx context.Context, account string, call func(context.Context, target.Session) error) error {
	// 读操作从取得会话到失效重登和一次重放都占用账号使用锁，避免页面读请求与执行链
	// 同时登录/使用同一账号时互相覆盖会话。Refresh 不在这里再次加锁，避免递归锁死。
	release := m.LockAccountUsage(account)
	defer release()
	active, err := m.getOrLogin(ctx, account)
	if err != nil {
		return err
	}
	return m.doReadWithSession(ctx, account, active, call)
}

// DoReadWithoutRelogin 只用当前缓存会话执行只读调用：会话不存在或失效都直接返回错误，
// 绝不重登。背景：目标平台同账号互踢，若画布轮询这类非关键读在会话失效后自动重登，
// 会把用户正在使用的浏览器会话踢下线，用户再登录又踢工具，形成互踢死循环（实测）。
// 只有真正执行动作的链路（DoRead/写路径）才允许检测并重登。
func (m *Manager) DoReadWithoutRelogin(ctx context.Context, account string, call func(context.Context, target.Session) error) error {
	release := m.LockAccountUsage(account)
	defer release()
	active, ok := m.cached(normalizeAccount(account))
	if !ok {
		return target.NewError(target.ErrorSessionExpired, errors.New("无缓存会话，且后台刷新不允许登录"))
	}
	callErr := call(target.WithRetryAttempt(ctx, false, 1), active)
	if callErr == nil {
		return nil
	}
	if target.IsKind(callErr, target.ErrorSessionExpired) {
		return callErr
	}
	if !target.IsRetryableReadError(callErr) {
		return callErr
	}
	// 可重试的网络抖动：一次重放，但仍不重登。
	if waitErr := m.waitRetry(ctx, 1); waitErr != nil {
		return waitErr
	}
	return call(target.WithRetryAttempt(ctx, true, 2), active)
}

// doReadWithSession 是 DoRead 的会话重试主体：会话失效重登一次并重放，可重试网络错误有界重试。
func (m *Manager) doReadWithSession(ctx context.Context, account string, active target.Session, call func(context.Context, target.Session) error) error {
	sessionRefreshed := false
	requestAttempt := 0
	networkFailures := 0
	for {
		requestAttempt++
		callContext := target.WithRetryAttempt(ctx, requestAttempt > 1, requestAttempt)
		err := call(callContext, active)
		if err == nil {
			return nil
		}
		if target.IsKind(err, target.ErrorSessionExpired) {
			if sessionRefreshed {
				m.invalidate(account, active.SID)
				return target.NewError(target.ErrorSessionExpired, errors.New("新会话仍然失效，可能存在浏览器或其他进程的外部会话竞争"))
			}
			sessionRefreshed = true
			m.invalidate(account, active.SID)
			var loginErr error
			active, loginErr = m.getOrLogin(ctx, account)
			if loginErr != nil {
				if target.IsKind(loginErr, target.ErrorLoginRejected) {
					return target.NewError(target.ErrorSessionExpired, loginErr)
				}
				return loginErr
			}
			continue
		}
		if !target.IsRetryableReadError(err) {
			return err
		}
		networkFailures++
		if networkFailures >= m.retryAttempts {
			return err
		}
		if waitErr := m.waitRetry(ctx, networkFailures); waitErr != nil {
			return waitErr
		}
	}
}

func (m *Manager) getOrLogin(ctx context.Context, account string) (target.Session, error) {
	key := normalizeAccount(account)
	if cached, ok := m.cached(key); ok {
		return cached, nil
	}
	lock := m.accountLock(key)
	lock.Lock()
	defer lock.Unlock()
	if cached, ok := m.cached(key); ok {
		return cached, nil
	}
	// 登录期间可能有 Refresh 使旧请求失效；代次不一致时丢弃旧登录结果并重新登录，
	// 避免迟到的旧 SID 覆盖刷新后的有效会话。
	return m.loginFresh(ctx, account)
}

// loginFresh 在已持有账号登录锁时获取新会话，并校验缓存代次后再写入。
func (m *Manager) loginFresh(ctx context.Context, account string) (target.Session, error) {
	key := normalizeAccount(account)
	for {
		version := m.cacheVersion(key)
		var active target.Session
		var err error
		for attempt := 1; attempt <= m.retryAttempts; attempt++ {
			active, err = m.client.Login(target.WithRetryAttempt(ctx, attempt > 1, attempt), strings.TrimSpace(account))
			if err == nil {
				break
			}
			if !target.IsRetryableWriteConnectError(err) || attempt >= m.retryAttempts {
				return target.Session{}, err
			}
			if waitErr := m.waitRetry(ctx, attempt); waitErr != nil {
				return target.Session{}, waitErr
			}
		}
		m.mu.Lock()
		if m.versions[key] == version {
			m.entries[key] = cacheEntry{session: active, expiresAt: m.now().Add(m.ttl), version: version}
			m.mu.Unlock()
			return active, nil
		}
		m.mu.Unlock()
	}
}

// waitRetry 等待第 attempt 次安全失败后的指数退避，并响应调用方取消。
func (m *Manager) waitRetry(ctx context.Context, attempt int) error {
	delay := m.retryBase
	if delay <= 0 {
		return nil
	}
	maxDelay := m.retryMax
	if maxDelay <= 0 {
		maxDelay = delay
	}
	for current := 1; current < attempt && delay < maxDelay; current++ {
		delay *= 2
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// cacheVersion 读取账号会话代次；代次只在刷新/失效时递增，保障迟到登录结果不会覆盖新会话。
func (m *Manager) cacheVersion(key string) uint64 {
	m.mu.RLock()
	version := m.versions[key]
	m.mu.RUnlock()
	return version
}

func (m *Manager) cached(key string) (target.Session, bool) {
	m.mu.RLock()
	entry, ok := m.entries[key]
	version := m.versions[key]
	m.mu.RUnlock()
	if !ok || entry.version != version {
		return target.Session{}, false
	}
	if !m.now().Before(entry.expiresAt) {
		m.mu.Lock()
		if current, exists := m.entries[key]; exists && current.version == version && !m.now().Before(current.expiresAt) {
			m.versions[key]++
			delete(m.entries, key)
		}
		m.mu.Unlock()
		return target.Session{}, false
	}
	return entry.session, true
}

func (m *Manager) invalidate(account, sid string) {
	key := normalizeAccount(account)
	m.mu.Lock()
	entry, ok := m.entries[key]
	if sid != "" && (!ok || entry.session.SID != sid) {
		m.mu.Unlock()
		return
	}
	m.versions[key]++
	delete(m.entries, key)
	m.mu.Unlock()
}

func (m *Manager) accountLock(key string) *sync.Mutex {
	m.lockMu.Lock()
	defer m.lockMu.Unlock()
	if lock, ok := m.accountLocks[key]; ok {
		return lock
	}
	lock := &sync.Mutex{}
	m.accountLocks[key] = lock
	return lock
}

// LockAccountUsage 占用某账号的会话（2026-09-11 用户裁决：当前人正在使用中就等别人用完再用）。
// 返回释放函数；调用方在持有期间完成「读待办 → 审批 → 核验」的完整操作。
// 同一账号的并发操作在此排队，避免重复登录把正在使用的会话踢下线。
// 注意：持有期间调用方不得再对本账号做会话刷新以外的 getOrLogin，否则会自己锁死自己。
func (m *Manager) LockAccountUsage(account string) func() {
	key := "use:" + normalizeAccount(account)
	m.useMu.Lock()
	if m.useLocks == nil {
		m.useLocks = map[string]*sync.Mutex{}
	}
	lock, ok := m.useLocks[key]
	if !ok {
		lock = &sync.Mutex{}
		m.useLocks[key] = lock
	}
	m.useMu.Unlock()
	// F-030/T03：账号锁等待结构化计时。同账号路径并发时，等待时长在此可见，
	// 与目标接口耗时（network.log duration_s）严格分开，不能混为一谈。
	started := time.Now()
	lock.Lock()
	if waited := time.Since(started); waited > 100*time.Millisecond {
		log.Printf("[session] 账号锁等待 account=%s waited=%s（同账号其他路径正在使用会话，串行排队中）", account, waited.Round(time.Millisecond))
	}
	return lock.Unlock
}

func normalizeAccount(account string) string {
	return strings.TrimSpace(account)
}
