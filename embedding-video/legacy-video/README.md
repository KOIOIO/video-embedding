# legacy-video

这个目录是历史 Go 后端工程，包含早期的 gRPC / API / worker 相关代码。

当前仓库后续部署和 Java 系统对接时，推荐使用 `../video-service/`。除非明确需要维护历史链路，不建议把新功能加到这个目录。

## 目录提示

- `cmd/`：历史入口命令。
- `configs/`：历史配置文件。
- `internal/`：历史业务与基础设施实现。
- `video/`：gRPC proto 及生成代码。

## 说明

根目录没有 Go module；如果确实需要运行或维护本历史工程，应先进入本目录再执行 Go 命令。
