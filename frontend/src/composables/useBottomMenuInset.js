import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

/**
 * useBottomMenuInset — 测量底部固定导航的真实高度，转化为内容区安全边距。
 *
 * 背景：App.vue 的底部菜单是 position:fixed，窄屏下菜单项会自动换行/折叠，
 * 高度不可预估；此前用固定 96px 估算内容区高度，小屏必然被遮挡。
 * 这里用 ResizeObserver 观测菜单实际高度，把内容区的可用高度动态算出来。
 */
export function useBottomMenuInset(target) {
  const inset = ref(96)

  let observer = null
  let retryTimer = null
  let stopWatch = null

  /** 兼容三种 ref 形态：DOM 元素 / Vue 组件实例（取 $el）/ null */
  function resolveEl(v) {
    if (!v) return null
    if (typeof Element !== 'undefined' && v instanceof Element) return v
    if (v.$el && typeof v.$el.getBoundingClientRect === 'function') return v.$el
    if (typeof v.getBoundingClientRect === 'function') return v
    return null
  }

  function measure() {
    const el = resolveEl(target && target.value)
    if (!el) return
    const h = el.getBoundingClientRect().height
    if (h > 0) inset.value = Math.ceil(h)
  }

  function observe() {
    if (typeof ResizeObserver === 'undefined') return
    if (observer) observer.disconnect()
    const el = resolveEl(target && target.value)
    if (!el) return
    observer = new ResizeObserver(() => measure())
    observer.observe(el)
  }

  onMounted(() => {
    measure()
    observe()
    // Naive UI menu 的 responsive 折叠是异步生效的，首次渲染后高度可能才稳定
    retryTimer = setTimeout(measure, 300)
    // 兜底：DOM 引用在下一 tick 才挂上时补一次
    stopWatch = watch(
      () => resolveEl(target && target.value) || null,
      () => { measure(); observe() },
      { flush: 'post' }
    )
  })

  onBeforeUnmount(() => {
    if (observer) { observer.disconnect(); observer = null }
    if (retryTimer) { clearTimeout(retryTimer); retryTimer = null }
    if (stopWatch) { stopWatch(); stopWatch = null }
    window.removeEventListener('resize', measure)
  })

  window.addEventListener('resize', measure, { passive: true })

  /** 内容区可用高度（视口高度减去底部导航） */
  const contentMaxHeight = computed(() => 'calc(100dvh - ' + inset.value + 'px)')
  /** 底部导航自身高度，供其它组件复用 */
  return { inset, contentMaxHeight }
}