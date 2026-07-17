export const navigationItems = [
  { key: 'diagnostics', label: '诊断中心', hint: 'Health' },
  { key: 'overview', label: '概览', hint: 'Engine' },
  { key: 'datasources', label: '数据源', hint: 'Inputs' },
  { key: 'effects', label: '命中效果', hint: 'Watch' },
  { key: 'trace', label: '链路追踪', hint: 'Trace' },
  { key: 'redis', label: 'Redis 状态', hint: 'Runtime' },
  { key: 'simulator', label: '预览调试', hint: 'Preview' },
]

export const panelRows = [
  ['Random Play', 'Gorse / RecBole / knowledge_match / random fallback'],
  ['Exposure', 'edu_recommend_exposure'],
  ['Watch', 'edu_user_video_recommend'],
  ['User State', 'Redis recent set + random-play bucket'],
]
