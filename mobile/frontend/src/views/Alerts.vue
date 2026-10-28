<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { showConfirmDialog, showToast } from 'vant'
import { api } from '../api'
import { onWS } from '../ws'
import { directionText, fmtPrice, timeShort } from '../format'
import AlertEdit from '../components/AlertEdit.vue'

const tab = ref('rules')
const alerts = ref([])
const events = ref([])
const watchlist = ref([])
const showPicker = ref(false)
const showEdit = ref(false)
const editStock = ref(null)
const editingAlert = ref(null)
let off = null

async function load() {
  try {
    const [a, e] = await Promise.all([api.listAlerts(''), api.listAlertEvents(100)])
    alerts.value = Array.isArray(a) ? a : []
    events.value = Array.isArray(e) ? e : []
  } catch (err) {
    showToast(err?.message || '加载提醒失败')
  }
}

async function openAdd() {
  try {
    const data = await api.getWatchlist()
    watchlist.value = Array.isArray(data) ? data : []
    if (!watchlist.value.length) {
      showToast('请先在自选页添加股票')
      return
    }
    showPicker.value = true
  } catch (e) {
    showToast(e?.message || '加载自选失败')
  }
}

function pickStock(s) {
  editStock.value = { code: s.code, name: s.name }
  editingAlert.value = null
  showPicker.value = false
  showEdit.value = true
}

function edit(a) {
  editStock.value = { code: a.code, name: a.name }
  editingAlert.value = a
  showEdit.value = true
}

async function toggle(a, v) {
  const old = a.enabled
  a.enabled = v
  try {
    await api.setAlertEnabled(a.id, v)
  } catch (e) {
    a.enabled = old
    showToast(e?.message || '设置失败')
  }
}

async function remove(a) {
  try {
    await showConfirmDialog({
      title: '删除提醒',
      message: `确定删除 ${a.name || a.code} ${directionText(a.direction)} ${a.price} 的提醒吗？`
    })
  } catch {
    return
  }
  try {
    await api.deleteAlert(a.id)
    showToast('已删除')
    await load()
  } catch (e) {
    showToast(e?.message || '删除失败')
  }
}

onMounted(() => {
  load()
  off = onWS('mobileAlertTriggered', () => load())
})

onBeforeUnmount(() => off && off())
</script>

<template>
  <div class="page">
    <div class="row" style="padding: 6px 4px 8px">
      <div>
        <div style="font-size: 22px; font-weight: 700">价格提醒</div>
        <div class="muted">只在交易时段检查；触发一次后价格回到另一侧才重新生效</div>
      </div>
      <van-button size="small" type="primary" round @click="openAdd">添加</van-button>
    </div>

    <van-tabs v-model:active="tab" shrink>
      <van-tab title="规则" name="rules">
        <van-empty v-if="!alerts.length" description="还没有价格提醒" />
        <div v-for="a in alerts" :key="a.id" class="card">
          <div class="row">
            <div>
              <div style="font-weight: 650">
                {{ a.name || a.code }}
                <span class="tag-mini" v-if="a.source === 'ai'">AI</span>
              </div>
              <div class="small" style="margin-top: 4px">
                <b>{{ directionText(a.direction) }} {{ fmtPrice(a.price) }}</b>
                <span class="muted mono" style="margin-left: 6px">{{ a.code }}</span>
              </div>
              <div v-if="a.note" class="muted" style="margin-top: 2px">{{ a.note }}</div>
              <div class="muted" style="margin-top: 2px">
                触发 {{ a.triggerCount }} 次
                <span v-if="a.lastTriggeredAt"> · 最近 {{ timeShort(a.lastTriggeredAt) }}</span>
              </div>
            </div>
            <div class="row" style="gap: 10px">
              <van-switch :model-value="a.enabled" size="22px" @update:model-value="(v) => toggle(a, v)" />
              <van-icon name="edit" size="19" @click="edit(a)" />
              <van-icon name="delete-o" size="19" color="#e5484d" @click="remove(a)" />
            </div>
          </div>
        </div>
      </van-tab>

      <van-tab title="触发记录" name="history">
        <van-empty v-if="!events.length" description="还没有触发记录" />
        <div v-for="e in events" :key="e.id" class="card">
          <div class="row">
            <div>
              <div style="font-weight: 600">{{ e.name || e.code }}</div>
              <div class="muted">{{ directionText(e.direction) }} {{ fmtPrice(e.threshold) }}</div>
            </div>
            <div style="text-align: right">
              <div class="price-mid">{{ fmtPrice(e.price) }}</div>
              <div class="muted">{{ timeShort(e.triggeredAt) }}</div>
            </div>
          </div>
        </div>
      </van-tab>
    </van-tabs>

    <van-popup v-model:show="showPicker" position="bottom" round style="max-height: 70vh">
      <div class="sheet">
        <div class="sheet-title">选择股票</div>
        <div style="max-height: 50vh; overflow-y: auto">
          <div
            v-for="s in watchlist"
            :key="s.code"
            class="row"
            style="padding: 12px 2px; border-bottom: 1px solid var(--ap-line)"
            @click="pickStock(s)"
          >
            <div>
              <div style="font-weight: 600">{{ s.name || s.code }}</div>
              <div class="muted mono">{{ s.code }}</div>
            </div>
            <div class="price-mid" :class="s.changePercent >= 0 ? 'up' : 'down'">{{ fmtPrice(s.price) }}</div>
          </div>
        </div>
      </div>
    </van-popup>

    <AlertEdit v-model:show="showEdit" :stock="editStock" :alert="editingAlert" @saved="load" />
  </div>
</template>
