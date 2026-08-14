import App from './App.vue'
import '@runtime/main.js'

// 隔离 iframe 内不展示目标登录页：目标 axios 遇到 401/请求失败时会 replace('/login')，
// 这会销毁表单工作区并吞掉后续指令；统一中止该导航，保证会话内工作区不被导航打断。
window.$router.beforeEach((to, from, next) => {
  if (to.path === '/login') {
    next(false)
    return
  }
  next()
})

// 完整 rsh-flow-components 入口先注册真实 FormMaking、自定义组件、Vuex 与原生路由；本地只追加隔离的配置路由。
window.$router.addRoutes([{ path: '/test-auto-form', name: 'testAutoFormRuntime', component: App }])
window.$router.replace('/test-auto-form').catch(() => {})
