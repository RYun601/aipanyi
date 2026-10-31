<script setup>
import { reactive, watch } from 'vue'
import { showToast } from 'vant'
import { api } from '../api'

const props = defineProps({
  show: { type: Boolean, default: false },
  stock: { type: Object, default: null },
  alert: { type: Object, default: null }
})
const emit = defineEmits(['update:show', 'saved'])

const form = reactive({
  direction: 'below',
  price: '',
  note: '',
  enabled: true
})

watch(
  () => props.show,
  (v) => {
    if (!v) return
    const a = props.alert
    form.direction = a?.direction || 'below'
    form.price = a?.price ? String(a.price) : ''
    form.note = a?.note || ''
    form.enabled = a ? !!a.enabled : true
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
  try {
    await api.saveAlert({
      id: props.alert?.id || 0,
      code: props.stock.code,
      name: props.stock.name || '',
      direction: form.direction,
      price,
      note: form.note,
      enabled: form.enabled,
      source: props.alert?.source || 'manual'
    })
    showToast('已保存')
    emit('saved')
    close()
  } catch (e) {
    showToast(e?.message || '保存失败')
  }
}
</script>

<template>
  <van-popup :show="show" position="bottom" round @update:show="emit('update:show', $event)">
    <div class="sheet">
      <div class="sheet-title">{{ alert ? '编辑价格提醒' : '添加价格提醒' }}</div>
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
      </van-cell-group>

      <div class="hint" style="margin-bottom: 12px">
        交易时段每 30 秒检查一次。同一方向只提醒一次，价格回到另一侧后会重新生效。
        离开 App 后仍会检查，但 Honor 需允许自启动/后台活动。
      </div>

      <van-button type="primary" block round @click="save">保存</van-button>
    </div>
  </van-popup>
</template>
