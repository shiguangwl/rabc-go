package v1

type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"1234@gmail.com"`
	Password string `json:"password" binding:"required" example:"123456"`
}
type LoginResponseData struct {
	AccessToken string `json:"accessToken"`
}
type LoginResponse struct {
	Response
	Data LoginResponseData
}

type AdminUserDataItem struct {
	ID        uint     `json:"id"`
	Username  string   `json:"username" example:"张三"`
	Nickname  string   `json:"nickname" example:"小Baby"`
	Email     string   `json:"email" example:"1234@gmail.com"`
	Phone     string   `json:"phone" example:"1858888888"`
	Roles     []string `json:"roles" example:""`
	UpdatedAt string   `json:"updatedAt"`
	CreatedAt string   `json:"createdAt"`
}
type GetAdminUsersRequest struct {
	Pagination
	Username string `form:"username" example:"张三"`
	Nickname string `form:"nickname" example:"小Baby"`
	Phone    string `form:"phone" example:"1858888888"`
	Email    string `form:"email" example:"1234@gmail.com"`
}
type GetAdminUserResponseData struct {
	ID        uint     `json:"id"`
	Username  string   `json:"username" example:"张三"`
	Nickname  string   `json:"nickname" example:"小Baby"`
	Email     string   `json:"email" example:"1234@gmail.com"`
	Phone     string   `json:"phone" example:"1858888888"`
	Roles     []string `json:"roles" example:""`
	UpdatedAt string   `json:"updatedAt"`
	CreatedAt string   `json:"createdAt"`
}
type GetAdminUserResponse struct {
	Response
	Data GetAdminUserResponseData
}
type GetAdminUsersResponseData struct {
	List  []AdminUserDataItem `json:"list"`
	Total int64               `json:"total"`
}
type GetAdminUsersResponse struct {
	Response
	Data GetAdminUsersResponseData
}
type AdminUserCreateRequest struct {
	Username string   `json:"username" binding:"required" example:"张三"`
	Nickname string   `json:"nickname" example:"小Baby"`
	Password string   `json:"password" binding:"required,min=6" example:"123456"`
	Email    string   `json:"email" binding:"omitempty,email" example:"1234@gmail.com"`
	Phone    string   `json:"phone" example:"1858888888"`
	Roles    []string `json:"roles" binding:"omitempty,max=20,dive,max=64" example:""`
}

// AdminUserUpdateRequest 中的 Roles 用 *[]string 而非 []string：
// 切片无法区分"未传字段"与"显式空数组"，会让漏传 roles 的请求误清空已有角色绑定。
// 指针语义：nil = 不动角色；非 nil = 全量同步到该列表（含传空数组明确清空）。
type AdminUserUpdateRequest struct {
	ID       uint      `json:"id" binding:"required" example:"1"`
	Username string    `json:"username" binding:"required" example:"张三"`
	Nickname string    `json:"nickname" example:"小Baby"`
	Password string    `json:"password" binding:"omitempty,min=6" example:"123456"`
	Email    string    `json:"email" binding:"omitempty,email" example:"1234@gmail.com"`
	Phone    string    `json:"phone" example:"1858888888"`
	Roles    *[]string `json:"roles" example:""`
}
type AdminUserDeleteRequest struct {
	ID uint `form:"id" binding:"required" example:"1"`
}

type MenuDataItem struct {
	ID         uint   `json:"id,omitempty"`         // 唯一id，使用整数表示
	ParentID   uint   `json:"parentId,omitempty"`   // 父级菜单的id，使用整数表示
	Weight     int    `json:"weight"`               // 排序权重
	Path       string `json:"path"`                 // 地址
	Title      string `json:"title"`                // 展示名称
	Name       string `json:"name,omitempty"`       // 同路由中的name，唯一标识
	Component  string `json:"component,omitempty"`  // 绑定的组件
	Locale     string `json:"locale,omitempty"`     // 本地化标识
	Icon       string `json:"icon,omitempty"`       // 图标，使用字符串表示
	Redirect   string `json:"redirect,omitempty"`   // 重定向地址
	KeepAlive  bool   `json:"keepAlive,omitempty"`  // 是否保活
	HideInMenu bool   `json:"hideInMenu,omitempty"` // 菜单是否隐藏
	URL        string `json:"url,omitempty"`        // iframe模式下的跳转url，不能与path重复
	UpdatedAt  string `json:"updatedAt,omitempty"`  // 更新时间
}
type GetMenuResponseData struct {
	List []MenuDataItem `json:"list"`
}

type GetMenuResponse struct {
	Response
	Data GetMenuResponseData
}

type MenuCreateRequest struct {
	ParentID   uint   `json:"parentId,omitempty"`       // 父级菜单的id，使用整数表示
	Weight     int    `json:"weight"`                   // 排序权重
	Path       string `json:"path" binding:"required"`  // 地址
	Title      string `json:"title" binding:"required"` // 展示名称
	Name       string `json:"name" binding:"required"`  // 同路由中的name，唯一标识
	Component  string `json:"component,omitempty"`      // 绑定的组件
	Locale     string `json:"locale,omitempty"`         // 本地化标识
	Icon       string `json:"icon,omitempty"`           // 图标，使用字符串表示
	Redirect   string `json:"redirect,omitempty"`       // 重定向地址
	KeepAlive  bool   `json:"keepAlive,omitempty"`      // 是否保活
	HideInMenu bool   `json:"hideInMenu,omitempty"`     // 菜单是否隐藏
	URL        string `json:"url,omitempty"`            // iframe模式下的跳转url，不能与path重复

}
type MenuUpdateRequest struct {
	ID         uint   `json:"id" binding:"required"`    // 唯一id，使用整数表示
	ParentID   uint   `json:"parentId,omitempty"`       // 父级菜单的id，使用整数表示
	Weight     int    `json:"weight"`                   // 排序权重
	Path       string `json:"path" binding:"required"`  // 地址
	Title      string `json:"title" binding:"required"` // 展示名称
	Name       string `json:"name" binding:"required"`  // 同路由中的name，唯一标识
	Component  string `json:"component,omitempty"`      // 绑定的组件
	Locale     string `json:"locale,omitempty"`         // 本地化标识
	Icon       string `json:"icon,omitempty"`           // 图标，使用字符串表示
	Redirect   string `json:"redirect,omitempty"`       // 重定向地址
	KeepAlive  bool   `json:"keepAlive,omitempty"`      // 是否保活
	HideInMenu bool   `json:"hideInMenu,omitempty"`     // 菜单是否隐藏
	URL        string `json:"url,omitempty"`            // iframe模式下的跳转url，不能与path重复
	UpdatedAt  string `json:"updatedAt"`
}
type MenuDeleteRequest struct {
	ID uint `form:"id" binding:"required"` // 唯一id，使用整数表示
}
type GetRoleListRequest struct {
	Pagination
	Sid  string `form:"sid" example:"1"`
	Name string `form:"name" example:"Admin"`
}
type RoleDataItem struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Sid       string `json:"sid"`
	UpdatedAt string `json:"updatedAt"`
	CreatedAt string `json:"createdAt"`
}
type GetRolesResponseData struct {
	List  []RoleDataItem `json:"list"`
	Total int64          `json:"total"`
}
type GetRolesResponse struct {
	Response
	Data GetRolesResponseData
}
type RoleCreateRequest struct {
	Sid  string `json:"sid" binding:"required" example:"1"`
	Name string `json:"name" binding:"required" example:"Admin"`
}
type RoleUpdateRequest struct {
	ID   uint   `json:"id" binding:"required" example:"1"`
	Name string `json:"name" binding:"required" example:"Admin"`
}
type RoleDeleteRequest struct {
	ID uint `form:"id" binding:"required" example:"1"`
}
type GetApisRequest struct {
	Pagination
	Group  string `form:"group" example:"权限管理"`
	Name   string `form:"name" example:"菜单列表"`
	Path   string `form:"path" example:"/v1/test"`
	Method string `form:"method" example:"GET"`
}
type ApiDataItem struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Method    string `json:"method"`
	Group     string `json:"group"`
	UpdatedAt string `json:"updatedAt"`
	CreatedAt string `json:"createdAt"`
}
type GetApisResponseData struct {
	List   []ApiDataItem `json:"list"`
	Total  int64         `json:"total"`
	Groups []string      `json:"groups"`
}
type GetApisResponse struct {
	Response
	Data GetApisResponseData
}
type ApiCreateRequest struct {
	Group  string `json:"group" binding:"required" example:"权限管理"`
	Name   string `json:"name" binding:"required" example:"菜单列表"`
	Path   string `json:"path" binding:"required,startswith=/" example:"/v1/test"`
	Method string `json:"method" binding:"required,oneof=GET POST PUT PATCH DELETE OPTIONS HEAD" example:"GET"`
}
type ApiUpdateRequest struct {
	ID     uint   `json:"id" binding:"required" example:"1"`
	Group  string `json:"group" binding:"required" example:"权限管理"`
	Name   string `json:"name" binding:"required" example:"菜单列表"`
	Path   string `json:"path" binding:"required,startswith=/" example:"/v1/test"`
	Method string `json:"method" binding:"required,oneof=GET POST PUT PATCH DELETE OPTIONS HEAD" example:"GET"`
}
type ApiDeleteRequest struct {
	ID uint `form:"id" binding:"required" example:"1"`
}
type GetUserPermissionsData struct {
	List []string `json:"list"`
}
type GetRolePermissionsRequest struct {
	Role string `form:"role" binding:"required" example:"admin"`
}
type GetRolePermissionsData struct {
	List []string `json:"list"`
}
type UpdateRolePermissionRequest struct {
	Role string   `json:"role" binding:"required" example:"admin"`
	List []string `json:"list" binding:"required" example:""`
}
