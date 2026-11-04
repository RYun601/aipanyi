<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { showConfirmDialog, showToast } from 'vant'
import { api } from '../api'
import { isNative, native } from '../native'

const loading = ref(true)
const settings = ref(null)
const tradeFeeRate = ref('')
const savingTradeFeeRate = ref(false)
const aiConfigs = ref([])
const prompts = ref([])
const builtinPrompts = ref([])
const version = ref({})
const summary = ref({})
const testing = ref(false)

const showAiEditor = ref(false)
const showPromptEditor = ref(false)
const showAiPicker = ref(false)
const showPromptPicker = ref(false)
const showPromptText = ref(false)
const promptTextView = ref(null)
const showAdvancedAi = ref(false)
const showModelPicker = ref(false)
const modelKeyword = ref('')
const modelOptions = ref([])
const fetchingModels = ref(false)
const fetchingModelInfo = ref(false)
const filteredModelOptions = computed(() => {
  const keyword = modelKeyword.value.trim().toLowerCase()
  if (!keyword) return modelOptions.value
  return modelOptions.value.filter((m) => m.toLowerCase().includes(keyword))
})

const aiForm = reactive({
  id: 0,
  name: '',
  baseUrl: '',
  apiKey: '',
  modelName: '',
  modelType: 'chat',
  maxTokens: '',
  contextWindow: '',
  temperature: 0.7,
  timeOut: 300,
  httpProxy: '',
  httpProxyEnabled: false,
  extraHeaders: '',
  thinking: false,
  supportVision: false
})

const promptForm = reactive({ id: 0, name: '', type: 'stock', content: '' })

const nativeState = reactive({
  isNative: false,
  notifications: false,
  ignoringBattery: false,
  exactAlarms: false
})

const aiOptions = computed(() =>
  aiConfigs.value.map((c) => ({ text: `${c.name} · ${c.modelName}`, value: c.id }))
)
const promptOptions = computed(() => [
  { text: '自动（按持仓选择内置）', value: 0 },
  ...builtinPrompts.value.map((p) => ({ text: p.name, value: Number(p.id) })),
  ...prompts.value.map((p) => ({ text: p.name, value: Number(p.id) }))
])
const allPromptRows = computed(() => [
  ...builtinPrompts.value.map((p) => ({ ...p, builtin: true })),
  ...prompts.value.map((p) => ({ ...p, builtin: false }))
])
const selectedAiName = computed(() => {
  const c = aiConfigs.value.find((x) => x.id === settings.value?.defaultAiConfigId)
  return c ? c.name : '未选择'
})
const selectedPromptName = computed(() => {
  const id = Number(settings.value?.defaultPromptId)
  if (!id) return '自动（按持仓选择内置）'
  const p = [...builtinPrompts.value, ...prompts.value].find((x) => Number(x.id) === id)
  return p ? p.name : '自动（按持仓选择内置）'
})

async function load() {
  loading.value = true
  try {
    const [s, a, p, v, sum, ap] = await Promise.all([
      api.getSettings(),
      api.listAIConfigs(),
      api.listPrompts(),
      api.getVersionInfo(),
      api.getDataSummary(),
      api.listAnalysisPrompts().catch(() => [])
    ])
    settings.value = s
    tradeFeeRate.value = s?.tradeFeeRate ?? ''
    aiConfigs.value = a || []
    prompts.value = p || []
    builtinPrompts.value = (ap || []).filter((x) => Number(x.id) < 0)
    version.value = v || {}
    summary.value = sum || {}
  } catch (e) {
    showToast(e?.message || '加载设置失败')
  } finally {
    loading.value = false
  }
  refreshNative()
}

async function saveTradeFeeRate() {
  if (savingTradeFeeRate.value) return
  savingTradeFeeRate.value = true
  try {
    const rate = await api.setTradeFeeRate(tradeFeeRate.value)
    tradeFeeRate.value = rate
    showToast(rate === '' ? '已切换为手动填写费用' : '交易费率已保存')
  } catch (e) {
    showToast(e?.message || '保存交易费率失败')
  } finally {
    savingTradeFeeRate.value = false
  }
}

function refreshNative() {
  nativeState.isNative = isNative()
  if (!nativeState.isNative) return
  nativeState.notifications = native.notificationsEnabled()
  nativeState.ignoringBattery = native.isIgnoringBatteryOptimizations()
  nativeState.exactAlarms = native.canScheduleExactAlarms()
}

async function sendTestNotification() {
  try {
    const msg = await api.sendTestNotification()
    showToast(msg || '已发送')
  } catch (e) {
    showToast(e?.message || '发送失败')
  }
}

async function saveSettings() {
  if (!settings.value) return
  try {
    const payload = {
      ...settings.value,
      pollIntervalSec: Number(settings.value.pollIntervalSec) || 30
    }
    await api.saveSettings(payload)
    showToast('设置已保存')
  } catch (e) {
    showToast(e?.message || '保存失败')
  }
}

function resetAiForm(c) {
  Object.assign(aiForm, {
    id: c?.id || 0,
    name: c?.name || '',
    baseUrl: c?.baseUrl || '',
    apiKey: c?.apiKey || '',
    modelName: c?.modelName || '',
    modelType: c?.modelType || 'chat',
    maxTokens: c?.maxTokens || '',
    contextWindow: c?.contextWindow || '',
    temperature: c?.temperature ?? 0.7,
    timeOut: c?.timeOut || 300,
    httpProxy: c?.httpProxy || '',
    httpProxyEnabled: !!c?.httpProxyEnabled,
    extraHeaders: c?.extraHeaders || '',
    thinking: !!c?.thinking,
    supportVision: !!c?.supportVision
  })
  showAdvancedAi.value = false
  modelOptions.value = []
  modelKeyword.value = ''
  showModelPicker.value = false
}

async function fetchModels() {
  if (!aiForm.baseUrl.trim() || !aiForm.apiKey.trim()) {
    showToast('请先填写 BaseUrl 和 API Key')
    return
  }
  fetchingModels.value = true
  try {
    const list = await api.fetchAiModels(aiForm.baseUrl, aiForm.apiKey, aiForm.extraHeaders)
    const options = Array.isArray(list) ? list : []
    if (!options.length) {
      showToast('未获取到模型，请检查地址、API Key 或服务是否实现 /models')
      return
    }
    modelOptions.value = options
    modelKeyword.value = ''
    showModelPicker.value = true
  } catch (e) {
    showToast(e?.message || '获取模型失败')
  } finally {
    fetchingModels.value = false
  }
}

async function pickModel(model) {
  aiForm.modelName = model
  if (!aiForm.name.trim()) aiForm.name = model
  showModelPicker.value = false
  await fillModelInfo()
}

async function fillModelInfo() {
  if (!aiForm.modelName.trim() || !aiForm.baseUrl.trim()) return
  fetchingModelInfo.value = true
  try {
    const info = await api.fetchAiModelInfo(
      aiForm.baseUrl,
      aiForm.apiKey,
      aiForm.modelName,
      aiForm.extraHeaders
    )
    if (!info) return
    const parts = []
    if (info.maxTokens > 0) {
      aiForm.maxTokens = info.maxTokens
      parts.push(`最大输出 ${info.maxTokens}`)
    }
    if (info.contextWindow > 0) {
      aiForm.contextWindow = info.contextWindow
      parts.push(`上下文 ${info.contextWindow}`)
    }
    if (parts.length) {
      const source = info.source === 'api' ? '接口' : '内置数据'
      showToast(`已自动填充：${parts.join('，')}（来源：${source}）`)
    }
  } catch (e) {
    showToast(e?.message || '获取模型参数失败')
  } finally {
    fetchingModelInfo.value = false
  }
}

function addAi() {
  resetAiForm(null)
  showAiEditor.value = true
}

function editAi(c) {
  resetAiForm(c)
  showAiEditor.value = true
}

function aiPayload() {
  return {
    ...aiForm,
    maxTokens: Number(aiForm.maxTokens) || 0,
    contextWindow: Number(aiForm.contextWindow) || 0,
    temperature: Number(aiForm.temperature) || 0,
    timeOut: Number(aiForm.timeOut) || 0
  }
}

async function saveAi() {
  if (!aiForm.name.trim() || !aiForm.baseUrl.trim() || !aiForm.modelName.trim()) {
    showToast('请填写名称、BaseUrl、模型名')
    return
  }
  try {
    await api.saveAIConfig(aiPayload())
    showToast('已保存')
    showAiEditor.value = false
    await load()
  } catch (e) {
    showToast(e?.message || '保存失败')
  }
}

async function testAi() {
  testing.value = true
  try {
    const msg = await api.testAIConfig(aiPayload())
    showToast(msg || '连接成功')
  } catch (e) {
    showToast(e?.message || '连接失败')
  } finally {
    testing.value = false
  }
}

async function removeAi(c) {
  try {
    await showConfirmDialog({ title: '删除 AI 配置', message: `确定删除「${c.name}」吗？` })
  } catch {
    return
  }
  try {
    await api.deleteAIConfig(c.id)
    showToast('已删除')
    await load()
  } catch (e) {
    showToast(e?.message || '删除失败')
  }
}

function addPrompt() {
  Object.assign(promptForm, { id: 0, name: '', type: 'stock', content: '' })
  showPromptEditor.value = true
}

function editPrompt(p) {
  Object.assign(promptForm, { id: p.id, name: p.name, type: p.type || 'stock', content: p.content })
  showPromptEditor.value = true
}

function viewPrompt(p) {
  promptTextView.value = p
  showPromptText.value = true
}

async function savePrompt() {
  if (!promptForm.name.trim() || !promptForm.content.trim()) {
    showToast('请填写名称和内容')
    return
  }
  try {
    await api.savePrompt({ ...promptForm })
    showToast('已保存')
    showPromptEditor.value = false
    await load()
  } catch (e) {
    showToast(e?.message || '保存失败')
  }
}

async function removePrompt(p) {
  try {
    await showConfirmDialog({ title: '删除提示词', message: `确定删除「${p.name}」吗？` })
  } catch {
    return
  }
  try {
    await api.deletePrompt(p.id)
    showToast('已删除')
    await load()
  } catch (e) {
    showToast(e?.message || '删除失败')
  }
}

function onAiPick({ selectedOptions }) {
  if (settings.value && selectedOptions?.[0]) {
    settings.value.defaultAiConfigId = selectedOptions[0].value
    saveSettings()
  }
  showAiPicker.value = false
}

function onPromptPick({ selectedOptions }) {
  if (settings.value && selectedOptions?.[0]) {
    settings.value.defaultPromptId = Number(selectedOptions[0].value) || 0
    saveSettings()
  }
  showPromptPicker.value = false
}

async function requestNotifications() {
  if (!nativeState.isNative) {
    showToast('请在 Android App 内操作')
    return
  }
  native.requestNotifications()
  setTimeout(refreshNative, 800)
}

function openNotificationSettings() {
  native.openNotificationSettings()
  setTimeout(refreshNative, 800)
}

function openBatterySettings() {
  native.openBatterySettings()
  setTimeout(refreshNative, 1500)
}

function openAutoStart() {
  native.openAutoStartSettings()
}

function openExactAlarmSettings() {
  native.requestExactAlarmPermission()
  setTimeout(refreshNative, 1500)
}

async function confirmExit() {
  try {
    await showConfirmDialog({
      title: '退出应用',
      message: '将立即停止后台监控；下一个交易窗口仍会按已启用提醒自动启动。'
    })
  } catch {
    return
  }
  native.exitApp()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div style="padding: 6px 4px 10px">
      <div style="font-size: 22px; font-weight: 700">设置</div>
      <div class="muted">AI、提示词、提醒与后台保活</div>
    </div>

    <van-loading v-if="loading && !settings" style="padding: 80px 0" vertical>加载中</van-loading>

    <template v-else-if="settings">
      <div class="card">
        <div style="font-size: 17px; font-weight: 650">成交记账</div>
        <van-field
          v-model="tradeFeeRate"
          type="number"
          label="交易费率 %"
          placeholder="留空为手动填写"
          input-align="right"
          clearable
          :disabled="savingTradeFeeRate"
        >
          <template #button>
            <van-button size="small" type="primary" plain :loading="savingTradeFeeRate" @click="saveTradeFeeRate">保存费率</van-button>
          </template>
        </van-field>
        <div class="muted">买入和卖出共用。填 0.03 表示 0.03%，按成交金额计算，费用四舍五入到分。</div>
      </div>

      <div class="card">
        <div class="row">
          <div style="font-size: 17px; font-weight: 650">AI 模型配置</div>
          <van-button size="small" plain type="primary" @click="addAi">新增</van-button>
        </div>
        <div v-if="!aiConfigs.length" class="muted" style="padding: 12px 0">
          还没有 AI 配置，添加一个 OpenAI 兼容服务即可使用分析功能。
        </div>
        <div v-for="c in aiConfigs" :key="c.id" style="padding: 10px 0; border-bottom: 1px solid var(--ap-line)">
          <div class="row">
            <div style="min-width: 0">
              <div style="font-weight: 600">{{ c.name }}</div>
              <div class="muted mono">{{ c.modelName }}</div>
              <div class="muted mono" style="word-break: break-all">{{ c.baseUrl }}</div>
            </div>
            <div class="row" style="gap: 10px">
              <van-icon name="edit" size="18" @click="editAi(c)" />
              <van-icon name="delete-o" size="18" color="#e5484d" @click="removeAi(c)" />
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="row">
          <div style="font-size: 17px; font-weight: 650">系统提示词</div>
          <van-button size="small" plain type="primary" @click="addPrompt">新增</van-button>
        </div>
        <div class="muted" style="margin: 6px 0 2px">
          内置提示词只读；自定义模板可新增、编辑或删除。
        </div>
        <div v-if="!allPromptRows.length" class="muted" style="padding: 12px 0">
          还没有提示词。
        </div>
        <div
          v-for="p in allPromptRows"
          :key="p.id"
          style="padding: 10px 0; border-bottom: 1px solid var(--ap-line)"
        >
          <div class="row">
            <div style="min-width: 0">
              <div style="font-weight: 600">{{ p.name }}</div>
              <div class="muted">{{ p.builtin ? '内置提示词 · 只读' : p.type || 'stock' }}</div>
            </div>
            <div class="row" style="gap: 10px; align-items: center">
              <van-tag v-if="p.builtin" type="primary" plain size="mini">内置</van-tag>
              <van-icon name="eye-o" size="18" @click="viewPrompt(p)" />
              <template v-if="!p.builtin">
                <van-icon name="edit" size="18" @click="editPrompt(p)" />
                <van-icon name="delete-o" size="18" color="#e5484d" @click="removePrompt(p)" />
              </template>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div style="font-size: 17px; font-weight: 650; margin-bottom: 8px">分析默认项</div>
        <van-cell-group inset>
          <van-field
            label="默认 AI"
            readonly
            :model-value="selectedAiName"
            is-link
            @click="showAiPicker = true"
          />
          <van-field
            label="默认提示词"
            readonly
            :model-value="selectedPromptName"
            is-link
            @click="showPromptPicker = true"
          />
          <van-field v-model="settings.pollIntervalSec" type="number" label="轮询秒数" input-align="right" />
        </van-cell-group>
        <div style="margin-top: 8px">
          <van-cell-group inset>
            <van-cell title="分析后自动提取提醒" center>
              <template #right-icon>
                <van-switch v-model="settings.autoExtractAlerts" size="22px" />
              </template>
            </van-cell>
            <van-cell title="发送系统通知" center>
              <template #right-icon>
                <van-switch v-model="settings.pushEnabled" size="22px" />
              </template>
            </van-cell>
          </van-cell-group>
        </div>
        <van-button type="primary" block round style="margin-top: 12px" @click="saveSettings">
          保存设置
        </van-button>
      </div>

      <div class="card">
        <div style="font-size: 17px; font-weight: 650">后台与通知</div>
        <div v-if="!nativeState.isNative" class="muted" style="padding: 8px 0">
          当前是浏览器预览模式；后台保活与系统通知只在 Android App 内生效。
        </div>
        <template v-else>
          <van-cell title="通知权限" :label="nativeState.notifications ? '已开启' : '未开启，价格提醒无法弹出'" center>
            <template #right-icon>
              <van-button
                size="small"
                :type="nativeState.notifications ? 'default' : 'danger'"
                plain
                @click="nativeState.notifications ? openNotificationSettings() : requestNotifications()"
              >
                {{ nativeState.notifications ? '管理' : '去开启' }}
              </van-button>
            </template>
          </van-cell>
          <van-cell title="电池优化" :label="nativeState.ignoringBattery ? '已允许后台运行' : '未加入白名单，可能被系统清理'" center>
            <template #right-icon>
              <van-button size="small" plain type="primary" @click="openBatterySettings">去设置</van-button>
            </template>
          </van-cell>
          <van-cell
            title="精确闹钟"
            :label="nativeState.exactAlarms ? '已授权，交易窗口可准时启动' : '未授权，系统可能延迟启动'"
            center
          >
            <template #right-icon>
              <van-button
                size="small"
                plain
                type="primary"
                @click="openExactAlarmSettings"
              >
                {{ nativeState.exactAlarms ? '已开启' : '去开启' }}
              </van-button>
            </template>
          </van-cell>
          <van-cell title="测试通知" label="发一条价格提醒，检查通知权限和渠道" center>
            <template #right-icon>
              <van-button size="small" plain type="primary" @click="sendTestNotification">发送</van-button>
            </template>
          </van-cell>
          <van-cell title="应用启动管理" label="Honor/MagicOS 需要允许自启动与后台活动" center>
            <template #right-icon>
              <van-button size="small" plain type="primary" @click="openAutoStart">去设置</van-button>
            </template>
          </van-cell>
          <div class="hint" style="margin-top: 10px">
            后台只在已启用提醒涉及的市场交易时段运行；非交易时段自动停止，下个窗口自动启动。
            当前启用市场由提醒规则决定（A 股 / 港股 / 美股）。
            设置 → 应用 → 应用启动管理 → AI盘译 → 手动管理，允许自启动、关联启动、后台活动。
          </div>
          <van-button size="small" plain style="margin-top: 10px" @click="refreshNative">刷新状态</van-button>
          <van-button
            type="danger"
            plain
            block
            round
            style="margin-top: 12px"
            @click="confirmExit"
          >
            退出应用
          </van-button>
        </template>
      </div>

      <div class="card">
        <div style="font-size: 17px; font-weight: 650">关于</div>
        <div class="row" style="margin-top: 8px">
          <span class="muted">版本</span>
          <span class="mono">{{ version.version || '-' }}</span>
        </div>
        <div class="row" style="margin-top: 6px">
          <span class="muted">自选 / 提醒</span>
          <span class="mono">{{ summary.followed || 0 }} / {{ summary.alerts || 0 }}</span>
        </div>
        <div class="muted" style="margin-top: 10px; line-height: 1.6">
          数据保存在手机本地 SQLite，不上传云端。AI 请求直接发往你配置的服务地址。
        </div>
        <a
          href="https://github.com/RYun601/go-stock"
          target="_blank"
          rel="noreferrer"
          class="muted"
          style="display: inline-block; margin-top: 10px"
        >
          源码与许可证（GPLv3）
        </a>
      </div>
    </template>

    <van-popup v-model:show="showAiEditor" position="bottom" round style="max-height: 92vh">
      <div class="sheet" style="max-height: 92vh; overflow-y: auto">
        <div class="sheet-title">{{ aiForm.id ? '编辑 AI 配置' : '新增 AI 配置' }}</div>
        <van-cell-group inset>
          <van-field v-model="aiForm.name" label="名称" placeholder="例如 DeepSeek" />
          <van-field v-model="aiForm.baseUrl" label="BaseUrl" placeholder="https://api.deepseek.com/v1" />
          <van-field v-model="aiForm.apiKey" type="password" label="API Key" placeholder="留空则保留原 Key" />
          <van-field
            v-model="aiForm.modelName"
            label="模型名"
            placeholder="选择模型或手动输入"
            @blur="fillModelInfo"
          >
            <template #button>
              <van-button size="small" type="primary" plain :loading="fetchingModels" @click="fetchModels">
                获取模型
              </van-button>
            </template>
          </van-field>
          <van-cell title="深度思考" center>
            <template #right-icon>
              <van-switch v-model="aiForm.thinking" size="22px" />
            </template>
          </van-cell>
        </van-cell-group>

        <van-cell
          title="高级参数"
          is-link
          :arrow-direction="showAdvancedAi ? 'up' : 'down'"
          @click="showAdvancedAi = !showAdvancedAi"
        />
        <van-cell-group v-if="showAdvancedAi" inset>
          <van-field v-model="aiForm.modelType" label="类型" placeholder="chat / embedding" />
          <van-field
            v-model="aiForm.maxTokens"
            type="number"
            label="MaxTokens"
            input-align="right"
            placeholder="留空自动（推荐）"
          />
          <van-field
            v-model="aiForm.contextWindow"
            type="number"
            label="上下文窗口"
            input-align="right"
            placeholder="留空自动"
          />
          <van-field v-model="aiForm.temperature" type="number" label="Temperature" input-align="right" />
          <van-field v-model="aiForm.timeOut" type="number" label="超时(秒)" input-align="right" />
          <van-field v-model="aiForm.httpProxy" label="HTTP 代理" placeholder="http://127.0.0.1:7890" />
          <van-cell title="启用代理" center>
            <template #right-icon><van-switch v-model="aiForm.httpProxyEnabled" size="22px" /></template>
          </van-cell>
          <van-cell title="支持图片" center>
            <template #right-icon><van-switch v-model="aiForm.supportVision" size="22px" /></template>
          </van-cell>
          <van-field
            v-model="aiForm.extraHeaders"
            label="额外 Header"
            type="textarea"
            rows="2"
            placeholder='{"x-team-id":"..."}'
          />
        </van-cell-group>

        <div class="row" style="margin-top: 14px">
          <van-button plain block round :loading="testing" @click="testAi">测试连接</van-button>
          <van-button type="primary" block round @click="saveAi">保存</van-button>
        </div>
      </div>
    </van-popup>

    <van-popup v-model:show="showModelPicker" position="bottom" round style="height: 70vh">
      <div class="sheet" style="height: 100%; display: flex; flex-direction: column">
        <div class="sheet-title">选择模型</div>
        <van-search v-model="modelKeyword" placeholder="搜索模型" />
        <div style="flex: 1; overflow-y: auto">
          <van-cell
            v-for="m in filteredModelOptions"
            :key="m"
            :title="m"
            is-link
            @click="pickModel(m)"
          />
          <van-empty v-if="!filteredModelOptions.length" description="没有匹配的模型" />
        </div>
      </div>
    </van-popup>

    <van-popup v-model:show="showPromptEditor" position="bottom" round style="max-height: 92vh">
      <div class="sheet" style="max-height: 92vh; overflow-y: auto">
        <div class="sheet-title">{{ promptForm.id ? '编辑提示词' : '新增提示词' }}</div>
        <van-cell-group inset>
          <van-field v-model="promptForm.name" label="名称" placeholder="例如 我的盘前策略" />
          <van-field v-model="promptForm.type" label="类型" placeholder="stock" />
          <van-field
            v-model="promptForm.content"
            type="textarea"
            label="内容"
            rows="10"
            autosize
            placeholder="系统提示词内容，可使用 {{stockName}} {{stockCode}} {{costPrice}} {{costVolume}}"
          />
        </van-cell-group>
        <van-button type="primary" block round style="margin-top: 14px" @click="savePrompt">保存</van-button>
      </div>
    </van-popup>

    <van-popup v-model:show="showAiPicker" position="bottom" round>
      <van-picker
        title="默认 AI 配置"
        :columns="aiOptions"
        @confirm="onAiPick"
        @cancel="showAiPicker = false"
      />
    </van-popup>

    <van-popup v-model:show="showPromptPicker" position="bottom" round>
      <van-picker
        title="默认提示词"
        :columns="promptOptions"
        @confirm="onPromptPick"
        @cancel="showPromptPicker = false"
      />
    </van-popup>

    <van-popup v-model:show="showPromptText" position="bottom" round closeable style="max-height: 80vh">
      <div class="sheet">
        <div class="sheet-title">{{ promptTextView?.name || '提示词全文' }}</div>
        <div class="muted" style="margin-bottom: 10px">
          {{ promptTextView?.builtin ? '内置提示词（只读）' : '自定义提示词' }}
        </div>
        <div class="prompt-full-text">{{ promptTextView?.content || '' }}</div>
        <van-button block plain round style="margin-top: 14px" @click="showPromptText = false">关闭</van-button>
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.prompt-full-text {
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--ap-text-2, #4b5563);
  font-size: 13px;
  line-height: 1.65;
}
</style>
