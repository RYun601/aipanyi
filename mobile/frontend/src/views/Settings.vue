<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { showConfirmDialog, showToast } from 'vant'
import { api } from '../api'
import { isNative, native } from '../native'

const loading = ref(true)
const settings = ref(null)
const aiConfigs = ref([])
const prompts = ref([])
const version = ref({})
const summary = ref({})
const testing = ref(false)

const showAiEditor = ref(false)
const showPromptEditor = ref(false)
const showAiPicker = ref(false)
const showPromptPicker = ref(false)
const showAdvancedAi = ref(false)

const aiForm = reactive({
  id: 0,
  name: '',
  baseUrl: '',
  apiKey: '',
  modelName: '',
  modelType: 'chat',
  maxTokens: 4096,
  contextWindow: 0,
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
  ignoringBattery: false
})

const aiOptions = computed(() =>
  aiConfigs.value.map((c) => ({ text: `${c.name} · ${c.modelName}`, value: c.id }))
)
const promptOptions = computed(() => [
  { text: '内置提示词', value: 0 },
  ...prompts.value.map((p) => ({ text: p.name, value: p.id }))
])
const selectedAiName = computed(() => {
  const c = aiConfigs.value.find((x) => x.id === settings.value?.defaultAiConfigId)
  return c ? c.name : '未选择'
})
const selectedPromptName = computed(() => {
  const p = prompts.value.find((x) => x.id === settings.value?.defaultPromptId)
  return p ? p.name : '内置提示词'
})

async function load() {
  loading.value = true
  try {
    const [s, a, p, v, sum] = await Promise.all([
      api.getSettings(),
      api.listAIConfigs(),
      api.listPrompts(),
      api.getVersionInfo(),
      api.getDataSummary()
    ])
    settings.value = s
    aiConfigs.value = a || []
    prompts.value = p || []
    version.value = v || {}
    summary.value = sum || {}
  } catch (e) {
    showToast(e?.message || '加载设置失败')
  } finally {
    loading.value = false
  }
  refreshNative()
}

function refreshNative() {
  nativeState.isNative = isNative()
  if (!nativeState.isNative) return
  nativeState.notifications = native.notificationsEnabled()
  nativeState.ignoringBattery = native.isIgnoringBatteryOptimizations()
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
    maxTokens: c?.maxTokens || 4096,
    contextWindow: c?.contextWindow || 0,
    temperature: c?.temperature ?? 0.7,
    timeOut: c?.timeOut || 300,
    httpProxy: c?.httpProxy || '',
    httpProxyEnabled: !!c?.httpProxyEnabled,
    extraHeaders: c?.extraHeaders || '',
    thinking: !!c?.thinking,
    supportVision: !!c?.supportVision
  })
  showAdvancedAi.value = false
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
    settings.value.defaultPromptId = selectedOptions[0].value
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
        <div v-if="!prompts.length" class="muted" style="padding: 12px 0">
          还没有自定义提示词；分析时会使用内置提示词。
        </div>
        <div v-for="p in prompts" :key="p.id" style="padding: 10px 0; border-bottom: 1px solid var(--ap-line)">
          <div class="row">
            <div>
              <div style="font-weight: 600">{{ p.name }}</div>
              <div class="muted">{{ p.type || 'stock' }}</div>
            </div>
            <div class="row" style="gap: 10px">
              <van-icon name="edit" size="18" @click="editPrompt(p)" />
              <van-icon name="delete-o" size="18" color="#e5484d" @click="removePrompt(p)" />
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
            建议：设置 → 应用 → 应用启动管理 → AI盘译 → 手动管理，允许自启动、关联启动、后台活动；
            App 已从最近任务隐藏，避免被划掉；从桌面图标或常驻通知打开。即使这样，系统仍可能清理，无法保证 100% 准时。
          </div>
          <van-button size="small" plain style="margin-top: 10px" @click="refreshNative">刷新状态</van-button>
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
          <van-field v-model="aiForm.modelName" label="模型名" placeholder="deepseek-chat" />
          <van-field v-model="aiForm.modelType" label="类型" placeholder="chat / embedding" />
          <van-field v-model="aiForm.maxTokens" type="number" label="MaxTokens" input-align="right" />
          <van-field v-model="aiForm.temperature" type="number" label="Temperature" input-align="right" />
          <van-field v-model="aiForm.timeOut" type="number" label="超时(秒)" input-align="right" />
        </van-cell-group>

        <van-cell title="高级选项" is-link :arrow-direction="showAdvancedAi ? 'up' : 'down'" @click="showAdvancedAi = !showAdvancedAi" />
        <van-cell-group v-if="showAdvancedAi" inset>
          <van-field v-model="aiForm.contextWindow" type="number" label="上下文窗口" input-align="right" />
          <van-field v-model="aiForm.httpProxy" label="HTTP 代理" placeholder="http://127.0.0.1:7890" />
          <van-cell title="启用代理" center>
            <template #right-icon><van-switch v-model="aiForm.httpProxyEnabled" size="22px" /></template>
          </van-cell>
          <van-cell title="深度思考" center>
            <template #right-icon><van-switch v-model="aiForm.thinking" size="22px" /></template>
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
  </div>
</template>
