package persistence

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"nlp-video-analysis/internal/model"
)

const schemaMigrationAdvisoryLockID int64 = 2026062301

// EnsureSchema serializes startup DDL across HTTP and worker processes.
func EnsureSchema(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return WithMigrationAdvisoryLock(db, func(tx *gorm.DB) error {
		if err := tx.Exec("CREATE EXTENSION IF NOT EXISTS vector;").Error; err != nil {
			return err
		}
		if err := EnsureRecSysSchema(tx); err != nil {
			return err
		}
		if err := tx.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoUserReaction{}, &model.EduUserReaction{}, &model.EduVideoSegment{}, &model.EduVideoVectorStage{}, &model.EduUserVideoRecommend{}, &model.EduUserVideoProfile{}, &model.EduRecommendExposure{}); err != nil {
			return err
		}
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_video ON edu_video_segment(video_id);`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_embedding ON edu_video_segment USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_video_recommend_user ON edu_user_video_recommend(user_id);`).Error
		_ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_video_recommend_video ON edu_user_video_recommend(video_id);`).Error
		_ = tx.Exec(`ALTER TABLE edu_user_video_recommend DROP CONSTRAINT IF EXISTS uk_user_question_segment;`).Error
		_ = tx.Exec(`ALTER TABLE edu_user_video_recommend DROP CONSTRAINT IF EXISTS uk_user_video_segment;`).Error
		_ = tx.Exec(`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uk_user_question_segment') THEN ALTER TABLE edu_user_video_recommend ADD CONSTRAINT uk_user_question_segment UNIQUE (user_id, question_id, video_segment_id); END IF; END $$;`).Error
		return EnsureIntegrity(tx)
	})
}

func WithMigrationAdvisoryLock(db *gorm.DB, migrate func(*gorm.DB) error) error {
	if db == nil || migrate == nil {
		return nil
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
