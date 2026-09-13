// F-034 T03：运行展示的集中动作中文映射。
// 已知动作键后端始终生成中文名；这里只为旧 DTO（历史运行、导航或目录缺失）做同一张白名单兜底。
// 未知键显示安全的中文占位并保留日志入口，绝不在界面渲染原始英文稳定键。
const actionLabels: Record<string, string> = {
  save_draft: '保存草稿',
  submit: '提交',
  resubmit: '重新提交',
  storage_form_data: '暂存当前表单',
  add_sign: '加签',
  transfer: '移交',
  approve: '同意',
  reject: '不同意',
  rollback_previous: '回退上一节点',
  retrieve: '取回',
  withdraw: '撤回',
  urge: '催办',
  forward: '转发',
  follow: '关注',
  unfollow: '取消关注',
  system_automatic: '系统自动动作',
}

// actionLabel 返回动作的用户可见名称：已知键映射中文，未知键返回安全占位。
// 调用方必须始终使用本函数，不允许模板出现 `actionName || action` 作为最终用户文本。
export function actionLabel(actionName: string | undefined, action: string | undefined): string {
  const known = action ? actionLabels[action] : undefined
  if (known) return known
  const provided = (actionName ?? '').trim()
  // 后端给出的名称是可信中文投影时直接使用；疑似原始稳定键（小写字母+下划线）不展示。
  if (provided && !/^[a-z0-9_]+$/.test(provided)) return provided
  return '未识别动作（请查看日志）'
}
