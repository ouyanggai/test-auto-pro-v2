// F-042 本轮：报告中心「目标接口耗时」的快速聚合读取器。
//
// 与 L3 运行详情的职责划分（本轮口径修正，必须一起理解）：
//   - 运行详情回答「这一步这次尝试发出了哪些请求」——需要把请求归属到步骤尝试，因此读 step.log 时间轴；
//   - 报告中心回答「这次路径运行一共打了目标多少次、各耗时多少」——只需要实例级总量，
//     因此只扫该路径运行目录的 network.log，累加完事，不归属、不建 DTO、不排序、不填充摘要。
//
// 旧实现借用了运行详情的逐尝试归属（interfaceStatsReader → RunOrchestrationService.PathRunInterfaceStats）：
// 每条路径运行各发一次尝试查询（N+1），每条路径运行还要全扫 step.log 推导时间窗口，
// 并对每个日志行建 map、构造 DTO、排序、填摘要。工作区实测一次首页读取要串行扫
// network.log 83 个文件 66.9 MB + step.log 131.4 MB，仅解析 network.log 就 1.13 s（纯 IO 31 ms）。
// 同时 step_id/attempt 对不上尝试的请求行（登录、模板读取、候选扫描、待办轮询，实测约占 12%）
// 被整行丢弃，报告的接口调用次数一直小于真实调用量。本文件按实例全量计修正该漏计。
package service

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// reportInterfaceScanWorkers 是日志扫描的固定并发度：报告读取是 IO 密集，
// 固定 worker 数即可把几十 MB 的串行扫描压到百毫秒内，又不会随运行目录数量放大句柄或连接压力。
const reportInterfaceScanWorkers = 8

// ReportInterfaceStats 是一次路径运行的目标接口传输事实汇总（报告中心口径：按实例全量计）。
type ReportInterfaceStats struct {
	DurationsMs      []float64 // 全部有真实耗时的接口请求（毫秒）
	WriteDurationsMs []float64 // 其中写请求的耗时（毫秒）
	ReadCount        int       // 读请求次数
	WriteCount       int       // 写请求次数
	AnyKnown         bool      // 是否读到任何真实耗时（无则前端显示「暂无真实请求耗时」）
}

// reportInterfaceCacheEntry 是终态路径运行的聚合缓存：按 (文件大小, 修改时间) 签名命中。
// 只缓存终态路径运行——它的 network.log 不会再增长；运行中的路径运行每次重算，避免报告显示旧数。
type reportInterfaceCacheEntry struct {
	size    int64
	modTime time.Time
	stats   ReportInterfaceStats
}

// ReportInterfaceScanner 是报告中心的接口传输事实聚合器：一次查询给一批日志定位，
// 用固定并发扫描，并把终态结果按文件签名缓存，重复进入 L0/L1/L2 不重复读盘。
type ReportInterfaceScanner struct {
	root  string // 日志根（logging.Router.Root()），与运行详情寻址口径一致
	mu    sync.Mutex
	cache map[uint64]reportInterfaceCacheEntry
	opens int // 累计打开 network.log 次数，仅供测试断言缓存是否生效
}

// NewReportInterfaceScanner 构造接口耗时聚合器；root 为空时退化为「无接口事实」，前端按缺失降级。
func NewReportInterfaceScanner(root string) *ReportInterfaceScanner {
	return &ReportInterfaceScanner{root: root, cache: map[uint64]reportInterfaceCacheEntry{}}
}

// OpenCount 返回累计打开 network.log 的次数（测试钩子：验证终态缓存命中与每目录只开一次）。
func (s *ReportInterfaceScanner) OpenCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opens
}

// ReportInterfaceSampleCapForTest 暴露单次聚合的路径运行采样上限，供测试构造越界样本
// （沿用仓库既有 `...ForTest` 钩子约定，不在生产路径上使用）。
func ReportInterfaceSampleCapForTest() int {
	return interfaceLogSampleCap
}

// Aggregate 对一批路径运行日志定位做有界并发聚合，返回按路径运行 ID 的结果与是否触达采样上限。
// dirs 必须已按最新在前排序：超过采样上限时只统计最近的样本并标记已截断（沿用首轮口径）。
// 单个目录读失败只跳过该条，绝不让一个坏目录使整页报告失败。
func (s *ReportInterfaceScanner) Aggregate(ctx context.Context, dirs []repository.ReportPathRunLogDir) (map[uint64]ReportInterfaceStats, bool) {
	out := make(map[uint64]ReportInterfaceStats, len(dirs))
	if s == nil || len(dirs) == 0 {
		return out, false
	}
	selected := dirs
	capped := false
	if len(selected) > interfaceLogSampleCap {
		selected = selected[:interfaceLogSampleCap]
		capped = true
	}
	// 任务通道一次备齐：不需要 feeder 协程，上下文取消时 worker 直接返回也不会阻塞发送方。
	jobs := make(chan repository.ReportPathRunLogDir, len(selected))
	for _, dir := range selected {
		jobs <- dir
	}
	close(jobs)
	type scanResult struct {
		pathRunID uint64
		stats     ReportInterfaceStats
	}
	results := make(chan scanResult, len(selected))
	workers := reportInterfaceScanWorkers
	if workers > len(selected) {
		workers = len(selected)
	}
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dir := range jobs {
				// 报告是只读统计：请求已取消就不再读盘，尽快收手。
				if ctx.Err() != nil {
					return
				}
				results <- scanResult{pathRunID: dir.PathRunID, stats: s.scanDir(dir)}
			}
		}()
	}
	wg.Wait()
	close(results)
	for result := range results {
		out[result.pathRunID] = result.stats
	}
	return out, capped
}

// scanDir 聚合一条路径运行：先按 (大小, 修改时间) 查缓存，未命中才打开 network.log。
// 只有终态路径运行写缓存——运行中的日志还在增长，缓存会让报告显示旧数。
func (s *ReportInterfaceScanner) scanDir(dir repository.ReportPathRunLogDir) ReportInterfaceStats {
	if strings.TrimSpace(dir.LogPath) == "" {
		return ReportInterfaceStats{}
	}
	// 与运行详情同一寻址口径：日志根 + 尝试行记录的 step.log 所在目录 + network.log。
	path := filepath.Join(s.root, filepath.Dir(filepath.FromSlash(dir.LogPath)), "network.log")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		// 历史日志已清理或从未落盘：没有事实就不统计，前端按「暂无真实请求耗时」降级。
		return ReportInterfaceStats{}
	}
	if cached, ok := s.cachedStats(dir.PathRunID, info); ok {
		return cached
	}
	s.mu.Lock()
	s.opens++
	s.mu.Unlock()
	stats, found := scanNetworkLogFile(path, strconv.FormatUint(dir.PathRunID, 10))
	if !found {
		return ReportInterfaceStats{}
	}
	if model.IsTerminalPathRunStatus(dir.Status) {
		s.mu.Lock()
		s.cache[dir.PathRunID] = reportInterfaceCacheEntry{size: info.Size(), modTime: info.ModTime(), stats: stats}
		s.mu.Unlock()
	}
	return stats
}

// cachedStats 命中终态缓存的条件：文件大小与修改时间都与读取时一致（日志未被追加或轮转）。
func (s *ReportInterfaceScanner) cachedStats(pathRunID uint64, info os.FileInfo) (ReportInterfaceStats, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[pathRunID]
	if !ok || entry.size != info.Size() || !entry.modTime.Equal(info.ModTime()) {
		return ReportInterfaceStats{}, false
	}
	return entry.stats, true
}

// scanNetworkLogFile 扫描一个路径运行目录的 network.log，只累加报告需要的样本与读写次数。
// 归属规则只有一条：行的 path_run_id 必须等于本次路径运行（目录内所有行都带这个字段，
// 保留判断是为了在异常混写时不把别的路径运行算进来）。
func scanNetworkLogFile(path string, pathRunID string) (ReportInterfaceStats, bool) {
	file, err := os.Open(path)
	if err != nil {
		return ReportInterfaceStats{}, false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	// network.log 单行最长可能到 1 MB（内网原样记录，含完整正文），缓冲上限与运行详情一致。
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	stats := ReportInterfaceStats{}
	for scanner.Scan() {
		line := scanner.Bytes()
		var (
			belongs     bool
			classKnown  bool
			isWrite     bool
			durationMs  float64
			hasDuration bool
			endpoint    []byte
		)
		// 逐字段扫描：只解析本函数用到的四个键，其余键连字符串都不构造，避免逐行分配。
		rest := line
		for len(rest) > 0 {
			token := rest
			if space := bytes.IndexByte(rest, ' '); space >= 0 {
				token, rest = rest[:space], rest[space+1:]
			} else {
				rest = nil
			}
			separator := bytes.IndexByte(token, '=')
			if separator <= 0 {
				continue
			}
			key, value := token[:separator], token[separator+1:]
			// switch string(bytes) 由编译器优化，不会为键名分配字符串。
			switch string(key) {
			case "path_run_id":
				belongs = string(value) == pathRunID
			case "request_class":
				classKnown = true
				isWrite = string(value) == "write"
			case "endpoint":
				endpoint = value
			case "duration_s":
				if ms, ok := parseLoggedDurationMs(value); ok {
					durationMs, hasDuration = ms, true
				}
			}
		}
		if !belongs {
			continue
		}
		if !classKnown {
			// 旧日志缺 request_class：按写端点白名单兜底，防写请求被误计为读取（与运行详情同口径）。
			isWrite = isWriteEndpoint(string(endpoint))
		}
		if isWrite {
			stats.WriteCount++
		} else {
			stats.ReadCount++
		}
		if !hasDuration {
			continue
		}
		stats.AnyKnown = true
		stats.DurationsMs = append(stats.DurationsMs, durationMs)
		if isWrite {
			stats.WriteDurationsMs = append(stats.WriteDurationsMs, durationMs)
		}
	}
	if err := scanner.Err(); err != nil {
		// 读到一半坏行/超长行：已累加的部分仍是真实事实，按已读到的事实返回，
		// 不伪造完整统计，也不让整页失败（首轮同纪律：缺事实就降级，不造数）。
		return stats, true
	}
	return stats, true
}

// parseLoggedDurationMs 把日志里的 duration_s（秒，三位小数）解析为毫秒。
// 空值与占位符「-」表示没有真实耗时，返回 false 让调用方按未知处理，绝不当 0ms。
func parseLoggedDurationMs(value []byte) (float64, bool) {
	if len(value) == 0 || string(value) == "-" {
		return 0, false
	}
	seconds, err := strconv.ParseFloat(string(value), 64)
	if err != nil {
		return 0, false
	}
	return seconds * 1000, true
}
