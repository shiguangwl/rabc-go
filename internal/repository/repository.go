package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"

	"nunu-layout-admin/pkg/config"
	"nunu-layout-admin/pkg/log"
	"nunu-layout-admin/pkg/zapgorm2"
)

type ctxKey int

const txCtxKey ctxKey = iota

type Repository struct {
	db *gorm.DB
	e  *casbin.SyncedEnforcer
	//rdb    *redis.Client
	logger *log.Logger
}

func NewRepository(
	logger *log.Logger,
	db *gorm.DB,
	e *casbin.SyncedEnforcer,
	// rdb *redis.Client,
) *Repository {
	return &Repository{
		db: db,
		e:  e,
		//rdb:    rdb,
		logger: logger,
	}
}

type Transaction interface {
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}

func NewTransaction(r *Repository) Transaction {
	return r
}

// DB return tx
// If you need to create a Transaction, you must call DB(ctx) and Transaction(ctx,fn)
func (r *Repository) DB(ctx context.Context) *gorm.DB {
	v := ctx.Value(txCtxKey)
	if v != nil {
		if tx, ok := v.(*gorm.DB); ok {
			return tx
		}
	}
	return r.db.WithContext(ctx)
}

func (r *Repository) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ctx = context.WithValue(ctx, txCtxKey, tx)
		return fn(ctx)
	})
}

func NewDB(conf *viper.Viper, l *log.Logger) *gorm.DB {
	var (
		db  *gorm.DB
		err error
	)

	logger := zapgorm2.New(l.Logger)
	driver := conf.GetString("data.db.user.driver")
	dsn := conf.GetString("data.db.user.dsn")
	dialector, err := newDialector(driver, dsn)
	if err != nil {
		panic(err)
	}
	db, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger,
		// 把 driver 错（如 dup key、FK 失败）翻译成 GORM sentinel
		// （gorm.ErrDuplicatedKey / gorm.ErrForeignKeyViolated），让 repo 层能用
		// errors.Is 精确判定，避免依赖 driver 字符串匹配。
		TranslateError: true,
	})
	if err != nil {
		panic(fmt.Errorf("open %s: %w", driver, err))
	}
	// SQL 日志显式开关：默认 local 打开、其它环境关闭，避免 staging/uat 把含 PII
	// 的 SQL 与参数无差别打到日志。配置项可通过 APP_DATA_DB_DEBUG 临时覆盖。
	debugDefault := config.IsLocal(conf)
	conf.SetDefault("data.db.debug", debugDefault)
	if conf.GetBool("data.db.debug") {
		db = db.Debug()
	}

	// Connection Pool config
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db
}

func newDialector(driver, dsn string) (gorm.Dialector, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mysql":
		return mysql.Open(dsn), nil
	case "postgres", "postgresql":
		return postgres.Open(dsn), nil
	case "sqlite", "sqlite3":
		return sqlite.Open(dsn), nil
	case "sqlserver", "mssql":
		return sqlserver.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported db driver %q (supported: mysql, postgres, sqlite, sqlserver)", driver)
	}
}

// CasbinModelConf 是 Casbin RBAC PERM 模型的字符串定义。
// 提到包级是为了让 repository 在事务路径上重新 parse 出独立的 model 实例
// （map 值类型，必须深隔离），避免与全局 enforcer 共享 model 引发并发竞态。
const CasbinModelConf = `
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
`

// NewCasbinModel 解析 CasbinModelConf 得到一个独立的 model.Model 实例。
// 用于事务路径上的临时 enforcer，每次返回新的 map，杜绝并发写共享 model 的隐患。
func NewCasbinModel() (model.Model, error) {
	return model.NewModelFromString(CasbinModelConf)
}

func NewCasbinEnforcer(conf *viper.Viper, l *log.Logger, db *gorm.DB) *casbin.SyncedEnforcer {
	// 关掉 adapter 的隐式 AutoMigrate / CREATE UNIQUE INDEX：
	// casbin_rule 已纳入 atlas 管控（见 db/atlas/main.go 与 db/migrations），
	// 应用启动期保持"零 DDL"，避免多副本同时启动抢着建表/建索引，也防止
	// adapter 升版偷偷 ALTER 列宽。schema 演进统一由 atlas migrate apply 完成。
	gormadapter.TurnOffAutoMigrate(db)
	a, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		panic(fmt.Errorf("casbin adapter init failed: %w", err))
	}
	m, err := NewCasbinModel()
	if err != nil {
		panic(fmt.Errorf("casbin model init failed: %w", err))
	}
	e, err := casbin.NewSyncedEnforcer(m, a)
	if err != nil {
		panic(fmt.Errorf("casbin enforcer init failed: %w", err))
	}

	// 每10秒自动加载策略，防止启动多服务进程策略不一致
	// 如果不想用轮询DB的方式，你也可以使用Casbin Watchers来同步策略，该方式需要基于Redis、Etcd等存储中间件
	// Watchers相关文档：https://casbin.org/zh/docs/watchers
	e.StartAutoLoadPolicy(10 * time.Second)

	// Enable Logger, decide whether to show it in terminal
	//e.EnableLog(true)

	// Save the policy back to DB.
	e.EnableAutoSave(true)

	return e
}

// NewRedis 预留：缓存层启用时由 wire 注入
func NewRedis(conf *viper.Viper) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     conf.GetString("data.redis.addr"),
		Password: conf.GetString("data.redis.password"),
		DB:       conf.GetInt("data.redis.db"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Errorf("redis init failed: %w", err))
	}

	return rdb
}
