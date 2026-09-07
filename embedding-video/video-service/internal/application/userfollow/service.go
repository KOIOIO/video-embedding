package userfollow

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"video-service/internal/infrastructure/persistence"
	"video-service/internal/model"
)

// ErrAlreadyFollowing 表示已经关注过该用户。
var ErrAlreadyFollowing = errors.New("already following")

// ErrCannotFollowSelf 表示不能关注自己。
var ErrCannotFollowSelf = errors.New("cannot follow yourself")

// FollowRepository 定义用户关注关系仓储接口。
type FollowRepository interface {
	CreateFollow(ctx context.Context, followerID, followingID uint64) error
	DeleteFollow(ctx context.Context, followerID, followingID uint64) error
	IsFollowing(ctx context.Context, followerID, followingID uint64) (bool, error)
	CountFollowers(ctx context.Context, userID uint64) (int64, error)
	CountFollowing(ctx context.Context, userID uint64) (int64, error)
	ListFollowing(ctx context.Context, userID uint64, page, pageSize int) ([]model.FollowWithProfile, error)
	ListFollowers(ctx context.Context, userID uint64, page, pageSize int) ([]model.FollowWithProfile, error)
}

// ProfileCounter 定义用户资料计数更新接口，用于事务中同步关注/粉丝数。
type ProfileCounter interface {
	IncrementFollowCount(userID uint64, delta int) error
	IncrementFansCount(userID uint64, delta int) error
}

// Service 用户关注应用服务。
type Service struct {
	db          *gorm.DB
	followRepo  FollowRepository
	profileRepo ProfileCounter
}

// NewService 创建用户关注服务。
func NewService(db *gorm.DB, followRepo FollowRepository, profileRepo ProfileCounter) *Service {
	return &Service{
		db:          db,
		followRepo:  followRepo,
		profileRepo: profileRepo,
	}
}

// Follow 关注用户，事务保证关系表与计数一致。
func (s *Service) Follow(ctx context.Context, followerID, followingID uint64) error {
	if followerID == followingID {
		return ErrCannotFollowSelf
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		followRepo := s.txFollowRepo(tx)
		profileRepo := s.txProfileRepo(tx)

		if err := followRepo.CreateFollow(ctx, followerID, followingID); err != nil {
			if isUniqueViolation(err) {
				return ErrAlreadyFollowing
			}
			return err
		}
		if err := profileRepo.IncrementFollowCount(followerID, 1); err != nil {
			return err
		}
		return profileRepo.IncrementFansCount(followingID, 1)
	})
}

// Unfollow 取消关注，事务保证关系表与计数一致。
func (s *Service) Unfollow(ctx context.Context, followerID, followingID uint64) error {
	if followerID == followingID {
		return ErrCannotFollowSelf
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		followRepo := s.txFollowRepo(tx)
		profileRepo := s.txProfileRepo(tx)

		if err := followRepo.DeleteFollow(ctx, followerID, followingID); err != nil {
			return err
		}
		if err := profileRepo.IncrementFollowCount(followerID, -1); err != nil {
			return err
		}
		return profileRepo.IncrementFansCount(followingID, -1)
	})
}

// GetRelation 查询当前用户与目标用户的关注关系，返回 "none" / "following" / "mutual"。
func (s *Service) GetRelation(ctx context.Context, userID, targetID uint64) (string, error) {
	if userID == 0 || targetID == 0 || userID == targetID {
		return "none", nil
	}
	following, err := s.followRepo.IsFollowing(ctx, userID, targetID)
	if err != nil {
		return "", err
	}
	if !following {
		return "none", nil
	}
	followed, err := s.followRepo.IsFollowing(ctx, targetID, userID)
	if err != nil {
		return "", err
	}
	if followed {
		return "mutual", nil
	}
	return "following", nil
}

// ListFollowing 查询指定用户的关注列表。
func (s *Service) ListFollowing(ctx context.Context, userID uint64, page, pageSize int) ([]model.FollowWithProfile, error) {
	return s.followRepo.ListFollowing(ctx, userID, page, pageSize)
}

// CountFollowing 查询指定用户的关注总数。
func (s *Service) CountFollowing(ctx context.Context, userID uint64) (int64, error) {
	return s.followRepo.CountFollowing(ctx, userID)
}

// ListFollowers 查询指定用户的粉丝列表。
func (s *Service) ListFollowers(ctx context.Context, userID uint64, page, pageSize int) ([]model.FollowWithProfile, error) {
	return s.followRepo.ListFollowers(ctx, userID, page, pageSize)
}

// CountFollowers 查询指定用户的粉丝总数。
func (s *Service) CountFollowers(ctx context.Context, userID uint64) (int64, error) {
	return s.followRepo.CountFollowers(ctx, userID)
}

// txFollowRepo 返回绑定到事务的关注仓储；若实现不支持事务绑定则返回原实例。
func (s *Service) txFollowRepo(tx *gorm.DB) FollowRepository {
	if txer, ok := s.followRepo.(interface{ WithTx(*gorm.DB) *persistence.GormUserFollowRepository }); ok {
		return txer.WithTx(tx)
	}
	return s.followRepo
}

// txProfileRepo 返回绑定到事务的资料计数仓储；若实现不支持事务绑定则返回原实例。
func (s *Service) txProfileRepo(tx *gorm.DB) ProfileCounter {
	if txer, ok := s.profileRepo.(interface{ WithTx(*gorm.DB) *persistence.GormUserProfileRepository }); ok {
		return txer.WithTx(tx)
	}
	return s.profileRepo
}

// isUniqueViolation 判断错误是否为唯一索引冲突（PostgreSQL / SQLite 通用关键字匹配）。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique index") ||
		strings.Contains(msg, "23505")
}
