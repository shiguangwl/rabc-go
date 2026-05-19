// Package api 管理 API 资源元数据，并维持其与 Casbin 策略的一致性。
//
// 不变量：API 资源以 (path, method) 为身份键；任何会改变该键的 DB 写操作
// 都必须在同一事务内按旧键清 Casbin 策略，避免授权遗留。
package api

import "rabc-go/api/apiv1"

type Query struct {
	apiv1.Pagination
	Group  string
	Name   string
	Path   string
	Method string
}
