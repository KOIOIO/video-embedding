package sqlqueries

// ========== 题库相关 SQL ==========

// ListQuestionBankQuery 分页查询题库列表
const ListQuestionBankQuery = `
SELECT id, source, source_question_id, content, answer, analysis, knowledge, subject, type, status, create_time, update_time
FROM edu_question_bank
WHERE deleted = 0
ORDER BY id DESC
LIMIT ? OFFSET ?;
`

// CountQuestionBankQuery 统计题库总数
const CountQuestionBankQuery = `
SELECT COUNT(*) FROM edu_question_bank WHERE deleted = 0;
`

// GetQuestionByIDQuery 根据ID获取题目详情
const GetQuestionByIDQuery = `
SELECT id, source, source_question_id, content, answer, analysis, knowledge, subject, type, status, create_time, update_time
FROM edu_question_bank
WHERE id = ? AND deleted = 0
LIMIT 1;
`

// GetQuestionEmbeddingByIDQuery 根据ID获取题目向量
const GetQuestionEmbeddingByIDQuery = `
SELECT embedding::text AS embedding
FROM edu_question_bank
WHERE id = ? AND deleted = 0
LIMIT 1;
`

// ========== 推荐相关 SQL ==========

// RecommendByQuestionQuery 根据问题向量查询相似视频分段
const RecommendByQuestionQuery = `
SELECT
  s.id AS video_segment_id,
  s.video_id AS video_id,
  s.start_time AS start_time_sec,
  s.end_time AS end_time_sec,
  (s.embedding <=> ?) AS distance,
  s.content_summary AS segment_title,
  r.video_url AS video_url,
  r.cover_url AS cover_url,
  r.status AS status,
  r.is_published AS is_published,
  r.is_recommend AS is_recommend,
  r.view_count AS view_count,
  r.create_time AS create_time,
  r.update_time AS update_time
FROM edu_video_segment s
JOIN edu_video_resource r ON r.id = s.video_id
WHERE s.deleted = 0 AND s.status = 1 AND r.deleted = 0
ORDER BY s.embedding <=> ?
LIMIT ?;
`

// RecommendByWeakKnowledgeVectorQuery 根据用户薄弱知识点向量查询相似视频分段。
const RecommendByWeakKnowledgeVectorQuery = `
SELECT
  s.id AS video_segment_id,
  s.video_id AS video_id,
  s.start_time AS start_time_sec,
  s.end_time AS end_time_sec,
  (s.embedding <=> ?) AS distance,
  CASE
    WHEN TRIM(COALESCE(s.content_summary, '')) <> '' THEN s.content_summary
    ELSE r.title
  END AS segment_title,
  COALESCE(CAST(s.knowledge_tags AS TEXT), '') AS knowledge_tags,
  r.title AS video_title,
  r.description AS description,
  r.video_url AS video_url,
  r.cover_url AS cover_url,
  r.status AS status,
  r.is_published AS is_published,
  r.is_recommend AS is_recommend,
  r.view_count AS view_count,
  r.create_time AS create_time,
  r.update_time AS update_time
FROM edu_video_segment s
JOIN edu_video_resource r ON r.id = s.video_id
LEFT JOIN edu_user_reaction ur
  ON ur.user_id = ?
 AND ur.video_segment_id = s.id
 AND ur.reaction_type = 'dislike'
 AND ur.deleted = 0
LEFT JOIN edu_video_user_reaction vur
  ON vur.user_id = ?
 AND vur.video_id = s.video_id
 AND vur.reaction_type = 'dislike'
 AND vur.deleted = 0
LEFT JOIN edu_user_video_recommend watched
  ON watched.user_id = ?
 AND watched.video_segment_id = s.id
 AND watched.is_watched = true
 AND watched.deleted = 0
WHERE s.deleted = 0
  AND s.status = 1
  AND s.embedding IS NOT NULL
  AND r.deleted = 0
  AND r.is_published = true
  AND (? = false OR r.is_recommend = true)
  AND r.status = 3
  AND TRIM(COALESCE(r.video_url, '')) <> ''
  AND ur.id IS NULL
  AND vur.id IS NULL
  AND watched.id IS NULL
ORDER BY s.embedding <=> ?
LIMIT ?;
`

// RecommendByQuestionWithProfileQuery 根据问题向量召回候选，同时计算候选与用户画像的距离和用户历史信号。
const RecommendByQuestionWithProfileQuery = `
SELECT
  s.id AS video_segment_id,
  s.video_id AS video_id,
  s.start_time AS start_time_sec,
  s.end_time AS end_time_sec,
  (s.embedding <=> ?) AS distance,
  (s.embedding <=> ?) AS profile_distance,
  s.content_summary AS segment_title,
  s.like_count AS like_count,
  s.double_like_count AS double_like_count,
  s.dislike_count AS dislike_count,
  r.video_url AS video_url,
  r.cover_url AS cover_url,
  r.status AS status,
  r.is_published AS is_published,
  r.is_recommend AS is_recommend,
  r.view_count AS view_count,
  r.create_time AS create_time,
  r.update_time AS update_time,
  COALESCE(ur.reaction_type = 'dislike' AND ur.deleted = 0, false) AS user_disliked,
  COALESCE(vur.reaction_type = 'dislike' AND vur.deleted = 0, false) AS user_video_disliked,
  COALESCE(w.is_watched, false) AS user_watched
FROM edu_video_segment s
JOIN edu_video_resource r ON r.id = s.video_id
LEFT JOIN edu_user_reaction ur
  ON ur.user_id = ?
 AND ur.video_segment_id = s.id
LEFT JOIN edu_video_user_reaction vur
  ON vur.user_id = ?
 AND vur.video_id = s.video_id
LEFT JOIN edu_user_video_recommend w
  ON w.user_id = ?
 AND w.video_segment_id = s.id
 AND w.deleted = 0
WHERE s.deleted = 0
  AND s.status = 1
  AND s.embedding IS NOT NULL
  AND r.deleted = 0
ORDER BY s.embedding <=> ?
LIMIT ?;
`

const GetUserVideoProfileQuery = `
SELECT
  user_id AS user_id,
  profile_vector::text AS profile_vector,
  positive_count AS positive_count,
  model_version AS model_version,
  status AS status
FROM edu_user_video_profile
WHERE user_id = ?
  AND model_version = ?
  AND status = 1
  AND deleted = 0
LIMIT 1;
`

const GetUserRecBoleEmbeddingQuery = `
SELECT
  user_id AS user_id,
  embedding::text AS embedding,
  model_version AS model_version,
  status AS status
FROM recsys.recommend_user_embedding
WHERE user_id = ?
  AND model_name = ?
  AND model_version = ?
  AND status = 1
  AND deleted = 0
LIMIT 1;
`

const GetActiveRecommendModelVersionQuery = `
SELECT model_version
FROM recsys.recommend_model_version
WHERE model_name = ?
  AND is_active = TRUE
  AND status = 1
  AND deleted = 0
ORDER BY published_at DESC, id DESC
LIMIT 1;
`

const RecommendByRecBoleQuery = `
SELECT
  s.id AS video_segment_id,
  s.video_id AS video_id,
  s.start_time AS start_time_sec,
  s.end_time AS end_time_sec,
  (ie.embedding <=> ?) AS distance,
  s.content_summary AS segment_title,
  r.video_url AS video_url,
  r.cover_url AS cover_url,
  r.status AS status,
  r.is_published AS is_published,
  r.is_recommend AS is_recommend,
  r.view_count AS view_count,
  r.create_time AS create_time,
  r.update_time AS update_time
FROM recsys.recommend_item_embedding ie
JOIN edu_video_segment s ON s.id = ie.video_segment_id
JOIN edu_video_resource r ON r.id = ie.video_id
WHERE ie.model_name = ?
  AND ie.model_version = ?
  AND ie.status = 1
  AND ie.deleted = 0
  AND s.deleted = 0
  AND s.status = 1
  AND r.deleted = 0
ORDER BY ie.embedding <=> ?
LIMIT ?;
`

// UpsertUserVideoRecommendQuery 插入或更新用户视频推荐记录
const UpsertUserVideoRecommendQuery = `
INSERT INTO edu_user_video_recommend
  (user_id, video_id, question_id, video_segment_id, recommend_score, is_watched, watch_duration, deleted, create_time, update_time)
VALUES
  (?, ?, ?, ?, ?, FALSE, 0, 0, ?, ?)
ON CONFLICT (user_id, question_id, video_segment_id)
DO UPDATE SET
  recommend_score = EXCLUDED.recommend_score,
  video_id = EXCLUDED.video_id,
  deleted = 0,
  update_time = EXCLUDED.update_time;
`

// ListRecommendationsQuery 列出用户推荐列表
const ListRecommendationsQuery = `
SELECT
  ur.question_id AS question_id,
  ur.video_id AS video_id,
  ur.video_segment_id AS video_segment_id,
  COALESCE(ur.recommend_score, 0) AS recommend_score,
  COALESCE(ur.is_watched, FALSE) AS is_watched,
  COALESCE(ur.watch_duration, 0) AS watch_duration,
  COALESCE(s.start_time, 0) AS start_time_sec,
  COALESCE(s.end_time, 0) AS end_time_sec,
  r.title AS title,
  r.video_url AS video_url,
  r.cover_url AS cover_url,
  r.status AS status,
  r.is_published AS is_published,
  r.is_recommend AS is_recommend,
  r.view_count AS view_count,
  r.create_time AS create_time,
  r.update_time AS update_time
FROM edu_user_video_recommend ur
JOIN edu_video_resource r ON r.id = ur.video_id
LEFT JOIN edu_video_segment s ON s.id = ur.video_segment_id AND s.deleted = 0
WHERE ur.deleted = 0 AND r.deleted = 0 AND ur.user_id = ? AND ur.question_id = ?
ORDER BY ur.recommend_score DESC NULLS LAST, ur.id DESC
LIMIT ?;
`

// ========== 观看报告相关 SQL ==========

// GetVideoIDBySegmentIDQuery 根据分段ID获取视频ID
const GetVideoIDBySegmentIDQuery = `
SELECT video_id FROM edu_video_segment WHERE id = ? AND deleted = 0 LIMIT 1;
`

// HasWatchedVideoForQuestionQuery 判断同一用户在同一题目下是否已对该视频产生观看记录
const HasWatchedVideoForQuestionQuery = `
SELECT 1 AS "exists"
FROM edu_user_video_recommend
WHERE user_id = ? AND question_id = ? AND video_id = ? AND deleted = 0
LIMIT 1;
`

// UpsertWatchRecordQuery 插入或更新观看记录
const UpsertWatchRecordQuery = `
INSERT INTO edu_user_video_recommend
  (user_id, video_id, question_id, video_segment_id, is_watched, watch_duration, deleted, create_time, update_time)
VALUES
  (?, ?, ?, ?, ?, ?, 0, ?, ?)
ON CONFLICT (user_id, question_id, video_segment_id)
DO UPDATE SET
  is_watched = edu_user_video_recommend.is_watched OR EXCLUDED.is_watched,
  watch_duration = GREATEST(COALESCE(edu_user_video_recommend.watch_duration, 0), EXCLUDED.watch_duration),
  deleted = 0,
  update_time = EXCLUDED.update_time
RETURNING xmax = 0 AS inserted;
`

// ========== 分段相关 SQL ==========

// UpsertHierarchicalSegmentsQuery 批量插入分层分段（需要动态生成 VALUES）
const UpsertHierarchicalSegmentsQueryPrefix = `
INSERT INTO edu_video_segment (video_id, segment_index, start_time, end_time, content_summary, knowledge_tags, status, deleted, create_time)
VALUES `

// ========== 相似视频相关 SQL ==========

// FindSimilarVideosQuery 查找相似视频
const FindSimilarVideosQuery = `
SELECT video_id, MIN(embedding <=> ?) AS dist
FROM edu_video_segment
WHERE deleted = 0 AND status = 1 AND video_id <> ?
GROUP BY video_id
ORDER BY dist ASC
LIMIT ?
`

// ========== 数据完整性相关 SQL ==========

// AddRecommendColumnsQuery 添加推荐相关列
const AddRecommendColumnsQuery = `
ALTER TABLE edu_user_video_recommend ADD COLUMN IF NOT EXISTS question_id BIGINT;
ALTER TABLE edu_user_video_recommend ADD COLUMN IF NOT EXISTS video_segment_id BIGINT;
ALTER TABLE edu_user_video_recommend ADD COLUMN IF NOT EXISTS recommend_score NUMERIC(5,4);
ALTER TABLE edu_user_video_recommend ADD COLUMN IF NOT EXISTS is_watched BOOLEAN DEFAULT FALSE;
ALTER TABLE edu_user_video_recommend ADD COLUMN IF NOT EXISTS watch_duration INT;
`

const AddVideoResourceUserIDQuery = `
ALTER TABLE edu_video_resource ADD COLUMN IF NOT EXISTS user_id BIGINT;
UPDATE edu_video_resource SET user_id = 1 WHERE user_id IS NULL OR user_id = 0;
ALTER TABLE edu_video_resource ALTER COLUMN user_id SET DEFAULT 1;
ALTER TABLE edu_video_resource ALTER COLUMN user_id SET NOT NULL;
`

const CreateVideoResourceUserIndexQuery = `CREATE INDEX IF NOT EXISTS idx_video_resource_user ON edu_video_resource(user_id);`

// CreateUserQuestionIndexQuery 创建用户-问题索引
const CreateUserQuestionIndexQuery = `CREATE INDEX IF NOT EXISTS idx_user_video_recommend_user_question ON edu_user_video_recommend(user_id, question_id);`

// CreateSegmentIndexQuery 创建分段索引
const CreateSegmentIndexQuery = `CREATE INDEX IF NOT EXISTS idx_user_video_recommend_segment ON edu_user_video_recommend(video_segment_id);`

const CreateVideoReactionUserIndexQuery = `CREATE INDEX IF NOT EXISTS idx_video_user_reaction_user ON edu_video_user_reaction(user_id);`

const CreateVideoReactionVideoIndexQuery = `CREATE INDEX IF NOT EXISTS idx_video_user_reaction_video ON edu_video_user_reaction(video_id);`

const CreateVideoReactionUniqueConstraintQuery = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_video_user_reaction_user_video') THEN
    ALTER TABLE edu_video_user_reaction
      ADD CONSTRAINT uk_video_user_reaction_user_video UNIQUE (user_id, video_id);
  END IF;
END$$;
`

const CreateUserReactionUserIndexQuery = `CREATE INDEX IF NOT EXISTS idx_user_reaction_user ON edu_user_reaction(user_id);`

const CreateUserReactionVideoIndexQuery = `CREATE INDEX IF NOT EXISTS idx_user_reaction_video ON edu_user_reaction(video_id);`

const CreateUserReactionSegmentIndexQuery = `CREATE INDEX IF NOT EXISTS idx_user_reaction_segment ON edu_user_reaction(video_segment_id);`

const CreateUserReactionUniqueConstraintQuery = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_user_reaction_user_segment') THEN
    ALTER TABLE edu_user_reaction
      ADD CONSTRAINT uk_user_reaction_user_segment UNIQUE (user_id, video_segment_id);
  END IF;
END$$;
`

const CreateUserVideoProfileUniqueConstraintQuery = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_user_video_profile_user_model') THEN
    ALTER TABLE edu_user_video_profile
      ADD CONSTRAINT uk_user_video_profile_user_model UNIQUE (user_id, model_version);
  END IF;
END$$;
`

const CreateRecommendExposureLookupIndexQuery = `CREATE INDEX IF NOT EXISTS idx_recommend_exposure_user_question_segment_time ON edu_recommend_exposure(user_id, question_id, video_segment_id, create_time DESC);`

const CreateRecommendExposureRequestIndexQuery = `CREATE INDEX IF NOT EXISTS idx_recommend_exposure_request_rank ON edu_recommend_exposure(request_id, rank);`

// CreateUserQuestionSegmentUniqueConstraintQuery 创建唯一约束
const CreateUserQuestionSegmentUniqueConstraintQuery = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_user_question_segment') THEN
    ALTER TABLE edu_user_video_recommend
      ADD CONSTRAINT uk_user_question_segment UNIQUE (user_id, question_id, video_segment_id);
  END IF;
END$$;
`

// CleanOrphanSegmentsQuery 清理孤立的分段记录
const CleanOrphanSegmentsQuery = `
DELETE FROM edu_video_segment s
WHERE NOT EXISTS (SELECT 1 FROM edu_video_resource r WHERE r.id = s.video_id);
`

// CleanOrphanRecommendationsQuery 清理孤立的推荐记录
const CleanOrphanRecommendationsQuery = `
DELETE FROM edu_user_video_recommend ur
WHERE NOT EXISTS (SELECT 1 FROM edu_video_resource r WHERE r.id = ur.video_id);
`

// CreateSegmentVideoForeignKeyQuery 创建分段表外键约束
const CreateSegmentVideoForeignKeyQuery = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_edu_video_segment_video_id') THEN
    ALTER TABLE edu_video_segment
      ADD CONSTRAINT fk_edu_video_segment_video_id
      FOREIGN KEY (video_id) REFERENCES edu_video_resource(id) ON DELETE RESTRICT;
  END IF;
END$$;
`

// CreateRecommendVideoForeignKeyQuery 创建推荐表外键约束
const CreateRecommendVideoForeignKeyQuery = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_edu_user_video_recommend_video_id') THEN
    ALTER TABLE edu_user_video_recommend
      ADD CONSTRAINT fk_edu_user_video_recommend_video_id
      FOREIGN KEY (video_id) REFERENCES edu_video_resource(id) ON DELETE RESTRICT;
  END IF;
END$$;
`

// CreateSyncVideoDeletedFunctionQuery 创建同步删除触发器函数
const CreateSyncVideoDeletedFunctionQuery = `
CREATE OR REPLACE FUNCTION sync_video_deleted()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.deleted IS DISTINCT FROM OLD.deleted THEN
    UPDATE edu_video_segment SET deleted = NEW.deleted WHERE video_id = NEW.id;
    UPDATE edu_user_video_recommend SET deleted = NEW.deleted WHERE video_id = NEW.id;
  END IF;
  RETURN NEW;
END;
$$;
`

// CreateSyncVideoDeletedTriggerQuery 创建同步删除触发器
const CreateSyncVideoDeletedTriggerQuery = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_sync_video_deleted') THEN
    CREATE TRIGGER trg_sync_video_deleted
    AFTER UPDATE OF deleted ON edu_video_resource
    FOR EACH ROW
    EXECUTE FUNCTION sync_video_deleted();
  END IF;
END$$;
`
