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
// 数据库地址：默认指向宿主机本地数据库。
//
// cmd/dbmigrate 会从 APP_CONF / APP_DATA_DB_USER_DSN 读取目标库并通过 --url
// 传给 atlas，避免在 HCL 里依赖新版函数导致旧 Atlas CLI 解析失败。
// ----------------------------------------------------------------------------

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
  url = "mysql://root:123456@127.0.0.1:3306/user"
  // dev DB 用单独 schema，避免 atlas 自启 docker 容器
  // （在 OrbStack 下 docker:// URL 会超时）。cmd/dbmigrate 会为本地库自动创建。
  dev = "mysql://root:123456@127.0.0.1:3306/atlas_dev"
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
  url = "postgres://postgres:123456@127.0.0.1:5432/user?sslmode=disable"
  // dev DB 由 cmd/dbmigrate 在本地自动创建。
  dev = "postgres://postgres:123456@127.0.0.1:5432/atlas_dev?sslmode=disable"
  migration {
    dir = "file://db/migrations/postgres"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
