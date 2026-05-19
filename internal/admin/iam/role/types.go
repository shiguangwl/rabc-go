// Package role 管理角色实体，并维护其与 Casbin role 主体的生命周期一致性。
//
// 不变量：角色 sid 即 Casbin role 主体（经 model.RoleSubject 命名空间化）；
// sid 一旦创建不可变更，删除角色必须同事务先撤 Casbin 策略再删 DB 行。
package role

import "rabc-go/api/apiv1"

type Query struct {
	apiv1.Pagination
	Sid  string
	Name string
}
