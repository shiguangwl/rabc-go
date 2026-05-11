package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	v1 "rabc-go/api/v1"
	"rabc-go/db/seed"
	"rabc-go/internal/model"
	"rabc-go/internal/repository"
	"rabc-go/pkg/config"
	"rabc-go/pkg/log"
	"rabc-go/pkg/sid"
)

// SeedServer 仅负责 RBAC 业务初始数据写入。
//
// 职责边界（与 atlas 严格分工）：
//   - schema 演进（建表/改列/索引）由 atlas migrate apply 处理，本服务前置依赖 schema 已存在
//   - 本服务只读写表中数据；不调 AutoMigrate / DropTable / 任何 DDL
//
// 行为开关由 conf "seed.reset" 控制：
//   - false（默认）：要求 RBAC 业务表为空才写入；非空直接拒绝，避免重复种子或主键冲突
//   - true（仅 dev/local）：先 TRUNCATE RBAC 业务表，再写入
//
// prod 环境永远禁止 seed.reset=true。
type SeedServer struct {
	db   *gorm.DB
	log  *log.Logger
	sid  *sid.Sid
	e    *casbin.SyncedEnforcer
	conf *viper.Viper
}

func NewSeedServer(
	db *gorm.DB,
	log *log.Logger,
	sid *sid.Sid,
	e *casbin.SyncedEnforcer,
	conf *viper.Viper,
) *SeedServer {
	return &SeedServer{
		e:    e,
		db:   db,
		log:  log,
		sid:  sid,
		conf: conf,
	}
}

// rbacTables 列出需要在 reset 时 truncate 的业务表名。
// 表名硬编码而非反射 model 取，避免 truncate 误及未预期的表。
var rbacTables = []string{"admin_users", "menu", "roles", "api", "casbin_rule"}

func (m *SeedServer) Start(ctx context.Context) error {
	if m.conf.GetBool("seed.reset") {
		return m.reset(ctx)
	}
	return m.seed(ctx)
}

// seed 在空库上写入初始数据；任一 RBAC 表非空则拒绝，避免半初始化状态继续写入。
func (m *SeedServer) seed(ctx context.Context) error {
	empty, err := m.areSeedTablesEmpty(ctx)
	if err != nil {
		return fmt.Errorf("check seed tables empty: %w", err)
	}
	if !empty {
		return errors.New("seed aborted: RBAC tables are not empty; pass -reset=true to wipe and re-seed (dev only)")
	}
	return m.runInitialData(ctx)
}

// mysqlDSNHostRe 提取 go-sql-driver/mysql DSN 中 protocol(addr) 的 addr 部分，
// 例如 "user:pwd@tcp(127.0.0.1:3306)/db?..." → "127.0.0.1:3306"。
var mysqlDSNHostRe = regexp.MustCompile(`@[a-zA-Z]+\(([^)]+)\)`)

// isLocalDSN 判定 dsn 的 host 是否落在本机白名单内。
// 仅给 reset() 做兜底防御：env=local 但 dsn 误指向远端时拦截 TRUNCATE。
func isLocalDSN(driver, dsn string) bool {
	switch normalizeDBDriver(driver) {
	case "mysql":
		return isLocalMySQLDSN(dsn)
	case "postgres":
		return isLocalPostgresDSN(dsn)
	default:
		return false
	}
}

func isLocalMySQLDSN(dsn string) bool {
	matches := mysqlDSNHostRe.FindStringSubmatch(dsn)
	if len(matches) < 2 {
		return false
	}
	return isLocalHost(hostFromAddr(matches[1]))
}

func isLocalPostgresDSN(dsn string) bool {
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		return isLocalHost(u.Hostname())
	}

	fields := strings.Fields(dsn)
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key != "host" {
			continue
		}
		return isLocalHost(hostFromAddr(strings.Trim(value, `"'`)))
	}
	return false
}

func hostFromAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}
	if idx := strings.LastIndex(addr, ":"); idx != -1 && strings.Count(addr, ":") == 1 {
		return addr[:idx]
	}
	return strings.Trim(addr, "[]")
}

func isLocalHost(host string) bool {
	switch host {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	}
	return false
}

// reset 清空业务表与 Casbin 策略，然后重新写入初始数据。
//
// 强破坏操作，准入条件（双重防御）：
//   - env=local（白名单，staging/uat/prod 均拒绝）
//   - dsn host 必须为本机环回（防御 env=local 但 dsn 指向远端环境的误用）
//   - conf "seed.reset" = true（来自 -reset CLI flag）
//
// 不调 DROP/AutoMigrate；schema 由 atlas 管。
func (m *SeedServer) reset(ctx context.Context) error {
	// 第一道防御：仅 local 允许 -reset。
	// 旧实现用 !IsProd 放行，意外把 staging 测试数据 TRUNCATE 的事故无法兜底。
	if !config.IsLocal(m.conf) {
		return fmt.Errorf("seed -reset is only allowed in env=local (current env=%q)", m.conf.GetString("env"))
	}
	// 第二道防御：dsn host 必须本机。env 字段是配置文件里的字符串标签，
	// 万一镜像里同时打包了 env=local 的 yml + 远端 dsn（CI 或人为失误），
	// 单靠 IsLocal 就会把生产/测试数据 TRUNCATE。host 校验是不可绕过的兜底。
	driver := m.conf.GetString("data.db.user.driver")
	dsn := m.conf.GetString("data.db.user.dsn")
	if !isLocalDSN(driver, dsn) {
		return errors.New("seed -reset rejected: dsn host must be 127.0.0.1/localhost/::1")
	}
	if err := m.truncateBusinessTables(ctx); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}
	// reset 路径已在 truncateBusinessTables 中 TRUNCATE casbin_rule，这里
	// 只需要让 enforcer 重新从库加载策略即可，无需 ClearPolicy + SavePolicy。
	if err := m.e.LoadPolicy(); err != nil {
		return fmt.Errorf("reload casbin policy after truncate: %w", err)
	}
	return m.runInitialData(ctx)
}

// truncateBusinessTables 用方言对应的 TRUNCATE 清表并重置自增序列。
func (m *SeedServer) truncateBusinessTables(ctx context.Context) error {
	for _, table := range rbacTables {
		stmt, err := truncateTableSQL(m.conf.GetString("data.db.user.driver"), table)
		if err != nil {
			return err
		}
		if err := m.db.WithContext(ctx).Exec(stmt).Error; err != nil {
			return fmt.Errorf("truncate %s: %w", table, err)
		}
	}
	return nil
}

func truncateTableSQL(driver, table string) (string, error) {
	switch normalizeDBDriver(driver) {
	case "mysql":
		return "TRUNCATE TABLE `" + table + "`", nil
	case "postgres":
		return `TRUNCATE TABLE "` + table + `" RESTART IDENTITY CASCADE`, nil
	default:
		return "", fmt.Errorf("seed -reset unsupported db driver %q", driver)
	}
}

func normalizeDBDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql":
		return "mysql"
	case "postgres", "postgresql":
		return "postgres"
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

// runInitialData 串行执行所有初始化步骤，任一失败立即返回。
func (m *SeedServer) runInitialData(ctx context.Context) error {
	if err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e, err := newSeedTxEnforcer(tx)
		if err != nil {
			return err
		}
		return m.runInitialDataWithDB(ctx, tx, e)
	}); err != nil {
		return err
	}
	if err := m.e.LoadPolicy(); err != nil {
		return fmt.Errorf("reload casbin policy after seed commit: %w", err)
	}
	return nil
}

func newSeedTxEnforcer(tx *gorm.DB) (casbin.IEnforcer, error) {
	gormadapter.TurnOffAutoMigrate(tx)
	a, err := gormadapter.NewAdapterByDB(tx)
	if err != nil {
		return nil, fmt.Errorf("init seed tx casbin adapter: %w", err)
	}
	casbinModel, err := repository.NewCasbinModel()
	if err != nil {
		return nil, fmt.Errorf("init seed tx casbin model: %w", err)
	}
	e, err := casbin.NewEnforcer(casbinModel, a)
	if err != nil {
		return nil, fmt.Errorf("init seed tx casbin enforcer: %w", err)
	}
	return e, nil
}

func (m *SeedServer) runInitialDataWithDB(ctx context.Context, db *gorm.DB, e casbin.IEnforcer) error {
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"initialAdminUser", func(ctx context.Context) error { return m.initialAdminUser(ctx, db) }},
		{"initialMenuData", func(ctx context.Context) error { return m.initialMenuData(ctx, db) }},
		{"initialApisData", func(ctx context.Context) error { return m.initialApisData(ctx, db) }},
		{"initialRBAC", func(ctx context.Context) error { return m.initialRBAC(ctx, db, e) }},
	}
	for _, step := range steps {
		if err := step.fn(ctx); err != nil {
			m.log.Error("seed step failed",
				zap.String("step", step.name), zap.Error(err))
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	m.log.Info("seed: initial data written")
	return nil
}

func (m *SeedServer) areSeedTablesEmpty(ctx context.Context) (bool, error) {
	for _, table := range rbacTables {
		var count int64
		if err := m.db.WithContext(ctx).Table(table).Count(&count).Error; err != nil {
			return false, fmt.Errorf("count %s: %w", table, err)
		}
		if count > 0 {
			return false, nil
		}
	}
	return true, nil
}

func (m *SeedServer) Stop(ctx context.Context) error {
	m.log.Info("seed stop")
	return nil
}
func (m *SeedServer) initialAdminUser(ctx context.Context, db *gorm.DB) error {
	// 初始密码取值优先级：APP_SEED_INITIAL_PASSWORD env > seed.initial_password yml。
	// 非 local 环境必须显式提供，缺失直接报错，避免弱密码进 prod。
	initialPassword := m.conf.GetString("seed.initial_password")
	if initialPassword == "" {
		// 仅 env=local 允许 fallback 弱默认。staging/uat/prod 都强制要求显式提供，
		// 避免任何对外环境上线后留下 admin/123456 这条万能后门。
		if !config.IsLocal(m.conf) {
			return errors.New("seed: APP_SEED_INITIAL_PASSWORD must be provided in non-local env")
		}
		initialPassword = "123456"
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(initialPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Create(&[]model.AdminUser{
		{Model: gorm.Model{ID: 1}, Username: "admin", Password: string(hashedPassword), Nickname: "Admin"},
		{Model: gorm.Model{ID: 2}, Username: "user", Password: string(hashedPassword), Nickname: "运营人员"},
	}).Error; err != nil {
		return err
	}
	return nil
}
func (m *SeedServer) initialRBAC(ctx context.Context, db *gorm.DB, e casbin.IEnforcer) error {
	roles := []model.Role{
		{Sid: model.AdminRole, Name: "超级管理员"},
		{Sid: "1000", Name: "运营人员"},
		{Sid: "1001", Name: "访客"},
	}
	if err := db.WithContext(ctx).Create(&roles).Error; err != nil {
		return err
	}
	// 不在此处 ClearPolicy + SavePolicy。原因：会清掉运行时由管理员分配的策略。
	// 显式 reset 路径已在 SeedServer.reset 内统一清理；默认 seed 路径下，
	// 依赖 Casbin AddXxx 的"已存在 ok=false err=nil"幂等语义即可重入。
	if _, err := e.AddRoleForUser(model.AdminUserID, model.AdminRole); err != nil {
		return fmt.Errorf("AddRoleForUser admin: %w", err)
	}

	menuList := make([]v1.MenuDataItem, 0)
	if err := json.Unmarshal([]byte(seed.MenuJSON), &menuList); err != nil {
		return fmt.Errorf("unmarshal menu data: %w", err)
	}
	for _, item := range menuList {
		if err := m.addPermissionForRole(e, model.AdminRole, model.MenuResourcePrefix+item.Path, "read"); err != nil {
			return err
		}
	}
	apiList := make([]model.Api, 0)
	if err := db.WithContext(ctx).Find(&apiList).Error; err != nil {
		return fmt.Errorf("load api list: %w", err)
	}
	for _, api := range apiList {
		if err := m.addPermissionForRole(e, model.AdminRole, model.ApiResourcePrefix+api.Path, api.Method); err != nil {
			return err
		}
	}

	// 添加运营人员权限
	if _, err := e.AddRoleForUser("2", "1000"); err != nil {
		return fmt.Errorf("AddRoleForUser operator: %w", err)
	}
	operatorPerms := []struct {
		resource string
		action   string
	}{
		{model.MenuResourcePrefix + "/profile/basic", "read"},
		{model.MenuResourcePrefix + "/profile/advanced", "read"},
		{model.MenuResourcePrefix + "/profile", "read"},
		{model.MenuResourcePrefix + "/dashboard", "read"},
		{model.MenuResourcePrefix + "/dashboard/workplace", "read"},
		{model.MenuResourcePrefix + "/dashboard/analysis", "read"},
		{model.MenuResourcePrefix + "/account/settings", "read"},
		{model.MenuResourcePrefix + "/account/center", "read"},
		{model.MenuResourcePrefix + "/account", "read"},
		{model.ApiResourcePrefix + "/v1/menus", http.MethodGet},
		{model.ApiResourcePrefix + "/v1/admin/user", http.MethodGet},
	}
	for _, p := range operatorPerms {
		if err := m.addPermissionForRole(e, "1000", p.resource, p.action); err != nil {
			return err
		}
	}
	return nil
}

// addPermissionForRole 为指定角色追加 (resource, action) 权限。
// Casbin AddPermissionForUser 已存在时返回 (false, nil)，幂等可重放。
// 失败 return error 让调用方早返，避免静默吞错。
func (m *SeedServer) addPermissionForRole(e casbin.IEnforcer, role, resource, action string) error {
	if _, err := e.AddPermissionForUser(role, resource, action); err != nil {
		return fmt.Errorf("AddPermissionForUser %s %s:%s: %w", role, resource, action, err)
	}
	m.log.Debug("seed: granted permission",
		zap.String("role", role),
		zap.String("resource", resource),
		zap.String("action", action),
	)
	return nil
}
func (m *SeedServer) initialApisData(ctx context.Context, db *gorm.DB) error {
	initialApis := []model.Api{

		{Group: "基础API", Name: "获取用户菜单列表", Path: "/v1/menus", Method: http.MethodGet},
		{Group: "基础API", Name: "获取管理员信息", Path: "/v1/admin/user", Method: http.MethodGet},

		{Group: "菜单管理", Name: "获取管理菜单", Path: "/v1/admin/menus", Method: http.MethodGet},
		{Group: "菜单管理", Name: "创建菜单", Path: "/v1/admin/menu", Method: http.MethodPost},
		{Group: "菜单管理", Name: "更新菜单", Path: "/v1/admin/menu", Method: http.MethodPut},
		{Group: "菜单管理", Name: "删除菜单", Path: "/v1/admin/menu", Method: http.MethodDelete},

		{Group: "权限模块", Name: "获取用户权限", Path: "/v1/admin/user/permissions", Method: http.MethodGet},
		{Group: "权限模块", Name: "获取角色权限", Path: "/v1/admin/role/permissions", Method: http.MethodGet},
		{Group: "权限模块", Name: "更新角色权限", Path: "/v1/admin/role/permission", Method: http.MethodPut},
		{Group: "权限模块", Name: "获取角色列表", Path: "/v1/admin/roles", Method: http.MethodGet},
		{Group: "权限模块", Name: "创建角色", Path: "/v1/admin/role", Method: http.MethodPost},
		{Group: "权限模块", Name: "更新角色", Path: "/v1/admin/role", Method: http.MethodPut},
		{Group: "权限模块", Name: "删除角色", Path: "/v1/admin/role", Method: http.MethodDelete},

		{Group: "权限模块", Name: "获取管理员列表", Path: "/v1/admin/users", Method: http.MethodGet},
		{Group: "权限模块", Name: "更新管理员信息", Path: "/v1/admin/user", Method: http.MethodPut},
		{Group: "权限模块", Name: "创建管理员账号", Path: "/v1/admin/user", Method: http.MethodPost},
		{Group: "权限模块", Name: "删除管理员", Path: "/v1/admin/user", Method: http.MethodDelete},

		{Group: "权限模块", Name: "获取API列表", Path: "/v1/admin/apis", Method: http.MethodGet},
		{Group: "权限模块", Name: "创建API", Path: "/v1/admin/api", Method: http.MethodPost},
		{Group: "权限模块", Name: "更新API", Path: "/v1/admin/api", Method: http.MethodPut},
		{Group: "权限模块", Name: "删除API", Path: "/v1/admin/api", Method: http.MethodDelete},
	}

	return db.WithContext(ctx).Create(&initialApis).Error
}
func (m *SeedServer) initialMenuData(ctx context.Context, db *gorm.DB) error {
	menuList := make([]v1.MenuDataItem, 0)
	err := json.Unmarshal([]byte(seed.MenuJSON), &menuList)
	if err != nil {
		m.log.Error("json.Unmarshal error", zap.Error(err))
		return err
	}
	menuListDb := make([]model.Menu, 0)
	for _, item := range menuList {
		menuListDb = append(menuListDb, model.Menu{
			Model: gorm.Model{
				ID: item.ID,
			},
			ParentID:   item.ParentID,
			Path:       item.Path,
			Title:      item.Title,
			Name:       item.Name,
			Component:  item.Component,
			Locale:     item.Locale,
			Weight:     item.Weight,
			Icon:       item.Icon,
			Redirect:   item.Redirect,
			URL:        item.URL,
			KeepAlive:  item.KeepAlive,
			HideInMenu: item.HideInMenu,
		})
	}
	return db.WithContext(ctx).Create(&menuListDb).Error
}
