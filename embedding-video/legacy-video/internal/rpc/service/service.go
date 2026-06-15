package service

import (
	"legacy-video/internal/application/videoapp"
	"legacy-video/internal/config"
	"legacy-video/video"

	"gorm.io/gorm"
)

// VideoService 是 gRPC 协议层适配器。
// 它负责把 protobuf 请求转换为 application 层输入，再把结果映射回 protobuf 响应。
type VideoService struct {
	video.UnimplementedVideoServiceServer
	App *videoapp.Service
	DB  *gorm.DB
	Cfg *config.Config
}

// NewVideoService 创建 gRPC 服务实例。
func NewVideoService(app *videoapp.Service, db *gorm.DB, cfg *config.Config) *VideoService {
	return &VideoService{App: app, DB: db, Cfg: cfg}
}
