<script setup>
import { reactive, watch } from 'vue'
import { showToast } from 'vant'
import { api } from '../api'
import AlertStrategyFields from './AlertStrategyFields.vue'

const props = defineProps({
  show: { type: Boolean, default: false },
  hasLedger: { type: Boolean, default: false },
  stock: { type: Object, default: null },
  alert: { type: Object, default: null }
})
const emit = defineEmits(['update:show', 'saved'])

const form = reactive({
  direction: 'below',
  price: '',
  note: '',
  enabled: true,
  mode: 'price',
  phase: 'intraday',
  minVolumeRatio: '',
  bookSide: '',
  bookPercent: '',
  usePosition: false,
  saving: false
})

watch(
  () => props.show,
  (v) => {
    if (!v) return
    const a = props.alert
    form.direction = a?.direction || 'below'
    form.price = a?.price ? String(a.price) : ''
    form.note = a?.note || ''
    form.enabled = a?.needsReview ? true : a ? !!a.enabled : true
    form.mode = a?.mode || 'price'
    form.phase = a?.phase || 'intraday'
    form.minVolumeRatio = a?.minVolumeRatio ? String(a.minVolumeRatio) : ''
    form.bookSide = a?.bookSide || ''
    form.bookPercent = a?.minBookBias ? String(a.minBookBias * 100) : ''
    form.usePosition = !!a?.usePosition
  }
)

function close() {
  emit('update:show', false)
}

async function save() {
  if (!props.stock?.code) {
    showToast('缺少股票信息')
    return
  }
  const price = Number(form.price)
  if (!Number.isFinite(price) || price <= 0) {
    showToast('请输入正确的价格')
    return
  }
  if (form.saving) return
  form.saving = true
  try {
    await api.saveAlert({
      id: props.alert?.id || 0,
      code: props.stock.code,
      name: props.stock.name || '',
      direction: form.direction,
      price,
      note: form.note,
      enabled: form.enabled,
      mode: form.mode,
      phase: form.phase,
      minVolumeRatio: Number(form.minVolumeRatio) || 0,
      bookSide: form.bookSide,
      minBookBias: (Number(form.bookPercent) || 0) / 100,
      usePosition: form.mode === 'strategy' && form.usePosition,
      source: props.alert?.source || 'manual'
    })
    showToast('已保存')
    emit('saved')
    close()
  } catch (e) {
    showToast(e?.message || '保存失败')
  } finally {
    form.saving = false
  }
}
</script>

<template>
  <van-popup :show="show" position="bottom" round @update:show="emit('update:show', $event)">
    <div class="sheet">
      <div class="sheet-title">{{ alert ? '编辑提醒' : '添加提醒' }}</div>
      <div class="muted" style="margin-bottom: 12px">
        {{ stock?.name }} {{ stock?.code }}
      </div>

      <van-cell-group inset style="margin: 0 0 12px">
        <van-field label="方向">
          <template #input>
            <van-radio-group v-model="form.direction" direction="horizontal">
              <van-radio name="below">跌破</van-radio>
              <van-radio name="above">涨到</van-radio>
            </van-radio-group>
          </template>
        </van-field>
        <van-field
          v-model="form.price"
          type="number"
          label="价格"
          placeholder="例如 12.30"
          input-align="right"
        />
        <van-field v-model="form.note" label="备注" placeholder="可选，例如：跌破支撑减仓" input-align="right" />
        <AlertStrategyFields :form="form" />
        <van-field v-if="form.mode === 'strategy' && (hasLedger || form.usePosition)" label="依据当前持仓">
          <template #input><van-switch v-model="form.usePosition" size="20" /></template>
        </van-field>
        <van-field v-if="form.mode === 'price' || form.phase !== 'close'" label="保存后启用">
          <template #input><van-switch v-model="form.enabled" size="20" /></template>
        </van-field>
      </van-cell-group>

      <van-button type="primary" block round :loading="form.saving" @click="save">保存</van-button>
    </div>
  </van-popup>
</template>
