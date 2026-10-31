<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api } from '../api'
import { changeClass, fmtPercent, fmtPrice, fmtSignedMoney } from '../format'

const router = useRouter()

const items = ref([])
const loading = ref(false)
const refreshing = ref(false)
const showSearch = ref(false)
const keyword = ref('')
const searchLoading = ref(false)
const results = ref([])
let timer = null
let searchTimer = null

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const data = await api.getWatchlist()
    items.value = Array.isArray(data) ? data : []
  } catch (e) {
    if (!silent) showToast(e?.message || '加载自选失败')
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

function refresh() {
  load()
}

function openStock(item) {
  router.push('/stock/' + encodeURIComponent(item.code))
}

function openSearch() {
  showSearch.value = true
  keyword.value = ''
  results.value = []
}

function onSearchInput(v) {
  keyword.value = v
  if (searchTimer) clearTimeout(searchTimer)
  if (!v || !v.trim()) {
    results.value = []
    return
  }
  searchTimer = setTimeout(doSearch, 280)
}

async function doSearch() {
  const q = keyword.value.trim()
  if (!q) return
  searchLoading.value = true
  try {
    const data = await api.searchStocks(q)
    results.value = Array.isArray(data) ? data : []
  } catch (e) {
    showToast(e?.message || '搜索失败')
  } finally {
    searchLoading.value = false
  }
}

async function addStock(row) {
  try {
    const msg = await api.addStock(row.code)
    showToast(msg || '已添加')
    showSearch.value = false
    await load()
  } catch (e) {
    showToast(e?.message || '添加失败')
  }
}

function marketLabel(m) {
  return m === 'HK' ? '港股' : m === 'US' ? '美股' : 'A股'
}

onMounted(() => {
  load()
  timer = setInterval(() => {
    if (!document.hidden) load(true)
  }, 10000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<template>
  <div class="page">
    <van-pull-refresh v-model="refreshing" @refresh="refresh">
      <div class="row" style="padding: 6px 4px 10px">
        <div>
          <div style="font-size: 22px; font-weight: 700">自选</div>
          <div class="muted">交易时段每 30 秒检查价格提醒</div>
        </div>
        <van-icon name="replay" size="20" style="color: #8a94a6" @click="load()" />
      </div>

      <van-loading v-if="loading && !items.length" style="padding: 60px 0" vertical>加载中</van-loading>

      <van-empty v-else-if="!items.length" class="empty-box" description="还没有自选股">
        <van-button type="primary" round size="small" @click="openSearch">添加股票</van-button>
      </van-empty>

      <div v-for="item in items" :key="item.code" class="stock-item" @click="openStock(item)">
        <div class="stock-main">
          <div class="stock-name">
            {{ item.name || item.code }}
            <span class="tag-mini">{{ marketLabel(item.market) }}</span>
          </div>
          <div class="stock-sub">
            <span class="mono">{{ item.code }}</span>
            <span v-if="item.costPrice > 0">成本 {{ fmtPrice(item.costPrice) }}</span>
            <span v-if="item.volume > 0">持仓 {{ item.volume }}</span>
          </div>
          <div v-if="item.costPrice > 0 && item.volume > 0" class="small" :class="changeClass(item.profitAmount)">
            盈亏 {{ fmtSignedMoney(item.profitAmount) }} ({{ fmtPercent(item.profitPercent) }})
          </div>
        </div>
        <div class="stock-quote">
          <div class="price-mid" :class="changeClass(item.changePercent)">{{ fmtPrice(item.price) }}</div>
          <div class="small" :class="changeClass(item.changePercent)">{{ fmtPercent(item.changePercent) }}</div>
          <div v-if="!item.hasQuote" class="muted" style="font-size: 10px">行情暂缺</div>
        </div>
      </div>
    </van-pull-refresh>

    <van-button class="floating-add" type="primary" icon="plus" round @click="openSearch">
      添加
    </van-button>

    <van-popup v-model:show="showSearch" position="bottom" round style="height: 82vh">
      <div class="sheet" style="height: 100%; display: flex; flex-direction: column">
        <div class="sheet-title">添加自选</div>
        <van-search
          :model-value="keyword"
          placeholder="输入代码或名称，如 600519 / 茅台"
          @update:model-value="onSearchInput"
          @search="doSearch"
        />
        <div style="flex: 1; overflow-y: auto; margin-top: 8px">
          <van-loading v-if="searchLoading" style="padding: 30px 0" vertical>搜索中</van-loading>
          <van-empty v-else-if="keyword && !results.length" description="没有匹配的股票" />
          <div
            v-for="row in results"
            :key="row.code"
            class="row"
            style="padding: 12px 4px; border-bottom: 1px solid var(--ap-line)"
            @click="addStock(row)"
          >
            <div>
              <div style="font-weight: 600">{{ row.name }}</div>
              <div class="muted mono">{{ row.code }}</div>
            </div>
            <van-icon name="plus" size="18" color="#2563eb" />
          </div>
        </div>
      </div>
    </van-popup>
  </div>
</template>
