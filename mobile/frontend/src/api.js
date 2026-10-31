const TOKEN = (typeof window !== 'undefined' && window.__AIPANYI_TOKEN__) || ''

export class ApiError extends Error {
  constructor(message, status) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/**
 * 调用 Go 核心的 /api/call。
 * 与桌面端 webserver 的协议一致：{method, args} -> {data} / {error}。
 */
export async function call(method, ...args) {
  if (!TOKEN || TOKEN === '__AIPANYI_TOKEN_VALUE__') {
    throw new ApiError('页面 token 未注入，请通过 Android App 或本地 Web 服务打开', 401)
  }
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 180000)
  let resp
  try {
    resp = await fetch('/api/call', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Aipanyi-Token': TOKEN
      },
      body: JSON.stringify({ method, args }),
      signal: controller.signal
    })
  } catch (e) {
    if (e && e.name === 'AbortError') throw new ApiError('请求超时，请检查网络或 AI 服务', 0)
    throw new ApiError('无法连接本地服务：' + (e?.message || e), 0)
  } finally {
    clearTimeout(timer)
  }

  const text = await resp.text()
  let payload
  try {
    payload = text ? JSON.parse(text) : {}
  } catch {
    throw new ApiError(text || `HTTP ${resp.status}`, resp.status)
  }
  if (resp.status === 401 || resp.status === 403) {
    throw new ApiError('本地服务鉴权失败，请重启 App', resp.status)
  }
  if (!resp.ok) {
    throw new ApiError(payload.error || `HTTP ${resp.status}`, resp.status)
  }
  if (payload.error) {
    throw new ApiError(payload.error, resp.status)
  }
  return payload.data
}

export const api = {
  // 自选
  getWatchlist: () => call('GetWatchlist'),
  searchStocks: (keyword) => call('SearchStocks', keyword),
  addStock: (code) => call('AddStock', code),
  removeStock: (code) => call('RemoveStock', code),
  setPosition: (code, costPrice, volume) => call('SetPosition', code, Number(costPrice) || 0, Number(volume) || 0),
  getStockDetail: (code) => call('GetStockDetail', code),

  // AI 分析
  startAnalysis: (code) => call('StartAnalysis', code),
  cancelAnalysis: () => call('CancelAnalysis'),
  getAnalysis: (code) => call('GetAnalysis', code),
  extractAlerts: (code) => call('ExtractAlerts', code),

  // AI 配置
  listAIConfigs: () => call('ListAIConfigs'),
  saveAIConfig: (cfg) => call('SaveAIConfig', cfg),
  deleteAIConfig: (id) => call('DeleteAIConfig', Number(id)),
  testAIConfig: (cfg) => call('TestAIConfig', cfg),

  // 提示词
  listPrompts: () => call('ListPrompts'),
  savePrompt: (p) => call('SavePrompt', p),
  deletePrompt: (id) => call('DeletePrompt', Number(id)),

  // 提醒
  listAlerts: (code = '') => call('ListAlerts', code),
  saveAlert: (alert) => call('SaveAlert', alert),
  deleteAlert: (id) => call('DeleteAlert', Number(id)),
  setAlertEnabled: (id, enabled) => call('SetAlertEnabled', Number(id), !!enabled),
  listAlertEvents: (limit = 100) => call('ListAlertEvents', Number(limit)),

  // 设置
  getSettings: () => call('GetMobileSettings'),
  saveSettings: (s) => call('SaveMobileSettings', s),
  getVersionInfo: () => call('GetVersionInfo'),
  getMonitorStatus: () => call('GetMonitorStatus'),
  getDataSummary: () => call('GetDataSummary'),
  sendTestNotification: () => call('SendTestNotification')
}
