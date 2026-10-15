// 欠阻尼阶跃响应：略有回弹，末端收敛。固定 600ms、60 个采样点，交给浏览器插值，不用 JS 逐帧驱动。
export const studioSpring = 'linear(' + Array.from({ length: 61 }, (_, index) => {
  if (index === 0) return '0'
  if (index === 60) return '1'
  const time = index / 60 * .6
  return (1 - Math.exp(-11 * time) * (Math.cos(15 * time) + 11 / 15 * Math.sin(15 * time))).toFixed(5)
}).join(',') + ')'

// 动画只持有外观节点；不读取业务 store、不触发请求，也不改页面或 iframe 的身份。
export function startStudioMotion(root: HTMLElement, content: HTMLElement) {
  const doc = root.ownerDocument
  const host = doc.defaultView!
  const running = new Set<Animation>()
  const seenDialogs = new WeakSet<Element>()
  let stopped = false
  let target: HTMLElement | null = null
  let frame: number | undefined
  let point = { x: 0, y: 0 }
  root.style.setProperty('--studio-spring', studioSpring)

  const animate = (element: HTMLElement, delay = 0, dialog = false) => {
    if (stopped || doc.hidden || running.size >= 12 || !element.isConnected) return
    // 独立 translate 不覆盖 Naive 弹窗自身的 transform；不动画布、表单或表格数据行。
    const animation = element.animate([
      { opacity: .35, translate: dialog ? '0 18px' : '0 10px' },
      { opacity: 1, translate: '0 0' },
    ], { duration: dialog ? 540 : 460, delay, easing: studioSpring })
    running.add(animation)
    void animation.finished.then(() => running.delete(animation), () => running.delete(animation))
  }
  const enterPage = () => {
    if (content.classList.contains('app-main--flush')) return
    const regions = content.querySelectorAll<HTMLElement>('.n-layout-scroll-container > * > :is(header, .page-heading, .plan-filters, .runs-view__toolbar, .n-card, .n-data-table, table, .settings-grid)')
    Array.from(regions).slice(0, 5).forEach((region, index) => animate(region, index * 34))
  }
  const enterDialog = (dialog: HTMLElement) => {
    if (seenDialogs.has(dialog)) return
    seenDialogs.add(dialog)
    animate(dialog, 0, true)
  }
  // 只处理新插入的弹窗；既有内容的轮询更新、列表翻页和输入不会反复播放入场。
  const observer = new MutationObserver(records => {
    for (const record of records) for (const added of record.addedNodes) {
      if (!(added instanceof HTMLElement)) continue
      if (added.matches('.app-dialog')) enterDialog(added)
      else added.querySelectorAll<HTMLElement>('.app-dialog').forEach(enterDialog)
    }
  })
  observer.observe(doc.body, { childList: true, subtree: true })

  const clearReflection = () => {
    if (frame !== undefined) host.cancelAnimationFrame(frame)
    frame = undefined
    target?.classList.remove('studio-reflecting')
    target?.style.removeProperty('--studio-x')
    target?.style.removeProperty('--studio-y')
    target = null
  }
  const paint = () => {
    frame = undefined
    if (stopped || !target?.isConnected) return
    const rect = target.getBoundingClientRect()
    target.style.setProperty('--studio-x', point.x - rect.left + 'px')
    target.style.setProperty('--studio-y', point.y - rect.top + 'px')
  }
  const move = (event: PointerEvent) => {
    if (stopped || doc.hidden || event.pointerType === 'touch') return
    const next = event.target instanceof Element
      ? event.target.closest<HTMLElement>('.n-card, .setting-card, .app-dialog, .studio-navigation__link') : null
    if (next !== target) {
      clearReflection()
      target = next
      target?.classList.add('studio-reflecting')
    }
    if (!target) return
    point = { x: event.clientX, y: event.clientY }
    if (frame === undefined) frame = host.requestAnimationFrame(paint)
  }
  const visibility = () => {
    root.dataset.studioMotion = doc.hidden ? 'paused' : 'running'
    if (doc.hidden) clearReflection()
    for (const animation of running) doc.hidden ? animation.pause() : animation.play()
  }
  doc.addEventListener('pointermove', move, { passive: true })
  doc.addEventListener('pointerleave', clearReflection)
  doc.addEventListener('visibilitychange', visibility)
  visibility()
  enterPage()
  return {
    enterPage,
    stop() {
      stopped = true
      observer.disconnect()
      doc.removeEventListener('pointermove', move)
      doc.removeEventListener('pointerleave', clearReflection)
      doc.removeEventListener('visibilitychange', visibility)
      clearReflection()
      for (const animation of running) animation.cancel()
      running.clear()
      root.style.removeProperty('--studio-spring')
      delete root.dataset.studioMotion
    },
  }
}
