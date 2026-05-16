// Package platform 收敛纯基础设施构造（无业务概念），供 wire 装配。
//
// 边界约束：不依赖 internal/model、internal/service、internal/repository
// 等业务包；仅依赖 pkg/ 与第三方库。业务 repo 通过 wire 注入这里产出的
// *gorm.DB / *redis.Client / *casbin.SyncedEnforcer。
package platform

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"

	"rabc-go/pkg/config"
	"rabc-go/pkg/log"
	"rabc-go/pkg/zapgorm2"
)

// NewDB 按 data.db.user.driver 选取 Dialector。
//
// TranslateError=true 让 driver 错（dup key / FK 失败等）翻译成 GORM sentinel
// （gorm.ErrDuplicatedKey / gorm.ErrForeignKeyViolated），repo 层据此用 errors.Is
// 精确分支，不依赖 driver 字符串匹配。
func NewDB(conf *viper.Viper, l *log.Logger) (*gorm.DB, func(), error) {
	logger := zapgorm2.New(l.Logger)
	driver := conf.GetString("data.db.user.driver")
	dsn := conf.GetString("data.db.user.dsn")
	dialector, err := newDialector(driver, dsn)
	if err != nil {
		return nil, nil, err
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger:         logger,
		TranslateError: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", driver, err)
	}
	// SQL 日志默认仅 local 开启：其它环境关闭可避免含 PII 的 SQL 与参数无差别落盘。
	// APP_DATA_DB_DEBUG 可临时覆盖。
	debugDefault := config.IsLocal(conf)
	conf.SetDefault("data.db.debug", debugDefault)
	if conf.GetBool("data.db.debug") {
		db = db.Debug()
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	cleanup := func() {
		if err := sqlDB.Close(); err != nil {
			l.Warn("关闭数据库连接失败", zap.Error(err))
		}
	}
	return db, cleanup, nil
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
