package model

import (
	"strings"

	"gorm.io/gorm"
)

const (
	AdminRole          = "admin"
	AdminUserID        = "1" // casbin policy subject 必须为 string，统一在此声明
	RoleSubjectPrefix  = "role:"
	MenuResourcePrefix = "menu:"
	ApiResourcePrefix  = "api:"
	PermSep            = ","
)

// RoleSubject 返回 Casbin 内部使用的角色 subject，避免角色 sid 与用户 ID 共用命名空间。
func RoleSubject(sid string) string {
	if sid == "" || strings.HasPrefix(sid, RoleSubjectPrefix) {
		return sid
	}
	return RoleSubjectPrefix + sid
}

// RoleSID 把 Casbin 内部角色 subject 转回对外 API 暴露的角色 sid。
func RoleSID(subject string) string {
	return strings.TrimPrefix(subject, RoleSubjectPrefix)
}

type AdminUser struct {
	gorm.Model
	Username string `gorm:"type:varchar(50);not null;uniqueIndex;comment:用户名"`
	Nickname string `gorm:"type:varchar(50);not null;comment:昵称"`
	Password string `gorm:"type:varchar(255);not null;comment:密码"`
	Email    string `gorm:"type:varchar(100);not null;comment:电子邮件"`
	Phone    string `gorm:"type:varchar(20);not null;comment:手机号"`
}

func (m *AdminUser) TableName() string {
	return "admin_users"
}

type Role struct {
	gorm.Model
	Name string `json:"name" gorm:"column:name;type:varchar(100);uniqueIndex;comment:角色名"`
	Sid  string `json:"sid" gorm:"column:sid;type:varchar(100);uniqueIndex;comment:角色标识"`
}

func (m *Role) TableName() string {
	return "roles"
}

type Api struct {
	gorm.Model
	// group 是 SQL 保留字，落库使用 group_name，避免多方言查询依赖手写引用符。
	Group  string `gorm:"column:group_name;type:varchar(100);not null;comment:API分组"`
	Name   string `gorm:"type:varchar(100);not null;comment:API名称"`
	Path   string `gorm:"type:varchar(255);not null;uniqueIndex:idx_api_path_method,priority:1;comment:API路径"`
	Method string `gorm:"type:varchar(20);not null;uniqueIndex:idx_api_path_method,priority:2;comment:HTTP方法"`
}

func (m *Api) TableName() string {
	return "api"
}
