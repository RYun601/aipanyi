# 移动端体验与后台生命周期实现计划

- 日期：2026-11-01
- 分支：`feat/mobile-ux-config-storage-background`
- 依据：[`docs/superpowers/specs/2026-11-01-mobile-ux-config-storage-background-design.md`](../specs/2026-11-01-mobile-ux-config-storage-background-design.md)
- 目标：完成自选左滑删除、模型配置简化、移动端文件归拢、按交易窗口自动启停后台服务，并留下可事后分析的生命周期日志。

## Goal

交付一个可以在真机验证的完整版本：

1. 自选列表左滑删除，删除后自动停用该股票提醒。
2. 移动端和桌面端都能一键获取模型列表，选择模型后自动填充最大输出和上下文窗口；日常只看到「深度思考」开关，其他参数收进「高级参数」。
3. 移动端数据库、日志、外置数据文件全部收进 `filesDir/aipanyi/`，旧数据自动迁移。
4. 后台服务只在已启用提醒涉及市场的交易窗口内运行；没有启用提醒时不常驻。
5. 设置页可以手动退出应用；退出后本次停止，下一个未来窗口仍会自动启动。
6. Kotlin 写 `aipanyi/logs/runtime.log`，Go 写 `aipanyi/logs/info.log`/`error.log`，能还原自动启停/退出时间线。

## Architecture

- 模型探测：`backend/modelinfo` 只放内置模型元数据；`backend/data/ai_models_api.go` 负责 HTTP 探测；桌面 `app.go` 和移动 `mobile/core/app.go` 都是薄包装。
- 存储：`mobile/core/storage.go` 计算新根目录 `filesDir/aipanyi/` 并迁移旧文件；`backend/data` 通过 `SetDataDir` 支持可配置数据目录。
- 调度：`backend/marketcal` 计算当前/下一个监控窗口；`mobile/core/monitor_schedule.go` 查询启用提醒的市场并通过 `Notifier.UpdateMonitorSchedule` 下发；Kotlin `MonitorScheduler` 用 `AlarmManager` 启停 `MonitorService`。
- Android 生命周期：`MonitorService` 维护 `uiVisible`，窗口结束且 UI 不可见时停止；手动退出停止本次并保留下一个闹钟。
- 日志：Kotlin `RuntimeLog` 写应用私有滚动日志；Go 沿用 zap/lumberjack。

## Tech Stack

- Go 1.27、Wails v2、Vue 3、Naive UI、Vant 4、Kotlin/Android SDK 34、gomobile。
- 测试：Go `testing` + `httptest`；前端用 `npm run build` 做编译验证；Android 用 Gradle 编译 + 真机验收。

## 执行前检查

```powershell
cd F:\aipanyi
git status --short
git branch --show-current
# 期望：## feat/mobile-ux-config-storage-background，工作区只有未跟踪的 .codegraph/
```

所有命令在仓库根目录执行，除非明确写了 `cd`。

---

## Task 1：抽取内置模型元数据并共用模型探测

**Files**

- Create: `backend/modelinfo/modelinfo.go`（由 `backend/agent/builtin_models.go` 迁入）
- Create: `backend/agent/builtin_models.go`（改为薄包装）
- Create: `backend/data/ai_models_api.go`
- Create: `backend/data/ai_models_api_test.go`
- Modify: `app.go`（`FetchAiModels` / `FetchAiModelInfo` 改为调用 `backend/data`）
- Modify: `mobile/core/app.go`
- Modify: `mobile/core/models.go`
- Modify: `mobile/frontend/src/api.js`

**Step 1：迁移内置模型表**

```powershell
New-Item -ItemType Directory -Force -Path backend\modelinfo | Out-Null
git mv backend/agent/builtin_models.go backend/modelinfo/modelinfo.go
```

把 `backend/modelinfo/modelinfo.go` 第一行 `package agent` 改为 `package modelinfo`。文件内容（两张表、前缀匹配、`GetBuiltinModelContextWindow`、`GetBuiltinModelMaxOutput`）保持不变。

新建 `backend/agent/builtin_models.go`：

```go
package agent

import "aipanyi/backend/modelinfo"

// GetBuiltinModelContextWindow 返回模型上下文窗口大小。
// 实际数据表已迁到 backend/modelinfo，避免移动端为了模型元数据引入整个 agent 包。
func GetBuiltinModelContextWindow(modelName string) int {
	return modelinfo.GetBuiltinModelContextWindow(modelName)
}

// GetBuiltinModelMaxOutput 返回模型单次生成的输出上限。
func GetBuiltinModelMaxOutput(modelName string) int {
	return modelinfo.GetBuiltinModelMaxOutput(modelName)
}
```

**Step 2：先写失败测试**

新建 `backend/data/ai_models_api_test.go`：

```go
package data

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAiModelsAndInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "m1"}, {"id": "m2"}, {"id": "m1"}},
			})
		case "/v1/models/m1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                 "m1",
				"context_length":     128000,
				"max_output_tokens":  8192,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	got := FetchAiModels(srv.URL+"/v1", "test-key", "")
	if len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("FetchAiModels() = %#v, want [m1 m2]", got)
	}

	info := FetchAiModelInfo(srv.URL+"/v1", "test-key", "m1", "")
	if info == nil {
		t.Fatal("FetchAiModelInfo() = nil")
	}
	if info.ContextWindow != 128000 || info.MaxTokens != 8192 || info.Source != "api" {
		t.Fatalf("FetchAiModelInfo() = %#v", info)
	}
}

func TestFetchAiModelInfoFallsBackToBuiltin(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	info := FetchAiModelInfo(srv.URL+"/v1", "test-key", "deepseek-chat", "")
	if info == nil {
		t.Fatal("FetchAiModelInfo() = nil")
	}
	if info.ContextWindow != 1000000 {
		t.Fatalf("ContextWindow = %d, want 1000000", info.ContextWindow)
	}
	if info.MaxTokens != 384000 {
		t.Fatalf("MaxTokens = %d, want 384000", info.MaxTokens)
	}
	if info.Source != "builtin" {
		t.Fatalf("Source = %q, want builtin", info.Source)
	}
}
```

运行并确认失败：

```powershell
go test ./backend/data -run TestFetchAiModel -count=1
```

预期：编译失败，`FetchAiModels` / `FetchAiModelInfo` / `AiModelInfo` 未定义。

**Step 3：实现共用模型探测**

新建 `backend/data/ai_models_api.go`：

```go
package data

import (
	"strings"

	"aipanyi/backend/logger"
	"aipanyi/backend/modelinfo"
)

// AiModelInfo 模型能力信息。MaxTokens 是输出上限，ContextWindow 是输入+输出总容量。
type AiModelInfo struct {
	ModelName     string `json:"modelName"`
	MaxTokens     int    `json:"maxTokens"`
	ContextWindow int    `json:"contextWindow"`
	Source        string `json:"source"` // api / builtin
}

type aiModelListItem struct {
	ID string `json:"id"`
}

// FetchAiModels 请求 OpenAI 兼容的 /models 接口，返回去重后的模型 ID。
// 失败返回空切片，不抛异常，方便前端保留手动输入。
func FetchAiModels(baseUrl, apiKey, extraHeaders string) []string {
	baseUrl = strings.TrimSpace(baseUrl)
	apiKey = strings.TrimSpace(apiKey)
	if baseUrl == "" || apiKey == "" {
		return []string{}
	}

	var respData struct {
		Data []aiModelListItem `json:"data"`
	}
	req := SharedHTTPClient.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetResult(&respData)
	if extra := BuildExtraHeaders(extraHeaders, ""); len(extra) > 0 {
		req = req.SetHeaders(extra)
	}

	resp, err := req.Get(strings.TrimRight(baseUrl, "/") + "/models")
	if err != nil {
		logger.SugaredLogger.Errorf("FetchAiModels error: %v", err)
		return []string{}
	}
	if resp.IsError() {
		logger.SugaredLogger.Errorf("FetchAiModels http error: %s", resp.Status())
		return []string{}
	}

	out := make([]string, 0, len(respData.Data))
	seen := make(map[string]bool, len(respData.Data))
	for _, item := range respData.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// FetchAiModelInfo 先请求 /models/{modelName}，再用内置模型表兜底。
func FetchAiModelInfo(baseUrl, apiKey, modelName, extraHeaders string) *AiModelInfo {
	baseUrl = strings.TrimSpace(baseUrl)
	modelName = strings.TrimSpace(modelName)
	if baseUrl == "" || modelName == "" {
		return nil
	}

	info := &AiModelInfo{ModelName: modelName}
	if apiKey != "" {
		type modelDetail struct {
			ID             string `json:"id"`
			MaxContextLen  int    `json:"max_context_length"`
			ContextLength  int    `json:"context_length"`
			MaxOutputTok   int    `json:"max_output_tokens"`
			MaxTokensField int    `json:"max_tokens"`
		}
		var detail modelDetail

		req := SharedHTTPClient.R().
			SetHeader("Authorization", "Bearer "+apiKey).
			SetHeader("Content-Type", "application/json").
			SetResult(&detail)
		if extra := BuildExtraHeaders(extraHeaders, ""); len(extra) > 0 {
			req = req.SetHeaders(extra)
		}

		resp, err := req.Get(strings.TrimRight(baseUrl, "/") + "/models/" + modelName)
		if err == nil && !resp.IsError() && detail.ID != "" {
			if detail.MaxContextLen > 0 {
				info.ContextWindow = detail.MaxContextLen
				info.Source = "api"
			} else if detail.ContextLength > 0 {
				info.ContextWindow = detail.ContextLength
				info.Source = "api"
			}
			if detail.MaxOutputTok > 0 {
				info.MaxTokens = detail.MaxOutputTok
				info.Source = "api"
			} else if detail.MaxTokensField > 0 {
				info.MaxTokens = detail.MaxTokensField
				info.Source = "api"
			}
		}
	}

	if info.ContextWindow == 0 {
		if cw := modelinfo.GetBuiltinModelContextWindow(modelName); cw > 0 {
			info.ContextWindow = cw
			if info.Source == "" {
				info.Source = "builtin"
			}
		}
	}
	if info.MaxTokens == 0 {
		if mo := modelinfo.GetBuiltinModelMaxOutput(modelName); mo > 0 {
			info.MaxTokens = mo
			if info.Source == "" {
				info.Source = "builtin"
			}
		}
	}
	return info
}
```

运行测试：

```powershell
go test ./backend/data -run TestFetchAiModel -count=1
go test ./backend/agent -run TestBuiltinKimiModelMetadata -count=1
```

**Step 4：桌面端改为薄包装**

把 `app.go` 中 `FetchAiModels` 的函数体替换为：

```go
func (a *App) FetchAiModels(baseUrl, apiKey, extraHeaders string) []string {
	return data.FetchAiModels(baseUrl, apiKey, extraHeaders)
}
```

把 `FetchAiModelInfo` 的函数体替换为：

```go
func (a *App) FetchAiModelInfo(baseUrl, apiKey, modelName, extraHeaders string) *AiModelInfo {
	info := data.FetchAiModelInfo(baseUrl, apiKey, modelName, extraHeaders)
	if info == nil {
		return nil
	}
	return &AiModelInfo{
		ModelName:     info.ModelName,
		MaxTokens:     info.MaxTokens,
		ContextWindow: info.ContextWindow,
		Source:        info.Source,
	}
}
```

保留 `type AiModelInfo struct` 不变，避免 Wails 绑定类型变化。

**Step 5：移动端 Go facade 和前端 API**

在 `mobile/core/models.go` 增加：

```go
type aiModelInfoDTO struct {
	ModelName     string `json:"modelName"`
	MaxTokens     int    `json:"maxTokens"`
	ContextWindow int    `json:"contextWindow"`
	Source        string `json:"source"`
}
```

在 `mobile/core/app.go` 的 AI 配置区域增加：

```go
func (a *mobileApp) FetchAiModels(baseUrl, apiKey, extraHeaders string) []string {
	return data.FetchAiModels(baseUrl, apiKey, extraHeaders)
}

func (a *mobileApp) FetchAiModelInfo(baseUrl, apiKey, modelName, extraHeaders string) *aiModelInfoDTO {
	info := data.FetchAiModelInfo(baseUrl, apiKey, modelName, extraHeaders)
	if info == nil {
		return nil
	}
	return &aiModelInfoDTO{
		ModelName:     info.ModelName,
		MaxTokens:     info.MaxTokens,
		ContextWindow: info.ContextWindow,
		Source:        info.Source,
	}
}
```

在 `mobile/frontend/src/api.js` 的 AI 配置区域增加：

```js
fetchAiModels: (baseUrl, apiKey, extraHeaders = '') =>
  call('FetchAiModels', baseUrl, apiKey, extraHeaders),
fetchAiModelInfo: (baseUrl, apiKey, modelName, extraHeaders = '') =>
  call('FetchAiModelInfo', baseUrl, apiKey, modelName, extraHeaders),
```

**Step 6：验证（合并到 Task 2 提交）**

```powershell
go test ./backend/data -run TestFetchAiModel -count=1
go test ./backend/agent -run TestBuiltinKimiModelMetadata -count=1
go build ./...
cd mobile/frontend; npm run build; cd ..\..
```

> 本任务的 Go 后端改动和 Task 2 的前端改动共用同一个提交，最终保持 4 个逻辑提交。

---

## Task 2：简化移动端和桌面端模型配置界面

**Files**

- Modify: `mobile/frontend/src/views/Settings.vue`
- Modify: `frontend/src/components/ai-config-manager.vue`

### 2.1 移动端

**Step 1：增加模型列表状态和函数**

在 `Settings.vue` 的 `showAdvancedAi` 附近增加：

```js
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
```

在 `resetAiForm` 末尾重置：

```js
modelOptions.value = []
modelKeyword.value = ''
showModelPicker.value = false
```

增加函数：

```js
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
```

**Step 2：重排编辑弹窗**

把编辑弹窗中的 `van-cell-group` 替换成下面的结构。常用区只保留名称、BaseUrl、API Key、模型名、深度思考：

```html
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
  <van-field v-model="aiForm.maxTokens" type="number" label="MaxTokens" input-align="right" />
  <van-field v-model="aiForm.contextWindow" type="number" label="上下文窗口" input-align="right" />
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
```

在 `van-popup` 之后增加模型选择弹窗：

```html
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
```

删除原来弹窗里的 `模型类型`、`MaxTokens`、`Temperature`、`超时`、`高级选项` 中重复的字段，避免出现两份。

### 2.2 桌面端

**Step 1：增加显式「获取模型」按钮**

在 `ai-config-manager.vue` 的模型名称 `n-form-item` 中，把原来的 `n-select` 改为：

```html
<n-form-item :label="editingConfig.modelType === 'embedding' ? '向量模型名称' : '模型名称'" required>
  <n-space vertical style="width: 100%" :size="8">
    <div style="display: flex; gap: 8px; width: 100%">
      <n-select
        v-model:value="editingConfig.modelName"
        :options="editingConfig._modelOptions || []"
        filterable
        tag
        :loading="editingConfig._loadingModels"
        :placeholder="editingConfig.modelType === 'embedding' ? '向量模型名，如 text-embedding-3-small / BAAI/bge-m3' : '选择模型或手动输入'"
        style="flex: 1"
        @update:value="(val) => onModelNameChange(editingConfig, val)"
      />
      <n-button
        :loading="editingConfig._loadingModels"
        @click="fetchAiModels(editingConfig)"
      >
        获取模型
      </n-button>
    </div>
    <n-text depth="3" style="font-size: 12px">
      选择模型后会自动填充输出上限和上下文窗口；也可在「高级参数」中手动覆盖。
    </n-text>
  </n-space>
</n-form-item>
```

删除 `n-select` 上的 `@click="fetchAiModels(editingConfig)"`，避免点下拉框就发请求。

**Step 2：拉取成功后刷新当前模型信息**

把 `fetchAiModels` 中 `if (!aiConfig.modelName && options.length > 0)` 这一段改为：

```js
if (options.length > 0) {
  if (!aiConfig.modelName) {
    aiConfig.modelName = options[0].value
    onModelNameChange(aiConfig, aiConfig.modelName)
  } else {
    await fetchModelInfo(aiConfig, aiConfig.modelName)
  }
  message.success(`已获取 ${options.length} 个模型`)
}
```

**Step 3：把高级字段收进折叠面板**

把 `模型类型`、`Temperature`、`MaxTokens`、`上下文窗口`、`Timeout`、`视觉理解`、`http代理`、`自定义Header` 这些 `n-form-item` 放进：

```html
<n-collapse style="margin-top: 8px">
  <n-collapse-item title="高级参数" name="advanced">
    <!-- 原来的模型类型、Temperature、MaxTokens、上下文窗口、Timeout、视觉理解、http代理、自定义Header -->
  </n-collapse-item>
</n-collapse>
```

保留「深度思考」在折叠面板外。删除原来的 `n-form-item label="深度思考"` 之前紧挨着的重复高级字段。

**Step 4：验证并提交**

```powershell
cd mobile/frontend; npm run build; cd ..\..
cd frontend; npm run build; cd ..
```

提交（把 Task 1 的后端改动一起纳入，形成第一个逻辑提交）：

```powershell
git add backend/modelinfo backend/agent/builtin_models.go backend/data/ai_models_api.go backend/data/ai_models_api_test.go app.go mobile/core/app.go mobile/core/models.go mobile/frontend/src/api.js mobile/frontend/src/views/Settings.vue frontend/src/components/ai-config-manager.vue
git commit -m "feat(ai): 模型配置支持一键获取并折叠高级参数"
```

---

## Task 3：自选左滑删除

**Files**

- Modify: `mobile/frontend/src/views/Watchlist.vue`

**Step 1：增加删除逻辑**

把 `import { showToast } from 'vant'` 改为：

```js
import { showConfirmDialog, showToast } from 'vant'
```

在 `items` 附近增加：

```js
const removing = ref('')
```

增加函数：

```js
async function removeItem(item) {
  try {
    await showConfirmDialog({
      title: '删除自选',
      message: `确定删除「${item.name || item.code}」吗？删除后该股票的价格提醒会同步停用。`
    })
  } catch {
    return
  }
  removing.value = item.code
  try {
    await api.removeStock(item.code)
    items.value = items.value.filter((x) => x.code !== item.code)
    showToast('已删除')
  } catch (e) {
    showToast(e?.message || '删除失败')
  } finally {
    removing.value = ''
  }
}
```

**Step 2：用 `van-swipe-cell` 包裹股票项**

把原来的：

```html
<div v-for="item in items" :key="item.code" class="stock-item" @click="openStock(item)">
  ...
</div>
```

改为：

```html
<van-swipe-cell v-for="item in items" :key="item.code">
  <div class="stock-item" @click="openStock(item)">
    ...
  </div>
  <template #right>
    <van-button
      square
      type="danger"
      text="删除"
      class="swipe-delete"
      :loading="removing === item.code"
      @click.stop="removeItem(item)"
    />
  </template>
</van-swipe-cell>
```

在文件末尾增加：

```html
<style scoped>
.swipe-delete {
  height: 100%;
  min-height: 64px;
}
</style>
```

**Step 3：验证并提交**

```powershell
cd mobile/frontend; npm run build; cd ..\..
```

提交：

```powershell
git add mobile/frontend/src/views/Watchlist.vue
git commit -m "feat(mobile): 自选列表支持左滑删除"
```

---

## Task 4：移动端运行时文件归拢到 `aipanyi/` 子目录

**Files**

- Create: `mobile/core/storage.go`
- Create: `mobile/core/storage_test.go`
- Modify: `mobile/core/core.go`
- Create: `backend/data/data_path.go`
- Create: `backend/data/data_path_test.go`
- Modify: `backend/data/stock_data_api.go`
- Modify: `backend/data/lhb_seat_api.go`
- Modify: `backend/data/policy_news_api.go`

### 4.1 可配置数据目录

**Step 1：先写测试**

新建 `backend/data/data_path_test.go`：

```go
package data

import (
	"path/filepath"
	"testing"
)

func TestSetDataDir(t *testing.T) {
	old := dataPath("stock_basic.json")
	defer SetDataDir(".")
	SetDataDir(filepath.Join("tmp", "aipanyi"))
	got := dataPath("stock_basic.json")
	want := filepath.Join("tmp", "aipanyi", "stock_basic.json")
	if got != want {
		t.Fatalf("dataPath() = %q, want %q (old=%q)", got, want, old)
	}
}
```

运行：

```powershell
go test ./backend/data -run TestSetDataDir -count=1
```

预期失败：`dataPath` 未定义。

**Step 2：实现**

新建 `backend/data/data_path.go`：

```go
package data

import (
	"path/filepath"
	"sync"
)

var (
	dataDirMu sync.RWMutex
	dataDir   = "."
)

// SetDataDir 设置本包写外置数据文件的根目录。
// 桌面端不调用，保持当前工作目录；移动端设为 filesDir/aipanyi。
func SetDataDir(dir string) {
	if dir == "" {
		dir = "."
	}
	dataDirMu.Lock()
	dataDir = dir
	dataDirMu.Unlock()
}

func dataPath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	dataDirMu.RLock()
	root := dataDir
	dataDirMu.RUnlock()
	return filepath.Join(root, name)
}
```

**Step 3：替换已知写入点**

`backend/data/stock_data_api.go`：

```go
// 原来：
// ioutil.WriteFile("stock_basic.json", resp.Body(), 0666)
// 改为：
stockBasicPath := dataPath("stock_basic.json")
_ = os.MkdirAll(filepath.Dir(stockBasicPath), 0755)
ioutil.WriteFile(stockBasicPath, resp.Body(), 0666)
```

如果文件没有 `path/filepath` 导入，补上：

```go
"path/filepath"
```

`backend/data/lhb_seat_api.go` 增加：

```go
func hotMoneySeatsPath() string { return dataPath(hotMoneySeatsFile) }
```

把该文件中所有文件操作里的 `hotMoneySeatsFile` 替换为 `hotMoneySeatsPath()`，包括：

- `os.ReadFile`
- `os.WriteFile`
- `os.Remove`
- `os.MkdirAll(filepath.Dir(...))`

保留 `const hotMoneySeatsFile = "data/hot_money_seats.json"` 和测试中对它的引用不变。

`backend/data/policy_news_api.go` 增加：

```go
func keyDepartmentsPath() string { return dataPath(keyDepartmentsFile) }
```

把该文件中所有 `keyDepartmentsFile` 的文件操作替换为 `keyDepartmentsPath()`。

**Step 4：运行测试**

```powershell
go test ./backend/data -run 'TestSetDataDir|TestFetchAiModel' -count=1
go build ./...
```

### 4.2 移动端启动目录与迁移

**Step 1：先写迁移测试**

新建 `mobile/core/storage_test.go`：

```go
package mobilecore

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyDataMovesKnownFiles(t *testing.T) {
	filesDir := t.TempDir()
	rootDir := filepath.Join(filesDir, "aipanyi")
	if err := os.MkdirAll(filepath.Join(rootDir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(filesDir, "aipanyi.db"), "db")
	writeTestFile(t, filepath.Join(filesDir, "logs", "info.log"), "old log")
	writeTestFile(t, filepath.Join(filesDir, "data", "hot_money_seats.json"), "seats")
	writeTestFile(t, filepath.Join(filesDir, "stock_basic.json"), "basic")
	writeTestFile(t, filepath.Join(rootDir, "logs", "runtime.log"), "runtime")

	stats := migrateLegacyData(filesDir, rootDir)
	if len(stats.errors) != 0 {
		t.Fatalf("migration errors: %v", stats.errors)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "aipanyi.db")); got != "db" {
		t.Fatalf("db = %q", got)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "logs", "info.log")); got != "old log" {
		t.Fatalf("info.log = %q", got)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "data", "hot_money_seats.json")); got != "seats" {
		t.Fatalf("seats = %q", got)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "stock_basic.json")); got != "basic" {
		t.Fatalf("stock_basic = %q", got)
	}
	if got := readTestFile(t, filepath.Join(rootDir, "logs", "runtime.log")); got != "runtime" {
		t.Fatalf("runtime.log = %q", got)
	}
	if _, err := os.Stat(filepath.Join(filesDir, "aipanyi.db")); !os.IsNotExist(err) {
		t.Fatalf("old db still exists: %v", err)
	}
}

func TestMigrateLegacyDataDoesNotOverwrite(t *testing.T) {
	filesDir := t.TempDir()
	rootDir := filepath.Join(filesDir, "aipanyi")
	writeTestFile(t, filepath.Join(filesDir, "aipanyi.db"), "old")
	writeTestFile(t, filepath.Join(rootDir, "aipanyi.db"), "new")

	stats := migrateLegacyData(filesDir, rootDir)
	if stats.skipped == 0 {
		t.Fatal("expected skipped file")
	}
	if got := readTestFile(t, filepath.Join(rootDir, "aipanyi.db")); got != "new" {
		t.Fatalf("target overwritten: %q", got)
	}
	if got := readTestFile(t, filepath.Join(filesDir, "aipanyi.db")); got != "old" {
		t.Fatalf("source changed: %q", got)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
```

运行：

```powershell
go test ./mobile/core -run TestMigrateLegacyData -count=1
```

预期失败：`migrateLegacyData` 未定义。

**Step 2：实现存储迁移**

新建 `mobile/core/storage.go`：

```go
package mobilecore

import (
	"fmt"
	"os"
	"path/filepath"
)

type migrationStats struct {
	moved   int
	skipped int
	errors  []string
}

func prepareDataRoot(dataDir string) (string, migrationStats, error) {
	rootDir := filepath.Join(dataDir, "aipanyi")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return "", migrationStats{}, fmt.Errorf("创建应用数据目录失败: %w", err)
	}
	return rootDir, migrateLegacyData(dataDir, rootDir), nil
}

func migrateLegacyData(dataDir, rootDir string) migrationStats {
	var stats migrationStats
	files := []struct{ src, dst string }{
		{filepath.Join(dataDir, "aipanyi.db"), filepath.Join(rootDir, "aipanyi.db")},
		{filepath.Join(dataDir, "aipanyi.db-wal"), filepath.Join(rootDir, "aipanyi.db-wal")},
		{filepath.Join(dataDir, "aipanyi.db-shm"), filepath.Join(rootDir, "aipanyi.db-shm")},
		{filepath.Join(dataDir, "stock_basic.json"), filepath.Join(rootDir, "stock_basic.json")},
		{filepath.Join(dataDir, "hot_money_seats.json"), filepath.Join(rootDir, "data", "hot_money_seats.json")},
		{filepath.Join(dataDir, "key_departments.json"), filepath.Join(rootDir, "data", "key_departments.json")},
	}
	for _, f := range files {
		moved, skipped, err := moveLegacyFile(f.src, f.dst)
		if moved {
			stats.moved++
		}
		if skipped {
			stats.skipped++
		}
		if err != nil {
			stats.errors = append(stats.errors, err.Error())
		}
	}

	for _, d := range []struct{ src, dst string }{
		{filepath.Join(dataDir, "logs"), filepath.Join(rootDir, "logs")},
		{filepath.Join(dataDir, "data"), filepath.Join(rootDir, "data")},
	} {
		moved, skipped, errs := mergeLegacyDir(d.src, d.dst)
		stats.moved += moved
		stats.skipped += skipped
		stats.errors = append(stats.errors, errs...)
	}
	return stats
}

func moveLegacyFile(src, dst string) (bool, bool, error) {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("检查旧文件 %s 失败: %w", src, err)
	}
	if _, err := os.Stat(dst); err == nil {
		return false, true, nil
	} else if !os.IsNotExist(err) {
		return false, false, fmt.Errorf("检查目标文件 %s 失败: %w", dst, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, false, fmt.Errorf("创建目标目录 %s 失败: %w", filepath.Dir(dst), err)
	}
	if err := os.Rename(src, dst); err != nil {
		return false, false, fmt.Errorf("迁移 %s -> %s 失败: %w", src, dst, err)
	}
	return true, false, nil
}

func mergeLegacyDir(src, dst string) (int, int, []string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, []string{fmt.Sprintf("读取旧目录 %s 失败: %v", src, err)}
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0, 0, []string{fmt.Sprintf("创建目标目录 %s 失败: %v", dst, err)}
	}

	moved, skipped := 0, 0
	var errs []string
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			m, k, e := mergeLegacyDir(s, d)
			moved += m
			skipped += k
			errs = append(errs, e...)
			_ = os.Remove(s)
			continue
		}
		m, k, err := moveLegacyFile(s, d)
		if m {
			moved++
		}
		if k {
			skipped++
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	_ = os.Remove(src)
	return moved, skipped, errs
}
```

**Step 3：修改 `mobile/core/core.go`**

在 `Start` 中把原来的目录/日志/数据库初始化替换为：

```go
rootDir, stats, err := prepareDataRoot(dataDir)
if err != nil {
	return "", err
}
logger.SetLogDir(filepath.Join(rootDir, "logs"))
logger.SugaredLogger.Infof("mobile: core start dataDir=%s root=%s", dataDir, rootDir)
if stats.moved > 0 || stats.skipped > 0 || len(stats.errors) > 0 {
	logger.SugaredLogger.Infof(
		"mobile: migrate legacy data moved=%d skipped=%d errors=%d",
		stats.moved, stats.skipped, len(stats.errors),
	)
	for _, msg := range stats.errors {
		logger.SugaredLogger.Warnf("mobile: migrate legacy data: %s", msg)
	}
}
data.SetDataDir(rootDir)

if !dbReady {
	db.InitWithOptions(filepath.Join(rootDir, "aipanyi.db"), db.MobileOptions())
	...
}
```

删除原来的：

```go
if err := os.MkdirAll(dataDir, 0o755); err != nil { ... }
logger.SetLogDir(filepath.Join(dataDir, "logs"))
db.InitWithOptions(filepath.Join(dataDir, "aipanyi.db"), ...)
```

`os` 仍然需要用于 `os.Stdout`，不要删除导入。

在 `Stop` 开头增加：

```go
logger.SugaredLogger.Info("mobile: core stop")
```

**Step 4：运行测试和构建**

```powershell
go test ./mobile/core -run TestMigrateLegacyData -count=1
go test ./backend/data -run 'TestSetDataDir|TestFetchAiModel' -count=1
go build ./...
go build -tags mobileassets ./mobile/...
```

**Step 5：提交**

```powershell
git add mobile/core/storage.go mobile/core/storage_test.go mobile/core/core.go backend/data/data_path.go backend/data/data_path_test.go backend/data/stock_data_api.go backend/data/lhb_seat_api.go backend/data/policy_news_api.go
git commit -m "refactor(mobile): 运行时文件归拢到应用私有子目录"
```

---

## Task 5：交易窗口计算

**Files**

- Modify: `backend/marketcal/marketcal.go`
- Create: `backend/marketcal/monitor_window_test.go`

**Step 1：先写测试**

新建 `backend/marketcal/monitor_window_test.go`：

```go
package marketcal

import (
	"testing"
	"time"
)

func TestMergeMonitorWindows(t *testing.T) {
	base := time.Date(2026, 11, 2, 9, 0, 0, 0, shanghai)
	in := []MonitorWindow{
		{Start: base, End: base.Add(2 * time.Hour)},
		{Start: base.Add(time.Hour), End: base.Add(3 * time.Hour)},
		{Start: base.Add(3 * time.Hour), End: base.Add(4 * time.Hour)},
		{Start: base.Add(5 * time.Hour), End: base.Add(6 * time.Hour)},
	}
	got := mergeMonitorWindows(in)
	if len(got) != 2 {
		t.Fatalf("mergeMonitorWindows() len = %d, want 2: %#v", len(got), got)
	}
	if !got[0].Start.Equal(base) || !got[0].End.Equal(base.Add(4*time.Hour)) {
		t.Fatalf("first window = %#v", got[0])
	}
	if !got[1].Start.Equal(base.Add(5*time.Hour)) || !got[1].End.Equal(base.Add(6*time.Hour)) {
		t.Fatalf("second window = %#v", got[1])
	}
}

func TestNextSessionSkipsNonTradingDays(t *testing.T) {
	loc := shanghai
	now := time.Date(2026, 11, 6, 16, 0, 0, 0, loc) // Friday 16:00
	isTradingDay := func(d time.Time) bool {
		return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday
	}
	start, end, ok := nextSession(now, loc, 9, 15, 15, 0, isTradingDay)
	if !ok {
		t.Fatal("nextSession() ok = false")
	}
	wantStart := time.Date(2026, 11, 9, 9, 15, 0, 0, loc)
	wantEnd := time.Date(2026, 11, 9, 15, 0, 0, 0, loc)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("nextSession() = %s ~ %s, want %s ~ %s", start, end, wantStart, wantEnd)
	}
}

func TestNextSessionInsideWindowKeepsStart(t *testing.T) {
	loc := shanghai
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, loc)
	isTradingDay := func(time.Time) bool { return true }
	start, end, ok := nextSession(now, loc, 9, 15, 15, 0, isTradingDay)
	if !ok {
		t.Fatal("nextSession() ok = false")
	}
	if !start.Before(now) || !end.After(now) {
		t.Fatalf("expected window containing now, got %s ~ %s", start, end)
	}
}
```

运行：

```powershell
go test ./backend/marketcal -run 'TestMergeMonitorWindows|TestNextSession' -count=1
```

预期失败：`MonitorWindow`、`mergeMonitorWindows`、`nextSession` 未定义。

**Step 2：实现窗口计算**

在 `backend/marketcal/marketcal.go` 增加 `sort` 导入和以下代码：

```go
// MonitorWindow 一个连续的监控窗口。
type MonitorWindow struct {
	Start time.Time
	End   time.Time
}

var newYork = func() *time.Location {
	if loc, err := time.LoadLocation("America/New_York"); err == nil {
		return loc
	}
	return time.FixedZone("EST", -5*60*60)
}()

// IsHKTradingDay 导出港股交易日判断，供监控窗口计算使用。
func IsHKTradingDay(date time.Time) bool { return isHKTradingDay(date) }

// IsUSTradingDay 导出美股交易日判断，内部先转纽约时间。
func IsUSTradingDay(date time.Time) bool {
	return isUSTradingDay(date.In(newYork))
}

func nextSession(
	now time.Time,
	loc *time.Location,
	startHour, startMin, endHour, endMin int,
	isTradingDay func(time.Time) bool,
) (time.Time, time.Time, bool) {
	now = now.In(loc)
	for i := 0; i <= 10; i++ {
		day := now.AddDate(0, 0, i)
		y, m, d := day.Date()
		start := time.Date(y, m, d, startHour, startMin, 0, 0, loc)
		end := time.Date(y, m, d, endHour, endMin, 0, 0, loc)
		if !end.After(now) {
			continue
		}
		if !isTradingDay(day) {
			continue
		}
		return start, end, true
	}
	return time.Time{}, time.Time{}, false
}

// NextTradingSession 返回某市场当前或下一个交易窗口。
func NextTradingSession(market string, now time.Time) (time.Time, time.Time, bool) {
	switch market {
	case "HK":
		return nextSession(now, shanghai, 9, 0, 16, 10, IsHKTradingDay)
	case "US":
		return nextSession(now, newYork, 4, 0, 20, 0, IsUSTradingDay)
	default:
		return nextSession(now, shanghai, 9, 15, 15, 0, IsTradingDay)
	}
}

func mergeMonitorWindows(in []MonitorWindow) []MonitorWindow {
	if len(in) == 0 {
		return nil
	}
	windows := append([]MonitorWindow(nil), in...)
	sort.Slice(windows, func(i, j int) bool {
		return windows[i].Start.Before(windows[j].Start)
	})
	out := []MonitorWindow{windows[0]}
	for _, w := range windows[1:] {
		last := &out[len(out)-1]
		if !w.Start.After(last.End) {
			if w.End.After(last.End) {
				last.End = w.End
			}
			continue
		}
		out = append(out, w)
	}
	return out
}

// NextMonitorWindow 返回包含 now 的窗口；都不包含时返回最早开始的未来窗口。
func NextMonitorWindow(markets []string, now time.Time) (MonitorWindow, bool) {
	windows := make([]MonitorWindow, 0, len(markets))
	seen := make(map[string]bool, len(markets))
	for _, market := range markets {
		market = strings.ToUpper(strings.TrimSpace(market))
		if market == "" || seen[market] {
			continue
		}
		seen[market] = true
		start, end, ok := NextTradingSession(market, now)
		if ok {
			windows = append(windows, MonitorWindow{Start: start, End: end})
		}
	}
	if len(windows) == 0 {
		return MonitorWindow{}, false
	}
	merged := mergeMonitorWindows(windows)
	for _, w := range merged {
		if !w.Start.After(now) && !w.End.Before(now) {
			return w, true
		}
	}
	for _, w := range merged {
		if w.Start.After(now) {
			return w, true
		}
	}
	return MonitorWindow{}, false
}

// NextMonitorWindowAfter 返回 after 之后的下一个窗口；after 在窗口内时从该窗口结束处继续找。
func NextMonitorWindowAfter(markets []string, after time.Time) (MonitorWindow, bool) {
	w, ok := NextMonitorWindow(markets, after)
	if !ok {
		return MonitorWindow{}, false
	}
	if w.Start.After(after) {
		return w, true
	}
	return NextMonitorWindow(markets, w.End)
}
```

**Step 3：运行测试**

```powershell
go test ./backend/marketcal -count=1
go build ./...
```

**Step 4：验证（合并到 Task 8 的后台提交）**

```powershell
go test ./backend/marketcal -count=1
go build ./...
```

> 交易窗口计算属于后台调度的一部分，不单独提交；Task 5-8 的改动合并为第 4 个逻辑提交。

---

## Task 6：Go 核心下发监控调度

**Files**

- Modify: `mobile/core/notifier.go`
- Create: `mobile/core/monitor_schedule.go`
- Modify: `mobile/core/core.go`
- Modify: `mobile/core/alert_store.go`
- Modify: `mobile/core/watchlist.go`

**Step 1：扩展 Notifier**

把 `mobile/core/notifier.go` 的接口改为：

```go
type Notifier interface {
	Notify(title, body, payload string)
	UpdateMonitorStatus(active int32)
	// UpdateMonitorSchedule 下发当前/下一个监控窗口，Unix 毫秒；0 表示没有。
	// int64 在 gomobile 中映射成 Java long，避免 2038 年问题。
	UpdateMonitorSchedule(startMs, endMs, nextStartMs int64)
}
```

增加发送函数：

```go
func sendMonitorSchedule(startMs, endMs, nextStartMs int64) {
	n := currentNotifier()
	if n == nil {
		return
	}
	defer func() { _ = recover() }()
	n.UpdateMonitorSchedule(startMs, endMs, nextStartMs)
}
```

**Step 2：实现调度刷新**

新建 `mobile/core/monitor_schedule.go`：

```go
package mobilecore

import (
	"sync"
	"time"

	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/backend/marketcal"
)

var (
	scheduleMu      sync.Mutex
	schedulePending bool
)

func scheduleMonitorRefresh() {
	if db.Dao == nil {
		return
	}
	scheduleMu.Lock()
	if schedulePending {
		scheduleMu.Unlock()
		return
	}
	schedulePending = true
	scheduleMu.Unlock()

	go func() {
		defer func() {
			scheduleMu.Lock()
			schedulePending = false
			scheduleMu.Unlock()
		}()
		refreshMonitorSchedule()
	}()
}

func enabledAlertMarkets() ([]string, error) {
	var markets []string
	err := db.Dao.Model(&priceAlert{}).
		Where("enabled = ?", true).
		Select("DISTINCT market").
		Pluck("market", &markets).Error
	return markets, err
}

func refreshMonitorSchedule() {
	markets, err := enabledAlertMarkets()
	if err != nil {
		logger.SugaredLogger.Errorf("mobile: list enabled alert markets failed: %v", err)
		return
	}
	if len(markets) == 0 {
		sendMonitorSchedule(0, 0, 0)
		logger.SugaredLogger.Info("mobile: monitor schedule cleared (no enabled alerts)")
		return
	}

	now := time.Now()
	current, ok := marketcal.NextMonitorWindow(markets, now)
	if !ok {
		sendMonitorSchedule(0, 0, 0)
		logger.SugaredLogger.Infof("mobile: monitor schedule cleared (no window) markets=%v", markets)
		return
	}

	var nextStart int64
	if next, ok := marketcal.NextMonitorWindowAfter(markets, current.End); ok {
		nextStart = next.Start.UnixMilli()
	}

	sendMonitorSchedule(current.Start.UnixMilli(), current.End.UnixMilli(), nextStart)
	logger.SugaredLogger.Infof(
		"mobile: monitor schedule markets=%v start=%s end=%s next=%s",
		markets,
		formatScheduleTime(current.Start.UnixMilli()),
		formatScheduleTime(current.End.UnixMilli()),
		formatScheduleTime(nextStart),
	)
}

func formatScheduleTime(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).In(time.Local).Format("2006-01-02 15:04:05")
}
```

**Step 3：在启动和提醒变更时触发**

在 `mobile/core/core.go` 的 `Start` 中，`started = true` 之后增加：

```go
scheduleMonitorRefresh()
```

在 `Start` 的幂等分支：

```go
if started && server != nil {
	setNotifier(n)
	scheduleMonitorRefresh()
	return server.URL(), nil
}
```

在 `mobile/core/alert_store.go` 中：

- `saveAlert` 更新分支：`updateMonitorStatus(lenAlertsEnabled())` 后增加 `scheduleMonitorRefresh()`。
- `saveAlert` 新建分支：`updateMonitorStatus(lenAlertsEnabled())` 后增加 `scheduleMonitorRefresh()`。
- `deleteAlert`：`updateMonitorStatus(...)` 后增加 `scheduleMonitorRefresh()`。
- `setAlertEnabled`：`updateMonitorStatus(...)` 后增加 `scheduleMonitorRefresh()`。

在 `mobile/core/watchlist.go` 的 `removeStock` 中：

```go
_ = db.Dao.Model(&priceAlert{}).Where("stock_code = ?", norm).Update("enabled", false).Error
updateMonitorStatus(lenAlertsEnabled())
scheduleMonitorRefresh()
```

**Step 4：验证**

```powershell
go build ./...
go build -tags mobileassets ./mobile/...
```

> 此步骤后 `mobilecore.Notifier` 已变更，旧 AAR 不再匹配，暂时不要在 Android Studio 编译；Task 7 完成后再统一重建 AAR。

**Step 5：验证（合并到 Task 8 的后台提交）**

```powershell
git add mobile/core/notifier.go mobile/core/monitor_schedule.go mobile/core/core.go mobile/core/alert_store.go mobile/core/watchlist.go
# 不单独 commit；等待 Android 壳和设置页完成后统一提交。
```

---

## Task 7：Android 调度、服务生命周期与运行日志

**Files**

- Create: `mobile/android/app/src/main/java/com/aipanyi/mobile/RuntimeLog.kt`
- Create: `mobile/android/app/src/main/java/com/aipanyi/mobile/AiPanyiApp.kt`
- Create: `mobile/android/app/src/main/java/com/aipanyi/mobile/MonitorScheduler.kt`
- Create: `mobile/android/app/src/main/java/com/aipanyi/mobile/MonitorAlarmReceiver.kt`
- Modify: `mobile/android/app/src/main/java/com/aipanyi/mobile/MonitorService.kt`
- Modify: `mobile/android/app/src/main/java/com/aipanyi/mobile/MainActivity.kt`
- Modify: `mobile/android/app/src/main/java/com/aipanyi/mobile/BootReceiver.kt`
- Modify: `mobile/android/app/src/main/java/com/aipanyi/mobile/AppNotifier.kt`
- Modify: `mobile/android/app/src/main/java/com/aipanyi/mobile/NativeBridge.kt`
- Modify: `mobile/android/app/src/main/AndroidManifest.xml`

### 7.1 运行日志

**Step 1：新建 `RuntimeLog.kt`**

```kotlin
package com.aipanyi.mobile

import android.content.Context
import android.util.Log
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/** 应用私有运行日志，文件位置：filesDir/aipanyi/logs/runtime.log。 */
object RuntimeLog {
    private const val TAG = "AiPanyiRuntime"
    private const val MAX_BYTES = 512 * 1024L
    private const val KEEP_FILES = 3

    @Volatile
    private var dir: File? = null
    private val lock = Any()

    fun init(context: Context) {
        if (dir != null) return
        synchronized(lock) {
            if (dir == null) {
                dir = File(context.filesDir, "aipanyi/logs").apply { mkdirs() }
            }
        }
    }

    fun info(tag: String, message: String) = write("I", tag, message, null)
    fun warn(tag: String, message: String, tr: Throwable? = null) = write("W", tag, message, tr)
    fun error(tag: String, message: String, tr: Throwable? = null) = write("E", tag, message, tr)

    private fun write(level: String, tag: String, message: String, tr: Throwable?) {
        val time = SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.US).format(Date())
        val line = buildString {
            append(time).append(' ').append(level).append('/').append(tag).append(' ').append(message)
            if (tr != null) {
                append(": ").append(tr.javaClass.simpleName).append(' ').append(tr.message ?: "")
            }
        }
        when (level) {
            "E" -> Log.e(TAG, line, tr)
            "W" -> Log.w(TAG, line, tr)
            else -> Log.i(TAG, line)
        }
        val logDir = dir ?: return
        synchronized(lock) {
            try {
                val file = File(logDir, "runtime.log")
                if (file.length() >= MAX_BYTES) rotate(logDir)
                file.appendText(line + "\n")
            } catch (_: Throwable) {
                // 日志失败不能影响业务。
            }
        }
    }

    private fun rotate(logDir: File) {
        File(logDir, "runtime.log.${KEEP_FILES - 1}").delete()
        for (i in KEEP_FILES - 2 downTo 1) {
            val from = File(logDir, "runtime.log.$i")
            if (from.exists()) from.renameTo(File(logDir, "runtime.log.${i + 1}"))
        }
        File(logDir, "runtime.log").renameTo(File(logDir, "runtime.log.1"))
    }
}
```

**Step 2：新建 `AiPanyiApp.kt`**

```kotlin
package com.aipanyi.mobile

import android.app.Application
import android.os.Process

class AiPanyiApp : Application() {
    override fun onCreate() {
        super.onCreate()
        RuntimeLog.init(this)
        RuntimeLog.info("AiPanyiApp", "process start pid=${Process.myPid()}")
    }
}
```

在 `AndroidManifest.xml` 的 `<application>` 增加 `android:name=".AiPanyiApp"`。

### 7.2 调度器

**Step 1：扩展 `AppNotifier` 常量**

```kotlin
companion object {
    const val PREFS = "aipanyi"
    const val KEY_ACTIVE = "active_alerts"
    const val KEY_URL = "local_url"
    const val KEY_WINDOW_START = "monitor_window_start_ms"
    const val KEY_WINDOW_END = "monitor_window_end_ms"
    const val KEY_NEXT_START = "monitor_next_start_ms"
    const val ACTION_CORE_READY = "com.aipanyi.mobile.CORE_READY"
}
```

**Step 2：新建 `MonitorScheduler.kt`**

```kotlin
package com.aipanyi.mobile

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.content.ContextCompat
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

object MonitorScheduler {
    private const val REQ_START = 1001

    fun applySchedule(context: Context, startMs: Long, endMs: Long, nextStartMs: Long) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        prefs.edit()
            .putLong(AppNotifier.KEY_WINDOW_START, startMs)
            .putLong(AppNotifier.KEY_WINDOW_END, endMs)
            .putLong(AppNotifier.KEY_NEXT_START, nextStartMs)
            .apply()

        val now = System.currentTimeMillis()
        RuntimeLog.info(
            "MonitorScheduler",
            "apply schedule start=${format(startMs)} end=${format(endMs)} next=${format(nextStartMs)} now=${format(now)}"
        )

        val active = startMs > 0 && startMs <= now && now < endMs
        if (active) {
            startMonitorService(context, "schedule_active")
            if (nextStartMs > now) scheduleStart(context, nextStartMs) else cancelStartAlarm(context)
        } else if (startMs > now) {
            scheduleStart(context, startMs)
        } else if (nextStartMs > now) {
            scheduleStart(context, nextStartMs)
        } else {
            cancelStartAlarm(context)
        }
    }

    fun scheduleNext(context: Context, reason: String) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val next = prefs.getLong(AppNotifier.KEY_NEXT_START, 0)
        val now = System.currentTimeMillis()
        val at = when {
            start > now -> start
            next > now -> next
            else -> 0L
        }
        if (at > 0) scheduleStart(context, at) else cancelStartAlarm(context)
        RuntimeLog.info("MonitorScheduler", "scheduleNext reason=$reason at=${format(at)}")
    }

    fun resumeAfterBoot(context: Context) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val end = prefs.getLong(AppNotifier.KEY_WINDOW_END, 0)
        val now = System.currentTimeMillis()
        if (start > 0 && start <= now && now < end) {
            RuntimeLog.info("MonitorScheduler", "boot inside window start=${format(start)} end=${format(end)}")
            startMonitorService(context, "boot_active")
            return
        }
        scheduleNext(context, "boot")
    }

    fun isInWindow(context: Context): Boolean {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val end = prefs.getLong(AppNotifier.KEY_WINDOW_END, 0)
        val now = System.currentTimeMillis()
        return start > 0 && start <= now && now < end
    }

    fun canScheduleExactAlarms(context: Context): Boolean {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S) return true
        val am = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        return am.canScheduleExactAlarms()
    }

    fun startMonitorService(context: Context, reason: String, action: String? = null) {
        try {
            val intent = Intent(context, MonitorService::class.java)
            if (action != null) intent.action = action
            ContextCompat.startForegroundService(context, intent)
            RuntimeLog.info("MonitorScheduler", "start service requested reason=$reason action=${action ?: "-"}")
        } catch (t: Throwable) {
            RuntimeLog.error("MonitorScheduler", "start service failed reason=$reason action=${action ?: "-"}", t)
            try {
                NotificationHelper.notifyPriceAlert(
                    context,
                    "AI盘译",
                    "后台监控启动失败，点击打开应用恢复",
                    ""
                )
            } catch (_: Throwable) {
            }
        }
    }

    private fun scheduleStart(context: Context, atMs: Long) {
        val am = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        val intent = Intent(context, MonitorAlarmReceiver::class.java)
            .setAction(MonitorAlarmReceiver.ACTION_ALARM)
        val pi = PendingIntent.getBroadcast(
            context,
            REQ_START,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val exact = canScheduleExactAlarms(context)
        try {
            if (exact) {
                am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, atMs, pi)
            } else {
                am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, atMs, pi)
            }
            RuntimeLog.info("MonitorScheduler", "alarm scheduled at=${format(atMs)} exact=$exact")
        } catch (t: Throwable) {
            RuntimeLog.error("MonitorScheduler", "alarm schedule failed at=${format(atMs)} exact=$exact", t)
            try {
                am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, atMs, pi)
                RuntimeLog.info("MonitorScheduler", "alarm fallback to inexact at=${format(atMs)}")
            } catch (fallback: Throwable) {
                RuntimeLog.error("MonitorScheduler", "alarm fallback failed", fallback)
            }
        }
    }

    private fun cancelStartAlarm(context: Context) {
        val am = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        val intent = Intent(context, MonitorAlarmReceiver::class.java)
            .setAction(MonitorAlarmReceiver.ACTION_ALARM)
        val pi = PendingIntent.getBroadcast(
            context,
            REQ_START,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        am.cancel(pi)
        RuntimeLog.info("MonitorScheduler", "alarm cancelled")
    }

    private fun format(ms: Long): String {
        if (ms <= 0) return "-"
        return SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(Date(ms))
    }
}
```

**Step 3：新建 `MonitorAlarmReceiver.kt`**

```kotlin
package com.aipanyi.mobile

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class MonitorAlarmReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        RuntimeLog.init(context)
        if (intent?.action != ACTION_ALARM) return
        RuntimeLog.info("MonitorAlarmReceiver", "alarm fired at=${System.currentTimeMillis()}")
        MonitorScheduler.startMonitorService(context, "alarm", MonitorService.ACTION_ALARM_START)
    }

    companion object {
        const val ACTION_ALARM = "com.aipanyi.mobile.MONITOR_ALARM"
    }
}
```

**Step 4：实现 `AppNotifier.updateMonitorSchedule`**

在 `AppNotifier.kt` 增加：

```kotlin
override fun updateMonitorSchedule(startMs: Long, endMs: Long, nextStartMs: Long) {
    RuntimeLog.info(
        "AppNotifier",
        "schedule from go start=$startMs end=$endMs next=$nextStartMs"
    )
    MonitorScheduler.applySchedule(context, startMs, endMs, nextStartMs)
    MonitorService.onScheduleChanged()
}

override fun updateMonitorStatus(active: Int) {
    RuntimeLog.info("AppNotifier", "active alerts=$active")
    context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        .edit()
        .putInt(KEY_ACTIVE, active)
        .apply()
    NotificationHelper.updateOngoing(context, active)
}
```

### 7.3 服务与 Activity 生命周期

**Step 1：替换 `MonitorService.kt`**

完整替换为：

```kotlin
package com.aipanyi.mobile

import android.app.Service
import android.content.Intent
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log

class MonitorService : Service() {
    private val notifier by lazy { AppNotifier(applicationContext) }
    private val handler = Handler(Looper.getMainLooper())

    @Volatile
    private var coreUrl: String? = null

    @Volatile
    private var uiVisible = false

    @Volatile
    private var stopping = false

    @Volatile
    private var waitingForSchedule = false

    private var stopRunnable: Runnable? = null
    private var alarmTimeout: Runnable? = null

    override fun onCreate() {
        super.onCreate()
        RuntimeLog.init(this)
        instance = this
        isRunning = true
        RuntimeLog.info(TAG, "service onCreate")
        NotificationHelper.ensureChannels(this)
        startForeground(
            NotificationHelper.ONGOING_ID,
            NotificationHelper.buildOngoing(this, activeCount())
        )

        Thread {
            try {
                RuntimeLog.info(TAG, "core starting dataDir=${filesDir.absolutePath}")
                val url = mobilecore.Mobilecore.start(filesDir.absolutePath, notifier)
                if (stopping) {
                    RuntimeLog.info(TAG, "core started after stop request, stopping core")
                    mobilecore.Mobilecore.stop()
                    return@Thread
                }
                coreUrl = url
                getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
                    .edit()
                    .putString(AppNotifier.KEY_URL, url)
                    .commit()
                RuntimeLog.info(TAG, "core started url=$url")
                sendCoreReady(url)
                if (!waitingForSchedule) {
                    scheduleStopFromPrefs()
                }
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "start mobile core failed", t)
                Log.e(TAG, "start mobile core failed", t)
            }
        }.start()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        RuntimeLog.init(this)
        val action = intent?.action ?: "null"
        RuntimeLog.info(TAG, "onStartCommand action=$action uiVisible=$uiVisible")

        when (action) {
            ACTION_UI_VISIBLE -> {
                uiVisible = true
                RuntimeLog.info(TAG, "ui visible")
            }
            ACTION_UI_HIDDEN -> {
                uiVisible = false
                RuntimeLog.info(TAG, "ui hidden")
                stopIfIdle("ui_hidden")
            }
            ACTION_EXIT -> {
                RuntimeLog.info(TAG, "manual exit action")
                stopForExit()
                return START_NOT_STICKY
            }
            ACTION_SCHEDULE_CHANGED -> {
                scheduleStopFromPrefs()
                stopIfIdle("schedule_changed")
            }
            ACTION_ALARM_START -> {
                // 这个闹钟是给“下一个窗口”用的；此时 SharedPreferences 里仍是上一个窗口的
                // start/end，不能按旧窗口立即停止。等 Go 核心启动/刷新后通过
                // UpdateMonitorSchedule 写入新窗口。
                stopRunnable?.let { handler.removeCallbacks(it) }
                stopRunnable = null
                uiVisible = false
                waitingForSchedule = true
                RuntimeLog.info(TAG, "alarm start; waiting for Go schedule")
                startAlarmTimeout()
                refreshCoreForAlarm()
            }
            else -> {
                scheduleStopFromPrefs()
                stopIfIdle("sticky_restart")
            }
        }

        coreUrl?.let { sendCoreReady(it) }
        if (!waitingForSchedule) {
            scheduleStopFromPrefs()
        }
        return START_STICKY
    }

    private fun refreshCoreForAlarm() {
        Thread {
            try {
                RuntimeLog.info(TAG, "refresh core for alarm")
                val url = mobilecore.Mobilecore.start(filesDir.absolutePath, notifier)
                if (stopping) {
                    mobilecore.Mobilecore.stop()
                    return@Thread
                }
                coreUrl = url
                getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
                    .edit()
                    .putString(AppNotifier.KEY_URL, url)
                    .commit()
                sendCoreReady(url)
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "refresh core for alarm failed", t)
            }
        }.start()
    }

    private fun startAlarmTimeout() {
        alarmTimeout?.let { handler.removeCallbacks(it) }
        val runnable = Runnable {
            if (waitingForSchedule) {
                waitingForSchedule = false
                RuntimeLog.warn(TAG, "no schedule received after alarm, stopping")
                stopIfIdle("alarm_schedule_timeout")
            }
        }
        alarmTimeout = runnable
        handler.postDelayed(runnable, 120_000L)
    }

    private fun scheduleStopFromPrefs() {
        if (waitingForSchedule) return
        val prefs = getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
        val end = prefs.getLong(AppNotifier.KEY_WINDOW_END, 0)
        stopRunnable?.let { handler.removeCallbacks(it) }
        stopRunnable = null
        if (end <= 0) return

        val delay = end - System.currentTimeMillis()
        val runnable = Runnable {
            RuntimeLog.info(TAG, "window end reached end=${formatTime(end)}")
            stopIfIdle("window_end")
        }
        stopRunnable = runnable
        if (delay <= 0) {
            RuntimeLog.info(TAG, "window already ended")
            handler.post(runnable)
        } else {
            RuntimeLog.info(TAG, "stop timer scheduled delayMs=$delay")
            handler.postDelayed(runnable, delay)
        }
    }

    private fun stopIfIdle(reason: String) {
        if (uiVisible) {
            RuntimeLog.info(TAG, "keep service reason=$reason uiVisible=true")
            return
        }
        if (MonitorScheduler.isInWindow(this)) {
            RuntimeLog.info(TAG, "keep service reason=$reason inside window")
            return
        }
        stopInternal(reason)
    }

    private fun stopForExit() {
        stopInternal("manual_exit")
    }

    private fun stopInternal(reason: String) {
        if (stopping) return
        stopping = true
        waitingForSchedule = false
        alarmTimeout?.let { handler.removeCallbacks(it) }
        alarmTimeout = null
        RuntimeLog.info(TAG, "stop service reason=$reason")
        try {
            mobilecore.Mobilecore.stop()
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "stop mobile core failed", t)
        }
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun sendCoreReady(url: String) {
        try {
            val ready = Intent(AppNotifier.ACTION_CORE_READY)
                .setPackage(packageName)
                .putExtra(AppNotifier.KEY_URL, url)
            sendBroadcast(ready)
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "send core ready failed", t)
        }
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        RuntimeLog.info(TAG, "onTaskRemoved uiVisible=$uiVisible")
        stopIfIdle("task_removed")
        super.onTaskRemoved(rootIntent)
    }

    override fun onDestroy() {
        RuntimeLog.info(TAG, "service onDestroy")
        stopRunnable?.let { handler.removeCallbacks(it) }
        alarmTimeout?.let { handler.removeCallbacks(it) }
        handler.removeCallbacksAndMessages(null)
        try {
            mobilecore.Mobilecore.stop()
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "onDestroy stop core failed", t)
        }
        instance = null
        isRunning = false
        MonitorScheduler.scheduleNext(applicationContext, "service_destroy")
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun activeCount(): Int =
        getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
            .getInt(AppNotifier.KEY_ACTIVE, 0)

    private fun formatTime(ms: Long): String =
        java.text.SimpleDateFormat("yyyy-MM-dd HH:mm:ss", java.util.Locale.US)
            .format(java.util.Date(ms))

    companion object {
        const val TAG = "MonitorService"
        const val ACTION_UI_VISIBLE = "com.aipanyi.mobile.UI_VISIBLE"
        const val ACTION_UI_HIDDEN = "com.aipanyi.mobile.UI_HIDDEN"
        const val ACTION_EXIT = "com.aipanyi.mobile.EXIT"
        const val ACTION_SCHEDULE_CHANGED = "com.aipanyi.mobile.SCHEDULE_CHANGED"
        const val ACTION_ALARM_START = "com.aipanyi.mobile.ALARM_START"

        @Volatile
        var instance: MonitorService? = null
            private set

        @Volatile
        var isRunning: Boolean = false
            private set

        fun onScheduleChanged() {
            instance?.let {
                it.handler.post {
                    it.waitingForSchedule = false
                    it.alarmTimeout?.let { r -> it.handler.removeCallbacks(r) }
                    it.alarmTimeout = null
                    it.scheduleStopFromPrefs()
                    it.stopIfIdle("schedule_changed")
                }
            }
        }
    }
}
```

> 说明：`ACTION_ALARM_START` 是专门给“下一个窗口”的闹钟用的。此时 SharedPreferences
> 里还是上一个窗口的 start/end，不能按旧窗口立即停止；服务会启动/刷新 Go 核心，
> Go 通过 `UpdateMonitorSchedule` 写入新窗口后，`onScheduleChanged` 再恢复正常停止计时。
> 如果 120 秒内没有收到新调度，服务会停止并写一条失败日志。

**Step 2：修改 `MainActivity.kt`**

增加字段：

```kotlin
private var exiting = false
```

在 `onCreate` 中记录：

```kotlin
RuntimeLog.init(this)
RuntimeLog.info(TAG, "onCreate")
```

把 `startMonitorServiceAndLoad()` 改为带可见 action：

```kotlin
private fun startMonitorServiceAndLoad() {
    errorView.visibility = View.GONE
    progress.visibility = View.VISIBLE
    registerCoreReceiver()
    startMonitorService(MonitorService.ACTION_UI_VISIBLE)
    handler.removeCallbacks(coreTimeout)
    handler.postDelayed(coreTimeout, 15000)
}
```

修改 `startMonitorService`：

```kotlin
private fun startMonitorService(action: String = MonitorService.ACTION_UI_VISIBLE) {
    try {
        ContextCompat.startForegroundService(
            this,
            Intent(this, MonitorService::class.java).setAction(action)
        )
        RuntimeLog.info(TAG, "start service action=$action")
    } catch (t: Throwable) {
        RuntimeLog.error(TAG, "start monitor service failed action=$action", t)
        Log.w(TAG, "start monitor service failed", t)
    }
}
```

增加 `onStart` / `onStop`：

```kotlin
override fun onStart() {
    super.onStart()
    RuntimeLog.info(TAG, "onStart")
    if (baseUrl != null) {
        startMonitorService(MonitorService.ACTION_UI_VISIBLE)
    }
}

override fun onStop() {
    super.onStop()
    RuntimeLog.info(TAG, "onStop exiting=$exiting")
    if (exiting || isChangingConfigurations) return
    if (MonitorScheduler.isInWindow(this)) {
        try {
            startService(
                Intent(this, MonitorService::class.java)
                    .setAction(MonitorService.ACTION_UI_HIDDEN)
            )
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "send ui hidden failed", t)
        }
    } else {
        RuntimeLog.info(TAG, "ui hidden outside window, stop service")
        stopService(Intent(this, MonitorService::class.java))
    }
}
```

增加手动退出：

```kotlin
fun exitApp() {
    exiting = true
    RuntimeLog.info(TAG, "manual exit requested")
    try {
        startService(
            Intent(this, MonitorService::class.java)
                .setAction(MonitorService.ACTION_EXIT)
        )
    } catch (t: Throwable) {
        RuntimeLog.error(TAG, "request service exit failed", t)
        stopService(Intent(this, MonitorService::class.java))
    }
    finishAndRemoveTask()
}
```

增加精确闹钟方法：

```kotlin
fun canScheduleExactAlarms(): Boolean = MonitorScheduler.canScheduleExactAlarms(this)

fun requestExactAlarmPermission() {
    RuntimeLog.info(TAG, "request exact alarm permission")
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S) return
    try {
        startActivity(
            Intent(
                Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM,
                Uri.parse("package:$packageName")
            )
        )
    } catch (t: Throwable) {
        RuntimeLog.error(TAG, "open exact alarm settings failed", t)
        openAppDetails()
    }
}
```

在 `onDestroy` 开头记录：

```kotlin
RuntimeLog.info(TAG, "onDestroy")
```

并在 `super.onDestroy()` 前保持原有清理逻辑。

**Step 3：修改 `BootReceiver.kt`**

```kotlin
package com.aipanyi.mobile

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        RuntimeLog.init(context)
        val action = intent.action ?: return
        if (action != Intent.ACTION_BOOT_COMPLETED && action != Intent.ACTION_MY_PACKAGE_REPLACED) return
        RuntimeLog.info("BootReceiver", "action=$action")
        val active = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
            .getInt(AppNotifier.KEY_ACTIVE, 0)
        RuntimeLog.info("BootReceiver", "active alerts=$active")
        if (active <= 0) return
        MonitorScheduler.resumeAfterBoot(context)
    }
}
```

**Step 4：修改 `NativeBridge.kt`**

增加：

```kotlin
@JavascriptInterface
fun exitApp() {
    activity.runOnUiThread { activity.exitApp() }
}

@JavascriptInterface
fun canScheduleExactAlarms(): Boolean = MonitorScheduler.canScheduleExactAlarms(activity)

@JavascriptInterface
fun requestExactAlarmPermission() {
    activity.runOnUiThread { activity.requestExactAlarmPermission() }
}
```

**Step 5：修改 Manifest**

在权限区增加：

```xml
<uses-permission android:name="android.permission.SCHEDULE_EXACT_ALARM" />
```

在 `<application>` 增加：

```xml
android:name=".AiPanyiApp"
```

在 `BootReceiver` 后增加：

```xml
<receiver
    android:name=".MonitorAlarmReceiver"
    android:exported="false" />
```

**Step 6：构建 AAR 和 APK**

```powershell
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-aar.ps1 -Version 0.1.0
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1 -Version 0.1.0 -SkipAar
```

如果 AAR 构建失败，按脚本提示先检查 `gomobile`、`ANDROID_HOME`、`ANDROID_NDK_HOME`、`JAVA_HOME`。Kotlin 编译必须通过后才能提交本任务。

**Step 7：暂存改动（合并到 Task 8 的后台提交）**

```powershell
git add mobile/android/app/src/main/java/com/aipanyi/mobile mobile/android/app/src/main/AndroidManifest.xml
# 不单独 commit；Task 8 的设置页与文档完成后统一提交。
```

---

## Task 8：设置页精确闹钟与退出入口，更新文档

**Files**

- Modify: `mobile/frontend/src/native.js`
- Modify: `mobile/frontend/src/views/Settings.vue`
- Modify: `mobile/android/README.md`
- Modify: `mobile/README.md`（如有必要）

**Step 1：扩展 `native.js`**

```js
exitApp() {
  const b = nativeBridge()
  if (b) safe(() => b.exitApp(), null)
},
canScheduleExactAlarms() {
  const b = nativeBridge()
  return b ? !!safe(() => b.canScheduleExactAlarms(), false) : false
},
requestExactAlarmPermission() {
  const b = nativeBridge()
  if (b) safe(() => b.requestExactAlarmPermission(), null)
}
```

**Step 2：Settings 状态**

在 `nativeState` 增加：

```js
exactAlarms: false
```

在 `refreshNative` 增加：

```js
nativeState.exactAlarms = native.canScheduleExactAlarms()
```

增加函数：

```js
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
```

**Step 3：模板增加状态行和退出按钮**

在「后台与通知」的 native 区域增加：

```html
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
```

把原来的提示文案改为：

```html
<div class="hint" style="margin-top: 10px">
  后台只在已启用提醒涉及的市场交易时段运行；非交易时段自动停止，下个窗口自动启动。
  当前启用市场由提醒规则决定（A 股 / 港股 / 美股）。
  设置 → 应用 → 应用启动管理 → AI盘译 → 手动管理，允许自启动、关联启动、后台活动。
</div>
```

在 native 区域末尾增加：

```html
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
```

**Step 4：更新 Android README**

把「已验证的 gomobile API」中的 `Notifier` 接口改为：

```java
public interface mobilecore.Notifier {
  void notify(String title, String body, String payload);
  void updateMonitorStatus(int active);
  void updateMonitorSchedule(long startMs, long endMs, long nextStartMs);
}
```

增加「运行数据与日志」小节：

```markdown
## 运行数据与日志

- 数据根目录：`filesDir/aipanyi/`
- SQLite：`filesDir/aipanyi/aipanyi.db`
- Go 日志：`filesDir/aipanyi/logs/info.log`、`error.log`
- Kotlin 生命周期日志：`filesDir/aipanyi/logs/runtime.log`（512 KB 滚动，保留 3 个文件）

交易日连手机分析：

```powershell
adb shell run-as com.aipanyi.mobile cat files/aipanyi/logs/runtime.log
adb shell run-as com.aipanyi.mobile cat files/aipanyi/logs/info.log
```

`run-as` 仅对 debug 包有效；release 包请用文件管理器进入应用私有目录导出。
```

增加「后台调度」小节：

```markdown
## 后台调度

- Go 核心按启用提醒的市场计算当前/下一个交易窗口，通过 `UpdateMonitorSchedule` 下发给 Kotlin。
- A 股 09:15–15:00、港股 09:00–16:10、美股 04:00–20:00（纽约时间，含盘前盘后）。
- 重叠/首尾相接的窗口合并为一个连续窗口。
- 没有启用提醒时不安排后台启动。
- 设置页「退出应用」停止本次运行，下一个未来窗口仍会自动启动。
- Android 12+ 若未授权 `SCHEDULE_EXACT_ALARM`，会降级为不精确闹钟；设置页可跳转授权。
```

**Step 5：构建前端并确认无编译错误**

```powershell
cd mobile/frontend; npm run build; cd ..\..
```

如果 Kotlin 代码在 Task 7 后又有改动，重新执行：

```powershell
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-aar.ps1 -Version 0.1.0
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1 -Version 0.1.0 -SkipAar
```

**Step 6：提交（第 4 个逻辑提交，包含 Task 5-8）**

```powershell
git add backend/marketcal mobile/core mobile/android/app/src/main/java/com/aipanyi/mobile mobile/android/app/src/main/AndroidManifest.xml mobile/frontend/src/native.js mobile/frontend/src/views/Settings.vue mobile/android/README.md
git commit -m "feat(mobile): 按交易窗口自动启停后台服务"
```

---

## Task 9：全量验证与 PR

**Step 1：Go 验证**

```powershell
go build ./...
go vet ./...
go test ./backend/marketcal ./backend/modelinfo ./backend/agent -count=1
go test ./backend/data -run 'TestSetDataDir|TestFetchAiModel' -count=1
go test ./mobile/core -run TestMigrateLegacyData -count=1
go test ./webserver/... -count=1
go build -tags mobileassets ./mobile/...
$env:GOOS="android"; $env:GOARCH="arm64"; $env:CGO_ENABLED="0"
go build -tags mobileassets ./mobile/...
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
```

**Step 2：前端验证**

```powershell
cd mobile/frontend
npm run build
cd ..\..
cd frontend
npm run build
cd ..
```

**Step 3：Android 构建验证**

```powershell
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-aar.ps1 -Version 0.1.0
powershell -ExecutionPolicy Bypass -File mobile/scripts/build-apk.ps1 -Version 0.1.0 -SkipAar
```

**Step 4：真机验收**

安装 `mobile/android/app/build/outputs/apk/debug/app-debug.apk` 后逐项检查：

1. 自选左滑出现「删除」；确认后股票消失，提醒页对应规则变为停用。
2. 设置 → AI 配置：填 BaseUrl + API Key，点「获取模型」，选择模型后弹出自动填充提示；不展开高级参数也能保存。
3. 用文件管理器或 `adb shell run-as` 确认 `files/aipanyi/aipanyi.db`、`files/aipanyi/logs/info.log`、`files/aipanyi/logs/runtime.log` 存在；旧根目录不再增长。
4. 删除所有提醒后退出 App，常驻通知消失；重新打开后服务可用，但离开页面后停止。
5. 只保留 A 股提醒：非 A 股窗口退出 App 后服务停止；窗口开始后 `runtime.log` 出现 `alarm fired` 和 `service onCreate`。
6. 只保留美股提醒：对应北京时间窗口自动启动/停止。
7. 窗口内点「退出应用」：页面关闭、服务停止；`runtime.log` 出现 `manual exit requested` / `stop service reason=manual_exit`，并保留 `scheduleNext` 的下一个窗口。
8. 精确闹钟未授权时设置页显示「去开启」；授权后重新进入设置页显示「已开启」。
9. 用以下命令导出时间线：

```powershell
adb shell run-as com.aipanyi.mobile cat files/aipanyi/logs/runtime.log
adb shell run-as com.aipanyi.mobile cat files/aipanyi/logs/info.log
```

**Step 5：检查 diff 和提交**

```powershell
git status --short
git diff --stat origin/main...HEAD
git log --oneline origin/main..HEAD
```

确认没有把 `frontend/dist/`、`node_modules/`、`mobile/android/app/libs/mobilecore.aar`、本地数据库或临时日志加入提交。

**Step 6：推送分支并开 PR**

```powershell
git push -u origin feat/mobile-ux-config-storage-background
gh pr create --base main --head feat/mobile-ux-config-storage-background --title "feat: 移动端模型配置、左滑删除、文件归拢与后台调度" --body @"
## 改了什么

- 自选列表支持左滑删除，删除后同步停用该股票提醒。
- 移动端/桌面端模型配置支持一键获取模型，选中后自动填充最大输出和上下文窗口；高级参数默认折叠。
- 移动端运行时文件统一到 filesDir/aipanyi/，旧数据自动迁移。
- 后台服务按启用提醒涉及市场的交易窗口自动启停；无提醒时不常驻。
- 设置页增加精确闹钟授权入口和「退出应用」按钮。
- 新增 Kotlin runtime.log 与 Go 调度日志，便于交易日事后分析。

## 为什么

解决移动端不能删除自选、模型参数需要手工填写、运行时文件散落、后台 7x24 常驻耗电的问题。

## 影响面

- 移动端数据库/日志路径变化，首次启动自动迁移。
- mobilecore.Notifier 新增 UpdateMonitorSchedule，需要重新构建 AAR。
- 桌面端模型配置界面调整，后端绑定签名不变。

## 验证

- go build ./...
- go vet ./...
- go test ./backend/marketcal ./backend/modelinfo ./backend/agent
- go test ./backend/data -run 'TestSetDataDir|TestFetchAiModel'
- go test ./mobile/core -run TestMigrateLegacyData
- go test ./webserver/...
- mobile/frontend npm run build
- frontend npm run build
- mobile/scripts/build-aar.ps1
- mobile/scripts/build-apk.ps1 -SkipAar

## 真机

- 已在 Android 真机验收自选删除、模型获取与自动填充、文件迁移、定时启停、手动退出和 runtime.log 时间线。
"@
```

PR 合入后不要删除功能分支（仓库约定）。

---

## 风险与回滚

- **模型接口不实现 `/models`**：前端保留手动输入，获取失败只提示，不阻塞保存。
- **精确闹钟未授权**：自动降级不精确闹钟；设置页提供授权入口。日志中 `exact=false` 可识别。
- **Android 12+ 后台启动 FGS 被拒**：`MonitorScheduler.startMonitorService` 捕获异常并发可点击通知，不会静默失败。
- **旧日志迁移冲突**：按文件合并，目标已存在则跳过，不覆盖；`runtime.log` 不会被旧日志覆盖。
- **服务被系统杀死**：当前窗口内会尽量保持；若进程被清理，已保存的下一个窗口闹钟仍可再次拉起。`onDestroy` 会再次调用 `scheduleNext`。
- **回滚**：本次改动集中在功能分支；如需回滚，直接 revert 对应提交。数据库路径迁移不可自动回退，回滚前需备份 `filesDir/aipanyi/`。
