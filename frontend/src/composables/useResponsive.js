/*
 * useResponsive.js — 响应式断点 composable
 *
 * 提供窗口宽度的响应式状态，供组件按需调整布局（而非全靠 CSS）。
 * 断点与 Naive UI / 常见实践对齐：
 *   < 768   手机
 *   768~1279 平板
 *   >= 1280 桌面
 *
 * 用法：
 *   const { isMobile, isTablet, isDesktop, width } = useResponsive()
 */

import { ref, computed, onMounted, onBeforeUnmount } from 'vue'

const BREAKPOINTS = { mobile: 768, tablet: 1280 }

/** 模块级单例：多组件共用同一份监听，避免重复绑定 resize。 */
const width = ref(typeof window !== 'undefined' ? window.innerWidth : 1280)
let bound = 0
let timer = null

function onResize() {
  if (timer) return
  timer = requestAnimationFrame(() => {
    timer = null
    width.value = window.innerWidth
  })
}

function bind() {
  if (bound === 0 && typeof window !== 'undefined') {
    window.addEventListener('resize', onResize, { passive: true })
    window.addEventListener('orientationchange', onResize, { passive: true })
  }
  bound++
}

function unbind() {
  bound = Math.max(0, bound - 1)
  if (bound === 0 && typeof window !== 'undefined') {
    window.removeEventListener('resize', onResize)
    window.removeEventListener('orientationchange', onResize)
    if (timer) { cancelAnimationFrame(timer); timer = null }
  }
}

export function useResponsive() {
  onMounted(bind)
  onBeforeUnmount(unbind)

  const isMobile = computed(() => width.value < BREAKPOINTS.mobile)
  const isTablet = computed(() => width.value >= BREAKPOINTS.mobile && width.value < BREAKPOINTS.tablet)
  const isDesktop = computed(() => width.value >= BREAKPOINTS.tablet)
  /** 窄屏（手机 + 平板），用于"需要紧凑布局"的场景 */
  const isNarrow = computed(() => width.value < BREAKPOINTS.tablet)

  return { width, isMobile, isTablet, isDesktop, isNarrow }
}

export { BREAKPOINTS }
