package model

import "gorm.io/gorm"

type Menu struct {
	gorm.Model
	ParentID   uint   `json:"parentId,omitempty" gorm:"column:parent_id;index;comment:父级菜单的id，使用整数表示"`     // 父级菜单的id，使用整数表示
	Path       string `json:"path" gorm:"column:path;type:varchar(255);comment:地址"`                        // 地址
	Title      string `json:"title" gorm:"column:title;type:varchar(100);comment:标题，使用字符串表示"`              // 标题，使用字符串表示
	Name       string `json:"name,omitempty" gorm:"column:name;type:varchar(100);comment:同路由中的name，用于保活"`  // 同路由中的name，用于保活
	Component  string `json:"component,omitempty" gorm:"column:component;type:varchar(255);comment:绑定的组件"` // 绑定的组件，默认类型：Iframe、RouteView、ComponentError
	Locale     string `json:"locale,omitempty" gorm:"column:locale;type:varchar(100);comment:本地化标识"`       // 本地化标识
	Icon       string `json:"icon,omitempty" gorm:"column:icon;type:varchar(100);comment:图标，使用字符串表示"`      // 图标，使用字符串表示
	Redirect   string `json:"redirect,omitempty" gorm:"column:redirect;type:varchar(255);comment:重定向地址"`   // 重定向地址
	URL        string `json:"url,omitempty" gorm:"column:url;type:varchar(255);comment:iframe模式下的跳转url"`   // iframe模式下的跳转url，不能与path重复
	KeepAlive  bool   `json:"keepAlive,omitempty" gorm:"column:keep_alive;default:false;comment:是否保活"`     // 是否保活
	HideInMenu bool   `json:"hideInMenu,omitempty" gorm:"column:hide_in_menu;default:false;comment:是否保活"`  // 是否保活
	Target     string `json:"target,omitempty" gorm:"column:target;type:varchar(20);comment:全连接跳转模式"`      // 全连接跳转模式：'_blank'、'_self'、'_parent'
	Weight     int    `json:"weight" gorm:"column:weight;type:int;default:0;comment:排序权重"`
}

func (m *Menu) TableName() string {
	return "menu"
}
