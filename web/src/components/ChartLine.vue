<template>
  <div class="glass-panel p-5 rounded-2xl border border-gray-200/50 dark:border-white/10 shadow-lg h-full flex flex-col">
    <div class="flex justify-between items-center mb-2 z-10 relative">
      <h3 class="text-sm font-bold text-gray-700 dark:text-gray-300 uppercase tracking-wider">{{ title }}</h3>
    </div>
    <div class="flex-1 w-full relative min-h-[220px]">
      <v-chart class="w-full h-full absolute inset-0" :option="chartOption" autoresize />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  GridComponent,
  DataZoomComponent
} from 'echarts/components'
import VChart from 'vue-echarts'

use([
  CanvasRenderer,
  LineChart,
  TitleComponent,
  TooltipComponent,
  GridComponent,
  DataZoomComponent
])

const props = defineProps({
  title: String,
  data: {
    type: Array,
    default: () => []
  },
  color: {
    type: String,
    default: '#3b82f6'
  },
  isDark: Boolean
})

const chartOption = computed(() => {
  const times = props.data.map(d => {
    const dObj = new Date(d.time)
    return `${dObj.getSeconds().toString().padStart(2, '0')}s`
  })
  const vals = props.data.map(d => d.value)
  
  const textColor = props.isDark ? '#9ca3af' : '#6b7280'
  const splitLineColor = props.isDark ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.05)'
  
  return {
    tooltip: { 
      trigger: 'axis',
      backgroundColor: props.isDark ? 'rgba(31, 41, 55, 0.9)' : 'rgba(255, 255, 255, 0.9)',
      borderColor: props.isDark ? '#374151' : '#e5e7eb',
      textStyle: { color: props.isDark ? '#f3f4f6' : '#111827' }
    },
    grid: { left: 40, right: 10, top: 10, bottom: 20 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: times,
      axisLabel: { color: textColor, showMinLabel: false, showMaxLabel: false },
      axisLine: { show: false },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: props.title.includes('%') ? 100 : undefined,
      splitLine: { lineStyle: { color: splitLineColor, type: 'dashed' } },
      axisLabel: { color: textColor }
    },
    series: [
      {
        data: vals,
        type: 'line',
        smooth: 0.4,
        symbol: 'none',
        lineStyle: { width: 3, color: props.color },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: props.color + '80' },
              { offset: 1, color: props.color + '00' }
            ]
          }
        }
      }
    ]
  }
})
</script>
