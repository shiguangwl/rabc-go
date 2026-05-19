---
name: schema-change
description: 改 GORM struct、加表、加索引、调字段类型时使用——按 Atlas 版本化流程生成、审计、落库 migration。仅查数据、改业务逻辑或调 SQL 性能不走本 skill。
---

# 表结构变更

应用启动**不**做 AutoMigrate，所有 schema 走 Atlas。GORM struct 是 schema 的唯一真相，DB 始终被驱动同步。

## 流程

### 1. 改 GORM struct

位置：`internal/model/<entity>.go`。在 tag 上写完整 `column / type / index / comment`，缺一项就会被 Atlas 当作变更生成空 diff 或冗余 SQL。

### 2. 新表强制：登记到 atlas loader

打开 `db/atlas/main.go`，把新 model 加入 `models()` 列表。**漏登记** → Atlas 看不到该 struct → migration 不含建表 SQL，且不会报错。

```go
func models() []any {
    return []any{
        &model.AdminUser{},
        &model.Menu{},
        // ... 在此追加新 model
    }
}
```

### 3. 选路径

| 阶段 | 命令 | 说明 |
|------|------|------|
| Prototype，schema 频繁变 | `make push` | `go run ./cmd/dbmigrate push`；不生成 SQL，仅本地 DSN（localhost/127.0.0.1/::1）允许 |
| 准备 commit / 发布 | 走下方 4–6 步 | **commit 前必须走版本化路径**；CI 会用 `migrate-validate` 兜底 |

### 4. 生成版本化 SQL

```bash
make migrate-diff name=<语义命名>   # 例：add_user_phone_index
```

未指定 `name` 时回退为 `YYYYMMDD_HHMMSS` 时间戳。命名最好用动词开头、明确意图（`add_xxx` / `drop_xxx` / `alter_xxx_type`）。

### 5. 审计 + 应用

```bash
make migrate-lint base=origin/main   # 检测 drop col / add NOT NULL 等破坏性变更
make migrate-apply                   # go run ./cmd/dbmigrate apply
```

破坏性 lint 报警若属预期（如确实要 drop 列），**手改 SQL**（不是改 struct 再 diff）后跑 `make migrate-hash` 重算 `atlas.sum`，再回到 `make migrate-validate` 复核。

### 6. 提交

```bash
git add internal/model/ db/atlas/main.go db/migrations/
```

`atlas.sum` 必须随 SQL 一同提交，否则 CI `migrate-validate` 失败。

## 关键边界

| 场景 | 注意 |
|------|------|
| 多方言 | migration 隔离在 `db/migrations/{mysql,postgres}/`，**不可**跨库混用；新增方言要在 `cmd/dbmigrate` 注册 |
| Casbin 表 | `gorm-adapter` 升级时同步比对 `db/atlas/main.go` 中 `casbinRule` 镜像，避免 adapter 内部表结构漂移导致策略丢失 |
| 部署 | 流水线在 server 启动**前**跑 `make migrate-apply`，应用本身不做 DDL；本地 `air` 启动也不会自动迁移 |
| 回滚 | Atlas 不支持自动 down，需要手写补偿 migration 反向打补丁 |

## 严禁

- 在生产 DSN 跑 `make push`：会绕过版本化、`atlas.sum` 不更新。
- 手动 `ALTER TABLE` 改库：让 GORM struct 与实际 schema 漂移，下次 `make migrate-diff` 会生成"反向 SQL"。
- 删除已合并到主干的 migration 文件：`atlas.sum` 链断裂，全员 `migrate-validate` 失败。
