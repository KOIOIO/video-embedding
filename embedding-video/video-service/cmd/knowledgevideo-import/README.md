# knowledgevideo-import 知识点视频初始化导入工具

`cmd/knowledgevideo-import` 是运维/初始化场景下的知识点视频批量导入工具：它直接扫描源 MinIO 对象前缀，跳过 HTTP、ZIP 上传和浏览器鉴权，把源视频复制到知识视频对象存储，创建批次与视频记录，并向转码队列投递任务。

适用场景：

- 首次上线时把已有的教学视频（大量、按批次组织）初始化到系统。
- 从旧存储迁移知识点视频，且迁移源是 S3/MinIO 兼容桶。
- 单次、可重复执行的导入（工具提供 dry-run 与幂等 key）。

> 该工具只做「初始化导入」。运行中的日常导入应继续使用 HTTP 接口 `POST /api/admin/knowledge-videos/batches`。

## 工作原理

```text
源 MinIO（SOURCE_MINIO_* 环境变量）
  └─ ListPrefix 扫描 --source-prefix 下的对象
       └─ XLSX 映射表（严格校验 id/name/video_name）
            ├─ 校验知识点 ID 与字典名称
            ├─ 校验视频 basename/相对路径是否唯一存在
            └─ 校验扩展名是否受支持
                 └─ 流式复制唯一源对象到知识视频对象存储
                      └─ 创建批次 + 视频记录 + 投递转码任务（knowledge_video:transcode:stream）
```

要点：

- 同一个源对象可被多行引用、挂到多个知识点；每个唯一对象只流式复制一次。
- 每行仍创建独立的视频 ID、HLS 前缀和转码任务。
- **视频级去重**：已入库（未软删除）的视频按「知识点 ID + 源文件名」识别；重新执行导入时，映射中已经上传过的行会被直接跳过，只补充新素材，不会重复入库。
- PostgreSQL、Redis、知识视频目标对象存储读取项目配置；源 MinIO 只通过 `SOURCE_MINIO_*` 环境变量传入。

## 前置条件

| 依赖 | 说明 |
|---|---|
| PostgreSQL | 目标库必须可达（`configs/video.yml` 或 `CONFIG_FILE` 指定，可用 `POSTGRES_DSN` 覆盖） |
| Redis | 单批次模式与批量正式执行需要（`REDIS_ADDR` / `REDIS_PASSWORD` 可覆盖） |
| 知识视频对象存储 | `KnowledgeVideoStorage` 配置块；凭证用 `COS_SECRET_ID`/`COS_SECRET_KEY` 或 `RUSTFS_ACCESS_KEY`/`RUSTFS_SECRET_KEY` 注入 |
| 源 MinIO | 必须提供 `SOURCE_MINIO_*` 环境变量 |

配置加载规则与 `cmd/httpapi` 一致：在 `video-service/` 目录下运行；macOS/Windows 默认 `configs/video.yml`，其他环境默认 `configs/video_prod.yml`，`CONFIG_FILE` 优先生效；启动时会按 `VIDEO_APP_ENV_FILE` 加载 dotenv 文件，shell 已有变量优先。

## 快速开始

```bash
cd video-service

# 1) 先预览源前缀下有哪些对象（不访问数据库）
SOURCE_MINIO_ENDPOINT=http://minio.example:9000 \
SOURCE_MINIO_BUCKET=source-bucket \
SOURCE_MINIO_ACCESS_KEY=<access-key> \
SOURCE_MINIO_SECRET_KEY=<secret-key> \
go run ./cmd/knowledgevideo-import \
  --list-only --source-prefix '_video/xlsx视频'

# 2) 检查单个映射文件的表头与校验问题
... go run ./cmd/knowledgevideo-import \
  --source-prefix '_video/xlsx视频' \
  --inspect-object 'batch-0001.xlsx'

# 3) dry-run 校验并统计（不写入任何数据）
... go run ./cmd/knowledgevideo-import \
  --mapping /path/to/mapping.xlsx \
  --source-prefix '_video/xlsx视频' \
  --upload-user-id 1 \
  --batch-key initial-import-001 \
  --dry-run

# 4) 确认输出后去掉 --dry-run 正式执行
... go run ./cmd/knowledgevideo-import \
  --mapping /path/to/mapping.xlsx \
  --source-prefix '_video/xlsx视频' \
  --upload-user-id 1 \
  --batch-key initial-import-001
```

> `--upload-user-id` 需要是数据库中已存在的用户 ID（批次归属人）。

## 命令与参数

### 单批次模式（默认）

| 参数 | 必填 | 作用 |
|---|---|---|
| `--mapping <path>` | 是 | 本地 XLSX 映射文件路径 |
| `--source-prefix <prefix>` | 是 | 允许的源 MinIO 对象前缀 |
| `--batch-key <key>` | 是 | 幂等 key；相同 key 已建批次时跳过 |
| `--upload-user-id <id>` | 是 | 批次归属用户 ID（正整数） |
| `--dry-run` | 否 | 只校验统计，不写入 |

### 调试模式

| 参数 | 必填 | 作用 |
|---|---|---|
| `--list-only` | 否（需 `--source-prefix`） | 列出源前缀对象与总字节数，不访问数据库 |
| `--inspect-object <key>` | 否（需 `--source-prefix`） | 下载并打印单个 XLSX 的表头、解析行数和校验问题 |

### 批量模式（`--all-prefix`）

当源桶按「一个目录 = 一个批次（`batch-*.xlsx` + `batch-*.zip`）」组织时，一次扫描整段前缀并逐批处理：

| 参数 | 必填 | 作用 |
|---|---|---|
| `--all-prefix <prefix>` | 是（该模式下） | 扫描并处理该前缀下的所有批次 |
| `--batch-key <key>` | 是 | 幂等 key，逐批以 `minio:<key>:<batch-name>` 去重 |
| `--upload-user-id <id>` | 是 | 批次归属用户 ID |
| `--report-dir <dir>` | 否 | 报告目录；默认系统临时目录下的 `knowledge-video-import-<batch-key>` |
| `--continue-on-error` | 否 | 批次失败后是否继续处理后续批次，默认 `true` |

批量模式示例：

```bash
SOURCE_MINIO_ENDPOINT=http://minio.example:9000 \
SOURCE_MINIO_BUCKET=source-bucket \
SOURCE_MINIO_ACCESS_KEY=<access-key> \
SOURCE_MINIO_SECRET_KEY=<secret-key> \
go run ./cmd/knowledgevideo-import \
  --all-prefix 'batch_imports/2026-08' \
  --upload-user-id 1 \
  --batch-key bulk-import-202608 \
  --report-dir /private/tmp/knowledge-video-import-report
```

批量规则：

- 前缀下同时存在 `batch-*.xlsx` 与 `batch-*.zip` 才算有效批次；只有其一记为 `invalid`。
- ZIP 内重复 basename、映射到的视频缺失会拒绝整个批次；未被映射引用的额外视频与元数据对象（`__MACOSX`、`.DS_Store`、`._*`）会被忽略。
- 表头错误、知识点不存在、名称不匹配、basename 歧义等校验失败会导致整批 `failed`。
- 行为与 HTTP 接口的差异：批量模式中 `video_name` 为空的整行会被跳过并计入 `skipped_empty_rows`（单批次模式则会直接报错）。

### 环境变量

| 环境变量 | 必填 | 作用 |
|---|---|---|
| `SOURCE_MINIO_ENDPOINT` | 是 | 源 MinIO S3 API 地址 |
| `SOURCE_MINIO_BUCKET` | 是 | 源 bucket |
| `SOURCE_MINIO_ACCESS_KEY` | 是 | 源 AccessKey |
| `SOURCE_MINIO_SECRET_KEY` | 是 | 源 SecretKey |
| `SOURCE_MINIO_USE_SSL` | 否 | 源 MinIO 是否走 HTTPS，默认 `false` |

## 映射文件格式与校验规则

读取第一个 worksheet，数据从第 2 行开始。表头接受以下写法（严格精确匹配）：

| 列 | 接受的表头 |
|---|---|
| 知识点 ID | `id` 或 `ID` |
| 知识点名称 | `name` 或 `完整路径` |
| 视频引用 | `video_name` 或 `视频文件名称` |

校验规则：

1. 表头必须是三列，不能有非空的第四列。
2. `id` 必须是正整数，且必须是数据库中已存在的知识点 ID。
3. `name` 必须与知识点字典名称精确匹配；`父级 / 叶子名称` 形式会自动按叶子名称规范化后再匹配。
4. `video_name` 可以是前缀下唯一的 basename，也可以是包含 `/` 的相对对象路径；basename 不唯一（歧义）或对象不存在都会报错。
5. 支持的扩展名：`.mp4`、`.mov`、`.mkv`、`.avi`、`.webm`、`.m4v`。
6. 空行（三列全空）会被忽略。

典型校验错误：

| 错误 | 含义 |
|---|---|
| `row=1 field=id: header must be exactly id` | 表头不精确（第 1 列） |
| `row=1 field=name: header must be exactly name` | 表头不精确（第 2 列） |
| `row=1 field=video_name: header must be exactly video_name` | 表头不精确（第 3 列） |
| `mapping must contain exactly three columns` | 存在非空第四列 |
| `id must be a positive integer` | id 不是正整数 |
| `name is required` | 名称缺失 |
| `video name is required` | 视频引用缺失 |
| `knowledge point does not exist` | 知识点 ID 不存在 |
| `knowledge point name does not match dictionary` | 名称与字典不一致 |
| `video is missing from object storage` | 源对象不存在 |
| `video basename is ambiguous` | basename 在源前缀下不唯一 |
| `unsupported video extension` | 扩展名不受支持 |

## 结果输出

dry-run / 校验通过后输出统计：

```text
validated rows=120 knowledge_points=8 unique_objects=120 reused_references=0 unique_bytes=5368709120 referenced_bytes=5368709120
dry_run=true writes=0
```

- `reused_references = rows - unique_objects`，即被多个知识点复用的源对象数。
- `unique_bytes` 是实际需要复制的字节数；`referenced_bytes` 是所有行引用的字节总和。

正式执行成功输出：

```text
skipped_existing=8
batch_id=123 created=120 enqueued=120
```

- `created` 创建的视频数；`enqueued` 成功投递转码任务数。入队失败会打到 stderr（`enqueue video_id=N: ...`）但不会中断导入。
- 若 `--batch-key` 已存在：`existing_batch_id=123`，直接跳过不重复入库。
- `skipped_existing` 是本批映射中已入库（按「知识点 ID + 源文件名」判定）而被跳过的视频数；若全部行都已存在，输出 `nothing to import: all rows already exist` 且不创建批次。

写入的目标对象：

```text
direct-import/<batch_id>/<sha256前8位hex>-<basename>   # 单批次模式
direct-import/<batch_id>/<basename>                    # 批量模式
manifests/<batch_id>/mapping.xlsx                      # 映射清单
```

## 批量报告

批量模式在 `--report-dir`（默认系统临时目录）下生成三个文件：

| 文件 | 内容 |
|---|---|
| `summary.json` | 全部批次结果：`batch`、`status`、`rows`、`unique_objects`、`skipped_empty_rows`、`skipped_existing_rows`、`error` |
| `valid-batches.csv` | `imported` / `skipped` / `dry-run` / `skipped-empty` / `skipped-existing` 批次 |
| `invalid-batches.csv` | `failed` / `invalid` 批次及错误详情 |

状态含义：

| status | 含义 |
|---|---|
| `imported` | 已入库并投递转码 |
| `skipped` | 该批次已存在（幂等去重） |
| `invalid` | 缺少 XLSX 或 ZIP |
| `skipped-empty` | 无有效数据行 |
| `skipped-existing` | 所有行引用的视频均已入库（视频级去重） |
| `failed` | 校验或执行失败，`error` 列含原因 |
| `dry-run` | 批量模式带 `--dry-run` 时的占位状态 |

排查建议：先看 `invalid-batches.csv` 的 `error` 列；最常见原因是表头不精确（`header must be exactly ...`）和整行缺少 `video_name`（`video name is required`）。修正映射文件后重新执行即可——幂等 key 会阻止已创建批次重复入库。

## 幂等与失败处理

- `--batch-key` 是幂等 key：单批次写入 `zip_file_name = minio:<batch-key>`；批量模式逐批写入 `minio:<batch-key>:<batch-name>`。已存在则跳过。
- **视频级去重**：无论批次 key 是否相同，只要某行引用的视频（知识点 ID + 源文件名）在 `edu_knowledge_video` 中已存在且未删除（`deleted = 0`），该行就会被跳过，只导入新素材；软删除过的同名视频会重新导入。批量模式下全部行都被跳过时状态记为 `skipped-existing`。
- 正式执行分多个阶段：复制对象 → 上传映射清单 → 创建批次记录 → 投递任务。中途失败会尽力删除本批次已上传的对象（`deleteUploaded`），避免留下孤儿对象。
- 转码任务投递后由 `cmd/worker` 消费；任务终态失败会进入 `knowledge_video:transcode:stream:dlq`，当前 `cmd/dlqctl` 不支持该队列，需要人工核对持久化失败状态、重试计数与 Redis payload。

## 验证导入结果

```bash
# 查询批次状态与视频列表
curl -H "Authorization: Bearer $ACCESS_TOKEN" \
  http://localhost:8081/api/admin/knowledge-videos/batches/123

# 查看知识点树（公开接口）
curl http://localhost:8081/api/knowledge-videos/tree
```

视频 `status=pending` 表示转码任务已入队；转码完成后可通过知识点视频接口获取播放地址。批量导入报告中的失败批次不会自动重试，修正映射后重新执行即可。

## 变更记录

### 2026-08-05：新增视频级去重（跳过已上传视频）

**背景**：此前工具只按 `--batch-key` 做批次级幂等；补充新素材时换用新的 batch key 会把映射中已入库过的视频重复导入，导致知识点下出现重复视频。

**改动**：

1. 新增 `filterExistingVideos`（main.go）：解析映射后，按「知识点 ID + 源文件名（basename）」批量查询 `edu_knowledge_video` 中未删除（`deleted = 0`）的记录，跳过已存在的行，只导入新素材。
2. 单批次模式：跳过行数输出 `skipped_existing=N`；全部行已存在时输出 `nothing to import: all rows already exist` 并直接返回，不创建批次、不复制对象。
3. 批量模式（bulk.go）：每个批次同样做视频级去重，`summary.json` / CSV 新增 `skipped_existing_rows` 列；某批全部行已存在时状态记为 `skipped-existing`（valid 类状态，不再被覆盖为 `imported`）。
4. 去重规则细节：
   - 只比较「知识点 ID + 源文件名」，同一视频挂到不同知识点互不影响。
   - 软删除（`deleted = 1`）过的同名视频会被重新导入。
   - 与 `--batch-key` 幂等并行生效：批次 key 相同走批次跳过，key 不同时靠视频级去重兜底。
5. 新增测试 `TestFilterRowsByExistingSkipsSamePointAndFile`、`TestFilterExistingVideosSkipsOnlyActiveDuplicates`（main_test.go），覆盖跨批次跳过与软删除重导场景。

**验证**：`go test ./cmd/knowledgevideo-import/`、`go vet ./cmd/knowledgevideo-import/` 通过。

**使用示例**：再次执行导入时直接用包含新旧全部素材的映射文件，换一个新的 `--batch-key` 即可——已上传的视频会被自动跳过，仅补充缺失素材。
