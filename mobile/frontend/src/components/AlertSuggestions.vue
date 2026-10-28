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

const state = reactive({ items: [] })

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
  let ok = 0
  for (const item of selected) {
    const price = Number(item.price)
    if (!Number.isFinite(price) || price <= 0) continue
    try {
      await api.saveAlert({
        id: 0,
        code: props.code,
        name: props.name,
        direction: item.direction,
        price,
        note: item.note,
        enabled: true,
        source: 'ai'
      })
      ok++
    } catch (e) {
      // 单条失败不阻塞其它条目，最后统一提示
      console.warn('save alert failed', e)
    }
  }
  if (ok > 0) {
    showToast(`已添加 ${ok} 条提醒`)
    emit('saved')
    close()
  } else {
    showToast('添加失败，请检查价格')
  }
}
</script>

<template>
  <van-popup :show="show" position="bottom" round @update:show="emit('update:show', $event)">
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
          <van-checkbox v-model="item.selected" :disabled="!item.valid">
            <span class="small">
              {{ directionText(item.direction) }}
              <b>{{ item.price }}</b>
            </span>
          </van-checkbox>
          <span class="tag-mini">{{ item.valid ? '可添加' : item.reason || '无效' }}</span>
        </div>
        <van-field
          v-if="item.valid"
          v-model="item.price"
          type="number"
          label="价格"
          input-align="right"
          style="margin-top: 8px"
        />
        <van-field v-if="item.valid" v-model="item.note" label="备注" input-align="right" />
      </div>

      <div class="row" style="margin-top: 12px">
        <van-button plain block round @click="close">取消</van-button>
        <van-button type="primary" block round @click="confirm">确认添加</van-button>
      </div>
    </div>
  </van-popup>
</template>
