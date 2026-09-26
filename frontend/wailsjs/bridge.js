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

/* ------------------------------------------------------------------ */
/* Wails runtime 兼容层                                                 */
/*                                                                     */
/* 以下导出与 wailsjs/runtime/runtime.js 的公开 API 一一对应，但内部     */
/* 全部走上面的双模式实现：桌面模式转发 window.runtime，Web 模式走       */
/* /api/call + WebSocket，窗口类 API 安全降级。                         */
/*                                                                     */
/* 为什么放在这里而不是 runtime.js：                                    */
/*   runtime/runtime.js 由 `wails build` 无条件重新生成，任何手写改动     */
/*   都会在下次构建时被静默覆盖；bridge.js 不在 wails 的管理范围内，      */
/*   是唯一能长期驻留自定义代码的位置。                                 */
/* ------------------------------------------------------------------ */

/** Web 模式下窗口类 API 的空操作实现（保留调用链不中断）。 */
const noopWindow = new Proxy({}, {
  get: () => () => {
    if (!isWails) return undefined;
  },
});

function win() {
  return isWails ? window.runtime : noopWindow;
}

export function LogPrint(message) {
  if (isWails) window.runtime.LogPrint(message);
  else console.log(message);
}

export function LogTrace(message) {
  if (isWails) window.runtime.LogTrace(message);
}

export function LogDebug(message) {
  if (isWails) window.runtime.LogDebug(message);
}

export function LogInfo(message) {
  if (isWails) window.runtime.LogInfo(message);
}

export function LogWarning(message) {
  if (isWails) window.runtime.LogWarning(message);
}

export function LogError(message) {
  if (isWails) window.runtime.LogError(message);
  else console.error(message);
}

export function LogFatal(message) {
  if (isWails) window.runtime.LogFatal(message);
  else console.error(message);
}

export function EventsOnMultiple(eventName, callback, maxCallbacks) {
  return onEvent(eventName, callback, maxCallbacks);
}

export function EventsOn(eventName, callback) {
  return EventsOnMultiple(eventName, callback, -1);
}

export function EventsOff(eventName, ...additionalEventNames) {
  return offEvent(eventName, ...additionalEventNames);
}

export function EventsOffAll() {
  return offAllEvents();
}

export function EventsOnce(eventName, callback) {
  return EventsOnMultiple(eventName, callback, 1);
}

export function EventsEmit(eventName, ...data) {
  if (!isWails) return; // Web 模式前端无上报通道，静默忽略
  window.runtime.EventsEmit(eventName, ...data);
}

export function WindowReload() { return win().WindowReload(); }
export function WindowReloadApp() { return win().WindowReloadApp(); }
export function WindowSetAlwaysOnTop(b) { return win().WindowSetAlwaysOnTop(b); }
export function WindowSetSystemDefaultTheme() { return win().WindowSetSystemDefaultTheme(); }
export function WindowSetLightTheme() { return win().WindowSetLightTheme(); }
export function WindowSetDarkTheme() { return win().WindowSetDarkTheme(); }
export function WindowCenter() { return win().WindowCenter(); }
export function WindowSetTitle(title) { if (isWails) window.runtime.WindowSetTitle(title); }
export function WindowFullscreen() { return win().WindowFullscreen(); }
export function WindowUnfullscreen() { return win().WindowUnfullscreen(); }
export function WindowIsFullscreen() { return win().WindowIsFullscreen(); }
export function WindowGetSize() { return win().WindowGetSize(); }
export function WindowSetSize(width, height) { return win().WindowSetSize(width, height); }
export function WindowSetMaxSize(width, height) { return win().WindowSetMaxSize(width, height); }
export function WindowSetMinSize(width, height) { return win().WindowSetMinSize(width, height); }
export function WindowSetPosition(x, y) { return win().WindowSetPosition(x, y); }
export function WindowGetPosition() { return win().WindowGetPosition(); }
export function WindowHide() { return win().WindowHide(); }
export function WindowShow() { return win().WindowShow(); }
export function WindowMaximise() { return win().WindowMaximise(); }
export function WindowToggleMaximise() { return win().WindowToggleMaximise(); }
export function WindowUnmaximise() { return win().WindowUnmaximise(); }
export function WindowIsMaximised() { return win().WindowIsMaximised(); }
export function WindowMinimise() { return win().WindowMinimise(); }
export function WindowUnminimise() { return win().WindowUnminimise(); }
export function WindowSetBackgroundColour(R, G, B, A) { return win().WindowSetBackgroundColour(R, G, B, A); }
export function ScreenGetAll() { return win().ScreenGetAll(); }
export function WindowIsMinimised() { return win().WindowIsMinimised(); }
export function WindowIsNormal() { return win().WindowIsNormal(); }

export function BrowserOpenURL(url) { return openURL(url); }
export function Environment() { return environment(); }

export function Quit() { if (isWails) window.runtime.Quit(); }
export function Hide() { if (isWails) window.runtime.Hide(); }
export function Show() { if (isWails) window.runtime.Show(); }

export function ClipboardGetText() {
  if (isWails) return window.runtime.ClipboardGetText();
  return navigator.clipboard.readText();
}

export function ClipboardSetText(text) {
  if (isWails) return window.runtime.ClipboardSetText(text);
  return navigator.clipboard.writeText(text);
}

export function OnFileDrop(callback, useDropTarget) {
  if (!isWails) return () => {}; // Web 模式使用元素级 dragover/drop，见各组件
  return window.runtime.OnFileDrop(callback, useDropTarget);
}

export function OnFileDropOff() { if (isWails) window.runtime.OnFileDropOff(); }
export function CanResolveFilePaths() { return isWails ? window.runtime.CanResolveFilePaths() : false; }
export function ResolveFilePaths(files) { return isWails ? window.runtime.ResolveFilePaths(files) : []; }

export function InitializeNotifications() { return isWails ? window.runtime.InitializeNotifications() : Promise.resolve(false); }
export function CleanupNotifications() { if (isWails) window.runtime.CleanupNotifications(); }
export function IsNotificationAvailable() { return isWails ? window.runtime.IsNotificationAvailable() : false; }
export function RequestNotificationAuthorization() { return isWails ? window.runtime.RequestNotificationAuthorization() : Promise.resolve('denied'); }
export function CheckNotificationAuthorization() { return isWails ? window.runtime.CheckNotificationAuthorization() : Promise.resolve('denied'); }
export function SendNotification(options) {
  if (isWails) return window.runtime.SendNotification(options);
  if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
    new Notification(options.title || 'AI盘译', { body: options.message });
  }
  return Promise.resolve();
}
export function SendNotificationWithActions(options) { return SendNotification(options); }
export function RegisterNotificationCategory(category) { if (isWails) window.runtime.RegisterNotificationCategory(category); }
export function RemoveNotificationCategory(categoryId) { if (isWails) window.runtime.RemoveNotificationCategory(categoryId); }
export function RemoveAllPendingNotifications() { if (isWails) window.runtime.RemoveAllPendingNotifications(); }
export function RemovePendingNotification(identifier) { if (isWails) window.runtime.RemovePendingNotification(identifier); }
export function RemoveAllDeliveredNotifications() { if (isWails) window.runtime.RemoveAllDeliveredNotifications(); }
export function RemoveDeliveredNotification(identifier) { if (isWails) window.runtime.RemoveDeliveredNotification(identifier); }
export function RemoveNotification(identifier) { if (isWails) window.runtime.RemoveNotification(identifier); }
