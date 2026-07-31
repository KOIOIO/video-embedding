# legacy-video

这个目录是历史 Go 后端工程，包含早期的 gRPC / API / worker 相关代码。

当前仓库后续部署和 Java 系统对接时，推荐使用 `../legacy-video-http/`。除非明确需要维护历史链路，不建议把新功能加到这个目录。

本目录的文档只说明历史工程边界；现行 API、配置与运维命令请查阅 `../legacy-video-http/README.md` 和根目录参数文档。

## 目录提示

- `cmd/`：历史入口命令。
- `configs/`：历史配置文件。
- `internal/`：历史业务与基础设施实现。
- `video/`：gRPC proto 及生成代码。

## 运行与验证

本目录有独立 `go.mod`（Go `1.26.1`）。仅在维护历史链路时从本目录执行：

```bash
go test ./...
```

历史入口以 `cmd/` 下现存命令为准。不要从仓库根目录运行 Go 命令，也不要把这里的 gRPC 契约作为新 Java 集成的依据。

## 说明

根目录没有 Go module；现行 HTTP 服务和 worker 的启动、测试、配置及部署命令不适用于本目录。
