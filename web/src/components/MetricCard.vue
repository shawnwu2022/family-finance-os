<script setup lang="ts">
defineProps<{
  label: string
  value: string
  hint?: string
  emphasis?: boolean
  // danger:数值为负面语义(如负 Safe-to-Spend),覆盖 emphasis 的权威蓝
  danger?: boolean
  // dangerLabel:danger 态的文字冗余标记,颜色之外再给色觉障碍用户一个可读信号
  dangerLabel?: string
}>()
</script>

<template>
  <article class="metric-card" :class="{ 'metric-card--emphasis': emphasis, 'metric-card--danger': danger }">
    <p class="metric-card__label">{{ label }}</p>
    <p class="metric-card__value">{{ value }}<span v-if="danger && dangerLabel" class="metric-card__danger-suffix">{{ dangerLabel }}</span></p>
    <p v-if="hint" class="metric-card__hint">{{ hint }}</p>
  </article>
</template>
