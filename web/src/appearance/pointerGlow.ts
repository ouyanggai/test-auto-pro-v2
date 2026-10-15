// 指针移动只合并成一个待绘制帧，并且只写光晕本身的合成变换，不触发整页变量重算。
export function startPointerGlow(spot: HTMLElement): () => void {
  const host = spot.ownerDocument.defaultView!
  let x = host.innerWidth * .7, y = host.innerHeight * .3
  let frame: number | undefined
  let stopped = false
  const half = spot.offsetWidth / 2
  const paint = () => {
    frame = undefined
    if (!stopped) spot.style.transform = 'translate3d(' + (x - half) + 'px, ' + (y - half) + 'px, 0)'
  }
  const onMove = (event: PointerEvent) => {
    x = event.clientX
    y = event.clientY
    if (frame === undefined) frame = host.requestAnimationFrame(paint)
  }
  host.addEventListener('pointermove', onMove, { passive: true })
  paint()
  return () => {
    stopped = true
    host.removeEventListener('pointermove', onMove)
    // 切皮肤或卸载时取消尚未执行的帧，防止旧回调覆盖下一套皮肤的光晕位置。
    if (frame !== undefined) host.cancelAnimationFrame(frame)
    spot.style.removeProperty('transform')
  }
}
