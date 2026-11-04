<script setup>
defineProps({ form: { type: Object, required: true } })
</script>

<template>
  <van-field label="提醒方式">
    <template #input>
      <van-radio-group v-model="form.mode" direction="horizontal">
        <van-radio name="price">纯盯价</van-radio>
        <van-radio name="strategy">策略确认</van-radio>
      </van-radio-group>
    </template>
  </van-field>
  <template v-if="form.mode === 'strategy'">
    <van-field label="时段">
      <template #input>
        <van-radio-group v-model="form.phase" direction="horizontal">
          <van-radio name="intraday">盘中</van-radio>
          <van-radio name="close">收盘复核</van-radio>
        </van-radio-group>
      </template>
    </van-field>
    <template v-if="form.phase === 'intraday'">
      <van-field v-model="form.minVolumeRatio" type="number" label="最低量比" placeholder="例如 1.5" />
      <van-field label="买卖盘方向">
        <template #input>
          <van-radio-group v-model="form.bookSide" direction="horizontal">
            <van-radio name="buy">买盘占优</van-radio>
            <van-radio name="sell">卖盘占优</van-radio>
          </van-radio-group>
        </template>
      </van-field>
      <van-field v-model="form.bookPercent" type="number" label="偏向阈值 %" placeholder="例如 15" />
    </template>
    <div class="muted" style="padding: 6px 0">
      {{ form.phase === 'close' ? '待人工复核。正式收盘快照尚未接入。' : '触价后，两条新行情均满足量比和五档买卖盘条件才推送。' }}
    </div>
  </template>
</template>
