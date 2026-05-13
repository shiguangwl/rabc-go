// Package main 提供 Atlas migration 的项目级命令入口。
//
// dbmigrate 只负责读取配置、准备本地 dev 库、组装 atlas 参数并执行 atlas。
// schema 真相源在 internal/model 与 db/atlas/main.go，migration 文件在
// db/migrations/{mysql,postgres}/。
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
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/viper"
)

const envPrefix = "APP"

// actionSpec 描述命令运行前必须满足的配置和安全边界。
type actionSpec struct {
	requiresName  bool
	requiresDSN   bool
	requiresLocal bool
	ensureDevDB   bool
	allDialects   bool
}

// actions 是 dbmigrate 支持的命令清单；新增命令时必须同步 buildAtlasArgs。
var actions = map[string]actionSpec{
	"diff":     {requiresName: true, requiresDSN: true, ensureDevDB: true},
	"apply":    {requiresDSN: true},
	"status":   {requiresDSN: true},
	"hash":     {allDialects: true},
	"validate": {allDialects: true},
	"lint":     {},
	"push":     {requiresDSN: true, requiresLocal: true, ensureDevDB: true},
}

var migrationDialects = []string{"mysql", "postgres"}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("dbmigrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	confPath := flags.String("conf", configPath(), "配置文件路径")
	name := flags.String("name", "", "迁移名称（用于 diff，例如 add_user_email）")
	base := flags.String("base", "", "lint 的 git 基准引用（例如 origin/main）；默认使用 --latest 1")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("解析命令行参数失败: %w", err)
	}
	if flags.NArg() != 1 {
		return errors.New("使用方法: dbmigrate [-conf config/local.yml] [-name X] [-base ref] " +
			"<diff|apply|status|hash|validate|lint|push>")
	}

	action := flags.Arg(0)
	spec, ok := actions[action]
	if !ok {
		return fmt.Errorf("不支持的迁移命令: %q", action)
	}
	if spec.requiresName && strings.TrimSpace(*name) == "" {
		return errors.New("迁移名称是必填项: -name <值>")
	}

	if spec.allDialects {
		for _, dialect := range migrationDialects {
			if err := runAtlas(action, dialect, buildAtlasArgs(action, dialect, *name, *base)); err != nil {
				return err
			}
		}
		return nil
	}

	conf, err := readConfig(*confPath)
	if err != nil {
		return fmt.Errorf("读取配置文件 %q 失败: %w", *confPath, err)
	}
	driver := conf.GetString("data.db.user.driver")
	dsn := strings.TrimSpace(conf.GetString("data.db.user.dsn"))
	dialect, err := normalizeDialect(driver)
	if err != nil {
		return fmt.Errorf("标准化数据库方言失败: %w", err)
	}
	if spec.requiresDSN && dsn == "" {
		return fmt.Errorf("执行迁移 %s 需要 data.db.user.dsn 配置", action)
	}
	if spec.requiresLocal {
		local, err := isLocalDSN(dialect, dsn)
		if err != nil {
			return fmt.Errorf("检查本地 DSN 失败: %w", err)
		}
		if !local {
			return errors.New("push 仅限本地开发：DSN 必须指向 localhost / 127.0.0.1 / ::1 或 unix socket；" +
				"非本地环境请走 migrate-diff + migrate-apply 版本化流程")
		}
	}
	if spec.ensureDevDB && dsn != "" {
		if err := ensureAtlasDevDB(dialect, dsn); err != nil {
			return fmt.Errorf("确保 Atlas 开发数据库存在失败: %w", err)
		}
	}

	argv := buildAtlasArgs(action, dialect, *name, *base)
	if spec.requiresDSN {
		atlasURL, err := atlasURL(dialect, dsn)
		if err != nil {
			return fmt.Errorf("构建 Atlas URL 失败: %w", err)
		}
		argv = append(argv, "--url", atlasURL)
	}

	return runAtlas(action, dialect, argv)
}

func ensureAtlasDevDB(dialect, dsn string) error {
	local, err := isLocalDSN(dialect, dsn)
	if err != nil {
		return fmt.Errorf("检查本地 DSN 失败: %w", err)
	}
	if !local {
		return nil
	}
	switch dialect {
	case "mysql":
		return ensureMySQLAtlasDevDB(dsn)
	case "postgres":
		return ensurePostgresAtlasDevDB(dsn)
	default:
		return fmt.Errorf("不支持的数据库: %q", dialect)
	}
}

func ensureMySQLAtlasDevDB(dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("解析 MySQL DSN 失败: %w", err)
	}
	if cfg.Net != "tcp" {
		return fmt.Errorf("MySQL 迁移仅支持 TCP 协议的 DSN，当前为 %q", cfg.Net)
	}
	cfg.DBName = ""
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer closeWithError(&err, db)
	_, err = db.ExecContext(context.Background(), "CREATE DATABASE IF NOT EXISTS `atlas_dev` DEFAULT CHARACTER SET utf8mb4")
	return err
}

func ensurePostgresAtlasDevDB(dsn string) error {
	adminDSN, err := postgresAdminDSN(dsn)
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return err
	}
	defer closeWithError(&err, db)

	var exists bool
	if queryErr := db.QueryRowContext(context.Background(), "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = 'atlas_dev')").Scan(&exists); queryErr != nil {
		err = queryErr
		return err
	}
	if exists {
		return nil
	}
	_, err = db.ExecContext(context.Background(), `CREATE DATABASE atlas_dev`)
	return err
}

func postgresAdminDSN(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("解析 PostgreSQL DSN 失败: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", fmt.Errorf("PostgreSQL DSN 必须使用 postgres/postgresql 协议，当前为 %q", u.Scheme)
	}
	u.Path = "/postgres"
	u.RawPath = ""
	return u.String(), nil
}

func runAtlas(action, dialect string, argv []string) error {
	cmd := exec.CommandContext(context.Background(), "atlas", argv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if dialect != "" {
		cmd.Env = append(cmd.Env, "ATLAS_DIALECT="+dialect)
	}
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("atlas CLI 未安装或不在 PATH，请先安装 Atlas")
		}
		return fmt.Errorf("执行 atlas %s 失败: %w", action, err)
	}
	return nil
}

// buildAtlasArgs 将项目命令映射为 atlas CLI 参数。
//
// atlas 的 declarative schema apply 在本项目暴露为 push，避免调用方接触
// atlas 子命令命名差异。
func buildAtlasArgs(action, dialect, name, base string) []string {
	env := []string{"--env", "local_" + dialect}
	switch action {
	case "diff":
		return append([]string{"migrate", "diff", name}, env...)
	case "hash":
		return []string{"migrate", "hash", "--dir", "file://db/migrations/" + dialect}
	case "validate":
		return []string{"migrate", "validate", "--dir", "file://db/migrations/" + dialect}
	case "lint":
		args := append([]string{"migrate", "lint"}, env...)
		if base != "" {
			return append(args, "--git-base", base)
		}
		return append(args, "--latest", "1")
	case "push":
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
	defer closeWithError(&err, f)

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

func closeWithError(errp *error, closer io.Closer) {
	if closeErr := closer.Close(); closeErr != nil && *errp == nil {
		*errp = closeErr
	}
}

func normalizeDialect(driver string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "mysql":
		return "mysql", nil
	case "postgres", "postgresql":
		return "postgres", nil
	default:
		return "", fmt.Errorf("迁移仅支持 mysql/postgres，当前驱动为 %q", driver)
	}
}

func atlasURL(dialect, dsn string) (string, error) {
	switch dialect {
	case "mysql":
		return mysqlAtlasURL(dsn)
	case "postgres":
		return dsn, nil
	default:
		return "", fmt.Errorf("不支持的 Atlas 方言: %q", dialect)
	}
}

// mysqlAtlasURL 将 go-sql-driver/mysql DSN 转为 atlas URL。
//
// 调用方必须传入 TCP DSN 且包含数据库名；query string 保留原始顺序与大小写，
// 以兼容依赖参数原文的 atlas 版本。
func mysqlAtlasURL(dsn string) (string, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("无效的 MySQL DSN: %w", err)
	}
	if cfg.Net != "tcp" {
		return "", fmt.Errorf("MySQL 迁移仅支持 TCP 协议的 DSN，当前为 %q", cfg.Net)
	}
	if cfg.DBName == "" {
		return "", errors.New("无效的 MySQL DSN: 数据库名称为空")
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
// push 会直接执行 DDL，调用前必须拒绝非本地 DSN。
func isLocalDSN(dialect, dsn string) (bool, error) {
	var host string
	switch dialect {
	case "mysql":
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			return false, fmt.Errorf("解析 MySQL DSN 失败: %w", err)
		}
		if cfg.Net == "unix" {
			return true, nil
		}
		host, _, err = net.SplitHostPort(cfg.Addr)
		if err != nil {
			return false, fmt.Errorf("分割 MySQL 地址 %q 失败: %w", cfg.Addr, err)
		}
	case "postgres":
		u, err := url.Parse(dsn)
		if err != nil {
			return false, fmt.Errorf("解析 PostgreSQL DSN 失败: %w", err)
		}
		host = u.Hostname()
	default:
		return false, fmt.Errorf("不支持的数据库方言: %q", dialect)
	}
	return isLocalHost(host), nil
}

// isLocalHost 把"本地"显式枚举，避免把 10.x/192.168.x 等 LAN 段当本地。
//
// 0.0.0.0 是监听地址语义，不属于可被 push 信任的客户端连接地址。
func isLocalHost(host string) bool {
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1", "":
		return true
	}
	return false
}
