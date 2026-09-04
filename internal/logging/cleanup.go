package logging

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupExpired 按保留期清理应用程序日志与计划业务日志，只删过期目录，绝不触碰数据库运行事实。
// 当天目录一律保留；按日期命名的目录按目录名判断，运行目录名是运行号，只能按最后修改时间判断。
// 返回实际删除的目录，便于测试和启动日志核对。
func CleanupExpired(root string, retentionDays int, now time.Time) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	if retentionDays < 1 {
		retentionDays = DefaultRetentionDays
	}
	cutoff := now.AddDate(0, 0, -retentionDays)
	today := now.Format("2006-01-02")
	removed := removeExpiredDateDirs(filepath.Join(root, applicationDirName), cutoff, today)
	return append(removed, cleanupPlanTrees(filepath.Join(root, plansDirName), cutoff, today)...)
}

// cleanupPlanTrees 遍历每个计划目录，分别清理配置阶段的日期目录与执行阶段的运行目录，
// 并把因此空掉的执行路径、配置、运行与计划目录一并收掉，避免留下一堆空壳目录。
func cleanupPlanTrees(plansRoot string, cutoff time.Time, today string) []string {
	plans, err := os.ReadDir(plansRoot)
	if err != nil {
		return nil
	}
	removed := make([]string, 0, len(plans))
	for _, plan := range plans {
		if !plan.IsDir() {
			continue
		}
		planDir := filepath.Join(plansRoot, plan.Name())
		configurationDir := filepath.Join(planDir, configurationDirName)
		for _, pathDir := range childDirs(configurationDir) {
			removed = append(removed, removeExpiredDateDirs(pathDir, cutoff, today)...)
			removeIfEmpty(pathDir)
		}
		removeIfEmpty(configurationDir)
		runsDir := filepath.Join(planDir, runsDirName)
		for _, pathDir := range childDirs(runsDir) {
			removed = append(removed, removeExpiredRunDirs(pathDir, cutoff)...)
			removeIfEmpty(pathDir)
		}
		removeIfEmpty(runsDir)
		removeIfEmpty(planDir)
	}
	return removed
}

// childDirs 返回某个目录下的全部子目录绝对路径，读不到时返回空切片。
func childDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	children := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			children = append(children, filepath.Join(dir, entry.Name()))
		}
	}
	return children
}

// removeExpiredDateDirs 按目录名解析日期并删除早于保留期的目录；无法解析的目录一律保留。
func removeExpiredDateDirs(dir string, cutoff time.Time, today string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	removed := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == today {
			continue
		}
		day, parseErr := time.ParseInLocation("2006-01-02", entry.Name(), cutoff.Location())
		if parseErr != nil || !day.Before(cutoff) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err == nil {
			removed = append(removed, path)
		}
	}
	return removed
}

// removeExpiredRunDirs 删除最后修改时间已过期的运行目录；运行目录名是运行号，不含日期。
func removeExpiredRunDirs(dir string, cutoff time.Time) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	removed := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err == nil {
			removed = append(removed, path)
		}
	}
	return removed
}

// removeIfEmpty 删除已经空掉的目录，忽略非空与不存在。
func removeIfEmpty(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(dir)
}
