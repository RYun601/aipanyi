export function fmtPrice(v) {
  const n = Number(v)
  if (!Number.isFinite(n) || n === 0) return '--'
  return n.toFixed(2)
}

export function fmtPercent(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '--'
  return (n >= 0 ? '+' : '') + n.toFixed(2) + '%'
}

export function fmtMoney(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '--'
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

export function fmtSignedMoney(v) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '--'
  return (n >= 0 ? '+' : '') + fmtMoney(n)
}

export function changeClass(v) {
  const n = Number(v)
  if (!Number.isFinite(n) || n === 0) return 'flat'
  return n > 0 ? 'up' : 'down'
}

export function directionText(d) {
  return d === 'below' ? '跌破' : '涨到'
}

export function timeShort(s) {
  if (!s) return ''
  return String(s).slice(5, 16)
}

export function alertStatus(a) {
  if (a.needsReview) return a.phase === 'close' ? '收盘待人工复核' : '待复核'
  if (a.status === 'position_changed') return '持仓已更新，策略待复核'
  if (a.mode !== 'strategy') return '纯盯价'
  if (a.reason === 'missing_indicators') return '已触价，等待量比或买卖盘'
  if (a.status === 'confirmed') return '条件已确认'
  if (a.status === 'observing') return '已触价，继续观察'
  if (a.status === 'invalidated') return '条件未满足'
  return '等待有效行情'
}
