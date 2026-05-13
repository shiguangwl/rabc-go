# Makefile 只封装项目约定流程。

NUNU_PKG := github.com/go-nunu/nunu
GOLANGCI_LINT := go tool -modfile=tools/lint/go.mod golangci-lint

PNPM ?= pnpm
DOCKER ?= docker
DOCKER_BUILDKIT ?= 1
IMAGE ?= rabc-go
TAG ?= latest

.DEFAULT_GOAL := help

.PHONY: help
help:  ## 显示所有可用命令及说明
	@printf "\n用法: make \033[36m<target>\033[0m\n\n命令列表:\n"
	@awk 'BEGIN {FS = ":.*##"} \
		/^[a-zA-Z][a-zA-Z0-9_-]*:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' \
		$(MAKEFILE_LIST) | sort
	@echo ""

.PHONY: init
init:  ## 安装本地开发工具
	go install tool
	go install -modfile=tools/lint/go.mod tool
	go install $(NUNU_PKG)@latest
	@echo ""
	@echo ">>> 以下工具不在 Go module 范围，请按需手装："
	@echo "    Atlas         : brew install ariga/tap/atlas   (或 curl -sSf https://atlasgo.sh | sh)"

.PHONY: fmt
fmt:  ## 格式化 Go 代码
	$(GOLANGCI_LINT) fmt ./...

.PHONY: lint-fast
lint-fast:  ## 执行编辑器同级快速 Go lint
	$(GOLANGCI_LINT) run --path-mode=abs --fast-only ./...

.PHONY: lint
lint:  ## 执行新增代码 Go lint
	$(GOLANGCI_LINT) run --path-mode=abs --new ./...

.PHONY: lint-full
lint-full:  ## 执行全量 Go lint 债务扫描
	$(GOLANGCI_LINT) run --path-mode=abs ./...

.PHONY: mock
mock:  ## 重生成 service/repository 的 mock（来源：源文件 //go:generate 注释）
	@mkdir -p test/mocks/service test/mocks/repository
	go generate ./internal/...

.PHONY: swag
swag:  ## 刷新 Swagger 文档到 ./docs/swagger
	go tool swag init -g cmd/server/main.go -o ./docs/swagger --parseDependency

.PHONY: test
test:  ## 执行全量 Go race 测试
	go test -race ./...

.PHONY: check
check:  ## 提交前完整质量门禁（fmt + vet + lint + race test + migrate validate）
	$(MAKE) fmt
	go vet ./...
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) migrate-validate

.PHONY: build
build: server-build  ## 构建前端 + 后端二进制

.PHONY: web-build
web-build:
	cd web && $(PNPM) build

.PHONY: server-build
server-build: web-build
	go build -ldflags="-s -w" -o ./bin/server ./cmd/server

.PHONY: server-build-only
server-build-only:  ## 仅编译后端，复用现有 web/dist 内嵌资源
	go build -ldflags="-s -w" -o ./bin/server ./cmd/server

.PHONY: docker-build
docker-build:  ## 构建生产镜像；可覆盖 IMAGE 和 TAG
	DOCKER_BUILDKIT=$(DOCKER_BUILDKIT) $(DOCKER) build -t $(IMAGE):$(TAG) .

.PHONY: clean
clean:  ## 清理本地构建产物（不删除已纳入版本控制的生成文件）
	rm -rf bin web/dist

.PHONY: push
push:  ## 【仅本地】直接同步 GORM struct → DB，不生成 SQL 文件
	go run ./cmd/dbmigrate push

.PHONY: migrate-diff
migrate-diff:  ## 生成新 migration（用法：make migrate-diff [name=add_xxx]，未提供 name 则使用日期时间）
	@if [ -z "$(name)" ]; then \
		name=$$(date +%Y%m%d_%H%M%S); \
		echo "INFO: 未指定 name，使用默认日期时间文件名: $$name"; \
	fi; \
	go run ./cmd/dbmigrate -name "$$name" diff

.PHONY: migrate-apply
migrate-apply:  ## 应用待执行 migration 到目标 DB
	go run ./cmd/dbmigrate apply

.PHONY: migrate-status
migrate-status:  ## 查看 migration 状态
	go run ./cmd/dbmigrate status

.PHONY: migrate-hash
migrate-hash:  ## 重新计算 atlas.sum（手改 SQL 或解决冲突后使用）
	go run ./cmd/dbmigrate hash

.PHONY: migrate-validate
migrate-validate:  ## 校验 migration 目录完整性（CI 用）
	go run ./cmd/dbmigrate validate

.PHONY: migrate-lint
migrate-lint:  ## 检测破坏性变更（drop col / add NOT NULL 等）；可选 base=origin/main
	go run ./cmd/dbmigrate $(if $(base),-base $(base),) lint

.PHONY: seed
seed:  ## 写入 RBAC 初始数据（要求业务表为空）
	go run ./cmd/seed

.PHONY: seed-reset
seed-reset:  ## 【仅 local】清空 RBAC 业务表和策略后重新 seed
	go run ./cmd/seed -reset=true
