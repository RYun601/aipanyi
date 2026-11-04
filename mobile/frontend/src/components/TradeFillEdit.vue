<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { showConfirmDialog, showToast } from 'vant'
import { api, submitTradeDraft } from '../api'

const props = defineProps({
  show: Boolean,
  code: { type: String, required: true },
  baseline: Boolean,
  fill: { type: Object, default: null },
  alertEventId: { type: Number, default: 0 },
  position: { type: Object, default: null },
  legacyCost: { type: Number, default: 0 },
  legacyVolume: { type: Number, default: 0 }
})
const emit = defineEmits(['update:show', 'saved'])
const form = reactive({})
const saving = ref(false)
const uncertain = ref(null)
const title = computed(() => props.baseline ? '核对期初持仓' : props.fill ? '更正成交' : '记录成交')

function localTime(value = Date.now()) {
  const date = new Date(value)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 19)
}

watch(() => props.show, (open) => {
  if (!open || uncertain.value) return
  Object.assign(form, {
    requestId: crypto.randomUUID(),
    side: props.fill?.side || 'buy',
    quantity: props.baseline ? String(props.position?.quantity ?? props.legacyVolume) : String(props.fill?.quantity || ''),
    price: props.baseline ? String(props.position?.averageCost ?? Number(props.legacyCost || 0).toFixed(4)) : props.fill?.price || '',
    fee: props.fill?.fee || '',
    zeroFee: false,
    note: props.fill?.note || '',
    at: localTime(props.fill?.at || Date.now())
  })
})

function close() {
  if (!saving.value) emit('update:show', false)
}

function makeRequest() {
  const at = new Date(form.at)
  const quantity = Number(form.quantity)
  if (!form.at || !form.quantity.trim() || !Number.isFinite(at.getTime()) ||
      !Number.isSafeInteger(quantity) || quantity < 0 || !form.price.trim()) {
    throw new Error('请填写有效的股数、价格和时间')
  }
  const base = { requestId: form.requestId, code: props.code }
  if (props.baseline) {
    return { kind: 'baseline', input: { ...base, at: at.toISOString(), quantity, unitCost: form.price } }
  }
  if (!form.fee.trim() && !form.zeroFee) throw new Error('请填写费用，或确认费用为 0')
  const fill = {
    ...base, at: at.toISOString(), quantity, price: form.price, fee: form.fee || '0',
    side: form.side, note: form.note, alertEventId: props.fill?.alertEventId || props.alertEventId || 0
  }
  return props.fill ? { kind: 'correction', input: { ...base, targetId: props.fill.id, replacement: fill } }
    : { kind: 'fill', input: fill }
}

const calls = {
  baseline: api.previewTradeBaseline,
  fill: api.previewTrade,
  correction: api.previewTradeCorrection
}

async function save() {
  if (saving.value) return
  saving.value = true
  let submitted = false
  let request
  try {
    request = uncertain.value
    if (!request) {
      request = makeRequest()
      const preview = await calls[request.kind](request.input)
      const before = props.position?.quantity ?? props.legacyVolume
      try {
        await showConfirmDialog({
          title: title.value,
          message: '股数 ' + before + ' → ' + preview.quantity +
            '\n剩余持仓均价 ' + preview.averageCost + '\n每股净投入 ' + preview.netCost,
          confirmButtonText: '确认入账'
        })
      } catch {
        return
      }
      request.input.expectedRevision = preview.revision - 1
    }
    submitted = true
    const result = await submitTradeDraft(request)
    uncertain.value = null
    emit('saved', result)
    emit('update:show', false)
    showToast('已入账')
  } catch (error) {
    if (submitted && (!error.status || error.status >= 500)) {
      uncertain.value = request
      showToast('结果未确认，请用同一笔请求重试')
    } else {
      uncertain.value = null
      showToast(error.message || '提交失败')
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <van-popup :show="show" position="bottom" round :close-on-click-overlay="!saving && !uncertain"
    @update:show="(open) => { if (!open && !uncertain) close() }">
    <div class="sheet">
      <div class="sheet-title">{{ title }}</div>
      <p v-if="baseline" class="muted">期初应早于首笔成交。新增对账期初会开启新一段账本，旧记录可在历史中查看。</p>
      <p v-else-if="fill" class="muted">原记录将保留，并关联本次更正。</p>
      <p v-else-if="alertEventId" class="muted">关联提醒 #{{ alertEventId }}</p>
      <van-cell-group :aria-busy="saving">
        <van-field v-if="!baseline" label="方向" :disabled="saving || !!uncertain">
          <template #input>
            <van-radio-group v-model="form.side" direction="horizontal" :disabled="saving || !!uncertain">
              <van-radio name="buy">买入</van-radio>
              <van-radio name="sell">卖出</van-radio>
            </van-radio-group>
          </template>
        </van-field>
        <van-field v-model="form.quantity" type="digit" label="股数" :disabled="saving || !!uncertain" />
        <van-field v-model="form.price" type="number" :label="baseline ? '单位成本' : '成交价格'" :disabled="saving || !!uncertain" />
        <van-field :label="baseline ? '期初时间' : '成交时间'">
          <template #input>
            <input v-model="form.at" type="datetime-local" step="1" class="trade-time" :disabled="saving || !!uncertain" />
          </template>
        </van-field>
        <template v-if="!baseline">
          <van-field v-model="form.fee" type="number" label="费用" :disabled="saving || !!uncertain" />
          <van-checkbox v-if="!form.fee" v-model="form.zeroFee" style="padding: 8px 16px" :disabled="saving || !!uncertain">费用按 0 记</van-checkbox>
          <van-field v-model="form.note" label="备注" placeholder="可选" maxlength="200" :disabled="saving || !!uncertain" />
        </template>
      </van-cell-group>
      <div class="row" style="margin-top: 12px">
        <van-button plain round block :disabled="saving || !!uncertain" @click="close">取消</van-button>
        <van-button type="primary" round block :loading="saving" @click="save">{{ uncertain ? '重试提交' : '预览并确认' }}</van-button>
      </div>
    </div>
  </van-popup>
</template>

<style scoped>
.trade-time { max-width: 100%; color: inherit; font: inherit; background: transparent; border: 0; }
</style>
