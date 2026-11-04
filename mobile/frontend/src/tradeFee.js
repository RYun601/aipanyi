const scale = 10_000n

function fixedDecimal(value) {
  const text = String(value ?? '').trim()
  if (!/^\d+(?:\.\d{1,4})?$/.test(text)) return null
  const [whole, fraction = ''] = text.split('.')
  return BigInt(whole) * scale + BigInt(fraction.padEnd(4, '0'))
}

function notional(quantity, price) {
  const shares = Number(quantity)
  const unitPrice = fixedDecimal(price)
  if (!Number.isSafeInteger(shares) || shares <= 0 || unitPrice === null || unitPrice <= 0n) return null
  return BigInt(shares) * unitPrice
}

function roundedCents(units, rate, denominator) {
  return (units * rate + denominator / 2n) / denominator
}

function asMoney(cents) {
  return String(cents / 100n) + '.' + String(cents % 100n).padStart(2, '0')
}

export function calculateTradeFee(quantity, price, percentage) {
  const units = notional(quantity, price)
  const rate = fixedDecimal(percentage)
  if (units === null || rate === null || rate > 100n * scale) return ''

  // Converting the percent to a ratio and the result to cents cancels the two factors of 100.
  return asMoney(roundedCents(units, rate, scale * scale))
}

export function estimateTradeFee(trade, rules) {
  if (trade.side !== 'buy' && trade.side !== 'sell') return null
  const rule = rules?.[trade.side]
  if (rule?.mode === 'fixed') {
    const total = calculateTradeFee(trade.quantity, trade.price, rule.rate)
    return total === '' ? null : { total }
  }
  const code = trade.code.toLowerCase()
  const aShare = /^(?:(?:sh)?(?:60|68)\d{4}(?:\.sh)?|(?:sz)?(?:00|30)\d{4}(?:\.sz)?)$/.test(code)
  if (rule?.mode !== 'aShare' || !aShare || String(trade.at ?? '').slice(0, 10) < '2023-08-28') return null

  const units = notional(trade.quantity, trade.price)
  const rate = fixedDecimal(rule.commission)
  const minimum = fixedDecimal(rule.minimum)
  if (units === null || rate === null || rate > 10_000n * scale || minimum === null || minimum % 100n !== 0n) return null

  const perTenThousand = scale * scale * 100n
  const nominalCommission = roundedCents(units, rate, perTenThousand)
  const minCents = minimum / 100n
  const commission = nominalCommission < minCents ? minCents : nominalCommission
  const stampDuty = trade.side === 'sell' ? roundedCents(units, 5n * scale, perTenThousand) : 0n
  const transfer = roundedCents(units, scale / 10n, perTenThousand)
  return {
    commission: asMoney(commission),
    stampDuty: asMoney(stampDuty),
    transfer: asMoney(transfer),
    total: asMoney(commission + stampDuty + transfer)
  }
}
