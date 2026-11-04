import assert from 'node:assert/strict'
import { test } from 'node:test'
import { calculateTradeFee, estimateTradeFee } from './tradeFee.js'

test('calculates a percentage of the fill value and rounds half a cent up', () => {
  assert.equal(calculateTradeFee('100', '12.34', '0.03'), '0.37')
  assert.equal(calculateTradeFee('50', '10', '0.001'), '0.01')
  assert.equal(calculateTradeFee('100', '12.34', '0'), '0.00')
})

test('leaves the fee blank until the rate and fill inputs are valid', () => {
  assert.equal(calculateTradeFee('100', '12.34', ''), '')
  assert.equal(calculateTradeFee('0', '12.34', '0.03'), '')
  assert.equal(calculateTradeFee('100', '12.34567', '0.03'), '')
  assert.equal(calculateTradeFee('100', '12.34', '100.0001'), '')
})

test('selects the rule by side and rounds each A-share fee separately', () => {
  const trade = { code: 'sz000001', quantity: '100', price: '25', at: '2025-11-04T11:00:00' }
  const rules = {
    buy: { mode: 'fixed', rate: '0.02' },
    sell: { mode: 'aShare', commission: '0.853', minimum: '0.50' }
  }
  assert.deepEqual(estimateTradeFee({ ...trade, side: 'buy' }, rules), { total: '0.50' })
  assert.deepEqual(estimateTradeFee({ ...trade, side: 'sell' }, rules),
    { commission: '0.50', stampDuty: '1.25', transfer: '0.03', total: '1.78' })
  rules.buy = { ...rules.sell, minimum: '0.40' }
  assert.deepEqual(estimateTradeFee({ ...trade, side: 'buy' }, rules),
    { commission: '0.40', stampDuty: '0.00', transfer: '0.03', total: '0.43' })
  assert.equal(estimateTradeFee({ ...trade, code: 'sh510300', side: 'sell' }, rules), null)
  assert.equal(estimateTradeFee({ ...trade, at: '2023-08-27T11:00:00', side: 'sell' }, rules), null)
})
