package apiv1

// SysConfigItem 管理端配置项的完整视图，含元数据。
type SysConfigItem struct {
	ID          uint   `json:"id"`
	ConfigKey   string `json:"configKey"`
	ConfigValue string `json:"configValue"`
	ValueType   string `json:"valueType"`
	ConfigGroup string `json:"configGroup"`
	Title       string `json:"title"`
	Remark      string `json:"remark"`
	IsPublic    bool   `json:"isPublic"`
	IsSystem    bool   `json:"isSystem"`
	Weight      int    `json:"weight"`
	UpdatedAt   string `json:"updatedAt"`
}

// ConfigGroupItem 按分组聚合的配置集合，对应前端一个 Tab。
type ConfigGroupItem struct {
	Group string          `json:"group"`
	Items []SysConfigItem `json:"items"`
}

type GetConfigsResponseData struct {
	Groups []ConfigGroupItem `json:"groups"`
}
type GetConfigsResponse struct {
	Response
	Data GetConfigsResponseData
}

// PublicConfigItem 是公开配置的最小投影。
//
// 刻意不复用 SysConfigItem：免鉴权接口不得下发 remark/isSystem/时间戳等内部信息。
type PublicConfigItem struct {
	ConfigKey   string `json:"configKey"`
	ConfigValue string `json:"configValue"`
	ValueType   string `json:"valueType"`
}
type GetPublicConfigsResponseData struct {
	List []PublicConfigItem `json:"list"`
}
type GetPublicConfigsResponse struct {
	Response
	Data GetPublicConfigsResponseData
}

type ConfigCreateRequest struct {
	ConfigKey   string `json:"configKey" binding:"required,max=128" example:"site.name"`
	ConfigValue string `json:"configValue" example:"Nunu Admin"`
	ValueType   string `json:"valueType" binding:"required,oneof=string int bool json" example:"string"`
	ConfigGroup string `json:"configGroup" binding:"required,max=64" example:"站点设置"`
	Title       string `json:"title" binding:"required,max=128" example:"站点名称"`
	Remark      string `json:"remark" binding:"omitempty,max=255" example:""`
	IsPublic    bool   `json:"isPublic" example:"false"`
	Weight      int    `json:"weight" example:"0"`
}

// ConfigUpdateItem 批量更新中的单项；只允许改值，元数据变更走删除后重建。
type ConfigUpdateItem struct {
	ConfigKey   string `json:"configKey" binding:"required,max=128" example:"site.name"`
	ConfigValue string `json:"configValue" example:"Nunu Admin"`
}

// BatchUpdateConfigRequest 批量改值，按 Tab 一次提交。整批全校验通过才落库。
//
// max=100 限制单批规模，避免异常请求生成过大的 IN 查询与事务循环。
type BatchUpdateConfigRequest struct {
	List []ConfigUpdateItem `json:"list" binding:"required,min=1,max=100,dive"`
}

type ConfigDeleteRequest struct {
	ID uint `form:"id" binding:"required" example:"1"`
}
