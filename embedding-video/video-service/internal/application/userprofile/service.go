package userprofile

import (
	"errors"
	"strings"
	"unicode/utf8"

	"video-service/internal/model"
)

// Repository 定义用户资料仓储接口，服务层不直接依赖 GORM 实现。
type Repository interface {
	GetByUserID(userID uint64) (*model.EduUserProfile, error)
	UpsertDefault(userID uint64) (*model.EduUserProfile, error)
	Update(userID uint64, updates map[string]interface{}) error
	UpdateAvatar(userID uint64, avatarURL string) error
}

// Service 用户资料应用服务。
type Service struct {
	repo Repository
}

// NewService 创建用户资料服务。
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetProfile 获取用户资料，不存在时懒初始化默认记录。
func (s *Service) GetProfile(userID uint64) (*model.EduUserProfile, error) {
	profile, err := s.repo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}
	if profile != nil {
		return profile, nil
	}
	return s.repo.UpsertDefault(userID)
}

// UpdateProfile 更新用户资料，校验昵称非空且长度合法。
func (s *Service) UpdateProfile(userID uint64, nickname, bio, location string, gender int16) error {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return errors.New("nickname is required")
	}
	if utf8.RuneCountInString(nickname) > 100 {
		return errors.New("nickname must be at most 100 characters")
	}
	if utf8.RuneCountInString(bio) > 500 {
		return errors.New("bio must be at most 500 characters")
	}
	if gender < 0 || gender > 2 {
		return errors.New("gender must be 0 (unknown), 1 (male), or 2 (female)")
	}
	// 确保资料记录存在
	if _, err := s.GetProfile(userID); err != nil {
		return err
	}
	return s.repo.Update(userID, map[string]interface{}{
		"nickname": nickname,
		"bio":      bio,
		"location": location,
		"gender":   gender,
	})
}

// UpdateAvatar 更新用户头像地址。
func (s *Service) UpdateAvatar(userID uint64, avatarURL string) error {
	if strings.TrimSpace(avatarURL) == "" {
		return errors.New("avatar URL is required")
	}
	// 确保资料记录存在
	if _, err := s.GetProfile(userID); err != nil {
		return err
	}
	return s.repo.UpdateAvatar(userID, avatarURL)
}
