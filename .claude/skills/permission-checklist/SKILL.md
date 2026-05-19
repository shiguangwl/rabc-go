---
name: permission-checklist
description: 新增后端接口或菜单后使用——输出 API/菜单/角色/账号 5 步闭环清单，避免普通账号被 Casbin 拦截 403。脱离当前 RBAC 实现的设计讨论不走本 skill。
---

# 权限上线 5 步清单

接口写完只是第一步——不走完下列流程，普通账号在 `AuthMiddleware` 处被 Casbin 拦截 403。超管 ID=1 自动放行，**不能**作为联调验证。

## 5 步闭环

| # | 步骤 | 入口 | 关键字段 / 注意 |
|---|------|------|----------------|
| 1 | 后端实现 | 代码 | `internal/server/http.go` 把 handler 挂到 `strict` 分组（默认带 `StrictAuth + AuthMiddleware`） |
| 2 | 登记 API | 后台「接口管理」 | `group / name / path / method` 必须与代码完全一致；`path` 含通配时按 Casbin 规则录入 |
| 3 | 登记菜单 | 后台「菜单管理」 | 若新增页面入口；`path` 作为菜单级权限身份 |
| 4 | 角色授权 | 后台「角色管理」 | 给目标角色勾选刚登记的 API / 菜单（按需多选） |
| 5 | 账号绑定 | 后台「管理员管理」 | 把目标账号绑定到该角色，登录后立即生效（无需重启） |

## Casbin 策略形态（参考）

```text
p, <role_sid>, api:/v1/admin/users, GET
p, <role_sid>, menu:/access/admin, read
g, <user_id>, <role_sid>
```

> `role_sid` 是角色的 stable ID（非 name），由 `role` 子域生成；菜单/API 改 `path` 会改变这条策略的资源标识。

## 变更副作用

| 变更类型 | 是否需要手动清理 | 说明 |
|---------|----------------|------|
| 修改菜单 `path` / 删菜单 | 否 | `menu.MenuUpdateAtomic` / `MenuDeleteAtomic` 已同事务清旧策略 |
| 修改接口 `path` / `method` | **是** | 接口管理里把旧条目同步改掉或删除，否则旧授权挂在旧路径上不生效 |
| 删角色 | **是** | 删角色前先在「角色管理」解除账号绑定，避免遗留 `g` 策略 |
| 仅改 handler 内部实现 | 否 | 路由签名未变，无需动 RBAC 配置 |

## 输出动作

向用户打印 5 步清单 + 上方"变更副作用"表；如果用户已经描述了具体改动（改 path / 删菜单 / 加接口），用一句话指出本次需要走哪几步、能跳过哪几步。
