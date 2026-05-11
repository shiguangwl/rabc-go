package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// envPrefix 是环境变量前缀。例如 security.jwt.key 会被 APP_SECURITY_JWT_KEY 覆盖。
const envPrefix = "APP"

// 环境标识
const (
	EnvProd  = "prod"
	EnvLocal = "local"
)

// envBoundKeys 列出"允许被环境变量覆盖"的配置项。
//
// 背景：AutomaticEnv 对 Get* 路径有效，但关键配置需要一份集中契约；
// 显式 BindEnv 也能覆盖后续 Unmarshal 场景，避免 yml 删行后 env 注入失效。
//
// 新增 key 时必须在此登记，触发安全审查（密钥/密码/连接串等敏感项归并管理）。
var envBoundKeys = []string{
	// 安全密钥
	"security.jwt.key",

	// 数据源
	"data.db.user.driver",
	"data.db.user.dsn",
	"data.db.debug",
	"data.redis.addr",
	"data.redis.password",

	// seed 命令控制
	"seed.initial_password",
	"seed.reset",

	// 日志：是否打印请求/响应 body（prod 默认关闭，调试时可临时 env 打开）
	"log.body.enabled",
	"log.body.max_bytes",
}

// IsProd 报告当前是否为生产环境。
func IsProd(conf *viper.Viper) bool {
	return conf.GetString("env") == EnvProd
}

// IsLocal 报告当前是否为本地开发环境。
//
// 与 !IsProd 不同：staging / dev / uat 等环境既非 prod 也非 local，
// 不应享受 local 的弱默认（如初始密码 "123456"、TRUNCATE -reset）。
// 任何"仅 local 允许"的开关都应当用本函数判定，而不是 !IsProd。
func IsLocal(conf *viper.Viper) bool {
	return conf.GetString("env") == EnvLocal
}

// prodRequiredKeys 列出 prod 环境必须提供的配置项。
// 值可以来自环境变量，也可以来自 config/prod.yml；任一缺失，启动期 panic
// 阻断部署，避免空值带病上线（例：空 JWT key 会让 HS256 用空字节签名，
// token 可被任意伪造）。
var prodRequiredKeys = []string{
	"security.jwt.key",
	"data.db.user.dsn",
}

// mustValidateProd 在 prod 环境下校验关键 secret 与 DSN 必须非空。
// 非 prod 环境直接放行，避免阻塞本地开发与单测。
func mustValidateProd(conf *viper.Viper) {
	if !IsProd(conf) {
		return
	}
	var missing []string
	for _, k := range prodRequiredKeys {
		if strings.TrimSpace(conf.GetString(k)) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		panic(fmt.Errorf("prod config missing required keys: %v", missingConfigHints(missing)))
	}
}

func envNameForKey(key string) string {
	return envPrefix + "_" + strings.ToUpper(strings.NewReplacer(".", "_").Replace(key))
}

func missingEnvNames(keys []string) []string {
	envs := make([]string, 0, len(keys))
	for _, key := range keys {
		envs = append(envs, envNameForKey(key))
	}
	return envs
}

func missingConfigHints(keys []string) []string {
	hints := make([]string, 0, len(keys))
	for _, key := range keys {
		hints = append(hints, fmt.Sprintf("%s or %s", key, envNameForKey(key)))
	}
	return hints
}

func NewConfig(p string) *viper.Viper {
	envConf := os.Getenv("APP_CONF")
	if envConf == "" {
		envConf = p
	}
	conf := getConfig(envConf)
	mustValidateProd(conf)
	return conf
}

func getConfig(path string) *viper.Viper {
	conf := viper.New()
	conf.SetConfigFile(path)
	if err := conf.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config %q: %w", path, err))
	}

	// 允许通过环境变量覆盖任意配置项；点号映射为下划线。
	// 例如：security.jwt.key → APP_SECURITY_JWT_KEY
	conf.SetEnvPrefix(envPrefix)
	conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	conf.AutomaticEnv()

	// 显式绑定关键 key，绕过 AutomaticEnv 对"yml 必须存在该 key"的限制
	for _, key := range envBoundKeys {
		if err := conf.BindEnv(key); err != nil {
			panic(fmt.Errorf("bind env for %q failed: %w", key, err))
		}
	}

	return conf
}
