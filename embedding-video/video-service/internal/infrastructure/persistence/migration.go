package persistence

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"video-service/internal/model"
)

const schemaMigrationAdvisoryLockID int64 = 2026062301

// EnsureSchema serializes startup DDL across HTTP and worker processes.
func EnsureSchema(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return WithMigrationAdvisoryLock(db, func(tx *gorm.DB) error {
		if tx.Dialector.Name() != "postgres" {
			return ensureKnowledgeVideoSchema(tx)
		}
		if err := tx.Exec("CREATE EXTENSION IF NOT EXISTS vector;").Error; err != nil {
			return err
		}
		if err := EnsureRecSysSchema(tx); err != nil {
			return err
		}
		if err := tx.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoUserReaction{}, &model.EduUserReaction{}, &model.EduVideoSegment{}, &model.EduVideoVectorStage{}, &model.EduUserVideoRecommend{}, &model.EduUserVideoProfile{}, &model.EduRecommendExposure{}, &model.EduKnowledgeVideoBatch{}, &model.EduKnowledgeVideo{}, &model.EduKnowledgeVideoPlayRecord{}, &model.EduVideoComment{}, &model.EduCommentLike{}, &model.EduUserProfile{}); err != nil {
			return err
		}
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_video ON edu_video_segment(video_id);`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_embedding ON edu_video_segment USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_video_recommend_user ON edu_user_video_recommend(user_id);`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_video_recommend_video ON edu_user_video_recommend(video_id);`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_video_comment_segment_root ON edu_video_comment(video_segment_id, root_id, create_time);`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_video_comment_replies ON edu_video_comment(root_id, create_time) WHERE parent_id <> 0;`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_comment_like_comment_user ON edu_comment_like(comment_id, user_id);`).Error
		_ = tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uk_user_profile_user_active ON edu_user_profile(user_id) WHERE deleted = 0;`).Error
		if err := ensureKnowledgeVideoIndexes(tx); err != nil {
			return err
		}
		return EnsureIntegrity(tx)
	})
}

func ensureKnowledgeVideoSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.EduKnowledgeVideoBatch{}, &model.EduKnowledgeVideo{}, &model.EduKnowledgeVideoPlayRecord{}); err != nil {
		return err
	}
	return ensureKnowledgeVideoIndexes(db)
}

func ensureKnowledgeVideoIndexes(db *gorm.DB) error {
	if err := db.Exec(`DROP INDEX IF EXISTS uk_knowledge_video_knowledge_point_active;`).Error; err != nil {
		return err
	}
	for _, statement := range []string{
		`CREATE INDEX IF NOT EXISTS idx_knowledge_video_knowledge_point_active ON edu_knowledge_video(knowledge_point_id) WHERE deleted = 0;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uk_knowledge_video_batch_source_file_active ON edu_knowledge_video(batch_id, source_file_name) WHERE deleted = 0;`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_video_batch_status ON edu_knowledge_video(batch_id, status);`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_video_play_record_user_create_time ON edu_knowledge_video_play_record(user_id, create_time);`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_video_play_record_knowledge_point_create_time ON edu_knowledge_video_play_record(knowledge_point_id, create_time);`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_video_play_record_knowledge_video_create_time ON edu_knowledge_video_play_record(knowledge_video_id, create_time);`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func WithMigrationAdvisoryLock(db *gorm.DB, migrate func(*gorm.DB) error) error {
	if db == nil || migrate == nil {
		return nil
	}
	if db.Dialector.Name() != "postgres" {
		return db.Transaction(migrate)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		zap.L().Info("db_schema_migration_wait_lock", zap.Int64("lock_id", schemaMigrationAdvisoryLockID))
		return runWithMigrationAdvisoryLock(
			func() error {
				return tx.Exec("SELECT pg_advisory_lock(?)", schemaMigrationAdvisoryLockID).Error
			},
			func() error {
				return tx.Exec("SELECT pg_advisory_unlock(?)", schemaMigrationAdvisoryLockID).Error
			},
			func() error {
				err := migrate(tx)
				if err != nil {
					zap.L().Error("db_schema_migration_failed", zap.Error(err))
					return err
				}
				zap.L().Info("db_schema_migration_finished", zap.Int64("lock_id", schemaMigrationAdvisoryLockID))
				return nil
			},
		)
	})
}

func runWithMigrationAdvisoryLock(lock func() error, unlock func() error, migrate func() error) error {
	if lock == nil || unlock == nil || migrate == nil {
		return nil
	}
	if err := lock(); err != nil {
		return err
	}
	migrateErr := migrate()
	unlockErr := unlock()
	return errors.Join(migrateErr, unlockErr)
}
