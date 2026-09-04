// 断点模型与命中判定（纲领第 5.2 节）。全部为纯函数：
// 命中判定只依据「当前步骤事实 + 生效断点集合 + 偏离标志」，可穷举对照。
// 当前生效断点集合由 run_controls 事实回放得出（回放在本文件底部），不新建可变断点表。
package control

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/model"
)

// Breakpoint 是一个生效中的断点：类型 + 挂载对象。
// 挂载对象三维度全部来自既有数据（步骤序号、节点键、动作键），不新造标识体系。
type Breakpoint struct {
	Type    model.BreakpointType
	StepNo  int    // 步骤断点：编译步骤序号
	NodeKey string // 节点断点：语义节点键
	Action  string // 动作断点：动作键
}

// key 返回断点的唯一键（同类型同对象只算一个断点）。
func (b Breakpoint) key() string {
	switch b.Type {
	case model.BreakpointStep:
		return fmt.Sprintf("step:%d", b.StepNo)
	case model.BreakpointNode:
		return fmt.Sprintf("node:%s", b.NodeKey)
	case model.BreakpointAction:
		return fmt.Sprintf("action:%s", b.Action)
	default:
		return string(b.Type)
	}
}

// Label 返回断点的中文业务名称（界面与日志用，不暴露内部键以外的东西）。
func (b Breakpoint) Label() string {
	switch b.Type {
	case model.BreakpointFirstWrite:
		return "首次写断点"
	case model.BreakpointPathDeviation:
		return "路径偏离断点"
	case model.BreakpointStep:
		return fmt.Sprintf("步骤断点（第 %d 步）", b.StepNo)
	case model.BreakpointNode:
		return "节点断点（" + nodeNameLabel(b.NodeKey) + "）"
	case model.BreakpointAction:
		return "动作断点（" + b.Action + "）"
	default:
		return string(b.Type)
	}
}

// nodeNameLabel 节点断点的挂载名：节点键原样（业务名称由 DTO 层结合节点表翻译）。
func nodeNameLabel(nodeKey string) string {
	return strings.TrimSpace(nodeKey)
}

// BreakpointSet 是当前生效断点集合。
type BreakpointSet struct {
	breakpoints map[string]Breakpoint
}

// NewBreakpointSet 创建空集合，并按默认规则置入强制断点：
// 路径偏离断点强制开启不可关闭；首次写断点默认开启（可被事实回放的删除移除）。
func NewBreakpointSet() *BreakpointSet {
	set := &BreakpointSet{breakpoints: map[string]Breakpoint{}}
	set.breakpoints[(Breakpoint{Type: model.BreakpointFirstWrite}).key()] = Breakpoint{Type: model.BreakpointFirstWrite}
	set.breakpoints[(Breakpoint{Type: model.BreakpointPathDeviation}).key()] = Breakpoint{Type: model.BreakpointPathDeviation}
	return set
}

// Add 增加一个断点；路径偏离断点不可添加副本（强制开启已内置），其余类型合法即加入。
func (s *BreakpointSet) Add(b Breakpoint) {
	if b.Type == model.BreakpointPathDeviation {
		return
	}
	s.breakpoints[b.key()] = b
}

// Remove 删除一个断点；路径偏离断点拒绝删除（强制开启），返回是否发生了删除。
func (s *BreakpointSet) Remove(b Breakpoint) bool {
	if b.Type == model.BreakpointPathDeviation {
		return false
	}
	key := b.key()
	if _, ok := s.breakpoints[key]; !ok {
		return false
	}
	delete(s.breakpoints, key)
	return true
}

// Contains 判断某断点是否生效。
func (s *BreakpointSet) Contains(b Breakpoint) bool {
	_, ok := s.breakpoints[b.key()]
	return ok
}

// List 返回全部生效断点（稳定排序：类型优先级 + 挂载键）。
func (s *BreakpointSet) List() []Breakpoint {
	result := make([]Breakpoint, 0, len(s.breakpoints))
	for _, b := range s.breakpoints {
		result = append(result, b)
	}
	sort.Slice(result, func(i, j int) bool {
		pi, pj := breakpointPriority(result[i].Type), breakpointPriority(result[j].Type)
		if pi != pj {
			return pi < pj
		}
		return result[i].key() < result[j].key()
	})
	return result
}

// breakpointPriority 是多命中主因的固定顺序（纲领：路径偏离、首次写、动作、节点、步骤）。
func breakpointPriority(t model.BreakpointType) int {
	switch t {
	case model.BreakpointPathDeviation:
		return 0
	case model.BreakpointFirstWrite:
		return 1
	case model.BreakpointAction:
		return 2
	case model.BreakpointNode:
		return 3
	case model.BreakpointStep:
		return 4
	default:
		return 9
	}
}

// StepFacts 是阶段 3 命中判定所需的当前步骤事实。
type StepFacts struct {
	StepNo        int
	NodeKey       string
	Action        string
	IsWriteStep   bool // 由动作目录的 TargetOperation 是否落在写端点白名单判定，不硬编码动作名
	DeviationHit  bool // 上一步 verify 认定的路径偏离事实
}

// BreakpointHit 是一次断点命中。
type BreakpointHit struct {
	Breakpoint Breakpoint
	Reason     string
}

// EvaluateBreakpointHits 返回该步骤在阶段 3 命中的全部断点（无命中返回空）。
// 命中不在这里决定“停不停”——那由模式与命令集决定；这里只如实给出全部命中与主因。
func EvaluateBreakpointHits(facts StepFacts, set *BreakpointSet) []BreakpointHit {
	hits := []BreakpointHit{}
	if set == nil {
		return hits
	}
	if facts.DeviationHit && set.Contains(Breakpoint{Type: model.BreakpointPathDeviation}) {
		hits = append(hits, BreakpointHit{
			Breakpoint: Breakpoint{Type: model.BreakpointPathDeviation},
			Reason:     "上一步核验发现实际命中分支与已配置路径不一致",
		})
	}
	if facts.IsWriteStep && set.Contains(Breakpoint{Type: model.BreakpointFirstWrite}) {
		hits = append(hits, BreakpointHit{
			Breakpoint: Breakpoint{Type: model.BreakpointFirstWrite},
			Reason:     "这是本次运行的第一个写请求（安全阀）",
		})
	}
	actionBP := Breakpoint{Type: model.BreakpointAction, Action: facts.Action}
	if facts.Action != "" && set.Contains(actionBP) {
		hits = append(hits, BreakpointHit{Breakpoint: actionBP, Reason: "步骤动作命中动作断点"})
	}
	nodeBP := Breakpoint{Type: model.BreakpointNode, NodeKey: facts.NodeKey}
	if facts.NodeKey != "" && set.Contains(nodeBP) {
		hits = append(hits, BreakpointHit{Breakpoint: nodeBP, Reason: "步骤进入挂断点的节点"})
	}
	stepBP := Breakpoint{Type: model.BreakpointStep, StepNo: facts.StepNo}
	if facts.StepNo > 0 && set.Contains(stepBP) {
		hits = append(hits, BreakpointHit{Breakpoint: stepBP, Reason: fmt.Sprintf("命中第 %d 步的步骤断点", facts.StepNo)})
	}
	// 主因按固定优先级排序：路径偏离、首次写、动作、节点、步骤。
	sort.SliceStable(hits, func(i, j int) bool {
		return breakpointPriority(hits[i].Breakpoint.Type) < breakpointPriority(hits[j].Breakpoint.Type)
	})
	return hits
}

// PrimaryHit 返回主因（优先级最高的命中）；无命中返回 nil。
func PrimaryHit(hits []BreakpointHit) *BreakpointHit {
	if len(hits) == 0 {
		return nil
	}
	return &hits[0]
}

// ValidateBreakpointTarget 校验断点挂载对象：断点只能挂在尚未执行的对象上。
// executedStepNos 是已落账步骤序号集合；executedNodeKeys 是已落账节点键集合。
func ValidateBreakpointTarget(b Breakpoint, executedStepNos map[int]bool, executedNodeKeys map[string]bool) error {
	switch b.Type {
	case model.BreakpointFirstWrite, model.BreakpointPathDeviation:
		return nil
	case model.BreakpointStep:
		if b.StepNo <= 0 {
			return fmt.Errorf("步骤断点必须指明大于 0 的步骤序号")
		}
		if executedStepNos[b.StepNo] {
			return fmt.Errorf("第 %d 步已经执行完成，断点只能挂在尚未执行的步骤上", b.StepNo)
		}
		return nil
	case model.BreakpointNode:
		if strings.TrimSpace(b.NodeKey) == "" {
			return fmt.Errorf("节点断点必须指明节点")
		}
		if executedNodeKeys[b.NodeKey] {
			return fmt.Errorf("该节点上已有已执行的步骤，断点只能挂在尚未执行的节点上")
		}
		return nil
	case model.BreakpointAction:
		if strings.TrimSpace(b.Action) == "" {
			return fmt.Errorf("动作断点必须指明动作")
		}
		return nil
	default:
		return fmt.Errorf("未知的断点类型：%s", string(b.Type))
	}
}

// ReplayBreakpoints 从控制事实回放当前生效断点集合。
// 集合以默认断点（首次写+路径偏离）为基底，breakpoint_set 逐条加入、breakpoint_remove 逐条移除；
// 路径偏离断点的删除事实一律无效（强制开启）。
func ReplayBreakpoints(controls []model.RunControl) *BreakpointSet {
	set := NewBreakpointSet()
	for _, control := range controls {
		bp := BreakpointFromControl(control)
		switch control.Kind {
		case model.ControlFactBreakpointSet:
			set.Add(bp)
		case model.ControlFactBreakpointRemove:
			set.Remove(bp)
		}
	}
	return set
}

// BreakpointFromControl 从控制事实行还原断点。
func BreakpointFromControl(control model.RunControl) Breakpoint {
	bp := Breakpoint{Type: control.BreakpointType}
	switch control.ObjectKind {
	case "step":
		no, _ := strconv.Atoi(control.ObjectKey)
		bp = Breakpoint{Type: model.BreakpointStep, StepNo: no}
	case "node":
		bp = Breakpoint{Type: model.BreakpointNode, NodeKey: control.ObjectKey}
	case "action":
		bp = Breakpoint{Type: model.BreakpointAction, Action: control.ObjectKey}
	default:
		bp = Breakpoint{Type: control.BreakpointType}
	}
	return bp
}

// BreakpointToObjectKey 把断点拆为落库所需的挂载对象种类与键。
func BreakpointToObject(b Breakpoint) (objectKind, objectKey string) {
	switch b.Type {
	case model.BreakpointStep:
		return "step", strconv.Itoa(b.StepNo)
	case model.BreakpointNode:
		return "node", b.NodeKey
	case model.BreakpointAction:
		return "action", b.Action
	default:
		return "run", ""
	}
}
