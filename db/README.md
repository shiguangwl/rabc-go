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

# 2. 数据库二选一（详见下方"DB 路径"）
```

## DB 路径

本仓库支持两条等价路径，按需选用；CLI/Make 命令在两条路径下完全一致。

### 路径 A — Docker Compose（默认推荐）

```bash
docker compose -f deploy/docker-compose/docker-compose.yml up -d --wait
# 容器自动监听 MySQL :3380、PostgreSQL :5432，并通过 initdb 脚本预建 atlas_dev 库。
make migrate-apply
```

`atlas.hcl` 默认 URL 即指向这套容器（MySQL 3380 / PostgreSQL 5432），无需任何环境变量。

### 路径 B — 本地原生 DB（不用 Docker）

适用：MySQL/PostgreSQL 已通过 brew/apt/yum 装在宿主、不想跑容器、或 CI 环境直连托管 DB。

```bash
# 1. 一次性手动建好 database
mysql -u root -p -e "CREATE DATABASE app; CREATE DATABASE atlas_dev;"
# 或 PostgreSQL：
# psql -U postgres -c "CREATE DATABASE app; CREATE DATABASE atlas_dev;"

# 2. 让运行时配置指向本地 DB（cmd/dbmigrate / cmd/server / cmd/seed 共用）
export APP_DATA_DB_USER_DRIVER=mysql
export APP_DATA_DB_USER_DSN='root:secret@tcp(127.0.0.1:3306)/app?charset=utf8mb4&parseTime=True&loc=Local'

# 3. 应用 migration、写 seed、启 server（不需要 docker）
make migrate-apply
go run ./cmd/seed
go run ./cmd/server     # 或 nunu run ./cmd/server 享 hot reload
```

PostgreSQL 路径同理，driver 改 `postgres`，DSN 使用 PostgreSQL URL。

> **不要用 `make bootstrap`**：它内置 `docker compose up`，是 Docker 路径的快捷入口。本地原生 DB 用户按上述 4 步手动跑即可——`bootstrap` 不是必经之路。

## 日常工作流

### Schema 变更（两条路径，按 schema 稳定度选）

**A 路径 — Drizzle 风格 push（早期 prototype，schema 频繁变）**

```bash
# 改 internal/model/*.go 后一条命令同步到本地 DB，不生成 SQL 文件
make push
```

`push` 仅在 DSN 指向 `localhost` / `127.0.0.1` / `::1` 时允许执行；
非本地 DSN 会被拒绝（防止误推 staging/prod）。**适用于 schema 还在快速摇摆的阶段**——
省去为每次试错都生成 SQL 的成本。一旦 schema 稳下来，准备 commit 前必须切到 B 路径生成版本化 SQL。

**B 路径 — 版本化（schema 已稳定、准备 commit/部署）**

```bash
# 1. 修改 internal/model/*.go 中的 GORM struct
# 2. 生成新 migration（替换 add_xxx 为本次变更的语义命名）
make migrate-diff name=add_xxx

# 3. 审计破坏性变更（drop col / add NOT NULL 等）
make migrate-lint

# 4. 应用到本地 DB
make migrate-apply

# 5. 提交 db/migrations/ 下生成的 SQL + atlas.sum
git add db/migrations/
```

> **铁律**：commit 前必须走 B 路径。`push` 不留痕迹，团队协作与生产部署都不能依赖它。
> `push` 与 `migrate-apply` 可以混用——push 改完的 schema 之后再走 diff，atlas 会生成
> "从上一个 migration 到当前 schema"的完整 SQL，等价于把多次 push 折叠成一次版本化提交。

## 场景对照表

按"我现在要做什么事"反查命令：

| 场景 | 命令 | 频率 |
|------|------|------|
| Schema 在快速试错（prototype 阶段） | `make push` | 每次改 model |
| Schema 已稳定，准备 commit | `make migrate-diff name=xxx` → `make migrate-lint` → `make migrate-apply` | 每次 schema 演化 |
| 危险操作（drop col / 加 NOT NULL） | 同上，但 lint 报警 → 手改 SQL → `go run ./cmd/dbmigrate hash` 重算 sum | schema 重构时 |
| 提交前完整门禁 | `make check`（含 vet + lint + race test + migrate validate） | 每次 commit/PR 前 |
| PR 关卡：检测破坏性变更 | `make migrate-lint base=origin/main` | CI 自动 |
| 部署到 staging / prod | `make migrate-apply`（带对应 DSN） | 每次发版 |
| 看待应用 migration / 当前状态 | `go run ./cmd/dbmigrate status` | debug 用 |
| 写 RBAC 初始数据（空库） | `go run ./cmd/seed` | 首次部署 |
| 仅 dev/local 清表重写 seed | `go run ./cmd/seed -reset=true` | 数据搞坏时 |

## 实战示例

### 示例 1 — 加普通字段（约 90% 场景）

```bash
# 1. 改 internal/model/admin.go：加字段如 Avatar string `gorm:"type:varchar(255)"`
# 2. 本地快速验证：直接同步 schema，立即跑应用测试
make push
# 3. 测试通过，准备提交
make migrate-diff name=add_admin_avatar    # 生成版本化 SQL；如果之前 push 过多次，atlas 会折叠
make migrate-lint                          # 通常无警告
make migrate-apply                         # 验证 SQL 在干净 DB 也能跑通
git add internal/model/admin.go db/migrations/
```

### 示例 2 — 危险变更（加 NOT NULL / 删列 / 改类型）

```bash
# 1. 改 model：例如把 Email 字段加 not null
make migrate-diff name=email_required
# 2. lint 会报警
make migrate-lint
#    → "adding NOT NULL constraint may fail if column contains NULL values"
# 3. 手改生成的 SQL：先 UPDATE 填默认值，再 ALTER
$EDITOR db/migrations/mysql/2026..._email_required.sql
# 4. 手改文件后 atlas.sum 失效，必须重算
go run ./cmd/dbmigrate hash
# 5. 再 lint 验证 + apply
make migrate-lint
make migrate-apply
```

### 示例 3 — CI 流水线（PR 检查）

```yaml
# .github/workflows/ci.yml 等价片段
- run: make check                              # 提交前完整门禁
- run: make migrate-lint base=origin/main      # 检测 PR 引入的破坏性 schema 变更
```

### 示例 4 — 部署到生产

```bash
# 必须在【应用启动前】跑（流水线步骤），不能让应用启动时自动跑。
# 配置优先读 APP_* 环境变量；未设置时回退读取 APP_CONF 指向的 YAML。
APP_CONF=config/prod.yml \
APP_DATA_DB_USER_DRIVER=mysql \
APP_DATA_DB_USER_DSN='<prod-dsn>' \
  make migrate-apply
# 应用启动后绝不再 migrate；多副本会抢着 DDL，schema 失败应阻塞部署而非运行时崩溃。
```

## 环境矩阵

| 环境 | schema 演进 | 初始数据 |
|------|-----------|---------|
| **本地 dev / MySQL** | `make push` 或 `make migrate-apply` | `go run ./cmd/seed`（仅首次） |
| **本地 dev / PostgreSQL** | 同上（先设 `APP_DATA_DB_USER_DRIVER=postgres`） | `APP_DATA_DB_USER_DRIVER=postgres ... go run ./cmd/seed` |
| **CI** | `make check`（含 migrate validate）+ `make migrate-lint base=origin/main` | n/a |
| **生产部署** | 部署流水线在应用启动前跑 `make migrate-apply`（prod DSN 来自环境变量或 `config/prod.yml`） | 仅首次部署跑 `go run ./cmd/seed`，后续不再 |

**禁止应用启动时自动跑 schema migration**：多副本会抢着 DDL，且 schema 失败应阻塞部署而非运行时崩溃。

## 注意事项

- **不要手改已生成的 migration 文件**。改 schema 必须发新 migration，atlas 通过 `atlas.sum` 校验文件完整性。
- **不要在生成 migration 前先 apply**：必须先 diff 出 SQL，再 apply，否则版本号与 schema 漂移。
- **迁移方言与目标库来自配置**：读取 `APP_CONF` 或 `config/local.yml` 的 `data.db.user.driver` / `data.db.user.dsn`，并支持 `APP_DATA_DB_USER_DRIVER` / `APP_DATA_DB_USER_DSN` 覆盖。
- **默认 MySQL 不等于只支持 MySQL**：应用运行时可按 `data.db.user.driver` 切换驱动；migration 按方言隔离在 `db/migrations/mysql` 与 `db/migrations/postgres`，不能跨库混用。
- **casbin_rule 已纳入 atlas 管理**：由 `db/atlas/main.go` 中本地 `casbinRule` 镜像 struct（列与 gorm-adapter v3.CasbinRule 对齐 + 显式声明 `idx_casbin_rule` 唯一索引）描述。运行时通过 `repository.NewCasbinEnforcer` 调用 `gormadapter.TurnOffAutoMigrate` 关掉 adapter 的隐式建表与 `CREATE UNIQUE INDEX`。升级 gorm-adapter 大版本前须先比对其 `CasbinRule` 字段是否仍兼容此镜像，不兼容时同步刷新镜像与 migration。
- **新增需要 atlas 管理的表**：必须在 `db/atlas/main.go:models()` 显式登记，否则 atlas 不会感知。
- **PostgreSQL 前置条件**：`atlas.hcl` 默认连接 `postgres://postgres:123456@127.0.0.1:5432/user?sslmode=disable` 与 `atlas_dev`，配置 `data.db.user.driver=postgres` 前需确保两个 database 已存在。
