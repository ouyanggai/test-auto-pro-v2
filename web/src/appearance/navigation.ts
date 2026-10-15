// 两种导航呈现共用同一份地址和名称，不为皮肤复制路由或增加另一套业务入口。
export const navigationItems = [
  { path: '/plans', label: '测试计划', symbol: 'plans' },
  { path: '/runs', label: '任务中心', symbol: 'runs' },
  { path: '/reports', label: '报告中心', symbol: 'reports' },
  { path: '/settings', label: '系统设置', symbol: 'settings' },
] as const
