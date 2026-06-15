package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	redis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var (
	rpcLogOnce sync.Once
	rpcLogFile *os.File
)

// ensureRPCLogger 确保 RPC 层日志初始化一次，供标准库 log 与 zap 共用。
func ensureRPCLogger() {
	rpcLogOnce.Do(func() {
		f, err := InitFileLogger("rpc")
		if err != nil {
			log.SetFlags(log.LstdFlags | log.Lmicroseconds)
			return
		}
		rpcLogFile = f
	})
}

// InitFileLogger 初始化服务日志输出，并把 log 与 zap 同时接到相同输出目标。
func InitFileLogger(serviceName string) (*os.File, error) {
	if serviceName == "" {
		serviceName = "app"
	}
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, err
	}
	logPath := filepath.Join("logs", serviceName+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	_ = initZapLogger(io.MultiWriter(os.Stdout, f))
	return f, nil
}

// initZapLogger 初始化全局 zap logger，供 worker、RPC 与基础设施层统一使用。
func initZapLogger(out io.Writer) error {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(out),
		zap.InfoLevel,
	)
	l := zap.New(core)
	zap.ReplaceGlobals(l)
	return nil
}

// UnaryAccessLogInterceptor 记录一元 gRPC 请求的调用方法、来源地址、状态码和耗时。
func UnaryAccessLogInterceptor() grpc.UnaryServerInterceptor {
	ensureRPCLogger()
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		addr := ""
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			addr = p.Addr.String()
		}

		zap.L().Info("rpc_unary",
			zap.String("method", info.FullMethod),
			zap.String("peer", addr),
			zap.String("code", code.String()),
			zap.Int64("latency_ms", dur.Milliseconds()),
		)
		return resp, err
	}
}

type idempotencyRecord struct {
	Status      string `json:"status"`
	Fingerprint string `json:"fingerprint"`
	GRPCCode    int32  `json:"grpc_code"`
	GRPCMessage string `json:"grpc_message"`
	RespType    string `json:"resp_type"`
	RespB64     string `json:"resp_b64"`
}

// UnaryIdempotencyInterceptor 为一元 RPC 提供幂等保护，避免上传等操作被重复提交。
func UnaryIdempotencyInterceptor(rdb *redis.Client) grpc.UnaryServerInterceptor {
	ttl := 10 * time.Minute

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if rdb == nil {
			return handler(ctx, req)
		}
		if info == nil {
			return handler(ctx, req)
		}

		md, _ := metadata.FromIncomingContext(ctx)
		key := strings.TrimSpace(firstMD(md, "idempotency-key"))
		if key == "" {
			return handler(ctx, req)
		}

		fpReq := strings.TrimSpace(firstMD(md, "idempotency-fingerprint"))
		fpLocal := fingerprintUnary(info.FullMethod, req)
		if fpReq != "" && fpReq != fpLocal {
			return nil, status.Error(codes.InvalidArgument, "idempotency-fingerprint mismatch")
		}
		fp := fpLocal
		if fpReq != "" {
			fp = fpReq
		}

		rkey := "rpc:idmp:" + info.FullMethod + ":" + key
		recProcessing := idempotencyRecord{Status: "processing", Fingerprint: fp}
		b, _ := json.Marshal(recProcessing)

		ok, redisErr := rdb.SetNX(ctx, rkey, string(b), ttl).Result()
		if redisErr != nil {
			return nil, status.Error(codes.Internal, redisErr.Error())
		}

		if !ok {
			raw, getErr := rdb.Get(ctx, rkey).Result()
			if getErr != nil {
				return nil, status.Error(codes.Aborted, "idempotency state unavailable")
			}
			var rec idempotencyRecord
			if err := json.Unmarshal([]byte(raw), &rec); err != nil {
				return nil, status.Error(codes.Aborted, "idempotency state corrupted")
			}
			if rec.Fingerprint != "" && rec.Fingerprint != fp {
				return nil, status.Error(codes.InvalidArgument, "idempotency payload mismatch")
			}
			if rec.Status != "done" {
				return nil, status.Error(codes.Aborted, "request is processing")
			}
			if rec.GRPCCode != int32(codes.OK) {
				return nil, status.Error(codes.Code(rec.GRPCCode), rec.GRPCMessage)
			}
			if rec.RespB64 == "" || rec.RespType == "" {
				return nil, nil
			}
			msg, uerr := unmarshalResp(rec.RespType, rec.RespB64)
			if uerr != nil {
				return nil, status.Error(codes.Internal, uerr.Error())
			}
			return msg, nil
		}

		defer func() {
			if r := recover(); r != nil {
				err = status.Error(codes.Internal, "panic")
				rec := idempotencyRecord{
					Status:      "done",
					Fingerprint: fp,
					GRPCCode:    int32(status.Code(err)),
					GRPCMessage: status.Convert(err).Message(),
				}
				b, _ := json.Marshal(rec)
				_ = rdb.Set(ctx, rkey, string(b), ttl).Err()
				_ = debug.Stack()
			}
		}()

		resp, err = handler(ctx, req)
		if err != nil {
			rec := idempotencyRecord{
				Status:      "done",
				Fingerprint: fp,
				GRPCCode:    int32(status.Code(err)),
				GRPCMessage: status.Convert(err).Message(),
			}
			b, _ := json.Marshal(rec)
			_ = rdb.Set(ctx, rkey, string(b), ttl).Err()
			return resp, err
		}

		rec := idempotencyRecord{
			Status:      "done",
			Fingerprint: fp,
			GRPCCode:    int32(codes.OK),
			GRPCMessage: "",
		}
		if m, ok := resp.(proto.Message); ok && m != nil {
			out, merr := proto.Marshal(m)
			if merr == nil {
				rec.RespType = string(proto.MessageName(m))
				rec.RespB64 = base64.StdEncoding.EncodeToString(out)
			}
		}
		b, _ = json.Marshal(rec)
		_ = rdb.Set(ctx, rkey, string(b), ttl).Err()
		return resp, nil
	}
}

type capturingServerStream struct {
	grpc.ServerStream
	lastSent any
}

// SendMsg 捕获服务端流最后一次发送的消息，用于幂等重放场景。
func (s *capturingServerStream) SendMsg(m any) error {
	s.lastSent = m
	return s.ServerStream.SendMsg(m)
}

// StreamIdempotencyInterceptor 为流式 RPC 提供幂等保护。
// 当前主要用于上传视频场景，复用第一次成功响应。
func StreamIdempotencyInterceptor(rdb *redis.Client) grpc.StreamServerInterceptor {
	ttl := 10 * time.Minute

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if rdb == nil || ss == nil || info == nil {
			return handler(srv, ss)
		}

		md, _ := metadata.FromIncomingContext(ss.Context())
		key := strings.TrimSpace(firstMD(md, "idempotency-key"))
		if key == "" {
			return handler(srv, ss)
		}

		fpReq := strings.TrimSpace(firstMD(md, "idempotency-fingerprint"))
		fp := fpReq
		if fp == "" {
			sum := sha256.Sum256([]byte(info.FullMethod))
			fp = hex.EncodeToString(sum[:])
		}

		rkey := "rpc:idmp:" + info.FullMethod + ":" + key
		recProcessing := idempotencyRecord{Status: "processing", Fingerprint: fp}
		b, _ := json.Marshal(recProcessing)
		ok, redisErr := rdb.SetNX(ss.Context(), rkey, string(b), ttl).Result()
		if redisErr != nil {
			return status.Error(codes.Internal, redisErr.Error())
		}

		if !ok {
			raw, getErr := rdb.Get(ss.Context(), rkey).Result()
			if getErr != nil {
				return status.Error(codes.Aborted, "idempotency state unavailable")
			}
			var rec idempotencyRecord
			if err := json.Unmarshal([]byte(raw), &rec); err != nil {
				return status.Error(codes.Aborted, "idempotency state corrupted")
			}
			if rec.Fingerprint != "" && rec.Fingerprint != fp {
				return status.Error(codes.InvalidArgument, "idempotency payload mismatch")
			}
			if rec.Status != "done" {
				return status.Error(codes.Aborted, "request is processing")
			}
			if rec.GRPCCode != int32(codes.OK) {
				return status.Error(codes.Code(rec.GRPCCode), rec.GRPCMessage)
			}
			if rec.RespB64 != "" && rec.RespType != "" {
				msg, uerr := unmarshalResp(rec.RespType, rec.RespB64)
				if uerr != nil {
					return status.Error(codes.Internal, uerr.Error())
				}
				if err := ss.SendMsg(msg); err != nil {
					return err
				}
			}
			return nil
		}

		wrapped := &capturingServerStream{ServerStream: ss}
		err := handler(srv, wrapped)

		if err != nil {
			rec := idempotencyRecord{
				Status:      "done",
				Fingerprint: fp,
				GRPCCode:    int32(status.Code(err)),
				GRPCMessage: status.Convert(err).Message(),
			}
			b, _ := json.Marshal(rec)
			_ = rdb.Set(ss.Context(), rkey, string(b), ttl).Err()
			return err
		}

		rec := idempotencyRecord{
			Status:      "done",
			Fingerprint: fp,
			GRPCCode:    int32(codes.OK),
		}
		if m, ok := wrapped.lastSent.(proto.Message); ok && m != nil {
			out, merr := proto.Marshal(m)
			if merr == nil {
				rec.RespType = string(proto.MessageName(m))
				rec.RespB64 = base64.StdEncoding.EncodeToString(out)
			}
		}
		b, _ = json.Marshal(rec)
		_ = rdb.Set(ss.Context(), rkey, string(b), ttl).Err()
		return nil
	}
}

// firstMD 从 metadata 中读取第一个非空值。
func firstMD(md metadata.MD, key string) string {
	if md == nil {
		return ""
	}
	vs := md.Get(key)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// fingerprintUnary 基于方法名和请求体构造一元 RPC 的幂等指纹。
func fingerprintUnary(fullMethod string, req any) string {
	var body []byte
	if m, ok := req.(proto.Message); ok && m != nil {
		body, _ = protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	} else {
		body, _ = json.Marshal(req)
	}
	sum := sha256.Sum256(append([]byte(fullMethod+"|"), body...))
	return hex.EncodeToString(sum[:])
}

// unmarshalResp 根据 protobuf 消息全名重建历史响应，用于幂等命中时直接返回。
func unmarshalResp(respType string, respB64 string) (proto.Message, error) {
	name := protoreflect.FullName(respType)
	mt, err := protoregistry.GlobalTypes.FindMessageByName(name)
	if err != nil {
		return nil, err
	}
	msg := mt.New().Interface()
	b, err := base64.StdEncoding.DecodeString(respB64)
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(b, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// StreamAccessLogInterceptor 记录流式 gRPC 请求的调用方法、状态与耗时。
func StreamAccessLogInterceptor() grpc.StreamServerInterceptor {
	ensureRPCLogger()
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		dur := time.Since(start)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		addr := ""
		if p, ok := peer.FromContext(ss.Context()); ok && p.Addr != nil {
			addr = p.Addr.String()
		}

		zap.L().Info("rpc_stream",
			zap.String("method", info.FullMethod),
			zap.String("peer", addr),
			zap.String("code", code.String()),
			zap.Int64("latency_ms", dur.Milliseconds()),
		)
		return err
	}
}
