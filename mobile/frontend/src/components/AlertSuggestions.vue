<script setup>
import { reactive, watch } from 'vue'
import { showToast } from 'vant'
import { api } from '../api'
import { directionText } from '../format'

const props = defineProps({
  show: { type: Boolean, default: false },
  code: { type: String, default: '' },
  name: { type: String, default: '' },
  suggestions: { type: Array, default: () => [] }
})
const emit = defineEmits(['update:show', 'saved'])

const state = reactive({ items: [], saving: false })

watch(
  () => props.show,
  (v) => {
    if (!v) return
    state.items = (props.suggestions || []).map((s) => ({
      ...s,
      selected: !!s.valid,
      price: String(s.price ?? ''),
      note: s.note || ''
    }))
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
  try {
    const results = await api.saveAlertSuggestions(selected.map((item) => ({
      code: props.code,
      name: props.name,
      direction: item.direction,
      price: Number(item.price) || 0,
      note: item.note,
      enabled: true,
      source: 'ai'
    })))
    let saved = 0
    let resolved = 0
    for (const result of results) {
      const item = selected[result.index]
      item.done = result.status === 'saved' || result.status === 'duplicate'
      item.result = result.status === 'saved' ? '已添加' : result.message
      if (result.status === 'saved') saved++
      if (item.done) {
        item.selected = false
        resolved++
      }
    }
    if (resolved) emit('saved')
    if (resolved === selected.length) {
      showToast(saved ? '已添加 ' + saved + ' 条提醒' : '所选提醒已存在')
      close()
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
      <div class="sheet-title">AI 建议的价格提醒</div>
      <div class="muted" style="margin-bottom: 12px">
        只保留分析中明确给出的价格；确认后才会生效。
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
          <span class="tag-mini">{{ item.result || (item.valid ? '可添加' : item.reason || '无效') }}</span>
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

      <div class="row" style="margin-top: 12px">
        <van-button plain block round :disabled="state.saving" @click="close">取消</van-button>
        <van-button type="primary" block round :loading="state.saving" @click="confirm">确认添加</van-button>
      </div>
    </div>
  </van-popup>
</template>

<style scoped>
.suggestion-note {
  margin-top: 8px;
}
.suggestion-note-label {
  margin-bottom: 4px;
  font-size: 12px;
}
</style>
