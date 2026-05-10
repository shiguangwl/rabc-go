# 数据库 Schema 与 Seed

本目录管理数据库 schema 演进与业务初始数据写入。

默认数据库是 **MySQL**。运行时连接层通过 `data.db.user.driver` 支持 `mysql`、`postgres`、`sqlite`、`sqlserver`；`cmd/dbmigrate` 默认读取 `APP_CONF` 或 `config/local.yml`，并按 driver 写入对应方言目录。

## 角色分工

| 路径 | 作用 |
|------|------|
| `internal/model/*.go` | **Schema 真相**。GORM struct 上的 tag 定义表结构 |
| `db/atlas/main.go` | 按 `ATLAS_DIALECT` 把 GORM struct 翻译成目标方言 DDL 的桥接程序 |
| `db/migrations/mysql/` | Atlas 生成并校验过的 MySQL 版本化 SQL 文件 |
| `db/migrations/postgres/` | Atlas 生成并校验过的 PostgreSQL 版本化 SQL 文件 |
| `atlas.hcl` | Atlas 项目配置：声明 schema 来源与 dev/migration 目录 |
| `cmd/seed` | 业务初始数据写入命令（默认要求空库 / `-reset` 重写） |

Schema 与数据严格分离：**atlas 负责 DDL，cmd/seed 负责 DML**。

## 前置条件

```bash
# 1. Atlas CLI（macOS 用 brew，其他平台用脚本）
brew install ariga/tap/atlas
# curl -sSf https://atlasgo.sh | sh

# 2. 本地默认 MySQL 依赖容器可用；PostgreSQL 需自行启动并创建 user/atlas_dev database

# 3. 启动本地依赖容器
cd deploy/docker-compose && docker compose up -d --wait
```

## 日常工作流

### Schema 变更

```bash
# 1. 修改 internal/model/*.go 中的 GORM struct
# 2. 生成新 migration（替换 add_xxx 为本次变更的语义命名）
make migrate-diff name=add_xxx

# 3. 查看待应用 migration
make migrate-status

# 4. 应用到本地 DB
make migrate-apply

# 5. 提交 db/migrations/ 下生成的 SQL + atlas.sum
git add db/migrations/
```

### 业务数据 Seed

```bash
# 首次部署：写入初始数据（要求 RBAC 业务表全为空）
make seed

# 仅 dev/local：清空 RBAC 业务表 + 重新写入
make seed-reset
```

## 与生产部署

| 环境 | schema 演进 | 初始数据 |
|------|-----------|---------|
| **本地 dev / MySQL** | `make migrate-apply` | `make seed`（仅首次） |
| **本地 dev / PostgreSQL** | `make migrate-apply`（配置 driver=postgres） | `APP_DATA_DB_USER_DRIVER=postgres APP_DATA_DB_USER_DSN='...' make seed` |
| **CI** | `make migrate-validate` | n/a |
| **生产部署** | 部署流水线在应用启动前跑 `go run ./cmd/dbmigrate apply` | 仅首次部署跑 `cmd/seed`，后续不再 |

**禁止应用启动时自动跑 schema migration**：多副本会抢着 DDL，且 schema 失败应阻塞部署而非运行时崩溃。

## 注意事项

- **不要手改已生成的 migration 文件**。改 schema 必须发新 migration，atlas 通过 `atlas.sum` 校验文件完整性。
- **不要在生成 migration 前先 apply**：必须先 diff 出 SQL，再 apply，否则版本号与 schema 漂移。
- **迁移方言与目标库来自配置**：读取 `APP_CONF` 或 `config/local.yml` 的 `data.db.user.driver` / `data.db.user.dsn`，并支持 `APP_DATA_DB_USER_DRIVER` / `APP_DATA_DB_USER_DSN` 覆盖。
- **默认 MySQL 不等于只支持 MySQL**：应用运行时可按 `data.db.user.driver` 切换驱动；migration 按方言隔离在 `db/migrations/mysql` 与 `db/migrations/postgres`，不能跨库混用。
- **casbin_rule 已纳入 atlas 管理**：由 `db/atlas/main.go` 中本地 `casbinRule` 镜像 struct（列与 gorm-adapter v3.CasbinRule 对齐 + 显式声明 `idx_casbin_rule` 唯一索引）描述。运行时通过 `repository.NewCasbinEnforcer` 调用 `gormadapter.TurnOffAutoMigrate` 关掉 adapter 的隐式建表与 `CREATE UNIQUE INDEX`。升级 gorm-adapter 大版本前须先比对其 `CasbinRule` 字段是否仍兼容此镜像，不兼容时同步刷新镜像与 migration。
- **针对已有业务表的旧库**：先对比现有 schema 与 `20260510075343_init.sql`，确认结构一致后用 `atlas migrate set 20260510075343 --env <env> --url <dsn>` 建立基线，**保留现有业务数据与策略数据**；切勿对旧库直接 `migrate-apply`，否则会因表已存在导致 `CREATE TABLE` 失败。
- **新增需要 atlas 管理的表**：必须在 `db/atlas/main.go:models()` 显式登记，否则 atlas 不会感知。
- **PostgreSQL 前置条件**：`atlas.hcl` 默认连接 `postgres://postgres:123456@127.0.0.1:5432/user?sslmode=disable` 与 `atlas_dev`，配置 `data.db.user.driver=postgres` 前需确保两个 database 已存在。
