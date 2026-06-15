const path = require("path");

const pptxModulePath = process.env.PPTXGENJS_MODULE_PATH || "pptxgenjs";
const pptxgen = require(path.isAbsolute(pptxModulePath)
  ? pptxModulePath
  : pptxModulePath);

const pptx = new pptxgen();

pptx.layout = "LAYOUT_16x9";
pptx.author = "OpenCode";
pptx.company = "OpenCode";
pptx.subject = "智能教学视频分析与推荐系统系统讲解";
pptx.title = "智能教学视频分析与推荐系统系统架构与工程设计";
pptx.lang = "zh-CN";

const C = {
  ink: "1F2937",
  slate: "475569",
  sky: "DCEAFE",
  blue: "1D4ED8",
  navy: "0F172A",
  teal: "0F766E",
  mint: "CCFBF1",
  amber: "B45309",
  sand: "FEF3C7",
  rose: "9F1239",
  pink: "FCE7F3",
  red: "DC2626",
  redSoft: "FEE2E2",
  green: "166534",
  greenSoft: "DCFCE7",
  grayBg: "F8FAFC",
  line: "CBD5E1",
  white: "FFFFFF",
};

function baseSlide(title, opts = {}) {
  const slide = pptx.addSlide();
  slide.background = { color: opts.background || C.grayBg };
  slide.addShape(pptx.ShapeType.rect, {
    x: 0, y: 0, w: 10, h: 0.45,
    fill: { color: opts.topBar || C.navy },
    line: { color: opts.topBar || C.navy },
  });
  slide.addText(title, {
    x: 0.55, y: 0.56, w: 6.8, h: 0.45,
    fontFace: "Microsoft YaHei",
    fontSize: 24,
    bold: true,
    color: opts.titleColor || C.ink,
    margin: 0,
  });
  slide.addText(opts.section || "Hengtao Video Service", {
    x: 7.45, y: 0.6, w: 1.95, h: 0.28,
    fontFace: "Calibri",
    fontSize: 10,
    color: C.slate,
    align: "right",
    margin: 0,
  });
  slide.addShape(pptx.ShapeType.line, {
    x: 0.55, y: 1.06, w: 8.85, h: 0,
    line: { color: C.line, width: 1 },
  });
  return slide;
}

function addFooter(slide, text = "智能教学视频分析与推荐系统 | 架构讲解") {
  slide.addText(text, {
    x: 0.55, y: 5.18, w: 4.6, h: 0.18,
    fontFace: "Calibri",
    fontSize: 9,
    color: "64748B",
    margin: 0,
  });
}

function addBulletList(slide, items, x, y, w, h, opts = {}) {
  const runs = [];
  items.forEach((item, idx) => {
    runs.push({
      text: item,
      options: { bullet: true, breakLine: idx !== items.length - 1 },
    });
  });
  slide.addText(runs, {
    x, y, w, h,
    fontFace: "Microsoft YaHei",
    fontSize: opts.fontSize || 15,
    color: opts.color || C.ink,
    breakLine: true,
    valign: "top",
    margin: 0.04,
    paraSpaceAfterPt: opts.paraSpaceAfterPt || 10,
  });
}

function addCard(slide, x, y, w, h, title, body, color, opts = {}) {
  slide.addShape(pptx.ShapeType.roundRect, {
    x, y, w, h,
    rectRadius: 0.08,
    fill: { color: opts.fill || C.white },
    line: { color: opts.line || C.line, width: 1 },
  });
  slide.addShape(pptx.ShapeType.rect, {
    x, y, w: 0.13, h,
    fill: { color },
    line: { color },
  });
  slide.addText(title, {
    x: x + 0.22, y: y + 0.14, w: w - 0.32, h: 0.24,
    fontFace: "Microsoft YaHei",
    fontSize: 16,
    bold: true,
    color: C.ink,
    margin: 0,
  });
  slide.addText(body, {
    x: x + 0.22, y: y + 0.43, w: w - 0.32, h: h - 0.52,
    fontFace: "Microsoft YaHei",
    fontSize: opts.fontSize || 11,
    color: C.slate,
    valign: "top",
    margin: 0,
    breakLine: false,
  });
}

function addTag(slide, text, x, y, w, color, fill) {
  slide.addShape(pptx.ShapeType.roundRect, {
    x, y, w, h: 0.28,
    rectRadius: 0.05,
    fill: { color: fill },
    line: { color: fill },
  });
  slide.addText(text, {
    x, y: y + 0.03, w, h: 0.16,
    fontFace: "Microsoft YaHei",
    fontSize: 9,
    bold: true,
    color,
    align: "center",
    margin: 0,
  });
}

function addArrow(slide, x, y, w, h, color) {
  slide.addShape(pptx.ShapeType.chevron, {
    x, y, w, h,
    fill: { color },
    line: { color },
  });
}

function addTimelineBox(slide, x, y, w, h, n, title, desc, fill, numColor) {
  slide.addShape(pptx.ShapeType.roundRect, {
    x, y, w, h,
    rectRadius: 0.08,
    fill: { color: fill },
    line: { color: fill },
  });
  slide.addShape(pptx.ShapeType.ellipse, {
    x: x + 0.12, y: y + 0.11, w: 0.42, h: 0.42,
    fill: { color: numColor },
    line: { color: numColor },
  });
  slide.addText(String(n), {
    x: x + 0.12, y: y + 0.20, w: 0.42, h: 0.12,
    fontFace: "Calibri",
    fontSize: 16,
    bold: true,
    color: C.white,
    align: "center",
    margin: 0,
  });
  slide.addText(title, {
    x: x + 0.62, y: y + 0.12, w: w - 0.72, h: 0.2,
    fontFace: "Microsoft YaHei",
    fontSize: 13,
    bold: true,
    color: C.ink,
    margin: 0,
  });
  slide.addText(desc, {
    x: x + 0.62, y: y + 0.34, w: w - 0.72, h: h - 0.42,
    fontFace: "Microsoft YaHei",
    fontSize: 10,
    color: C.slate,
    margin: 0,
    valign: "top",
  });
}

function addSectionDivider(slide, text, x, y, w, fill, color) {
  slide.addShape(pptx.ShapeType.roundRect, {
    x, y, w, h: 0.34,
    rectRadius: 0.05,
    fill: { color: fill },
    line: { color: fill },
  });
  slide.addText(text, {
    x, y: y + 0.05, w, h: 0.16,
    fontFace: "Microsoft YaHei",
    fontSize: 11,
    bold: true,
    color,
    align: "center",
    margin: 0,
  });
}

async function main() {
// Slide 1: Cover
{
  const slide = pptx.addSlide();
  slide.background = { color: C.navy };
  slide.addShape(pptx.ShapeType.rect, {
    x: 0, y: 0, w: 10, h: 5.625,
    fill: { color: C.navy },
    line: { color: C.navy },
  });
  slide.addShape(pptx.ShapeType.rect, {
    x: 0.55, y: 0.72, w: 3.65, h: 3.7,
    fill: { color: C.blue, transparency: 65 },
    line: { color: C.blue, transparency: 100 },
  });
  slide.addShape(pptx.ShapeType.rect, {
    x: 4.55, y: 1.05, w: 4.85, h: 2.85,
    fill: { color: "1E3A8A", transparency: 30 },
    line: { color: "1E3A8A", transparency: 100 },
  });
  slide.addShape(pptx.ShapeType.line, {
    x: 4.9, y: 1.55, w: 3.6, h: 0,
    line: { color: C.mint, width: 2 },
  });
  slide.addText("智能教学视频分析与推荐系统\n系统架构与工程设计", {
    x: 0.78, y: 1.16, w: 5.7, h: 1.5,
    fontFace: "Microsoft YaHei",
    fontSize: 25,
    bold: true,
    color: C.white,
    breakLine: true,
    margin: 0,
    valign: "mid",
  });
  slide.addText("面向 Java 业务系统的视频处理、推荐与播放服务", {
    x: 0.82, y: 2.95, w: 4.9, h: 0.32,
    fontFace: "Microsoft YaHei",
    fontSize: 13,
    color: "BFDBFE",
    margin: 0,
  });
  slide.addText("汇报重点", {
    x: 5.0, y: 1.8, w: 1.3, h: 0.2,
    fontFace: "Microsoft YaHei",
    fontSize: 12,
    bold: true,
    color: C.white,
    margin: 0,
  });
  addBulletList(slide, [
    "系统定位与服务边界",
    "总体架构与核心处理链路",
    "设计亮点、工程优势与现实约束",
    "风险、限制与后续演进方向",
  ], 5.0, 2.1, 3.9, 1.55, { fontSize: 14, color: C.white, paraSpaceAfterPt: 8 });
  slide.addText("适用对象：技术评审 / 业务管理 / 客户汇报", {
    x: 0.82, y: 4.82, w: 5.8, h: 0.22,
    fontFace: "Microsoft YaHei",
    fontSize: 11,
    color: "CBD5E1",
    margin: 0,
  });
}

// Slide 2: Executive summary
{
  const slide = baseSlide("执行摘要", { section: "Overview" });
  addTag(slide, "系统判断", 0.58, 1.25, 0.92, C.blue, C.sky);
  slide.addText("这是一个面向 Java 业务系统的独立视频 HTTP 后端，不只做上传与播放，还承担视频处理、推荐、播放代理、观看回流与向量化扩展能力。", {
    x: 0.58, y: 1.62, w: 8.82, h: 0.55,
    fontFace: "Microsoft YaHei",
    fontSize: 16,
    bold: true,
    color: C.ink,
    margin: 0,
  });
  addCard(slide, 0.6, 2.35, 2.7, 1.55, "对业务的价值", "统一视频上传、转码、推荐与播放服务，减少 Java 主系统承载的媒体处理复杂度。", C.blue, { fill: C.white, fontSize: 12 });
  addCard(slide, 3.45, 2.35, 2.7, 1.55, "对技术的价值", "通过 HTTP API + Redis Streams + Worker 把同步请求与长耗时任务拆开，形成可演进架构。", C.teal, { fill: C.white, fontSize: 12 });
  addCard(slide, 6.3, 2.35, 3.1, 1.55, "必须同时看到的事实", "当前方案可部署、能力完整，但外部依赖重、异步链路长、资源治理门槛不低。", C.rose, { fill: C.white, fontSize: 12 });
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.6, y: 4.25, w: 8.8, h: 0.62,
    rectRadius: 0.06,
    fill: { color: C.navy },
    line: { color: C.navy },
  });
  slide.addText("一句话结论：这是一个工程上务实、链路完整、适合中等规模教育视频场景的综合后端，但并不是零治理成本的平台化方案。", {
    x: 0.85, y: 4.43, w: 8.3, h: 0.18,
    fontFace: "Microsoft YaHei",
    fontSize: 13,
    bold: true,
    color: C.white,
    margin: 0,
    align: "center",
  });
  addFooter(slide);
}

// Slide 3: Background and goals
{
  const slide = baseSlide("项目背景与建设目标", { section: "Context" });
  addSectionDivider(slide, "为什么独立出来做视频服务", 0.62, 1.24, 2.15, C.sky, C.blue);
  addTimelineBox(slide, 0.62, 1.7, 2.2, 1.03, 1, "主系统减负", "Java 系统不再直接承担媒体处理细节，把复杂的上传、转码、资源组织转交专门服务。", C.white, C.blue);
  addTimelineBox(slide, 0.62, 2.9, 2.2, 1.03, 2, "异步化处理", "视频转码、封面生成、向量化都属于长耗时任务，不适合同步阻塞主请求。", C.white, C.teal);
  addTimelineBox(slide, 0.62, 4.1, 2.2, 0.78, 3, "统一对外接口", "通过 REST API + Swagger 降低 Java 接入和跨团队协作成本。", C.white, C.rose);

  slide.addShape(pptx.ShapeType.roundRect, {
    x: 3.18, y: 1.4, w: 6.08, h: 3.6,
    rectRadius: 0.08,
    fill: { color: C.white },
    line: { color: C.line, width: 1 },
  });
  slide.addText("本系统希望同时达成的四类目标", {
    x: 3.45, y: 1.62, w: 3.2, h: 0.22,
    fontFace: "Microsoft YaHei",
    fontSize: 17,
    bold: true,
    color: C.ink,
    margin: 0,
  });
  addCard(slide, 3.45, 2.0, 2.58, 1.15, "能力解耦", "把视频能力从主业务系统中独立出来，形成稳定接口边界。", C.blue, { fill: C.sky, line: C.sky, fontSize: 11 });
  addCard(slide, 6.2, 2.0, 2.58, 1.15, "处理闭环", "从上传、转码到播放、推荐与回流，形成完整业务链路。", C.teal, { fill: C.mint, line: C.mint, fontSize: 11 });
  addCard(slide, 3.45, 3.35, 2.58, 1.15, "接入统一", "新调用统一走 REST 路径，减少历史兼容路径继续蔓延。", C.amber, { fill: C.sand, line: C.sand, fontSize: 11 });
  addCard(slide, 6.2, 3.35, 2.58, 1.15, "推荐增强", "借助 pgvector、ASR 与 Embedding，为题目关联推荐提供基础能力。", C.rose, { fill: C.pink, line: C.pink, fontSize: 11 });
  addFooter(slide);
}

// Slide 4: Scope
{
  const slide = baseSlide("系统边界", { section: "Scope" });
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.62, y: 1.4, w: 4.06, h: 3.35,
    rectRadius: 0.08,
    fill: { color: C.white },
    line: { color: C.greenSoft, width: 1.4 },
  });
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 5.1, y: 1.4, w: 4.28, h: 3.35,
    rectRadius: 0.08,
    fill: { color: C.white },
    line: { color: C.redSoft, width: 1.4 },
  });
  slide.addText("系统负责什么", {
    x: 0.88, y: 1.62, w: 1.9, h: 0.2,
    fontFace: "Microsoft YaHei", fontSize: 17, bold: true, color: C.green, margin: 0,
  });
  slide.addText("系统不负责什么", {
    x: 5.36, y: 1.62, w: 2.2, h: 0.2,
    fontFace: "Microsoft YaHei", fontSize: 17, bold: true, color: C.red, margin: 0,
  });
  addBulletList(slide, [
    "视频上传、列表、删除、发布与推荐状态维护",
    "HLS 转码、封面处理与任务状态查询",
    "对象存储中视频与切片资源的代理访问",
    "题目关联推荐、相似视频与观看记录回流",
    "题库查询与向量化扩展的数据基础建设",
  ], 0.88, 2.0, 3.45, 2.3, { fontSize: 13 });
  addBulletList(slide, [
    "超大规模 CDN 分发和全国边缘加速体系",
    "低延迟实时直播或音视频通话平台",
    "复杂多租户媒体编排与强隔离平台",
    "通用 AI 平台或独立模型训练系统",
    "无限扩展的海量吞吐承诺",
  ], 5.36, 2.0, 3.6, 2.3, { fontSize: 13 });
  addFooter(slide, "边界清晰比能力堆砌更重要");
}

// Slide 5: Architecture
{
  const slide = baseSlide("总体架构", { section: "Architecture" });
  addSectionDivider(slide, "接入层", 0.62, 1.3, 1.15, C.sky, C.blue);
  addSectionDivider(slide, "应用层", 2.32, 1.3, 1.15, C.mint, C.teal);
  addSectionDivider(slide, "异步层", 5.18, 1.3, 1.15, C.sand, C.amber);
  addSectionDivider(slide, "基础设施", 7.95, 1.3, 1.15, C.pink, C.rose);

  addCard(slide, 0.62, 1.72, 1.55, 1.0, "Java 系统", "通过 HTTP/JSON 调用上传、播放、推荐、回流等接口。", C.blue, { fill: C.white, fontSize: 10 });
  addCard(slide, 0.62, 3.0, 1.55, 1.0, "浏览器/播放器", "消费播放地址与 HLS 资源，通过代理接口访问对象存储。", C.blue, { fill: C.white, fontSize: 10 });
  addCard(slide, 2.32, 1.72, 2.28, 2.28, "cmd/httpapi + videoapp.Service", "统一承载 REST API、业务编排、上传启动逻辑、任务投递、推荐查询与观看记录上报。", C.teal, { fill: C.white, fontSize: 11 });
  addCard(slide, 5.18, 1.72, 2.02, 1.02, "transcode worker", "消费转码任务，调用 FFmpeg 生成 HLS 切片与封面，回写状态。", C.amber, { fill: C.white, fontSize: 10 });
  addCard(slide, 5.18, 2.98, 2.02, 1.02, "vector worker", "执行 ASR、分段、Embedding、片段精修，为推荐检索写入向量数据。", C.amber, { fill: C.white, fontSize: 10 });
  addCard(slide, 7.95, 1.72, 1.45, 0.78, "PostgreSQL + pgvector", "元数据、片段、向量检索", C.rose, { fill: C.white, fontSize: 9 });
  addCard(slide, 7.95, 2.65, 1.45, 0.78, "Redis Streams", "任务队列、状态、活跃计数", C.rose, { fill: C.white, fontSize: 9 });
  addCard(slide, 7.95, 3.58, 1.45, 0.78, "S3 / RustFS", "原视频、HLS 切片、封面", C.rose, { fill: C.white, fontSize: 9 });
  addCard(slide, 7.95, 4.51, 1.45, 0.78, "FFmpeg / AI", "媒体处理与内容理解依赖", C.rose, { fill: C.white, fontSize: 9 });
  addArrow(slide, 2.02, 2.1, 0.24, 0.3, C.blue);
  addArrow(slide, 4.78, 2.1, 0.24, 0.3, C.teal);
  addArrow(slide, 7.46, 2.1, 0.24, 0.3, C.amber);
  slide.addShape(pptx.ShapeType.line, {
    x: 3.45, y: 4.36, w: 4.95, h: 0,
    line: { color: C.line, width: 1.2, dash: "dash" },
  });
  slide.addText("架构关键词：统一 HTTP 接入 + 长任务异步化 + 文件与元数据分离 + 推荐链路可降级", {
    x: 2.32, y: 4.52, w: 5.3, h: 0.2,
    fontFace: "Microsoft YaHei", fontSize: 11, color: C.slate, margin: 0,
  });
  addFooter(slide);
}

// Slide 6: Core flows
{
  const slide = baseSlide("核心业务链路", { section: "Flows" });
  addTimelineBox(slide, 0.62, 1.55, 8.78, 0.7, 1, "上传与转码", "上传接口先保存对象与视频记录，再投递转码任务；worker 成功后才 ACK，并生成 HLS 与封面。", C.white, C.blue);
  addTimelineBox(slide, 0.62, 2.45, 8.78, 0.7, 2, "推荐与向量化", "向量化 worker 下载原视频，做粗分段、ASR、LLM 精修、Embedding，并写入片段与向量。", C.white, C.teal);
  addTimelineBox(slide, 0.62, 3.35, 8.78, 0.7, 3, "播放与资源代理", "Java 或前端请求播放信息；播放器再通过 /videos/*filepath 代理访问对象存储中的 HLS 资源。", C.white, C.amber);
  addTimelineBox(slide, 0.62, 4.25, 8.78, 0.7, 4, "观看记录回流", "观看完成或中途退出时上报观看记录，为运营分析与后续推荐闭环提供数据。", C.white, C.rose);
  slide.addText("并发特征", {
    x: 0.62, y: 5.0, w: 0.8, h: 0.18,
    fontFace: "Microsoft YaHei", fontSize: 11, bold: true, color: C.ink, margin: 0,
  });
  slide.addText("真实压力不是单一队列长度，而是“多个视频并发处理”叠加“单视频内部多阶段并发”，最终共同竞争 FFmpeg、AI、对象存储和数据库。", {
    x: 1.48, y: 4.98, w: 7.9, h: 0.24,
    fontFace: "Microsoft YaHei", fontSize: 10, color: C.slate, margin: 0,
  });
  addFooter(slide);
}

// Slide 7: Modules
{
  const slide = baseSlide("关键模块拆解", { section: "Modules" });
  addCard(slide, 0.62, 1.45, 2.06, 1.45, "接入层", "Gin Router + REST API\n职责：协议适配、参数绑定、统一 JSON 返回与 Swagger 协作。\n代价：同时保留兼容路径会增加维护成本。", C.blue, { fill: C.sky, line: C.sky, fontSize: 11 });
  addCard(slide, 2.96, 1.45, 2.06, 1.45, "应用层", "videoapp.Service\n职责：聚合上传、播放、推荐、题库与回流逻辑。\n代价：如果继续扩张，服务层会越来越像编排中心。", C.teal, { fill: C.mint, line: C.mint, fontSize: 11 });
  addCard(slide, 5.3, 1.45, 2.06, 1.45, "异步层", "transcode worker / vector worker\n职责：承载长任务、失败重试与状态回写。\n代价：链路长、定位问题需要更强观测性。", C.amber, { fill: C.sand, line: C.sand, fontSize: 11 });
  addCard(slide, 7.64, 1.45, 1.74, 1.45, "存储与外部依赖", "PostgreSQL、Redis、S3、FFmpeg、AI provider。\n代价：故障面多。", C.rose, { fill: C.pink, line: C.pink, fontSize: 11 });

  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.62, y: 3.3, w: 8.76, h: 1.35,
    rectRadius: 0.08,
    fill: { color: C.white },
    line: { color: C.line, width: 1 },
  });
  slide.addText("模块协作关系", {
    x: 0.86, y: 3.54, w: 1.4, h: 0.18,
    fontFace: "Microsoft YaHei", fontSize: 15, bold: true, color: C.ink, margin: 0,
  });
  slide.addText("接入层负责“接单”，应用层负责“编排”，异步层负责“耗时执行”，存储与外部依赖负责“落地与计算”。当前边界清晰，但随着推荐能力增强，应用层与异步层之间的责任还会继续细化。", {
    x: 0.86, y: 3.86, w: 8.1, h: 0.46,
    fontFace: "Microsoft YaHei", fontSize: 12, color: C.slate, margin: 0,
  });
  addFooter(slide);
}

// Slide 8: Principles
{
  const slide = baseSlide("设计思想与架构原则", { section: "Principles" });
  addCard(slide, 0.62, 1.42, 2.05, 1.2, "服务解耦", "让 Java 主系统通过 HTTP 访问视频能力，而不是卷入媒体处理实现细节。", C.blue, { fill: C.white, fontSize: 11 });
  addCard(slide, 2.95, 1.42, 2.05, 1.2, "异步优先", "把转码、封面、向量化等长耗时任务从同步请求中剥离。", C.teal, { fill: C.white, fontSize: 11 });
  addCard(slide, 5.28, 1.42, 2.05, 1.2, "存算分离", "对象存储承载媒体文件，数据库承载元数据、状态与向量检索。", C.amber, { fill: C.white, fontSize: 11 });
  addCard(slide, 7.61, 1.42, 1.77, 1.2, "降级优先", "AI 抖动时尽量返回可用结果，而不是直接把推荐接口打挂。", C.rose, { fill: C.white, fontSize: 11 });
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.62, y: 3.0, w: 8.76, h: 1.62,
    rectRadius: 0.08,
    fill: { color: C.navy },
    line: { color: C.navy },
  });
  slide.addText("为什么这些原则对这个系统重要", {
    x: 0.9, y: 3.22, w: 2.6, h: 0.2,
    fontFace: "Microsoft YaHei", fontSize: 16, bold: true, color: C.white, margin: 0,
  });
  addBulletList(slide, [
    "视频业务不是单一 CRUD，真正复杂的是大文件、异步任务、媒体处理和推荐协同。",
    "如果没有解耦和异步化，Java 主系统会直接承受慢请求、重依赖和失败恢复问题。",
    "如果没有降级与渐进演进思路，AI 链路会让系统很难稳定落地。",
  ], 0.9, 3.56, 7.8, 0.86, { fontSize: 12, color: C.white, paraSpaceAfterPt: 7 });
  addFooter(slide);
}

// Slide 9: Highlights
{
  const slide = baseSlide("设计亮点", { section: "Highlights" });
  addCard(slide, 0.62, 1.45, 2.0, 1.48, "亮点 1", "HTTP 服务与 worker 分离，接口响应与长任务处理职责清晰。", C.blue, { fill: C.sky, line: C.sky, fontSize: 12 });
  addCard(slide, 2.88, 1.45, 2.0, 1.48, "亮点 2", "Redis Streams 使用消费者组、成功后 ACK、DLQ，基础可靠性优于简单即时确认。", C.teal, { fill: C.mint, line: C.mint, fontSize: 12 });
  addCard(slide, 5.14, 1.45, 2.0, 1.48, "亮点 3", "推荐链路支持 AI provider 异常时的降级处理，不把外部抖动直接暴露给调用方。", C.amber, { fill: C.sand, line: C.sand, fontSize: 12 });
  addCard(slide, 7.4, 1.45, 1.98, 1.48, "亮点 4", "阶段任务结构已落地，为后续向量链路拆分成多阶段 worker 预留空间。", C.rose, { fill: C.pink, line: C.pink, fontSize: 11 });
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.62, y: 3.35, w: 8.76, h: 1.18,
    rectRadius: 0.08,
    fill: { color: C.white },
    line: { color: C.line, width: 1 },
  });
  slide.addText("注意：亮点不是“没有问题”，而是“在当前业务目标下做出了合理取舍”。", {
    x: 0.92, y: 3.7, w: 8.1, h: 0.22,
    fontFace: "Microsoft YaHei", fontSize: 14, bold: true, color: C.ink, margin: 0, align: "center",
  });
  addFooter(slide);
}

// Slide 10: Advantages
{
  const slide = baseSlide("处理优势与工程收益", { section: "Advantages" });
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.62, y: 1.42, w: 3.08, h: 3.45,
    rectRadius: 0.08,
    fill: { color: C.navy },
    line: { color: C.navy },
  });
  slide.addText("工程收益", {
    x: 0.88, y: 1.7, w: 1.2, h: 0.18,
    fontFace: "Microsoft YaHei", fontSize: 17, bold: true, color: C.white, margin: 0,
  });
  addBulletList(slide, [
    "同步接口不被长任务拖住，用户和主系统响应更稳定。",
    "多视频并发 + 单视频内部并发，提升整体处理吞吐。",
    "对象存储降低本地磁盘耦合，媒体文件组织更清晰。",
    "视频、题目、推荐、回流放在同一体系内，更容易形成业务闭环。",
  ], 0.88, 2.05, 2.45, 2.4, { fontSize: 12, color: C.white, paraSpaceAfterPt: 8 });
  addCard(slide, 4.05, 1.55, 2.48, 1.38, "对业务方", "上线的是一整套视频能力，而不是只会“接收文件”的单点接口。", C.blue, { fill: C.white, fontSize: 12 });
  addCard(slide, 6.72, 1.55, 2.48, 1.38, "对技术方", "具备可继续拆分、扩容和治理的结构基础，而不是一步到位的僵硬实现。", C.teal, { fill: C.white, fontSize: 12 });
  addCard(slide, 4.05, 3.18, 2.48, 1.38, "对客户方", "可以清楚说明功能完整性、推荐增强能力和接入方式。", C.amber, { fill: C.white, fontSize: 12 });
  addCard(slide, 6.72, 3.18, 2.48, 1.38, "对后续建设", "现有设计已经给监控、队列治理、阶段拆分和质量优化留出了抓手。", C.rose, { fill: C.white, fontSize: 12 });
  addFooter(slide);
}

// Slide 11: Risks
{
  const slide = baseSlide("风险、短板与当前限制", { section: "Trade-offs" });
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.62, y: 1.42, w: 8.76, h: 3.48,
    rectRadius: 0.08,
    fill: { color: C.white },
    line: { color: C.redSoft, width: 1.4 },
  });
  addCard(slide, 0.86, 1.72, 2.02, 1.1, "依赖重", "HTTP 服务启动即依赖 PostgreSQL、Redis、对象存储和部分数据库扩展。", C.red, { fill: C.redSoft, line: C.redSoft, fontSize: 11 });
  addCard(slide, 3.08, 1.72, 2.02, 1.1, "资源竞争", "统一 worker 入口简单，但转码与向量化在同一进程中更容易互相争抢资源。", C.red, { fill: C.redSoft, line: C.redSoft, fontSize: 11 });
  addCard(slide, 5.3, 1.72, 2.02, 1.1, "外部波动", "FFmpeg、ASR、Embedding、LLM 的抖动会直接影响处理时延和稳定性。", C.red, { fill: C.redSoft, line: C.redSoft, fontSize: 11 });
  addCard(slide, 7.52, 1.72, 1.62, 1.1, "规模边界", "更适合中等规模，不宜直接承诺超大规模能力。", C.red, { fill: C.redSoft, line: C.redSoft, fontSize: 11 });
  slide.addText("这些问题不等于系统不可用，而是说明它需要配套的容量规划、监控告警和演进节奏。", {
    x: 1.0, y: 3.55, w: 8.0, h: 0.22,
    fontFace: "Microsoft YaHei", fontSize: 14, bold: true, color: C.ink, margin: 0, align: "center",
  });
  addFooter(slide);
}

// Slide 12: Operations
{
  const slide = baseSlide("运维与治理挑战", { section: "Operations" });
  addSectionDivider(slide, "需要长期盯住的四类信号", 0.62, 1.28, 2.18, C.sky, C.blue);
  addTimelineBox(slide, 0.62, 1.75, 4.2, 0.88, 1, "任务堆积", "转码队列、向量队列、死信流是否持续增长，是否出现消费滞后。", C.white, C.blue);
  addTimelineBox(slide, 5.0, 1.75, 4.2, 0.88, 2, "共享资源压力", "FFmpeg CPU、对象存储延迟、数据库连接和 AI provider 限流是否成为瓶颈。", C.white, C.teal);
  addTimelineBox(slide, 0.62, 2.95, 4.2, 0.88, 3, "失败分类", "哪些是可重试失败，哪些是终态失败；错误分类不清会放大重试成本。", C.white, C.amber);
  addTimelineBox(slide, 5.0, 2.95, 4.2, 0.88, 4, "质量与降级", "推荐链路降级时能否被识别、告知并持续评估结果质量。", C.white, C.rose);
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.62, y: 4.25, w: 8.58, h: 0.58,
    rectRadius: 0.06,
    fill: { color: C.navy },
    line: { color: C.navy },
  });
  slide.addText("稳定运行不仅靠代码正确，还靠监控、容量治理和故障处理机制持续跟上。", {
    x: 0.95, y: 4.44, w: 7.95, h: 0.18,
    fontFace: "Microsoft YaHei", fontSize: 13, bold: true, color: C.white, margin: 0, align: "center",
  });
  addFooter(slide);
}

// Slide 13: Fit / non-fit
{
  const slide = baseSlide("适用场景与不适用场景", { section: "Fit" });
  addCard(slide, 0.62, 1.48, 4.06, 3.12, "适用场景", "1. 教育视频平台，尤其是题目与视频片段存在关联关系的场景。\n2. 中等规模内容服务，需要上传、转码、封面、播放、推荐与回流的完整闭环。\n3. 需要对接 Java 主系统，但又不希望把媒体处理细节塞回主服务的团队。", C.green, { fill: C.greenSoft, line: C.greenSoft, fontSize: 13 });
  addCard(slide, 5.12, 1.48, 4.26, 3.12, "不适用场景", "1. 超大规模直播分发或强边缘节点调度平台。\n2. 极低延迟实时音视频处理。\n3. 强多租户隔离、复杂计费和深度编排要求的平台化场景。\n4. 希望“几乎零运维”地承接所有媒体任务的团队。", C.red, { fill: C.redSoft, line: C.redSoft, fontSize: 13 });
  addFooter(slide);
}

// Slide 14: Roadmap and summary
{
  const slide = baseSlide("演进方向与总结", { section: "Roadmap" });
  addCard(slide, 0.62, 1.48, 2.0, 1.25, "下一步 1", "将统一 worker 逐步拆分部署，减少转码与向量化互相争抢资源。", C.blue, { fill: C.sky, line: C.sky, fontSize: 12 });
  addCard(slide, 2.88, 1.48, 2.0, 1.25, "下一步 2", "继续推进向量化链路阶段化和分布式化，降低单 worker 复杂度。", C.teal, { fill: C.mint, line: C.mint, fontSize: 12 });
  addCard(slide, 5.14, 1.48, 2.0, 1.25, "下一步 3", "补强任务监控、死信治理、限流与容量规划，提升可运维性。", C.amber, { fill: C.sand, line: C.sand, fontSize: 12 });
  addCard(slide, 7.4, 1.48, 1.98, 1.25, "下一步 4", "优化推荐质量评估、缓存和兼容路径收敛。", C.rose, { fill: C.pink, line: C.pink, fontSize: 12 });
  slide.addShape(pptx.ShapeType.roundRect, {
    x: 0.62, y: 3.18, w: 8.76, h: 1.22,
    rectRadius: 0.08,
    fill: { color: C.navy },
    line: { color: C.navy },
  });
  slide.addText("总结", {
    x: 0.95, y: 3.45, w: 0.8, h: 0.2,
    fontFace: "Microsoft YaHei", fontSize: 18, bold: true, color: C.white, margin: 0,
  });
  slide.addText("这是一套工程上务实、能力较完整、已经具备落地价值的视频服务架构。它的优势在于链路闭环、异步解耦和推荐扩展能力；它的挑战在于依赖重、治理复杂、扩展到更大规模时必须继续拆分与优化。", {
    x: 1.8, y: 3.35, w: 7.15, h: 0.54,
    fontFace: "Microsoft YaHei", fontSize: 14, color: C.white, margin: 0,
  });
  addFooter(slide, "结论：适合当前目标，但后续成长依赖治理能力");
}

await pptx.writeFile({ fileName: "docs/presentations/hengtao-video-service-architecture-review.pptx" });
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
