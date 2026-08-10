package formruntimemaintenance

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Syncer 只同步清单映射到 upstream 原样区，并保护本地适配层。
type Syncer struct {
	workspaceRoot string
	sourceRoot    string
	manifest      Manifest
}

// NewSyncer 创建受控同步器。
func NewSyncer(workspaceRoot, sourceRoot string, manifest Manifest) (*Syncer, error) {
	workspace, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return nil, err
	}
	source, err := filepath.Abs(filepath.Clean(sourceRoot))
	if err != nil {
		return nil, err
	}
	return &Syncer{workspaceRoot: workspace, sourceRoot: source, manifest: manifest}, nil
}

// Sync 在覆盖前用当前项目 Git 状态识别 upstream 未提交修改，避免静默抹掉本地现场。
func (s *Syncer) Sync(ctx context.Context, output io.Writer) error {
	status, err := gitRaw(ctx, s.workspaceRoot, "status", "--porcelain=v1", "--untracked-files=all", "--", "form-runtime/upstream")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("%w: 请先处理 form-runtime/upstream 的本地修改", ErrTargetModified)
	}
	for _, mapping := range s.manifest.Mappings {
		if err := ctx.Err(); err != nil {
			return err
		}
		source := filepath.Join(s.sourceRoot, filepath.Clean(mapping.Source))
		target := filepath.Join(s.workspaceRoot, "form-runtime", filepath.Clean(mapping.Target))
		if mapping.Type == "file" {
			if err := copyFile(source, target); err != nil {
				return err
			}
		} else if err := mirrorDirectory(source, target); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(output, "[SYNC] %s -> %s\n", mapping.Source, mapping.Target)
	}
	return nil
}

// Check 比较每个映射的路径集合与内容摘要，保证重复同步幂等且没有额外文件。
func (s *Syncer) Check(ctx context.Context, output io.Writer) error {
	for _, mapping := range s.manifest.Mappings {
		if err := ctx.Err(); err != nil {
			return err
		}
		source := filepath.Join(s.sourceRoot, filepath.Clean(mapping.Source))
		target := filepath.Join(s.workspaceRoot, "form-runtime", filepath.Clean(mapping.Target))
		left, err := pathDigest(source)
		if err != nil {
			return err
		}
		right, err := pathDigest(target)
		if err != nil {
			return err
		}
		if left != right {
			return fmt.Errorf("同步校验不一致: %s -> %s", mapping.Source, mapping.Target)
		}
		_, _ = fmt.Fprintf(output, "[SYNC_CHECK] %s 已与固定来源一致\n", mapping.Target)
	}
	return nil
}

// copyFile 原子替换单个同步文件，避免半写入状态。
func copyFile(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("读取同步来源 %s: %w", source, err)
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary := target + ".sync-candidate"
	if err := os.WriteFile(temporary, content, info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

// mirrorDirectory 在 upstream 目标内做精确镜像，调用方已通过 Git 状态确认没有未知本地修改。
func mirrorDirectory(source, target string) error {
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
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
		return copyFile(path, destination)
	})
}

// pathDigest 对目录路径、相对文件名和内容做稳定摘要，用于 sync-check/status。
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
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
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
