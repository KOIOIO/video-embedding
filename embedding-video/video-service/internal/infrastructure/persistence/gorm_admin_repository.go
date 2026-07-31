package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"video-service/internal/application/adminauth"
)

type GormAdminRepository struct {
	db *gorm.DB
}

func NewGormAdminRepository(db *gorm.DB) *GormAdminRepository {
	return &GormAdminRepository{db: db}
}

func (r *GormAdminRepository) FindActiveAdminByUsername(ctx context.Context, username string) (adminauth.Admin, bool, error) {
	return r.find(ctx, "username = ?", username)
}

func (r *GormAdminRepository) FindActiveAdminByID(ctx context.Context, id uint64) (adminauth.Admin, bool, error) {
	return r.find(ctx, "id = ?", id)
}

func (r *GormAdminRepository) find(ctx context.Context, predicate string, value any) (adminauth.Admin, bool, error) {
	var admin adminauth.Admin
	err := r.db.WithContext(ctx).Table("sys_user").
		Select("id, username, password AS password_hash, real_name, user_type, status, deleted").
		Where(predicate, value).
		Where("user_type = ? AND status = ? AND deleted = ?", 3, 1, 0).
		Take(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return adminauth.Admin{}, false, nil
	}
	if err != nil {
		return adminauth.Admin{}, false, err
	}
	return admin, true, nil
}
