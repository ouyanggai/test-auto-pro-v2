import { nextTick } from 'vue'

// 记录最近一次指针按下位置，换肤动画从用户点击处向外扩散；没有指针（键盘操作）时从屏幕中上部开始。
let lastPoint: { x: number; y: number } | null = null
if (typeof window !== 'undefined') {
  window.addEventListener('pointerdown', event => { lastPoint = { x: event.clientX, y: event.clientY } }, { capture: true, passive: true })
}

type ViewTransitionDocument = Document & { startViewTransition?: (update: () => Promise<void>) => { ready: Promise<void>; finished: Promise<void> } }

/**
 * 以圆形扩散的 View Transition 执行一次外观更新。
 * update 只能修改外观偏好；浏览器不支持时直接执行，行为与无动画完全一致。
 * 动画只是新旧快照之间的遮罩过渡，页面、iframe 和画布实例不会重建。
 */
export async function switchAppearance(update: () => void): Promise<void> {
  const doc = document as ViewTransitionDocument
  if (!doc.startViewTransition) {
    update()
    return
  }
  const x = lastPoint?.x ?? window.innerWidth / 2
  const y = lastPoint?.y ?? window.innerHeight * 0.3
  const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
  const transition = doc.startViewTransition(async () => {
    update()
    // 等 Vue 与 Naive 把新主题渲染完再让浏览器截新快照。
    await nextTick()
  })
  try {
    await transition.ready
    document.documentElement.animate(
      { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
      { duration: 720, easing: 'cubic-bezier(.65, 0, .2, 1)', pseudoElement: '::view-transition-new(root)' },
    )
    await transition.finished
  } catch {
    // 快照被页面切换打断时，外观更新本身已经在回调里完成，无需补偿。
  }
}
