package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
)

const envPrefix = "APP"

var supportedCommands = map[string]bool{"diff": true, "apply": true, "status": true, "hash": true, "validate": true}

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
	name := flags.String("name", "", "migration name for diff")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("usage: dbmigrate [-conf config/local.yml] [-name add_xxx] <diff|apply|status|hash|validate>")
	}

	action := flags.Arg(0)
	if !supportedCommands[action] {
		return fmt.Errorf("unsupported migrate command %q", action)
	}
	if action == "diff" && strings.TrimSpace(*name) == "" {
		return fmt.Errorf("migration name is required: go run ./cmd/dbmigrate -name add_xxx diff")
	}

	conf, err := readConfig(*confPath)
	if err != nil {
		return fmt.Errorf("read config %q: %w", *confPath, err)
	}
	driver := conf.GetString("data.db.user.driver")
	dsn := strings.TrimSpace(conf.GetString("data.db.user.dsn"))
	dialect, err := normalizeDialect(driver)
	if err != nil {
		return err
	}
	if action != "diff" && action != "hash" && action != "validate" && dsn == "" {
		return fmt.Errorf("data.db.user.dsn is required for migrate %s", action)
	}

	argv := []string{"migrate", action}
	if action == "diff" {
		argv = append(argv, *name)
	}
	argv = append(argv, "--env", "local_"+dialect)
	if dsn != "" && needsTargetURL(action) {
		atlasURL, err := atlasURL(dialect, dsn)
		if err != nil {
			return err
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
		return err
	}
	return nil
}

func needsTargetURL(action string) bool {
	return action == "apply" || action == "status"
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

func readDriver(path string) (string, error) {
	conf, err := readConfig(path)
	if err != nil {
		return "", err
	}
	return conf.GetString("data.db.user.driver"), nil
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

func mysqlAtlasURL(dsn string) (string, error) {
	beforeDB, afterAt, ok := strings.Cut(dsn, "@")
	if !ok {
		return "", fmt.Errorf("invalid mysql dsn: missing @")
	}
	protocol, rest, ok := strings.Cut(afterAt, "(")
	if !ok || protocol != "tcp" {
		return "", fmt.Errorf("mysql migration only supports tcp DSN")
	}
	addr, afterAddr, ok := strings.Cut(rest, ")/")
	if !ok {
		return "", fmt.Errorf("invalid mysql dsn: missing database")
	}
	dbName := afterAddr
	query := ""
	if i := strings.IndexByte(afterAddr, '?'); i >= 0 {
		dbName = afterAddr[:i]
		query = afterAddr[i:]
	}
	if strings.TrimSpace(dbName) == "" {
		return "", fmt.Errorf("invalid mysql dsn: database is empty")
	}
	return "mysql://" + beforeDB + "@" + addr + "/" + dbName + query, nil
}
