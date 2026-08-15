# 项目当前文档全量同步设计

## 目标

以当前代码、配置、部署脚本和生成的 OpenAPI 契约为事实来源，同步仓库中所有面向开发、联调、部署和运维的当前有效文档。更新必须消除已确认的鉴权、配置、接口、推荐训练和部署描述漂移，并保持中文、英文参数文档语义一致。

## 更新范围

本次更新覆盖以下当前入口文档：

- 根目录 `README.md`。
- `docs/README.md`。
- `video-service/README.md`。
- `legacy-video/README.md`。
- `hls-web/README.md`。
- `recbole-training/README.md`。
- 中文参数参考 `PROJECT_PARAMETERS.md`。
- 英文参数参考 `PROJECT_PARAMETERS_EN.md`。
- 部署手册 `deployment/DEPLOYMENT.md`。
- RecBole 算法交接文档 `recbole-training/ALGORITHM_HANDOFF.md`。
- `video-service/docs/downstream-service-readiness-review.md`。
- `video-service/docs/gorse-recommendation-runbook.md`。
- `video-service/docs/recbole-recommendation-pipeline.md`。

若某个 README 的现有事实仍然正确，也要检查其导航、项目边界和当前入口是否完整；不通过无意义的日期或格式改动制造变更。

## 保留范围

以下内容保留原貌：

- `docs/superpowers/` 和 `video-service/docs/superpowers/` 中已有的设计与实施计划。这些文件记录当时决策，不作为当前运行契约。
- HTTP 服务目录下的向量边界设计/实施文档、内容切分执行文档和 `videoapp-test-plan.md`。它们同样是时间点设计或测试计划，不改写为当前运行手册。
- 带明确日期和统计口径的成本报告、架构评审材料及面试材料；它们不是当前参数或运维手册。
- `video-service/docs/swagger/` 下的生成文件。发现契约问题时只报告，不直接手改生成产物。
- `hls-web/dist/`、锁文件、二进制演示文稿和其他构建或生成产物。

## 事实来源与优先级

发生冲突时按以下顺序确定文档内容：

1. 当前运行代码和测试，包括 HTTP 路由、DTO、配置加载器、worker 入口和 RecBole 流水线。
2. 当前配置类型、已提交的本地/生产配置、Compose 文件、环境变量模板和部署脚本。
3. 生成的 Swagger/OpenAPI，用于核对标准 HTTP 路径、请求和响应结构。
4. 现有 README、运行手册和历史设计，仅作为背景与术语参考。

任何仅存在于旧文档、但当前代码或脚本不再支持的参数都必须删除或标明历史属性。示例值必须区分本地配置、HTTP 服务生产配置和服务器交付配置，不能把其中一套值描述成所有环境的统一默认值。

## 内容设计

### README 体系

根 README 负责项目地图、推荐入口、部署形态和文档导航，不重复完整参数表。HTTP 服务 README 负责运行边界、鉴权、接口分类、worker、配置与常用命令。前端 README 负责真实的管理员 JWT 登录流程、工作区、代理路径和前端测试。RecBole README 负责训练输入、预切分文件、虚拟知识点视频 item、数据质量门禁、产物与发布边界。历史 Go README 只说明遗留工程边界和独立验证方式。`docs/README.md` 负责区分当前运行文档与历史决策记录。

### 双语参数参考

中文和英文参数文档保持相同章节结构与同等信息量，完整覆盖：

- 顶层配置及 `Auth`、`KnowledgeVideoStorage`、`KnowledgeVideoWorker`、`Recommendation`、`Gorse` 等当前配置块。
- 全部 Redis key、对象存储、worker、推荐和 AI 参数，并明确默认值与示例环境的区别。
- 当前明确消费的环境变量，包括加载优先级、敏感值要求和进程作用域。
- 生成 OpenAPI 中的全部当前路径分类，以及路由代码中仍需说明的兼容路径。
- 管理员登录与鉴权、分片/归档进度、知识点视频导入/播放/观看会话、推荐管理、内部 RecBole 回退和媒体代理参数。
- RecBole 脚本实际支持的变量、命令行门禁参数和数据质量统计，不记录脚本没有消费的伪环境变量。

OpenAPI 是请求/响应 schema 的最终机器可读契约；参数文档提供调用语义、必填性、默认行为、鉴权边界和运维说明，不复制无法长期维护的生成内容。

### 部署与运行手册

部署文档继续区分根 Compose 联调栈与 `deployment/` 交付拓扑，补齐环境文件选择、JWT、镜像校验、布局校验、平台默认值和服务职责。Gorse、RecBole 和对象存储描述必须与当前 Compose、配置和脚本一致，不把可选集成描述成当前生产主链路。

## 已确认需要修复的事实

- 前端已经使用后端 `/api/auth/login`、`/api/auth/me` 和 Bearer JWT，不能继续描述为浏览器内固定账号校验。
- 当前 Swagger 包含 51 条路径，而双语参数文档只列出 33 个接口标题；缺失路径必须补齐或在分类索引中明确归属。
- 双语参数文档缺少当前鉴权、知识点视频、管理推荐、Redis key 和若干环境变量。
- 知识点视频观看会话会形成 RecBole 训练期虚拟 item，但虚拟 item 不进入验证/测试目标、发布 embedding 或在线候选。
- RecBole 发布门禁还检查正样本行数和正样本用户数；流水线只接受脚本实际读取的环境变量。
- 根 README 不能继续声称 Gorse 使用宿主 Redis；当前配置使用 PostgreSQL store，并保留独立的向量存储设置。
- 部署配置、本地配置和 HTTP 服务生产配置存在不同的超时、对象存储和 Gorse 开关，文档必须分环境描述。

## 验证策略

文档更新完成后执行以下检查：

1. 对照配置类型和 YAML 键，确认双语配置章节没有缺项或不存在的参数。
2. 对照环境变量读取代码、Shell 脚本和环境模板，确认变量名、优先级和默认值。
3. 对照 Swagger 路径与路由注册，确认标准接口覆盖、鉴权分类和兼容路由说明。
4. 运行 Markdown 相对链接检查、`git diff --check` 和中英文标题/接口路径集合对比。
5. 运行与文档事实相关的轻量测试：部署布局校验、前端单元测试/构建、RecBole 单元测试，以及 HTTP 服务的配置、路由和知识点视频相关 Go 测试。
6. 使用无本轮上下文的独立审查者回答典型开发、部署和 API 调用问题，并检查歧义、矛盾与未说明前提。

## 完成标准

- 六个 README 都经过当前实现核对，且职责清晰、互相导航正确。
- 中文和英文参数参考结构对齐，当前配置、环境变量和接口没有已知缺口。
- 鉴权、部署、知识点视频观看信号和 RecBole 发布边界描述与实现一致。
- 所有修改过的命令、路径和相对链接通过自动或人工验证。
- 历史记录和生成产物未被改写。
