# docs

这个目录存放仓库级文档、架构材料和演示文稿。当前可部署服务为
`../video-service/`；根目录不包含可运行的 Go module。

## 仓库级文档

- `presentations/`：视频服务架构评审演示文稿及生成脚本。
- `superpowers/specs/`：HTTP 迁移、向量分段、统一控制台和 RecBole 指标展示等设计记录。
- `superpowers/plans/`：与上述设计对应的实施计划。这些文件记录历史决策，不替代当前运行手册。

最近的控制台设计入口：

- [`superpowers/specs/2026-07-13-frontend-console-merge-design.md`](superpowers/specs/2026-07-13-frontend-console-merge-design.md)
- [`superpowers/plans/2026-07-13-frontend-console-merge-plan.md`](superpowers/plans/2026-07-13-frontend-console-merge-plan.md)

## 当前运行文档

- [仓库总览](../README.md)
- [HTTP 服务说明](../video-service/README.md)
- [服务器部署手册](../deployment/DEPLOYMENT.md)
- [RecBole 训练说明](../recbole-training/README.md)
- [前端联调控制台](../hls-web/README.md)
- [下游服务就绪检查](../video-service/docs/downstream-service-readiness-review.md)
- [RecBole 推荐流水线](../video-service/docs/recbole-recommendation-pipeline.md)
- [Gorse 可选集成运行手册](../video-service/docs/gorse-recommendation-runbook.md)
- [Swagger / OpenAPI](../video-service/docs/swagger/swagger.yaml)

`video-service/docs/swagger/` 是生成产物目录，请通过注解和生成流程更新，不要直接手改生成文件。
