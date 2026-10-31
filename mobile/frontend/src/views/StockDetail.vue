<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import { api } from '../api'
import { onWS } from '../ws'
import { changeClass, directionText, fmtPercent, fmtPrice, fmtSignedMoney, timeShort } from '../format'
import MarkdownView from '../components/MarkdownView.vue'
import AlertEdit from '../components/AlertEdit.vue'
import AlertSuggestions from '../components/AlertSuggestions.vue'

const props = defineProps({ code: { type: String, required: true } })
const router = useRouter()

const detail = ref(null)
const loading = ref(true)
const running = ref(false)
const draft = ref('')
const reasoning = ref('')
const cost = ref('')
const volume = ref('')
const showPosition = ref(false)
const showAlertEdit = ref(false)
const editingAlert = ref(null)
const showSuggestions = ref(false)
const suggestions = ref([])
let offAnalysis = null
let offSuggestions = null

const item = computed(() => detail.value?.item || {})
const alerts = computed(() => detail.value?.alerts || [])
const savedAnalysis = computed(() => detail.value?.analysis || null)
const displayAnalysis = computed(() => draft.value || savedAnalysis.value?.content || '')
const canExtract = computed(() => !running.value && !!displayAnalysis.value.trim())

async function load() {
  loading.value = true
  try {
    detail.value = await api.getStockDetail(props.code)
    const it = detail.value?.item || {}
    cost.value = it.costPrice ? String(it.costPrice) : ''
    volume.value = it.volume ? String(it.volume) : ''
  } catch (e) {
    showToast(e?.message || '加载个股失败')
  } finally {
    loading.value = false
  }
}

function onAnalysis(payload) {
  if (!payload || payload.code !== props.code) return
  if (payload.delta) {
    draft.value += payload.delta
    running.value = true
  }
  if (payload.reasoning) reasoning.value += payload.reasoning
  if (payload.done) {
    running.value = false
    if (payload.error) showToast(payload.error)
    if (payload.canceled) showToast('已取消分析')
    if (payload.saved) load()
  }
}

async function startAnalysis() {
  if (running.value) return
  running.value = true
  draft.value = ''
  reasoning.value = ''
  try {
    await api.startAnalysis(props.code)
    showToast('AI 分析已开始')
  } catch (e) {
    running.value = false
    showToast(e?.message || '启动分析失败')
  }
}

async function cancelAnalysis() {
  try {
    await api.cancelAnalysis()
  } catch (e) {
    showToast(e?.message || '取消失败')
  }
}

function openPosition() {
  const it = item.value
  cost.value = it.costPrice ? String(it.costPrice) : ''
  volume.value = it.volume ? String(it.volume) : ''
  showPosition.value = true
}

async function savePosition() {
  const c = Number(cost.value)
  const v = Number(volume.value)
  if (!Number.isFinite(c) || c < 0 || !Number.isFinite(v) || v < 0) {
    showToast('请输入正确的成本价和持仓数量')
    return
  }
  try {
    const msg = await api.setPosition(props.code, c, v)
    showToast(msg || '已保存')
    showPosition.value = false
    await load()
  } catch (e) {
    showToast(e?.message || '保存失败')
  }
}

function openAddAlert() {
  editingAlert.value = null
  showAlertEdit.value = true
}

function openEditAlert(a) {
  editingAlert.value = a
  showAlertEdit.value = true
}

async function toggleAlert(a, enabled) {
  const old = a.enabled
  a.enabled = enabled
  try {
    await api.setAlertEnabled(a.id, enabled)
  } catch (e) {
    a.enabled = old
    showToast(e?.message || '设置失败')
  }
}

async function removeAlert(a) {
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

async function extractAlerts() {
  try {
    const data = await api.extractAlerts(props.code)
    const list = Array.isArray(data) ? data : []
    if (!list.length) {
      showToast('本次分析没有明确价格')
      return
    }
    suggestions.value = list
    showSuggestions.value = true
  } catch (e) {
    showToast(e?.message || '提取提醒失败')
  }
}

onMounted(() => {
  load()
  offAnalysis = onWS('mobileAnalysis', onAnalysis)
  offSuggestions = onWS('mobileAlertSuggestions', (payload) => {
    if (!payload || payload.code !== props.code) return
    if (payload.error) {
      showToast(payload.error)
      return
    }
    suggestions.value = payload.suggestions || []
    if (suggestions.value.length) showSuggestions.value = true
  })
})

onBeforeUnmount(() => {
  offAnalysis && offAnalysis()
  offSuggestions && offSuggestions()
})

watch(
  () => props.code,
  () => {
    draft.value = ''
    reasoning.value = ''
    load()
  }
)
</script>

<template>
  <div>
    <van-nav-bar
      :title="item.name || code"
      left-arrow
      fixed
      placeholder
      @click-left="router.back()"
    />

    <div class="page">
      <van-loading v-if="loading && !detail" style="padding: 80px 0" vertical>加载中</van-loading>

      <template v-else>
        <div class="card">
          <div class="row">
            <div>
              <div class="muted mono">{{ item.code || code }}</div>
              <div class="price-big" :class="changeClass(item.changePercent)">{{ fmtPrice(item.price) }}</div>
              <div class="small" :class="changeClass(item.changePercent)">
                {{ item.changeAmount >= 0 ? '+' : '' }}{{ fmtPrice(item.changeAmount) }} / {{ fmtPercent(item.changePercent) }}
              </div>
            </div>
            <div style="text-align: right">
              <div class="muted">成本 / 持仓</div>
              <div class="price-mid">
                {{ item.costPrice ? fmtPrice(item.costPrice) : '--' }}
                <span class="muted" style="font-size: 13px">/ {{ item.volume || 0 }}</span>
              </div>
              <div v-if="item.costPrice > 0 && item.volume > 0" class="small" :class="changeClass(item.profitAmount)">
                {{ fmtSignedMoney(item.profitAmount) }} ({{ fmtPercent(item.profitPercent) }})
              </div>
              <van-button size="small" plain type="primary" style="margin-top: 6px" @click="openPosition">
                设置成本
              </van-button>
            </div>
          </div>
          <div v-if="item.quoteTime" class="muted" style="margin-top: 8px">行情时间 {{ item.quoteTime }}</div>
        </div>

        <div class="card">
          <div class="row">
            <div>
              <div style="font-size: 17px; font-weight: 650">AI 分析</div>
              <div class="muted">
                <span v-if="running">正在分析，请稍候…</span>
                <span v-else-if="savedAnalysis">上次 {{ savedAnalysis.createdAt }} · {{ savedAnalysis.modelName || 'AI' }}</span>
                <span v-else>会带上成本价与持仓数量</span>
              </div>
            </div>
            <div class="row" style="gap: 6px">
              <van-button v-if="running" size="small" plain type="danger" @click="cancelAnalysis">取消</van-button>
              <van-button v-else size="small" type="primary" @click="startAnalysis">开始分析</van-button>
            </div>
          </div>

          <div v-if="running && !displayAnalysis" style="padding: 18px 0">
            <van-loading size="22">AI 正在读取行情与策略…</van-loading>
          </div>

          <template v-if="displayAnalysis">
            <div class="divider"></div>
            <MarkdownView :content="displayAnalysis" />
            <details v-if="reasoning" style="margin-top: 10px">
              <summary class="muted">思考过程</summary>
              <div class="muted" style="white-space: pre-wrap; margin-top: 6px">{{ reasoning }}</div>
            </details>
            <div class="row" style="margin-top: 12px">
              <van-button size="small" plain type="primary" :disabled="!canExtract" @click="extractAlerts">
                从分析中提取提醒
              </van-button>
            </div>
          </template>
        </div>

        <div class="card">
          <div class="row">
            <div>
              <div style="font-size: 17px; font-weight: 650">价格提醒</div>
              <div class="muted">交易时段每 30 秒检查一次</div>
            </div>
            <van-button size="small" plain type="primary" @click="openAddAlert">添加</van-button>
          </div>

          <div v-if="!alerts.length" class="muted" style="padding: 14px 0">
            还没有提醒。可以手动添加，也可以让 AI 从分析里提取。
          </div>

          <div v-for="a in alerts" :key="a.id" style="padding: 10px 0; border-bottom: 1px solid var(--ap-line)">
            <div class="row">
              <div>
                <div>
                  <b>{{ directionText(a.direction) }} {{ fmtPrice(a.price) }}</b>
                  <span v-if="a.source === 'ai'" class="tag-mini" style="margin-left: 6px">AI</span>
                </div>
                <div v-if="a.note" class="muted">{{ a.note }}</div>
                <div class="muted">
                  触发 {{ a.triggerCount }} 次
                  <span v-if="a.lastTriggeredAt"> · 最近 {{ timeShort(a.lastTriggeredAt) }}</span>
                </div>
              </div>
              <div class="row" style="gap: 8px">
                <van-switch :model-value="a.enabled" size="20px" @update:model-value="(v) => toggleAlert(a, v)" />
                <van-icon name="edit" size="18" @click="openEditAlert(a)" />
                <van-icon name="delete-o" size="18" color="#e5484d" @click="removeAlert(a)" />
              </div>
            </div>
          </div>
        </div>

        <div class="hint">
          提醒依赖本地后台服务。若手机清理了 App，通知会停止；请在系统设置里允许自启动、后台活动，并关闭电池优化。
        </div>
      </template>
    </div>

    <van-popup v-model:show="showPosition" position="bottom" round>
      <div class="sheet">
        <div class="sheet-title">成本与持仓</div>
        <div class="muted" style="margin-bottom: 12px">
          AI 分析会自动读取这里的成本价和持仓数量。
        </div>
        <van-cell-group inset style="margin: 0 0 12px">
          <van-field v-model="cost" type="number" label="成本价" placeholder="例如 12.30" input-align="right" />
          <van-field v-model="volume" type="number" label="持仓(股)" placeholder="例如 1000" input-align="right" />
        </van-cell-group>
        <van-button type="primary" block round @click="savePosition">保存</van-button>
      </div>
    </van-popup>

    <AlertEdit
      v-model:show="showAlertEdit"
      :stock="{ code: item.code || code, name: item.name || '' }"
      :alert="editingAlert"
      @saved="load"
    />

    <AlertSuggestions
      v-model:show="showSuggestions"
      :code="item.code || code"
      :name="item.name || ''"
      :suggestions="suggestions"
      @saved="load"
    />
  </div>
</template>
