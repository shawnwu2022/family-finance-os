<script setup lang="ts">
defineProps<{
  label: string
  value: string
  hint?: string
  emphasis?: boolean
}>()
</script>

<template>
  <article class="metric-card" :class="{ 'metric-card--emphasis': emphasis }">
    <p class="metric-card__label">{{ label }}</p>
    <p class="metric-card__value">{{ value }}</p>
    <p v-if="hint" class="metric-card__hint">{{ hint }}</p>
  </article>
</template>
