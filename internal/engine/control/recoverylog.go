// recovery.log 写入器（纲领第 6.2 节）：对账输入、逐维度判断、结论与最终决定逐行可查。
// 行格式与关联键同 step.log / control.log；写失败不影响对账事实落库。
package control

import "fmt"

// RecoveryLog 把对账过程写进运行目录的 recovery.log；write 由装配层提供（复用 F-013 路由）。
type RecoveryLog struct {
	write func(pathRunID uint64, message string)
}

// NewRecoveryLog 装配 recovery.log 写入器。
func NewRecoveryLog(write func(pathRunID uint64, message string)) *RecoveryLog {
	return &RecoveryLog{write: write}
}

// LogFact 写一行对账过程记录。
func (l *RecoveryLog) LogFact(pathRunID uint64, message string) {
	if l == nil || l.write == nil {
		return
	}
	l.write(pathRunID, fmt.Sprintf("%s", message))
}
