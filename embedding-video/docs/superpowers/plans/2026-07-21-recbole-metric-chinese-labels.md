# RecBole 性能指标中文说明 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 RecBole 性能趋势筛选器中显示中文指标名称，并随当前选择展示简短中文解释，同时保持 API 指标值不变。

**Architecture:** 在现有 Vue 单文件组件内维护按标准指标值索引的展示元数据。下拉框将后端返回的指标值映射为本地中文名称，未知值回退到后端标签；计算属性根据当前标准指标值提供动态说明。

**Tech Stack:** Vue 3 Composition API、Vitest、Vite

---

### Task 1: 中文指标名称与动态说明

**Files:**
- Modify: `hls-web/src/recommendation/components/RecBolePerformanceChart.test.js`
- Modify: `hls-web/src/recommendation/components/RecBolePerformanceChart.vue`
- Modify: `hls-web/src/recommendation/recommendation.css`

- [ ] **Step 1: 写入失败的组件契约测试**

在现有 `provides RecBole metrics and stable request states` 测试中增加断言，要求组件包含四个中文名称、四条说明、动态说明区域，并继续使用标准默认指标值：

```js
expect(source).toContain("const metric = ref('NDCG@20')")
expect(source).toContain("'召回率（Recall@20）'")
expect(source).toContain("'排序质量（NDCG@20）'")
expect(source).toContain("'命中率（Hit@20）'")
expect(source).toContain("'准确率（Precision@20）'")
expect(source).toContain('衡量前 20 条推荐覆盖用户感兴趣内容的程度，越高越好。')
expect(source).toContain('衡量前 20 条推荐的相关性和排序位置，相关内容越靠前得分越高。')
expect(source).toContain('衡量前 20 条推荐中至少命中一条用户感兴趣内容的比例，越高越好。')
expect(source).toContain('衡量前 20 条推荐中用户感兴趣内容所占的比例，越高越好。')
expect(source).toContain('class="recbole-metric-description"')
```

- [ ] **Step 2: 运行测试并确认红灯**

Run: `cd hls-web && npm test -- src/recommendation/components/RecBolePerformanceChart.test.js`

Expected: FAIL，提示缺少中文指标名称。

- [ ] **Step 3: 添加最小展示元数据和映射**

在组件脚本中加入固定元数据，默认指标列表继续保留标准 `value`：

```js
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

const displayedMetrics = computed(() => availableMetrics.value.map((option) => ({
  ...option,
  label: metricPresentation[option.value]?.label || option.label || option.value,
})))
const metricDescription = computed(() => (
  metricPresentation[metric.value]?.description || '该指标暂时没有补充说明。'
))
```

模板中的选项遍历改为 `displayedMetrics`，趋势图 `aria-label` 也读取 `displayedMetrics`。在选择框后加入：

```vue
<p class="recbole-metric-description" aria-live="polite">
  {{ metricDescription }}
</p>
```

在组件可继承的推荐页样式中为说明文字设置紧凑字号、可读颜色和稳定宽度，不改变现有筛选器布局：

```css
.recbole-performance-panel :deep(.recbole-metric-description) {
  margin: 0;
  color: #53645f;
  font-size: 0.78rem;
  line-height: 1.5;
}
```

- [ ] **Step 4: 运行组件测试并确认绿灯**

Run: `cd hls-web && npm test -- src/recommendation/components/RecBolePerformanceChart.test.js`

Expected: PASS，组件契约测试全部通过。

- [ ] **Step 5: 运行前端完整测试和构建**

Run: `cd hls-web && npm test`

Expected: PASS，无失败测试。

Run: `cd hls-web && npm run build`

Expected: exit 0，Vite 生成生产构建。

- [ ] **Step 6: 浏览器验证**

启动 `hls-web` 开发服务器，分别使用桌面和移动视口打开推荐控制台的命中效果页。验证四项中文标签、选择后的说明更新、标准指标请求值、文字换行，以及控件之间不存在重叠。

- [ ] **Step 7: 提交实现**

```bash
git add hls-web/src/recommendation/components/RecBolePerformanceChart.vue \
  hls-web/src/recommendation/components/RecBolePerformanceChart.test.js \
  hls-web/src/recommendation/recommendation.css
git commit -m "feat: explain RecBole metrics in Chinese"
```
