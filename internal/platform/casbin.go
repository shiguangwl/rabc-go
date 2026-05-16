package platform

import (
	"fmt"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"rabc-go/pkg/log"
)

// CasbinModelConf 暴露到包级，便于事务路径用 NewCasbinModel 解析出独立 model 实例。
// model 内部是 map，与全局 enforcer 共享会触发并发读写竞态，必须深隔离。
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

// NewCasbinModel 返回新解析的 model.Model 实例。
// 调用方需保证不与全局 enforcer 共享，否则 model 内部 map 会被并发读写。
func NewCasbinModel() (model.Model, error) {
	return model.NewModelFromString(CasbinModelConf)
}

// NewCasbinEnforcer 构造 SyncedEnforcer。
//
// 不变量：schema 由 atlas 统一管控（见 db/atlas/main.go 与 db/migrations），
// 应用启动期必须保持零 DDL，避免多副本启动竞争建表/索引，也防止 adapter
// 升版偷偷 ALTER 列宽。
//
// 多副本策略一致性靠 10s 轮询兜底；若权限变更需要更低延迟，再引入 Casbin Watcher。
func NewCasbinEnforcer(_ *viper.Viper, _ *log.Logger, db *gorm.DB) (*casbin.SyncedEnforcer, func(), error) {
	gormadapter.TurnOffAutoMigrate(db)
	a, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, nil, fmt.Errorf("casbin adapter init failed: %w", err)
	}
	m, err := NewCasbinModel()
	if err != nil {
		return nil, nil, fmt.Errorf("casbin model init failed: %w", err)
	}
	e, err := casbin.NewSyncedEnforcer(m, a)
	if err != nil {
		return nil, nil, fmt.Errorf("casbin enforcer init failed: %w", err)
	}

	e.StartAutoLoadPolicy(10 * time.Second)

	e.EnableAutoSave(true)

	return e, e.StopAutoLoadPolicy, nil
}
