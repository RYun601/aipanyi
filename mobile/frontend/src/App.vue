<script setup>
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { connectWS } from './ws'

const route = useRoute()

onMounted(() => connectWS())
</script>

<template>
  <router-view v-slot="{ Component }">
    <component :is="Component" :key="route.path" />
  </router-view>

  <van-tabbar route fixed placeholder safe-area-inset-bottom>
    <van-tabbar-item replace to="/watchlist" icon="star-o">自选</van-tabbar-item>
    <van-tabbar-item replace to="/alerts" icon="bell">提醒</van-tabbar-item>
    <van-tabbar-item replace to="/settings" icon="setting-o">设置</van-tabbar-item>
  </van-tabbar>
</template>
