package formruntimemaintenance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileLogStore 把技术日志写入运行目录，并按尾部上限提供 UI 读取。
type FileLogStore struct {
	root         string
	maxReadBytes int64
}

// NewFileLogStore 创建文件日志仓储。
func NewFileLogStore(root string, maxReadBytes int64) (*FileLogStore, error) {
	if root == "" {
		return nil, errors.New("缺少表单运行时维护日志目录")
	}
	if maxReadBytes <= 0 {
		maxReadBytes = 256 * 1024
	}
	return &FileLogStore{root: filepath.Clean(root), maxReadBytes: maxReadBytes}, nil
}

// Open 以追加模式打开任务日志，使 Worker 重启恢复时继续同一日志。
func (s *FileLogStore) Open(ctx context.Context, jobID uint64) (io.WriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := s.path(jobID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

// Read 只返回日志尾部，避免一次 API 读取无限增长的构建输出。
func (s *FileLogStore) Read(ctx context.Context, jobID uint64) (Log, error) {
	if err := ctx.Err(); err != nil {
		return Log{}, err
	}
	file, err := os.Open(s.path(jobID))
	if errors.Is(err, os.ErrNotExist) {
		return Log{}, ErrLogNotFound
	}
	if err != nil {
		return Log{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Log{}, err
	}
	truncated := info.Size() > s.maxReadBytes
	if truncated {
		if _, err := file.Seek(-s.maxReadBytes, io.SeekEnd); err != nil {
			return Log{}, err
		}
	}
	content, err := io.ReadAll(io.LimitReader(file, s.maxReadBytes))
	if err != nil {
		return Log{}, err
	}
	return Log{Content: string(content), Truncated: truncated}, nil
}

// path 生成固定任务日志路径，不接受请求提供任意文件名。
func (s *FileLogStore) path(jobID uint64) string {
	return filepath.Join(s.root, fmt.Sprintf("job-%d.log", jobID))
}

// MemoryLogStore 为状态机单元测试保存内存日志。
type MemoryLogStore struct {
	mu   sync.Mutex
	logs map[uint64]*bytes.Buffer
}

// NewMemoryLogStore 创建内存日志仓储。
func NewMemoryLogStore() *MemoryLogStore {
	return &MemoryLogStore{logs: make(map[uint64]*bytes.Buffer)}
}

// Open 返回线程安全追加写器。
func (s *MemoryLogStore) Open(_ context.Context, jobID uint64) (io.WriteCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.logs[jobID] == nil {
		s.logs[jobID] = &bytes.Buffer{}
	}
	return &memoryWriter{store: s, jobID: jobID}, nil
}

// Read 返回日志副本。
func (s *MemoryLogStore) Read(_ context.Context, jobID uint64) (Log, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buffer := s.logs[jobID]
	if buffer == nil {
		return Log{}, ErrLogNotFound
	}
	return Log{Content: buffer.String()}, nil
}

type memoryWriter struct {
	store *MemoryLogStore
	jobID uint64
}

// Write 线程安全追加日志。
func (w *memoryWriter) Write(content []byte) (int, error) {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	return w.store.logs[w.jobID].Write(content)
}

// Close 结束内存写器，实际缓冲区继续供 API 读取。
func (w *memoryWriter) Close() error { return nil }
