package formruntimemaintenance

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// Syncer 调用 rsh-flow-components 原生同步脚本的本项目适配入口。
type Syncer struct {
	workspaceRoot string
	manifest      Manifest
}

// NewSyncer 创建固定脚本同步器，并确认来源路径就是清单声明的参考仓库。
func NewSyncer(workspaceRoot, sourceRoot string, manifest Manifest) (*Syncer, error) {
	workspace, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return nil, err
	}
	source, err := filepath.Abs(filepath.Clean(sourceRoot))
	if err != nil {
		return nil, err
	}
	expectedSource := filepath.Join(workspace, filepath.Clean(manifest.SourceRoot))
	if source != expectedSource {
		return nil, fmt.Errorf("%w: 同步来源不是清单固定仓库", ErrSourceInvalid)
	}
	return &Syncer{workspaceRoot: workspace, manifest: manifest}, nil
}

// Sync 以任务创建时 HEAD 执行原生 sync.js，目标只能是该任务的隔离候选源码。
func (s *Syncer) Sync(ctx context.Context, source SourceState, targetRoot string, output io.Writer) error {
	return s.run(ctx, "sync.js", source, targetRoot, output)
}

// Check 以同一任务快照执行原生 sync-check.js，保证候选源码与实际构建入口一致。
func (s *Syncer) Check(ctx context.Context, source SourceState, targetRoot string, output io.Writer) error {
	return s.run(ctx, "sync-check.js", source, targetRoot, output)
}

// run 只执行项目内固定 Node 脚本，不接受 API 传入脚本名、仓库或命令参数。
func (s *Syncer) run(ctx context.Context, script string, source SourceState, targetRoot string, output io.Writer) error {
	if source.Repository != s.manifest.Repository || source.Branch != s.manifest.SourceBranch || len(source.Head) != 40 || source.Dirty {
		return fmt.Errorf("%w: 任务来源快照不合法", ErrSourceInvalid)
	}
	target, err := filepath.Abs(filepath.Clean(targetRoot))
	if err != nil {
		return err
	}
	allowedRoot := filepath.Join(s.workspaceRoot, ".runtime", "form-runtime-maintenance", "workspaces")
	if !within(allowedRoot, target) || filepath.Base(target) != "runtime-source" {
		return fmt.Errorf("%w: 候选同步目标不在任务工作区", ErrSourceInvalid)
	}
	command := exec.CommandContext(ctx, "node", filepath.Join(s.workspaceRoot, "form-runtime", "scripts", script))
	command.Dir = s.workspaceRoot
	command.Env = append(os.Environ(),
		"FORM_RUNTIME_EXPECTED_HEAD="+source.Head,
		"FORM_RUNTIME_TARGET_ROOT="+target,
	)
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		return fmt.Errorf("执行表单运行时 %s: %w", script, err)
	}
	return nil
}

// pathDigest 对目录路径、相对文件名和内容做稳定摘要，用于 bootstrap 版本命名。
func pathDigest(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if !info.IsDir() {
		content, err := os.ReadFile(root)
		if err != nil {
			return "", err
		}
		_, _ = hash.Write(content)
		return fmt.Sprintf("%x", hash.Sum(nil)), nil
	}
	paths := make([]string, 0)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("表单运行时源码与产物不得包含符号链接")
		}
		if !entry.IsDir() {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			// 目标摘要写在该文件里，必须排除自身才能稳定复算并识别其他未知修改。
			if relative != ".f007-source.json" {
				paths = append(paths, relative)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, relative := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return "", err
		}
		_, _ = hash.Write([]byte(relative + "\x00"))
		_, _ = hash.Write(content)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
