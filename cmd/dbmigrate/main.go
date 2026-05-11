// Package main 是 atlas migrate 子命令的薄包装层。
//
// 设计：dbmigrate 不直接接触 schema，仅做"读配置 → 拼 atlas argv → exec atlas"
// 三件事。schema 真相源在 internal/model 与 db/atlas/main.go；migration 文件
// 在 db/migrations/{mysql,postgres}/。
//
// 支持的 atlas action：
//
//	diff      生成新版本化 SQL（需 -name <语义命名>）
//	apply     向目标库应用待执行 migration（需 DSN）
//	status    查看 migration 状态（需 DSN）
//	hash      重算 atlas.sum（手改 SQL/合并冲突后用）
//	validate  校验 migration 目录完整性（CI 用，无需 DSN）
//	lint      CI 级破坏性变更检测（drop col / add NOT NULL 等）；
//	          默认 --latest 1，可用 -base origin/main 改 git diff 模式
//	push      drizzle 风格：直接把 GORM struct 同步到 DB 不留 SQL 文件，仅供
//	          本地快速迭代（强制 DSN 指向 localhost，非 local 拒绝执行）
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

const envPrefix = "APP"

// actionSpec 描述每个 action 对配置/参数的需求，集中在一处便于扩展，
// 避免 if/else 链散落在 run() 里。
type actionSpec struct {
	requiresName  bool // diff
	requiresDSN   bool // apply / status / push
	requiresLocal bool // push 安全栅栏：拒绝非本地 DSN，避免误推 staging/prod
}

// actions 为只读查表；新增 action 同步更新 buildAtlasArgs 即可。
var actions = map[string]actionSpec{
	"diff":     {requiresName: true},
	"apply":    {requiresDSN: true},
	"status":   {requiresDSN: true},
	"hash":     {},
	"validate": {},
	"lint":     {},
	"push":     {requiresDSN: true, requiresLocal: true},
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("dbmigrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	confPath := flags.String("conf", configPath(), "config path")
	name := flags.String("name", "", "migration name for diff (e.g. add_user_email)")
	base := flags.String("base", "", "git base ref for lint (e.g. origin/main); default uses --latest 1")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if flags.NArg() != 1 {
		return errors.New("usage: dbmigrate [-conf config/local.yml] [-name X] [-base ref] " +
			"<diff|apply|status|hash|validate|lint|push>")
	}

	action := flags.Arg(0)
	spec, ok := actions[action]
	if !ok {
		return fmt.Errorf("unsupported migrate command %q", action)
	}
	if spec.requiresName && strings.TrimSpace(*name) == "" {
		return errors.New("migration name is required: -name <value>")
	}

	conf, err := readConfig(*confPath)
	if err != nil {
		return fmt.Errorf("read config %q: %w", *confPath, err)
	}
	driver := conf.GetString("data.db.user.driver")
	dsn := strings.TrimSpace(conf.GetString("data.db.user.dsn"))
	dialect, err := normalizeDialect(driver)
	if err != nil {
		return fmt.Errorf("normalize dialect: %w", err)
	}
	if spec.requiresDSN && dsn == "" {
		return fmt.Errorf("data.db.user.dsn is required for migrate %s", action)
	}
	if spec.requiresLocal {
		local, err := isLocalDSN(dialect, dsn)
		if err != nil {
			return fmt.Errorf("check local dsn: %w", err)
		}
		if !local {
			return errors.New("push 仅限本地开发：DSN 必须指向 localhost / 127.0.0.1 / ::1 或 unix socket；" +
				"非本地环境请走 migrate-diff + migrate-apply 版本化流程")
		}
	}

	argv := buildAtlasArgs(action, dialect, *name, *base)
	if spec.requiresDSN {
		atlasURL, err := atlasURL(dialect, dsn)
		if err != nil {
			return fmt.Errorf("build atlas url: %w", err)
		}
		argv = append(argv, "--url", atlasURL)
	}

	cmd := exec.Command("atlas", argv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "ATLAS_DIALECT="+dialect)
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("atlas CLI 未安装或不在 PATH，请先安装 Atlas")
		}
		return fmt.Errorf("run atlas %s: %w", action, err)
	}
	return nil
}

// buildAtlasArgs 把 dbmigrate 的 action 翻译成 atlas CLI 的参数序列。
//
// 设计：atlas 自身命名不一致——dbmigrate 的 push 对应 atlas 的 `schema apply`，
// 其他 action 名字一致。把这种命名映射收敛在一处，避免散在 run() 各 if 分支。
func buildAtlasArgs(action, dialect, name, base string) []string {
	env := []string{"--env", "local_" + dialect}
	switch action {
	case "diff":
		return append([]string{"migrate", "diff", name}, env...)
	case "lint":
		args := append([]string{"migrate", "lint"}, env...)
		if base != "" {
			return append(args, "--git-base", base)
		}
		// 默认 lint 最近 1 个 migration：local 开发最常用语义；
		// CI 跑 PR 检查时建议传 -base=origin/main 覆盖。
		return append(args, "--latest", "1")
	case "push":
		// drizzle-push 风格：把 GORM struct 描述的目标 schema 直接同步到 DB，
		// 不生成 migration 文件。atlas 的 declarative 模式（schema apply）即此功能。
		// --auto-approve 跳过交互确认；安全性由 requiresLocal 在 run() 里强制保证。
		return append([]string{"schema", "apply", "--auto-approve"}, env...)
	default:
		return append([]string{"migrate", action}, env...)
	}
}

func configPath() string {
	if path := os.Getenv("APP_CONF"); path != "" {
		return path
	}
	return "config/local.yml"
}

func readConfig(path string) (*viper.Viper, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	conf := viper.New()
	conf.SetConfigType("yaml")
	if err := conf.ReadConfig(f); err != nil {
		return nil, err
	}
	conf.SetEnvPrefix(envPrefix)
	conf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	conf.AutomaticEnv()
	for _, key := range []string{"data.db.user.driver", "data.db.user.dsn"} {
		if err := conf.BindEnv(key); err != nil {
			return nil, err
		}
	}
	return conf, nil
}

func normalizeDialect(driver string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "mysql":
		return "mysql", nil
	case "postgres", "postgresql":
		return "postgres", nil
	default:
		return "", fmt.Errorf("migration only supports mysql/postgres, current driver=%q", driver)
	}
}

func atlasURL(dialect, dsn string) (string, error) {
	switch dialect {
	case "mysql":
		return mysqlAtlasURL(dsn)
	case "postgres":
		return dsn, nil
	default:
		return "", fmt.Errorf("unsupported atlas dialect %q", dialect)
	}
}

// mysqlAtlasURL 把 GORM/go-sql-driver 风格的 MySQL DSN 转成 atlas URL。
//
// 用 mysql.ParseDSN 而非手写 strings.Cut 切片：后者对密码含 @ / 等特殊字符
// 直接出错，且不能识别非 tcp 协议；前者是上游官方解析器，行为稳定。
//
// 但 query string 不走 cfg.FormatDSN()——FormatDSN 会按字典序重排参数，并把
// parseTime=True 转成小写 parseTime=true，影响某些 atlas 旧版本的兼容性。
// 改为从原 DSN 直接截取 ? 之后保留原顺序与大小写。
func mysqlAtlasURL(dsn string) (string, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("invalid mysql dsn: %w", err)
	}
	if cfg.Net != "tcp" {
		return "", fmt.Errorf("mysql migration only supports tcp DSN, got %q", cfg.Net)
	}
	if cfg.DBName == "" {
		return "", errors.New("invalid mysql dsn: database is empty")
	}
	u := url.URL{
		Scheme: "mysql",
		User:   userInfo(cfg.User, cfg.Passwd),
		Host:   cfg.Addr,
		Path:   "/" + cfg.DBName,
	}
	if i := strings.IndexByte(dsn, '?'); i >= 0 {
		u.RawQuery = dsn[i+1:]
	}
	return u.String(), nil
}

// userInfo 在密码为空时退化成 `user@host`，避免输出冗余的 `user:@host`。
func userInfo(user, passwd string) *url.Userinfo {
	if user == "" {
		return nil
	}
	if passwd == "" {
		return url.User(user)
	}
	return url.UserPassword(user, passwd)
}

// isLocalDSN 在 DSN 指向本地 host 时返回 true，是 push 的安全栅栏。
//
// 必要性：push（atlas schema apply）会直接执行 DDL，跳过版本化 SQL；
// 如果 DSN 指向 staging/prod，会绕过 review 流程造成不可挽回的破坏。
// 在 Go 层做检查而不是放在 Makefile/shell，是因为下面已经有 mysql.ParseDSN
// 与 net/url.Parse 可复用，shell 解析 DSN 易碎且不可测。
func isLocalDSN(dialect, dsn string) (bool, error) {
	var host string
	switch dialect {
	case "mysql":
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			return false, fmt.Errorf("parse mysql dsn: %w", err)
		}
		// unix socket 显式视为本地：避免依赖 SplitHostPort 报错被静默吞而"巧合通过"。
		if cfg.Net == "unix" {
			return true, nil
		}
		host, _, err = net.SplitHostPort(cfg.Addr)
		if err != nil {
			return false, fmt.Errorf("split mysql addr %q: %w", cfg.Addr, err)
		}
	case "postgres":
		u, err := url.Parse(dsn)
		if err != nil {
			return false, fmt.Errorf("parse postgres dsn: %w", err)
		}
		host = u.Hostname()
	default:
		return false, fmt.Errorf("unsupported dialect %q", dialect)
	}
	return isLocalHost(host), nil
}

// isLocalHost 把"本地"显式枚举，避免把 10.x/192.168.x 等 LAN 段当本地。
// LAN 段可能是同事的开发机或测试环境，push 不该自动信任。
//
// 0.0.0.0 故意不放入白名单：它是"监听 0.0.0.0"语义而非"客户端连接 0.0.0.0"，
// 出现在 DSN 里基本是配置错误，宁可让 push 失败让用户排查。
//
// 大小写归一化：mysql/postgres DSN 解析后均保留 host 原始大小写，
// 这里统一转小写后比较，避免 LOCALHOST/Localhost 被判为远程。
func isLocalHost(host string) bool {
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1", "":
		return true
	}
	return false
}
