import assert from 'node:assert/strict'
import { test } from 'node:test'
import { calculateTradeFee } from './tradeFee.js'

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
