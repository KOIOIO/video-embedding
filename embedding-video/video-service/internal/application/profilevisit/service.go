package profilevisit

import (
	"context"
	"errors"
	"time"

	"video-service/internal/infrastructure/persistence"
)

// ErrInvalidOwnerID 表示被访问者 ID 无效。
var ErrInvalidOwnerID = errors.New("owner id must be positive")

// ErrInvalidDate 表示日期格式无效。
var ErrInvalidDate = errors.New("invalid date format, expected YYYY-MM-DD")

// VisitRepository 定义用户主页访问仓储接口。
type VisitRepository interface {
	UpsertVisit(ctx context.Context, visitorID, ownerID uint64, visitDate time.Time) error
	GetDailyStats(ctx context.Context, ownerID uint64, date time.Time) (int64, int64, error)
	GetRecentStats(ctx context.Context, ownerID uint64, days int) ([]persistence.DailyVisitStat, error)
}

// Service 用户主页访问应用服务。
type Service struct {
	repo VisitRepository
}

// NewService 创建用户主页访问服务。
func NewService(repo VisitRepository) *Service {
	return &Service{repo: repo}
}

// RecordVisit 记录一次主页访问。自己访问自己不记录。
func (s *Service) RecordVisit(ctx context.Context, visitorID, ownerID uint64) error {
	if ownerID == 0 {
		return ErrInvalidOwnerID
	}
	if visitorID == ownerID {
		return nil
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return s.repo.UpsertVisit(ctx, visitorID, ownerID, today)
}

// GetDailyStats 获取指定用户在指定日期的访问统计。
func (s *Service) GetDailyStats(ctx context.Context, ownerID uint64, dateStr string) (int64, int64, error) {
	if ownerID == 0 {
		return 0, 0, ErrInvalidOwnerID
	}
	var date time.Time
	if dateStr == "" {
		now := time.Now()
		date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	} else {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return 0, 0, ErrInvalidDate
		}
		date = parsed
	}
	return s.repo.GetDailyStats(ctx, ownerID, date)
}
