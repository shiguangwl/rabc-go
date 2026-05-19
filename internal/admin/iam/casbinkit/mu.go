// Package casbinkit 提供 RBAC 子域共享的 Casbin 编排原语：进程级互斥、
// 事务绑定 enforcer、策略 diff 同步、引用完整性校验。
//
// 依赖边界：本包仅依赖 internal/model + internal/platform + api/apiv1，
// 不得反向 import user / role / menu / api / permission 子域。
package casbinkit

import "sync"

// RBACMu 是进程内 RBAC 复合写串行化的互斥锁，wire 单例注入各子域 repo。
//
// 不变量：所有"业务表 + Casbin 策略"的复合写必须持锁执行。绕开本锁会出现两类问题：
//   - 全局 SyncedEnforcer 的 StartAutoLoadPolicy 轮询可能在 tx 提交后、Reload 前
//     把旧策略加载回内存，破坏"写完立即可见"；
//   - 临时 tx-bound enforcer 与全局 enforcer 各持有独立 model 副本，并发写会
//     产生策略快照漂移。
//
// 适用范围：单进程内串行。多副本部署必须额外引入 DB advisory lock，否则两台
// 实例同时写仍会重现上述漂移。
type RBACMu struct {
	sync.Mutex
}

func NewRBACMu() *RBACMu { return &RBACMu{} }
