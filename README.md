# rabc-go

基于 Gin + GORM + Casbin + Atlas + Vue3 + Ant Design Vue 的 RBAC 后台脚手架。

## 技术栈

| 类别 | 选型 |
|------|------|
| HTTP | Gin |
| ORM | GORM |
| 权限 | Casbin + gorm-adapter |
| 认证 | golang-jwt |
| 配置 | Viper |
| 日志 | zap + lumberjack |
| DI | Google Wire |
| 迁移 | Atlas |
| 前端 | Vue3 + Vite (Mist) + Ant Design Vue + UnoCSS |

## 目录结构

| 路径 | 作用 |
|------|------|
| `cmd/server` | HTTP API 与内嵌前端 |
| `cmd/seed` | 写入 RBAC 初始数据 |
| `cmd/task` | 定时任务进程 |
| `cmd/dbmigrate` | Atlas 命令封装 |
| `internal/handler` | HTTP 控制器 |
| `internal/service` | 业务逻辑 |
| `internal/repository` | GORM 与 Casbin 数据访问 |
| `internal/model` | GORM struct，schema 来源 |
| `internal/middleware` | JWT / Casbin / 日志等中间件 |
| `internal/server` | server 与 seed 装配 |
| `pkg/` | config / log / jwt 等基础设施 |
| `db/atlas` | GORM → Atlas schema 桥接 |
| `db/migrations/{mysql,postgres}` | 版本化 SQL |
| `web/` | Vue3 前端，`web/dist` 由 `web/embed.go` 内嵌 |
| `deploy/docker-compose` | 本地依赖容器 |
| `deploy/build` | 生产镜像 Dockerfile |

## 环境要求

- Go 1.25+
- Node.js 18.15+
- Docker / Docker Compose
- Atlas CLI

```bash
make init                                      # Go 工具链：Wire / mockgen / swag / nunu
brew install ariga/tap/atlas                   # 或 curl -sSf https://atlasgo.sh | sh
```

## 快速开始

```bash
make bootstrap     # 起依赖容器 → migrate-apply → seed → nunu run
```

| 服务 | 地址 |
|------|------|
| 后端 + 内嵌前端 | http://127.0.0.1:8000 |
| Swagger | http://127.0.0.1:8000/swagger/index.html |
| 前端 dev server | http://127.0.0.1:6678 |

| 默认账号（仅 `env: local`） | 密码 |
|------|------|
| `admin` | `123456` |
| `user` | `123456` |

非 local 环境必须设 `APP_SEED_INITIAL_PASSWORD`，否则 seed 启动 panic。

## 手动启动

```bash
docker compose -f deploy/docker-compose/docker-compose.yml up -d --wait
make migrate-apply
make seed
go run ./cmd/server          # 或 nunu run ./cmd/server
```

前端独立开发：

```bash
cd web && npm install && npm run dev
```

不使用 Docker、直连宿主 DB 的步骤：见 [db/README.md](db/README.md)。

## 配置

`config/<env>.yml` + `APP_*` 环境变量（点号 → 下划线）。

| YAML | 环境变量 |
|------|---------|
| `http.port` | `APP_HTTP_PORT` |
| `security.jwt.key` | `APP_SECURITY_JWT_KEY` |
| `data.db.user.driver` | `APP_DATA_DB_USER_DRIVER` |
| `data.db.user.dsn` | `APP_DATA_DB_USER_DSN` |
| `data.db.debug` | `APP_DATA_DB_DEBUG` |
| `data.redis.addr` | `APP_DATA_REDIS_ADDR` |
| `data.redis.password` | `APP_DATA_REDIS_PASSWORD` |
| `seed.initial_password` | `APP_SEED_INITIAL_PASSWORD` |
| `log.body.enabled` | `APP_LOG_BODY_ENABLED` |

Atlas 迁移目标库复用 `APP_DATA_DB_USER_DRIVER` / `APP_DATA_DB_USER_DSN`。

## 数据库

| 职责 | 入口 |
|------|------|
| 运行时连接 | `data.db.user.{driver,dsn}` |
| Schema DDL | Atlas + `db/migrations/<dialect>/` |
| 业务 DML | `cmd/seed` |

两条工作流：

```bash
# A. 本地快速迭代（不生成 SQL；仅本地 DSN 允许）
make push

# B. 版本化（commit / 部署必走）
make migrate-diff name=add_xxx
make migrate-lint
make migrate-apply
```

应用启动**不执行** `AutoMigrate`，Casbin adapter 的隐式建表也已关闭——
schema 全部由 Atlas 管控，避免多副本启动期抢着 DDL。新增需 Atlas 管理的表，
需在 `db/atlas/main.go` 的 `models()` 显式登记。

migration 按方言隔离在 `db/migrations/{mysql,postgres}/`，不可跨库混用。
完整流程与场景对照：[db/README.md](db/README.md)。

## 权限开发流程

1. 新增后端路由 / Handler / Service / Repository。
2. 后台「接口管理」登记 API。
3. 后台「菜单管理」登记菜单。
4. 后台「角色管理」给角色分配菜单 / API。
5. 后台「管理员管理」给账号绑定角色。

策略格式：

```text
p, <role_sid>, api:/v1/admin/users, GET
p, <role_sid>, menu:/access/admin, read
g, <user_id>, <role_sid>
```

超管账号 ID 固定为 `1`，后端鉴权直接放行（防呆设计，避免误删权限导致系统不可恢复）；
普通账号必须命中 Casbin 策略。

## 常用命令

| 命令 | 作用 |
|------|------|
| `make help` | 列出全部 target |
| `make init` | 安装 Wire / mockgen / swag / nunu |
| `make bootstrap` | 起依赖 → 迁移 → seed → 热加载启服 |
| `make test` | `go test -race ./...` |
| `make check` | vet + lint + race test + migrate validate |
| `make mock` | 重新生成 service/repository mock |
| `make swag` | 刷新 Swagger 到 `./docs` |
| `make build` | 构建前端 + 后端二进制到 `bin/server` |
| `make clean` | 清理 `bin/` 与 `web/dist/` |
| `make push` | GORM struct 同步到本地 DB |
| `make migrate-diff name=xxx` | 生成新 migration |
| `make migrate-apply` | 应用待执行 migration |
| `make migrate-status` | 查看 migration 状态 |
| `make migrate-lint [base=origin/main]` | 检测破坏性变更 |
| `make migrate-validate` | 校验 migration 目录 |
| `make migrate-hash` | 重算 `atlas.sum` |
| `make seed` | 写入初始数据（要求 RBAC 表空） |
| `make seed-reset` | 仅 local：清表后重新 seed |
| `make docker-server` | 构建 server 镜像 |
| `make docker-task` | 构建 task 镜像 |

## 部署

```bash
make build
./bin/server -conf config/prod.yml
```

- migration 在应用启动前由流水线执行：`APP_DATA_DB_USER_DSN=... make migrate-apply`。
- seed 仅首次部署执行。
- 敏感配置走环境变量；`config/prod.yml` 留空占位。
- 前端两种部署模式：内嵌进 server 二进制，或独立部署反代 `/v1`。

镜像构建：

```bash
make docker-server REGISTRY=registry.example.com VERSION=v1.2.3
make docker-task   REGISTRY=registry.example.com VERSION=v1.2.3
```

## License

[MIT](./LICENSE)
