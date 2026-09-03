package logging

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupExpired 按保留期清理配置桶与运行目录，只删过期目录，绝不触碰数据库运行事实。
// 当天目录一律保留；配置桶按目录名的日期判断，运行目录按目录最后修改时间判断。
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
	removed := make([]string, 0, 4)
	removed = append(removed, cleanupDailyFiles(root, cutoff, today)...)
	removed = append(removed, cleanupConfigBuckets(filepath.Join(root, "config"), cutoff, today)...)
	removed = append(removed, cleanupArchive(filepath.Join(root, "archive"), cutoff, today)...)
	removed = append(removed, cleanupRunBuckets(filepath.Join(root, "runs"), cutoff)...)
	return removed
}

// cleanupDailyFiles 删除按天分文件的全局日志（app-<日期>.log、app-error-<日期>.log）里已过期的文件，
// 连带它们的轮转副本；当天文件一律保留，文件名解析不出日期的一律不动。
func cleanupDailyFiles(root string, cutoff time.Time, today string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	removed := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "app") || strings.Contains(name, today) {
			continue
		}
		day, ok := dailyFileDate(name)
		if !ok || !day.Before(cutoff) {
			continue
		}
		path := filepath.Join(root, name)
		if err := os.Remove(path); err == nil {
			removed = append(removed, path)
		}
	}
	return removed
}

// dailyFileDate 从按天文件名里取出日期，形如 app-2026-09-04.log 或其轮转副本 app-2026-09-04.log.1。
func dailyFileDate(name string) (time.Time, bool) {
	fields := strings.Split(name, "-")
	if len(fields) < 4 {
		return time.Time{}, false
	}
	dayPart := strings.Join(fields[len(fields)-3:], "-")
	if index := strings.Index(dayPart, ".log"); index >= 0 {
		dayPart = dayPart[:index]
	}
	day, err := time.Parse("2006-01-02", dayPart)
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

// cleanupConfigBuckets 删除 logs/config/<日期> 里已经过期的日期目录。
func cleanupConfigBuckets(dir string, cutoff time.Time, today string) []string {
	return removeExpiredDateDirs(dir, cutoff, today)
}

// cleanupArchive 删除 logs/archive/<日期> 里已经过期的按日归档目录。
func cleanupArchive(dir string, cutoff time.Time, today string) []string {
	return removeExpiredDateDirs(dir, cutoff, today)
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
		if parseErr != nil {
			continue
		}
		if !day.Before(cutoff) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err == nil {
			removed = append(removed, path)
		}
	}
	return removed
}

// cleanupRunBuckets 删除 logs/runs/<计划>/<路径>/<运行号> 里最后修改时间已过期的运行目录，
// 并清掉因此变空的计划与路径目录；运行目录名不含日期，只能按修改时间判断。
func cleanupRunBuckets(root string, cutoff time.Time) []string {
	plans, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	removed := make([]string, 0, len(plans))
	for _, plan := range plans {
		if !plan.IsDir() {
			continue
		}
		planDir := filepath.Join(root, plan.Name())
		paths, pathErr := os.ReadDir(planDir)
		if pathErr != nil {
			continue
		}
		for _, path := range paths {
			if !path.IsDir() {
				continue
			}
			pathDir := filepath.Join(planDir, path.Name())
			runs, runErr := os.ReadDir(pathDir)
			if runErr != nil {
				continue
			}
			for _, run := range runs {
				if !run.IsDir() {
					continue
				}
				info, infoErr := run.Info()
				if infoErr != nil || !info.ModTime().Before(cutoff) {
					continue
				}
				runDir := filepath.Join(pathDir, run.Name())
				if err := os.RemoveAll(runDir); err == nil {
					removed = append(removed, runDir)
				}
			}
			removeIfEmpty(pathDir)
		}
		removeIfEmpty(planDir)
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
