# RABC-Go

RABC-Go 是一个基于 **Gin + GORM + Casbin + Vue3 + Ant Design Vue** 的 RBAC 管理后台模板。项目当前重点提供后台权限管理的基础闭环：登录认证、菜单/API 权限、角色授权、管理员管理、数据库版本化迁移与前端静态资源打包。

> 默认数据库是 **MySQL**。运行时连接层仍支持 GORM 的 `mysql`、`postgres`、`sqlite`、`sqlserver` 驱动；Atlas migration 目前内置 MySQL 与 PostgreSQL 两套方言目录，默认使用 MySQL。

## 实际能力

- **JWT 登录认证**：`/v1/login` 签发 Token，受保护接口通过 `Authorization` 访问。
- **Casbin RBAC 权限控制**：接口权限使用 `api:<path> + HTTP Method`，菜单权限使用 `menu:<path> + read`。
- **权限后台**：支持管理员、角色、菜单、API 资源的增删改查，以及角色权限分配。
- **超管防呆**：ID 为 `1` 的超管在后端鉴权中直接放行，避免误删权限导致无法恢复。
- **默认 MySQL + Atlas 迁移**：GORM struct 是 schema 来源，Atlas 当前负责生成和应用 MySQL 版本化 SQL migration。
- **Seed 初始数据**：`cmd/seed` 只写业务初始数据，不建表、不删表、不执行 DDL。
- **Swagger 文档**：服务启动后访问 `/swagger/index.html`。
- **前端集成**：后端可嵌入并托管 `web/dist`，也支持前后端分离开发。

## 技术栈

| 类别 | 技术 | 说明 |
|------|------|------|
| HTTP | Gin | 路由、中间件、参数绑定 |
| ORM | GORM | 默认连接 MySQL，运行时支持 mysql/postgres/sqlite/sqlserver，schema 由 Atlas 管理 |
| 权限 | Casbin | RBAC 策略存储在 `casbin_rule` 表 |
| 认证 | golang-jwt | JWT 签发与校验 |
| 配置 | Viper | YAML + 环境变量覆盖 |
| 日志 | zap + lumberjack | 结构化日志与日志滚动 |
| DI | Wire | 编译期依赖注入 |
| 迁移 | Atlas | 版本化 SQL migration |
| 前端 | Vue3 + Vite/Mist + Ant Design Vue | 管理后台 UI |

## 环境要求

- Git
- Go 1.25 或更高版本
- Node.js 18.15 或更高版本
- Docker / Docker Compose
- Atlas CLI

安装 Go 开发工具：

```bash
make init
```

安装 Atlas CLI：

```bash
# macOS
brew install ariga/tap/atlas

# 其他平台
curl -sSf https://atlasgo.sh | sh
```

## 快速开始

一键启动本地后端开发环境：

```bash
make bootstrap
```

该命令会依次执行：

1. 启动 `deploy/docker-compose` 中的 MySQL 容器。
2. 执行 `make migrate-apply` 应用 Atlas migration。
3. 执行 `go run ./cmd/seed` 写入初始账号、菜单、API 与 Casbin 策略。
4. 执行 `nunu run ./cmd/server` 热加载启动后端服务。

默认访问地址：

| 服务 | 地址 |
|------|------|
| 后端与内嵌前端 | `http://127.0.0.1:8000` |
| Swagger | `http://127.0.0.1:8000/swagger/index.html` |

默认账号：

| 角色 | 用户名 | 密码 |
|------|--------|------|
| 超级管理员 | `admin` | `123456` |
| 运营人员 | `user` | `123456` |

非 local 环境不会默认使用 `123456`，首次 seed 必须通过 `APP_SEED_INITIAL_PASSWORD` 显式注入初始密码。

## 手动启动

启动本地依赖：

```bash
cd deploy/docker-compose
docker compose up -d --wait
cd ../..
```

应用数据库迁移：

```bash
make migrate-apply
```

写入初始数据：

```bash
make seed
```

启动后端：

```bash
go run ./cmd/server
```

启动前端开发服务：

```bash
cd web
npm install
npm run dev
```

前端开发服务默认端口为 `6678`。开发环境配置见 `web/.env.development`，其中 `VITE_APP_BASE_API_DEV=/dev-api` 会代理到前端 Nitro mock 服务；直连后端时可按联调需要调整为后端地址。

## 数据库与初始化

项目当前数据库管理分为两条线：

| 职责 | 入口 | 说明 |
|------|------|------|
| 运行时连接 | `data.db.user.driver` + `data.db.user.dsn` | 默认 MySQL；可切换 `mysql`、`postgres`、`sqlite`、`sqlserver` |
| Schema 变更 | Atlas + `db/migrations/<db>/` | 负责建表、改列、索引等 DDL；默认从配置文件识别 driver |
| 业务初始数据 | `cmd/seed` | 负责账号、菜单、API、角色、Casbin 策略等 DML |

常用命令：

```bash
# 生成 migration，name 换成本次变更语义名
make migrate-diff name=add_xxx

# 查看 migration 状态
make migrate-status

# 应用 migration
make migrate-apply

# 校验 migration 目录完整性，CI 可使用
make migrate-validate

# 首次写入初始数据，要求 RBAC 业务表全为空
make seed

# 仅 local 开发可用：清空 RBAC 业务表和策略后重新 seed
make seed-reset
```

注意事项：

- 应用启动不执行 `AutoMigrate`，Casbin adapter 的隐式建表也已关闭。
- 迁移命令读取 `APP_CONF` 或 `config/local.yml`，并支持 `APP_DATA_DB_USER_DRIVER` / `APP_DATA_DB_USER_DSN` 覆盖。
- migration 按方言隔离在 `db/migrations/mysql` 与 `db/migrations/postgres`，不能跨库混用。
- 新增表或字段时，先改 `internal/model/*.go`，再用 `make migrate-diff name=xxx` 生成 SQL。
- 新增需要 Atlas 管理的表时，必须在 `db/atlas/main.go` 中登记 model。
- `make seed-reset` 是破坏性操作，只允许 `env=local` 且 DSN 指向本机。
- 详细数据库流程见 [db/README.md](db/README.md)。

## 权限开发流程

新增后端接口或前端菜单后，需要让权限资源和策略同步：

1. 在后端新增路由、Handler、Service、Repository 等代码。
2. 如涉及新表，按 Atlas 流程生成并应用 migration。
3. 在后台“接口管理”新增 API 资源。
4. 在后台“菜单管理”新增前端菜单。
5. 在后台“角色管理”给角色分配菜单/API 权限。

权限资源格式：

```text
# API 权限
p, <role>, api:/v1/admin/users, GET

# 菜单权限
p, <role>, menu:/access/admin, read

# 用户绑定角色
g, <user_id>, <role_sid>
```

超管账号对应用户 ID `1`，后端鉴权会直接放行；普通账号必须命中 Casbin 策略。

## 常用命令

| 命令 | 作用 |
|------|------|
| `make init` | 安装 Wire、mockgen、swag、nunu 等 Go 工具 |
| `make bootstrap` | 启动依赖、应用迁移、写 seed、热加载启动后端 |
| `make test` | 执行 `go test -race ./...` |
| `make swag` | 重新生成 Swagger 文档 |
| `make build` | 构建前端并编译后端二进制到 `bin/server` |
| `go run ./cmd/server` | 启动 HTTP 服务 |
| `go run ./cmd/task` | 启动定时任务进程 |
| `wire ./cmd/server/wire` | 重新生成 server 依赖注入代码 |
| `wire ./cmd/seed/wire` | 重新生成 seed 依赖注入代码 |
| `wire ./cmd/task/wire` | 重新生成 task 依赖注入代码 |

## 构建部署

构建前后端：

```bash
make build
```

运行编译产物：

```bash
./bin/server -conf config/prod.yml
```

生产部署建议：

- 在应用启动前由部署流水线执行 `go run ./cmd/dbmigrate apply`，该命令会按配置选择方言目录并使用配置中的 DSN。
- 首次部署后再运行 `cmd/seed` 写入初始数据；后续部署不要重复 seed。
- 生产敏感配置通过环境变量注入，映射关系见 `config/prod.yml`。
- 前端可以使用后端内嵌静态资源，也可以独立部署到 Nginx/CDN 后反向代理 `/v1` API。

## 项目入口

| 路径 | 说明 |
|------|------|
| `cmd/server` | HTTP API 与内嵌前端服务 |
| `cmd/seed` | 初始业务数据写入 |
| `cmd/task` | 定时任务进程 |
| `internal/handler` | HTTP 控制器 |
| `internal/service` | 业务逻辑 |
| `internal/repository` | GORM 与 Casbin 数据访问 |
| `internal/model` | GORM model，schema 来源 |
| `db/migrations` | Atlas 版本化 SQL |
| `db/atlas` | GORM model 到 Atlas schema 的桥接 |
| `web` | Vue3 管理后台前端 |
