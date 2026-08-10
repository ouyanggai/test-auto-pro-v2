package formruntimemaintenance

import (
	"bytes"
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

// CommandRunner 抽象 pnpm 命令，便于证明构建失败不会切换 live 目录。
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

// HealthChecker 核验当前 live 产物和可选运行中 HTTP 地址。
type HealthChecker interface {
	Check(context.Context, string, string) error
}

// StaticHTTPHealthChecker 验证入口、静态资源及可选本地 HTTP 服务。
type StaticHTTPHealthChecker struct {
	client *http.Client
}

// NewStaticHTTPHealthChecker 创建短超时健康检查器。
func NewStaticHTTPHealthChecker() *StaticHTTPHealthChecker {
	return &StaticHTTPHealthChecker{client: &http.Client{Timeout: 3 * time.Second}}
}

// Check 确认构建入口引用的资源存在；配置 URL 时再核对 pnpm 服务实际可访问。
func (c *StaticHTTPHealthChecker) Check(ctx context.Context, liveDir, healthURL string) error {
	indexPath := filepath.Join(liveDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("读取表单运行时入口: %w", err)
	}
	if !bytes.Contains(content, []byte("assets/")) {
		return errors.New("表单运行时入口没有构建资源")
	}
	assets, err := filepath.Glob(filepath.Join(liveDir, "assets", "*.js"))
	if err != nil || len(assets) == 0 {
		return errors.New("表单运行时候选缺少 JavaScript 资源")
	}
	if strings.TrimSpace(healthURL) == "" {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("访问表单运行时健康地址: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("表单运行时健康地址返回 HTTP %d", response.StatusCode)
	}
	return nil
}

// PnpmOperatorOptions 固定本地 pnpm 候选构建、live 目录和运行状态目录。
type PnpmOperatorOptions struct {
	WorkspaceRoot string
	RuntimeDir    string
	LiveDir       string
	StateRoot     string
	HealthURL     string
}

// PnpmOperator 以版本目录代替旧 V2 Docker 镜像，并保留候选/previous 回退语义。
type PnpmOperator struct {
	workspaceRoot string
	runtimeDir    string
	liveDir       string
	stateRoot     string
	healthURL     string
	syncer        *Syncer
	runner        CommandRunner
	health        HealthChecker
}

// NewPnpmOperator 创建 pnpm 运行时算子并限制所有目录在项目或运行目录内。
func NewPnpmOperator(options PnpmOperatorOptions, syncer *Syncer, runner CommandRunner, health HealthChecker) (*PnpmOperator, error) {
	workspace, err := filepath.Abs(filepath.Clean(options.WorkspaceRoot))
	if err != nil {
		return nil, err
	}
	runtimeDir, err := filepath.Abs(filepath.Clean(options.RuntimeDir))
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
	if !within(workspace, runtimeDir) || !within(workspace, liveDir) || !within(workspace, stateRoot) || syncer == nil {
		return nil, errors.New("表单运行时维护目录必须位于当前项目内")
	}
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	if health == nil {
		health = NewStaticHTTPHealthChecker()
	}
	return &PnpmOperator{workspaceRoot: workspace, runtimeDir: runtimeDir, liveDir: liveDir, stateRoot: stateRoot,
		healthURL: strings.TrimSpace(options.HealthURL), syncer: syncer, runner: runner, health: health}, nil
}

// within 判断路径是否位于固定根目录，阻止版本名或配置逃逸。
func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// Sync 执行受控清单同步，本地适配层不在映射内。
func (o *PnpmOperator) Sync(ctx context.Context, output io.Writer) error {
	return o.syncer.Sync(ctx, output)
}

// SyncCheck 校验 upstream 原样区与固定来源逐文件一致。
func (o *PnpmOperator) SyncCheck(ctx context.Context, output io.Writer) error {
	return o.syncer.Check(ctx, output)
}

// BuildCandidate 将 Vite 输出写入版本目录，构建完成前绝不触碰 live 目录。
func (o *PnpmOperator) BuildCandidate(ctx context.Context, jobID uint64, output io.Writer) (string, error) {
	if jobID == 0 {
		return "", errors.New("非法维护任务编号")
	}
	version := fmt.Sprintf("candidate-job-%d", jobID)
	destination := o.versionDir(version)
	if err := os.RemoveAll(destination); err != nil {
		return "", err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", err
	}
	_, _ = fmt.Fprintf(output, "$ pnpm --dir form-runtime build（候选 %s）\n", version)
	err := o.runner.Run(ctx, Command{Name: "pnpm", Args: []string{"--dir", o.runtimeDir, "build"}, Dir: o.workspaceRoot,
		Env: map[string]string{"FORM_RUNTIME_OUT_DIR": destination}}, output)
	if err != nil {
		_ = os.RemoveAll(destination)
		return "", err
	}
	if err := o.health.Check(ctx, destination, ""); err != nil {
		_ = os.RemoveAll(destination)
		return "", fmt.Errorf("候选静态健康检查失败: %w", err)
	}
	return version, nil
}

// CurrentVersion 返回 live 版本；首次运行时先把现有可用产物快照成 bootstrap previous。
func (o *PnpmOperator) CurrentVersion(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var state deploymentState
	content, err := os.ReadFile(o.statePath())
	if err == nil && json.Unmarshal(content, &state) == nil && validVersion(state.Current) {
		return state.Current, nil
	}
	if err := o.health.Check(ctx, o.liveDir, ""); err != nil {
		return "", fmt.Errorf("当前表单运行时不可作为 previous: %w", err)
	}
	digest, err := pathDigest(o.liveDir)
	if err != nil {
		return "", err
	}
	version := "bootstrap-" + digest[:12]
	if _, err := os.Stat(o.versionDir(version)); errors.Is(err, os.ErrNotExist) {
		if err := copyDirectory(o.liveDir, o.versionDir(version)); err != nil {
			return "", err
		}
	}
	if err := o.writeState(version); err != nil {
		return "", err
	}
	return version, nil
}

// Restart 原子替换 live 目录并立即健康检查；任一失败都恢复 previous 并再次验证。
func (o *PnpmOperator) Restart(ctx context.Context, candidate, previous string, output io.Writer) error {
	if !validVersion(candidate) || !validVersion(previous) {
		return errors.New("候选或 previous 版本名不受支持")
	}
	if err := o.deploy(ctx, candidate); err == nil {
		if healthErr := o.health.Check(ctx, o.liveDir, o.healthURL); healthErr == nil {
			return nil
		} else {
			err = fmt.Errorf("候选切换后健康检查失败: %w", healthErr)
		}
		return o.rollback(ctx, previous, err, output)
	} else {
		return o.rollback(ctx, previous, fmt.Errorf("候选切换失败: %w", err), output)
	}
}

// Verify 再次确认当前版本和健康；迟到健康失败仍恢复 previous。
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
	return o.rollback(ctx, previous, fmt.Errorf("最终健康核验失败: %w", err), output)
}

// deploy 用同一父目录中的 rename 原子替换 live，并更新持久版本指针。
func (o *PnpmOperator) deploy(ctx context.Context, version string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	source := o.versionDir(version)
	if err := o.health.Check(ctx, source, ""); err != nil {
		return err
	}
	next := o.liveDir + ".candidate"
	previousLive := o.liveDir + ".previous"
	if err := os.RemoveAll(next); err != nil {
		return err
	}
	if err := os.RemoveAll(previousLive); err != nil {
		return err
	}
	if err := copyDirectory(source, next); err != nil {
		return err
	}
	if _, err := os.Stat(o.liveDir); err == nil {
		if err := os.Rename(o.liveDir, previousLive); err != nil {
			return err
		}
	}
	if err := os.Rename(next, o.liveDir); err != nil {
		_ = os.Rename(previousLive, o.liveDir)
		return err
	}
	if err := o.writeState(version); err != nil {
		_ = os.RemoveAll(o.liveDir)
		_ = os.Rename(previousLive, o.liveDir)
		return err
	}
	_ = os.RemoveAll(previousLive)
	return nil
}

// rollback 恢复 previous 并再次健康核验，把恢复事实作为结构化错误返回。
func (o *PnpmOperator) rollback(ctx context.Context, previous string, cause error, output io.Writer) error {
	_, _ = fmt.Fprintf(output, "[ROLLBACK] 恢复 previous 版本 %s\n", previous)
	if err := o.deploy(ctx, previous); err != nil {
		return &RecoveryError{Cause: fmt.Errorf("%v；回退失败: %w", cause, err), Status: RecoveryFailed, Message: "候选版本失败，previous 版本也未能恢复。"}
	}
	if err := o.health.Check(ctx, o.liveDir, o.healthURL); err != nil {
		return &RecoveryError{Cause: fmt.Errorf("%v；回退健康检查失败: %w", cause, err), Status: RecoveryFailed, Message: "已尝试恢复 previous 版本，但健康检查仍失败。"}
	}
	return &RecoveryError{Cause: cause, Status: RecoverySucceeded, Message: "候选版本失败，已恢复并验证 previous 版本。"}
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
func (o *PnpmOperator) statePath() string {
	return filepath.Join(o.stateRoot, "current.json")
}

// versionDir 返回固定版本目录并拒绝路径注入。
func (o *PnpmOperator) versionDir(version string) string {
	return filepath.Join(o.stateRoot, "versions", version)
}

// validVersion 限制候选和 previous 名称只能来自本算子。
func validVersion(version string) bool {
	return versionNamePattern.MatchString(version)
}

// copyDirectory 精确复制构建产物，不跟随符号链接。
func copyDirectory(source, target string) error {
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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
