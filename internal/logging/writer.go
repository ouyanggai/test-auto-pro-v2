package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Writer 是一个有界日志文件写入器：同文件写入串行、超限轮转、返回记录所在行号。
// 行号会被后续切片存进运行记录用于日志深链，所以签名现在就固定下来。
type Writer struct {
	mu           sync.Mutex
	path         string
	lines        int64
	size         int64
	maxFileBytes int64
	maxBackups   int
	prepared     bool
}

// NewWriter 创建指定文件的写入器；目录在首次写入时才创建，避免空目录。
func NewWriter(path string) *Writer {
	return &Writer{path: filepath.Clean(path), maxFileBytes: DefaultMaxFileBytes, maxBackups: DefaultMaxBackups}
}

// SetLimits 覆盖容量上限与保留份数，供测试构造小文件验证轮转。
func (w *Writer) SetLimits(maxFileBytes int64, maxBackups int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if maxFileBytes > 0 {
		w.maxFileBytes = maxFileBytes
	}
	if maxBackups >= 0 {
		w.maxBackups = maxBackups
	}
}

// Path 返回该写入器的目标文件路径。
func (w *Writer) Path() string { return w.path }

// WriteLine 追加一条记录并返回它所在的行号；写入失败只降级为一次标准错误输出。
func (w *Writer) WriteLine(line string) int64 {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.prepare(); err != nil {
		reportWriteFailure(w.path, err)
		return 0
	}
	payload := strings.TrimRight(line, "\n") + "\n"
	if w.maxFileBytes > 0 && w.size+int64(len(payload)) > w.maxFileBytes && w.size > 0 {
		if err := w.rotate(); err != nil {
			reportWriteFailure(w.path, err)
		}
	}
	file, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		reportWriteFailure(w.path, err)
		return 0
	}
	defer file.Close()
	written, err := file.WriteString(payload)
	if err != nil {
		reportWriteFailure(w.path, err)
		return 0
	}
	w.size += int64(written)
	w.lines += int64(strings.Count(payload, "\n"))
	return w.lines
}

// WriteBlock 追加一个多行块，返回块首行号；只有 curl 日志与 panic 栈允许多行。
func (w *Writer) WriteBlock(header, body, footer string) int64 {
	if w == nil {
		return 0
	}
	lines := []string{header}
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		lines = append(lines, line)
	}
	lines = append(lines, footer)
	first := int64(0)
	for index, line := range lines {
		lineNumber := w.WriteLine(line)
		if index == 0 {
			first = lineNumber
		}
	}
	return first
}

// prepare 首次写入时创建目录并读取已有文件的行数与大小，保证行号连续。
func (w *Writer) prepare() error {
	if w.prepared {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return err
	}
	content, err := os.ReadFile(w.path)
	if err == nil {
		w.size = int64(len(content))
		w.lines = int64(strings.Count(string(content), "\n"))
	} else if !os.IsNotExist(err) {
		return err
	}
	w.prepared = true
	return nil
}

// rotate 把当前文件轮转为 .1 并顺移历史文件，行号从新文件重新开始。
func (w *Writer) rotate() error {
	for index := w.maxBackups; index >= 1; index-- {
		source := fmt.Sprintf("%s.%d", w.path, index)
		if index == w.maxBackups {
			_ = os.Remove(source)
			continue
		}
		_ = os.Rename(source, fmt.Sprintf("%s.%d", w.path, index+1))
	}
	if w.maxBackups >= 1 {
		if err := os.Rename(w.path, w.path+".1"); err != nil && !os.IsNotExist(err) {
			return err
		}
	} else if err := os.Remove(w.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	w.size, w.lines = 0, 0
	return nil
}

// reportWriteFailure 日志写入失败时只在标准错误提示一次，绝不影响主流程。
func reportWriteFailure(path string, err error) {
	fmt.Fprintf(os.Stderr, "日志写入失败 path=%s err=%v\n", path, err)
}

// Router 按作用域把日志文件路由到全局目录、配置桶或运行目录，并复用同一文件的写入器。
type Router struct {
	mu      sync.Mutex
	root    string
	now     func() time.Time
	writers map[string]*Writer
}

// NewRouter 创建目录路由；now 可注入以固定测试时间。
func NewRouter(root string, now func() time.Time) *Router {
	if now == nil {
		now = time.Now
	}
	return &Router{root: filepath.Clean(root), now: now, writers: map[string]*Writer{}}
}

// Root 返回日志根目录。
func (r *Router) Root() string { return r.root }

// Global 返回全局程序日志或程序错误日志的写入器。
func (r *Router) Global(name string) *Writer { return r.writer(filepath.Join(r.root, name)) }

// Archive 返回按日归档目录下的写入器，供全局日志按日切分。
func (r *Router) Archive(name string, day time.Time) *Writer {
	return r.writer(filepath.Join(r.root, "archive", day.Format("2006-01-02"), name))
}

// Bucket 按作用域返回该日志文件所在的桶写入器：
// 运行作用域进 logs/runs/<计划名>/<路径名>/<运行号>/，其余进 logs/config/<日期>/。
func (r *Router) Bucket(scope Scope, name string) *Writer {
	return r.writer(filepath.Join(r.BucketDir(scope), name))
}

// BucketDir 返回当前作用域对应的日志桶目录，目录段全部经过清洗。
func (r *Router) BucketDir(scope Scope) string {
	if scope.IsRun() {
		return filepath.Join(r.root, "runs",
			SanitizePathSegment(scope.PlanName), SanitizePathSegment(scope.PathName), SanitizePathSegment(scope.RunSeq))
	}
	return filepath.Join(r.root, "config", r.now().Format("2006-01-02"))
}

// writer 复用同一文件的写入器，保证并发写入串行且行号连续。
func (r *Router) writer(path string) *Writer {
	clean := filepath.Clean(path)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.writers[clean]; ok {
		return existing
	}
	created := NewWriter(clean)
	r.writers[clean] = created
	return created
}
