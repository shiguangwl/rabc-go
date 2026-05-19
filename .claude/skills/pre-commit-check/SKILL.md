---
name: pre-commit-check
description: 准备 git commit / 推送 / 合 PR 前使用——按 make check 跑完整质量门禁并给修复方向。临时单跑 lint/test 用对应 make 目标，不走本 skill。
---

# 提交前质量门禁

## 执行

```bash
make check
```

按顺序运行：

| # | 阶段 | 等价命令 | 关注点 |
|---|------|----------|--------|
| 1 | `make fmt` | `golangci-lint fmt ./...` | 含 gofumpt/goimports；会**直接改写**文件 |
| 2 | `go vet ./...` | 同名 | 影子变量、未使用结果、不正确 printf 等 |
| 3 | `make lint` | `golangci-lint run --path-mode=abs --new ./...` | 只 lint 新增/变更代码；全量债务用 `make lint-full` |
| 4 | `make test` | `go test -race ./...` | 必须开 race，禁止用 `-skip` 跳过 |
| 5 | `make migrate-validate` | `go run ./cmd/dbmigrate validate` | 校验 `atlas.sum` 与 SQL 完整性 |

## 失败处置

| 阶段 | 常见原因 | 处置 |
|------|---------|------|
| `make fmt` | 代码未格式化 | `make fmt` 已自动改写，`git add` 后重跑 |
| `go vet` | 可疑写法 | 按提示改代码；**禁止** `//nolint` 绕过 vet |
| `make lint` | 新增代码触发规则 | 按 `.golangci.yml` 修复；想看全量债务跑 `make lint-full` |
| `make test` | race / 业务回归 | 复现失败用例 → 修根因；**禁止** `-skip` 或删测试 |
| `make migrate-validate` | `atlas.sum` 与 SQL 不一致 | 改完 SQL 后 `make migrate-hash`，再 `make migrate-validate` 确认 |

## 按需追加

| 触发条件 | 额外命令 | 作用 |
|---------|---------|------|
| 改了 handler swagger 注释 / DTO | `make swag` | 同步 `docs/swagger`，避免前后端契约漂移 |
| 本次包含 schema 变更 | `make migrate-lint base=origin/main` | 检测 drop col / add NOT NULL 等破坏性变更 |
| 改了 Wire 装配 | `make wire` | 重生成 `wire_gen.go`，CI 会比对 |
| 仅想本地快跑预检 | `make lint-fast` | 编辑器同级 lint，比 `make lint` 略快 |

## 严禁

- `git commit --no-verify` 跳过 hook。
- 用 `git commit --amend` / `git rebase` 修历史提交——项目规范要求新建提交（fix-up 也另起一条）。
- 用 `//nolint` 整片绕过 lint；个别豁免须加注释说明原因。
