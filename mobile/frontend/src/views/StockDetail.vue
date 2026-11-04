<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import { api, submitTradeDraft } from '../api'
import { onWS } from '../ws'
import { alertStatus, changeClass, directionText, fmtPercent, fmtPrice, fmtSignedMoney, timeShort } from '../format'
import MarkdownView from '../components/MarkdownView.vue'
import AlertEdit from '../components/AlertEdit.vue'
import AlertSuggestions from '../components/AlertSuggestions.vue'
import TradeFillEdit from '../components/TradeFillEdit.vue'
import TradeHistory from '../components/TradeHistory.vue'

const props = defineProps({ code: { type: String, required: true } })
const router = useRouter()
const route = useRoute()
const alertContext = ref(null)
const alertEventId = computed(() => Number(route.query.eventId) || 0)
const contextText = { confirmed: "条件仍满足", invalidated: "条件已失效", observing: "等待指标确认", waiting: "等待新鲜行情", touched: "历史触价", batch: "同批策略" }

const pendingTrade = ref(null)
const restoringTrade = ref(false)
const tradeStrategy = ref(null)
const tradePosition = computed(() => tradeStrategy.value?.position?.revision ? tradeStrategy.value.position : null)
const showTrade = ref(false)
const showTradeHistory = ref(false)
const editTrade = ref(null)
const editBaseline = ref(false)
const tradeRefreshing = ref(false)
const strategyText = {
  pending: '正在重算',
  quote_pending: '行情待更新',
  rules_pending: '待设定策略条件',
  closed: '已清仓',
  observing: '继续观察',
  waiting_rebuy: '等待回补条件',
  conditions_met: '已有规则条件仍满足',
  outside_session: '等待交易时段'
}
let tradeLoadId = 0
let offTrade = null

const detail = ref(null)
const loading = ref(true)
const running = ref(false)
const draft = ref('')
const reasoning = ref('')
const cost = ref('')
const volume = ref('')
const showPosition = ref(false)
const withPosition = ref(false)
const positionOptions = ref([])
const loadingPositions = ref(false)
const showAlertEdit = ref(false)
const editingAlert = ref(null)
const showSuggestions = ref(false)
const suggestions = ref([])
const showAnalysisSheet = ref(false)
const analysisPrompts = ref([])
const selectedPromptId = ref(0)
const analysisQuestion = ref('')
const loadingAnalysisPrompts = ref(false)
const showPromptPicker = ref(false)
const showPromptPreview = ref(false)
const analysisElsewhere = ref(false)
let offAnalysis = null
let offSuggestions = null
let statusTimer = null

const item = computed(() => detail.value?.item || {})
const alerts = computed(() => detail.value?.alerts || [])
const savedAnalysis = computed(() => detail.value?.analysis || null)
const displayAnalysis = computed(() => draft.value || savedAnalysis.value?.content || '')
const canExtract = computed(() => !running.value && !!displayAnalysis.value.trim())
const currentCode = computed(() => (item.value.code || props.code || '').toLowerCase())
const selectedPositionCodes = computed(() =>
  positionOptions.value.filter((p) => p.selected).map((p) => p.code)
)
const selectedPromptName = computed(() => {
  const selected = analysisPrompts.value.find((p) => Number(p.id) === Number(selectedPromptId.value))
  return selected?.name || '默认（跟随设置）'
})
const promptColumns = computed(() =>
  analysisPrompts.value.map((p) => ({ text: p.name, value: String(p.id) }))
)
const promptPickerValue = computed(() => [String(selectedPromptId.value)])
const previewPrompts = computed(() =>
  analysisPrompts.value.filter((p) => Number(p.id) !== 0 && String(p.content || '').trim() !== '')
)
const allSelected = computed({
  get: () => positionOptions.value.length > 0 && positionOptions.value.every((p) => p.selected),
  set: (v) => {
    positionOptions.value.forEach((p) => {
      p.selected = !!v
    })
  }
})

async function loadAlertContext() {
  const code = props.code
  const id = alertEventId.value
  alertContext.value = null
  if (!id) return
  try {
    const result = await api.getAlertContext(code, id)
    if (code === props.code && id === alertEventId.value) alertContext.value = result
  } catch (e) {
    if (code === props.code && id === alertEventId.value) showToast(e?.message || '读取提醒失败')
  }
}

watch([() => props.code, alertEventId], loadAlertContext, { immediate: true })

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

async function loadTradeDraft() {
  const code = props.code
  try {
    const draft = await api.getTradeDraft(code)
    if (code === props.code) pendingTrade.value = draft
  } catch {
    // Confirmation always checks the persistent draft before sending a new write.
  }
}

async function restoreTradeDraft() {
  if (restoringTrade.value || !pendingTrade.value) return
  restoringTrade.value = true
  try {
    const result = await submitTradeDraft(pendingTrade.value)
    showToast('已核对并恢复上次提交')
    await onTradeSaved(result)
  } catch (error) {
    showToast(error.message || '恢复提交失败')
    await loadTradeDraft()
  } finally {
    restoringTrade.value = false
  }
}

async function loadTradeStrategy(force = false) {
  const code = props.code
  const request = ++tradeLoadId
  if (force) tradeRefreshing.value = true
  try {
    const result = await (force ? api.refreshTradeStrategy(code) : api.getTradeStrategy(code))
    if (code === props.code && request === tradeLoadId) tradeStrategy.value = result
  } catch (e) {
    if (force) showToast(e?.message || '刷新策略失败')
  } finally {
    if (force) tradeRefreshing.value = false
  }
}

function openFill() {
  editTrade.value = null
  editBaseline.value = false
  showTrade.value = true
}

function openBaseline() {
  editTrade.value = null
  editBaseline.value = true
  showTrade.value = true
}

function openTradeCorrection(fill) {
  showTradeHistory.value = false
  editTrade.value = fill
  editBaseline.value = false
  showTrade.value = true
}

function openHistoryBaseline(baseline) {
  showTradeHistory.value = false
  openBaseline()
  editTrade.value = baseline
}

async function onTradeSaved(receipt) {
  const code = props.code
  await Promise.all([loadTradeStrategy(), loadTradeDraft()])
  if (code !== props.code) return
  const current = tradePosition.value || receipt
  if (detail.value?.item) {
    detail.value.item.costPrice = Number(current.averageCost)
    detail.value.item.volume = current.quantity
  }
  load()
}

function stopStatusPolling() {
  if (statusTimer) {
    window.clearInterval(statusTimer)
    statusTimer = null
  }
}

function startStatusPolling() {
  if (statusTimer) return
  statusTimer = window.setInterval(() => {
    syncAnalysisStatus()
  }, 5000)
}

// 页面在分析途中被切走再回来时，本地 running 状态已经丢失；这里以服务端状态为准。
async function syncAnalysisStatus() {
  try {
    const status = await api.getAnalysisStatus()
    const isRunning = !!status?.running
    const activeCode = String(status?.code || '').toLowerCase()
    if (isRunning && activeCode === currentCode.value) {
      running.value = true
      analysisElsewhere.value = false
    } else if (isRunning) {
      running.value = false
      analysisElsewhere.value = true
    } else {
      const wasRunning = running.value
      running.value = false
      analysisElsewhere.value = false
      if (wasRunning) await load()
    }
    if (isRunning) startStatusPolling()
    else stopStatusPolling()
  } catch {
    // 状态查询失败不应该影响详情页本身。
  }
}

function onAnalysis(payload) {
  if (!payload || payload.code !== props.code) return
  analysisElsewhere.value = false
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

async function openAnalysisSheet() {
  if (running.value) return
  if (analysisElsewhere.value) {
    showToast('另一只股票正在分析，请稍候')
    return
  }
  if (withPosition.value && selectedPositionCodes.value.length === 0) {
    showToast('请选择至少一只持仓，或关闭持仓开关')
    return
  }
  analysisQuestion.value = ''
  closePromptOverlays()
  showAnalysisSheet.value = true
  await loadAnalysisPrompts()
}

async function loadAnalysisPrompts() {
  if (analysisPrompts.value.length) return
  loadingAnalysisPrompts.value = true
  try {
    const list = await api.listAnalysisPrompts()
    analysisPrompts.value = Array.isArray(list) ? list : []
    try {
      const settings = await api.getSettings()
      const defaultId = Number(settings?.defaultPromptId) || 0
      selectedPromptId.value = analysisPrompts.value.some((p) => Number(p.id) === defaultId) ? defaultId : 0
    } catch {
      // 读取默认提示词失败不影响选择模板，退回到“跟随设置”。
      selectedPromptId.value = 0
    }
  } catch (e) {
    showToast(e?.message || '读取提示词失败')
    analysisPrompts.value = [
      { id: 0, name: '默认（跟随设置）', content: '', type: 'default' },
      { id: -1, name: '内置：中性分析', content: '', type: 'builtin' },
      { id: -2, name: '内置：持仓分析', content: '', type: 'builtin' }
    ]
    selectedPromptId.value = 0
  } finally {
    loadingAnalysisPrompts.value = false
  }
}

function openPromptPicker() {
  if (loadingAnalysisPrompts.value) return
  showPromptPreview.value = false
  showPromptPicker.value = true
}

function onPromptConfirm(payload) {
  const value = payload?.selectedValues?.[0] ?? payload?.selectedOptions?.[0]?.value
  selectedPromptId.value = Number(value) || 0
  showPromptPicker.value = false
}

function openPromptPreview() {
  showPromptPicker.value = false
  showPromptPreview.value = true
}

function closePromptOverlays() {
  showPromptPicker.value = false
  showPromptPreview.value = false
}

async function confirmStartAnalysis() {
  if (running.value) return
  if (analysisElsewhere.value) {
    showToast('另一只股票正在分析，请稍候')
    return
  }
  if (withPosition.value && selectedPositionCodes.value.length === 0) {
    showToast('请选择至少一只持仓，或关闭持仓开关')
    return
  }
  showAnalysisSheet.value = false
  running.value = true
  draft.value = ''
  reasoning.value = ''
  try {
    await api.startAnalysis(
      props.code,
      withPosition.value,
      withPosition.value ? selectedPositionCodes.value : [],
      selectedPromptId.value,
      analysisQuestion.value
    )
    showToast('AI 分析已开始')
  } catch (e) {
    running.value = false
    showToast(e?.message || '启动分析失败')
  }
}

async function loadPositionOptions() {
  loadingPositions.value = true
  try {
    const list = await api.getWatchlist()
    const positions = (Array.isArray(list) ? list : [])
      .filter((x) => Number(x.costPrice) > 0 && Number(x.volume) > 0)
      .map((x) => ({ ...x, selected: true }))
    positionOptions.value = positions
    if (!positions.length) {
      withPosition.value = false
      showToast('还没有配置成本/持仓的股票')
    }
  } catch (e) {
    withPosition.value = false
    showToast(e?.message || '读取持仓失败')
  } finally {
    loadingPositions.value = false
  }
}

async function onWithPositionChange(v) {
  if (!v) {
    positionOptions.value = []
    return
  }
  await loadPositionOptions()
}

async function cancelAnalysis() {
  try {
    await api.cancelAnalysis()
  } catch (e) {
    showToast(e?.message || '取消失败')
  }
}

function openPosition() {
  if (tradePosition.value) {
    openBaseline()
    return
  }
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
    if (withPosition.value) await loadPositionOptions()
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
  offTrade = onWS('mobileTradeStrategy', (message) => {
    if (message?.code === currentCode.value) loadTradeStrategy()
  })
  loadTradeStrategy()
  loadTradeDraft()
  load()
  syncAnalysisStatus()
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
  stopStatusPolling()
  offAnalysis && offAnalysis()
  offSuggestions && offSuggestions()
  offTrade && offTrade()
  tradeLoadId++
})

watch(
  () => props.code,
  () => {
    stopStatusPolling()
    draft.value = ''
    reasoning.value = ''
    running.value = false
    analysisElsewhere.value = false
    withPosition.value = false
    positionOptions.value = []
    loadingPositions.value = false
    tradeStrategy.value = null
    pendingTrade.value = null
    showTrade.value = false
    showTradeHistory.value = false
    loadTradeStrategy()
    loadTradeDraft()
    load()
    syncAnalysisStatus()
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
        <div v-if="alertContext" class="card">
          <div class="row"><b>提醒依据</b><span class="tag-mini">{{ contextText[alertContext.status] }}</span></div>
          <div v-for="e in alertContext.events" :key="e.id" class="small" style="margin-top: 6px">
            {{ timeShort(e.triggeredAt) }} · {{ directionText(e.direction) }} {{ fmtPrice(e.threshold) }}
            <span class="tag-mini">{{ contextText[e.currentStatus] }}</span>
            <span v-if="e.evidence"> · 量比 {{ e.evidence.volumeRatio }} · 买卖盘 {{ fmtPercent(e.evidence.bookBias * 100) }}</span>
          </div>
        </div>
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
                {{ tradePosition ? "对账期初" : "设置成本" }}
              </van-button>
            </div>
          </div>
          <div v-if="item.quoteTime" class="muted" style="margin-top: 8px">行情时间 {{ item.quoteTime }}</div>
        </div>

        <div v-if="item.followed || tradePosition" class="card">
          <div class="row">
            <b>今日策略</b>
            <van-button v-if="tradePosition && item.followed" size="small" plain :loading="tradeRefreshing" @click="loadTradeStrategy(true)">刷新策略</van-button>
          </div>
          <template v-if="tradePosition">
            <div class="muted" style="margin: 6px 0">
              {{ tradeStrategy.tradingDate }} · {{ strategyText[tradeStrategy.status] }}
              <span v-if="tradeStrategy.quoteAt"> · 行情 {{ new Date(tradeStrategy.quoteAt).toLocaleTimeString() }}</span>
            </div>
            <div class="small">剩余 {{ tradePosition.quantity }} 股 · 均价 {{ tradePosition.averageCost }} · 每股净投入 {{ tradePosition.netCost }}</div>
            <div class="small">本轮净投入 {{ tradePosition.netInvested }} · 本段已实现 {{ tradePosition.realized }}</div>
            <template v-if="tradeStrategy.bought && tradeStrategy.tradeIds?.length">
              <div class="small">今日买入 {{ tradeStrategy.bought.quantity }} 股，均价 {{ tradeStrategy.bought.averagePrice }}，净支出 {{ tradeStrategy.bought.netAmount }}</div>
              <div class="small">今日卖出 {{ tradeStrategy.sold.quantity }} 股，均价 {{ tradeStrategy.sold.averagePrice }}，净回款 {{ tradeStrategy.sold.netAmount }}</div>
              <div class="small">今日已实现 {{ tradeStrategy.todayRealized }} · 本轮回补 {{ tradeStrategy.matchedShares }} 股 · 未配对 {{ tradeStrategy.unmatchedShares }} 股</div>
              <div v-if="tradeStrategy.matchedShares" class="small">已回补净差额（含费用） {{ tradeStrategy.matchedSavings }}</div>
            </template>
            <div v-else class="small">已保存期初持仓，当前账段暂无成交。买入或卖出需确认入账后才计入汇总。</div>
          </template>
          <div v-else class="muted" style="margin: 6px 0">可先填写买入或卖出，首次提交前需核对期初持仓。</div>
          <div v-if="pendingTrade" class="small" style="margin-top: 8px">
            有一笔提交待核对
            <van-button size="small" plain :loading="restoringTrade" @click="restoreTradeDraft">恢复上次提交</van-button>
          </div>
          <div class="row" style="margin-top: 8px">
            <van-button v-if="item.followed" :disabled="!!pendingTrade" size="small" type="primary" plain @click="openFill">记录成交</van-button>
            <van-button size="small" plain @click="showTradeHistory = true">{{ tradeStrategy?.tradeIds?.length ? '成交记录及策略依据' : '成交记录' }}</van-button>
          </div>
        </div>

        <div class="card">
          <div class="row">
            <div>
              <div style="font-size: 17px; font-weight: 650">AI 分析</div>
              <div class="muted">
                <span v-if="running">正在分析，请稍候…</span>
                <span v-else-if="analysisElsewhere">另一只股票正在分析，请稍候…</span>
                <span v-else-if="savedAnalysis">上次 {{ savedAnalysis.createdAt }} · {{ savedAnalysis.modelName || 'AI' }}</span>
                <span v-else>点「开始分析」可选提示词和本次问题</span>
              </div>
            </div>
            <div class="row" style="gap: 6px">
              <van-button v-if="running" size="small" plain type="danger" @click="cancelAnalysis">取消</van-button>
              <van-button v-else-if="analysisElsewhere" size="small" disabled>分析进行中</van-button>
              <van-button v-else size="small" type="primary" @click="openAnalysisSheet">开始分析</van-button>
            </div>
          </div>

          <div class="position-toggle">
            <div class="row position-toggle-head">
              <div>
                <div class="position-toggle-title">带入持仓信息</div>
                <div class="muted">默认关闭；开启后可选股票，把成本和持仓发送给 AI 做组合判断</div>
              </div>
              <van-switch
                v-model="withPosition"
                size="22px"
                :disabled="running"
                @update:model-value="onWithPositionChange"
              />
            </div>

            <div v-if="withPosition" class="position-picker">
              <div class="row position-picker-head">
                <span class="muted">选择持仓（默认全选，仅列出已配置成本/持仓的股票）</span>
                <van-checkbox
                  v-if="positionOptions.length"
                  v-model="allSelected"
                  shape="square"
                  icon-size="15px"
                  :disabled="running"
                >
                  全选
                </van-checkbox>
              </div>
              <div v-if="loadingPositions" class="muted" style="padding: 8px 0">正在读取持仓…</div>
              <div v-else-if="!positionOptions.length" class="muted" style="padding: 8px 0">
                还没有配置成本/持仓的股票，请先在自选股里设置。
              </div>
              <div
                v-for="p in positionOptions"
                :key="p.code"
                class="position-row"
                :class="{ current: p.code === currentCode }"
              >
                <div class="position-main">
                  <div class="position-name">
                    {{ p.name || p.code }}
                    <span class="muted mono">{{ p.code }}</span>
                    <van-tag v-if="p.code === currentCode" type="primary" plain size="mini">当前</van-tag>
                  </div>
                  <div class="muted position-meta">
                    成本 {{ fmtPrice(p.costPrice) }} · 持仓 {{ p.volume }} 股
                    <span v-if="p.costPrice > 0 && p.volume > 0" :class="changeClass(p.profitAmount)">
                      · {{ fmtSignedMoney(p.profitAmount) }} ({{ fmtPercent(p.profitPercent) }})
                    </span>
                  </div>
                </div>
                <van-switch v-model="p.selected" size="20px" :disabled="running" />
              </div>
              <div class="muted" style="margin-top: 6px">
                只把勾选的股票和成本/持仓发送给 AI；不含现金、未勾选股票。
              </div>
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
                <span class="tag-mini">{{ alertStatus(a) }}</span>
                <van-switch :model-value="a.enabled" size="20px" :disabled="a.needsReview" @update:model-value="(v) => toggleAlert(a, v)" />
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
          AI 分析不再默认读取；在 AI 分析卡片打开「带入持仓信息」并勾选后才会使用。
        </div>
        <van-cell-group inset style="margin: 0 0 12px">
          <van-field v-model="cost" type="number" label="成本价" placeholder="例如 12.30" input-align="right" />
          <van-field v-model="volume" type="number" label="持仓(股)" placeholder="例如 1000" input-align="right" />
        </van-cell-group>
        <van-button type="primary" block round @click="savePosition">保存</van-button>
      </div>
    </van-popup>

    <van-popup
      v-model:show="showAnalysisSheet"
      position="bottom"
      round
      closeable
      :style="{ maxHeight: '88%' }"
      @closed="closePromptOverlays"
    >
      <div class="sheet analysis-sheet">
        <div class="sheet-title">开始分析</div>
        <div class="muted analysis-sheet-hint">
          选择本次使用的系统提示词，并可补充本次问题。只影响这一次，不会修改设置里的默认提示词。
        </div>

        <div v-if="withPosition" class="analysis-context">
          已带入 {{ selectedPositionCodes.length }} 只持仓
          <span v-if="selectedPositionCodes.includes(currentCode)">（含当前股票）</span>
        </div>
        <div v-else class="analysis-context muted">本次不带入持仓信息</div>

        <div class="analysis-section-title">系统提示词</div>
        <div v-if="loadingAnalysisPrompts" class="muted" style="padding: 8px 0">正在读取提示词…</div>
        <template v-else>
          <van-cell-group inset>
            <van-cell
              title="本次使用"
              :value="selectedPromptName"
              is-link
              clickable
              @click="openPromptPicker"
            />
          </van-cell-group>
          <van-button
            v-if="previewPrompts.length"
            class="prompt-preview-btn"
            size="small"
            type="primary"
            plain
            block
            icon="eye-o"
            @click="openPromptPreview"
          >
            查看提示词全文
          </van-button>
        </template>

        <div class="analysis-section-title">本次问题</div>
        <van-field
          v-model="analysisQuestion"
          type="textarea"
          rows="3"
          autosize
          maxlength="2000"
          show-word-limit
          placeholder="可补充本次关注点，例如：结合我的持仓给出止损位。留空则使用默认问题。"
        />

        <div class="analysis-sheet-actions">
          <van-button block plain @click="showAnalysisSheet = false">取消</van-button>
          <van-button
            block
            type="primary"
            :loading="running"
            :disabled="loadingAnalysisPrompts"
            @click="confirmStartAnalysis"
          >
            开始分析
          </van-button>
        </div>
      </div>
    </van-popup>

    <van-popup
      v-model:show="showPromptPicker"
      position="bottom"
      round
      :z-index="3000"
    >
      <van-picker
        title="选择系统提示词"
        :columns="promptColumns"
        :model-value="promptPickerValue"
        @confirm="onPromptConfirm"
        @cancel="showPromptPicker = false"
      />
    </van-popup>

    <van-popup
      v-model:show="showPromptPreview"
      position="bottom"
      round
      closeable
      :style="{ height: '86%' }"
      :z-index="3100"
    >
      <div class="sheet prompt-preview-sheet">
        <div class="sheet-title">提示词全文</div>
        <div class="muted prompt-preview-hint">
          这里只查看全文，返回后可在「本次使用」下拉框中选择。内置提示词和已保存模板都会列在下面。
        </div>
        <div v-for="p in previewPrompts" :key="p.id" class="prompt-preview-card">
          <div class="prompt-preview-head">
            <div class="prompt-preview-name">{{ p.name }}</div>
            <van-tag
              v-if="Number(p.id) === Number(selectedPromptId)"
              type="primary"
              plain
              size="mini"
            >
              本次选中
            </van-tag>
            <van-tag v-else type="default" plain size="mini">
              {{ p.type === 'builtin' ? '内置' : '已保存' }}
            </van-tag>
          </div>
          <div class="prompt-full-content">{{ p.content }}</div>
        </div>
        <div class="muted prompt-preview-note">
          「默认（跟随设置）」会使用设置页当前选中的系统提示词，不在这里重复展示正文。
        </div>
        <van-button block plain round @click="showPromptPreview = false">返回</van-button>
      </div>
    </van-popup>

    <TradeFillEdit
      :key="code"
      v-model:show="showTrade"
      :code="code"
      :baseline="editBaseline"
      :fill="editTrade"
      :alert-event-id="alertContext ? alertEventId : 0"
      :legacy-cost="item.costPrice || 0"
      :legacy-volume="item.volume || 0"
      :position="tradePosition"
      @saved="onTradeSaved"
    />
    <TradeHistory
      :key="'history-' + code"
      v-model:show="showTradeHistory"
      :code="code"
      :revision="tradePosition?.revision || 0"
      :highlights="tradeStrategy?.tradeIds || []"
      @edit="openTradeCorrection"
      @edit-baseline="openHistoryBaseline"
      @saved="onTradeSaved"
    />
    <AlertEdit
      v-model:show="showAlertEdit"
      :stock="{ code: item.code || code, name: item.name || '' }"
      :alert="editingAlert"
      :has-ledger="!!tradePosition"
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

<style scoped>
.position-toggle {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--ap-line);
}
.position-toggle-head {
  justify-content: space-between;
  gap: 12px;
}
.position-toggle-title {
  font-size: 15px;
  font-weight: 650;
}
.position-picker {
  margin-top: 10px;
}
.position-picker-head {
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 2px;
}
.position-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 0;
  border-bottom: 1px solid var(--ap-line);
}
.position-row.current {
  background: var(--ap-primary-soft, rgba(25, 137, 250, 0.06));
  margin: 0 -8px;
  padding-left: 8px;
  padding-right: 8px;
  border-radius: 8px;
}
.position-main {
  flex: 1;
  min-width: 0;
}
.position-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
}
.position-meta {
  font-size: 12px;
  margin-top: 2px;
  line-height: 1.45;
}
.analysis-sheet {
  padding-bottom: calc(16px + env(safe-area-inset-bottom));
}
.analysis-sheet-hint {
  margin-bottom: 10px;
  line-height: 1.55;
}
.analysis-context {
  margin: 6px 0 2px;
  font-size: 13px;
  font-weight: 600;
}
.analysis-section-title {
  margin: 14px 0 6px;
  font-size: 14px;
  font-weight: 650;
}
.prompt-preview-btn {
  margin-top: 8px;
}
.prompt-preview-sheet {
  padding-bottom: calc(18px + env(safe-area-inset-bottom));
}
.prompt-preview-hint {
  margin-bottom: 12px;
  line-height: 1.55;
}
.prompt-preview-card {
  padding: 12px 0;
  border-bottom: 1px solid var(--ap-line);
}
.prompt-preview-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}
.prompt-preview-name {
  min-width: 0;
  font-size: 14px;
  font-weight: 650;
}
.prompt-full-content {
  color: var(--ap-text-2, #4b5563);
  font-size: 13px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}
.prompt-preview-note {
  margin: 12px 0;
  line-height: 1.55;
}
.analysis-sheet-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 16px;
}
</style>
