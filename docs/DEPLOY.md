# 部署说明

本文档说明当前仓库的生产镜像与 SSH 远程 Docker Compose 部署流程。应用启动只启动 HTTP 服务，不会自动执行 migration 或 seed。

## 前置条件

本地构建机需要：

- Docker
- SSH / SCP
- 可选 `pv`：仅用于显示镜像传输进度
- 可选 `sshpass`：仅在使用 `SSH_PASSWORD` 密码登录时需要

远端服务器需要：

- Docker 与 Docker Compose v2（`docker compose`）
- 能访问生产 MySQL 或 PostgreSQL
- 能访问生产 Redis

数据库 schema 必须在应用启动前执行：

```bash
APP_CONF=config/prod.yml \
APP_DATA_DB_USER_DRIVER=mysql \
APP_DATA_DB_USER_DSN='root:password@tcp(mysql:3306)/user?charset=utf8mb4&parseTime=True&loc=Local' \
make migrate-apply
```

首次部署还需要执行 seed。非 `env: local` 必须设置 `APP_SEED_INITIAL_PASSWORD`，否则 seed 会拒绝写入弱默认密码：

```bash
APP_CONF=config/prod.yml \
APP_DATA_DB_USER_DRIVER=mysql \
APP_DATA_DB_USER_DSN='root:password@tcp(mysql:3306)/user?charset=utf8mb4&parseTime=True&loc=Local' \
APP_DATA_REDIS_ADDR='redis:6379' \
APP_SECURITY_JWT_KEY='change-me-to-a-long-random-secret' \
APP_SEED_INITIAL_PASSWORD='change-me-on-first-login' \
go run ./cmd/seed
```

`cmd/seed` 默认要求 RBAC 表为空；`-reset=true` 仅允许 `env: local` 且 DSN 指向本机。

## 构建镜像

推荐使用 Makefile：

```bash
make docker-build IMAGE=rabc-go TAG=latest
```

等价 Docker 命令：

```bash
DOCKER_BUILDKIT=1 docker build -t rabc-go:latest .
```

镜像构建会：

1. 使用 `node:22-alpine` 构建 `web/dist`
2. 使用 `golang:1.25-alpine` 编译 `cmd/server`
3. 复制 `config/prod.yml` 到 `/app/config/prod.yml`
4. 以非 root 用户运行 `/app/server`

容器内默认 `APP_CONF=/app/config/prod.yml`，HTTP 监听 `0.0.0.0:8000`。

## 远程部署

部署脚本默认读取 `scripts/.deploy.env`，并上传 `deploy/docker-compose.yml` 到远端部署目录。脚本不依赖镜像仓库：它会在本地构建镜像，通过 `docker save | gzip | ssh docker load` 传到远端，再执行 `docker compose up -d --pull never --remove-orphans`。

```bash
cp scripts/.deploy.env.example scripts/.deploy.env
cp deploy/.env.production.example deploy/.env.production

# 填写真实 SSH、DSN、Redis、JWT 等配置后先预演
scripts/deploy.sh --dry-run
scripts/deploy.sh
```

`scripts/.deploy.env` 至少需要提供：

```bash
SSH_HOST=example.com
SSH_USER=deploy
REMOTE_DIR=/opt/rabc-go
IMAGE_NAME=rabc-go
ENV_FILES=deploy/.env.production
```

生产环境变量文件由 `ENV_FILES` 指定。脚本会把文件上传为远端部署目录下的 `.env.production`，供 `deploy/docker-compose.yml` 的 `env_file` 读取。

`deploy/.env.production` 至少应提供：

```bash
APP_SECURITY_JWT_KEY=change-me-to-a-long-random-secret
APP_DATA_DB_USER_DRIVER=mysql
APP_DATA_DB_USER_DSN=root:password@tcp(mysql:3306)/user?charset=utf8mb4&parseTime=True&loc=Local
APP_DATA_REDIS_ADDR=redis:6379
APP_DATA_REDIS_PASSWORD=
APP_DATA_REDIS_DB=0
```

`prod` 启动期强制校验 `APP_SECURITY_JWT_KEY` 与 `APP_DATA_DB_USER_DSN` 非空；Redis 会在启动期 `PING`，不可用时进程直接失败。

## 常用配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `COMPOSE_FILE` | `deploy/docker-compose.yml` | 本地 Compose 模板，上传后固定命名为远端 `docker-compose.yml` |
| `ENV_FILES` | `deploy/.env.production` | 本地环境变量文件，上传后固定命名为远端 `.env.production` |
| `APP_PORT` | `8000` | 远端宿主机暴露端口，映射到容器 `8000` |
| `CONTAINER_NAME` | `rabc-go` | 容器名称 |
| `IMAGE_NAME` | 无默认值 | 镜像名称，必填 |
| `IMAGE_TAG` | 当前时间戳 | 本次部署镜像标签 |
| `IMAGE_PLATFORM` | `linux/amd64` | `docker build --platform` 参数 |
| `IMAGE_RETENTION_COUNT` | `5` | 远端保留的历史镜像数量 |
| `COMPOSE_PROJECT_NAME` | 空 | 非空时追加 `docker compose -p` |
| `DOCKER_NETWORK` | 空 | 非空时生成 `docker-compose.network.yml` 并接入外部网络 |
| `COMPOSE_VARS` | `IMAGE_NAME IMAGE_TAG APP_PORT CONTAINER_NAME` | 传给远端 Compose 命令的变量白名单 |

## 回滚

脚本部署成功后会在远端写入 `.last_deploy_tag`。回滚会读取该标签，确认远端镜像存在后重新执行 Compose：

```bash
scripts/deploy.sh --rollback
```

回滚只切换应用镜像，不回滚数据库 migration。执行破坏性 migration 前需要单独设计数据库回滚方案。

## 部署边界

- 应用启动不执行 `AutoMigrate`；Casbin adapter 的自动建表也已关闭。
- `scripts/deploy.sh` 不执行 `make migrate-apply` 和 `go run ./cmd/seed`。
- `deploy/docker-compose.yml` 只包含 `app` 服务；MySQL、PostgreSQL、Redis 需要外部提供。
- 健康检查访问 `http://127.0.0.1:8000/`，实际由内嵌前端首页响应。
