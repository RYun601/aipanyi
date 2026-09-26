/*
 * fileBridge.js — 文件保存/打开的双模式桥接
 *
 * 背景：Wails 的 runtime.SaveFileDialog / OpenFileDialog 在浏览器中不存在。
 *   - 桌面模式：保持原行为，调用后端带对话框的方法（由调用方传入 desktop 回调）；
 *   - Web 模式：
 *       保存 → 直接用 Blob + a.download 触发浏览器下载（无需服务端参与）；
 *       打开 → <input type="file"> 上传到 /api/upload，拿服务端路径继续原有流程。
 */

import { isWails } from '../../wailsjs/bridge.js'

const MIME = {
  png: 'image/png',
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  gif: 'image/gif',
  webp: 'image/webp',
  md: 'text/markdown;charset=utf-8',
  markdown: 'text/markdown;charset=utf-8',
  txt: 'text/plain;charset=utf-8',
  csv: 'text/csv;charset=utf-8',
  json: 'application/json;charset=utf-8',
  zip: 'application/zip',
  xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
}

function mimeOf(filename) {
  const ext = (filename.split('.').pop() || '').toLowerCase()
  return MIME[ext] || 'application/octet-stream'
}

function base64ToBlob(base64, mime) {
  const clean = base64.includes(',') ? base64.split(',')[1] : base64
  const bin = atob(clean)
  const len = bin.length
  const bytes = new Uint8Array(len)
  for (let i = 0; i < len; i++) bytes[i] = bin.charCodeAt(i)
  return new Blob([bytes], { type: mime })
}

function triggerDownload(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 2000)
}

/**
 * 保存 base64 内容（图片 / Word 等二进制）。
 * @param filename 文件名（含扩展名）
 * @param base64   base64 字符串（可带 data: 前缀）
 * @param desktop  桌面模式回调，返回原 Wails 方法的 Promise
 */
export async function saveBase64(filename, base64, desktop) {
  if (isWails) return desktop ? desktop() : ''
  if (!base64) return ''
  triggerDownload(base64ToBlob(base64, mimeOf(filename)), filename)
  return filename
}

/**
 * 保存文本内容（Markdown / JSON / CSV 等）。
 */
export async function saveText(filename, text, desktop) {
  if (isWails) return desktop ? desktop() : ''
  if (text == null) return ''
  triggerDownload(new Blob([text], { type: mimeOf(filename) }), filename)
  return filename
}

/**
 * 选择文件并上传到服务端，返回服务端路径（Web 模式）。
 * 桌面模式返回空串，由调用方改走原对话框方法。
 * @param accept 例如 ".xlsx,.xls,.csv" 或 ".zip"
 */
export function pickAndUpload(accept = '') {
  if (isWails) return Promise.resolve('')
  return new Promise((resolve) => {
    const input = document.createElement('input')
    input.type = 'file'
    if (accept) input.accept = accept
    input.style.display = 'none'
    document.body.appendChild(input)
    input.onchange = async () => {
      const file = input.files && input.files[0]
      document.body.removeChild(input)
      if (!file) { resolve(''); return }
      try {
        const fd = new FormData()
        fd.append('file', file)
        const token = window.__AIPANYI_TOKEN__ || ''
        const resp = await fetch('/api/upload', {
          method: 'POST',
          headers: { 'X-Aipanyi-Token': token },
          body: fd,
        })
        const j = await resp.json()
        resolve(j.path || '')
      } catch (e) {
        console.error('[fileBridge] 上传失败:', e)
        resolve('')
      }
    }
    input.click()
  })
}
