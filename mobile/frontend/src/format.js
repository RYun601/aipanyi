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
