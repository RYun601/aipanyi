/** Android WebView 注入的 native bridge；浏览器里为 null。 */
export function nativeBridge() {
  if (typeof window === 'undefined') return null
  return window.AiPanyi || null
}

export function isNative() {
  return !!nativeBridge()
}

function safe(fn, fallback) {
  try {
    return fn()
  } catch (e) {
    console.warn('[native]', e)
    return fallback
  }
}

export const native = {
  notificationsEnabled() {
    const b = nativeBridge()
    return b ? !!safe(() => b.notificationsEnabled(), false) : false
  },
  requestNotifications() {
    const b = nativeBridge()
    return b ? safe(() => b.requestNotifications(), false) : false
  },
  isIgnoringBatteryOptimizations() {
    const b = nativeBridge()
    return b ? !!safe(() => b.isIgnoringBatteryOptimizations(), false) : false
  },
  openBatterySettings() {
    const b = nativeBridge()
    if (b) safe(() => b.openBatterySettings(), null)
  },
  openAutoStartSettings() {
    const b = nativeBridge()
    if (b) safe(() => b.openAutoStartSettings(), null)
  },
  openNotificationSettings() {
    const b = nativeBridge()
    if (b) safe(() => b.openNotificationSettings(), null)
  },
  version() {
    const b = nativeBridge()
    return b ? String(safe(() => b.appVersion(), '') || '') : ''
  }
}
