package adminauth

import "context"

type Repository interface {
	FindActiveAdminByUsername(ctx context.Context, username string) (Admin, bool, error)
	FindActiveAdminByID(ctx context.Context, id uint64) (Admin, bool, error)
	FindActiveUserByUsername(ctx context.Context, username string) (Admin, bool, error)
	FindActiveUserByID(ctx context.Context, id uint64) (Admin, bool, error)
	CreateUser(ctx context.Context, admin Admin) (uint64, error)
}
