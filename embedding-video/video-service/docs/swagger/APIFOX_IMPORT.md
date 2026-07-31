# Apifox 导入说明

## 导入文件

在 Apifox 中选择“项目设置 → 导入数据 → OpenAPI/Swagger”，导入以下文件：

- 推荐：`docs/swagger/swagger.json`
- 备选：`docs/swagger/swagger.yaml`

两个文件内容等价，均使用 Swagger 2.0 格式。不要导入 `docs.go`。

## 环境配置

本地环境的基础 URL 可配置为：

```text
http://localhost:8081
```

前端联调环境也可以配置为 Vite 或 Nginx 的访问地址，由其代理 `/api`、`/videos` 和 `/knowledge-video-media`。

## 接口分类

导入后接口按以下业务域分组：

1. 系统与健康
2. 视频资源
3. 视频上传
4. 视频播放与转码
5. 视频互动
6. 视频片段
7. 题目
8. 推荐
9. 推荐管理
10. 知识点视频
11. 内部接口
12. 媒体访问

`内部接口` 用于服务间联调，不建议客户端形成直接依赖。文档只收录标准 REST 路径，不收录 `/api/video/...`、`/api/question/...` 等历史兼容别名。

## 文件上传

普通视频和知识点视频导入接口使用 `multipart/form-data`。在 Apifox 调试时应按接口参数选择本地文件，不要将文件内容改成 JSON 字符串。

媒体访问接口返回 HLS 清单或二进制分片，并支持 HTTP Range。Apifox 可用于检查状态码和响应头，实际播放建议使用浏览器或 HLS 播放器。

## 更新文档

修改 handler Swagger 注解后，在服务模块目录执行：

```bash
go generate ./docs/swagger
go test ./internal/http/router -run Swagger
```

生成文件必须与注解一同提交，保证仓库中的 Apifox 导入文件与运行接口一致。
