---
name: new-subdomain
description: 新增业务子域（新模块/新业务包）时使用——按 vertical slice 生成 repository/service/handler 三件套并装配 Wire 与路由。改字段或扩现有 handler 不走本 skill。
---

# 新增业务子域

按 vertical slice 风格落盘——同包内 `repository.go` / `service.go` / `handler.go` 三件套；DTO 集中在 `api/apiv1/`。

> 跨子域依赖必须由消费者侧声明 interface，再用 `wire.Bind` 注入；禁止包间直接导入具体结构体类型。

## 第一步：归属判定（强制询问）

| 归属 | 目录 | 适用 |
|------|------|------|
| 系统级 / 基座 | `internal/admin/<域>/` | RBAC、菜单、API 管理、配置中心、日志中心、审计等平台能力 |
| 业务级 | `internal/app/<域>/` | 订单、商品、会员、内容等具体业务功能 |

- 用户未明示时，先用 `AskUserQuestion` 二选一。
- 业务级目录 `internal/app/` 目前为空，第一个业务子域要顺手把目录建出来。
- IAM 聚合（账号 / 角色 / 权限 / 菜单 / API / casbinkit）位于 `internal/admin/iam/<子域>/`，新增 IAM 相关子域继续落入该聚合；其他独立的系统能力（audit、sysconfig 等）直接落 `internal/admin/<域>/`，不要嵌入 `iam/`。

## 第二步：生成骨架

在选定目录下创建：

- `repository.go` — 定义 `Repo` 结构体与 `NewRepo(db *gorm.DB, ...) *Repo`，承担数据访问。涉及 DB + Casbin 的写操作方法名以 `Atomic` 结尾，参考 `internal/admin/iam/menu/repository.go` 的 `MenuUpdateAtomic / MenuDeleteAtomic`，必须在同一事务里同步清理 Casbin 策略。
- `service.go` — 定义 `Service` 结构体与 `NewService(...)`。**跨子域只声明本侧 interface**（参考 `internal/admin/iam/menu` 中的 `PermissionReader`），由 Wire `wire.Bind` 绑定到上游 `*Repo`。
- `handler.go` — 定义 `Handler` 结构体与 `NewHandler(svc *Service)`；每个方法上方写 Swagger 注释；统一通过 `apiv1.WriteResponse(ctx, err, nil)` / `apiv1.HandleSuccess(ctx, data)` 出参。

DTO（请求 / 响应）集中放 `api/apiv1/<topic>.go`（参考 `admin.go`），**严禁散落到子域包内**。

## 第三步：装配 Wire

- 编辑 `cmd/server/wire/wire.go`。
- 系统级 / RBAC 相关追加到现有 `rbacSet`：登记 `xxx.NewRepo` / `NewService` / `NewHandler`。
- 业务级新建对应 `wire.NewSet`（如 `appSet`），并在 `NewWire` 的 `wire.Build(...)` 列表中加入。
- 跨子域依赖：在消费侧声明 interface 后，加 `wire.Bind(new(consumer.SomeReader), new(*provider.Repo))`，对齐现有 `wire.Bind(new(menu.PermissionReader), new(*permission.Repo))` 形态。
- 执行 `make wire` 重生成 `wire_gen.go`。

## 第四步：挂路由

在 `internal/server/http.go`：

| 场景 | 分组 | 说明 |
|------|------|------|
| 登录、刷新、注销等公开接口 | `noAuth` | 不走 `StrictAuth + AuthMiddleware` |
| 默认业务接口 | `strict` | 已组合 `StrictAuth + AuthMiddleware`，Casbin 兜底 |

路径约定：系统级走 `/admin/<资源>`，业务级遵循各域命名，**保持单复数与现有 handler 一致**（列表用复数：`/admin/users`，单体用单数：`/admin/user`）。

## 第五步：刷文档 + 质量门禁

```bash
make swag      # 同步 docs/swagger
make check     # fmt + vet + lint + race test + migrate-validate
```

## 第六步：闭环提示

若新增接口要被普通账号调用，提醒用户继续走 `permission-checklist` skill（API 登记 → 菜单登记 → 角色授权 → 账号绑定）。超管 ID=1 自动放行，**不能**作为联调验证。
