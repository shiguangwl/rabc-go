# ==============================================================================
# 项目工程命令入口
# ------------------------------------------------------------------------------
# 速查：`make help` 列出全部 target
# 数据库 schema 详细文档：db/README.md
#
# 设计原则：优先收纳复合任务、非默认 flag 与项目约定入口；
# 标准 Go 命令仍可直接敲，Make target 负责降低团队记忆成本。
# ==============================================================================

# ------------------------------------------------------------------------------
# 变量
# ------------------------------------------------------------------------------
# 工具版本：Wire / mockgen / swag 通过 go.mod 的 tool 指令锁定（Go 1.24+），
# `go install tool` 一次性安装到 GOBIN，调用处用 `go tool <name>` 复用同一版本。
# nunu 只负责本地热加载，不参与构建产物。
NUNU_PKG := github.com/go-nunu/nunu

# 前端包管理器与 web/pnpm-lock.yaml 保持一致
PNPM ?= pnpm

.DEFAULT_GOAL := help

# ------------------------------------------------------------------------------
# 帮助
# ------------------------------------------------------------------------------

.PHONY: help
help:  ## 显示所有可用命令及说明
	@printf "\n用法: make \033[36m<target>\033[0m\n\n命令列表:\n"
	@awk 'BEGIN {FS = ":.*##"} \
		/^[a-zA-Z][a-zA-Z0-9_-]*:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' \
		$(MAKEFILE_LIST) | sort
	@echo ""

# ------------------------------------------------------------------------------
# 工具
# ------------------------------------------------------------------------------

.PHONY: init
init:  ## 安装 Go 工具链（版本由 go.mod 的 tool 指令锁定）
	go install tool
	go install $(NUNU_PKG)@latest
	@echo ""
	@echo ">>> 以下工具不在 Go module 范围，请按需手装："
	@echo "    Atlas         : brew install ariga/tap/atlas   (或 curl -sSf https://atlasgo.sh | sh)"
	@echo "    golangci-lint : brew install golangci-lint     (或 https://golangci-lint.run/usage/install/)"

# ------------------------------------------------------------------------------
# 代码生成
# ------------------------------------------------------------------------------

.PHONY: mock
mock:  ## 重生成 service/repository 的 mock（来源：源文件 //go:generate 注释）
	@mkdir -p test/mocks/service test/mocks/repository
	go generate ./internal/...

.PHONY: swag
swag:  ## 刷新 Swagger 文档到 ./docs/swagger
	go tool swag init -g cmd/server/main.go -o ./docs/swagger --parseDependency

# ------------------------------------------------------------------------------
# 质量门禁
# ------------------------------------------------------------------------------
# check 把提交前要走的步骤打包成一道门；test 保留为高频入口，
# 避免 README、CI 与开发者肌肉记忆分裂。

.PHONY: test
test:  ## 执行全量 Go race 测试
	go test -race ./...

.PHONY: check
check:  ## 提交前完整质量门禁（vet + lint + race test + migrate validate）
	go vet ./...
	golangci-lint run ./...
	$(MAKE) test
	$(MAKE) migrate-validate

# ------------------------------------------------------------------------------
# 构建
# ------------------------------------------------------------------------------
# build 走依赖触发，可用 make -j2 build 并行；web-build / server-build 不
# 写 ## 注释，help 不展示，但仍可单独调用（前端 hot reload 场景偶用）。

.PHONY: build
build: web-build server-build  ## 构建前端 + 后端二进制

.PHONY: web-build
web-build:
	cd web && $(PNPM) build

.PHONY: server-build
server-build:
	go build -ldflags="-s -w" -o ./bin/server ./cmd/server

.PHONY: clean
clean:  ## 清理本地构建产物（不删除已纳入版本控制的生成文件）
	rm -rf bin web/dist

# ------------------------------------------------------------------------------
# Atlas schema migration（详见 db/README.md）
# cmd/dbmigrate 默认读 APP_CONF 或 config/local.yml，并把目标库 DSN 传给 atlas。
# ------------------------------------------------------------------------------

.PHONY: push
push:  ## 【仅本地】Drizzle 风格：直接同步 GORM struct → DB，不生成 SQL 文件
	go run ./cmd/dbmigrate push

.PHONY: migrate-diff
migrate-diff:  ## 生成新 migration（用法：make migrate-diff name=add_xxx）
	@if [ -z "$(name)" ]; then \
		echo "ERROR: name 不能为空，例如 make migrate-diff name=add_user_email" >&2; \
		exit 1; \
	fi
	go run ./cmd/dbmigrate -name $(name) diff

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

# ------------------------------------------------------------------------------
# Seed 业务初始数据
# ------------------------------------------------------------------------------

.PHONY: seed
seed:  ## 写入 RBAC 初始数据（要求业务表为空）
	go run ./cmd/seed

.PHONY: seed-reset
seed-reset:  ## 【仅 local】清空 RBAC 业务表和策略后重新 seed
	go run ./cmd/seed -reset=true
