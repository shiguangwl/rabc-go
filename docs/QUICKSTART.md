# nunu-layout-admin 快速入门

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
nunu-layout-admin/
├── cmd/                    # 程序入口（每个生成一个二进制）
│   ├── server/             # HTTP 服务主入口
│   ├── seed/               # 初始业务数据写入（不建表）
│   └── dbmigrate/          # Atlas migration 命令封装
│
├── api/apiv1/                 # 对外 DTO + 错误码
│   ├── v1.go               # 统一响应封装、错误码注册表
│   ├── errors.go           # 业务错误码常量
│   └── admin.go            # 后台管理请求/响应结构体
│
├── internal/               # 私有业务代码（外部模块禁止 import）
│   ├── handler/            # 控制器层：解析请求 → 调 Service
│   ├── service/            # 业务逻辑层：编排 Repository
│   ├── repository/         # 数据访问层：GORM + Casbin
│   ├── model/              # GORM 实体 + 业务常量
│   ├── middleware/         # Gin 中间件（CORS、JWT、RBAC、日志、签名）
│   └── server/             # Server 接口实现（HTTP/Seed）
│
├── pkg/                    # 可复用工具包（理论上可被其他项目引用）
│   ├── app/                # 应用容器：聚合多个 Server 优雅启停
│   ├── config/             # Viper 封装
│   ├── jwt/                # JWT 签发/解析
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
├── deploy/remote/          # 远程部署配置
├── storage/                # 运行时日志等本地文件
├── web/                    # Vue3 前端，构建产物由 web/embed.go 内嵌
└── Makefile                # help/init/build/test/migrate/seed
```

### 三层调用链

```
HTTP 请求
  ↓
[middleware] CORS → 日志 → JWT → Casbin RBAC
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

| 类别      | 框架                       | 作用                         | 项目里的位置                                                  |
| --------- | -------------------------- | ---------------------------- | ------------------------------------------------------------- |
| Web 框架  | Gin                        | HTTP 路由、参数绑定、中间件  | `internal/handler`、`internal/middleware`                     |
| ORM       | GORM v2                    | 多驱动数据库操作             | `internal/repository`                                         |
| 权限      | Casbin                     | RBAC 策略引擎                | `internal/middleware/rbac.go`、`internal/repository/admin.go` |
| 依赖注入  | Wire                       | 编译期生成装配代码           | `cmd/*/wire/`                                                 |
| 配置      | Viper                      | YAML 配置加载                | `pkg/config`                                                  |
| 日志      | zap + lumberjack           | 结构化日志 + 滚动切割        | `pkg/log`                                                     |
| 认证      | golang-jwt v5              | JWT 签发解析                 | `pkg/jwt`                                                     |
| 会话      | redis/go-redis v9          | refresh token、会话索引、吊销 | `internal/repository/auth.go`、`internal/service/auth.go`     |
| 密码      | golang.org/x/crypto/bcrypt | 密码哈希                     | `internal/service/admin.go`                                   |
| 分布式 ID | sonyflake                  | 雪花 ID + Base62             | `pkg/sid`                                                     |
| API 文档  | swag                       | 注解生成 Swagger             | `docs/swagger/`（自动生成）                                   |
| 校验      | go-playground/validator    | binding tag 校验（Gin 内置） | `api/apiv1/admin.go`                                             |
| 工具库    | duke-git/lancet            | 字符串/MD5/UUID 等工具       | 散见各处                                                      |

---

## 四、核心框架详解

### 4.1 Wire — 依赖注入（必须先理解）

#### 它解决什么问题？

任何分层项目的 main 函数都要按拓扑顺序 new 一堆对象：

```go
// 假设没有 Wire，你得自己写：
db := repository.NewDB(conf, logger)
enforcer := repository.NewCasbinEnforcer(conf, logger, db)
repo := repository.NewRepository(logger, db, enforcer)
adminRepo := repository.NewAdminRepository(repo)
baseService := service.NewService(...)
authService := service.NewAuthService(baseService, authRepo, adminRepo, authConfig)
adminService := service.NewAdminService(baseService, adminRepo, authService)
adminHandler := handler.NewAdminHandler(baseHandler, adminService, authService)
authHandler := handler.NewAuthHandler(baseHandler, authService)
httpServer := server.NewHTTPServer(logger, conf, jwtUtil, enforcer, adminHandler, authHandler)
// ...继续 20 行
```

**痛点**：顺序敏感、改一个依赖牵全身、三个 cmd 入口都要写一遍。

#### Wire 怎么解决

你只写"零件清单"（`cmd/server/wire/wire.go`），Wire 工具帮你生成上面那段装配代码到 `wire_gen.go`。

**清单（你写）**：

```go
//go:build wireinject              // build tag：只在跑 wire 工具时编译

var repositorySet = wire.NewSet(   // 把构造函数捆成一组
    repository.NewDB,
    repository.NewRepository,
    repository.NewCasbinEnforcer,
    repository.NewAdminRepository,
)

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
    panic(wire.Build(repositorySet, serviceSet, handlerSet, ...))  // panic 不会执行，只给 wire 工具看签名
}
```

**生成产物（自动）**：

```go
//go:build !wireinject              // 取反，正常编译只走这个文件

func NewWire(v *viper.Viper, l *log.Logger) (*app.App, func(), error) {
    db := repository.NewDB(v, l)
    enforcer := repository.NewCasbinEnforcer(v, l, db)
    repo := repository.NewRepository(l, db, enforcer)
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
    noAuthRouter := v1.Group("/")              // 公开路由
    noAuthRouter.POST("/login", adminHandler.Login)
    noAuthRouter.POST("/auth/refresh", authHandler.Refresh)
    noAuthRouter.POST("/auth/logout", authHandler.Logout)

    strictAuthRouter := v1.Group("/").Use(     // 鉴权路由（链式中间件）
        middleware.StrictAuth(jwt, logger),
        middleware.AuthMiddleware(e),
    )
    strictAuthRouter.GET("/admin/users", adminHandler.GetAdminUsers)
    strictAuthRouter.POST("/admin/user", adminHandler.AdminUserCreate)
}
```

#### 参数绑定 + 校验（`internal/handler/admin.go:35`）

```go
func (h *AdminHandler) Login(ctx *gin.Context) {
    var req v1.LoginRequest
    if err := ctx.ShouldBindJSON(&req); err != nil {  // 自动反序列化 + 校验
        v1.WriteResponse(ctx, v1.ErrBadRequest, nil)
        return
    }
    result, _ := h.authService.Login(ctx, &req)
    v1.HandleSuccess(ctx, v1.LoginResponseData{
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
        claims, err := j.ParseToken(tokenString)
        if err != nil {
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
    Username string `gorm:"type:varchar(50);uniqueIndex;not null"`
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

#### 多驱动连接（`internal/repository/repository.go:77`）

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
scope := r.DB(ctx).Model(&model.AdminUser{})
if req.Username != "" {
    scope = scope.Where("username LIKE ?", "%"+req.Username+"%")  // 参数化防注入
}
scope.Count(&total).Error
scope.Offset((req.Page-1)*req.PageSize).Limit(req.PageSize).Find(&list)

// 增删改
r.DB(ctx).Create(m)
r.DB(ctx).Where("id = ?", id).Updates(m)
r.DB(ctx).Where("id = ?", id).Delete(&model.AdminUser{})
r.DB(ctx).Where("id = ?", id).First(&m)
```

#### 事务（`internal/repository/repository.go:43`）

```go
type Transaction interface {
    Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// 使用：
s.tm.Transaction(ctx, func(txCtx context.Context) error {
    if err := s.adminRepository.RoleCreate(txCtx, ...); err != nil { return err }
    return s.adminRepository.UpdateUserRoles(txCtx, ...)
})
```

巧妙之处：`*gorm.DB` 通过 `context.WithValue` 塞进 ctx，下层 `r.DB(ctx)` 取出，事务内外代码完全一致。

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

#### 模型定义（`internal/repository/repository.go:113`）

```go
m, err := model.NewModelFromString(`
[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
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
    MenuResourcePrefix = "menu:"   // 前端菜单可见性
    ApiResourcePrefix  = "api:"    // 后端 API 鉴权
)
```

策略举例：

- `(admin, menu:/dashboard, read)` → admin 角色能看到 `/dashboard` 菜单
- `(admin, api:/v1/admin/users, GET)` → admin 能调这个接口

#### API 鉴权中间件（`internal/middleware/rbac.go:13`）

```go
func AuthMiddleware(e *casbin.SyncedEnforcer) gin.HandlerFunc {
    return func(ctx *gin.Context) {
        uid := ctx.MustGet("claims").(*jwt.MyCustomClaims).UserID
        if convertor.ToString(uid) == model.AdminUserID {  // 防呆：超管 ID=1 直接放行
            ctx.Next(); return
        }
        sub := convertor.ToString(uid)
        obj := model.ApiResourcePrefix + ctx.Request.URL.Path
        act := ctx.Request.Method
        allowed, _ := e.Enforce(sub, obj, act)             // ← 一行决定通不通
        if !allowed {
            ctx.Abort(); return
        }
        ctx.Next()
    }
}
```

#### 常用 Casbin API

```go
e.AddRoleForUser("123", "admin")                              // 给用户 123 加 admin 角色
e.DeleteRoleForUser("123", "admin")                           // 移除角色
e.GetRolesForUser("123")                                      // 查用户的所有角色
e.AddPermissionForUser("admin", "api:/users", "GET")          // 给角色加权限
e.DeletePermissionForUser("admin", "api:/users", "GET")       // 移权限
e.GetPermissionsForUser("admin")                              // 查角色直接权限
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

// 使用：
conf.GetString("http.host")
conf.GetInt("http.port")
conf.GetString("data.db.user.dsn")
```

支持环境变量 `APP_CONF` 覆盖配置路径。

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

Refresh token 不写入 JWT，由 `internal/service/auth.go` 生成并只保存哈希到 Redis：

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

#### Swag 文档（`internal/handler/admin.go`）

```go
// Login godoc
// @Summary 账号登录
// @Tags 用户模块
// @Param request body v1.LoginRequest true "params"
// @Success 200 {object} v1.LoginResponse
// @Router /v1/login [post]
func (h *AdminHandler) Login(ctx *gin.Context) { ... }
```

跑 `make swag` → 访问 `http://127.0.0.1:8000/swagger/index.html`。

---

## 五、业务开发实战：新增"商品"模块

按这条装配线走，你能独立写出完整功能。每一步对应一层架构，强制保持单一职责。

| 步骤         | 文件                                  | 干什么                                                            |
| ------------ | ------------------------------------- | ----------------------------------------------------------------- |
| ① 模型       | `internal/model/product.go`           | 定义 `Product` struct + GORM tag                                  |
| ② DTO        | `api/apiv1/product.go`                   | 定义请求/响应结构体                                               |
| ③ Repository | `internal/repository/product.go`      | `ProductRepository` interface + 实现                              |
| ④ Service    | `internal/service/product.go`         | `ProductService` interface + 业务逻辑                             |
| ⑤ Handler    | `internal/handler/product.go`         | 解析请求 → 调 service（带 swag 注解）                             |
| ⑥ 路由       | `internal/server/http.go`             | 在 `strictAuthRouter` 加路由                                      |
| ⑦ Wire 注入  | `cmd/server/wire/wire.go`             | 加 `NewProductRepository/Service/Handler`                         |
| ⑧ 重生成     | shell                                 | `go tool wire ./cmd/server/wire`                                  |
| ⑨ 建表       | `db/atlas/main.go` + `db/migrations/` | 在 `models()` 登记模型并执行 `make migrate-diff name=add_product` |
| ⑩ 配权限     | 后台界面                              | API 管理新增 → 角色管理分配权限                                   |

### 模板代码（参照 admin 模块）

**Repository**：

```go
type ProductRepository interface {
    Create(ctx context.Context, m *model.Product) error
    GetList(ctx context.Context, req *v1.GetProductsRequest) ([]model.Product, int64, error)
}

type productRepository struct{ *Repository }

func NewProductRepository(r *Repository) ProductRepository {
    return &productRepository{Repository: r}
}

func (r *productRepository) Create(ctx context.Context, m *model.Product) error {
    return r.DB(ctx).Create(m).Error
}
```

**Service**：

```go
type ProductService interface {
    Create(ctx context.Context, req *v1.ProductCreateRequest) error
}

type productService struct {
    *Service
    productRepository repository.ProductRepository
}

func NewProductService(s *Service, repo repository.ProductRepository) ProductService {
    return &productService{Service: s, productRepository: repo}
}
```

**Handler**：

```go
type ProductHandler struct {
    *Handler
    productService service.ProductService
}

func NewProductHandler(h *Handler, s service.ProductService) *ProductHandler {
    return &ProductHandler{Handler: h, productService: s}
}
```

**Wire 清单更新**：

```go
var repositorySet = wire.NewSet(
    // ...原有
    repository.NewProductRepository,
)
var serviceSet = wire.NewSet(
    // ...原有
    service.NewProductService,
)
var handlerSet = wire.NewSet(
    // ...原有
    handler.NewProductHandler,
)
```

更新后必须跑：`go tool wire ./cmd/server/wire`

---

## 六、常用命令

```bash
# 一次性安装所有工具
make init

# 准备本地数据库 + 数据迁移 + 热加载启动
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS user;"
make migrate-apply
make seed
nunu run ./cmd/server

# 仅启动 HTTP 服务（开发模式）
go run ./cmd/server

# 应用 schema migration
make migrate-apply

# 写入初始业务数据（首次部署执行）
make seed

# 重新生成 Wire 装配代码（改完 wire.go 必须执行）
go tool wire ./cmd/server/wire
go tool wire ./cmd/seed/wire

# 生成 Swagger 文档
make swag

# 编译生产二进制
make build
```

---

## 七、推荐学习路径

1. **跑起来**：`make init` → 按常用命令启动 MySQL/Redis、迁移、seed、server → 浏览器访问 `http://127.0.0.1:8000`，local 环境用 `admin/123456` 登录。
2. **跟一遍 Login 全链路**：从 `internal/handler/admin.go:Login` → `internal/service/auth.go:Login` → `internal/repository/admin_user_repo.go:GetAdminUserByUsername`，理解三层是怎么传 ctx、传错误和写 refresh session 的。
3. **看懂 Wire**：对照 `cmd/server/wire/wire.go`（清单）和 `cmd/server/wire/wire_gen.go`（生成产物），看每个 `New*` 函数的入参从哪儿来。
4. **照葫芦画瓢**：仿照 admin 写一个简单 CRUD（比如 Article 文章），跑通"新增 API → 配权限 → 调通"完整流程。
5. **读 RBAC 闭环**：`internal/middleware/rbac.go` + `internal/repository/repository.go:NewCasbinEnforcer` + `internal/server/seed.go:initialRBAC` 三处合看，理解菜单/API 双前缀策略。

掌握以上 + 三层调用链，就能在这个项目上独立做 80% 业务开发。

---

## 八、Go 语言关键概念速查

| 概念               | 说明                                                   | 项目中的例子                                      |
| ------------------ | ------------------------------------------------------ | ------------------------------------------------- |
| 结构体嵌入         | 类似继承但更轻量                                       | `AdminUser` 嵌入 `gorm.Model`                     |
| Interface 隐式实现 | 不需要 `implements` 关键字，方法集匹配即实现           | `productRepository` 实现 `ProductRepository`      |
| ctx 传参           | `context.Context` 是 Go 惯用的"请求上下文"，贯穿调用链 | 所有 service/repository 方法第一个参数            |
| build tag          | 文件顶部 `//go:build xxx` 控制是否参与编译             | `wire.go` 用 `wireinject` 隔离                    |
| 多返回值           | Go 函数可返回多个值，错误通常是最后一个                | `(token string, err error)`                       |
| `_ = xxx`          | 忽略返回值                                             | `roles, _ := s.adminRepository.GetUserRoles(...)` |
| panic/recover      | 异常机制，但 Go 提倡用错误返回值而非 panic             | Wire 的 `panic(wire.Build(...))` 是占位符         |

---

## 九、参考链接

- [go-nunu 脚手架](https://github.com/go-nunu/nunu)
- [Gin 文档](https://gin-gonic.com/docs/)
- [GORM 文档](https://gorm.io/docs/)
- [Casbin 中文文档](https://casbin.org/zh/docs/overview)
- [Wire 教程](https://github.com/google/wire/blob/main/_tutorial/README.md)
- [zap 文档](https://pkg.go.dev/go.uber.org/zap)
