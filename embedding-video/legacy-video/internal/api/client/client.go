package client

import (
	"context"
	"log"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"legacy-video/video"
)

var (
	mu          sync.RWMutex
	grpcConn    *grpc.ClientConn
	videoClient video.VideoServiceClient
)

// InitVideoClient 初始化视频服务客户端
func InitVideoClient(addr string) error {
	// 创建到 RPC 服务的长连接，API 层通过它把 HTTP 请求转发给 gRPC 服务。
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("创建gRPC连接失败: %v\n", err)
		return err
	}

	// 切换全局客户端引用，便于 handler 直接复用。
	mu.Lock()
	if grpcConn != nil {
		_ = grpcConn.Close()
	}
	grpcConn = conn
	videoClient = video.NewVideoServiceClient(conn)
	mu.Unlock()
	log.Printf("视频服务客户端初始化成功: %s\n", addr)
	return nil
}

// GetVideoClient 获取视频服务客户端
func GetVideoClient() video.VideoServiceClient {
	mu.RLock()
	defer mu.RUnlock()
	return videoClient
}

// Close 关闭当前持有的 gRPC 连接。
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if grpcConn == nil {
		return nil
	}
	err := grpcConn.Close()
	grpcConn = nil
	videoClient = nil
	return err
}

// Ping 测试连接
func Ping() error {
	ctx := context.Background()
	// 使用一个轻量级读接口确认 RPC 端是否可达。
	_, err := videoClient.ListVideos(ctx, &video.ListVideosRequest{
		FilterType: video.VideoType_ALL,
	})
	return err
}
