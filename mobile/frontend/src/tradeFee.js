const scale = 10_000n

function fixedDecimal(value) {
  const text = String(value ?? '').trim()
  if (!/^\d+(?:\.\d{1,4})?$/.test(text)) return null
  const [whole, fraction = ''] = text.split('.')
  return BigInt(whole) * scale + BigInt(fraction.padEnd(4, '0'))
}

export function calculateTradeFee(quantity, price, percentage) {
  const shares = Number(quantity)
  const unitPrice = fixedDecimal(price)
  const rate = fixedDecimal(percentage)
  if (!Number.isSafeInteger(shares) || shares <= 0 || unitPrice === null || unitPrice <= 0n ||
      rate === null || rate > 100n * scale) return ''

  // Converting the percent to a ratio and the result to cents cancels the two factors of 100.
  const denominator = scale * scale
  const cents = (BigInt(shares) * unitPrice * rate + denominator / 2n) / denominator
  return String(cents / 100n) + '.' + String(cents % 100n).padStart(2, '0')
}
