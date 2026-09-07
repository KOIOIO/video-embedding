<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import HlsPlayer from '../components/HlsPlayer.vue'
import { fetchKnowledgeTree, pollKnowledgeVideoBatch, recordKnowledgePlayback, resolveKnowledgePlayback, uploadKnowledgeVideoBatch } from '../knowledgeVideo/api.js'
import '../knowledgeVideo/knowledgeVideo.css'

const archive = ref(null)
const mapping = ref(null)
const playbackUserId = 6
const uploading = ref(false)
const uploadPercent = ref(0)
const uploadError = ref('')
const validationIssues = ref([])
const batchProgress = ref(null)
const tree = ref([])
const treeLoading = ref(false)
const treeError = ref('')
const search = ref('')
const statusFilter = ref('all')
const selectedNode = ref(null)
const playback = ref(null)
const playbackLoading = ref(false)
const playbackError = ref('')
const recordedVideoIds = new Set()
let disposed = false

const statusOptions = [
  ['all', '全部'], ['unassigned', '未配置'], ['processing', '处理中'], ['ready', '可播放'], ['failed', '失败'],
]

function displayStatus(status) {
  return { unassigned: '未配置', pending: '待处理', transcoding: '转码中', ready: '可播放', failed: '失败' }[status] || status
}

function statusGroup(status) {
  return status === 'pending' || status === 'transcoding' ? 'processing' : status
}

function flatten(nodes, depth = 0, output = []) {
  for (const node of nodes) {
    output.push({ ...node, depth })
    flatten(node.children, depth + 1, output)
  }
  return output
}

const visibleNodes = computed(() => {
  const query = search.value.trim().toLowerCase()
  return flatten(tree.value).filter((node) => {
    const matchesText = !query || String(node.id).includes(query) || node.name.toLowerCase().includes(query)
    const matchesStatus = statusFilter.value === 'all' || statusGroup(node.status) === statusFilter.value
    return matchesText && matchesStatus
  })
})

const completedCount = computed(() => Number(batchProgress.value?.ready_count || 0) + Number(batchProgress.value?.failed_count || 0))
const transcodePercent = computed(() => {
  const total = Number(batchProgress.value?.total_count || 0)
  return total ? Math.round((completedCount.value / total) * 100) : 0
})

async function loadTree() {
  treeLoading.value = true
  treeError.value = ''
  try { tree.value = await fetchKnowledgeTree() } catch (error) { treeError.value = error.message } finally { treeLoading.value = false }
}

function setArchive(event) { archive.value = event.target.files?.[0] || null }
function setMapping(event) { mapping.value = event.target.files?.[0] || null }

async function submitUpload() {
  if (!archive.value || !mapping.value) return
  uploading.value = true
  uploadPercent.value = 0
  uploadError.value = ''
  validationIssues.value = []
  batchProgress.value = null
  try {
    const accepted = await uploadKnowledgeVideoBatch({ archive: archive.value, mapping: mapping.value, onProgress: ({ percent }) => { uploadPercent.value = percent } })
    batchProgress.value = { ...accepted, status: accepted.status || 'processing', ready_count: 0, failed_count: 0 }
    const terminal = await pollKnowledgeVideoBatch(accepted.batch_id, { cancelled: () => disposed })
    if (terminal) {
      batchProgress.value = terminal
      await loadTree()
    }
  } catch (error) {
    uploadError.value = error.message
    validationIssues.value = error.issues || []
  } finally { uploading.value = false }
}

async function selectNode(node) {
  selectedNode.value = node
  playback.value = null
  playbackError.value = ''
  recordedVideoIds.clear()
  playbackLoading.value = true
  try { playback.value = await resolveKnowledgePlayback(node.id) } catch (error) { playbackError.value = error.message } finally { playbackLoading.value = false }
}

function recordPlayback(knowledgeVideoId) {
  if (recordedVideoIds.has(knowledgeVideoId)) return
  recordedVideoIds.add(knowledgeVideoId)
  void recordKnowledgePlayback(knowledgeVideoId, playbackUserId).catch((error) => {
    console.error('knowledge video playback record failed', error)
  })
}

onMounted(loadTree)
onBeforeUnmount(() => { disposed = true })
</script>

<template>
  <section class="knowledge-video-workspace">
    <header class="kv-heading">
      <div><p>内容运营</p><h1>知识点视频</h1></div>
      <button class="kv-secondary" type="button" :disabled="treeLoading" @click="loadTree">刷新列表</button>
    </header>

    <div class="kv-layout">
      <aside class="kv-upload-panel">
        <div class="kv-section-title"><span>01</span><div><h2>批量上传</h2><p>ZIP 视频包与 XLSX 元数据表</p></div></div>
        <form class="kv-upload-form" @submit.prevent="submitUpload">
          <label class="kv-file-field"><span>视频压缩包</span><input type="file" accept=".zip,application/zip" required @change="setArchive" /><small>{{ archive?.name || '选择 ZIP 文件' }}</small></label>
          <label class="kv-file-field"><span>元数据表格</span><input type="file" accept=".xlsx" required @change="setMapping" /><small>{{ mapping?.name || '选择 XLSX 文件' }}</small></label>
          <button class="kv-primary" type="submit" :disabled="uploading || !archive || !mapping">{{ uploading ? '处理中' : '开始上传' }}</button>
        </form>

        <div v-if="uploading || batchProgress" class="kv-progress-stack">
          <div><div class="kv-progress-label"><span>HTTP 上传</span><strong>{{ uploadPercent }}%</strong></div><progress :value="uploadPercent" max="100" /></div>
          <div v-if="batchProgress"><div class="kv-progress-label"><span>转码处理</span><strong>{{ completedCount }}/{{ batchProgress.total_count }}</strong></div><progress :value="transcodePercent" max="100" /></div>
          <p v-if="batchProgress" class="kv-batch-status">批次 #{{ batchProgress.batch_id }} · {{ batchProgress.status }}</p>
          <ul v-if="batchProgress?.videos?.length" class="kv-batch-list"><li v-for="video in batchProgress.videos" :key="video.id"><span>{{ video.knowledge_point_name }}</span><b :data-status="video.status">{{ displayStatus(video.status) }}</b></li></ul>
        </div>
        <p v-if="uploadError" class="kv-error">{{ uploadError }}</p>
        <ul v-if="validationIssues.length" class="kv-issues"><li v-for="(issue, index) in validationIssues" :key="index">{{ issue.message || issue.code }}</li></ul>
      </aside>

      <main class="kv-browser">
        <section class="kv-tree-panel">
          <div class="kv-section-title"><span>02</span><div><h2>知识点列表</h2><p>{{ visibleNodes.length }} 个匹配项</p></div></div>
          <div class="kv-search-row"><input v-model="search" type="search" placeholder="搜索知识点 ID 或名称" aria-label="搜索知识点" /></div>
          <div class="kv-filters" aria-label="视频状态筛选"><button v-for="option in statusOptions" :key="option[0]" type="button" :class="{ active: statusFilter === option[0] }" @click="statusFilter = option[0]">{{ option[1] }}</button></div>
          <p v-if="treeLoading" class="kv-empty">正在加载知识点...</p><p v-else-if="treeError" class="kv-error">{{ treeError }}</p><p v-else-if="!visibleNodes.length" class="kv-empty">暂无匹配知识点</p>
          <div v-else class="kv-tree" role="tree"><button v-for="node in visibleNodes" :key="node.id" class="tree-node" :class="{ selected: selectedNode?.id === node.id }" :style="{ '--depth': node.depth }" type="button" role="treeitem" @click="selectNode(node)"><span class="tree-branch" aria-hidden="true"></span><span class="tree-copy"><strong>{{ node.name }}</strong><small>ID {{ node.id }}</small></span><span class="kv-status" :data-status="node.status">{{ displayStatus(node.status) }}</span></button></div>
        </section>

        <section class="kv-player-panel">
          <div class="kv-section-title"><span>03</span><div><h2>{{ selectedNode?.name || '视频预览' }}</h2><p>{{ selectedNode ? `知识点 ID ${selectedNode.id}` : '从列表选择一个知识点' }}</p></div></div>
          <div v-if="playbackLoading" class="kv-player-empty">正在获取播放地址...</div>
          <div v-else-if="playback?.videos?.length" class="kv-player-grid">
            <div v-for="video in playback.videos" :key="video.knowledge_video_id" class="kv-player-window">
              <HlsPlayer :src="video.playback_url" :title="video.display_name || video.source_file_name || selectedNode.name" :autoplay="false" @play="recordPlayback(video.knowledge_video_id)" />
            </div>
          </div>
          <div v-else class="kv-player-empty"><p>{{ playbackError || (selectedNode ? '暂无可播放视频' : '从列表选择一个知识点') }}</p></div>
        </section>
      </main>
    </div>
  </section>
</template>
