<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  fetchRecommendationDiagnostics,
  fetchRecommendationDatasources,
  fetchRecommendationEffects,
  fetchRecommendationOverview,
  fetchRecommendationRedisState,
  previewByQuestion,
  previewRandomPlay,
  traceByQuestion,
  traceRandomPlay,
} from './api/recommendationConsole.js'
import PreviewTable from './components/PreviewTable.vue'
import {
  isKnownSection,
  isValidConsoleLogin,
  readActiveSection,
  readAuthenticated,
  writeActiveSection,
  writeAuthenticated,
} from './config/consoleSession.js'
import { navigationItems, panelRows } from './config/navigation.js'

const browserStorage = typeof window === 'undefined' ? null : window.localStorage
const isAuthenticated = ref(readAuthenticated(browserStorage))
const loginError = ref('')
const loginForm = reactive({
  username: '',
  password: '',
})
const activeSection = ref(readActiveSection(browserStorage, 'diagnostics', navigationItems))
const overview = ref(null)
const overviewLoading = ref(false)
const overviewError = ref('')
const diagnostics = ref(null)
const diagnosticsLoading = ref(false)
const diagnosticsError = ref('')
const diagnosticsForm = reactive({
  days: '14',
  limit: '20',
})
const datasources = ref(null)
const datasourcesLoading = ref(false)
const datasourcesError = ref('')
const effects = ref(null)
const effectsLoading = ref(false)
const effectsError = ref('')
const effectsForm = reactive({
  days: '14',
})

const randomForm = reactive({
  userID: '7',
  limit: '5',
})
const randomLoading = ref(false)
const randomError = ref('')
const randomItems = ref([])

const questionForm = reactive({
  questionID: '',
  questionText: '',
  userID: '7',
  limit: '5',
})
const questionLoading = ref(false)
const questionError = ref('')
const questionItems = ref([])

const traceResult = ref(null)
const randomTraceForm = reactive({
  userID: '7',
  limit: '5',
})
const randomTraceLoading = ref(false)
const randomTraceError = ref('')

const questionTraceForm = reactive({
  questionID: '',
  questionText: '',
  userID: '7',
  limit: '5',
})
const questionTraceLoading = ref(false)
const questionTraceError = ref('')

const redisForm = reactive({
  userID: '7',
})
const redisState = ref(null)
const redisLoading = ref(false)
const redisError = ref('')

const statusCards = computed(() => {
  const data = overview.value
  return [
    {
      label: '推荐引擎',
      value: data?.engine || '未连接',
      detail: data ? `preview_only: ${data.preview_only ? 'true' : 'false'}` : '等待管理 API',
      tone: 'neutral',
    },
    {
      label: 'Gorse',
      value: data?.gorse?.configured ? '已配置' : '未配置',
      detail: data?.gorse ? `candidate ${data.gorse.candidate_limit || 0} / min ${data.gorse.min_recommend_items || 0}` : '等待概览数据',
      tone: 'amber',
    },
    {
      label: 'RecBole',
      value: data?.recbole?.active_model_version || '未发现',
      detail: data?.recbole?.active_model_found ? 'active model' : 'default / none',
      tone: 'green',
    },
    {
      label: 'Redis',
      value: data?.redis?.bucket_enabled ? '播放桶启用' : '播放桶未启用',
      detail: data?.redis ? `recent ${data.redis.recent_ttl_seconds || 0}s / max ${data.redis.recent_max_size || 0}` : '等待概览数据',
      tone: 'blue',
    },
  ]
})

const datasourceGroups = computed(() => {
  const data = datasources.value || {}
  return [
    {
      title: '视频与片段池',
      rows: [
        ['视频总数', formatNumber(data.video_total)],
        ['已发布视频', formatNumber(data.published_videos)],
        ['推荐池视频', formatNumber(data.recommend_videos)],
        ['片段总数', formatNumber(data.segment_total)],
        ['可播放片段', formatNumber(data.playable_segments)],
        ['已向量化片段', `${formatNumber(data.embedded_segments)} / ${formatPercent(data.segment_embedding_rate)}`],
      ],
    },
    {
      title: '曝光与观看',
      rows: [
        ['曝光记录', formatNumber(data.exposure_total)],
        ['观看曝光', `${formatNumber(data.watched_exposures)} / ${formatPercent(data.exposure_watch_rate)}`],
        ['推荐记录', formatNumber(data.recommendation_rows)],
        ['观看推荐', `${formatNumber(data.watched_recommendations)} / ${formatPercent(data.recommendation_watch_rate)}`],
      ],
    },
    {
      title: '用户信号',
      rows: [
        ['RecBole 用户', formatNumber(data.recbole_users)],
        ['RecBole 片段', formatNumber(data.recbole_items)],
        ['Reaction 行为', formatNumber(data.reaction_rows)],
      ],
    },
  ]
})

const diagnosticsHealthCards = computed(() => {
  const rows = diagnostics.value?.health || []
  if (!rows.length) {
    return [
      { label: '健康检查', value: '未加载', detail: '等待诊断接口', tone: 'neutral' },
      { label: '数据新鲜度', value: '未加载', detail: '等待诊断接口', tone: 'neutral' },
      { label: '最近请求', value: '未加载', detail: '等待诊断接口', tone: 'neutral' },
      { label: '任务状态', value: '未加载', detail: '等待诊断接口', tone: 'neutral' },
    ]
  }
  return rows.slice(0, 4).map((row) => ({
    label: row.name || row.key,
    value: row.value || row.status,
    detail: row.detail || row.status,
    tone: toneForStatus(row.status),
  }))
})

const diagnosticsSummaryRows = computed(() => {
  const data = diagnostics.value
  if (!data) {
    return []
  }
  return [
    ['generated_at', formatDateTime(data.generated_at)],
    ['days', formatNumber(data.days)],
    ['request_limit', formatNumber(data.request_limit)],
    ['health_checks', formatNumber(data.health?.length)],
    ['freshness_sources', formatNumber(data.freshness?.length)],
  ]
})

const latestDailyEffect = computed(() => {
  const rows = effects.value?.daily || []
  return rows.length ? rows[rows.length - 1] : null
})

const effectStatusCards = computed(() => {
  const latest = latestDailyEffect.value
  return [
    {
      label: '观察窗口',
      value: `${effects.value?.days || effectsForm.days} 天`,
      detail: '基于曝光表 create_time',
      tone: 'neutral',
    },
    {
      label: '最近曝光',
      value: formatNumber(latest?.exposures),
      detail: latest?.day || '暂无数据',
      tone: 'amber',
    },
    {
      label: '最近观看',
      value: formatNumber(latest?.watched),
      detail: `watch_rate ${formatPercent(latest?.watch_rate)}`,
      tone: 'green',
    },
    {
      label: '策略数量',
      value: formatNumber(effects.value?.strategies?.length),
      detail: '按 strategy + model_version',
      tone: 'blue',
    },
  ]
})

const overviewRows = computed(() => {
  const data = overview.value
  if (!data) {
    return panelRows
  }
  return [
    ['Engine', data.engine || '-'],
    ['Gorse', data.gorse?.configured ? `candidate_limit=${data.gorse.candidate_limit || 0}, shadow=${data.gorse.shadow_mode}` : 'not configured'],
    ['RecBole', data.recbole?.active_model_version || '-'],
    ['Redis recent', `ttl=${data.redis?.recent_ttl_seconds || 0}s, max_size=${data.redis?.recent_max_size || 0}`],
    ['Redis bucket', `enabled=${data.redis?.bucket_enabled ? 'true' : 'false'}, ttl=${data.redis?.bucket_ttl_seconds || 0}s`],
  ]
})

const traceSummaryRows = computed(() => {
  const data = traceResult.value
  if (!data) {
    return []
  }
  return [
    ['mode', data.mode || '-'],
    ['engine', data.engine || '-'],
    ['user_id', formatNumber(data.user_id)],
    ['question_id', data.question_id ? formatNumber(data.question_id) : '-'],
    ['limit', formatNumber(data.limit)],
    ['preview_only', data.preview_only ? 'true' : 'false'],
  ]
})

const redisSummaryCards = computed(() => {
  if (!redisState.value) {
    return [
      { label: '播放桶', value: '未读取', detail: '输入 user_id 后读取', tone: 'neutral' },
      { label: 'recent 去重', value: '未读取', detail: '输入 user_id 后读取', tone: 'neutral' },
    ]
  }
  const bucket = redisState.value?.bucket
  const recent = redisState.value?.recent
  return [
    {
      label: '播放桶',
      value: bucket?.enabled ? formatNumber(bucket.count) : '未启用',
      detail: bucket?.enabled ? `ttl ${formatTTL(bucket.ttl_seconds)} / max ${bucket.max_size || 0}` : 'RandomPlayBucket unavailable',
      tone: bucket?.exists ? 'green' : 'neutral',
    },
    {
      label: 'recent 去重',
      value: recent?.enabled ? formatNumber(recent.count) : '未启用',
      detail: recent?.enabled ? `ttl ${formatTTL(recent.ttl_seconds)} / max ${recent.max_size || 0}` : 'RecentSegments unavailable',
      tone: recent?.over_limit ? 'amber' : 'blue',
    },
  ]
})

const generatedAtText = computed(() => {
  if (!overview.value?.generated_at) {
    return 'not loaded'
  }
  return new Date(overview.value.generated_at).toLocaleString()
})

onMounted(() => {
  if (isAuthenticated.value) {
    loadAll()
  }
})

function submitLogin() {
  loginError.value = ''
  if (!isValidConsoleLogin(loginForm.username, loginForm.password)) {
    loginError.value = '账号或密码不正确'
    return
  }
  writeAuthenticated(browserStorage, true)
  isAuthenticated.value = true
  loginForm.password = ''
  loadAll()
}

function logout() {
  writeAuthenticated(browserStorage, false)
  isAuthenticated.value = false
  loginForm.password = ''
}

function selectSection(section) {
  if (!isKnownSection(section, navigationItems)) {
    return
  }
  activeSection.value = section
  writeActiveSection(browserStorage, section, navigationItems)
}

function loadAll() {
  loadDiagnostics()
  loadOverview()
  loadDatasources()
  loadEffects()
}

async function loadDiagnostics() {
  diagnosticsError.value = ''
  if (!toPositiveInteger(diagnosticsForm.days)) {
    diagnosticsError.value = 'days 必须是正整数'
    return
  }
  if (!toPositiveInteger(diagnosticsForm.limit)) {
    diagnosticsError.value = 'limit 必须是正整数'
    return
  }
  diagnosticsLoading.value = true
  try {
    const response = await fetchRecommendationDiagnostics({
      days: diagnosticsForm.days,
      limit: diagnosticsForm.limit,
    })
    diagnostics.value = response.data
  } catch (error) {
    diagnosticsError.value = readableError(error)
  } finally {
    diagnosticsLoading.value = false
  }
}

async function loadOverview() {
  overviewLoading.value = true
  overviewError.value = ''
  try {
    const response = await fetchRecommendationOverview()
    overview.value = response.data
  } catch (error) {
    overviewError.value = readableError(error)
  } finally {
    overviewLoading.value = false
  }
}

async function loadDatasources() {
  datasourcesLoading.value = true
  datasourcesError.value = ''
  try {
    const response = await fetchRecommendationDatasources()
    datasources.value = response.data
  } catch (error) {
    datasourcesError.value = readableError(error)
  } finally {
    datasourcesLoading.value = false
  }
}

async function loadEffects() {
  effectsError.value = ''
  if (!toPositiveInteger(effectsForm.days)) {
    effectsError.value = 'days 必须是正整数'
    return
  }
  effectsLoading.value = true
  try {
    const response = await fetchRecommendationEffects({ days: effectsForm.days })
    effects.value = response.data
  } catch (error) {
    effectsError.value = readableError(error)
  } finally {
    effectsLoading.value = false
  }
}

async function runRandomPreview() {
  randomError.value = ''
  if (!toPositiveInteger(randomForm.userID)) {
    randomError.value = 'user_id 必须是正整数'
    return
  }
  randomLoading.value = true
  try {
    const response = await previewRandomPlay({
      userID: randomForm.userID,
      limit: randomForm.limit,
    })
    randomItems.value = response.data?.items || []
  } catch (error) {
    randomError.value = readableError(error)
  } finally {
    randomLoading.value = false
  }
}

async function runQuestionPreview() {
  questionError.value = ''
  if (!toPositiveInteger(questionForm.questionID) && !questionForm.questionText.trim()) {
    questionError.value = 'question_id 或 question_text 至少填写一个'
    return
  }
  questionLoading.value = true
  try {
    const response = await previewByQuestion({
      questionID: questionForm.questionID,
      questionText: questionForm.questionText,
      userID: questionForm.userID,
      limit: questionForm.limit,
    })
    questionItems.value = response.data?.items || []
  } catch (error) {
    questionError.value = readableError(error)
  } finally {
    questionLoading.value = false
  }
}

async function runRandomTrace() {
  randomTraceError.value = ''
  if (!toPositiveInteger(randomTraceForm.userID)) {
    randomTraceError.value = 'user_id 必须是正整数'
    return
  }
  randomTraceLoading.value = true
  try {
    const response = await traceRandomPlay({
      userID: randomTraceForm.userID,
      limit: randomTraceForm.limit,
    })
    traceResult.value = response.data
  } catch (error) {
    randomTraceError.value = readableError(error)
  } finally {
    randomTraceLoading.value = false
  }
}

async function runQuestionTrace() {
  questionTraceError.value = ''
  if (!toPositiveInteger(questionTraceForm.questionID) && !questionTraceForm.questionText.trim()) {
    questionTraceError.value = 'question_id 或 question_text 至少填写一个'
    return
  }
  questionTraceLoading.value = true
  try {
    const response = await traceByQuestion({
      questionID: questionTraceForm.questionID,
      questionText: questionTraceForm.questionText,
      userID: questionTraceForm.userID,
      limit: questionTraceForm.limit,
    })
    traceResult.value = response.data
  } catch (error) {
    questionTraceError.value = readableError(error)
  } finally {
    questionTraceLoading.value = false
  }
}

async function loadRedisState() {
  redisError.value = ''
  if (!toPositiveInteger(redisForm.userID)) {
    redisError.value = 'user_id 必须是正整数'
    return
  }
  redisLoading.value = true
  try {
    const response = await fetchRecommendationRedisState({ userID: redisForm.userID })
    redisState.value = response.data
  } catch (error) {
    redisError.value = readableError(error)
  } finally {
    redisLoading.value = false
  }
}

function toPositiveInteger(value) {
  const number = Number(value)
  if (!Number.isFinite(number) || number <= 0) {
    return 0
  }
  return Math.trunc(number)
}

function readableError(error) {
  return error?.message || 'request failed'
}

function formatNumber(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) {
    return '0'
  }
  return new Intl.NumberFormat('zh-CN').format(number)
}

function formatPercent(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) {
    return '0.0%'
  }
  return `${(number * 100).toFixed(1)}%`
}

function formatScore(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) {
    return '0.000'
  }
  return number.toFixed(3)
}

function formatTTL(value) {
  const seconds = Number(value)
  if (!Number.isFinite(seconds)) {
    return '0s'
  }
  if (seconds < 0) {
    return seconds === -2 ? 'not exists' : 'no expire'
  }
  if (seconds < 60) {
    return `${Math.trunc(seconds)}s`
  }
  const minutes = Math.floor(seconds / 60)
  const rest = Math.trunc(seconds % 60)
  return rest ? `${minutes}m ${rest}s` : `${minutes}m`
}

function formatDateTime(value) {
  if (!value) {
    return '-'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }
  return date.toLocaleString()
}

function toneForStatus(status) {
  switch (status) {
    case 'ok':
      return 'green'
    case 'error':
      return 'red'
    case 'warn':
      return 'amber'
    default:
      return 'neutral'
  }
}
</script>

<template>
  <main v-if="!isAuthenticated" class="login-shell">
    <section class="login-panel" aria-label="推荐控制台登录">
      <div class="brand-block login-brand">
        <span class="brand-mark">RC</span>
        <div>
          <strong>Recommendation Console</strong>
          <small>Hengshui Tablet Video</small>
        </div>
      </div>

      <form class="login-form" @submit.prevent="submitLogin">
        <label>
          <span>账号</span>
          <input v-model="loginForm.username" autocomplete="username" autofocus />
        </label>
        <label>
          <span>密码</span>
          <input v-model="loginForm.password" type="password" autocomplete="current-password" />
        </label>
        <p v-if="loginError" class="inline-error" role="alert">{{ loginError }}</p>
        <button class="control-button" type="submit">登录</button>
      </form>
    </section>
  </main>

  <main v-else class="console-shell">
    <aside class="console-sidebar" aria-label="推荐控制台导航">
      <div class="brand-block">
        <span class="brand-mark">RC</span>
        <div>
          <strong>Recommendation Console</strong>
          <small>Hengshui Tablet Video</small>
        </div>
      </div>

      <nav class="nav-stack">
        <button
          v-for="item in navigationItems"
          :key="item.key"
          class="nav-item"
          :class="{ active: item.key === activeSection }"
          type="button"
          @click="selectSection(item.key)"
        >
          <span>{{ item.label }}</span>
          <small>{{ item.hint }}</small>
        </button>
      </nav>
    </aside>

    <section class="console-workspace">
      <header class="workspace-header">
        <div>
          <p class="eyebrow">业务推荐控制台</p>
          <h1>推荐链路观测与调试</h1>
        </div>
        <div class="header-actions">
          <span class="environment-pill">local / api proxy</span>
          <button class="control-button" type="button" :disabled="diagnosticsLoading || overviewLoading || datasourcesLoading || effectsLoading" @click="loadAll">
            {{ diagnosticsLoading || overviewLoading || datasourcesLoading || effectsLoading ? '刷新中' : '刷新' }}
          </button>
          <button class="control-button secondary-button" type="button" @click="logout">
            退出
          </button>
        </div>
      </header>

      <section class="metric-grid" aria-label="推荐链路状态">
        <article v-for="card in statusCards" :key="card.label" class="metric-card" :data-tone="card.tone">
          <span>{{ card.label }}</span>
          <strong>{{ card.value }}</strong>
          <small>{{ card.detail }}</small>
        </article>
      </section>

      <p v-if="overviewError" class="inline-error" role="alert">{{ overviewError }}</p>

      <section v-if="activeSection === 'diagnostics'" class="section-stack" id="diagnostics">
        <header class="section-heading">
          <div>
            <p class="eyebrow">Diagnostics</p>
            <h2>推荐链路诊断中心</h2>
          </div>
          <form class="inline-form" @submit.prevent="loadDiagnostics">
            <label>
              <span>days</span>
              <input v-model="diagnosticsForm.days" inputmode="numeric" />
            </label>
            <label>
              <span>limit</span>
              <input v-model="diagnosticsForm.limit" inputmode="numeric" />
            </label>
            <button class="control-button" type="submit" :disabled="diagnosticsLoading">
              {{ diagnosticsLoading ? '诊断中' : '刷新诊断' }}
            </button>
          </form>
        </header>
        <p v-if="diagnosticsError" class="inline-error" role="alert">{{ diagnosticsError }}</p>

        <section class="metric-grid" aria-label="推荐诊断健康状态">
          <article v-for="card in diagnosticsHealthCards" :key="card.label" class="metric-card compact" :data-tone="card.tone">
            <span>{{ card.label }}</span>
            <strong>{{ card.value }}</strong>
            <small>{{ card.detail }}</small>
          </article>
        </section>

        <div class="diagnostics-layout">
          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Summary</p>
                <h2>诊断摘要</h2>
              </div>
              <span class="environment-pill">{{ diagnosticsLoading ? 'loading' : 'read only' }}</span>
            </header>
            <div class="pipeline-table">
              <div v-for="row in diagnosticsSummaryRows" :key="row[0]" class="pipeline-row">
                <strong>{{ row[0] }}</strong>
                <span>{{ row[1] }}</span>
              </div>
            </div>
            <div class="trace-stage-list diagnostics-health-list">
              <div v-for="check in diagnostics?.health || []" :key="check.key" class="trace-stage">
                <strong>{{ check.name }}</strong>
                <span :data-status="check.status">{{ check.status }}</span>
                <small>{{ check.detail }}</small>
              </div>
            </div>
          </article>

          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Freshness</p>
                <h2>数据新鲜度</h2>
              </div>
            </header>
            <div class="simple-table-wrap">
              <table v-if="diagnostics?.freshness?.length" class="simple-table">
                <thead>
                  <tr>
                    <th>source</th>
                    <th>status</th>
                    <th>latest</th>
                    <th>age</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in diagnostics.freshness" :key="row.source">
                    <td>{{ row.label || row.source }}</td>
                    <td><span class="status-chip" :data-status="row.status">{{ row.status }}</span></td>
                    <td>{{ row.has_data ? formatDateTime(row.latest_at) : '-' }}</td>
                    <td>{{ row.detail || '-' }}</td>
                  </tr>
                </tbody>
              </table>
              <p v-else class="empty-state">暂无新鲜度数据</p>
            </div>
          </article>
        </div>

        <div class="effect-layout">
          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Requests</p>
                <h2>最近推荐请求</h2>
              </div>
            </header>
            <div class="simple-table-wrap">
              <table v-if="diagnostics?.recent_requests?.length" class="simple-table strategy-table">
                <thead>
                  <tr>
                    <th>request</th>
                    <th>user</th>
                    <th>question</th>
                    <th>strategy</th>
                    <th>watch</th>
                    <th>last</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in diagnostics.recent_requests" :key="row.request_id">
                    <td>{{ row.request_id }}</td>
                    <td>{{ row.user_id }}</td>
                    <td>{{ row.question_id || '-' }}</td>
                    <td>
                      <span class="strategy-chip">{{ row.strategy || '-' }}</span>
                      <small>{{ row.model_version || 'no model' }}</small>
                    </td>
                    <td>{{ formatNumber(row.watched) }} / {{ formatNumber(row.exposures) }} · {{ formatPercent(row.watch_rate) }}</td>
                    <td>{{ formatDateTime(row.last_event_time) }}</td>
                  </tr>
                </tbody>
              </table>
              <p v-else class="empty-state">暂无推荐请求历史</p>
            </div>
          </article>

          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Tasks</p>
                <h2>训练与同步状态</h2>
              </div>
            </header>
            <div class="trace-stage-list">
              <div v-for="task in diagnostics?.tasks || []" :key="task.name" class="trace-stage">
                <strong>{{ task.name }}</strong>
                <span :data-status="task.status">{{ task.status }}</span>
                <small>{{ task.detail }}</small>
                <small v-if="task.has_run_time">last: {{ formatDateTime(task.last_run_at) }}</small>
              </div>
              <p v-if="!diagnostics?.tasks?.length" class="empty-state">暂无任务状态</p>
            </div>
          </article>
        </div>
      </section>

      <section v-if="activeSection === 'overview'" class="console-band" id="overview">
        <div class="band-copy">
          <p class="eyebrow">Pipeline</p>
          <h2>Random-play 主链路</h2>
          <small>generated_at: {{ generatedAtText }}</small>
        </div>

        <div class="pipeline-table">
          <div v-for="row in overviewRows" :key="row[0]" class="pipeline-row">
            <strong>{{ row[0] }}</strong>
            <span>{{ row[1] }}</span>
          </div>
        </div>
      </section>

      <section v-if="activeSection === 'datasources'" class="section-stack" id="datasources">
        <header class="section-heading">
          <div>
            <p class="eyebrow">Inputs</p>
            <h2>推荐数据源健康</h2>
          </div>
          <button class="control-button" type="button" :disabled="datasourcesLoading" @click="loadDatasources">
            {{ datasourcesLoading ? '刷新中' : '刷新数据源' }}
          </button>
        </header>
        <p v-if="datasourcesError" class="inline-error" role="alert">{{ datasourcesError }}</p>

        <div class="data-source-grid">
          <article v-for="group in datasourceGroups" :key="group.title" class="source-card">
            <h3>{{ group.title }}</h3>
            <div class="source-row" v-for="row in group.rows" :key="row[0]">
              <span>{{ row[0] }}</span>
              <strong>{{ row[1] }}</strong>
            </div>
          </article>
        </div>
      </section>

      <section v-if="activeSection === 'effects'" class="section-stack" id="effects">
        <header class="section-heading">
          <div>
            <p class="eyebrow">Hit Effect</p>
            <h2>推荐命中效果变化</h2>
          </div>
          <form class="inline-form" @submit.prevent="loadEffects">
            <label>
              <span>days</span>
              <input v-model="effectsForm.days" inputmode="numeric" />
            </label>
            <button class="control-button" type="submit" :disabled="effectsLoading">
              {{ effectsLoading ? '加载中' : '查看' }}
            </button>
          </form>
        </header>
        <p v-if="effectsError" class="inline-error" role="alert">{{ effectsError }}</p>

        <section class="metric-grid" aria-label="命中效果状态">
          <article v-for="card in effectStatusCards" :key="card.label" class="metric-card compact" :data-tone="card.tone">
            <span>{{ card.label }}</span>
            <strong>{{ card.value }}</strong>
            <small>{{ card.detail }}</small>
          </article>
        </section>

        <div class="effect-layout">
          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Daily</p>
                <h2>按天变化</h2>
              </div>
            </header>
            <div class="simple-table-wrap">
              <table v-if="effects?.daily?.length" class="simple-table">
                <thead>
                  <tr>
                    <th>day</th>
                    <th>exposures</th>
                    <th>watched</th>
                    <th>watch_rate</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in effects.daily" :key="row.day">
                    <td>{{ row.day }}</td>
                    <td>{{ formatNumber(row.exposures) }}</td>
                    <td>{{ formatNumber(row.watched) }}</td>
                    <td>{{ formatPercent(row.watch_rate) }}</td>
                  </tr>
                </tbody>
              </table>
              <p v-else class="empty-state">暂无命中效果日趋势</p>
            </div>
          </article>

          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Strategy</p>
                <h2>按策略拆分</h2>
              </div>
            </header>
            <div class="simple-table-wrap">
              <table v-if="effects?.strategies?.length" class="simple-table strategy-table">
                <thead>
                  <tr>
                    <th>strategy</th>
                    <th>model</th>
                    <th>exposures</th>
                    <th>watch_rate</th>
                    <th>avg_rank</th>
                    <th>avg_score</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in effects.strategies" :key="`${row.strategy}-${row.model_version}`">
                    <td><span class="strategy-chip">{{ row.strategy || '-' }}</span></td>
                    <td>{{ row.model_version || 'no model' }}</td>
                    <td>{{ formatNumber(row.exposures) }}</td>
                    <td>{{ formatPercent(row.watch_rate) }}</td>
                    <td>{{ formatScore(row.average_rank) }}</td>
                    <td>{{ formatScore(row.average_score) }}</td>
                  </tr>
                </tbody>
              </table>
              <p v-else class="empty-state">暂无策略效果数据</p>
            </div>
          </article>
        </div>
      </section>

      <section v-if="activeSection === 'trace'" class="section-stack" id="trace">
        <header class="section-heading">
          <div>
            <p class="eyebrow">Trace</p>
            <h2>推荐链路追踪</h2>
          </div>
        </header>

        <div class="tool-grid">
          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Random-play</p>
                <h2>播放链路 Trace</h2>
              </div>
              <button class="control-button" type="button" :disabled="randomTraceLoading" @click="runRandomTrace">
                {{ randomTraceLoading ? '追踪中' : '追踪' }}
              </button>
            </header>
            <form class="control-form" @submit.prevent="runRandomTrace">
              <label>
                <span>user_id</span>
                <input v-model="randomTraceForm.userID" inputmode="numeric" />
              </label>
              <label>
                <span>limit</span>
                <input v-model="randomTraceForm.limit" inputmode="numeric" />
              </label>
            </form>
            <p v-if="randomTraceError" class="inline-error" role="alert">{{ randomTraceError }}</p>
          </article>

          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">By-question</p>
                <h2>题目链路 Trace</h2>
              </div>
              <button class="control-button" type="button" :disabled="questionTraceLoading" @click="runQuestionTrace">
                {{ questionTraceLoading ? '追踪中' : '追踪' }}
              </button>
            </header>
            <form class="control-form question-form" @submit.prevent="runQuestionTrace">
              <label>
                <span>question_id</span>
                <input v-model="questionTraceForm.questionID" inputmode="numeric" />
              </label>
              <label>
                <span>user_id</span>
                <input v-model="questionTraceForm.userID" inputmode="numeric" />
              </label>
              <label>
                <span>limit</span>
                <input v-model="questionTraceForm.limit" inputmode="numeric" />
              </label>
              <label class="wide-field">
                <span>question_text</span>
                <textarea v-model="questionTraceForm.questionText" rows="3" />
              </label>
            </form>
            <p v-if="questionTraceError" class="inline-error" role="alert">{{ questionTraceError }}</p>
          </article>
        </div>

        <article class="tool-panel">
          <header class="panel-heading">
            <div>
              <p class="eyebrow">Result</p>
              <h2>Trace 结果</h2>
            </div>
            <span class="environment-pill">{{ traceResult?.preview_only ? 'preview only' : 'not loaded' }}</span>
          </header>

          <div v-if="traceResult" class="trace-layout">
            <div class="pipeline-table">
              <div v-for="row in traceSummaryRows" :key="row[0]" class="pipeline-row">
                <strong>{{ row[0] }}</strong>
                <span>{{ row[1] }}</span>
              </div>
            </div>

            <div class="trace-stage-list">
              <div v-for="stage in traceResult.stages || []" :key="`${stage.name}-${stage.status}`" class="trace-stage">
                <strong>{{ stage.name }}</strong>
                <span :data-status="stage.status">{{ stage.status }}</span>
                <small>{{ stage.detail }}</small>
              </div>
            </div>
          </div>

          <div class="simple-table-wrap">
            <table v-if="traceResult?.items?.length" class="simple-table trace-table">
              <thead>
                <tr>
                  <th>rank</th>
                  <th>segment</th>
                  <th>title</th>
                  <th>strategy</th>
                  <th>score</th>
                  <th>status</th>
                  <th>reasons</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in traceResult.items" :key="`${item.rank}-${item.video_segment_id}`">
                  <td>{{ item.rank }}</td>
                  <td>{{ item.video_segment_id }}</td>
                  <td>{{ item.title || '-' }}</td>
                  <td>
                    <span class="strategy-chip">{{ item.strategy || '-' }}</span>
                    <small>{{ item.model_version || 'no model' }}</small>
                  </td>
                  <td>{{ formatScore(item.recommend_score) }}</td>
                  <td>{{ item.status || '-' }}</td>
                  <td>
                    <span v-for="reason in item.reasons || []" :key="reason" class="reason-chip">{{ reason }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
            <p v-else class="empty-state">暂无 Trace 结果</p>
          </div>
        </article>
      </section>

      <section v-if="activeSection === 'redis'" class="section-stack" id="redis">
        <header class="section-heading">
          <div>
            <p class="eyebrow">Redis Runtime</p>
            <h2>播放桶与最近播放状态</h2>
          </div>
          <form class="inline-form" @submit.prevent="loadRedisState">
            <label>
              <span>user_id</span>
              <input v-model="redisForm.userID" inputmode="numeric" />
            </label>
            <button class="control-button" type="submit" :disabled="redisLoading">
              {{ redisLoading ? '读取中' : '读取' }}
            </button>
          </form>
        </header>
        <p v-if="redisError" class="inline-error" role="alert">{{ redisError }}</p>

        <section class="metric-grid redis-metrics" aria-label="Redis 推荐状态">
          <article v-for="card in redisSummaryCards" :key="card.label" class="metric-card compact" :data-tone="card.tone">
            <span>{{ card.label }}</span>
            <strong>{{ card.value }}</strong>
            <small>{{ card.detail }}</small>
          </article>
        </section>

        <div class="effect-layout">
          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Bucket</p>
                <h2>random-play 播放桶</h2>
              </div>
              <span class="environment-pill">{{ redisState?.bucket?.exists ? 'exists' : 'empty' }}</span>
            </header>
            <div class="pipeline-table">
              <div class="pipeline-row">
                <strong>ttl</strong>
                <span>{{ formatTTL(redisState?.bucket?.ttl_seconds) }}</span>
              </div>
              <div class="pipeline-row">
                <strong>count</strong>
                <span>{{ formatNumber(redisState?.bucket?.count) }} / {{ redisState?.bucket?.max_size || 0 }}</span>
              </div>
              <div class="pipeline-row">
                <strong>refill below</strong>
                <span>{{ redisState?.bucket?.min_size || 0 }}</span>
              </div>
            </div>
            <div class="simple-table-wrap">
              <table v-if="redisState?.bucket?.items?.length" class="simple-table">
                <thead>
                  <tr>
                    <th>rank</th>
                    <th>segment</th>
                    <th>video</th>
                    <th>strategy</th>
                    <th>score</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in redisState.bucket.items" :key="`${item.rank}-${item.video_segment_id}`">
                    <td>{{ item.rank }}</td>
                    <td>{{ item.video_segment_id }}</td>
                    <td>{{ item.video_id }}</td>
                    <td><span class="strategy-chip">{{ item.strategy || '-' }}</span></td>
                    <td>{{ formatScore(item.score) }}</td>
                  </tr>
                </tbody>
              </table>
              <p v-else class="empty-state">播放桶暂无内容</p>
            </div>
          </article>

          <article class="tool-panel">
            <header class="panel-heading">
              <div>
                <p class="eyebrow">Recent</p>
                <h2>最近播放去重</h2>
              </div>
              <span class="environment-pill">{{ redisState?.recent?.over_limit ? 'over limit' : 'bounded' }}</span>
            </header>
            <div class="pipeline-table">
              <div class="pipeline-row">
                <strong>ttl</strong>
                <span>{{ formatTTL(redisState?.recent?.ttl_seconds) }}</span>
              </div>
              <div class="pipeline-row">
                <strong>count</strong>
                <span>{{ formatNumber(redisState?.recent?.count) }} / {{ redisState?.recent?.max_size || 0 }}</span>
              </div>
              <div class="pipeline-row">
                <strong>over_limit</strong>
                <span>{{ redisState?.recent?.over_limit ? 'true' : 'false' }}</span>
              </div>
            </div>
            <div v-if="redisState?.recent?.segment_ids?.length" class="segment-id-cloud">
              <span v-for="segmentID in redisState.recent.segment_ids" :key="segmentID">{{ segmentID }}</span>
            </div>
            <p v-else class="empty-state">recent set 暂无内容</p>
          </article>
        </div>
      </section>

      <section v-if="activeSection === 'simulator'" class="tool-grid" id="simulator">
        <article class="tool-panel">
          <header class="panel-heading">
            <div>
              <p class="eyebrow">Preview</p>
              <h2>Random-play</h2>
            </div>
            <button class="control-button" type="button" :disabled="randomLoading" @click="runRandomPreview">
              {{ randomLoading ? '预览中' : '预览' }}
            </button>
          </header>

          <form class="control-form" @submit.prevent="runRandomPreview">
            <label>
              <span>user_id</span>
              <input v-model="randomForm.userID" inputmode="numeric" />
            </label>
            <label>
              <span>limit</span>
              <input v-model="randomForm.limit" inputmode="numeric" />
            </label>
          </form>
          <p v-if="randomError" class="inline-error" role="alert">{{ randomError }}</p>
          <PreviewTable :items="randomItems" empty-text="暂无 random-play 预览结果" />
        </article>

        <article class="tool-panel">
          <header class="panel-heading">
            <div>
              <p class="eyebrow">Preview</p>
              <h2>题目推荐</h2>
            </div>
            <button class="control-button" type="button" :disabled="questionLoading" @click="runQuestionPreview">
              {{ questionLoading ? '预览中' : '预览' }}
            </button>
          </header>

          <form class="control-form question-form" @submit.prevent="runQuestionPreview">
            <label>
              <span>question_id</span>
              <input v-model="questionForm.questionID" inputmode="numeric" />
            </label>
            <label>
              <span>user_id</span>
              <input v-model="questionForm.userID" inputmode="numeric" />
            </label>
            <label>
              <span>limit</span>
              <input v-model="questionForm.limit" inputmode="numeric" />
            </label>
            <label class="wide-field">
              <span>question_text</span>
              <textarea v-model="questionForm.questionText" rows="3" />
            </label>
          </form>
          <p v-if="questionError" class="inline-error" role="alert">{{ questionError }}</p>
          <PreviewTable :items="questionItems" empty-text="暂无题目推荐预览结果" />
        </article>
      </section>
    </section>
  </main>
</template>
