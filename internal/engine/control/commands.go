// 运行模式与可用命令集合（纲领第 5.1、5.3 节）。
// 三种模式的差别就是可用命令集合：单步运行只有执行一步；人工控制与自动运行（暂停时）有三条。
// 可用命令集合由服务端按模式与当前状态计算，前端只渲染、不自行推断。
package control

import "test-auto-pro-v2/internal/model"

// PauseState 是路径运行在阶段 3 停下时的控制状态分类。
type PauseState string

const (
	// PauseStateWaiting 表示停在阶段 3 等待放行（单步停、断点命中、暂停生效都归这里）。
	PauseStateWaiting PauseState = "waiting"
	// PauseStateDeviation 表示路径偏离断点强制停止：不产出放行类命令。
	PauseStateDeviation PauseState = "deviation"
	// PauseStateUncertain 表示写结果不确定：不产出任何重试或继续入口（F-016 已定，不得回退）。
	PauseStateUncertain PauseState = "uncertain"
	// PauseStateFinished 表示路径运行已到终态：无控制命令。
	PauseStateFinished PauseState = "finished"
)

// AvailableCommands 按模式与控制状态计算可用命令集合。
// 规则（设计要点「三种模式的差别就是可用命令集合」）：
//   - 单步运行等待放行：只有执行一步——另两条命令与“每一步执行前都停”直接冲突；
//   - 自动/人工控制等待放行：三条命令（此时“继续运行直到命中断点”才成立）；
//   - 路径偏离停止：无放行类命令（后续步骤在偏离后的结构上不成立，放行必然门禁不通过）；
//   - 写结果不确定或已终态：无任何命令。
func AvailableCommands(mode model.RunMode, state PauseState) []model.ControlCommand {
	switch state {
	case PauseStateDeviation, PauseStateUncertain, PauseStateFinished:
		return []model.ControlCommand{}
	case PauseStateWaiting:
		switch mode {
		case model.RunModeSingleStep:
			return []model.ControlCommand{model.CommandStep}
		case model.RunModeAuto, model.RunModeManual:
			return []model.ControlCommand{model.CommandStep, model.CommandNextNode, model.CommandContinue}
		}
	}
	return []model.ControlCommand{}
}

// CommandLabel 返回命令的中文显示名与停止条件说明（「继续运行」按钮必须说明跑到什么条件为止）。
func CommandLabel(command model.ControlCommand) string {
	switch command {
	case model.CommandStep:
		return "执行一步"
	case model.CommandNextNode:
		return "执行到下一节点（到语义节点变化处停下；剩余步骤都在同一节点时执行到场景走完）"
	case model.CommandContinue:
		return "继续运行（直到命中断点、需要人工或本路径结束）"
	default:
		return string(command)
	}
}

// CommandDescription 是控制命令的完整描述（进 DTO）。
type CommandDescription struct {
	Command model.ControlCommand `json:"command"`
	Label   string               `json:"label"`
}

// DescribeCommands 把命令键集合转为带中文说明的 DTO 列表。
func DescribeCommands(commands []model.ControlCommand) []CommandDescription {
	result := make([]CommandDescription, 0, len(commands))
	for _, command := range commands {
		result = append(result, CommandDescription{Command: command, Label: CommandLabel(command)})
	}
	return result
}

// NextNodeBoundary 判断「执行到下一节点」是否已到达边界：
// 从发出命令时的语义节点 fromNode 出发，连续执行到 plan 取到的步骤语义节点不同时在阶段 3 暂停。
// 剩余步骤都在同一节点时返回 false（调用方执行到场景走完后停止）。
func NextNodeBoundary(fromNode, nextStepNode string) bool {
	return nextStepNode != fromNode
}
