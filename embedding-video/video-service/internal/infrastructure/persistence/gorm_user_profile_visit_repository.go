package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"video-service/internal/model"
)

// DailyVisitStat 每日访问统计结果。
type DailyVisitStat struct {
	Date           time.Time `gorm:"column:visit_date" json:"date"`
	UniqueVisitors int64     `gorm:"column:unique_visitors" json:"unique_visitors"`
	TotalVisits    int64     `gorm:"column:total_visits" json:"total_visits"`
}

// GormUserProfileVisitRepository 用户主页访问仓储的 GORM 实现。
type GormUserProfileVisitRepository struct {
	db *gorm.DB
}

// NewGormUserProfileVisitRepository 创建用户主页访问仓储。
func NewGormUserProfileVisitRepository(db *gorm.DB) *GormUserProfileVisitRepository {
	return &GormUserProfileVisitRepository{db: db}
}

// UpsertVisit 记录一次主页访问，同一天同一访客重复访问时 visit_count 递增。
func (r *GormUserProfileVisitRepository) UpsertVisit(ctx context.Context, visitorID, ownerID uint64, visitDate time.Time) error {
	dateOnly := time.Date(visitDate.Year(), visitDate.Month(), visitDate.Day(), 0, 0, 0, 0, visitDate.Location())
	if r.db.Dialector.Name() == "postgres" {
		return r.db.WithContext(ctx).Exec(`
			INSERT INTO edu_user_profile_visit (visitor_id, owner_id, visit_date, visit_count, deleted)
			VALUES (?, ?, ?, 1, 0)
			ON CONFLICT (visitor_id, owner_id, visit_date) WHERE deleted = 0
			DO UPDATE SET visit_count = edu_user_profile_visit.visit_count + 1, update_time = NOW()
		`, visitorID, ownerID, dateOnly).Error
	}
	// SQLite 3.24+ 支持 ON CONFLICT；SQLite 中 DO UPDATE 可直接引用列名
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO edu_user_profile_visit (visitor_id, owner_id, visit_date, visit_count, deleted)
		VALUES (?, ?, ?, 1, 0)
		ON CONFLICT (visitor_id, owner_id, visit_date) WHERE deleted = 0
		DO UPDATE SET visit_count = visit_count + 1
	`, visitorID, ownerID, dateOnly).Error
}

// GetDailyStats 查询指定用户在指定日期的独立访客数和总访问次数。
func (r *GormUserProfileVisitRepository) GetDailyStats(ctx context.Context, ownerID uint64, date time.Time) (int64, int64, error) {
	dateOnly := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	var result struct {
		UniqueVisitors int64 `gorm:"column:unique_visitors"`
		TotalVisits    int64 `gorm:"column:total_visits"`
	}
	err := r.db.WithContext(ctx).
		Model(&model.EduUserProfileVisit{}).
		Select("COUNT(DISTINCT visitor_id) AS unique_visitors, COALESCE(SUM(visit_count), 0) AS total_visits").
		Where("owner_id = ? AND visit_date = ? AND deleted = ?", ownerID, dateOnly, 0).
		Scan(&result).Error
	if err != nil {
		return 0, 0, err
	}
	return result.UniqueVisitors, result.TotalVisits, nil
}

// GetRecentStats 查询指定用户最近 N 天的每日访问统计。
func (r *GormUserProfileVisitRepository) GetRecentStats(ctx context.Context, ownerID uint64, days int) ([]DailyVisitStat, error) {
	if days < 1 {
		days = 7
	}
	var results []DailyVisitStat
	err := r.db.WithContext(ctx).
		Model(&model.EduUserProfileVisit{}).
		Select("visit_date, COUNT(DISTINCT visitor_id) AS unique_visitors, COALESCE(SUM(visit_count), 0) AS total_visits").
		Where("owner_id = ? AND deleted = ?", ownerID, 0).
		Group("visit_date").
		Order("visit_date DESC").
		Limit(days).
		Scan(&results).Error
	return results, err
}
