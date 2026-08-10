package backend_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/formruntimemaintenance"
)

type maintenanceFixture struct {
	workspace string
	source    string
	manifest  formruntimemaintenance.Manifest
	inspector *formruntimemaintenance.GitSourceInspector
	syncer    *formruntimemaintenance.Syncer
}

// newMaintenanceFixture 创建两个临时 Git 仓库，模拟固定参考来源和当前项目实际运行源码区。
func newMaintenanceFixture(t *testing.T) maintenanceFixture {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	source := filepath.Join(workspace, "reference", "rsh-flow-components")
	mustMkdir(t, filepath.Join(source, "src"))
	mustWrite(t, filepath.Join(source, "src", "component.js"), "export default 'source'\n")
	mustWrite(t, filepath.Join(source, "src", "main.js"), "const components = [{ name: 'custom-one', component: {} }]\n")
	mustWrite(t, filepath.Join(source, "package.json"), `{"name":"source"}`)
	git(t, source, "init", "-b", "master")
	git(t, source, "config", "user.email", "test@example.com")
	git(t, source, "config", "user.name", "测试")
	git(t, source, "add", ".")
	git(t, source, "commit", "-m", "来源基线")
	git(t, source, "remote", "add", "origin", "ssh://fixed/rsh-flow-components.git")

	mustMkdir(t, filepath.Join(workspace, "form-runtime", "runtime-source", "src"))
	mustMkdir(t, filepath.Join(workspace, "form-runtime", "src"))
	mustWrite(t, filepath.Join(workspace, "form-runtime", "runtime-source", "src", "component.js"), "export default 'source'\n")
	mustWrite(t, filepath.Join(workspace, "form-runtime", "runtime-source", "src", "main.js"), "const components = [{ name: 'custom-one', component: {} }]\n")
	mustWrite(t, filepath.Join(workspace, "form-runtime", "runtime-source", "package.json"), `{"name":"source"}`)
	mustWrite(t, filepath.Join(workspace, "form-runtime", "src", "adapter.js"), "export const protectedAdapter = true\n")
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sync-config.js", "sync.js", "sync-check.js"} {
		content, readErr := os.ReadFile(filepath.Join(projectRoot, "form-runtime", "scripts", name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		mustWrite(t, filepath.Join(workspace, "form-runtime", "scripts", name), string(content))
	}
	manifest := formruntimemaintenance.Manifest{
		Repository: "rsh-flow-components", SourceRoot: "reference/rsh-flow-components",
		SourceRemote: "ssh://fixed/rsh-flow-components.git", SourceBranch: "master",
		Mappings: []formruntimemaintenance.Mapping{
			{Source: "src", Target: "runtime-source/src", Type: "directory"},
			{Source: "package.json", Target: "runtime-source/package.json", Type: "file"},
		},
		GeneratedTargetPaths: []string{"runtime-source/.f007-source.json"},
		ProtectedLocalPaths:  []string{"src", "scripts"},
	}
	manifestContent, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(workspace, "form-runtime", "sync-manifest.json"), string(manifestContent))
	head := strings.TrimSpace(git(t, source, "rev-parse", "HEAD"))
	metadata := fmt.Sprintf(`{"repository":"rsh-flow-components","remote":"ssh://fixed/rsh-flow-components.git","branch":"master","head":"%s","digest":"%s","targetDigest":"%s"}`, head, strings.Repeat("b", 64), testDirectoryDigest(t, filepath.Join(workspace, "form-runtime", "runtime-source")))
	mustWrite(t, filepath.Join(workspace, "form-runtime", "runtime-source", ".f007-source.json"), metadata)
	git(t, workspace, "init", "-b", "main")
	git(t, workspace, "config", "user.email", "test@example.com")
	git(t, workspace, "config", "user.name", "测试")
	git(t, workspace, "add", ".")
	git(t, workspace, "commit", "-m", "项目基线")

	inspector, err := formruntimemaintenance.NewGitSourceInspector(workspace, manifest, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	syncer, err := formruntimemaintenance.NewSyncer(workspace, source, manifest)
	if err != nil {
		t.Fatal(err)
	}
	return maintenanceFixture{workspace: workspace, source: source, manifest: manifest, inspector: inspector, syncer: syncer}
}

// testDirectoryDigest 按生产摘要规则计算测试运行源码，排除元数据自身。
func testDirectoryDigest(t *testing.T, root string) string {
	t.Helper()
	paths := make([]string, 0)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if filepath.ToSlash(relative) != ".f007-source.json" {
				paths = append(paths, filepath.ToSlash(relative))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, relative := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = hash.Write([]byte(relative + "\x00"))
		_, _ = hash.Write(content)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// git 在测试仓库执行固定 Git 命令。
func git(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v 失败：%v %s", args, err, output)
	}
	return string(output)
}

// mustMkdir 创建测试目录。
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// mustWrite 写入测试文件。
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestFormRuntimeMaintenanceSourceAndSyncGuards 验证固定来源、脏源、映射精确性、本地适配保护和幂等。
func TestFormRuntimeMaintenanceSourceAndSyncGuards(t *testing.T) {
	fixture := newMaintenanceFixture(t)
	state, err := fixture.inspector.Inspect(context.Background())
	if err != nil || state.Dirty || len(state.Head) != 40 {
		t.Fatalf("固定来源检查失败：state=%+v err=%v", state, err)
	}
	target := filepath.Join(fixture.workspace, ".runtime", "form-runtime-maintenance", "workspaces", "job-1", "runtime-source")
	var syncOutput bytes.Buffer
	if err := fixture.syncer.Sync(context.Background(), state, target, &syncOutput); err != nil {
		t.Fatalf("%v\n%s", err, syncOutput.String())
	}
	if err := fixture.syncer.Check(context.Background(), state, target, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := fixture.syncer.Sync(context.Background(), state, target, io.Discard); err != nil {
		t.Fatalf("重复同步必须幂等：%v", err)
	}
	adapter, _ := os.ReadFile(filepath.Join(fixture.workspace, "form-runtime", "src", "adapter.js"))
	if !strings.Contains(string(adapter), "protectedAdapter") {
		t.Fatal("同步覆盖了本地适配层")
	}

	mustWrite(t, filepath.Join(fixture.source, "uncommitted.js"), "dirty")
	dirty, err := fixture.inspector.Inspect(context.Background())
	if err != nil || !dirty.Dirty {
		t.Fatalf("脏来源未被识别：state=%+v err=%v", dirty, err)
	}
	service := formruntimemaintenance.NewService(fixture.inspector, formruntimemaintenance.NewMemoryStore(time.Now), formruntimemaintenance.NewMemoryLogStore())
	if _, err := service.CreateJob(context.Background()); !errors.Is(err, formruntimemaintenance.ErrSourceInvalid) {
		t.Fatalf("脏来源仍创建任务：%v", err)
	}
}

// TestFormRuntimeMaintenanceRejectsHeadAndTargetChanges 验证新 HEAD 可建任务、旧任务快照变化与运行源码修改会被拒绝。
func TestFormRuntimeMaintenanceRejectsHeadAndTargetChanges(t *testing.T) {
	fixture := newMaintenanceFixture(t)
	oldState, err := fixture.inspector.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(fixture.source, "src", "component.js"), "export default 'new head'\n")
	git(t, fixture.source, "add", ".")
	git(t, fixture.source, "commit", "-m", "来源更新")
	newState, err := fixture.inspector.Inspect(context.Background())
	if err != nil || newState.Head == oldState.Head {
		t.Fatalf("固定分支的新 HEAD 应可被任务记录：state=%+v err=%v", newState, err)
	}
	target := filepath.Join(fixture.workspace, ".runtime", "form-runtime-maintenance", "workspaces", "job-2", "runtime-source")
	if err := fixture.syncer.Sync(context.Background(), oldState, target, io.Discard); err == nil {
		t.Fatal("来源 HEAD 在任务执行前变化却未拒绝")
	}
	mustWrite(t, filepath.Join(fixture.workspace, "form-runtime", "runtime-source", "src", "component.js"), "local change")
	operator, _ := newTestPnpmOperator(t, fixture, nil, healthCheckerFunc(func(context.Context, string, string) error { return nil }))
	if err := operator.Sync(context.Background(), 2, newState, io.Discard); !errors.Is(err, formruntimemaintenance.ErrTargetModified) {
		t.Fatalf("实际运行源码未提交修改未拒绝：%v", err)
	}
}

type commandRunnerFunc func(context.Context, formruntimemaintenance.Command, io.Writer) error

// Run 调用测试函数模拟 pnpm 构建。
func (f commandRunnerFunc) Run(ctx context.Context, command formruntimemaintenance.Command, output io.Writer) error {
	return f(ctx, command, output)
}

type healthCheckerFunc func(context.Context, string, string) error

// Check 调用测试函数模拟静态与 HTTP 健康检查。
func (f healthCheckerFunc) Check(ctx context.Context, directory, url string) error {
	return f(ctx, directory, url)
}

// writeRuntimeBuild 写入最小可验证运行时产物。
func writeRuntimeBuild(t *testing.T, directory, marker string) {
	t.Helper()
	mustWrite(t, filepath.Join(directory, "index.html"), `<script src="assets/app.js"></script>`)
	mustWrite(t, filepath.Join(directory, "assets", "app.js"), marker)
	metadata := formruntimemaintenance.RuntimeBuildMetadata{Service: "rsh-flow-components", SourceRepository: "rsh-flow-components", SourceBranch: "master", SourceHead: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64), BuildHash: marker}
	content, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(directory, "runtime-health.json"), string(content))
}

// newTestPnpmOperator 创建隔离 pnpm 算子。
func newTestPnpmOperator(t *testing.T, fixture maintenanceFixture, runner formruntimemaintenance.CommandRunner, checker formruntimemaintenance.HealthChecker) (*formruntimemaintenance.PnpmOperator, string) {
	t.Helper()
	live := filepath.Join(fixture.workspace, "web", "dist", "form-runtime")
	writeRuntimeBuild(t, live, "previous")
	operator, err := formruntimemaintenance.NewPnpmOperator(formruntimemaintenance.PnpmOperatorOptions{
		WorkspaceRoot: fixture.workspace, RuntimeDir: filepath.Join(fixture.workspace, "form-runtime"), LiveDir: live,
		LiveSourceDir: filepath.Join(fixture.workspace, "form-runtime", "runtime-source"),
		StateRoot:     filepath.Join(fixture.workspace, ".runtime", "form-runtime-maintenance"), HealthURL: "http://runtime.test/health",
	}, fixture.syncer, runner, checker)
	if err != nil {
		t.Fatal(err)
	}
	return operator, live
}

// TestPnpmOperatorBuildFailureKeepsCurrentAndHealthFailureRollsBack 验证候选构建失败不影响当前版本、健康失败回退 previous。
func TestPnpmOperatorBuildFailureKeepsCurrentAndHealthFailureRollsBack(t *testing.T) {
	fixture := newMaintenanceFixture(t)
	failing, live := newTestPnpmOperator(t, fixture, commandRunnerFunc(func(context.Context, formruntimemaintenance.Command, io.Writer) error {
		return errors.New("build failed")
	}), healthCheckerFunc(func(_ context.Context, directory, _ string) error {
		if _, err := os.Stat(filepath.Join(directory, "index.html")); err != nil {
			return err
		}
		return nil
	}))
	if _, err := failing.BuildCandidate(context.Background(), 1, io.Discard); err == nil {
		t.Fatal("候选构建失败未返回错误")
	}
	current, _ := os.ReadFile(filepath.Join(live, "assets", "app.js"))
	if string(current) != "previous" {
		t.Fatal("构建失败改变了当前运行时")
	}

	runner := commandRunnerFunc(func(_ context.Context, command formruntimemaintenance.Command, _ io.Writer) error {
		writeRuntimeBuild(t, command.Env["FORM_RUNTIME_OUT_DIR"], "candidate")
		return nil
	})
	checker := healthCheckerFunc(func(_ context.Context, directory, url string) error {
		content, err := os.ReadFile(filepath.Join(directory, "assets", "app.js"))
		if err != nil {
			return err
		}
		if url != "" && string(content) == "candidate" {
			return errors.New("candidate unhealthy")
		}
		return nil
	})
	operator, live := newTestPnpmOperator(t, fixture, runner, checker)
	state, err := fixture.inspector.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := operator.Sync(context.Background(), 2, state, io.Discard); err != nil {
		t.Fatal(err)
	}
	previous, err := operator.CurrentVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := operator.BuildCandidate(context.Background(), 2, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	err = operator.Restart(context.Background(), candidate, previous, io.Discard)
	var recovery *formruntimemaintenance.RecoveryError
	if !errors.As(err, &recovery) || recovery.Status != formruntimemaintenance.RecoverySucceeded {
		t.Fatalf("健康失败没有确认回退：%v", err)
	}
	current, _ = os.ReadFile(filepath.Join(live, "assets", "app.js"))
	if string(current) != "previous" {
		t.Fatalf("previous 未恢复：%s", current)
	}
}

// TestPnpmOperatorBuildConsumesSynchronizedCandidate 验证来源代表性改动进入实际构建输入和候选产物，而非只更新闲置快照。
func TestPnpmOperatorBuildConsumesSynchronizedCandidate(t *testing.T) {
	fixture := newMaintenanceFixture(t)
	mustWrite(t, filepath.Join(fixture.source, "src", "component.js"), "export default 'candidate-source-change'\n")
	git(t, fixture.source, "add", ".")
	git(t, fixture.source, "commit", "-m", "候选源码更新")
	state, err := fixture.inspector.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runner := commandRunnerFunc(func(_ context.Context, command formruntimemaintenance.Command, _ io.Writer) error {
		content, readErr := os.ReadFile(filepath.Join(command.Env["FORM_RUNTIME_SOURCE_DIR"], "src", "component.js"))
		if readErr != nil {
			return readErr
		}
		writeRuntimeBuild(t, command.Env["FORM_RUNTIME_OUT_DIR"], string(content))
		return nil
	})
	operator, _ := newTestPnpmOperator(t, fixture, runner, healthCheckerFunc(func(context.Context, string, string) error { return nil }))
	if err := operator.Sync(context.Background(), 3, state, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := operator.SyncCheck(context.Background(), 3, state, io.Discard); err != nil {
		t.Fatal(err)
	}
	version, err := operator.BuildCandidate(context.Background(), 3, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := os.ReadFile(filepath.Join(fixture.workspace, ".runtime", "form-runtime-maintenance", "versions", version, "assets", "app.js"))
	if err != nil || !strings.Contains(string(artifact), "candidate-source-change") {
		t.Fatalf("同步变更没有进入候选构建产物：%s err=%v", artifact, err)
	}
}

type fakeRuntimeOperator struct {
	current      string
	restartCalls int
	verifyCalls  int
}

// Sync 模拟同步成功。
func (o *fakeRuntimeOperator) Sync(context.Context, uint64, formruntimemaintenance.SourceState, io.Writer) error {
	return nil
}

// SyncCheck 模拟同步校验成功。
func (o *fakeRuntimeOperator) SyncCheck(context.Context, uint64, formruntimemaintenance.SourceState, io.Writer) error {
	return nil
}

// BuildCandidate 返回固定候选。
func (o *fakeRuntimeOperator) BuildCandidate(context.Context, uint64, io.Writer) (string, error) {
	return "candidate-job-1", nil
}

// CurrentVersion 返回当前版本。
func (o *fakeRuntimeOperator) CurrentVersion(context.Context) (string, error) { return o.current, nil }

// Restart 记录恢复阶段是否重新切换候选。
func (o *fakeRuntimeOperator) Restart(_ context.Context, candidate, _ string, _ io.Writer) error {
	o.restartCalls++
	o.current = candidate
	return nil
}

// Verify 记录最终健康核验。
func (o *fakeRuntimeOperator) Verify(context.Context, string, string, io.Writer) error {
	o.verifyCalls++
	return nil
}

type fixedInspector struct {
	state formruntimemaintenance.SourceState
}

// Inspect 返回固定来源快照。
func (i fixedInspector) Inspect(context.Context) (formruntimemaintenance.SourceState, error) {
	return i.state, nil
}

// TestMaintenanceStoreLeaseAndResume 验证单活动任务、fencing 和 RESTART 崩溃恢复。
func TestMaintenanceStoreLeaseAndResume(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	store := formruntimemaintenance.NewMemoryStore(func() time.Time { return now })
	source := formruntimemaintenance.SourceState{Repository: "fixed", Branch: "master", Head: strings.Repeat("a", 40), InspectedAt: now}
	job, err := store.Create(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), source); !errors.Is(err, formruntimemaintenance.ErrJobAlreadyActive) {
		t.Fatalf("单活动任务未生效：%v", err)
	}
	first, _ := store.ClaimNext(context.Background(), formruntimemaintenance.Claim{WorkerID: "worker-1", LeaseDuration: time.Minute})
	if err := store.UpdateProgress(context.Background(), formruntimemaintenance.Progress{ID: job.ID, WorkerID: "worker-1", FencingToken: first.FencingToken, LeaseDuration: time.Minute, Stage: formruntimemaintenance.StageRestart, Candidate: "candidate-job-1", Previous: "bootstrap-123456789012"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	second, _ := store.ClaimNext(context.Background(), formruntimemaintenance.Claim{WorkerID: "worker-2", LeaseDuration: time.Minute})
	if err := store.UpdateProgress(context.Background(), formruntimemaintenance.Progress{ID: job.ID, WorkerID: "worker-1", FencingToken: first.FencingToken, LeaseDuration: time.Minute, Stage: formruntimemaintenance.StageVerify}); !errors.Is(err, formruntimemaintenance.ErrStaleLease) {
		t.Fatalf("旧 fencing token 仍能写入：%v", err)
	}
	// 让 Pipeline 使用第二次领取相同 worker；租约到期后才能再次领取。
	now = now.Add(2 * time.Minute)
	operator := &fakeRuntimeOperator{current: "bootstrap-123456789012"}
	pipeline := formruntimemaintenance.NewPipeline(store, fixedInspector{state: source}, operator, formruntimemaintenance.NewMemoryLogStore(), formruntimemaintenance.WorkerOptions{WorkerID: "worker-3", LeaseDuration: time.Minute, RenewalInterval: time.Hour})
	processed, err := pipeline.ProcessNext(context.Background())
	if err != nil || !processed || operator.restartCalls != 1 || operator.verifyCalls != 1 {
		t.Fatalf("恢复阶段未继续切换与核验：processed=%v err=%v restart=%d verify=%d second=%+v", processed, err, operator.restartCalls, operator.verifyCalls, second)
	}
	completed, _ := store.Get(context.Background(), job.ID)
	if completed.Status != formruntimemaintenance.JobSucceeded || completed.AttemptCount != 3 {
		t.Fatalf("恢复任务最终状态不正确：%+v", completed)
	}
}

// TestMaintenanceLogTruncationAndHTTPHealth 验证日志尾部截断和真实 HTTP 快照轮询。
func TestMaintenanceLogTruncationAndHTTPHealth(t *testing.T) {
	logs, err := formruntimemaintenance.NewFileLogStore(t.TempDir(), 8)
	if err != nil {
		t.Fatal(err)
	}
	writer, _ := logs.Open(context.Background(), 9)
	_, _ = writer.Write([]byte("0123456789abcdef"))
	_ = writer.Close()
	log, err := logs.Read(context.Background(), 9)
	if err != nil || !log.Truncated || log.Content != "89abcdef" {
		t.Fatalf("日志截断不正确：log=%+v err=%v", log, err)
	}
	directory := t.TempDir()
	writeRuntimeBuild(t, directory, "healthy")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests++
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusOK)
		if requests == 1 {
			_, _ = response.Write([]byte(`{"service":"rsh-flow-components","sourceRepository":"rsh-flow-components","sourceBranch":"master","sourceHead":"cccccccccccccccccccccccccccccccccccccccc","sourceDigest":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","buildHash":"previous"}`))
			return
		}
		_, _ = response.Write([]byte(`{"service":"rsh-flow-components","sourceRepository":"rsh-flow-components","sourceBranch":"master","sourceHead":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","sourceDigest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","buildHash":"served"}`))
	}))
	defer server.Close()
	if err := formruntimemaintenance.NewStaticHTTPHealthChecker().Check(context.Background(), directory, server.URL); err != nil {
		t.Fatalf("pnpm 运行时健康检查失败：%v", err)
	}
	if requests < 2 {
		t.Fatal("健康检查没有等待实际服务从 previous 切换到候选")
	}
}

// TestManifestRejectsProtectedMapping 验证同步清单不能覆盖本地适配层。
func TestManifestRejectsProtectedMapping(t *testing.T) {
	workspace := t.TempDir()
	manifestPath := filepath.Join(workspace, "manifest.json")
	mustWrite(t, manifestPath, `{"repository":"fixed","sourceRoot":"source","sourceRemote":"ssh://fixed","sourceBranch":"master","mappings":[{"source":"src","target":"runtime-source/src","type":"directory"}],"protectedLocalPaths":["runtime-source/src"]}`)
	if _, err := formruntimemaintenance.LoadManifest(workspace, manifestPath); !errors.Is(err, formruntimemaintenance.ErrSourceInvalid) {
		t.Fatalf("覆盖本地适配层的清单未拒绝：%v", err)
	}
}
