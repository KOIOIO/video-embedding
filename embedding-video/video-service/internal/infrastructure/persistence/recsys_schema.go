package persistence

import "gorm.io/gorm"

var recsysSchemaStatements = []string{
	`CREATE SCHEMA IF NOT EXISTS recsys`,
	`CREATE TABLE IF NOT EXISTS recsys.recommend_model_version (
  id BIGSERIAL PRIMARY KEY,
  model_name TEXT NOT NULL,
  model_version TEXT NOT NULL,
  framework TEXT NOT NULL DEFAULT 'recbole',
  algorithm TEXT NOT NULL DEFAULT '',
  artifact_path TEXT,
  metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  is_active BOOLEAN DEFAULT FALSE,
  status SMALLINT DEFAULT 1,
  published_at TIMESTAMP,
  create_time TIMESTAMP,
  update_time TIMESTAMP,
  deleted SMALLINT DEFAULT 0
)`,
	`CREATE TABLE IF NOT EXISTS recsys.recommend_user_embedding (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  embedding vector(64) NOT NULL,
  model_name TEXT NOT NULL,
  model_version TEXT NOT NULL,
  status SMALLINT DEFAULT 1,
  deleted SMALLINT DEFAULT 0,
  create_time TIMESTAMP,
  update_time TIMESTAMP
)`,
	`CREATE TABLE IF NOT EXISTS recsys.recommend_item_embedding (
  id BIGSERIAL PRIMARY KEY,
  video_segment_id BIGINT NOT NULL,
  video_id BIGINT NOT NULL,
  embedding vector(64) NOT NULL,
  model_name TEXT NOT NULL,
  model_version TEXT NOT NULL,
  status SMALLINT DEFAULT 1,
  deleted SMALLINT DEFAULT 0,
  create_time TIMESTAMP,
  update_time TIMESTAMP
)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_recsys_recommend_model_version_name_version ON recsys.recommend_model_version(model_name, model_version)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_recsys_recommend_user_embedding_user_model ON recsys.recommend_user_embedding(user_id, model_name, model_version)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_recsys_recommend_item_embedding_segment_model ON recsys.recommend_item_embedding(video_segment_id, model_name, model_version)`,
	`CREATE INDEX IF NOT EXISTS idx_recsys_recommend_model_version_active ON recsys.recommend_model_version(model_name, is_active, status, deleted, published_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_recsys_recommend_user_embedding_lookup ON recsys.recommend_user_embedding(user_id, model_name, model_version, status, deleted)`,
	`CREATE INDEX IF NOT EXISTS idx_recsys_recommend_item_embedding_lookup ON recsys.recommend_item_embedding(model_name, model_version, status, deleted)`,
}

func RecSysSchemaStatements() []string {
	return append([]string(nil), recsysSchemaStatements...)
}

func EnsureRecSysSchema(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	for _, statement := range recsysSchemaStatements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
