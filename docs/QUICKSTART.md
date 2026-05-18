# rabc-go 快速入门

> 面向 Go 生态新手的项目入门文档。读完这份文档你能：理解项目分层、掌握所有依赖框架的用法、知道怎么开发新业务功能。

---

## 一、项目定位

基于 [go-nunu](https://github.com/go-nunu/nunu) 脚手架的 RBAC 后台模板，提供：

- **Gin** Web 框架 + **GORM** ORM + **Casbin** 权限引擎
- **Redis** 管理 refresh token、会话索引与吊销
- **Wire** 编译期依赖注入
- 三个独立可执行入口：HTTP 服务、初始数据 Seed、数据库迁移
- 默认账号仅 `env: local` 使用：`admin/123456`（超管）、`user/123456`（运营）

---

## 二、项目结构

```
rabc-go/
├── cmd/                    # 程序入口（每个生成一个二进制）
│   ├── server/             # HTTP 服务主入口
│   ├── seed/               # 初始业务数据写入（不建表）
│   └── dbmigrate/          # Atlas migration 命令封装
│
├── api/apiv1/              # 对外 DTO + 错误码
│   ├── v1.go               # 统一响应封装、错误码注册表
│   ├── errors.go           # 业务错误码常量
│   ├── admin.go            # 后台管理请求/响应结构体
│   └── auth.go             # refresh / logout 请求响应结构体
│
├── internal/               # 私有业务代码（外部模块禁止 import）
│   ├── admin/rbac/         # RBAC 业务域（vertical slice）
│   │   ├── api/            # repository + service + handler
│   │   ├── menu/           # repository + service + handler
│   │   ├── permission/     # repository + service + handler
│   │   ├── casbinkit/      # Casbin enforcer / 互斥锁 / 校验
│   │   ├── role/           # repository + service + handler
│   │   └── user/           # repository + service + handler + types
│   ├── auth/               # 双 Token 鉴权与会话管理
│   │   └── lua/            # Redis 原子脚本
│   ├── model/              # 跨子域共享 GORM 实体 + 业务常量
│   ├── middleware/         # Gin 中间件（CORS、JWT、RBAC、日志、签名）
│   └── server/             # Server 接口实现（HTTP/Seed）
│
├── pkg/                    # 可复用工具包（理论上可被其他项目引用）
│   ├── app/                # 应用容器：聚合多个 Server 优雅启停
│   ├── config/             # Viper 封装
│   ├── jwt/                # JWT 签发/解析 + UserIDFromCtx
│   ├── log/                # zap + lumberjack 日志
│   ├── server/             # 通用 HTTP/gRPC Server 壳
│   ├── sid/                # Sonyflake 分布式 ID
│   └── zapgorm2/           # GORM SQL 日志桥接到 zap
│
├── config/                 # YAML 配置（local.yml / prod.yml）
├── db/                     # Atlas schema、migration 与 seed 静态数据
│   ├── atlas/              # GORM → Atlas schema 桥接
│   ├── migrations/         # MySQL/PostgreSQL 版本化 SQL
│   └── seed/               # 菜单等初始数据
├── docs/                   # 项目文档
│   └── swagger/            # Swagger 生成产物（勿手改）
├── deploy/                 # 远程部署配置
├── storage/                # 运行时日志等本地文件
├── web/                    # Vue3 前端，生产构建通过 embed_frontend 标签内嵌 dist
└── Makefile                # help/init/build/test/migrate/seed
```

### 三层调用链

```
HTTP 请求
  ↓
[middleware] 日志 / Recovery；受保护路由额外经过 JWT → Casbin RBAC
  ↓
[handler]   解析参数（ctx.ShouldBind）+ 调 service + 返响应
  ↓
[service]   业务规则（编排多个 repo + 处理领域逻辑）
  ↓
[repository] GORM 查库 / Casbin 查权限
  ↓
数据库
```

---

## 三、技术栈速览

| 类别      | 框架                       | 作用                          | 项目里的位置                                                  |
| --------- | -------------------------- | ----------------------------- | ------------------------------------------------------------- |
| Web 框架  | Gin                        | HTTP 路由、参数绑定、中间件   | `internal/auth/handler.go`、`internal/admin/rbac/*/handler.go`、`internal/middleware` |
| ORM       | GORM v2                    | 多驱动数据库操作              | `internal/admin/rbac/*/repository.go`                                         |
| 权限      | Casbin                     | RBAC 策略引擎                 | `internal/middleware/rbac.go`、`internal/admin/rbac/casbinkit/` |
| 依赖注入  | Wire                       | 编译期生成装配代码            | `cmd/*/wire/`                                                 |
| 配置      | Viper                      | YAML 配置加载                 | `pkg/config`                                                  |
| 日志      | zap + lumberjack           | 结构化日志 + 滚动切割         | `pkg/log`                                                     |
| 认证      | golang-jwt v5              | JWT 签发解析                  | `pkg/jwt`                                                     |
| 会话      | redis/go-redis v9          | refresh token、会话索引、吊销 | `internal/auth/`     |
| 密码      | golang.org/x/crypto/bcrypt | 密码哈希                      | `internal/auth/`、`internal/server/seed.go`                    |
| 分布式 ID | sonyflake                  | 雪花 ID + Base62              | `pkg/sid`                                                     |
| API 文档  | swag                       | 注解生成 Swagger              | `docs/swagger/`（自动生成）                                   |
| 校验      | go-playground/validator    | binding tag 校验（Gin 内置）  | `api/apiv1/admin.go`、`api/apiv1/auth.go`                     |
| 工具库    | duke-git/lancet            | 字符串/MD5/UUID 等工具        | 散见各处                                                      |

---

## 四、核心框架详解

### 4.1 Wire — 依赖注入（必须先理解）

#### 它解决什么问题？

任何分层项目的 main 函数都要按拓扑顺序 new 一堆对象：

```go
// 假设没有 Wire，你得自己写：
db, cleanupDB, err := platform.NewDB(conf, logger)
defer cleanupDB()
enforcer, cleanupCasbin, err := platform.NewCasbinEnforcer(conf, logger, db)
defer cleanupCasbin()
redisClient := platform.NewRedis(conf)
rbacMu := casbinkit.NewRBACMu()
userRepo := user.NewRepo(db, enforcer, logger, rbacMu)
authRepo := auth.NewRepository(redisClient)
authService := auth.NewService(logger, jwtUtil, authRepo, userRepo, auth.LoadConfig(conf, logger))
userService := user.NewService(logger, userRepo, authService)
authHandler := auth.NewHandler(authService)
userHandler := user.NewHandler(userService)
roleHandler := role.NewHandler(role.NewService(role.NewRepo(db, enforcer, logger, rbacMu)))
httpServer := server.NewHTTPServer(logger, conf, jwtUtil, enforcer, authHandler, userHandler, roleHandler, ...)
// ...继续 20 行
```

**痛点**：顺序敏感、改一个依赖牵全身，`cmd/server` 与 `cmd/seed` 都要装配一遍。

#### Wire 怎么解决

你只写"零件清单"（`cmd/server/wire/wire.go`），Wire 工具帮你生成上面那段装配代码到 `wire_gen.go`。

**清单（你写）**：

```go
//go:build wireinject              // build tag：只在跑 wire 工具时编译

var platformSet = wire.NewSet(     // 基础设施构造函数
    platform.NewDB,
    platform.NewCasbinEnforcer,
    platform.NewRedis,
)

var rbacSet = wire.NewSet(         // RBAC vertical slice 构造函数
    casbinkit.NewRBACMu,
    user.NewRepo,
    user.NewService,
    user.NewHandler,
    role.NewRepo,
    role.NewService,
    role.NewHandler,
    menu.NewRepo,
    menu.NewService,
    menu.NewHandler,
    rbacapi.NewRepo,
    rbacapi.NewService,
    rbacapi.NewHandler,
    permission.NewRepo,
    permission.NewService,
    permission.NewHandler,
    wire.Bind(new(menu.PermissionReader), new(*permission.Repo)),
    wire.Bind(new(auth.UserLookup), new(*user.Repo)),
)

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
    panic(wire.Build(platformSet, rbacSet, authSet, serverSet, jwt.NewJwt, newApp))
}
```

**生成产物（自动）**：

```go
//go:build !wireinject              // 取反，正常编译只走这个文件

func NewWire(v *viper.Viper, l *log.Logger) (*app.App, func(), error) {
    db, cleanup, err := platform.NewDB(v, l)
    enforcer, cleanup2, err := platform.NewCasbinEnforcer(v, l, db)
    rbacMu := casbinkit.NewRBACMu()
    userRepo := user.NewRepo(db, enforcer, l, rbacMu)
    // ...自动按依赖顺序拼出来
}
```

#### 关键命令

```bash
make init                                            # 安装 go.mod tool 锁定的 Wire 等工具
go tool wire ./cmd/server/wire                       # 重新生成 wire_gen.go
go tool wire ./cmd/seed/wire
```

#### 三大优势

1. **改依赖零负担**：改 `NewXXX` 函数签名 → 跑 `wire` → 三个 cmd 装配代码自动更新
2. **编译期检查**：漏依赖、循环依赖直接在 `wire` 命令报错，代码生成不出来
3. **零运行时开销**：生成的是普通函数调用，无反射

#### 新人易踩坑

| 坑                          | 解决                                   |
| --------------------------- | -------------------------------------- |
| 改了 wire.go 忘跑 wire 命令 | 真正运行的是 wire_gen.go，必须重新生成 |
| Provider 没加进 wire.NewSet | wire 当它不存在，编译失败              |

---

### 4.2 Gin — Web 框架

#### 路由注册（`internal/server/http.go`）

```go
v1 := s.Group("/v1")
{
    noAuth := v1.Group("/")                    // 公开路由
    noAuth.POST("/login", authHandler.Login)
    noAuth.POST("/auth/refresh", authHandler.Refresh)
    noAuth.POST("/auth/logout", authHandler.Logout)

    strict := v1.Group("/").Use(               // 鉴权路由（链式中间件）
        middleware.StrictAuth(jwtUtil, logger),
        middleware.AuthMiddleware(e),
    )
    strict.GET("/menus", menuHandler.GetMenus)
    strict.GET("/admin/users", userHandler.GetAdminUsers)
    strict.POST("/admin/user", userHandler.AdminUserCreate)
}
```

#### 参数绑定 + 校验（`internal/auth/handler.go`）

```go
func (h *Handler) Login(ctx *gin.Context) {
    var req apiv1.LoginRequest
    if err := ctx.ShouldBindJSON(&req); err != nil {  // 自动反序列化 + 校验
        apiv1.WriteResponse(ctx, apiv1.ErrBadRequest, nil)
        return
    }
    result, err := h.svc.Login(ctx, &req)
    if err != nil {
        apiv1.WriteResponse(ctx, err, nil)
        return
    }
    apiv1.HandleSuccess(ctx, apiv1.LoginResponseData{
        AccessToken:  result.AccessToken,
        RefreshToken: result.RefreshToken,
        ExpiresIn:    result.ExpiresIn,
    })
}
```

DTO 上的 tag（`api/apiv1/admin.go`）决定校验规则：

```go
type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}
```

常用 binding 规则：`required`、`email`、`min=6`、`max=20`、`oneof=a b c`。

#### 自定义中间件（`internal/middleware/jwt.go`）

```go
func StrictAuth(j *jwt.JWT, logger *log.Logger) gin.HandlerFunc {
    return func(ctx *gin.Context) {
        tokenString := ctx.Request.Header.Get("Authorization")
        if tokenString == "" {
            apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
            ctx.Abort()
            return
        }
        claims, err := j.ParseToken(tokenString)
        if err != nil {
            apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
            ctx.Abort()                  // 中断后续处理
            return
        }
        ctx.Set("claims", claims)        // 给下游处理函数传数据
        ctx.Next()                       // 调用下一个中间件/handler
    }
}
```

---

### 4.3 GORM v2 — ORM

#### 模型定义（`internal/model/admin.go`）

```go
type AdminUser struct {
    gorm.Model                                                  // 嵌入：自动加 ID/CreatedAt/UpdatedAt/DeletedAt
    Username string `gorm:"type:varchar(50);not null;uniqueIndex;comment:用户名"`
    Nickname string `gorm:"type:varchar(50);not null;comment:昵称"`
    Password string `gorm:"type:varchar(255);not null;comment:密码"`
    Email    string `gorm:"type:varchar(100);not null;comment:电子邮件"`
    Phone    string `gorm:"type:varchar(20);not null;comment:手机号"`
    IsDisabled bool `gorm:"type:boolean;not null;default:false;comment:是否禁用"`
    LastLoginAt *time.Time `gorm:"comment:最后登录时间"`
}
func (m *AdminUser) TableName() string { return "admin_users" }
```

> **Go 概念**：`gorm.Model` 通过结构体嵌入达到类似继承的效果。

#### 多驱动连接（`internal/platform/`）

```go
switch driver {
case "mysql":
    return mysql.Open(dsn), nil
case "postgres", "postgresql":
    return postgres.Open(dsn), nil
case "sqlite", "sqlite3":
    return sqlite.Open(dsn), nil
case "sqlserver", "mssql":
    return sqlserver.Open(dsn), nil
}
```

切运行时连接只改 `config/local.yml` 的 `data.db.user.driver` 和 `dsn`。Schema migration 按方言隔离在 `db/migrations/mysql` 与 `db/migrations/postgres`，迁移命令默认按配置里的 driver 选择方言。

#### 常用查询模式

```go
// 链式查询：Model → Where → Order → Offset/Limit → Find
scope := r.db.WithContext(ctx).Model(&model.AdminUser{})
if req.Username != "" {
    scope = scope.Where("username LIKE ?", "%"+req.Username+"%")  // 参数化防注入
}
scope.Count(&total).Error
scope.Offset((req.Page-1)*req.PageSize).Limit(req.PageSize).Find(&list)

// 增删改
r.db.WithContext(ctx).Create(m)
r.db.WithContext(ctx).Where("id = ?", id).Updates(m)
r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.AdminUser{})
r.db.WithContext(ctx).Where("id = ?", id).First(&m)
```

#### 事务（`internal/admin/rbac/*/repository.go`）

```go
err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    if err := casbinkit.EnsureRole(tx, role); err != nil {
        return err
    }
    e, err := casbinkit.NewTxEnforcer(tx)
    if err != nil {
        return err
    }
    _, err = e.AddPermissionForUser(model.RoleSubject(role), resource, action)
    return err
})
casbinkit.Reload(ctx, r.e, r.logger)
```

当前代码在 repository 内直接使用 GORM 事务。涉及 DB 与 Casbin 策略同时变更的接口，会用 `casbinkit.NewTxEnforcer(tx)` 把策略写入同一个事务；提交后再 `Reload` 全局 enforcer，让权限变更尽快可见。

#### Schema 迁移

```bash
# 修改 internal/model/*.go 后生成新 migration
make migrate-diff name=add_xxx

# 应用到本地数据库
make migrate-apply

# 首次写入业务初始数据
make seed
```

应用启动不执行 `AutoMigrate`。Schema 由 Atlas migration 管理，初始账号、菜单、API 与 Casbin 策略由 `cmd/seed` 写入。

---

### 4.4 Casbin — RBAC 权限引擎

#### 核心概念

权限抽象为 **(subject, object, action)** 三元组：

| 元素 | 含义              | 例子                  |
| ---- | ----------------- | --------------------- |
| sub  | 主体（用户/角色） | `admin`               |
| obj  | 资源              | `api:/v1/admin/users` |
| act  | 操作              | `GET`                 |

加 `g(user, role)` 关系实现 RBAC。

#### 模型定义（`internal/platform/casbin.go`）

```go
m, err := model.NewModelFromString(`
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`)
e, _ := casbin.NewSyncedEnforcer(m, a)
e.StartAutoLoadPolicy(10 * time.Second)   // 多实例策略一致性
e.EnableAutoSave(true)
```

#### 双前缀资源设计（`internal/model/admin.go:5`）

```go
const (
    RoleSubjectPrefix  = "role:"   // 角色 subject 命名空间
    MenuResourcePrefix = "menu:"   // 前端菜单可见性
    APIResourcePrefix  = "api:"    // 后端 API 鉴权
)
```

策略举例：

- `(role:admin, menu:/dashboard, read)` → admin 角色能看到 `/dashboard` 菜单
- `(role:admin, api:/v1/admin/users, GET)` → admin 能调这个接口
- `g, 1, role:admin` → 用户 `1` 继承 admin 角色

#### API 鉴权中间件（`internal/middleware/rbac.go:13`）

```go
func AuthMiddleware(e *casbin.SyncedEnforcer) gin.HandlerFunc {
    return func(ctx *gin.Context) {
        v, exists := ctx.Get("claims")
        if !exists {
            apiv1.WriteResponse(ctx, apiv1.ErrUnauthorized, nil)
            ctx.Abort()
            return
        }
        uid := v.(*jwt.MyCustomClaims).UserID
        if strconv.FormatUint(uint64(uid), 10) == model.AdminUserID {
            ctx.Next(); return
        }
        sub := strconv.FormatUint(uint64(uid), 10)
        obj := model.APIResourcePrefix + ctx.Request.URL.Path
        act := ctx.Request.Method
        allowed, err := e.Enforce(sub, obj, act)
        if err != nil {
            apiv1.WriteResponse(ctx, apiv1.ErrInternalServerError.WithCause(err), nil)
            ctx.Abort()
            return
        }
        if !allowed {
            apiv1.WriteResponse(ctx, apiv1.ErrForbidden, nil)
            ctx.Abort(); return
        }
        ctx.Next()
    }
}
```

#### 常用 Casbin API

```go
e.AddRoleForUser("123", model.RoleSubject("admin"))            // 给用户 123 加 admin 角色
e.DeleteRoleForUser("123", model.RoleSubject("admin"))         // 移除角色
e.GetRolesForUser("123")                                      // 查用户的所有角色
e.AddPermissionForUser(model.RoleSubject("admin"), "api:/users", "GET")
e.DeletePermissionForUser(model.RoleSubject("admin"), "api:/users", "GET")
e.GetPermissionsForUser(model.RoleSubject("admin"))            // 查角色直接权限
e.GetImplicitPermissionsForUser("123")                        // 查用户所有权限（含角色继承）
e.Enforce(sub, obj, act)                                      // 判定能不能
```

---

### 4.5 配置 / 日志 / 认证

#### Viper 配置（`pkg/config/config.go`）

```go
conf := viper.New()
conf.SetConfigFile("config/local.yml")
conf.ReadInConfig()
conf.SetEnvPrefix("APP")
conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
conf.AutomaticEnv()

// 使用：
conf.GetString("http.host")
conf.GetInt("http.port")
conf.GetString("data.db.user.dsn")
```

支持环境变量 `APP_CONF` 覆盖配置路径。配置读取启用了 `AutomaticEnv`，常用键还会显式 `BindEnv`，例如 `data.db.user.dsn` → `APP_DATA_DB_USER_DSN`。完整清单以 `pkg/config/config.go` 的 `envBoundKeys` 为准。

#### Zap 日志（`pkg/log/log.go`）

Uber 开源的高性能结构化日志（比标准库快 10 倍）。

```go
logger.Info("server start", zap.String("host", "127.0.0.1:8000"))
logger.Error("db error", zap.Error(err))
logger.WithContext(ctx).Info("Request")   // 带 trace ID
```

配套 lumberjack 滚动切割：按文件大小、份数、天数自动归档。

#### JWT 与 refresh 会话

```go
// 签发 access token，sid 用于和 refresh session 关联
token, _ := j.GenToken(userID, time.Now().Add(accessTTL), map[string]any{"sid": sid})

// 解析
claims, err := j.ParseToken(tokenString)
userID := claims.UserID
```

密钥来自 `config/local.yml` 的 `security.jwt.key`。

Refresh token 不写入 JWT，由 `internal/auth/` 生成并只保存哈希到 Redis：

- `POST /v1/auth/refresh`：旧 refresh 立即失效，返回新 access + 新 refresh。
- `POST /v1/auth/logout`：删除当前 refresh 对应 session，不影响同用户其他设备。
- 管理端踢下线：通过 `AuthService.RevokeAllUserSessions` 或 `KickSession` 删除 Redis session。

#### bcrypt 密码哈希

```go
// 加密存储
hash, _ := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)

// 校验
err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(input))
```

bcrypt 自带盐和成本因子，**永远不要用 MD5/SHA1 存密码**。

---

### 4.6 Swagger

#### Swag 文档（`internal/auth/handler.go` 与 `internal/admin/rbac/*/handler.go`）

```go
// Login godoc
// @Summary 账号登录
// @Tags 用户模块
// @Param request body apiv1.LoginRequest true "params"
// @Success 200 {object} apiv1.LoginResponse
// @Router /v1/login [post]
func (h *Handler) Login(ctx *gin.Context) { ... }
```

跑 `make swag` → 访问 `http://127.0.0.1:8000/swagger/index.html`。

---

## 五、开发新业务功能

每个业务功能对应 `internal/admin/rbac/<子域>/` 下的 vertical slice，自包含 repository + service + handler。

**新增子域步骤**：

1. 在 `internal/admin/rbac/<新子域>/` 目录下新建：
   - `repository.go` — GORM 数据访问
   - `service.go` — 业务逻辑编排
   - `handler.go` — HTTP 解析 + 响应
2. 在 `api/apiv1/` 新增 DTO 文件（请求/响应结构体）
3. 在 `internal/model/` 新增对应的 GORM 实体
4. 在 `cmd/server/wire/wire.go` 的 `rbacSet` 中追加：`NewRepo`、`NewService`、`NewHandler`
5. 跑 `go tool wire ./cmd/server/wire` 生成装配代码
6. 在 `internal/server/http.go` 注册路由
7. 在 `db/atlas/main.go` 的 `models()` 登记新实体，执行 `make migrate-diff name=xxx`
8. 如接口需要权限控制，在后台「API 资源管理」登记接口，再到「角色管理」分配权限

参照 `internal/admin/rbac/user/` 或 `internal/admin/rbac/role/` 的结构即可。

---

## 六、常用命令

```bash
# 一次性安装所有工具
make init

# 准备本地数据库 + 数据迁移 + 热加载启动
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS user;"
make migrate-apply
make seed
make dev

# 仅启动 HTTP 服务（开发模式）
go run ./cmd/server

# 应用 schema migration
make migrate-apply

# 写入初始业务数据（首次部署执行）
make seed

# 重新生成 Wire 装配代码（改完 wire.go 必须执行）
make wire

# 生成 Swagger 文档
make swag

# 编译生产二进制
make build
```

---

## 七、推荐学习路径

1. **跑起来**：`make init` → 按常用命令启动 MySQL/Redis、迁移、seed、server → 浏览器访问 `http://127.0.0.1:8000`，local 环境用 `admin/123456` 登录。
2. **跟一遍 Login 全链路**：从 `internal/auth/handler.go:Login` → `internal/auth/service.go:Login` → `internal/admin/rbac/user/repository.go:GetAdminUserByUsername`，理解 handler 怎么传 ctx、service 怎么做认证规则、Redis 怎么写 refresh session。
3. **看懂 Wire**：对照 `cmd/server/wire/wire.go`（清单）和 `cmd/server/wire/wire_gen.go`（生成产物），看每个 `New*` 函数的入参从哪儿来。
4. **照葫芦画瓢**：仿照 admin 写一个简单 CRUD（比如 Article 文章），跑通"新增 API → 配权限 → 调通"完整流程。
5. **读 RBAC 闭环**：`internal/middleware/rbac.go` + `internal/admin/rbac/casbinkit/` + `internal/server/seed.go:initialRBAC` 三处合看，理解菜单/API 双前缀策略。

掌握以上 + 三层调用链，就能在这个项目上独立做 80% 业务开发。

---

## 八、Go 语言关键概念速查

| 概念               | 说明                                                   | 项目中的例子                                      |
| ------------------ | ------------------------------------------------------ | ------------------------------------------------- |
| 结构体嵌入         | 类似继承但更轻量                                       | `AdminUser` 嵌入 `gorm.Model`                     |
| Interface 隐式实现 | 不需要 `implements` 关键字，方法集匹配即实现           | `*user.Repo` 实现 `auth.UserLookup`               |
| ctx 传参           | `context.Context` 是 Go 惯用的"请求上下文"，贯穿调用链 | 所有 service/repository 方法第一个参数            |
| build tag          | 文件顶部 `//go:build xxx` 控制是否参与编译             | `wire.go` 用 `wireinject` 隔离                    |
| 多返回值           | Go 函数可返回多个值，错误通常是最后一个                | `(token string, err error)`                       |
| `_ = xxx`          | 忽略返回值                                             | `tokenString, _ = ctx.Cookie("accessToken")`      |
| panic/recover      | 异常机制，但 Go 提倡用错误返回值而非 panic             | Wire 的 `panic(wire.Build(...))` 是占位符         |

---

## 九、参考链接

- [go-nunu 脚手架](https://github.com/go-nunu/nunu)
- [Gin 文档](https://gin-gonic.com/docs/)
- [GORM 文档](https://gorm.io/docs/)
- [Casbin 中文文档](https://casbin.org/zh/docs/overview)
- [Wire 教程](https://github.com/google/wire/blob/main/_tutorial/README.md)
- [zap 文档](https://pkg.go.dev/go.uber.org/zap)
