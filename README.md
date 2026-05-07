# N-Admin

N-Admin 是一个基于 [go-nunu](https://github.com/go-nunu/nunu) 开发的开源管理后台模板，采用 **Gin + Casbin（RBAC）+ Vue3 + AntDesignVue + AntdvPro** 技术栈，提供快速开发的基础架构。

<p align="center"><img src="https://github.com/go-nunu/nunu-layout-admin/blob/main/web/src/assets/images/preview-home.png?raw=true"></p>
<p align="center"><img src="https://github.com/go-nunu/nunu-layout-admin/blob/main/web/src/assets/images/preview-api.png?raw=true"></p>

## 要求
要运行项目，您需要在系统上安装以下软件：

* Git
* Golang 1.23 或 更高版本
* NodeJS 18 或 更高版本

## 快速开始

```
# 1. clone项目
git clone https://github.com/go-nunu/nunu-layout-admin.git

# 2. 启动项目
cd nunu-layout-admin
go run cmd/server/main.go

# 3. 访问项目
浏览器访问：http://localhost:8000


超管账号：admin
超管密码：123456

普通用户账号：user
普通用户密码：123456
```
## 📚 角色权限操作流程
当添加API接口或菜单时，需要手动添加权限策略。

1. 添加API接口（操作路径：权限模块->接口管理->添加API）
2. 添加前端菜单（操作路径：权限模块->菜单管理->添加菜单）
3. 添加权限策略（操作路径：权限模块->角色管理->添加角色/分配权限）


## 📌 功能特性
- ✅**权限管理**：基于 Casbin 实现 RBAC 角色权限控制，权限粒度支持接口和菜单控制。
- ✅**多数据库支持**：支持 MySQL、Postgres、Sqlite 等数据库。
- ✅**管理员管理**：支持管理员账号增删改查，密码加密存储。
- ✅**JWT 认证**：支持 Token 认证，提供登录、登出功能。
- ✅**前后端分离**：RESTful API 设计，支持前后端独立部署。
- ✅**支持一键打包**：整站打包为一个可执行二进制文件。
- ✅**防呆设计**：超管账号始终拥有所有菜单及API权限，防止误操作。


## 🚀 技术栈

### 后端技术栈
- **[go-nunu](https://github.com/go-nunu/nunu)** - 轻量级 Golang 脚手架
- **[Gin](https://github.com/gin-gonic/gin)** - 轻量级 Web 框架
- **[Casbin](https://github.com/casbin/casbin)** - 权限管理（RBAC）
- **[GORM](https://github.com/go-gorm/gorm)** - Golang ORM 框架
- **JWT** - 认证和授权
- **MySQL/Postgres/Sqlite** - 数据库支持

### 前端技术栈
- **[AntdvPro](https://github.com/antdv-pro/antdv-pro)** - 企业级中后台前端/设计解决方案
- **[Vue3](https://github.com/vuejs/)** - 渐进式 JavaScript 框架
- **[Vite](https://github.com/vitejs/vite)** - 极速构建工具



## 📦 安装与运行

### 1️⃣ 克隆项目
```bash
git clone https://github.com/go-nunu/nunu-layout-admin.git
cd nunu-layout-admin
```

### 2️⃣ 后端启动
#### 配置修改
编辑 `config/local.yml` 并修改必要的配置信息。

#### 执行数据迁移，初始化项目数据（仅项目首次启动时执行）
```bash
go run cmd/migration/main.go
```

#### 运行后端服务
```bash
go run cmd/server/main.go
```

或者使用 `nunu run` 进行热加载开发：
```bash
nunu run
```

### 3️⃣ 前端启动
```bash
cd web
npm install
npm run dev
```

### 4️⃣ 访问地址
后端服务运行在 `http://localhost:8000`

前端服务运行在 `http://localhost:6678`


## 🔑 权限管理（RBAC）
本项目使用 **Casbin** 进行角色权限管理。

- **角色**：管理员、普通用户等
- **权限**：增删改查等操作权限
- **模型**：采用 `RBAC` 访问控制模型
- **存储**：权限策略存储于数据库

示例策略：

API接口
```
p, admin, api:/api/user, GET
p, admin, api:/api/user, POST
p, admin, api:/api/user, PUT
p, admin, api:/api/user, DELETE
p, user, api:/api/profile, GET
```
前端菜单
```
p, admin, menu:/users, read
p, user, menu:/admin/roles, read
```

## 📦 打包部署
```
cd web
npm run build

cd ../
go build -o server cmd/server/main.go
./server

访问：http://127.0.0.1:8000/
```


注意：上面的方法会将服务端和前端的静态资源一起打包到可执行二进制程序中。

如果不需要Golang来渲染前端，可以采用Nginx等反向代理工具将前端静态资源部署到Nginx上。


## 📜 许可证
本项目基于 **MIT License** 开源，欢迎贡献！

