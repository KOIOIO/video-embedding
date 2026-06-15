package impl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"legacy-video/internal/api/client"
	"legacy-video/video"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// withIdempotency 为上传链路生成幂等 metadata，并透传给 RPC 层拦截器。
func withIdempotency(c *gin.Context, fullMethod string, req any) context.Context {
	var body []byte
	if m, ok := req.(proto.Message); ok {
		body, _ = protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	} else {
		body, _ = json.Marshal(req)
	}

	fp := sha256.Sum256(append([]byte(fullMethod+"|"), body...))
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		key = "auto-" + hex.EncodeToString(fp[:16])
	}

	base := context.Background()
	if c != nil && c.Request != nil {
		base = c.Request.Context()
	}

	return metadata.AppendToOutgoingContext(base,
		"idempotency-key", key,
		"idempotency-fingerprint", hex.EncodeToString(fp[:]),
	)
}

// grpcStreamWriter 把 io.Writer 调用转成 gRPC 客户端流消息，便于直接复用 io.Copy。
type grpcStreamWriter struct {
	stream video.VideoService_UploadVideoClient
}

// Write 把一块文件字节包装成 chunk_data 消息发到上传流。
func (w *grpcStreamWriter) Write(p []byte) (int, error) {
	err := w.stream.Send(&video.UploadVideoRequest{
		Data: &video.UploadVideoRequest_ChunkData{
			ChunkData: p,
		},
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// UploadVideo 上传视频API
func UploadVideo(c *gin.Context) {
	// 获取视频文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "获取文件失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// 发送元数据
	meta := &video.VideoMeta{
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Title:       c.PostForm("title"),
		Description: c.PostForm("description"),
	}

	// 创建 gRPC 流式请求后，HTTP multipart 文件会边读边转发到 RPC 上传流。
	stream, err := client.GetVideoClient().UploadVideo(withIdempotency(c, "/video.VideoService/UploadVideo", meta))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "创建上传流失败: " + err.Error(),
		})
		return
	}

	if err := stream.Send(&video.UploadVideoRequest{
		Data: &video.UploadVideoRequest_Meta{
			Meta: meta,
		},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "发送元数据失败: " + err.Error(),
		})
		return
	}

	// 发送文件数据
	w := &grpcStreamWriter{stream: stream}
	if _, err := io.Copy(w, file); err != nil && err != io.EOF {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "发送数据失败: " + err.Error(),
		})
		return
	}

	// 接收响应
	response, err := stream.CloseAndRecv()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "接收响应失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   response.Success,
		"message":   response.Message,
		"file_name": response.FileName,
		"raw_url":   response.RawUrl,
		"hls_url":   response.HlsUrl,
		"task_id":   response.TaskId,
		"video_id":  response.VideoId,
	})
}
