package persistence

import (
	"context"
	"errors"
	"strings"

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
	return r.find(ctx, "username = ? AND user_type = ?", username, adminauth.UserTypeAdmin)
}

func (r *GormAdminRepository) FindActiveAdminByID(ctx context.Context, id uint64) (adminauth.Admin, bool, error) {
	return r.find(ctx, "id = ? AND user_type = ?", id, adminauth.UserTypeAdmin)
}

func (r *GormAdminRepository) FindActiveUserByUsername(ctx context.Context, username string) (adminauth.Admin, bool, error) {
	return r.find(ctx, "username = ?", username)
}

func (r *GormAdminRepository) FindActiveUserByID(ctx context.Context, id uint64) (adminauth.Admin, bool, error) {
	return r.find(ctx, "id = ?", id)
}

func (r *GormAdminRepository) CreateUser(ctx context.Context, admin adminauth.Admin) (uint64, error) {
	row := sysUserRow{
		Username:  admin.Username,
		Password:  admin.PasswordHash,
		RealName:  admin.RealName,
		UserType:  admin.UserType,
		Status:    admin.Status,
		Deleted:   admin.Deleted,
	}
	result := r.db.WithContext(ctx).Table("sys_user").Create(&row)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) || isUniqueViolation(result.Error) {
			return 0, adminauth.ErrUsernameExists
		}
		return 0, result.Error
	}
	return row.ID, nil
}

type sysUserRow struct {
	ID       uint64 `gorm:"column:id;primaryKey"`
	Username string `gorm:"column:username"`
	Password string `gorm:"column:password"`
	RealName string `gorm:"column:real_name"`
	UserType int16  `gorm:"column:user_type"`
	Status   int16  `gorm:"column:status"`
	Deleted  int16  `gorm:"column:deleted"`
}

func (sysUserRow) TableName() string { return "sys_user" }

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint") || strings.Contains(msg, "unique index")
}

func (r *GormAdminRepository) find(ctx context.Context, predicate string, values ...any) (adminauth.Admin, bool, error) {
	var admin adminauth.Admin
	err := r.db.WithContext(ctx).Table("sys_user").
		Select("id, username, password AS password_hash, real_name, user_type, status, deleted").
		Where(predicate, values...).
		Where("status = ? AND deleted = ?", 1, 0).
		Take(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return adminauth.Admin{}, false, nil
	}
	if err != nil {
		return adminauth.Admin{}, false, err
	}
	return admin, true, nil
}
