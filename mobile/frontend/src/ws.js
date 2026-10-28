const TOKEN = (typeof window !== 'undefined' && window.__AIPANYI_TOKEN__) || ''

let socket = null
let reconnectTimer = null
let reconnectDelay = 1000
let closedByApp = false

const handlers = new Map()

function emit(event, data) {
  const set = handlers.get(event)
  if (!set) return
  for (const fn of Array.from(set)) {
    try {
      fn(data)
    } catch (e) {
      console.error('[ws] handler error', event, e)
    }
  }
}

function scheduleReconnect() {
  if (closedByApp || reconnectTimer) return
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connectWS()
  }, reconnectDelay)
  reconnectDelay = Math.min(reconnectDelay * 2, 10000)
}

export function connectWS() {
  if (typeof window === 'undefined') return
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) return
  if (!TOKEN || TOKEN === '__AIPANYI_TOKEN_VALUE__') return
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  try {
    socket = new WebSocket(`${proto}//${location.host}/ws?token=${encodeURIComponent(TOKEN)}`)
  } catch (e) {
    scheduleReconnect()
    return
  }
  socket.onopen = () => {
    reconnectDelay = 1000
  }
  socket.onmessage = (ev) => {
    let frame
    try {
      frame = JSON.parse(ev.data)
    } catch {
      return
    }
    if (frame && frame.event) emit(frame.event, frame.data)
  }
  socket.onclose = () => {
    socket = null
    scheduleReconnect()
  }
  socket.onerror = () => {
    try {
      socket && socket.close()
    } catch {}
  }
}

export function closeWS() {
  closedByApp = true
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  try {
    socket && socket.close()
  } catch {}
  socket = null
}

/** 订阅事件，返回取消函数。 */
export function onWS(event, fn) {
  if (!handlers.has(event)) handlers.set(event, new Set())
  handlers.get(event).add(fn)
  connectWS()
  return () => offWS(event, fn)
}

export function offWS(event, fn) {
  const set = handlers.get(event)
  if (!set) return
  if (fn) set.delete(fn)
  else set.clear()
  if (set.size === 0) handlers.delete(event)
}

if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) {
      reconnectDelay = 1000
      connectWS()
    }
  })
}
