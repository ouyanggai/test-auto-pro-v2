// TARGET_COMPONENT_NAMES 固定记录 rsh-flow-components bff4ef8 的 FormMaking 自定义组件注册名。
// 这些源码完整保存在 upstream 原样区，但当前均依赖旧工作台路由、Vuex 或业务接口，尚不能在隔离运行时安全启用。
export const TARGET_COMPONENT_NAMES = new Set([
  'custom-upload-excel',
  'out-bound-material-select',
  'in-bound-material-select',
  'custom-weather',
  'custome-select-project',
  'custome-expense-budgetType',
  'general-list-select-show',
  'person-mulSelect',
  'general-flow-list-mulSelect',
  'custome-info-select',
  'ltd-or-dep-select',
  'custome-file-view',
  'custome-file-import',
  'legal-contract-doctable',
  'contract-seal-review-business',
  'flow-list-mul-select',
  'request_payout',
  'city-select',
  'travel-route-planning',
  'travel-order-management'
])

// targetComponents 只允许经过隔离读取验证的目标组件进入运行时。
// 当前为空是刻意的安全边界：保留原源码与识别名，同时由模板扫描明确 unsupported，绝不降级为普通控件。
export default []
