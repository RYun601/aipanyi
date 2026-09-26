/*
 _       __      _ __
| |     / /___ _(_) /____
| | /| / / __ `/ / / ___/
| |/ |/ / /_/ / / (__  )
|__/|__/\__,_/_/_/____/
The electron alternative for Go
(c) Lea Anthony 2019-present

-- AI盘译 双模式适配 -------------------------------------------------
本文件在 Wails 原生实现之上包了一层桥接：
  - EventsOn/EventsOff/EventsOnce 等事件 API 走 bridge.js（Web 模式下经 WebSocket）；
  - 窗口类 API（Window*）在 Web 模式下退化为安全空操作，避免浏览器报错；
  - 其余（日志、剪贴板等）保持原生行为。
--------------------------------------------------------------------
*/

import { onEvent, offEvent, offAllEvents, openURL, environment, isWails } from '../bridge.js';

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
