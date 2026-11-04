<script setup>
import { ref, watch } from 'vue'
import { showConfirmDialog, showToast } from 'vant'
import { api, submitTradeDraft } from '../api'

const props = defineProps({
  show: Boolean,
  code: { type: String, required: true },
  revision: { type: Number, default: 0 },
  highlights: { type: Array, default: () => [] }
})
const emit = defineEmits(['update:show', 'edit', 'edit-baseline', 'saved'])
const records = ref([])
const more = ref(false)
const loading = ref(false)
const voiding = ref(false)
const uncertain = ref(null)
let generation = 0

async function load(reset) {
  const code = props.code
  const version = reset ? ++generation : generation
  if (reset) {
    records.value = []
    more.value = false
  }
  loading.value = true
  try {
    const cursor = records.value.at(-1)?.id || 0
    const rows = await api.listTrades(code, cursor, 30)
    if (version === generation && code === props.code) {
      records.value.push(...rows)
      more.value = rows.length === 30
    }
  } catch (e) {
    if (version === generation) showToast(e.message || '读取成交失败')
  } finally {
    if (version === generation) loading.value = false
  }
}

watch([() => props.show, () => props.revision, () => props.code], ([open]) => {
  if (open) load(true)
})

async function voidFill(fill) {
  if (voiding.value) return
  const code = props.code
  let input = uncertain.value
  voiding.value = true
  try {
    if (!input) {
      input = { code, targetId: fill.id, requestId: crypto.randomUUID(), replacement: null }
      const preview = await api.previewTradeCorrection(input)
      try {
        await showConfirmDialog({
          title: '撤销成交 #' + fill.id,
          message: '撤销后剩余 ' + preview.quantity + ' 股，持仓均价 ' + preview.averageCost +
            '\n每股净投入 ' + preview.netCost,
          confirmButtonText: '确认撤销'
        })
      } catch {
        return
      }
      input.expectedRevision = preview.revision - 1
    }
    uncertain.value = input
    const result = await submitTradeDraft({ kind: "correction", input })
    uncertain.value = null
    emit('saved', result)
    showToast('已撤销')
  } catch (e) {
    if (e.status && e.status < 500) uncertain.value = null
    showToast(uncertain.value ? '结果未确认，请重试原撤销请求' : e.message || '撤销失败')
  } finally {
    voiding.value = false
  }
}

function recordLabel(item) {
  if (item.kind === 'baseline') return '期初持仓'
  if (item.kind === 'void') return '撤销 #' + item.targetId
  const side = item.side === 'buy' ? '买入' : '卖出'
  return item.kind === 'replace' ? '修改 #' + item.targetId + ' · ' + side : side
}
</script>

<template>
  <van-popup :show="show" position="bottom" round :close-on-click-overlay="!voiding && !uncertain"
    @update:show="(value) => { if (!voiding && !uncertain) emit('update:show', value) }">
    <div class="sheet">
      <div class="row">
        <div class="sheet-title">成交记录</div>
        <van-button size="small" plain :disabled="voiding || !!uncertain" @click="emit('update:show', false)">关闭</van-button>
      </div>
      <van-empty v-if="!loading && !records.length" description="暂无成交记录" />
      <div v-for="item in records" :key="item.id" class="card">
        <div class="row">
          <b>#{{ item.id }} {{ recordLabel(item) }}</b>
          <span v-if="item.superseded" class="tag-mini">已修改或撤销</span>
          <span v-else-if="highlights.includes(item.id)" class="tag-mini">策略依据</span>
        </div>
        <div v-if="item.kind === 'baseline'" class="small">{{ item.quantity }} 股 · 单位成本 {{ item.price }} · 期初核对</div>
        <div v-else-if="item.kind !== 'void'" class="small">{{ item.quantity }} 股 × {{ item.price }} · 费用 {{ item.fee }}</div>
        <div class="muted">{{ new Date(item.at || item.recordedAt).toLocaleString() }}</div>
        <div v-if="item.note" class="muted">{{ item.note }}</div>
        <div v-if="item.alertEventId" class="muted">来源提醒 #{{ item.alertEventId }}</div>
        <div v-if="item.canCorrect" style="margin-top: 6px">
          <van-button size="small" plain :disabled="voiding || !!uncertain" @click="emit('edit', item)">修改</van-button>
          <van-button size="small" plain :disabled="voiding || !!uncertain" @click="voidFill(item)">撤销</van-button>
        </div>
        <van-button v-if="item.canAdjustBaseline" size="small" plain :disabled="voiding || !!uncertain" @click="emit('edit-baseline', item)">
          修改期初
        </van-button>
      </div>
      <van-button v-if="uncertain" block type="primary" :loading="voiding" @click="voidFill(null)">重试撤销</van-button>
      <van-button v-if="more || loading" block plain :loading="loading" @click="load(false)">加载更多</van-button>
    </div>
  </van-popup>
</template>
