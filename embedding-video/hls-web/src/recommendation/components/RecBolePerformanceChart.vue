<script setup>
import { computed, onMounted, ref } from 'vue'
import { fetchRecBolePerformance } from '../api/recommendationConsole.js'
import {
  buildTrendGeometry,
  formatPerformanceValue,
  normalizePerformancePoints,
} from '../recbolePerformance.js'

const metricPresentation = {
  'Recall@20': {
    label: '召回率（Recall@20）',
    description: '衡量前 20 条推荐覆盖用户感兴趣内容的程度，越高越好。',
  },
  'NDCG@20': {
    label: '排序质量（NDCG@20）',
    description: '衡量前 20 条推荐的相关性和排序位置，相关内容越靠前得分越高。',
  },
  'Hit@20': {
    label: '命中率（Hit@20）',
    description: '衡量前 20 条推荐中至少命中一条用户感兴趣内容的比例，越高越好。',
  },
  'Precision@20': {
    label: '准确率（Precision@20）',
    description: '衡量前 20 条推荐中用户感兴趣内容所占的比例，越高越好。',
  },
}

const defaultMetrics = Object.entries(metricPresentation).map(([value, presentation]) => ({
  value,
  label: presentation.label,
}))

const today = new Date()
const sevenDaysAgo = new Date(today)
sevenDaysAgo.setDate(today.getDate() - 7)

const beginDate = ref(formatDateInput(sevenDaysAgo))
const endDate = ref(formatDateInput(today))
const metric = ref('NDCG@20')
const availableMetrics = ref(defaultMetrics)
const points = ref([])
const loading = ref(false)
const error = ref('')
const selectedPoint = ref(null)
let requestID = 0

const geometry = computed(() => buildTrendGeometry(points.value, {
  width: 720,
  height: 280,
  padding: 44,
}))
const latestPoint = computed(() => geometry.value.points.at(-1) || null)
const maxPoint = computed(() => geometry.value.points.reduce((best, point) => (
  !best || point.value > best.value ? point : best
), null))
const displayedMetrics = computed(() => availableMetrics.value.map((option) => ({
  ...option,
  label: metricPresentation[option.value]?.label || option.label || option.value,
})))
const metricDescription = computed(() => (
  metricPresentation[metric.value]?.description || '该指标暂时没有补充说明。'
))
const tooltip = computed(() => {
  const point = selectedPoint.value
  if (!point) return null
  const x = Math.min(Math.max(point.x, 92), 628)
  const y = point.y < 76 ? point.y + 18 : point.y - 54
  return { point, x, y }
})

onMounted(loadPerformance)

async function loadPerformance() {
  error.value = ''
  selectedPoint.value = null
  const begin = dateBoundaryISO(beginDate.value, false)
  const end = dateBoundaryISO(endDate.value, true)
  if (!begin || !end || Date.parse(begin) > Date.parse(end)) {
    error.value = '请选择有效的起止日期'
    return
  }

  const currentRequestID = ++requestID
  loading.value = true
  try {
    const response = await fetchRecBolePerformance({ metric: metric.value, begin, end })
    if (currentRequestID !== requestID) return
    const data = response?.data || {}
    points.value = normalizePerformancePoints(data.points)
    if (Array.isArray(data.available_metrics) && data.available_metrics.length) {
      availableMetrics.value = data.available_metrics
    }
    if (data.metric) metric.value = data.metric
  } catch (requestError) {
    if (currentRequestID !== requestID) return
    points.value = []
    error.value = requestError?.message || 'RecBole 性能数据加载失败'
  } finally {
    if (currentRequestID === requestID) loading.value = false
  }
}

function dateBoundaryISO(value, endOfDay) {
  const parts = String(value || '').split('-').map(Number)
  if (parts.length !== 3 || parts.some((part) => !Number.isInteger(part))) return ''
  const date = new Date(
    parts[0],
    parts[1] - 1,
    parts[2],
    endOfDay ? 23 : 0,
    endOfDay ? 59 : 0,
    endOfDay ? 59 : 0,
    endOfDay ? 999 : 0,
  )
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString()
}

function formatDateInput(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatAxisDate(timestamp) {
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit' }).format(new Date(timestamp))
}

function formatTimestamp(timestamp) {
  if (!timestamp) return '-'
  return new Date(timestamp).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <article class="tool-panel recbole-performance-panel" aria-label="RecBole 推荐性能趋势">
    <header class="panel-heading recbole-performance-heading">
      <div>
        <p class="eyebrow">RecBole Quality</p>
        <h2>推荐性能趋势</h2>
      </div>
      <form class="recbole-performance-filters" @submit.prevent="loadPerformance">
        <label>
          <span>开始日期</span>
          <input v-model="beginDate" type="date" />
        </label>
        <label>
          <span>结束日期</span>
          <input v-model="endDate" type="date" />
        </label>
        <label class="recbole-metric-select">
          <span>性能指标</span>
          <select v-model="metric" @change="loadPerformance">
            <option v-for="option in displayedMetrics" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </label>
        <button class="control-button" type="submit" :disabled="loading">
          {{ loading ? '加载中' : '刷新' }}
        </button>
      </form>
    </header>

    <p class="recbole-metric-description" aria-live="polite">
      {{ metricDescription }}
    </p>

    <p v-if="error" class="inline-error recbole-performance-error" role="alert">
      <span>{{ error }}</span>
      <button class="secondary-button" type="button" @click="loadPerformance">重试</button>
    </p>

    <div class="recbole-chart-stage">
      <div v-if="loading" class="recbole-chart-state" role="status">正在读取 RecBole 性能数据</div>
      <div v-else-if="!geometry.points.length && !error" class="recbole-chart-state" role="status">该时间范围暂无指标</div>
      <svg
        v-else-if="geometry.points.length"
        class="recbole-trend-chart"
        viewBox="0 0 720 280"
        role="img"
        :aria-label="`${displayedMetrics.find((item) => item.value === metric)?.label || metric}趋势图`"
      >
        <g class="recbole-grid-lines">
          <line
            v-for="tick in geometry.yTicks"
            :key="tick.y"
            x1="44"
            x2="676"
            :y1="tick.y"
            :y2="tick.y"
          />
        </g>
        <g class="recbole-axis-labels">
          <text v-for="tick in geometry.yTicks" :key="`y-${tick.y}`" x="36" :y="tick.y + 4" text-anchor="end">
            {{ formatPerformanceValue(tick.value) }}
          </text>
          <text v-for="tick in geometry.xTicks" :key="`x-${tick.time}`" :x="tick.x" y="268" text-anchor="middle">
            {{ formatAxisDate(tick.timestamp) }}
          </text>
        </g>
        <path class="recbole-area" :d="geometry.areaPath" />
        <path class="recbole-line" :d="geometry.linePath" />
        <g class="recbole-chart-points">
          <circle
            v-for="point in geometry.points"
            :key="point.timestamp"
            :cx="point.x"
            :cy="point.y"
            r="10"
            tabindex="0"
            @mouseenter="selectedPoint = point"
            @mouseleave="selectedPoint = null"
            @focus="selectedPoint = point"
            @blur="selectedPoint = null"
          />
        </g>
        <g v-if="tooltip" class="recbole-tooltip" pointer-events="none">
          <rect :x="tooltip.x - 82" :y="tooltip.y" width="164" height="58" rx="4" />
          <text :x="tooltip.x" :y="tooltip.y + 16" text-anchor="middle">{{ formatPerformanceValue(tooltip.point.value) }}</text>
          <text :x="tooltip.x" :y="tooltip.y + 32" text-anchor="middle">{{ tooltip.point.modelVersion || '-' }}</text>
          <text :x="tooltip.x" :y="tooltip.y + 48" text-anchor="middle">{{ formatTimestamp(tooltip.point.timestamp) }}</text>
        </g>
      </svg>
    </div>

    <div class="recbole-performance-summary" aria-label="RecBole 性能摘要">
      <div><span>当前值</span><strong>{{ formatPerformanceValue(latestPoint?.value) }}</strong></div>
      <div><span>最高值</span><strong>{{ formatPerformanceValue(maxPoint?.value) }}</strong></div>
      <div><span>数据点</span><strong>{{ geometry.points.length }}</strong></div>
      <div><span>更新时间</span><strong>{{ formatTimestamp(latestPoint?.timestamp) }}</strong></div>
    </div>
  </article>
</template>
