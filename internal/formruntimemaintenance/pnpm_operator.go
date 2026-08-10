package formruntimemaintenance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var versionNamePattern = regexp.MustCompile(`^(candidate-job-[1-9][0-9]*|bootstrap-[a-f0-9]{12})$`)

// Command 描述固定 pnpm 构建命令；API 不可覆盖名称、参数或目录。
type Command struct {
	Name string
	Args []string
	Dir  string
	Env  map[string]string
}

// CommandRunner 抽象 pnpm 命令，便于证明构建失败不会切换 live 源码或产物。
type CommandRunner interface {
	Run(context.Context, Command, io.Writer) error
}

// ExecCommandRunner 直接执行固定命令，不经过 shell。
type ExecCommandRunner struct{}

// Run 执行命令并同时写出 stdout/stderr。
func (ExecCommandRunner) Run(ctx context.Context, command Command, output io.Writer) error {
	process := exec.CommandContext(ctx, command.Name, command.Args...)
	process.Dir = command.Dir
	process.Stdout = output
	process.Stderr = output
	process.Env = append(os.Environ(), sortedEnvironment(command.Env)...)
	if err := process.Run(); err != nil {
		return fmt.Errorf("执行 %s: %w", command.Name, err)
	}
	return nil
}

// RuntimeBuildMetadata 是候选产物与运行中服务共同公开的源码快照。
type RuntimeBuildMetadata struct {
	Service          string `json:"service"`
	SourceRepository string `json:"sourceRepository"`
	SourceBranch     string `json:"sourceBranch"`
	SourceHead       string `json:"sourceHead"`
	SourceDigest     string `json:"sourceDigest"`
	BuildHash        string `json:"buildHash"`
}

// HealthChecker 核验静态候选，并在切换后通过真实 HTTP 服务确认同一源码快照已经生效。
type HealthChecker interface {
	Check(context.Context, string, string) error
}

// StaticHTTPHealthChecker 验证构建资源，并轮询运行服务直到它报告本次候选源码。
type StaticHTTPHealthChecker struct {
	client       *http.Client
	timeout      time.Duration
	pollInterval time.Duration
}

// NewStaticHTTPHealthChecker 创建带真实 HTTP 等待边界的健康检查器。
func NewStaticHTTPHealthChecker() *StaticHTTPHealthChecker {
	return &StaticHTTPHealthChecker{
		client:       &http.Client{Timeout: 3 * time.Second},
		timeout:      30 * time.Second,
		pollInterval: 250 * time.Millisecond,
	}
}

// Check 确认候选有完整入口与 JS；提供 URL 时必须等到运行服务报告相同 HEAD 和摘要。
func (c *StaticHTTPHealthChecker) Check(ctx context.Context, directory, healthURL string) error {
	expected, err := readRuntimeBuildMetadata(directory)
	if err != nil {
		return err
	}
	javascriptFound := false
	err = filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".js") {
			javascriptFound = true
		}
		return nil
	})
	if err != nil || !javascriptFound {
		return errors.New("表单运行时候选缺少 JavaScript 资源")
	}
	if strings.TrimSpace(healthURL) == "" {
		return nil
	}
	deadline := time.Now().Add(c.timeout)
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		actual, requestErr := c.readHTTP(ctx, healthURL)
		if requestErr == nil && sameRuntimeSource(expected, actual) {
			return nil
		}
		if requestErr != nil {
			lastErr = requestErr
		} else {
			lastErr = fmt.Errorf("运行服务仍是 %s/%s", actual.SourceHead, actual.SourceDigest)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("表单运行时 HTTP 健康检查未切换到候选: %w", lastErr)
		}
		timer := time.NewTimer(c.pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// readHTTP 读取运行中服务的编译后快照，不把普通 index.html 成功误当成候选生效。
func (c *StaticHTTPHealthChecker) readHTTP(ctx context.Context, healthURL string) (RuntimeBuildMetadata, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return RuntimeBuildMetadata{}, err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return RuntimeBuildMetadata{}, fmt.Errorf("访问表单运行时健康地址: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return RuntimeBuildMetadata{}, fmt.Errorf("表单运行时健康地址返回 HTTP %d", response.StatusCode)
	}
	var metadata RuntimeBuildMetadata
	if err := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&metadata); err != nil {
		return RuntimeBuildMetadata{}, fmt.Errorf("解析表单运行时健康响应: %w", err)
	}
	return metadata, nil
}

// readRuntimeBuildMetadata 读取只有成功编译才会生成的候选快照。
func readRuntimeBuildMetadata(directory string) (RuntimeBuildMetadata, error) {
	content, err := os.ReadFile(filepath.Join(directory, "runtime-health.json"))
	if err != nil {
		return RuntimeBuildMetadata{}, fmt.Errorf("读取表单运行时编译快照: %w", err)
	}
	var metadata RuntimeBuildMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return RuntimeBuildMetadata{}, fmt.Errorf("解析表单运行时编译快照: %w", err)
	}
	if metadata.Service != "rsh-flow-components" || len(metadata.SourceHead) != 40 || len(metadata.SourceDigest) != 64 || metadata.BuildHash == "" {
		return RuntimeBuildMetadata{}, errors.New("表单运行时编译快照不完整")
	}
	return metadata, nil
}

// sameRuntimeSource 只比较更新正确性所需的仓库、分支、HEAD 与输入摘要，开发和生产构建 hash 可以不同。
func sameRuntimeSource(expected, actual RuntimeBuildMetadata) bool {
	return expected.Service == actual.Service && expected.SourceRepository == actual.SourceRepository &&
		expected.SourceBranch == actual.SourceBranch && expected.SourceHead == actual.SourceHead && expected.SourceDigest == actual.SourceDigest
}

// PnpmOperatorOptions 固定候选源码、pnpm 构建、live 源码/产物和运行状态目录。
type PnpmOperatorOptions struct {
	WorkspaceRoot string
	RuntimeDir    string
	LiveSourceDir string
	LiveDir       string
	StateRoot     string
	HealthURL     string
}

// PnpmOperator 用隔离候选源码与版本产物替代旧 V2 Docker 镜像，并保留 previous 回退语义。
type PnpmOperator struct {
	workspaceRoot string
	runtimeDir    string
	liveSourceDir string
	liveDir       string
	stateRoot     string
	healthURL     string
	syncer        *Syncer
	runner        CommandRunner
	health        HealthChecker
}

// NewPnpmOperator 创建算子并限制所有可写目录在当前项目内，健康 URL 不能为空。
func NewPnpmOperator(options PnpmOperatorOptions, syncer *Syncer, runner CommandRunner, health HealthChecker) (*PnpmOperator, error) {
	workspace, err := filepath.Abs(filepath.Clean(options.WorkspaceRoot))
	if err != nil {
		return nil, err
	}
	runtimeDir, err := filepath.Abs(filepath.Clean(options.RuntimeDir))
	if err != nil {
		return nil, err
	}
	liveSourceDir, err := filepath.Abs(filepath.Clean(options.LiveSourceDir))
	if err != nil {
		return nil, err
	}
	liveDir, err := filepath.Abs(filepath.Clean(options.LiveDir))
	if err != nil {
		return nil, err
	}
	stateRoot, err := filepath.Abs(filepath.Clean(options.StateRoot))
	if err != nil {
		return nil, err
	}
	if !within(workspace, runtimeDir) || !within(workspace, liveSourceDir) || !within(workspace, liveDir) || !within(workspace, stateRoot) || syncer == nil {
		return nil, errors.New("表单运行时维护目录必须位于当前项目内")
	}
	if strings.TrimSpace(options.HealthURL) == "" {
		return nil, errors.New("表单运行时维护必须配置真实 HTTP 健康地址")
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if health == nil {
		health = NewStaticHTTPHealthChecker()
	}
	return &PnpmOperator{
		workspaceRoot: workspace, runtimeDir: runtimeDir, liveSourceDir: liveSourceDir, liveDir: liveDir,
		stateRoot: stateRoot, healthURL: strings.TrimSpace(options.HealthURL), syncer: syncer, runner: runner, health: health,
	}, nil
}

// within 判断路径是否位于固定根目录，阻止版本名或配置逃逸。
func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// Sync 从当前可用源码复制隔离候选，再用任务 HEAD 执行原生 sync.js；失败不触碰 live。
func (o *PnpmOperator) Sync(ctx context.Context, jobID uint64, source SourceState, output io.Writer) error {
	if jobID == 0 {
		return errors.New("非法维护任务编号")
	}
	if err := o.ensureLiveSourceUnmodified(ctx); err != nil {
		return err
	}
	workspace := o.jobWorkspace(jobID)
	if err := os.RemoveAll(workspace); err != nil {
		return err
	}
	candidateSource := o.candidateSourceDir(jobID)
	if err := copyDirectory(o.liveSourceDir, candidateSource); err != nil {
		return fmt.Errorf("准备候选源码: %w", err)
	}
	return o.syncer.Sync(ctx, source, candidateSource, output)
}

// SyncCheck 对同一隔离候选执行原生 sync-check.js，构建不会读取闲置快照。
func (o *PnpmOperator) SyncCheck(ctx context.Context, jobID uint64, source SourceState, output io.Writer) error {
	return o.syncer.Check(ctx, source, o.candidateSourceDir(jobID), output)
}

// ensureLiveSourceUnmodified 拒绝覆盖当前项目中尚未提交的运行时源码现场。
func (o *PnpmOperator) ensureLiveSourceUnmodified(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// 维护任务会受控替换 tracked 源码，因此不能把任务自身的合法更新误判为 Git 脏文件；
	// 完整目标摘要同时能识别已跟踪和未跟踪的人工改动，并允许已部署 previous 继续更新。
	if _, err := readRuntimeSourceMetadata(o.liveSourceDir); err != nil {
		return fmt.Errorf("%w: %v", ErrTargetModified, err)
	}
	return nil
}

// BuildCandidate 用实际候选 runtime-source 构建版本产物，成功前绝不触碰 live 源码或服务。
func (o *PnpmOperator) BuildCandidate(ctx context.Context, jobID uint64, output io.Writer) (string, error) {
	if jobID == 0 {
		return "", errors.New("非法维护任务编号")
	}
	version := fmt.Sprintf("candidate-job-%d", jobID)
	destination := o.versionDir(version)
	if err := os.RemoveAll(destination); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}
	candidateSource := o.candidateSourceDir(jobID)
	_, _ = fmt.Fprintf(output, "$ pnpm --dir form-runtime build（候选 %s，源码 %s）\n", version, candidateSource)
	err := o.runner.Run(ctx, Command{
		Name: "pnpm", Args: []string{"--dir", o.runtimeDir, "build"}, Dir: o.workspaceRoot,
		Env: map[string]string{"FORM_RUNTIME_OUT_DIR": destination, "FORM_RUNTIME_SOURCE_DIR": candidateSource},
	}, output)
	if err != nil {
		_ = os.RemoveAll(destination)
		return "", err
	}
	if err := o.health.Check(ctx, destination, ""); err != nil {
		_ = os.RemoveAll(destination)
		return "", fmt.Errorf("候选静态健康检查失败: %w", err)
	}
	if err := copyDirectory(candidateSource, o.sourceVersionDir(version)); err != nil {
		_ = os.RemoveAll(destination)
		return "", fmt.Errorf("保存候选源码版本: %w", err)
	}
	return version, nil
}

// CurrentVersion 返回 live 版本；首次运行会把实际源码与产物一起快照成 bootstrap previous。
func (o *PnpmOperator) CurrentVersion(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	liveMetadata, err := readRuntimeSourceMetadata(o.liveSourceDir)
	if err != nil {
		return "", fmt.Errorf("当前表单运行时源码不可作为 previous: %w", err)
	}
	var state deploymentState
	content, err := os.ReadFile(o.statePath())
	if err == nil && json.Unmarshal(content, &state) == nil && validVersion(state.Current) {
		if versionMetadata, sourceErr := readRuntimeSourceMetadata(o.sourceVersionDir(state.Current)); sourceErr == nil && sameRuntimeSourceMetadata(liveMetadata, versionMetadata) {
			return state.Current, nil
		}
	}
	version := "bootstrap-" + liveMetadata.TargetDigest[:12]
	if _, err := os.Stat(o.versionDir(version)); errors.Is(err, os.ErrNotExist) {
		if staticErr := o.health.Check(ctx, o.liveDir, ""); staticErr == nil {
			if err := copyDirectory(o.liveDir, o.versionDir(version)); err != nil {
				return "", err
			}
		} else {
			if err := o.runner.Run(ctx, Command{
				Name: "pnpm", Args: []string{"--dir", o.runtimeDir, "build"}, Dir: o.workspaceRoot,
				Env: map[string]string{"FORM_RUNTIME_OUT_DIR": o.versionDir(version), "FORM_RUNTIME_SOURCE_DIR": o.liveSourceDir},
			}, io.Discard); err != nil {
				return "", fmt.Errorf("构建首次 previous: %w", err)
			}
			if err := o.health.Check(ctx, o.versionDir(version), ""); err != nil {
				_ = os.RemoveAll(o.versionDir(version))
				return "", fmt.Errorf("核验首次 previous: %w", err)
			}
		}
	}
	if err := copyDirectory(o.liveSourceDir, o.sourceVersionDir(version)); err != nil {
		return "", err
	}
	if err := o.writeState(version); err != nil {
		return "", err
	}
	return version, nil
}

// sameRuntimeSourceMetadata 比较 live 与持久版本的完整源码事实，避免 current 指针掩盖人工改动。
func sameRuntimeSourceMetadata(left, right runtimeSourceMetadata) bool {
	return left.Repository == right.Repository && left.Branch == right.Branch && left.Head == right.Head &&
		left.Digest == right.Digest && left.TargetDigest == right.TargetDigest
}

// Restart 同时切换真实运行源码与生产产物，并要求运行中的 HTTP 服务报告候选快照。
func (o *PnpmOperator) Restart(ctx context.Context, candidate, previous string, output io.Writer) error {
	if !validVersion(candidate) || !validVersion(previous) {
		return errors.New("候选或 previous 版本名不受支持")
	}
	if err := o.deploy(ctx, candidate); err == nil {
		if healthErr := o.health.Check(ctx, o.liveDir, o.healthURL); healthErr == nil {
			return nil
		} else {
			err = fmt.Errorf("候选切换后 HTTP 健康检查失败: %w", healthErr)
		}
		return o.rollback(ctx, previous, err, output)
	} else {
		return o.rollback(ctx, previous, fmt.Errorf("候选切换失败: %w", err), output)
	}
}

// Verify 再次确认源码/产物版本和真实 HTTP 服务；迟到健康失败仍恢复 previous。
func (o *PnpmOperator) Verify(ctx context.Context, candidate, previous string, output io.Writer) error {
	current, err := o.CurrentVersion(ctx)
	if err == nil && current == candidate {
		err = o.health.Check(ctx, o.liveDir, o.healthURL)
	} else if err == nil {
		err = fmt.Errorf("当前版本 %s 与候选 %s 不一致", current, candidate)
	}
	if err == nil {
		return nil
	}
	return o.rollback(ctx, previous, fmt.Errorf("最终 HTTP 健康核验失败: %w", err), output)
}

// deploy 原子替换 live 源码和产物；任一步失败都恢复替换前现场。
func (o *PnpmOperator) deploy(ctx context.Context, version string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	buildSource := o.versionDir(version)
	runtimeSource := o.sourceVersionDir(version)
	if err := o.health.Check(ctx, buildSource, ""); err != nil {
		return err
	}
	if _, err := readRuntimeSourceMetadata(runtimeSource); err != nil {
		return err
	}
	nextBuild, previousBuild := o.liveDir+".candidate", o.liveDir+".previous"
	nextSource, previousSource := o.liveSourceDir+".candidate", o.liveSourceDir+".previous"
	for _, directory := range []string{nextBuild, previousBuild, nextSource, previousSource} {
		if err := os.RemoveAll(directory); err != nil {
			return err
		}
	}
	if err := copyDirectory(buildSource, nextBuild); err != nil {
		return err
	}
	if err := copyDirectory(runtimeSource, nextSource); err != nil {
		_ = os.RemoveAll(nextBuild)
		return err
	}
	if err := moveIfPresent(o.liveDir, previousBuild); err != nil {
		return err
	}
	if err := moveIfPresent(o.liveSourceDir, previousSource); err != nil {
		_ = restoreMoved(previousBuild, o.liveDir)
		return err
	}
	if err := os.Rename(nextBuild, o.liveDir); err != nil {
		_ = restoreMoved(previousSource, o.liveSourceDir)
		_ = restoreMoved(previousBuild, o.liveDir)
		return err
	}
	if err := os.Rename(nextSource, o.liveSourceDir); err != nil {
		_ = os.RemoveAll(o.liveDir)
		_ = restoreMoved(previousSource, o.liveSourceDir)
		_ = restoreMoved(previousBuild, o.liveDir)
		return err
	}
	if err := o.writeState(version); err != nil {
		_ = os.RemoveAll(o.liveDir)
		_ = os.RemoveAll(o.liveSourceDir)
		_ = restoreMoved(previousSource, o.liveSourceDir)
		_ = restoreMoved(previousBuild, o.liveDir)
		return err
	}
	_ = os.RemoveAll(previousBuild)
	_ = os.RemoveAll(previousSource)
	return nil
}

// moveIfPresent 把当前目录移到同父级 previous；目录尚不存在时允许首次部署继续。
func moveIfPresent(source, target string) error {
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Rename(source, target)
}

// restoreMoved 把 previous 恢复到 live，用于多目录切换中途失败。
func restoreMoved(source, target string) error {
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Rename(source, target)
}

// rollback 恢复 previous 的源码和产物，并再次通过同一 HTTP 地址核验。
func (o *PnpmOperator) rollback(ctx context.Context, previous string, cause error, output io.Writer) error {
	_, _ = fmt.Fprintf(output, "[ROLLBACK] 恢复 previous 版本 %s\n", previous)
	if err := o.deploy(ctx, previous); err != nil {
		return &RecoveryError{Cause: fmt.Errorf("%v；回退失败: %w", cause, err), Status: RecoveryFailed, Message: "候选版本失败，previous 版本也未能恢复。"}
	}
	if err := o.health.Check(ctx, o.liveDir, o.healthURL); err != nil {
		return &RecoveryError{Cause: fmt.Errorf("%v；回退 HTTP 健康检查失败: %w", cause, err), Status: RecoveryFailed, Message: "已恢复 previous 文件，但运行服务健康检查仍失败。"}
	}
	return &RecoveryError{Cause: cause, Status: RecoverySucceeded, Message: "候选版本失败，已恢复并验证 previous 版本。"}
}

type runtimeSourceMetadata struct {
	Repository   string `json:"repository"`
	Branch       string `json:"branch"`
	Head         string `json:"head"`
	Digest       string `json:"digest"`
	TargetDigest string `json:"targetDigest"`
}

// readRuntimeSourceMetadata 核验同步脚本写入的实际源码快照。
func readRuntimeSourceMetadata(directory string) (runtimeSourceMetadata, error) {
	content, err := os.ReadFile(filepath.Join(directory, ".f007-source.json"))
	if err != nil {
		return runtimeSourceMetadata{}, err
	}
	var metadata runtimeSourceMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return runtimeSourceMetadata{}, err
	}
	if len(metadata.Head) != 40 || len(metadata.Digest) != 64 || len(metadata.TargetDigest) != 64 {
		return runtimeSourceMetadata{}, errors.New("表单运行时源码快照不完整")
	}
	actualDigest, err := pathDigest(directory)
	if err != nil || actualDigest != metadata.TargetDigest {
		return runtimeSourceMetadata{}, errors.New("表单运行时源码存在同步任务之外的修改")
	}
	return metadata, nil
}

type deploymentState struct {
	Current   string    `json:"current"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// writeState 原子记录当前 live 版本，Worker 恢复时不靠目录名猜测。
func (o *PnpmOperator) writeState(version string) error {
	if !validVersion(version) {
		return errors.New("非法表单运行时版本名")
	}
	if err := os.MkdirAll(o.stateRoot, 0o755); err != nil {
		return err
	}
	content, err := json.Marshal(deploymentState{Current: version, UpdatedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	temporary := o.statePath() + ".candidate"
	if err := os.WriteFile(temporary, content, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, o.statePath())
}

// statePath 返回固定 current 指针路径。
func (o *PnpmOperator) statePath() string { return filepath.Join(o.stateRoot, "current.json") }

// versionDir 返回固定候选构建产物目录。
func (o *PnpmOperator) versionDir(version string) string {
	return filepath.Join(o.stateRoot, "versions", version)
}

// sourceVersionDir 返回与构建产物一一对应的完整 rsh-flow-components 源码目录。
func (o *PnpmOperator) sourceVersionDir(version string) string {
	return filepath.Join(o.stateRoot, "sources", version)
}

// jobWorkspace 返回同步和 sync-check 共用的隔离任务目录。
func (o *PnpmOperator) jobWorkspace(jobID uint64) string {
	return filepath.Join(o.stateRoot, "workspaces", fmt.Sprintf("job-%d", jobID))
}

// candidateSourceDir 返回任务实际构建消费的源码目录。
func (o *PnpmOperator) candidateSourceDir(jobID uint64) string {
	return filepath.Join(o.jobWorkspace(jobID), "runtime-source")
}

// validVersion 限制候选和 previous 名称只能来自本算子。
func validVersion(version string) bool { return versionNamePattern.MatchString(version) }

// copyDirectory 精确复制源码或构建产物，不跟随符号链接。
func copyDirectory(source, target string) error {
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("表单运行时维护不复制符号链接")
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copyFileWithMode(path, destination, info.Mode().Perm())
	})
}

// copyFileWithMode 复制部署文件并保留执行位。
func copyFileWithMode(source, target string, mode os.FileMode) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, content, mode)
}

// sortedEnvironment 让测试和日志中的环境变量顺序稳定。
func sortedEnvironment(environment map[string]string) []string {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, key+"="+environment[key])
	}
	return values
}
