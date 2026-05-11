package model

import "gorm.io/gorm"

type Menu struct {
	gorm.Model
	ParentID   uint   `json:"parentId,omitempty" gorm:"column:parent_id;index;comment:父级菜单id，0 表示根菜单"`
	Path       string `json:"path" gorm:"column:path;type:varchar(255);comment:前端路由路径"`
	Title      string `json:"title" gorm:"column:title;type:varchar(100);comment:菜单显示标题"`
	Name       string `json:"name,omitempty" gorm:"column:name;type:varchar(100);comment:路由唯一标识，对应前端路由 name"`
	Component  string `json:"component,omitempty" gorm:"column:component;type:varchar(255);comment:绑定组件，常用：Iframe/RouteView/ComponentError"`
	Locale     string `json:"locale,omitempty" gorm:"column:locale;type:varchar(100);comment:i18n key"`
	Icon       string `json:"icon,omitempty" gorm:"column:icon;type:varchar(100);comment:图标"`
	Redirect   string `json:"redirect,omitempty" gorm:"column:redirect;type:varchar(255);comment:重定向地址"`
	URL        string `json:"url,omitempty" gorm:"column:url;type:varchar(255);comment:iframe 模式下的跳转 URL"`
	KeepAlive  bool   `json:"keepAlive,omitempty" gorm:"column:keep_alive;default:false;comment:是否保活页面状态"`
	HideInMenu bool   `json:"hideInMenu,omitempty" gorm:"column:hide_in_menu;default:false;comment:是否在菜单中隐藏"`
	Target     string `json:"target,omitempty" gorm:"column:target;type:varchar(20);comment:链接打开方式：_blank/_self/_parent"`
	Weight     int    `json:"weight" gorm:"column:weight;type:int;default:0;comment:排序权重，越大越靠前"`
}

func (m *Menu) TableName() string {
	return "menu"
}
