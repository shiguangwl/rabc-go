package casbinkit

import (
	"context"
	"fmt"
	"time"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"rabc-go/internal/platform"
	"rabc-go/pkg/log"
)

// NewTxEnforcer 返回一个 adapter 绑定到给定 tx 的临时 Casbin enforcer，
// 让 Casbin 写与业务写共享同一事务边界。
//
// 必须遵守的两项约束：
//  1. TurnOffAutoMigrate：MySQL 在事务内执行 DDL 会触发隐式 commit，破坏原子性。
//     casbin_rule 表结构由 atlas 在应用启动期建好，运行期严禁再执行 AutoMigrate。
//  2. 每次构造都重新 parse model：Casbin Model 是 map 值类型，并发写共享同一
//     实例会产生竞态，禁止复用全局 enforcer 的 model。
func NewTxEnforcer(tx *gorm.DB) (casbin.IEnforcer, error) {
	gormadapter.TurnOffAutoMigrate(tx)
	a, err := gormadapter.NewAdapterByDB(tx)
	if err != nil {
		return nil, fmt.Errorf("init tx casbin adapter: %w", err)
	}
	m, err := platform.NewCasbinModel()
	if err != nil {
		return nil, fmt.Errorf("init tx casbin model: %w", err)
	}
	e, err := casbin.NewEnforcer(m, a)
	if err != nil {
		return nil, fmt.Errorf("init tx casbin enforcer: %w", err)
	}
	return e, nil
}

// Reload 触发全局 SyncedEnforcer 立即看见事务提交后的策略变更。
//
// 失败仅记录日志：StartAutoLoadPolicy 的周期轮询作为最终一致兜底，
// 这里的"立即可见"是优化而非正确性要求。
func Reload(ctx context.Context, e *casbin.SyncedEnforcer, logger *log.Logger) {
	if err := e.LoadPolicy(); err != nil {
		// 对瞬时抖动做一次短退避重试；仍失败则交给 AutoLoad 兜底。
		time.Sleep(100 * time.Millisecond)
		if err2 := e.LoadPolicy(); err2 != nil {
			logger.WithContext(ctx).Error(
				"重载 Casbin 策略失败",
				zap.NamedError("first_error", err),
				zap.NamedError("retry_error", err2),
			)
		}
	}
}
