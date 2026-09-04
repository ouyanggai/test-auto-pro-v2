package control

import (
	"fmt"
	"strconv"
	"time"

	"test-auto-pro-v2/internal/model"
)

// ControlLog 把控制事实逐行写进运行目录的 control.log（纲领第 6.2、6.3 节），
// 行格式与关联键同 step.log；断点命中行额外带断点类型与挂载对象。
// 写入失败不影响控制事实落库（数据库仍是权威），日志只是可达性补充。
type ControlLog struct {
	write func(pathRunID uint64, fields []fmt.Stringer)
}

// controlLogField 是一行里的键值对（避免直接依赖 logging 包，装配层负责格式化）。
type controlLogField struct {
	Key   string
	Value string
}

// String 实现 fmt.Stringer。
func (f controlLogField) String() string { return f.Key + "=" + f.Value }

// NewControlLog 装配 control.log 写入器；write 由装配层提供（复用 F-013 运行目录路由）。
func NewControlLog(write func(pathRunID uint64, fields []fmt.Stringer)) *ControlLog {
	return &ControlLog{write: write}
}

// LogFact 把一条控制事实写为一行 control.log。
func (l *ControlLog) LogFact(pathRunID uint64, control model.RunControl, stepNo int) {
	if l == nil || l.write == nil {
		return
	}
	fields := []fmt.Stringer{
		controlLogField{Key: "kind", Value: string(control.Kind)},
		controlLogField{Key: "command", Value: string(control.Command)},
		controlLogField{Key: "breakpoint_type", Value: string(control.BreakpointType)},
		controlLogField{Key: "object_kind", Value: control.ObjectKind},
		controlLogField{Key: "object_key", Value: control.ObjectKey},
		controlLogField{Key: "step_id", Value: strconv.Itoa(stepNo)},
		controlLogField{Key: "reason", Value: control.Reason},
	}
	l.write(pathRunID, fields)
}

// 时间基准供装配层格式化 time= 字段。
var _ = time.Now
