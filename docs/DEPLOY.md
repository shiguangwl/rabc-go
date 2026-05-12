# 部署说明

## 构建镜像

```bash
docker build -t rabc-go:latest .
```

## 远程部署

部署脚本默认读取 `scripts/.deploy.env`，并使用 `deploy/remote/docker-compose.yml`。

```bash
cp scripts/.deploy.env.example scripts/.deploy.env
cp deploy/remote/.env.production.example deploy/remote/.env.production
scripts/deploy.sh --dry-run
scripts/deploy.sh
```

`scripts/.deploy.env` 至少需要提供：

```bash
SSH_HOST=example.com
SSH_USER=deploy
REMOTE_DIR=/opt/rabc-go
IMAGE_NAME=rabc-go
ENV_FILES=deploy/remote/.env.production
```

生产环境变量文件由 `ENV_FILES` 指定，脚本会上传为远端部署目录下的
`.env.production`，供 Docker Compose 读取。

## 常用配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `COMPOSE_FILE` | `deploy/remote/docker-compose.yml` | 本地 Compose 模板 |
| `APP_PORT` | `8000` | 远端宿主机暴露端口 |
| `CONTAINER_NAME` | `rabc-go` | 容器名称 |
| `IMAGE_TAG` | 当前时间戳 | 本次部署镜像标签 |
