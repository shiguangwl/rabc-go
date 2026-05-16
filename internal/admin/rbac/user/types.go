// Package user 管理管理员账户实体，并负责其与 Casbin 用户-角色绑定的生命周期一致性。
//
// 不变量：
//   - 用户 ID 即 Casbin 用户主体；删除用户必须同事务清其全部角色绑定。
//   - 跨域接口（AuthRevoker、Repo 的 GetUserRoles 等）在本包定义，由其他子域实现，
//     避免 user 反向依赖 auth。
package user

import "rabc-go/api/apiv1"

type Query struct {
	apiv1.Pagination
	Username string
	Nickname string
	Email    string
	Phone    string
}
