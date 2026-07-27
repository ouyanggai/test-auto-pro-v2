package session

import (
	"context"
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
}

type Manager struct {
	client LoginClient
	ttl    time.Duration
	now    func() time.Time

	mu           sync.RWMutex
	entries      map[string]cacheEntry
	lockMu       sync.Mutex
	accountLocks map[string]*sync.Mutex
}

type Option func(*Manager)

func WithClock(clock func() time.Time) Option {
	return func(manager *Manager) {
		if clock != nil {
			manager.now = clock
		}
	}
}

func NewManager(client LoginClient, ttl time.Duration, options ...Option) *Manager {
	manager := &Manager{
		client:       client,
		ttl:          ttl,
		now:          time.Now,
		entries:      make(map[string]cacheEntry),
		accountLocks: make(map[string]*sync.Mutex),
	}
	for _, option := range options {
		option(manager)
	}
	return manager
}

func (m *Manager) Verify(ctx context.Context, account string) (target.AccountSummary, error) {
	session, err := m.getOrLogin(ctx, account)
	if err != nil {
		return target.AccountSummary{}, err
	}
	return session.Summary, nil
}

// DoRead 只对会话失效执行一次重登和一次只读重放。
func (m *Manager) DoRead(ctx context.Context, account string, call func(context.Context, target.Session) error) error {
	session, err := m.getOrLogin(ctx, account)
	if err != nil {
		return err
	}
	err = call(ctx, session)
	if !target.IsKind(err, target.ErrorSessionExpired) {
		return err
	}
	m.invalidate(account, session.SID)
	session, err = m.getOrLogin(ctx, account)
	if err != nil {
		if target.IsKind(err, target.ErrorLoginRejected) {
			return target.NewError(target.ErrorSessionExpired, err)
		}
		return err
	}
	err = call(ctx, session)
	if target.IsKind(err, target.ErrorSessionExpired) {
		m.invalidate(account, session.SID)
		return target.NewError(target.ErrorSessionExpired, err)
	}
	return err
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
	session, err := m.client.Login(ctx, strings.TrimSpace(account))
	if err != nil {
		return target.Session{}, err
	}
	m.mu.Lock()
	m.entries[key] = cacheEntry{session: session, expiresAt: m.now().Add(m.ttl)}
	m.mu.Unlock()
	return session, nil
}

func (m *Manager) cached(key string) (target.Session, bool) {
	m.mu.RLock()
	entry, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok {
		return target.Session{}, false
	}
	if !m.now().Before(entry.expiresAt) {
		m.mu.Lock()
		delete(m.entries, key)
		m.mu.Unlock()
		return target.Session{}, false
	}
	return entry.session, true
}

func (m *Manager) invalidate(account, sid string) {
	key := normalizeAccount(account)
	m.mu.Lock()
	entry, ok := m.entries[key]
	if ok && (sid == "" || entry.session.SID == sid) {
		delete(m.entries, key)
	}
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

func normalizeAccount(account string) string {
	return strings.TrimSpace(account)
}
