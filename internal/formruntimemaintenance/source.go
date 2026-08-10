package formruntimemaintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Mapping 描述固定来源到 upstream 原样区的唯一允许复制边界。
type Mapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// Manifest 固定来源仓库、远端、分支、同步映射与本地保护区；HEAD 由每次任务动态记录。
type Manifest struct {
	Repository           string    `json:"repository"`
	SourceRoot           string    `json:"sourceRoot"`
	SourceRemote         string    `json:"sourceRemote"`
	SourceBranch         string    `json:"sourceBranch"`
	Mappings             []Mapping `json:"mappings"`
	PreservedTargetPaths []string  `json:"preservedTargetPaths"`
	GeneratedTargetPaths []string  `json:"generatedTargetPaths"`
	ProtectedLocalPaths  []string  `json:"protectedLocalPaths"`
	ExcludedSourcePaths  []string  `json:"excludedSourcePaths"`
}

// LoadManifest 读取并严格校验项目内固定同步清单，拒绝路径逃逸和本地适配覆盖。
func LoadManifest(workspaceRoot, manifestPath string) (Manifest, error) {
	content, err := os.ReadFile(filepath.Clean(manifestPath))
	if err != nil {
		return Manifest{}, fmt.Errorf("读取表单运行时同步清单: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("解析表单运行时同步清单: %w", err)
	}
	if strings.TrimSpace(manifest.Repository) == "" || strings.TrimSpace(manifest.SourceRemote) == "" ||
		strings.TrimSpace(manifest.SourceBranch) == "" || !safeRelativePath(manifest.SourceRoot) || len(manifest.Mappings) == 0 {
		return Manifest{}, fmt.Errorf("%w: 清单缺少固定来源", ErrSourceInvalid)
	}
	boundaryPaths := append(append(append([]string{}, manifest.PreservedTargetPaths...), manifest.GeneratedTargetPaths...), manifest.ProtectedLocalPaths...)
	boundaryPaths = append(boundaryPaths, manifest.ExcludedSourcePaths...)
	for _, path := range boundaryPaths {
		if !safeRelativePath(path) {
			return Manifest{}, fmt.Errorf("%w: 清单包含非法边界路径 %s", ErrSourceInvalid, path)
		}
	}
	for _, mapping := range manifest.Mappings {
		if !safeRelativePath(mapping.Source) || !safeRelativePath(mapping.Target) ||
			(mapping.Type != "file" && mapping.Type != "directory") || !strings.HasPrefix(filepath.ToSlash(mapping.Target), "runtime-source/") {
			return Manifest{}, fmt.Errorf("%w: 非法同步映射 %s -> %s", ErrSourceInvalid, mapping.Source, mapping.Target)
		}
		for _, protected := range manifest.ProtectedLocalPaths {
			if pathOverlaps(mapping.Target, protected) {
				return Manifest{}, fmt.Errorf("%w: 映射会覆盖本地适配 %s", ErrSourceInvalid, protected)
			}
		}
	}
	if !filepath.IsAbs(workspaceRoot) {
		return Manifest{}, fmt.Errorf("%w: 工作区必须是绝对路径", ErrSourceInvalid)
	}
	return manifest, nil
}

// safeRelativePath 拒绝绝对路径、空路径和任何父级逃逸。
func safeRelativePath(value string) bool {
	cleaned := filepath.Clean(strings.TrimSpace(value))
	return cleaned != "." && !filepath.IsAbs(cleaned) && cleaned != ".." && !strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

// pathOverlaps 判断两个相对目录是否会发生父子覆盖。
func pathOverlaps(left, right string) bool {
	left = filepath.ToSlash(filepath.Clean(left))
	right = filepath.ToSlash(filepath.Clean(right))
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

// GitSourceInspector 只核对清单固定的参考仓库，不接受 API 传入任意路径或分支。
type GitSourceInspector struct {
	workspaceRoot string
	manifest      Manifest
	now           func() time.Time
}

// NewGitSourceInspector 创建固定来源检查器。
func NewGitSourceInspector(workspaceRoot string, manifest Manifest, now func() time.Time) (*GitSourceInspector, error) {
	root, err := filepath.Abs(filepath.Clean(workspaceRoot))
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &GitSourceInspector{workspaceRoot: root, manifest: manifest, now: now}, nil
}

// SourceRoot 返回清单内固定来源的规范绝对路径，仅供同步算子使用。
func (i *GitSourceInspector) SourceRoot() string {
	return filepath.Join(i.workspaceRoot, filepath.Clean(i.manifest.SourceRoot))
}

// Inspect 校验仓库、远端、固定分支和工作树，并把检查当刻 HEAD 返回给任务持久化。
func (i *GitSourceInspector) Inspect(ctx context.Context) (SourceState, error) {
	root := i.SourceRoot()
	remote, err := gitOutput(ctx, root, "remote", "get-url", "origin")
	if err != nil || remote != i.manifest.SourceRemote {
		return SourceState{}, fmt.Errorf("%w: 来源远端不符", ErrSourceInvalid)
	}
	branch, err := gitOutput(ctx, root, "branch", "--show-current")
	if err != nil || branch != i.manifest.SourceBranch {
		return SourceState{}, fmt.Errorf("%w: 来源分支必须是 %s", ErrSourceInvalid, i.manifest.SourceBranch)
	}
	head, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil || len(head) != 40 {
		return SourceState{}, fmt.Errorf("%w: 无法读取来源 HEAD", ErrSourceInvalid)
	}
	status, err := gitRaw(ctx, root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return SourceState{}, fmt.Errorf("读取表单运行时来源状态: %w", err)
	}
	changed := parseChangedFiles(status)
	return SourceState{
		Repository:   i.manifest.Repository,
		Branch:       i.manifest.SourceBranch,
		Head:         head,
		Dirty:        len(changed) > 0,
		ChangedFiles: changed,
		InspectedAt:  i.now().UTC(),
	}, nil
}

// gitOutput 执行固定 Git 只读命令并去除行尾空白。
func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	output, err := gitRaw(ctx, root, args...)
	return strings.TrimSpace(output), err
}

// gitRaw 执行固定 Git 只读命令，不经过 shell 拼接。
func gitRaw(ctx context.Context, root string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

// parseChangedFiles 只公开排序后的路径和 Git 状态，不读取或记录文件内容。
func parseChangedFiles(status string) []ChangedFile {
	items := make([]ChangedFile, 0)
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if _, renamed, ok := strings.Cut(path, " -> "); ok {
			path = renamed
		}
		items = append(items, ChangedFile{Path: filepath.ToSlash(strings.Trim(path, `"`)), Status: strings.TrimSpace(line[:2])})
	}
	sort.Slice(items, func(left, right int) bool { return items[left].Path < items[right].Path })
	return items
}
