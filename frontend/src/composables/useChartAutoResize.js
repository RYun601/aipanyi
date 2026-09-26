import { onBeforeUnmount, onMounted, ref } from 'vue'

/**
 * useChartAutoResize — ECharts / lightweight-charts 实例的容器自适应。
 *
 * 背景：原代码里大量图表只在 echarts.init 时按容器尺寸渲染一次，
 * 之后窗口缩放、侧栏折叠、面板显隐都不再 resize，小屏/分屏下图表被裁切或留白。
 *
 * 用法：
 *   const chartBox = ref(null)
 *   const { register } = useChartAutoResize(chartBox)
 *   const chart = echarts.init(chartBox.value)
 *   register(chart)          // 之后容器尺寸变化会自动 chart.resize()
 *
 * 说明：
 *   - ResizeObserver 优先（能感知容器自身尺寸变化，而非仅窗口尺寸）；
 *   - window resize 作为兜底（部分环境 ResizeObserver 不可用）；
 *   - rAF 合流，避免连续回调导致的重排风暴；
 *   - 组件卸载时自动解绑，chart 实例随 DOM 一起回收，无需手动 dispose。
 */
export function useChartAutoResize(containerRef) {
  const charts = new Set()
  let observer = null
  let rafId = null

  function flush() {
    rafId = null
    charts.forEach((c) => {
      try { c && c.resize && c.resize() } catch (e) { /* 实例已销毁，忽略 */ }
    })
  }

  function schedule() {
    if (rafId !== null) return
    rafId = requestAnimationFrame(flush)
  }

  function register(chart) {
    if (!chart) return () => {}
    charts.add(chart)
    // 注册后立刻按当前容器尺寸校准一次，修复“先出图后布局”的错位
    schedule()
    return () => charts.delete(chart)
  }

  function observe() {
    const el = containerRef && containerRef.value
    if (!el || typeof ResizeObserver === 'undefined') return
    if (observer) observer.disconnect()
    observer = new ResizeObserver(schedule)
    observer.observe(el)
  }

  onMounted(() => {
    observe()
    window.addEventListener('resize', schedule, { passive: true })
  })

  onBeforeUnmount(() => {
    if (observer) { observer.disconnect(); observer = null }
    if (rafId !== null) { cancelAnimationFrame(rafId); rafId = null }
    charts.clear()
    window.removeEventListener('resize', schedule)
  })

  // 容器 ref 可能晚一 tick 才挂载
  const retry = setTimeout(observe, 200)
  onBeforeUnmount(() => clearTimeout(retry))

  return { register }
}

/**
 * useWindowSize — 直接给出窗口宽高（rAF 合流）。
 * 供需要按宽度切换图表配置（如隐藏部分坐标轴标签）的组件使用。
 */
export function useWindowSize() {
  const width = ref(typeof window !== 'undefined' ? window.innerWidth : 1280)
  const height = ref(typeof window !== 'undefined' ? window.innerHeight : 800)
  let rafId = null

  function onResize() {
    if (rafId !== null) return
    rafId = requestAnimationFrame(() => {
      rafId = null
      width.value = window.innerWidth
      height.value = window.innerHeight
    })
  }

  onMounted(() => window.addEventListener('resize', onResize, { passive: true }))
  onBeforeUnmount(() => {
    window.removeEventListener('resize', onResize)
    if (rafId !== null) cancelAnimationFrame(rafId)
  })

  return { width, height }
}

/**
 * attachChartResize — 直接对 (DOM 元素, chart 实例) 挂自适应，返回解绑函数。
 * 适用于图表容器通过 getElementById / querySelector 动态获取的场景（如弹窗内图表）。
 */
export function attachChartResize(el, chart) {
  if (!el || !chart) return () => {}
  let rafId = null
  function schedule() {
    if (rafId !== null) return
    rafId = requestAnimationFrame(() => {
      rafId = null
      try { chart.resize && chart.resize() } catch (e) { /* 已销毁，忽略 */ }
    })
  }
  let observer = null
  if (typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(schedule)
    observer.observe(el)
  }
  window.addEventListener('resize', schedule, { passive: true })
  return () => {
    if (observer) { observer.disconnect(); observer = null }
    window.removeEventListener('resize', schedule)
    if (rafId !== null) cancelAnimationFrame(rafId)
  }
}
