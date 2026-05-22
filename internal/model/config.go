package model

import "gorm.io/gorm"

// 配置值类型：决定前端渲染控件，以及后端取值时的解析方式。
const (
	ConfigTypeString = "string"
	ConfigTypeInt    = "int"
	ConfigTypeBool   = "bool"
	ConfigTypeJSON   = "json"
)

// SysConfig 运行时系统配置项，管理员在后台增改、应用无需重启即可读到新值。
//
// 与文件配置的边界：数据库连接、JWT 密钥等启动期且敏感的配置走 config/*.yml +
// pkg/config；本表只存站点信息、功能开关等运行期业务参数。
//
// 不变量：
//   - 严禁存放密钥 / 令牌等敏感值——IsPublic 项会经免鉴权接口下发到前端。
//   - ConfigKey 一旦被代码引用即为稳定契约，不可改名；改展示文案改 Title。
//   - IsSystem=true 的内置项禁止删除，且不可改 Key/ValueType/Group/IsPublic 等
//     元数据，仅 ConfigValue 可改。
type SysConfig struct {
	gorm.Model
	ConfigKey   string `json:"configKey" gorm:"column:config_key;type:varchar(128);not null;uniqueIndex;comment:配置键，程序读取的稳定标识"`
	ConfigValue string `json:"configValue" gorm:"column:config_value;type:text;not null;comment:配置值，统一以字符串存储"`
	ValueType   string `json:"valueType" gorm:"column:value_type;type:varchar(16);not null;default:string;comment:值类型：string/int/bool/json"`
	ConfigGroup string `json:"configGroup" gorm:"column:config_group;type:varchar(64);not null;index;comment:配置分组，决定前端 Tab"`
	Title       string `json:"title" gorm:"column:title;type:varchar(128);not null;comment:展示名称"`
	Remark      string `json:"remark" gorm:"column:remark;type:varchar(255);not null;comment:配置说明"`
	IsPublic    bool   `json:"isPublic" gorm:"column:is_public;type:boolean;not null;default:false;comment:是否允许未登录访问"`
	IsSystem    bool   `json:"isSystem" gorm:"column:is_system;type:boolean;not null;default:false;comment:内置配置，禁止删除与改元数据"`
	Weight      int    `json:"weight" gorm:"column:weight;type:int;not null;default:0;comment:组内排序权重，越大越靠前"`
}

func (*SysConfig) TableName() string {
	return "sys_config"
}
