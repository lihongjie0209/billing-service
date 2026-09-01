# billing-service

平台账单服务，负责套餐、用量价格、租户订阅、账单、支付尝试和退款。服务不读取其他服务数据库：用量通过 `metering-service` gRPC 查询，权限通过 `authorization-service` 决策。

## 接口

- 浏览器和管理端使用 `/api/v1/**` 的 POST+JSON API，响应统一为 `{code,message,body,request_id}`。
- 服务间使用中央仓库 `platform-protos` 的 `platform.billing.v1.BillingService`，独立 gRPC 端口默认为 `9090`。
- 实现通用 Export Provider 的 `billing.invoices` 和 Import Provider 的 `billing.plans`；导入套餐先逐行规范化与校验，应用时以套餐 code 作为持久幂等边界。
- `/live`、`/ready`、`/metrics`、受保护 Swagger/pprof 由公共脚手架能力提供。
- JWT 由 identity-service 签发并通过 JWKS 校验；免认证与 PSK 路由均支持配置通配符。
- 套餐和用量价格是平台全局目录，其创建、更新、查询和删除统一在 `__platform__` 范围授权；订阅、账单和支付仍按当前主体的租户与应用范围授权，并在领域层再次校验 tenant scope。

OpenAPI UI 默认位于 `/swagger/index.html`。主要前端路由：

- `/api/v1/plans/{create,update,get,list}` 与 `/plans/usage-prices/{upsert,delete}`
- `/api/v1/subscriptions/{create,change,cancel,get,list}`
- `/api/v1/invoices/{preview,generate,finalize,void,get,list}`
- `/api/v1/payments/create-attempt`、`/payments/apply-result`、`/payments/refunds/record`

## 数据与并发

- PostgreSQL 为默认数据库，数据库 `platform`、schema `billing`、迁移表 `billing_schema_migrations`。
- PostgreSQL、Kingbase 使用 schema 隔离；MySQL 使用独立数据库。三种迁移均由本服务维护。
- 所有领域表都有 `version` 以及 `created_at/updated_at/created_by/updated_by`；更新使用乐观锁。
- 发票生成、支付创建、提供商回调和退款均有持久化幂等记录。
- 每个租户只允许一个当前订阅；立即/期末取消会安全释放占用。下周期换套餐由服务内定时任务推进，并用 Redis 分布式锁限制并发实例。
- 金额使用最小货币单位的 `int64`，时间存为带时区时间，默认展示时区为 `Asia/Shanghai`。

## 事件总线

领域事务将 protobuf `platform.common.v1.EventEnvelope` 写入 `billing_outbox_events`。公共 SDK dispatcher 在提交后发布到 NATS JetStream 的共享 `PLATFORM_EVENTS` 流，主题统一为 `platform.billing.*.v1`，流 subjects 固定为 `platform.>`。发布失败可重试，数据库提交不会因消息系统短暂故障丢失。

## 配置

配置优先级为命令行 profile/环境变量、`config-{profile}.yaml`、基础 `config.yaml`、默认值。支持 `development`、`test`、`production`，所有字段可用 `APP_` 前缀环境变量覆盖。生产密钥、JWKS 地址、数据库凭据、PSK 和上游凭据必须由 Secret 注入，禁止提交真实值。

```bash
go run ./cmd/api -config config/config.yaml -env development
go run ./cmd/migrate -config config/config.yaml -direction up
```

服务启动迁移可由 `migration.auto_up` 控制；Kubernetes 默认使用 init container，迁移按 billing 专属 schema 和记录表加锁。

## 开发与测试

```bash
make test
make test-race
make lint
make swagger-check
make build
```

本机只运行单元、race、vet、lint、生成漂移和配置检查。`make test-integration` 使用 Testcontainers 验证 PostgreSQL/MySQL 迁移、Redis 锁/幂等及 HTTP/gRPC，但按项目约定仅由 GitHub CI 执行。集成测试不依赖其他服务运行，所有上游使用本服务内 fake 或协议 stub。

构建通过 ldflags 注入版本、Git commit 和构建时间；`api -version` 可验证产物元数据。
