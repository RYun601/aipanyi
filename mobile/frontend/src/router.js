import { createRouter, createWebHashHistory } from 'vue-router'
import Watchlist from './views/Watchlist.vue'
import StockDetail from './views/StockDetail.vue'
import Alerts from './views/Alerts.vue'
import Settings from './views/Settings.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/watchlist' },
    { path: '/watchlist', name: 'watchlist', component: Watchlist },
    { path: '/stock/:code', name: 'stock', component: StockDetail, props: true },
    { path: '/alerts', name: 'alerts', component: Alerts },
    { path: '/settings', name: 'settings', component: Settings }
  ]
})

export default router
