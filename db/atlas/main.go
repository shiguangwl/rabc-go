// Package main 是 atlas-provider-gorm 的 schema 加载器入口。
//
// 工作原理：
//
//	atlas CLI 通过 atlas.hcl 中 `data "external_schema"` 块调用本程序，
//	本程序导入项目内全部 GORM model 并交给 gormschema.New(dialect).Load(...)
//	翻译成指定方言 DDL，写入 stdout 后由 atlas 接管做 schema diff 与
//	migration 文件生成。
//
// 维护规则：
//   - 新增需要被 atlas 管理的表，必须显式登记到 models() 列表，避免漏迁
//   - ATLAS_DIALECT 为空时默认 mysql；运行时驱动支持见 repository.NewDB
package main

import (
	"fmt"
	"io"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"

	"nunu-layout-admin/internal/model"
)

// casbinRule 是 atlas schema 镜像，专为 DDL 生成存在，运行时不被 enforcer 使用。
//
// 列定义须与 gorm-adapter v3 的 CasbinRule 严格对齐：
// id(uint pk autoIncrement) + ptype/v0..v5 (varchar 100)。
//
// 在镜像里额外补上 (ptype,v0,v1,v2,v3,v4,v5) 唯一索引：adapter 在 runtime
// AutoMigrate 之后会单独 CREATE UNIQUE INDEX idx_casbin_rule，但它的 struct
// 本身没声明该索引。我们禁用了 AutoMigrate，索引必须由 atlas 接管下来，
// 否则缺少唯一约束会让策略表出现重复行。
//
// 升级 gorm-adapter 大版本前，须先比对其 CasbinRule 字段是否仍兼容此镜像；
// 字段漂移会导致 atlas migrate diff 误判，需同步刷新本结构与 migration。
type casbinRule struct {
	ID    uint   `gorm:"primaryKey;autoIncrement"`
	Ptype string `gorm:"size:100;uniqueIndex:idx_casbin_rule,priority:1"`
	V0    string `gorm:"size:100;uniqueIndex:idx_casbin_rule,priority:2"`
	V1    string `gorm:"size:100;uniqueIndex:idx_casbin_rule,priority:3"`
	V2    string `gorm:"size:100;uniqueIndex:idx_casbin_rule,priority:4"`
	V3    string `gorm:"size:100;uniqueIndex:idx_casbin_rule,priority:5"`
	V4    string `gorm:"size:100;uniqueIndex:idx_casbin_rule,priority:6"`
	V5    string `gorm:"size:100;uniqueIndex:idx_casbin_rule,priority:7"`
}

func (casbinRule) TableName() string { return "casbin_rule" }

// models 集中声明纳入 atlas schema 管理的表。
func models() []any {
	return []any{
		&model.AdminUser{},
		&model.Menu{},
		&model.Role{},
		&model.Api{},
		&casbinRule{},
	}
}

func main() {
	dialect := os.Getenv("ATLAS_DIALECT")
	if dialect == "" {
		dialect = "mysql"
	}
	stmts, err := gormschema.New(dialect).Load(models()...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load gorm schema: %v\n", err)
		os.Exit(1)
	}
	_, _ = io.WriteString(os.Stdout, stmts)
}
