/*
 * bridge.js — AI盘译 前端双模式桥接层
 *
 * 目的：让同一份前端代码同时运行在两种形态下，业务代码零改动：
 *   - 桌面模式（Wails）：检测到 window.runtime / window.go 时，直接调用原生绑定；
 *   - Web 模式（-tags web）：通过 POST /api/call 反射派发 + WebSocket 事件总线。
 *
 * 鉴权：Web 模式下由服务端把启动令牌注入 index.html（window.__AIPANYI_TOKEN__），
 *       本模块在每次请求头与 WS 查询参数中携带。
 *
 * 本文件由 webserver shim 引入，替代 wails 生成的 window['go'] 直接调用。
 */

const TOKEN = (typeof window !== 'undefined' && window.__AIPANYI_TOKEN__) || '';

/** 是否运行在 Wails 桌面容器中。 */
export const isWails = typeof window !== 'undefined'
  && !!(window.runtime && window['go'] && window['go']['main'] && window['go']['main']['App']);

/** 是否运行在浏览器（Web 服务）模式。 */
export const isWeb = !isWails;

/* ------------------------------------------------------------------ */
/* 绑定调用                                                            */
/* ------------------------------------------------------------------ */

/**
 * 调用一个后端绑定方法。
 * @param {string} name 方法名（与 Go 侧 App 的方法名一致）
 * @param {Array} args  参数数组
 * @returns {Promise<any>} Wails 模式下返回原生 Promise；Web 模式下返回解析后的 data
 */
export function call(name, args = []) {
  if (isWails) {
    return window['go']['main']['App'][name](...args);
  }
  return fetch('/api/call', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Aipanyi-Token': TOKEN,
    },
    body: JSON.stringify({ method: name, args }),
  }).then(async (resp) => {
    const body = await resp.json().catch(() => ({}));
    if (!resp.ok) {
      throw new Error(body.error || ('HTTP ' + resp.status));
    }
    if (body.error) {
      // 与 Wails 语义对齐：后端 error 以前端异常形式抛出
      throw new Error(body.error);
    }
    return body.data;
  });
}

/* ------------------------------------------------------------------ */
/* 事件总线（替代 runtime.EventsOn / EventsOff）                        */
/* ------------------------------------------------------------------ */

const listeners = new Map(); // eventName -> Set<callback>
let ws = null;
let wsAlive = false;
let reconnectTimer = null;

function dispatch(eventName, data) {
  const set = listeners.get(eventName);
  if (!set) return;
  for (const cb of Array.from(set)) {
    try {
      cb(data);
    } catch (e) {
      console.error('[bridge] 事件回调异常:', eventName, e);
    }
  }
}

function connectWS() {
  if (isWails || wsAlive) return;
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${proto}//${location.host}/ws?token=${encodeURIComponent(TOKEN)}`;
  try {
    ws = new WebSocket(url);
  } catch (e) {
    scheduleReconnect();
    return;
  }
  ws.onopen = () => { wsAlive = true; };
  ws.onmessage = (ev) => {
    let frame;
    try { frame = JSON.parse(ev.data); } catch { return; }
    if (frame && frame.event) dispatch(frame.event, frame.data);
  };
  ws.onclose = () => { wsAlive = false; scheduleReconnect(); };
  ws.onerror = () => { try { ws.close(); } catch {} };
}

function scheduleReconnect() {
  if (reconnectTimer) return;
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connectWS();
  }, 2000);
}

/** 订阅事件。Wails 模式下直接走原生实现。 */
export function onEvent(eventName, callback, maxCallbacks = -1) {
  if (isWails) {
    return window.runtime.EventsOnMultiple(eventName, callback, maxCallbacks);
  }
  connectWS();
  if (!listeners.has(eventName)) listeners.set(eventName, new Set());
  listeners.get(eventName).add(callback);
  return () => offEvent(eventName, callback);
}

/** 取消订阅。 */
export function offEvent(eventName, ...additionalEventNames) {
  if (isWails) {
    return window.runtime.EventsOff(eventName, ...additionalEventNames);
  }
  const names = [eventName, ...additionalEventNames].filter(Boolean);
  for (const n of names) listeners.delete(n);
}

/** 取消全部订阅。 */
export function offAllEvents() {
  if (isWails) return window.runtime.EventsOffAll();
  listeners.clear();
}

/** Web 模式下就绪的连接数（调试用）。 */
export function wsState() {
  return { isWails, isWeb, wsConnected: wsAlive, events: listeners.size };
}

/* ------------------------------------------------------------------ */
/* 环境信息                                                            */
/* ------------------------------------------------------------------ */

/** 兼容 runtime.Environment()：Web 模式下返回浏览器环境。 */
export function environment() {
  if (isWails) return window.runtime.Environment();
  return Promise.resolve({
    platform: 'browser',
    arch: 'web',
    buildType: 'web',
    isWeb: true,
  });
}

/** 兼容 runtime.BrowserOpenURL()。 */
export function openURL(url) {
  if (isWails) return window.runtime.BrowserOpenURL(url);
  window.open(url, '_blank', 'noopener');
}
