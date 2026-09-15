import App from './App.vue'
import '@runtime/main.js'
import { FORM_RUNTIME_VERSION } from './runtime/protocol'

// 执行引擎（无头探针与后续执行桥接）与 iframe 宿主共用同一版本标识；
// 在真实入口脚本执行后立即可读，不依赖组件挂载完成，探针据此确认加载的是真实 runtime。
window.__formRuntimeVersion__ = FORM_RUNTIME_VERSION

// 隔离 iframe 内不展示目标登录页：目标 axios 遇到 401/请求失败时会 replace('/login')，
// 这会销毁表单工作区并吞掉后续指令；统一中止该导航，保证会话内工作区不被导航打断。
window.$router.beforeEach((to, from, next) => {
  if (to.path === '/login') {
    next(false)
    return
  }
  next()
})

// 完整 rsh-flow-components 入口先注册真实 FormMaking、自定义组件、Vuex 与原生路由；配置协议只把目标宿主页面挂到隔离路由下。
// 表单页面由 runtime-source 自己渲染，本地 App 只作为数据协议适配层，不导入或复制宿主表单组件。
window.$router.addRoutes([{
  path: '/test-auto-form',
  name: 'testAutoFormRuntime',
  component: App,
  children: [{
    path: '',
    component: () => import('@runtime/views/GroupApproveManage/Submitted/components/OtherSteps2.vue')
  }]
}])
window.$router.replace('/test-auto-form').catch(() => {})
