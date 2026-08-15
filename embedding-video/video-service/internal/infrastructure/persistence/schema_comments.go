package persistence

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type columnCommentDefinition struct {
	Name    string
	Comment string
}

type tableCommentDefinition struct {
	Schema  string
	Table   string
	Comment string
	Columns []columnCommentDefinition
}

func columnComment(name, comment string) columnCommentDefinition {
	return columnCommentDefinition{Name: name, Comment: comment}
}

var schemaCommentDefinitions = []tableCommentDefinition{
	{
		Schema:  "public",
		Table:   "edu_video_resource",
		Comment: "视频资源表",
		Columns: []columnCommentDefinition{
			columnComment("id", "视频资源主键 ID"),
			columnComment("user_id", "上传用户 ID"),
			columnComment("title", "视频标题"),
			columnComment("description", "视频描述"),
			columnComment("video_url", "原始视频访问地址"),
			columnComment("duration", "视频时长（秒）"),
			columnComment("cover_url", "视频封面访问地址"),
			columnComment("status", "视频处理状态：1 已上传，2 处理中，3 已完成，4 失败"),
			columnComment("error_msg", "视频处理失败信息"),
			columnComment("is_published", "是否已发布"),
			columnComment("is_recommend", "是否加入推荐池"),
			columnComment("view_count", "浏览次数"),
			columnComment("like_count", "点赞次数"),
			columnComment("double_like_count", "双击点赞次数"),
			columnComment("dislike_count", "点踩次数"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_video_user_reaction",
		Comment: "用户视频互动记录表",
		Columns: []columnCommentDefinition{
			columnComment("id", "互动记录主键 ID"),
			columnComment("user_id", "用户 ID"),
			columnComment("video_id", "视频资源 ID"),
			columnComment("reaction_type", "互动类型：like 点赞、double_like 双击点赞、dislike 点踩"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_user_reaction",
		Comment: "用户视频片段互动记录表",
		Columns: []columnCommentDefinition{
			columnComment("id", "互动记录主键 ID"),
			columnComment("user_id", "用户 ID"),
			columnComment("video_id", "视频资源 ID"),
			columnComment("video_segment_id", "视频片段 ID"),
			columnComment("reaction_type", "互动类型：like 点赞、double_like 双击点赞、dislike 点踩"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_video_segment",
		Comment: "视频内容片段表",
		Columns: []columnCommentDefinition{
			columnComment("id", "视频片段主键 ID"),
			columnComment("video_id", "所属视频资源 ID"),
			columnComment("segment_index", "片段序号"),
			columnComment("start_time", "片段开始时间（秒）"),
			columnComment("end_time", "片段结束时间（秒）"),
			columnComment("content_summary", "片段内容摘要"),
			columnComment("embedding", "片段内容向量（1536 维）"),
			columnComment("knowledge_tags", "片段知识点标签数组"),
			columnComment("like_count", "点赞次数"),
			columnComment("double_like_count", "双击点赞次数"),
			columnComment("dislike_count", "点踩次数"),
			columnComment("status", "片段状态：1 可用"),
			columnComment("create_time", "创建时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_video_vector_stage",
		Comment: "视频向量化阶段记录表",
		Columns: []columnCommentDefinition{
			columnComment("id", "阶段记录主键 ID"),
			columnComment("task_id", "向量化任务 ID"),
			columnComment("video_id", "视频资源 ID"),
			columnComment("stage", "处理阶段标识"),
			columnComment("segment_index", "阶段内片段序号"),
			columnComment("segment_id", "关联视频片段 ID"),
			columnComment("status", "阶段状态：0 待处理，1 处理中，2 完成，3 失败，4 跳过"),
			columnComment("object_key", "阶段产物对象存储键"),
			columnComment("text", "阶段输出文本"),
			columnComment("error_message", "阶段失败信息"),
			columnComment("retry_count", "重试次数"),
			columnComment("start_time", "片段开始时间（秒）"),
			columnComment("end_time", "片段结束时间（秒）"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_user_video_recommend",
		Comment: "用户视频推荐与观看记录表",
		Columns: []columnCommentDefinition{
			columnComment("id", "推荐记录主键 ID"),
			columnComment("user_id", "用户 ID"),
			columnComment("video_id", "视频资源 ID"),
			columnComment("recommend_level", "推荐等级"),
			columnComment("question_id", "关联题目 ID"),
			columnComment("video_segment_id", "关联视频片段 ID"),
			columnComment("recommend_score", "推荐得分"),
			columnComment("is_watched", "是否已观看"),
			columnComment("watch_duration", "观看时长（秒）"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_user_video_profile",
		Comment: "用户视频兴趣画像表",
		Columns: []columnCommentDefinition{
			columnComment("id", "画像记录主键 ID"),
			columnComment("user_id", "用户 ID"),
			columnComment("profile_vector", "用户偏好画像向量（1536 维）"),
			columnComment("positive_count", "正向行为次数"),
			columnComment("negative_count", "负向行为次数"),
			columnComment("watch_count", "观看行为次数"),
			columnComment("source_event_count", "画像来源事件总数"),
			columnComment("last_event_time", "最近来源事件时间"),
			columnComment("model_version", "画像模型版本"),
			columnComment("status", "画像状态：1 有效"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_recommend_exposure",
		Comment: "推荐曝光明细表",
		Columns: []columnCommentDefinition{
			columnComment("id", "曝光记录主键 ID"),
			columnComment("request_id", "推荐请求 ID"),
			columnComment("user_id", "用户 ID"),
			columnComment("question_id", "关联题目 ID"),
			columnComment("video_id", "视频资源 ID"),
			columnComment("video_segment_id", "视频片段 ID"),
			columnComment("rank", "推荐排序位置"),
			columnComment("score", "推荐得分"),
			columnComment("strategy", "推荐策略"),
			columnComment("model_version", "推荐模型版本"),
			columnComment("clicked", "是否点击"),
			columnComment("watched", "是否观看"),
			columnComment("clicked_time", "点击时间"),
			columnComment("watched_time", "观看时间"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_knowledge_video_batch",
		Comment: "知识点视频导入批次表",
		Columns: []columnCommentDefinition{
			columnComment("id", "导入批次主键 ID"),
			columnComment("upload_user_id", "上传用户 ID"),
			columnComment("zip_file_name", "上传压缩包文件名"),
			columnComment("xlsx_file_name", "映射表文件名"),
			columnComment("xlsx_object_key", "映射表对象存储键"),
			columnComment("total_count", "批次视频总数"),
			columnComment("ready_count", "处理成功视频数"),
			columnComment("failed_count", "处理失败视频数"),
			columnComment("status", "批次状态：1 处理中，2 已完成，3 部分失败，4 全部失败"),
			columnComment("error_message", "批次错误信息"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_knowledge_video",
		Comment: "知识点视频表",
		Columns: []columnCommentDefinition{
			columnComment("id", "知识点视频主键 ID"),
			columnComment("batch_id", "导入批次 ID"),
			columnComment("knowledge_point_id", "知识点 ID"),
			columnComment("knowledge_point_name", "导入时知识点名称"),
			columnComment("source_file_name", "源视频文件名"),
			columnComment("source_object_key", "源视频对象存储键"),
			columnComment("hls_object_prefix", "HLS 产物对象键前缀"),
			columnComment("hls_master_object_key", "HLS 主播放列表对象存储键"),
			columnComment("duration", "视频时长（秒）"),
			columnComment("status", "转码状态：0 待处理，1 转码中，2 就绪，3 失败"),
			columnComment("error_message", "转码失败信息"),
			columnComment("enqueue_time", "最近入队时间"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "public",
		Table:   "edu_knowledge_video_play_record",
		Comment: "知识点视频观看记录表",
		Columns: []columnCommentDefinition{
			columnComment("id", "观看记录主键 ID"),
			columnComment("user_id", "用户 ID"),
			columnComment("knowledge_point_id", "知识点 ID"),
			columnComment("knowledge_video_id", "知识点视频 ID"),
			columnComment("session_id", "观看会话 ID"),
			columnComment("watch_duration", "该会话累计观看时长（秒）"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
		},
	},
	{
		Schema:  "recsys",
		Table:   "recommend_model_version",
		Comment: "推荐模型版本表",
		Columns: []columnCommentDefinition{
			columnComment("id", "模型版本记录主键 ID"),
			columnComment("model_name", "模型名称"),
			columnComment("model_version", "模型版本号"),
			columnComment("framework", "训练框架"),
			columnComment("algorithm", "推荐算法名称"),
			columnComment("artifact_path", "模型产物路径"),
			columnComment("metrics_json", "模型评估指标 JSON"),
			columnComment("is_active", "是否为当前生效版本"),
			columnComment("status", "记录状态：1 有效"),
			columnComment("published_at", "模型发布时间"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
		},
	},
	{
		Schema:  "recsys",
		Table:   "recommend_user_embedding",
		Comment: "推荐用户向量表",
		Columns: []columnCommentDefinition{
			columnComment("id", "用户向量记录主键 ID"),
			columnComment("user_id", "用户 ID"),
			columnComment("embedding", "用户推荐向量（64 维）"),
			columnComment("model_name", "模型名称"),
			columnComment("model_version", "模型版本号"),
			columnComment("status", "向量状态：1 有效"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
		},
	},
	{
		Schema:  "recsys",
		Table:   "recommend_item_embedding",
		Comment: "推荐视频片段向量表",
		Columns: []columnCommentDefinition{
			columnComment("id", "片段向量记录主键 ID"),
			columnComment("video_segment_id", "视频片段 ID"),
			columnComment("video_id", "视频资源 ID"),
			columnComment("embedding", "片段推荐向量（64 维）"),
			columnComment("model_name", "模型名称"),
			columnComment("model_version", "模型版本号"),
			columnComment("status", "向量状态：1 有效"),
			columnComment("deleted", "逻辑删除标记：0 未删除，非 0 已删除"),
			columnComment("create_time", "创建时间"),
			columnComment("update_time", "更新时间"),
		},
	},
}

// SchemaCommentStatements returns the deterministic PostgreSQL COMMENT statements for service-owned tables.
func SchemaCommentStatements() []string {
	count := 0
	for _, definition := range schemaCommentDefinitions {
		count += 1 + len(definition.Columns)
	}
	statements := make([]string, 0, count)
	for _, definition := range schemaCommentDefinitions {
		tableName := quotePostgresIdentifier(definition.Schema) + "." + quotePostgresIdentifier(definition.Table)
		statements = append(statements, fmt.Sprintf("COMMENT ON TABLE %s IS %s", tableName, quotePostgresLiteral(definition.Comment)))
		for _, column := range definition.Columns {
			columnName := tableName + "." + quotePostgresIdentifier(column.Name)
			statements = append(statements, fmt.Sprintf("COMMENT ON COLUMN %s IS %s", columnName, quotePostgresLiteral(column.Comment)))
		}
	}
	return statements
}

// EnsureSchemaComments applies the service-owned table and column comments to PostgreSQL.
func EnsureSchemaComments(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, statement := range SchemaCommentStatements() {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func quotePostgresIdentifier(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\"\"") + "\""
}

func quotePostgresLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
