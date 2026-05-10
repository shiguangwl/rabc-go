# ==============================================================================
# 工作流速查
# ------------------------------------------------------------------------------
#   首次准备   make init             装 wire / mockgen / swag / nunu（atlas 另装）
#   日常开发   make bootstrap        起依赖容器 + atlas apply + seed + nunu 启动 server
#   接口变更   make mock             重生成 service/repository 的 mock
#   注解变更   make swag             刷新 Swagger 文档到 ./docs
#   提交前    make test             跑全量 Go race 测试
#   发布构建   make build            前端 npm 打包 + 后端瘦身二进制 ./bin/server
#   镜像验证   make docker           构建 cmd/task 镜像并试跑（定时任务用，非常驻）
#
# 数据库 schema（详见 db/README.md，cmd/dbmigrate 读取 APP_CONF/config/local.yml）：
#   make migrate-diff name=xxx       从 GORM struct 生成新版本 SQL migration
#   make migrate-apply               应用待执行 migration 到本地 DB
#   make migrate-status              查看 migration 状态
#   make migrate-hash                重新计算 atlas.sum（手改文件后必跑）
#   make migrate-validate            校验 migration 目录完整性（CI 用）
#
# 业务初始数据（首次部署或 reset 后跑）：
#   make seed                        写入 RBAC 初始数据（要求空表）
#   make seed-reset                  TRUNCATE + 重写（仅 dev/local）
# ==============================================================================

# 安装开发工具链
.PHONY: init
init:
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/golang/mock/mockgen@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/go-nunu/nunu@latest
	@echo ""
	@echo ">>> Atlas CLI 不在 go install 范围，请按需手装："
	@echo "    macOS : brew install ariga/tap/atlas"
	@echo "    其它  : curl -sSf https://atlasgo.sh | sh"

# 一键启动本地开发环境：依赖容器 → atlas 同步 schema → 写入初始数据 → 启 server
.PHONY: bootstrap
bootstrap:
	cd ./deploy/docker-compose && docker compose up -d --wait && cd ../../
	$(MAKE) migrate-apply
	go run ./cmd/seed
	nunu run ./cmd/server

# 生成接口 mock
.PHONY: mock
mock:
	mockgen -source=internal/service/user.go -destination test/mocks/service/user.go
	mockgen -source=internal/repository/user.go -destination test/mocks/repository/user.go
	mockgen -source=internal/repository/repository.go -destination test/mocks/repository/repository.go

# 运行全量 Go 测试
.PHONY: test
test:
	go test -race ./...

# 构建前后端
.PHONY: build
build: web-build server-build

.PHONY: web-build
web-build:
	cd web && npm run build

.PHONY: server-build
server-build:
	go build -ldflags="-s -w" -o ./bin/server ./cmd/server

# 构建并运行 task 镜像
.PHONY: docker
docker:
	docker build -f deploy/build/Dockerfile --build-arg APP_RELATIVE_PATH=./cmd/task -t 1.1.1.1:5000/demo-task:v1 .
	docker run --rm -i 1.1.1.1:5000/demo-task:v1

# 生成 Swagger 文档
.PHONY: swag
swag:
	swag init  -g cmd/server/main.go -o ./docs --parseDependency

# ------------------------------------------------------------------------------
# Atlas schema migration
# ------------------------------------------------------------------------------
# 生成新 migration：用法 make migrate-diff name=add_xxx
.PHONY: migrate-diff
migrate-diff:
	@if [ -z "$(name)" ]; then echo "ERROR: name 不能为空，例如 make migrate-diff name=add_user_email"; exit 1; fi
	go run ./cmd/dbmigrate -name $(name) diff

# 应用待执行 migration
.PHONY: migrate-apply
migrate-apply:
	go run ./cmd/dbmigrate apply

# 查看 migration 状态
.PHONY: migrate-status
migrate-status:
	go run ./cmd/dbmigrate status

# 重新计算 atlas.sum（仅手改/合并冲突后才需要）
.PHONY: migrate-hash
migrate-hash:
	go run ./cmd/dbmigrate hash

# 校验 migration 目录完整性（CI 用）
.PHONY: migrate-validate
migrate-validate:
	go run ./cmd/dbmigrate validate

# ------------------------------------------------------------------------------
# Seed 业务初始数据
# ------------------------------------------------------------------------------
.PHONY: seed
seed:
	go run ./cmd/seed

# 仅 dev/local：TRUNCATE RBAC 业务表 + 重新 seed
.PHONY: seed-reset
seed-reset:
	go run ./cmd/seed -reset=true
