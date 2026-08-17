<script setup lang="ts">
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { init, use, type ECharts } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

use([BarChart, GridComponent, TooltipComponent, CanvasRenderer])

const props = defineProps<{
  income: number
  expense: number
  incomeLabel: string
  expenseLabel: string
}>()

const root = ref<HTMLDivElement | null>(null)
let chart: ECharts | undefined
let resizeObserver: ResizeObserver | undefined

function render() {
  if (!root.value) return
  if (!chart) chart = init(root.value)
  chart.setOption({
    animationDuration: 250,
    grid: { top: 12, left: 18, right: 18, bottom: 28, containLabel: true },
    tooltip: {
      trigger: 'axis',
      formatter: () => `收入：${props.incomeLabel}<br/>支出：${props.expenseLabel}`,
    },
    xAxis: {
      type: 'category',
      data: ['收入', '支出'],
      axisTick: { show: false },
      axisLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      splitLine: { lineStyle: { type: 'dashed' } },
    },
    series: [
      {
        type: 'bar',
        barMaxWidth: 54,
        data: [props.income, props.expense],
        itemStyle: { borderRadius: [8, 8, 0, 0] },
      },
    ],
  })
}

onMounted(() => {
  render()
  if (root.value) {
    resizeObserver = new ResizeObserver(() => chart?.resize())
    resizeObserver.observe(root.value)
  }
})

watch(() => [props.income, props.expense, props.incomeLabel, props.expenseLabel], render)

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div ref="root" class="cashflow-chart" role="img" aria-label="当月收入和支出柱状图" />
</template>
