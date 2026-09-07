package usersearch

import (
	"context"
	"errors"
	"strings"

	"video-service/internal/infrastructure/persistence"
)

// UserSearchDTO 用户搜索结果传输对象。
type UserSearchDTO struct {
	ID          uint64 `json:"id"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatar_url"`
	IsFollowing bool   `json:"is_following"`
}

// Repository 用户搜索仓储接口，服务层不直接依赖 GORM 实现。
type Repository interface {
	SearchUsers(ctx context.Context, currentUserID uint64, keyword string, page, pageSize int) ([]persistence.UserSearchResult, int64, error)
}

// Service 用户搜索应用服务。
type Service struct {
	repo Repository
}

// NewService 创建用户搜索服务。
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

const maxPageSize = 50

// Search 校验参数并调用仓储执行搜索。
func (s *Service) Search(ctx context.Context, currentUserID uint64, keyword string, page, pageSize int) ([]UserSearchDTO, int64, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, 0, errors.New("keyword is required")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	results, total, err := s.repo.SearchUsers(ctx, currentUserID, keyword, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]UserSearchDTO, len(results))
	for i, r := range results {
		dtos[i] = UserSearchDTO{
			ID:          r.ID,
			Username:    r.Username,
			Nickname:    r.Nickname,
			AvatarURL:   r.AvatarURL,
			IsFollowing: r.IsFollowing,
		}
	}
	return dtos, total, nil
}
