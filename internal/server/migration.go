package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net/http"
	v1 "nunu-layout-admin/api/v1"
	"nunu-layout-admin/internal/model"
	"nunu-layout-admin/pkg/log"
	"nunu-layout-admin/pkg/sid"
	"os"
)

type MigrateServer struct {
	db  *gorm.DB
	log *log.Logger
	sid *sid.Sid
	e   *casbin.SyncedEnforcer
}

func NewMigrateServer(
	db *gorm.DB,
	log *log.Logger,
	sid *sid.Sid,
	e *casbin.SyncedEnforcer,
) *MigrateServer {
	return &MigrateServer{
		e:   e,
		db:  db,
		log: log,
		sid: sid,
	}
}
func (m *MigrateServer) Start(ctx context.Context) error {
	m.db.Migrator().DropTable(
		&model.AdminUser{},
		&model.Menu{},
		&model.Role{},
		&model.Api{},
	)
	if err := m.db.AutoMigrate(
		&model.AdminUser{},
		&model.Menu{},
		&model.Role{},
		&model.Api{},
	); err != nil {
		m.log.Error("user migrate error", zap.Error(err))
		return err
	}
	err := m.initialAdminUser(ctx)
	if err != nil {
		m.log.Error("initialAdminUser error", zap.Error(err))
	}

	err = m.initialMenuData(ctx)
	if err != nil {
		m.log.Error("initialMenuData error", zap.Error(err))
	}

	err = m.initialApisData(ctx)
	if err != nil {
		m.log.Error("initialApisData error", zap.Error(err))
	}

	err = m.initialRBAC(ctx)
	if err != nil {
		m.log.Error("initialRBAC error", zap.Error(err))
	}

	m.log.Info("AutoMigrate success")
	os.Exit(0)
	return nil
}
func (m *MigrateServer) Stop(ctx context.Context) error {
	m.log.Info("AutoMigrate stop")
	return nil
}
func (m *MigrateServer) initialAdminUser(ctx context.Context) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	err = m.db.Create(&model.AdminUser{
		Model:    gorm.Model{ID: 1},
		Username: "admin",
		Password: string(hashedPassword),
		Nickname: "Admin",
	}).Error
	return m.db.Create(&model.AdminUser{
		Model:    gorm.Model{ID: 2},
		Username: "user",
		Password: string(hashedPassword),
		Nickname: "运营人员",
	}).Error

}
func (m *MigrateServer) initialRBAC(ctx context.Context) error {
	roles := []model.Role{
		{Sid: model.AdminRole, Name: "超级管理员"},
		{Sid: "1000", Name: "运营人员"},
		{Sid: "1001", Name: "访客"},
	}
	if err := m.db.Create(&roles).Error; err != nil {
		return err
	}
	m.e.ClearPolicy()
	err := m.e.SavePolicy()
	if err != nil {
		m.log.Error("m.e.SavePolicy error", zap.Error(err))
		return err
	}
	_, err = m.e.AddRoleForUser(model.AdminUserID, model.AdminRole)
	if err != nil {
		m.log.Error("m.e.AddRoleForUser error", zap.Error(err))
		return err
	}
	menuList := make([]v1.MenuDataItem, 0)
	err = json.Unmarshal([]byte(menuData), &menuList)
	if err != nil {
		m.log.Error("json.Unmarshal error", zap.Error(err))
		return err
	}
	for _, item := range menuList {
		m.addPermissionForRole(model.AdminRole, model.MenuResourcePrefix+item.Path, "read")
	}
	apiList := make([]model.Api, 0)
	err = m.db.Find(&apiList).Error
	if err != nil {
		m.log.Error("m.db.Find(&apiList).Error error", zap.Error(err))
		return err
	}
	for _, api := range apiList {
		m.addPermissionForRole(model.AdminRole, model.ApiResourcePrefix+api.Path, api.Method)
	}

	// 添加运营人员权限
	_, err = m.e.AddRoleForUser("2", "1000")
	if err != nil {
		m.log.Error("m.e.AddRoleForUser error", zap.Error(err))
		return err
	}
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/profile/basic", "read")
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/profile/advanced", "read")
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/profile", "read")
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/dashboard", "read")
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/dashboard/workplace", "read")
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/dashboard/analysis", "read")
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/account/settings", "read")
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/account/center", "read")
	m.addPermissionForRole("1000", model.MenuResourcePrefix+"/account", "read")
	m.addPermissionForRole("1000", model.ApiResourcePrefix+"/v1/menus", http.MethodGet)
	m.addPermissionForRole("1000", model.ApiResourcePrefix+"/v1/admin/user", http.MethodGet)

	return nil
}
func (m *MigrateServer) addPermissionForRole(role, resource, action string) {
	_, err := m.e.AddPermissionForUser(role, resource, action)
	if err != nil {
		m.log.Sugar().Info("为角色 %s 添加权限 %s:%s 失败: %v", role, resource, action, err)
		return
	}
	fmt.Printf("为角色 %s 添加权限: %s %s\n", role, resource, action)
}
func (m *MigrateServer) initialApisData(ctx context.Context) error {
	initialApis := []model.Api{

		{Group: "基础API", Name: "获取用户菜单列表", Path: "/v1/menus", Method: http.MethodGet},
		{Group: "基础API", Name: "获取管理员信息", Path: "/v1/admin/user", Method: http.MethodGet},

		{Group: "菜单管理", Name: "获取管理菜单", Path: "/v1/admin/menus", Method: http.MethodGet},
		{Group: "菜单管理", Name: "创建菜单", Path: "/v1/admin/menu", Method: http.MethodPost},
		{Group: "菜单管理", Name: "更新菜单", Path: "/v1/admin/menu", Method: http.MethodPut},
		{Group: "菜单管理", Name: "删除菜单", Path: "/v1/admin/menu", Method: http.MethodDelete},

		{Group: "权限模块", Name: "获取用户权限", Path: "/v1/admin/user/permissions", Method: http.MethodGet},
		{Group: "权限模块", Name: "获取角色权限", Path: "/v1/admin/role/permissions", Method: http.MethodGet},
		{Group: "权限模块", Name: "更新角色权限", Path: "/v1/admin/role/permission", Method: http.MethodPut},
		{Group: "权限模块", Name: "获取角色列表", Path: "/v1/admin/roles", Method: http.MethodGet},
		{Group: "权限模块", Name: "创建角色", Path: "/v1/admin/role", Method: http.MethodPost},
		{Group: "权限模块", Name: "更新角色", Path: "/v1/admin/role", Method: http.MethodPut},
		{Group: "权限模块", Name: "删除角色", Path: "/v1/admin/role", Method: http.MethodDelete},

		{Group: "权限模块", Name: "获取管理员列表", Path: "/v1/admin/users", Method: http.MethodGet},
		{Group: "权限模块", Name: "更新管理员信息", Path: "/v1/admin/user", Method: http.MethodPut},
		{Group: "权限模块", Name: "创建管理员账号", Path: "/v1/admin/user", Method: http.MethodPost},
		{Group: "权限模块", Name: "删除管理员", Path: "/v1/admin/user", Method: http.MethodDelete},

		{Group: "权限模块", Name: "获取API列表", Path: "/v1/admin/apis", Method: http.MethodGet},
		{Group: "权限模块", Name: "创建API", Path: "/v1/admin/api", Method: http.MethodPost},
		{Group: "权限模块", Name: "更新API", Path: "/v1/admin/api", Method: http.MethodPut},
		{Group: "权限模块", Name: "删除API", Path: "/v1/admin/api", Method: http.MethodDelete},
	}

	return m.db.Create(&initialApis).Error
}
func (m *MigrateServer) initialMenuData(ctx context.Context) error {
	menuList := make([]v1.MenuDataItem, 0)
	err := json.Unmarshal([]byte(menuData), &menuList)
	if err != nil {
		m.log.Error("json.Unmarshal error", zap.Error(err))
		return err
	}
	menuListDb := make([]model.Menu, 0)
	for _, item := range menuList {
		menuListDb = append(menuListDb, model.Menu{
			Model: gorm.Model{
				ID: item.ID,
			},
			ParentID:   item.ParentID,
			Path:       item.Path,
			Title:      item.Title,
			Name:       item.Name,
			Component:  item.Component,
			Locale:     item.Locale,
			Weight:     item.Weight,
			Icon:       item.Icon,
			Redirect:   item.Redirect,
			URL:        item.URL,
			KeepAlive:  item.KeepAlive,
			HideInMenu: item.HideInMenu,
		})
	}
	return m.db.Create(&menuListDb).Error
}

var menuData = `[
 {
    "id": 18,
    "parentId": 15,
    "path": "/access/admin",
    "title": "管理员账号",
    "name": "accessAdmin",
    "component": "/access/admin",
    "locale": "menu.access.admin"
  },
  {
    "id": 2,
    "parentId": 0,
    "title": "分析页",
    "icon": "DashboardOutlined",
    "component": "/dashboard/analysis",
    "path": "/dashboard/analysis",
    "name": "DashboardAnalysis",
    "keepAlive": true,
    "locale": "menu.dashboard.analysis",
    "weight": 2
  },
  {
    "id": 1,
    "parentId": 0,
    "title": "仪表盘",
    "icon": "DashboardOutlined",
    "component": "RouteView",
    "redirect": "/dashboard/analysis",
    "path": "/dashboard",
    "name": "Dashboard",
    "locale": "menu.dashboard"
  },
  {
    "id": 3,
    "parentId": 0,
    "title": "表单页",
    "icon": "FormOutlined",
    "component": "RouteView",
    "redirect": "/form/basic",
    "path": "/form",
    "name": "Form",
    "locale": "menu.form"
  },
  {
    "id": 5,
    "parentId": 0,
    "title": "链接",
    "icon": "LinkOutlined",
    "component": "RouteView",
    "redirect": "/link/iframe",
    "path": "/link",
    "name": "Link",
    "locale": "menu.link"
  },
  {
    "id": 6,
    "parentId": 5,
    "title": "AntDesign",
    "url": "https://ant.design/",
    "component": "Iframe",
    "path": "/link/iframe",
    "name": "LinkIframe",
    "keepAlive": true,
    "locale": "menu.link.iframe"
  },
  {
    "id": 7,
    "parentId": 5,
    "title": "AntDesignVue",
    "url": "https://antdv.com/",
    "component": "Iframe",
    "path": "/link/antdv",
    "name": "LinkAntdv",
    "keepAlive": true,
    "locale": "menu.link.antdv"
  },
  {
    "id": 8,
    "parentId": 5,
    "path": "https://www.baidu.com",
    "name": "LinkExternal",
    "title": "跳转百度",
    "locale": "menu.link.external"
  },
  {
    "id": 9,
    "parentId": 0,
    "title": "菜单",
    "icon": "BarsOutlined",
    "component": "RouteView",
    "path": "/menu",
    "redirect": "/menu/menu1",
    "name": "Menu",
    "locale": "menu.menu"
  },
  {
    "id": 10,
    "parentId": 9,
    "title": "菜单1",
    "component": "/menu/menu1",
    "path": "/menu/menu1",
    "name": "MenuMenu11",
    "keepAlive": true,
    "locale": "menu.menu.menu1"
  },
  {
    "id": 11,
    "parentId": 9,
    "title": "菜单2",
    "component": "/menu/menu2",
    "path": "/menu/menu2",
    "keepAlive": true,
    "locale": "menu.menu.menu2"
  },
  {
    "id": 12,
    "parentId": 9,
    "path": "/menu/menu3",
    "redirect": "/menu/menu3/menu1",
    "title": "菜单1-1",
    "component": "RouteView",
    "locale": "menu.menu.menu3"
  },
  {
    "id": 13,
    "parentId": 12,
    "path": "/menu/menu3/menu1",
    "component": "/menu/menu-1-1/menu1",
    "title": "菜单1-1-1",
    "keepAlive": true,
    "locale": "menu.menu3.menu1"
  },
  {
    "id": 14,
    "parentId": 12,
    "path": "/menu/menu3/menu2",
    "component": "/menu/menu-1-1/menu2",
    "title": "菜单1-1-2",
    "keepAlive": true,
    "locale": "menu.menu3.menu2"
  },
  {
    "id": 15,
    "path": "/access",
    "component": "RouteView",
    "redirect": "/access/common",
    "title": "权限模块",
    "name": "Access",
    "parentId": 0,
    "icon": "ClusterOutlined",
    "locale": "menu.access",
    "weight": 1
  },
  {
    "id": 51,
    "parentId": 15,
    "path": "/access/role",	
    "title": "角色管理",
    "name": "AccessRoles",
    "component": "/access/role",
    "locale": "menu.access.roles"
  },
{
    "id": 52,
    "parentId": 15,
    "path": "/access/menu",	
    "title": "菜单管理",
    "name": "AccessMenu",
    "component": "/access/menu",
    "locale": "menu.access.menus"
  },
{
    "id": 53,
    "parentId": 15,
    "path": "/access/api",	
    "title": "API管理",
    "name": "AccessAPI",
    "component": "/access/api",
    "locale": "menu.access.api"
  },
  {
    "id": 19,
    "parentId": 0,
    "title": "异常页",
    "icon": "WarningOutlined",
    "component": "RouteView",
    "redirect": "/exception/403",
    "path": "/exception",
    "name": "Exception",
    "locale": "menu.exception"
  },
  {
    "id": 20,
    "parentId": 19,
    "path": "/exception/403",
    "title": "403",
    "name": "403",
    "component": "/exception/403",
    "locale": "menu.exception.not-permission"
  },
  {
    "id": 21,
    "parentId": 19,
    "path": "/exception/404",
    "title": "404",
    "name": "404",
    "component": "/exception/404",
    "locale": "menu.exception.not-find"
  },
  {
    "id": 22,
    "parentId": 19,
    "path": "/exception/500",
    "title": "500",
    "name": "500",
    "component": "/exception/500",
    "locale": "menu.exception.server-error"
  },
  {
    "id": 23,
    "parentId": 0,
    "title": "结果页",
    "icon": "CheckCircleOutlined",
    "component": "RouteView",
    "redirect": "/result/success",
    "path": "/result",
    "name": "Result",
    "locale": "menu.result"
  },
  {
    "id": 24,
    "parentId": 23,
    "path": "/result/success",
    "title": "成功页",
    "name": "ResultSuccess",
    "component": "/result/success",
    "locale": "menu.result.success"
  },
  {
    "id": 25,
    "parentId": 23,
    "path": "/result/fail",
    "title": "失败页",
    "name": "ResultFail",
    "component": "/result/fail",
    "locale": "menu.result.fail"
  },
  {
    "id": 26,
    "parentId": 0,
    "title": "列表页",
    "icon": "TableOutlined",
    "component": "RouteView",
    "redirect": "/list/card-list",
    "path": "/list",
    "name": "List",
    "locale": "menu.list"
  },
  {
    "id": 27,
    "parentId": 26,
    "path": "/list/card-list",
    "title": "卡片列表",
    "name": "ListCard",
    "component": "/list/card-list",
    "locale": "menu.list.card-list"
  },
  {
    "id": 28,
    "parentId": 0,
    "title": "详情页",
    "icon": "ProfileOutlined",
    "component": "RouteView",
    "redirect": "/profile/basic",
    "path": "/profile",
    "name": "Profile",
    "locale": "menu.profile"
  },
  {
    "id": 29,
    "parentId": 28,
    "path": "/profile/basic",
    "title": "基础详情页",
    "name": "ProfileBasic",
    "component": "/profile/basic/index",
    "locale": "menu.profile.basic"
  },
  {
    "id": 30,
    "parentId": 26,
    "path": "/list/search-list",
    "title": "搜索列表",
    "name": "SearchList",
    "component": "/list/search-list",
    "locale": "menu.list.search-list"
  },
  {
    "id": 31,
    "parentId": 30,
    "path": "/list/search-list/articles",
    "title": "搜索列表（文章）",
    "name": "SearchListArticles",
    "component": "/list/search-list/articles",
    "locale": "menu.list.search-list.articles"
  },
  {
    "id": 32,
    "parentId": 30,
    "path": "/list/search-list/projects",
    "title": "搜索列表（项目）",
    "name": "SearchListProjects",
    "component": "/list/search-list/projects",
    "locale": "menu.list.search-list.projects"
  },
  {
    "id": 33,
    "parentId": 30,
    "path": "/list/search-list/applications",
    "title": "搜索列表（应用）",
    "name": "SearchListApplications",
    "component": "/list/search-list/applications",
    "locale": "menu.list.search-list.applications"
  },
  {
    "id": 34,
    "parentId": 26,
    "path": "/list/basic-list",
    "title": "标准列表",
    "name": "BasicCard",
    "component": "/list/basic-list",
    "locale": "menu.list.basic-list"
  },
  {
    "id": 35,
    "parentId": 28,
    "path": "/profile/advanced",
    "title": "高级详细页",
    "name": "ProfileAdvanced",
    "component": "/profile/advanced/index",
    "locale": "menu.profile.advanced"
  },
  {
    "id": 4,
    "parentId": 3,
    "title": "基础表单",
    "component": "/form/basic-form/index",
    "path": "/form/basic-form",
    "name": "FormBasic",
    "keepAlive": false,
    "locale": "menu.form.basic-form"
  },
  {
    "id": 36,
    "parentId": 0,
    "title": "个人页",
    "icon": "UserOutlined",
    "component": "RouteView",
    "redirect": "/account/center",
    "path": "/account",
    "name": "Account",
    "locale": "menu.account"
  },
  {
    "id": 37,
    "parentId": 36,
    "path": "/account/center",
    "title": "个人中心",
    "name": "AccountCenter",
    "component": "/account/center",
    "locale": "menu.account.center"
  },
  {
    "id": 38,
    "parentId": 36,
    "path": "/account/settings",
    "title": "个人设置",
    "name": "AccountSettings",
    "component": "/account/settings",
    "locale": "menu.account.settings"
  },
  {
    "id": 39,
    "parentId": 3,
    "title": "分步表单",
    "component": "/form/step-form/index",
    "path": "/form/step-form",
    "name": "FormStep",
    "keepAlive": false,
    "locale": "menu.form.step-form"
  },
  {
    "id": 40,
    "parentId": 3,
    "title": "高级表单",
    "component": "/form/advanced-form/index",
    "path": "/form/advanced-form",
    "name": "FormAdvanced",
    "keepAlive": false,
    "locale": "menu.form.advanced-form"
  },
  {
    "id": 41,
    "parentId": 26,
    "path": "/list/table-list",
    "title": "查询表格",
    "name": "ConsultTable",
    "component": "/list/table-list",
    "locale": "menu.list.consult-table"
  },
  {
    "id": 42,
    "parentId": 1,
    "title": "监控页",
    "component": "/dashboard/monitor",
    "path": "/dashboard/monitor",
    "name": "DashboardMonitor",
    "keepAlive": true,
    "locale": "menu.dashboard.monitor"
  },
  {
    "id": 43,
    "parentId": 1,
    "title": "工作台",
    "component": "/dashboard/workplace",
    "path": "/dashboard/workplace",
    "name": "DashboardWorkplace",
    "keepAlive": true,
    "locale": "menu.dashboard.workplace"
  },
  {
    "id": 44,
    "parentId": 26,
    "path": "/list/crud-table",
    "title": "增删改查表格",
    "name": "CrudTable",
    "component": "/list/crud-table",
    "locale": "menu.list.crud-table"
  },
  {
    "id": 45,
    "parentId": 9,
    "path": "/menu/menu4",
    "redirect": "/menu/menu4/menu1",
    "title": "菜单2-1",
    "component": "RouteView",
    "locale": "menu.menu.menu4"
  },
  {
    "id": 46,
    "parentId": 45,
    "path": "/menu/menu4/menu1",
    "component": "/menu/menu-2-1/menu1",
    "title": "菜单2-1-1",
    "keepAlive": true,
    "locale": "menu.menu4.menu1"
  },
  {
    "id": 47,
    "parentId": 45,
    "path": "/menu/menu4/menu2",
    "component": "/menu/menu-2-1/menu2",
    "title": "菜单2-1-2",
    "keepAlive": true,
    "locale": "menu.menu4.menu2"
  }
]`
