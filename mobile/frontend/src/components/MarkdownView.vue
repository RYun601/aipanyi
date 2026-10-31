<script setup>
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

const props = defineProps({
  content: { type: String, default: '' }
})

marked.setOptions({ gfm: true, breaks: true })

const html = computed(() => {
  const src = props.content || ''
  if (!src.trim()) return ''
  try {
    return DOMPurify.sanitize(marked.parse(src))
  } catch {
    return DOMPurify.sanitize(src.replace(/\n/g, '<br>'))
  }
})
</script>

<template>
  <div class="md-body" v-html="html"></div>
</template>
