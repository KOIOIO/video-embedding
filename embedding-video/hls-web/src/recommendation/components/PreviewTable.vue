<script setup>
defineProps({
  items: {
    type: Array,
    default: () => [],
  },
  emptyText: {
    type: String,
    default: '暂无预览结果',
  },
})

function formatScore(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) {
    return '0.000'
  }
  return number.toFixed(3)
}
</script>

<template>
  <div class="preview-table-wrap">
    <table v-if="items.length" class="preview-table">
      <thead>
        <tr>
          <th>rank</th>
          <th>segment</th>
          <th>title</th>
          <th>strategy</th>
          <th>score</th>
          <th>range</th>
          <th>play_url</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in items" :key="`${item.rank}-${item.video_segment_id}`">
          <td>{{ item.rank }}</td>
          <td>{{ item.video_segment_id }}</td>
          <td>{{ item.title || '-' }}</td>
          <td>
            <span class="strategy-chip">{{ item.strategy || '-' }}</span>
            <small>{{ item.model_version || 'no model' }}</small>
          </td>
          <td>{{ formatScore(item.recommend_score) }}</td>
          <td>{{ item.start_time_sec }}-{{ item.end_time_sec }}s</td>
          <td>
            <a v-if="item.play_url" :href="item.play_url" target="_blank" rel="noreferrer">{{ item.play_url }}</a>
            <span v-else>-</span>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-else class="empty-state">{{ emptyText }}</p>
  </div>
</template>
