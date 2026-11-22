<script setup>
import { reactive, watch } from 'vue'
import { showToast } from 'vant'
import { api } from '../api'
import { directionText } from '../format'
import AlertStrategyFields from './AlertStrategyFields.vue'

const props = defineProps({
  show: { type: Boolean, default: false },
  code: { type: String, default: '' },
  name: { type: String, default: '' },
  suggestions: { type: Array, default: () => [] }
})
const emit = defineEmits(['update:show', 'saved'])

const state = reactive({ items: [], saving: false, notice: '' })

function canEnableSuggestion(item) {
  const price = Number(item.price)
  const ratio = Number(item.minVolumeRatio)
  const percent = Number(item.bookPercent)
  return item.mode === 'strategy' && item.phase === 'intraday' &&
    Number.isFinite(price) && price > 0 &&
    Number.isFinite(ratio) && ratio > 0 &&
    (item.bookSide === 'buy' || item.bookSide === 'sell') &&
    percent > 0 && percent <= 100
}

watch(
  () => props.show,
  (v) => {
    if (!v) return
    state.notice = ''
    state.items = (props.suggestions || []).map((s) => {
      const basisRevision = Number(s.positionRevision)
      const item = {
        ...s,
        selected: !!s.valid,
        price: String(s.price ?? ''),
        note: s.note || '',
        mode: s.mode || 'strategy',
        phase: s.phase || 'intraday',
        minVolumeRatio: s.minVolumeRatio ? String(s.minVolumeRatio) : '',
        bookPercent: s.minBookBias ? String(s.minBookBias * 100) : '',
        bookSide: s.bookSide || '',
        // 0 表示分析时还没有期初持仓账本，不能当作可绑定的持仓版本。
        positionRevision: Number.isFinite(basisRevision) && basisRevision > 0 ? basisRevision : null
      }
      item.enabled = canEnableSuggestion(item)
      return item
    })
  }
)

function close() {
  emit('update:show', false)
}

async function confirm() {
  const selected = state.items.filter((x) => x.valid && x.selected)
  if (!selected.length) {
    showToast('请至少选择一条')
    return
  }
  if (state.saving) return
  state.saving = true
  state.notice = ''
  try {
    const results = await api.saveAlertSuggestions(selected.map((item) => ({
      code: props.code,
      name: props.name,
      direction: item.direction,
      price: Number(item.price) || 0,
      note: item.note,
      enabled: item.mode === 'price' || (item.enabled && canEnableSuggestion(item)),
      mode: item.mode,
      phase: item.phase,
      minVolumeRatio: Number(item.minVolumeRatio) || 0,
      bookSide: item.bookSide,
      minBookBias: (Number(item.bookPercent) || 0) / 100,
      usePosition: item.mode === 'strategy' && item.positionRevision != null,
      positionRevision: item.positionRevision ?? null,
      source: 'ai'
    })))
    let saved = 0
    let savedPaused = 0
    let resolved = 0
    for (const result of results) {
      const item = selected[result.index]
      item.done = result.status === 'saved' || result.status === 'duplicate'
      item.result = result.message || (result.status === 'saved' ? '已添加' : '')
      if (result.status === 'saved') {
        saved++
        if (result.message) savedPaused++
      }
      if (item.done) {
        item.selected = false
        resolved++
      }
    }
    if (resolved) emit('saved')
    if (resolved === selected.length) {
      const summary = saved ? '已替换为 ' + saved + ' 条提醒' : '所选提醒已存在'
      showToast(savedPaused ? summary + '，' + savedPaused + ' 条因行情不足暂未启用' : summary)
      close()
    } else {
      // 失败原因已经逐条显示在对应条目上；这里只给汇总，避免 toast 盖住正在修改的表单。
      state.notice = selected.filter((item) => !item.done).length + ' 条提醒未保存，请按上方提示修改后重试'
    }
  } catch (e) {
    emit('saved')
    showToast(e.message || '提交结果未确认，请重试')
  } finally {
    state.saving = false
  }
}
</script>

<template>
  <van-popup :show="show" position="bottom" round :close-on-click-overlay="!state.saving" @update:show="emit('update:show', $event)">
    <div class="sheet">
      <div class="sheet-title">AI 建议的提醒</div>
      <div class="muted" style="margin-bottom: 12px">
        确认添加后，所选提醒会替换这只股票的全部旧提醒，包括手动创建的提醒。收盘规则会保存为待复核。
      </div>

      <div v-if="!state.items.length" class="muted" style="padding: 12px 0">
        本次分析没有识别到明确价格。
      </div>

      <div v-for="(item, idx) in state.items" :key="idx" class="card" :style="item.valid ? '' : 'opacity:.55'">
        <div class="row">
          <van-checkbox v-model="item.selected" :disabled="!item.valid || item.done || state.saving">
            <span class="small">
              {{ directionText(item.direction) }}
              <b>{{ item.price }}</b>
            </span>
          </van-checkbox>
          <span class="tag-mini">{{ item.result || (item.reason || (item.valid ? '可添加' : '无效')) }}</span>
        </div>
        <van-field
          v-if="item.valid"
          v-model="item.price"
          :disabled="item.done || state.saving"
          type="number"
          label="价格"
          input-align="right"
          style="margin-top: 8px"
        />
        <fieldset v-if="item.valid && item.selected && !item.done" :disabled="state.saving" class="strategy-fields">
          <AlertStrategyFields :form="item" />
          <div v-if="item.mode === 'strategy' && item.positionRevision != null" class="muted">依据分析时的持仓版本</div>
          <template v-if="item.mode === 'strategy' && item.phase === 'intraday'">
            <van-checkbox
              :model-value="item.enabled && canEnableSuggestion(item)"
              :disabled="state.saving || !canEnableSuggestion(item)"
              @update:model-value="item.enabled = $event"
            >立即启用</van-checkbox>
            <div v-if="!canEnableSuggestion(item)" class="muted">补齐条件后可启用，也可先保存为待复核。</div>
          </template>
        </fieldset>
        <div v-if="item.valid" class="suggestion-note">
          <div class="muted suggestion-note-label">备注</div>
          <van-field
            v-model="item.note"
            :disabled="item.done || state.saving"
            type="textarea"
            rows="2"
            autosize
            placeholder="可编辑备注"
          />
        </div>
      </div>

      <div v-if="state.notice" class="save-notice">{{ state.notice }}</div>

      <div class="row" style="margin-top: 12px">
        <van-button plain block round :disabled="state.saving" @click="close">取消</van-button>
        <van-button type="primary" block round :loading="state.saving" @click="confirm">确认添加</van-button>
      </div>
    </div>
  </van-popup>
</template>

<style scoped>
.strategy-fields { border: 0; margin: 0; padding: 0; min-width: 0; }
.suggestion-note {
  margin-top: 8px;
}
.suggestion-note-label {
  margin-bottom: 4px;
  font-size: 12px;
}
.save-notice {
  margin-top: 10px;
  font-size: 13px;
  color: #ee0a24;
}
</style>
