// formComponentLifecycle 桥接原生 FormMaking 注册组件的销毁清理；Vue 2 不执行 beforeUnmount，旧引用会让后续联动拿到历史实例数组。
// 以组件实际注入的注册/注销能力限定范围，统一覆盖 report、card、inline、列、页签与折叠容器，不按字段或表单打补丁。
export const formComponentLifecycle = {
  beforeDestroy () {
    const inject = this.$options.inject || {}
    if (!inject.generateComponentInstance || !inject.deleteComponentInstance) return
    const cleanup = this.$options.beforeUnmount
    if (typeof cleanup === 'function') cleanup.call(this)
  }
}
