// atlas.hcl
//
// Atlas schema-diff 配置。GORM struct 是 schema 的 source of truth：
// `db/atlas` 程序按 ATLAS_DIALECT 把 GORM model 翻译成对应方言 DDL，
// atlas 用其与目标库对比，生成版本化 migration 文件到方言专属目录。
//
// 工作流：
//   make migrate-diff name=add_xxx        // 生成新 migration
//   make migrate-apply                    // 应用 migration 到目标 DB
//   make migrate-lint                     // CI 级破坏性变更检测
//
// 默认 migration 方言是 MySQL；运行时驱动由 data.db.user.driver 决定。
// PostgreSQL 使用 local_postgres env 与 db/migrations/postgres 目录。
//
// ----------------------------------------------------------------------------
// 数据库地址由 cmd/dbmigrate 从 APP_CONF / APP_DATA_DB_USER_DSN 读取后注入，
// atlas.hcl 只声明 schema 来源、dev 库变量和 migration 目录。
// ----------------------------------------------------------------------------

variable "db_url" {
  type = string
}

variable "dev_url" {
  type = string
}

data "external_schema" "gorm" {
  program = [
    "go",
    "run",
    "-mod=mod",
    "./db/atlas",
  ]
}

env "local_mysql" {
  src = data.external_schema.gorm.url
  url = var.db_url
  dev = var.dev_url
  migration {
    dir = "file://db/migrations/mysql"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

env "local_postgres" {
  src = data.external_schema.gorm.url
  url = var.db_url
  dev = var.dev_url
  migration {
    dir = "file://db/migrations/postgres"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
