package casbinkit

import (
	"github.com/casbin/casbin/v2"

	"rabc-go/internal/model"
)

// UpdateUserRolesOn 把用户角色绑定收敛到 roles 集合（diff 同步，非全量重写）。
//
// roles 为空 → 视为"清空该用户全部角色"。调用方应在持 RBACMu 的同一事务内调用，
// 否则与并发写竞争会产生中间态。
func UpdateUserRolesOn(e casbin.IEnforcer, uid string, roles []string) error {
	if len(roles) == 0 {
		_, err := e.DeleteRolesForUser(uid)
		return err
	}
	old, err := e.GetRolesForUser(uid)
	if err != nil {
		return err
	}
	oldSet := make(map[string]struct{}, len(old))
	newSet := make(map[string]struct{}, len(roles))
	for _, v := range old {
		oldSet[v] = struct{}{}
	}
	for _, v := range roles {
		newSet[model.RoleSubject(v)] = struct{}{}
	}
	var addRoles, delRoles []string
	for k := range oldSet {
		if _, ok := newSet[k]; !ok {
			delRoles = append(delRoles, k)
		}
	}
	for k := range newSet {
		if _, ok := oldSet[k]; !ok {
			addRoles = append(addRoles, k)
		}
	}
	if len(addRoles) == 0 && len(delRoles) == 0 {
		return nil
	}
	for _, role := range delRoles {
		if _, err := e.DeleteRoleForUser(uid, role); err != nil {
			return err
		}
	}
	// IEnforcer 未暴露批量 AddRolesForUser，逐条调用是接口约束而非选择。
	for _, role := range addRoles {
		if _, err := e.AddRoleForUser(uid, role); err != nil {
			return err
		}
	}
	return nil
}

// RemoveByObjectAct 清理所有匹配 (obj, act) 的策略。
//
// 必须用 RemoveFilteredPolicy 一步完成：先 GetFilteredPolicy 再 RemovePolicies
// 的两步式存在 TOCTOU 窗口，且 adapter 批删可能静默吞错。
func RemoveByObjectAct(e casbin.IEnforcer, obj, act string) error {
	_, err := e.RemoveFilteredPolicy(1, obj, act)
	return err
}
